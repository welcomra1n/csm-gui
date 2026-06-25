package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func base64DecodeStd(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "csm-gui")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// App struct
type App struct {
	ctx     context.Context
	ptyMgr  *PtyManager
	version string
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{}
	a.ptyMgr = NewPtyManager(a)
	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ListSessions returns all discovered sessions with metadata applied.
func (a *App) ListSessions() []*Session {
	sessions := discoverSessions()
	meta := loadMetadata()
	for _, s := range sessions {
		if folder, ok := meta.SessionFolders[s.ID]; ok {
			s.Folder = folder
		}
		if tags, ok := meta.SessionTags[s.ID]; ok {
			s.Tags = tags
		}
		if recap, ok := meta.Recaps[s.ID]; ok {
			s.Recap = recap
		}
	}
	return sessions
}

// GetSession returns a single session by ID, or nil if not found.
func (a *App) GetSession(id string) *Session {
	sessions := a.ListSessions()
	for _, s := range sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// PinSession pins or unpins a session.
func (a *App) PinSession(id string, pinned bool) error {
	pins := loadPins()
	unpins := loadUnpins()

	if pinned {
		pins[id] = true
		delete(unpins, id)
	} else {
		delete(pins, id)
		unpins[id] = true
	}

	if err := savePins(pins); err != nil {
		return err
	}
	saveUnpins(unpins)
	return nil
}

// SetSessionTag sets tags on a session.
func (a *App) SetSessionTag(id string, tags []string) error {
	meta := loadMetadata()
	if meta.SessionTags == nil {
		meta.SessionTags = make(map[string][]string)
	}
	meta.SessionTags[id] = tags
	saveMetadata(meta)
	return nil
}

// SetSessionFolder assigns a session to a folder.
func (a *App) SetSessionFolder(id string, folder string) error {
	meta := loadMetadata()
	if meta.SessionFolders == nil {
		meta.SessionFolders = make(map[string]string)
	}
	if folder == "" {
		delete(meta.SessionFolders, id)
	} else {
		meta.SessionFolders[id] = folder
		// Ensure folder exists in the list
		found := false
		for _, f := range meta.Folders {
			if f == folder {
				found = true
				break
			}
		}
		if !found {
			meta.Folders = append(meta.Folders, folder)
		}
	}
	saveMetadata(meta)
	return nil
}

// CreateFolder adds a new empty folder.
func (a *App) CreateFolder(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty name")
	}
	meta := loadMetadata()
	for _, f := range meta.Folders {
		if f == name {
			return nil
		}
	}
	meta.Folders = append(meta.Folders, name)
	saveMetadata(meta)
	return nil
}

// RenameFolder renames a folder and migrates all sessions in it.
func (a *App) RenameFolder(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("empty name")
	}
	meta := loadMetadata()
	if meta.SessionFolders == nil {
		meta.SessionFolders = make(map[string]string)
	}
	for id, f := range meta.SessionFolders {
		if f == oldName {
			meta.SessionFolders[id] = newName
		}
	}
	newFolders := []string{}
	seen := map[string]bool{}
	for _, f := range meta.Folders {
		if f == oldName {
			f = newName
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		newFolders = append(newFolders, f)
	}
	if !seen[newName] {
		newFolders = append(newFolders, newName)
	}
	meta.Folders = newFolders
	saveMetadata(meta)
	return nil
}

// DeleteFolder removes a folder (sessions inside become unfiled).
func (a *App) DeleteFolder(name string) error {
	meta := loadMetadata()
	if meta.SessionFolders != nil {
		for id, f := range meta.SessionFolders {
			if f == name {
				delete(meta.SessionFolders, id)
			}
		}
	}
	out := meta.Folders[:0]
	for _, f := range meta.Folders {
		if f != name {
			out = append(out, f)
		}
	}
	meta.Folders = out
	saveMetadata(meta)
	return nil
}

// ListFolders returns the list of defined folders.
func (a *App) ListFolders() []string {
	meta := loadMetadata()
	if meta.Folders == nil {
		return []string{}
	}
	return meta.Folders
}

// RenameAlias sets a user-defined alias for a session.
func (a *App) RenameAlias(id string, alias string) error {
	aliases := loadAliases()
	if alias == "" {
		delete(aliases, id)
	} else {
		aliases[id] = alias
	}
	return saveAliases(aliases)
}

// DeleteSession moves a session's JSONL file to the trash directory.
func (a *App) DeleteSession(id string) error {
	s := a.GetSession(id)
	if s == nil {
		return fmt.Errorf("session not found: %s", id)
	}
	return deleteSession(s)
}

// DeleteSessions trashes many sessions in one scan. Returns count deleted.
// Parallelized for big groups (scroll = 742). Skips per-file .meta sidecar
// when batch > 50 to avoid 2× WriteFile syscalls per session.
func (a *App) DeleteSessions(ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	all := discoverSessionsFast()
	var matched []*Session
	for _, s := range all {
		if wanted[s.ID] {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		return 0, nil
	}
	skipMeta := len(matched) > 50

	const workers = 8
	jobs := make(chan *Session, len(matched))
	var wg sync.WaitGroup
	var count int64
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				if err := deleteSessionEx(s, skipMeta); err == nil {
					atomic.AddInt64(&count, 1)
				}
			}
		}()
	}
	for _, s := range matched {
		jobs <- s
	}
	close(jobs)
	wg.Wait()
	return int(count), nil
}

// AppVersion returns the embedded build version.
func (a *App) AppVersion() string {
	if a.version == "" {
		return "dev"
	}
	return a.version
}

// UpdateInfo describes the latest release available upstream.
type UpdateInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	URL         string `json:"url"`
	HasUpdate   bool   `json:"hasUpdate"`
	Body        string `json:"body"`
}

// CheckUpdate queries GitHub for the latest csm-gui release.
func (a *App) CheckUpdate() (*UpdateInfo, error) {
	resp, err := httpGet("https://api.github.com/repos/welcomra1n/csm-gui/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("network: %w", err)
	}
	var raw struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := jsonUnmarshal(resp, &raw); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	latest := strings.TrimPrefix(raw.TagName, "v")
	current := strings.TrimPrefix(a.AppVersion(), "v")
	return &UpdateInfo{
		Current:   current,
		Latest:    latest,
		URL:       raw.HTMLURL,
		HasUpdate: latest != "" && latest != current && current != "dev",
		Body:      raw.Body,
	}, nil
}

// ApplyUpdate runs the platform's package manager to upgrade csm-gui.
// Returns the command output (or error).
func (a *App) ApplyUpdate() (string, error) {
	if runtime.GOOS == "darwin" {
		augmentPATH()
		brew, err := exec.LookPath("brew")
		if err != nil {
			brew = resolveViaShell("brew")
			if brew == "" {
				return "", fmt.Errorf("brew not found in PATH")
			}
		}
		// Refresh tap then upgrade (force handles edge cases)
		c1 := exec.Command(brew, "update")
		hideConsole(c1)
		out1, _ := c1.CombinedOutput()
		c2 := exec.Command(brew, "reinstall", "--cask", "csm-gui")
		hideConsole(c2)
		out2, err := c2.CombinedOutput()
		return string(out1) + "\n" + string(out2), err
	}
	if runtime.GOOS == "windows" {
		// Refresh bucket list first (can run while csm.exe is alive).
		if scoop, err := exec.LookPath("scoop"); err == nil {
			c1 := exec.Command(scoop, "update")
			hideConsole(c1)
			_, _ = c1.CombinedOutput()
		}
		// The actual `scoop update csm-gui` must happen AFTER this process
		// exits or scoop fails to overwrite the locked csm.exe. Schedule
		// a detached VBS that waits for our exit, runs scoop, then relaunches.
		exe, _ := os.Executable()
		spawnWindowsUpdater(exe)
		go func() {
			time.Sleep(300 * time.Millisecond)
			wruntime.Quit(a.ctx)
		}()
		return "updater scheduled — app will close and relaunch when scoop finishes", nil
	}
	return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}

// RestartApp relaunches the application after a 1-second delay,
// then quits the current instance so the package manager's new binary
// is used on next launch.
func (a *App) RestartApp() error {
	if runtime.GOOS == "darwin" {
		appPath := "/Applications/csm.app"
		if _, err := os.Stat(appPath); err != nil {
			// fallback to find via brew cask
			appPath = ""
			matches, _ := filepath.Glob("/opt/homebrew/Caskroom/csm-gui/*/csm.app")
			if len(matches) > 0 {
				appPath = matches[0]
			}
		}
		if appPath == "" {
			return fmt.Errorf("could not locate csm.app")
		}
		go func() {
			exec.Command("sh", "-c", "sleep 1 && open '"+appPath+"' &").Start()
			time.Sleep(200 * time.Millisecond)
			wruntime.Quit(a.ctx)
		}()
		return nil
	}
	if runtime.GOOS == "windows" {
		exe, _ := os.Executable()
		go func() {
			spawnWindowsRelauncher(exe)
			time.Sleep(200 * time.Millisecond)
			wruntime.Quit(a.ctx)
		}()
		return nil
	}
	return fmt.Errorf("unsupported platform")
}

// Permission describes one app capability shown in the permissions modal.
type Permission struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"` // "os" or "app"
	Required    bool   `json:"required"`
	SystemURL   string `json:"systemUrl"`
	Enabled     bool   `json:"enabled"`
}

// ListPermissions returns the full permission set + current state.
func (a *App) ListPermissions() []Permission {
	meta := loadMetadata()
	prefs := meta.Prefs
	get := func(k string, def bool) bool {
		if v, ok := prefs[k]; ok {
			return v
		}
		return def
	}

	osFullDisk := ""
	osNotif := ""
	if runtime.GOOS == "darwin" {
		osFullDisk = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"
		osNotif = "x-apple.systempreferences:com.apple.preference.notifications"
	}

	return []Permission{
		{
			Key:      "fs-access",
			Label:    "파일 접근권한",
			Category: "os",
			Required: true,
			SystemURL: osFullDisk,
			Enabled:  true,
			Description: "다음 경로 읽기/쓰기:\n" +
				"• ~/.claude/projects/**/*.jsonl — Claude Code 세션 본문 (읽기)\n" +
				"• ~/.claude/csm-metadata.json — 별칭, 폴더, 태그, 핀, 열린 탭, recap (읽기/쓰기)\n" +
				"• ~/.claude/session-trash/ — 삭제한 세션 보관 (쓰기)\n" +
				"• ~/.codex/sessions/**/*.jsonl — Codex CLI 세션 (읽기)\n" +
				"• ~/.codex/session_index.jsonl — Codex 인덱스 (읽기/쓰기, 삭제 시)\n" +
				"• /tmp, $TMPDIR — 클립보드 이미지 임시 저장 (쓰기)\n" +
				"• /Applications/csm.app, /opt/homebrew/Caskroom/csm-gui — 업데이트 후 재시작 시 위치 확인 (읽기)",
		},
		{
			Key:      "pty-exec",
			Label:    "외부 CLI 실행 권한",
			Category: "app",
			Required: true,
			SystemURL: "",
			Enabled:  true,
			Description: "다음 바이너리를 PTY로 실행:\n" +
				"• claude — Claude Code CLI 세션\n" +
				"• codex — Codex CLI 세션\n" +
				"• zsh / pwsh.exe — 일반 셸 탭\n" +
				"• node.exe — Windows에서 npm .cmd shim 우회용 (콘솔 플리커 방지)\n" +
				"• brew / scoop — 자동 업데이트\n" +
				"• wscript.exe (Windows) / open (macOS) — 업데이트 후 자동 재시작",
		},
		{
			Key:         "notifications",
			Label:       "시스템 알림 표시",
			Description: "서브에이전트 완료 시 OS 알림(창이 비활성일 때). 끄려면 토글 해제.",
			Category:    "app",
			Required:    false,
			SystemURL:   osNotif,
			Enabled:     get("notifications", true),
		},
		{
			Key:         "auto-recap",
			Label:       "탭 닫을 때 자동 요약",
			Description: "탭 종료 시 백그라운드로 claude -p 호출해 3줄 recap 생성. claude 토큰 사용.",
			Category:    "app",
			Required:    false,
			SystemURL:   "",
			Enabled:     get("auto-recap", true),
		},
		{
			Key:      "update-check",
			Label:    "네트워크 접근 (업데이트 확인)",
			Category: "app",
			Required: false,
			SystemURL: "",
			Enabled:  get("update-check", true),
			Description: "설정창 열 때 GitHub에 HTTPS GET 요청:\n" +
				"• api.github.com/repos/welcomra1n/csm-gui/releases/latest — 최신 버전 조회\n" +
				"• 응답으로 받은 zip URL만 사용 (다른 호스트 안 건드림)\n" +
				"• 끄면 설정창에서 수동 클릭 시에만 조회",
		},
	}
}

// SetPermission updates an app-level preference toggle.
func (a *App) SetPermission(key string, enabled bool) error {
	meta := loadMetadata()
	if meta.Prefs == nil {
		meta.Prefs = map[string]bool{}
	}
	meta.Prefs[key] = enabled
	saveMetadata(meta)
	return nil
}

// SaveClipboardImage writes a base64 PNG to a temp file and returns the path.
// Frontend pastes clipboard image -> reads as data URL -> sends base64 here.
func (a *App) SaveClipboardImage(base64Data string) (string, error) {
	// strip data URL prefix if present
	if idx := strings.Index(base64Data, ","); idx > 0 {
		base64Data = base64Data[idx+1:]
	}
	data, err := base64DecodeStd(base64Data)
	if err != nil {
		return "", err
	}
	tmpDir := os.TempDir()
	name := fmt.Sprintf("csm-paste-%d.png", time.Now().UnixNano())
	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ProcessDroppedPath mirrors the clipboard-paste flow for dropped files:
// image files are copied into the OS temp dir under a csm-drop-* name and
// the new temp path is returned, matching how SaveClipboardImage behaves
// for clipboard images. Non-image files and directories pass through
// unchanged so dropping a project folder still inserts the real path.
func (a *App) ProcessDroppedPath(srcPath string) (string, error) {
	if srcPath == "" {
		return "", fmt.Errorf("empty path")
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return srcPath, nil
	}
	if info.IsDir() {
		return srcPath, nil
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	isImage := false
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".tif", ".svg", ".heic", ".heif":
		isImage = true
	}
	if !isImage {
		return srcPath, nil
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return srcPath, nil
	}
	tmpDir := os.TempDir()
	name := fmt.Sprintf("csm-drop-%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(tmpDir, name)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return srcPath, nil
	}
	return dst, nil
}

// OpenURL opens a URL via the system default handler (e.g. System Settings).
func (a *App) OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	if runtime.GOOS == "windows" {
		c := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		hideConsole(c)
		return c.Start()
	}
	return exec.Command("xdg-open", url).Start()
}

// SaveOpenTabs persists the list of open tabs so they survive app updates
// (localStorage is wiped when brew reinstall replaces the .app bundle).
func (a *App) SaveOpenTabs(tabs []SavedTab) error {
	meta := loadMetadata()
	meta.OpenTabs = tabs
	saveMetadata(meta)
	return nil
}

// LoadOpenTabs returns the previously saved tabs.
func (a *App) LoadOpenTabs() []SavedTab {
	meta := loadMetadata()
	return meta.OpenTabs
}

// GetMetadata returns the raw metadata object.
func (a *App) GetMetadata() *Metadata {
	return loadMetadata()
}

// GenerateRecap calls claude headlessly to produce a 2-3 sentence recap
// of the session and caches it in metadata. Force=true regenerates.
func (a *App) GenerateRecap(id string, force bool) (string, error) {
	meta := loadMetadata()
	if !force {
		if r, ok := meta.Recaps[id]; ok && r != "" {
			return r, nil
		}
	}
	s := a.GetSession(id)
	if s == nil {
		return "", fmt.Errorf("session not found")
	}

	// Build context: first + recent messages
	var ctx strings.Builder
	limit := 30
	start := 0
	if len(s.Messages) > limit {
		start = len(s.Messages) - limit
	}
	for i := start; i < len(s.Messages); i++ {
		m := s.Messages[i]
		ctx.WriteString(strings.ToUpper(m.Type))
		ctx.WriteString(": ")
		c := m.Content
		if len(c) > 500 {
			c = c[:500] + "…"
		}
		ctx.WriteString(c)
		ctx.WriteString("\n\n")
	}

	augmentPATH()
	cmd := "claude"
	if lp, lerr := exec.LookPath(cmd); lerr == nil {
		cmd = lp
	} else if rp := resolveViaShell(cmd); rp != "" {
		cmd = rp
	}

	prompt := "한국어로 정확히 세 줄로 이 세션을 요약해줘. 각 줄은 한 문장씩. 형식:\n1. [주제]\n2. [한 일/논의한 것]\n3. [결론/다음 단계]\n\n세션:\n" + ctx.String()

	finalCmd := cmd
	finalArgs := []string{"-p", prompt, "--dangerously-skip-permissions"}
	if nodeExe, jsPath, ok := resolveNpmShim(cmd); ok {
		finalCmd = nodeExe
		finalArgs = append([]string{jsPath}, finalArgs...)
	}
	recapCmd := exec.Command(finalCmd, finalArgs...)
	hideConsole(recapCmd)
	out, err := recapCmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude exec: %w", err)
	}
	recap := strings.TrimSpace(string(out))
	if recap == "" {
		return "", fmt.Errorf("empty recap")
	}

	meta.Recaps[id] = recap
	saveMetadata(meta)
	return recap, nil
}
