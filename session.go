package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ── Provider ─────────────────────────────────────────────────────────────────

type Provider int

const (
	ProviderClaude Provider = iota
	ProviderCodex
)

func (p Provider) Label() string {
	switch p {
	case ProviderCodex:
		return "Codex"
	default:
		return "Claude"
	}
}

func (p Provider) MarshalJSON() ([]byte, error) {
	switch p {
	case ProviderCodex:
		return []byte(`"codex"`), nil
	default:
		return []byte(`"claude"`), nil
	}
}

// ── Core types ────────────────────────────────────────────────────────────────

type Subagent struct {
	ToolUseID    string `json:"toolUseId"`
	SubagentType string `json:"subagentType"`
	Description  string `json:"description"`
	Completed    bool   `json:"completed"`
}


type Session struct {
	ID           string   `json:"id"`
	ProjectDir   string   `json:"projectDir"`
	ProjectName  string   `json:"projectName"`
	SessionFile  string   `json:"sessionFile"`
	ModTime      time.Time `json:"modTime"`
	FileSize     int64    `json:"fileSize"`
	MessageCount int      `json:"messageCount"`
	UserMsgCount int      `json:"userMsgCount"`
	AsstMsgCount int      `json:"asstMsgCount"`
	FirstUserMsg string   `json:"firstUserMsg"`
	LastUserMsg  string   `json:"lastUserMsg"`
	GitBranch    string   `json:"gitBranch"`
	CWD          string   `json:"cwd"`
	Messages     []Message `json:"messages"`
	Alias        string   `json:"alias"`
	Selected     bool     `json:"selected"`
	Provider     Provider `json:"provider"`
	Entrypoint   string   `json:"entrypoint"`
	Active       bool     `json:"active"`
	Pinned       bool     `json:"pinned"`
	InputTokens  int64    `json:"inputTokens"`
	OutputTokens int64    `json:"outputTokens"`
	Loaded       bool     `json:"loaded"`
	Tags         []string `json:"tags"`
	Folder       string   `json:"folder"`
	Subagents    []Subagent `json:"subagents"`
	Recap        string     `json:"recap"`
}

type Message struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type rawLine struct {
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"message,omitempty"`
	GitBranch  string          `json:"gitBranch,omitempty"`
	CWD        string          `json:"cwd,omitempty"`
	Entrypoint string          `json:"entrypoint,omitempty"`
}

type msgEnvelope struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Usage   *msgUsage       `json:"usage,omitempty"`
}

type msgUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
}

type taskInput struct {
	Description  string `json:"description"`
	SubagentType string `json:"subagent_type"`
}

// ── Path helpers ──────────────────────────────────────────────────────────────

func decodePath(enc string) string {
	if enc == "" {
		return ""
	}
	if len(enc) >= 2 && enc[1] == '-' && ((enc[0] >= 'A' && enc[0] <= 'Z') || (enc[0] >= 'a' && enc[0] <= 'z')) {
		root := string(enc[0]) + ":"
		rest := enc[2:]
		if result := resolveEncoded(root+string(filepath.Separator), rest); result != "" {
			return result
		}
		return root + string(filepath.Separator) + strings.ReplaceAll(rest, "-", string(filepath.Separator))
	}
	if result := resolveEncoded("/", enc[1:]); result != "" {
		return result
	}
	return "/" + strings.ReplaceAll(enc[1:], "-", "/")
}

func resolveEncoded(base, remaining string) string {
	if remaining == "" {
		return base
	}
	parts := strings.Split(remaining, "-")
	for segLen := len(parts); segLen >= 1; segLen-- {
		segment := strings.Join(parts[:segLen], "-")
		candidate := filepath.Join(base, segment)
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		rest := ""
		if segLen < len(parts) {
			rest = strings.Join(parts[segLen:], "-")
		}
		if rest == "" {
			return candidate
		}
		if result := resolveEncoded(candidate, rest); result != "" {
			return result
		}
	}
	return ""
}

func lastSegment(p string) string {
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

// ── Text helpers ──────────────────────────────────────────────────────────────

var metaTags = []string{
	"<local-command-caveat>", "</local-command-caveat>",
	"<command-name>", "</command-name>",
	"<command-message>", "</command-message>",
	"<command-args>", "</command-args>",
	"<local-command-stdout>", "</local-command-stdout>",
	"<system-reminder>", "</system-reminder>",
}

func cleanMeta(s string) string {
	for _, tag := range metaTags {
		s = strings.ReplaceAll(s, tag, "")
	}
	return strings.TrimSpace(s)
}

func parseSubagents(raw json.RawMessage, sess *Session) {
	if len(raw) == 0 || raw[0] != '[' {
		return
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(b, &head) != nil {
			continue
		}
		switch head.Type {
		case "tool_use":
			var tu toolUseBlock
			if json.Unmarshal(b, &tu) != nil || tu.Name != "Task" {
				continue
			}
			var in taskInput
			json.Unmarshal(tu.Input, &in)
			sess.Subagents = append(sess.Subagents, Subagent{
				ToolUseID:    tu.ID,
				SubagentType: in.SubagentType,
				Description:  in.Description,
				Completed:    false,
			})
		case "tool_result":
			var tr toolResultBlock
			if json.Unmarshal(b, &tr) != nil || tr.ToolUseID == "" {
				continue
			}
			for i := range sess.Subagents {
				if sess.Subagents[i].ToolUseID == tr.ToolUseID {
					sess.Subagents[i].Completed = true
					break
				}
			}
		}
	}
}

func extractText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return cleanMeta(s)
	}
	var blocks []contentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, cleanMeta(b.Text))
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func isMeaningfulMsg(s string) bool {
	if len(s) < 3 {
		return false
	}
	first := s
	if idx := strings.IndexAny(s, "\r\n"); idx >= 0 {
		first = strings.TrimSpace(s[:idx])
	}
	if first == "" {
		return false
	}
	if strings.HasPrefix(first, "Caveat:") {
		return false
	}
	if strings.HasPrefix(first, "/") {
		word := strings.Fields(first)[0]
		if !strings.Contains(word[1:], "/") {
			return false
		}
	}
	for _, prefix := range []string{"Set model to", "model", "Model set to"} {
		if first == prefix || strings.HasPrefix(first, prefix+" ") {
			return false
		}
	}
	return true
}

// ── JSONL session loading ─────────────────────────────────────────────────────

func loadSessionFast(path string) *Session {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	dir := filepath.Base(filepath.Dir(path))
	ppath := decodePath(dir)
	home, _ := os.UserHomeDir()
	pname := lastSegment(ppath)
	if ppath == home {
		pname = "미분류"
	}
	return &Session{
		ID:          strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		ProjectDir:  ppath,
		ProjectName: pname,
		SessionFile: path,
		ModTime:     info.ModTime(),
		FileSize:    info.Size(),
	}
}

func loadSessionDetail(sess *Session) {
	f, err := os.Open(sess.SessionFile)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 10<<20)

	for sc.Scan() {
		var raw rawLine
		if json.Unmarshal(sc.Bytes(), &raw) != nil {
			continue
		}
		if raw.Type == "custom-title" {
			var ct struct {
				CustomTitle string `json:"customTitle"`
			}
			if json.Unmarshal(sc.Bytes(), &ct) == nil && ct.CustomTitle != "" {
				sess.Alias = ct.CustomTitle
			}
			continue
		}
		if raw.Type != "user" && raw.Type != "assistant" {
			continue
		}
		var env msgEnvelope
		if json.Unmarshal(raw.Message, &env) != nil {
			continue
		}
		if env.Usage != nil {
			sess.InputTokens += env.Usage.InputTokens
			sess.OutputTokens += env.Usage.OutputTokens
		}

		// Parse subagents (Task tool uses + matching tool_results)
		parseSubagents(env.Content, sess)

		text := extractText(env.Content)
		if text == "" {
			continue
		}

		sess.Messages = append(sess.Messages, Message{Type: raw.Type, Content: text})
		sess.MessageCount++

		if sess.GitBranch == "" && raw.GitBranch != "" {
			sess.GitBranch = raw.GitBranch
		}
		if sess.CWD == "" && raw.CWD != "" {
			sess.CWD = raw.CWD
		}
		if sess.Entrypoint == "" && raw.Entrypoint != "" {
			sess.Entrypoint = raw.Entrypoint
		}

		if raw.Type == "user" {
			sess.UserMsgCount++
			c := strings.TrimSpace(text)
			if isMeaningfulMsg(c) {
				if sess.FirstUserMsg == "" {
					sess.FirstUserMsg = c
				}
				sess.LastUserMsg = c
			}
		} else {
			sess.AsstMsgCount++
		}
	}
	sess.Loaded = true
}

// ── Codex session loading ─────────────────────────────────────────────────────

type codexIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
	CWD        string `json:"cwd"`
}

type codexLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type codexPayload struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type codexContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func extractCodexText(raw json.RawMessage) string {
	var blocks []codexContentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "input_text" || b.Type == "output_text" {
				if t := strings.TrimSpace(b.Text); t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func findCodexSessionFile(id string) string {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".codex", "sessions")
	years, _ := os.ReadDir(base)
	for _, y := range years {
		if !y.IsDir() {
			continue
		}
		months, _ := os.ReadDir(filepath.Join(base, y.Name()))
		for _, m := range months {
			if !m.IsDir() {
				continue
			}
			days, _ := os.ReadDir(filepath.Join(base, y.Name(), m.Name()))
			for _, d := range days {
				if !d.IsDir() {
					continue
				}
				files, _ := os.ReadDir(filepath.Join(base, y.Name(), m.Name(), d.Name()))
				for _, f := range files {
					if strings.Contains(f.Name(), id) {
						return filepath.Join(base, y.Name(), m.Name(), d.Name(), f.Name())
					}
				}
			}
		}
	}
	return ""
}

func loadCodexSession(entry codexIndexEntry) *Session {
	sessionFile := findCodexSessionFile(entry.ID)
	if sessionFile == "" {
		return nil
	}

	info, err := os.Stat(sessionFile)
	if err != nil {
		return nil
	}

	cwd := entry.CWD
	home, _ := os.UserHomeDir()
	projectName := lastSegment(cwd)
	if projectName == "" || strings.HasPrefix(projectName, "20") || cwd == home {
		projectName = "미분류"
	}

	sess := &Session{
		ID:          entry.ID,
		ProjectDir:  cwd,
		ProjectName: projectName,
		SessionFile: sessionFile,
		ModTime:     info.ModTime(),
		FileSize:    info.Size(),
		CWD:         cwd,
		Provider:    ProviderCodex,
	}

	f, err := os.Open(sessionFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 10<<20)

	for sc.Scan() {
		var line codexLine
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Type == "session_meta" && line.Payload != nil {
			var meta struct {
				CWD string `json:"cwd"`
			}
			if json.Unmarshal(line.Payload, &meta) == nil && meta.CWD != "" {
				sess.CWD = meta.CWD
				sess.ProjectDir = meta.CWD
				home, _ := os.UserHomeDir()
				name := lastSegment(meta.CWD)
				if strings.HasPrefix(name, "20") || meta.CWD == home {
					name = "미분류"
				}
				sess.ProjectName = name
			}
			continue
		}
		if line.Type != "response_item" || line.Payload == nil {
			continue
		}
		var payload codexPayload
		if json.Unmarshal(line.Payload, &payload) != nil {
			continue
		}
		if payload.Role != "user" && payload.Role != "assistant" {
			continue
		}
		text := extractCodexText(payload.Content)
		if text == "" {
			continue
		}
		if len(text) > 2000 {
			text = text[:2000] + "..."
		}

		msgType := payload.Role
		sess.Messages = append(sess.Messages, Message{Type: msgType, Content: text})
		sess.MessageCount++

		if msgType == "user" {
			sess.UserMsgCount++
			c := strings.TrimSpace(text)
			if len(c) > 3 && !strings.HasPrefix(c, "#") && !strings.HasPrefix(c, "<") {
				if sess.FirstUserMsg == "" {
					sess.FirstUserMsg = c
				}
				sess.LastUserMsg = c
			}
		} else {
			sess.AsstMsgCount++
		}
	}

	if entry.ThreadName != "" {
		sess.Alias = entry.ThreadName
		if len(sess.Alias) > 40 {
			sess.Alias = sess.Alias[:37] + "..."
		}
	}
	if sess.FirstUserMsg == "" && entry.ThreadName != "" {
		sess.FirstUserMsg = entry.ThreadName
	}

	if sess.MessageCount == 0 {
		return nil
	}
	return sess
}

func discoverCodexSessions() []*Session {
	home, _ := os.UserHomeDir()
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")

	f, err := os.Open(indexPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := make(map[string]codexIndexEntry)
	var order []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 10<<20)

	for sc.Scan() {
		var entry codexIndexEntry
		if json.Unmarshal(sc.Bytes(), &entry) != nil {
			continue
		}
		if _, exists := seen[entry.ID]; !exists {
			order = append(order, entry.ID)
		}
		seen[entry.ID] = entry
	}

	var out []*Session
	for _, id := range order {
		entry := seen[id]
		if s := loadCodexSession(entry); s != nil {
			out = append(out, s)
		}
	}
	return out
}

// ── Active session detection ──────────────────────────────────────────────────

type activeInfo struct {
	Env string
	PID string
}

var cachedActiveIDs map[string]activeInfo
var cachedActiveIDsTime time.Time

func refreshActiveIDs() map[string]activeInfo {
	if time.Since(cachedActiveIDsTime) < 5*time.Second && cachedActiveIDs != nil {
		return cachedActiveIDs
	}
	ids := make(map[string]activeInfo)

	detectEnv := func(line string) string {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "ssh") || strings.Contains(lower, "sshd") {
			return "ssh"
		}
		return "local"
	}

	extractPID := func(line string) string {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0]
		}
		return ""
	}

	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "wmic process where \"commandline like '%--resume%'\" get processid,commandline /format:list").CombinedOutput(); err == nil {
			lines := strings.Split(string(out), "\n")
			var cmdLine, pid string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "CommandLine=") {
					cmdLine = line[len("CommandLine="):]
				} else if strings.HasPrefix(line, "ProcessId=") {
					pid = line[len("ProcessId="):]
				}
				if cmdLine != "" && pid != "" {
					if idx := strings.Index(cmdLine, "--resume"); idx >= 0 {
						rest := strings.TrimSpace(cmdLine[idx+len("--resume"):])
						fields := strings.Fields(rest)
						if len(fields) > 0 {
							ids[fields[0]] = activeInfo{Env: "local", PID: pid}
						}
					} else if idx := strings.Index(cmdLine, "resume"); idx >= 0 {
						rest := strings.TrimSpace(cmdLine[idx+len("resume"):])
						fields := strings.Fields(rest)
						for _, f := range fields {
							if !strings.HasPrefix(f, "-") {
								ids[f] = activeInfo{Env: "local", PID: pid}
								break
							}
						}
					}
					cmdLine, pid = "", ""
				}
			}
		}
	} else {
		if out, err := exec.Command("pgrep", "-afl", "claude.*--resume").CombinedOutput(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if idx := strings.Index(line, "--resume"); idx >= 0 {
					rest := strings.TrimSpace(line[idx+len("--resume"):])
					fields := strings.Fields(rest)
					if len(fields) > 0 {
						ids[fields[0]] = activeInfo{Env: detectEnv(line), PID: extractPID(line)}
					}
				}
			}
		}
		if out, err := exec.Command("pgrep", "-afl", "codex.*resume").CombinedOutput(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if idx := strings.Index(line, "resume"); idx >= 0 {
					rest := strings.TrimSpace(line[idx+len("resume"):])
					fields := strings.Fields(rest)
					for _, f := range fields {
						if !strings.HasPrefix(f, "-") {
							ids[f] = activeInfo{Env: detectEnv(line), PID: extractPID(line)}
							break
						}
					}
				}
			}
		}
	}

	cachedActiveIDs = ids
	cachedActiveIDsTime = time.Now()
	return ids
}

func isSessionActiveByProcess(sessionID string) bool {
	ids := refreshActiveIDs()
	_, exists := ids[sessionID]
	return exists
}

// ── Main discovery ────────────────────────────────────────────────────────────

func discoverSessionsFast() []*Session {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	aliases := loadAliases()
	var out []*Session
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".jsonl") {
				if s := loadSessionFast(filepath.Join(dir, f.Name())); s != nil {
					if alias, ok := aliases[s.ID]; ok {
						s.Alias = alias
					}
					out = append(out, s)
				}
			}
		}
	}
	codexSessions := discoverCodexSessions()
	for _, cs := range codexSessions {
		if alias, ok := aliases[cs.ID]; ok {
			cs.Alias = alias
		}
		out = append(out, cs)
	}
	pins := loadPins()
	for _, s := range out {
		if pins[s.ID] {
			s.Pinned = true
		}
	}
	for _, s := range out {
		s.Active = isSessionActiveByProcess(s.ID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out
}

func discoverSessions() []*Session {
	out := discoverSessionsFast()

	// Parallel load (8 workers)
	const workers = 8
	jobs := make(chan *Session, len(out))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				loadSessionDetail(s)
			}
		}()
	}
	for _, s := range out {
		jobs <- s
	}
	close(jobs)
	wg.Wait()

	valid := out[:0]
	for _, s := range out {
		if s.Provider == ProviderCodex {
			if s.MessageCount > 0 {
				valid = append(valid, s)
			}
			continue
		}
		if s.MessageCount > 0 && s.UserMsgCount >= 1 && s.AsstMsgCount >= 1 {
			valid = append(valid, s)
		}
	}
	return valid
}
