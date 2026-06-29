package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	gopty "github.com/aymanbagabas/go-pty"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ptySession struct {
	id   string
	pty  gopty.Pty
	cmd  *gopty.Cmd
	done chan struct{}

	// Korean jamo streaming-compose state. macOS IMEs hand us one
	// conjoining/compat jamo per WritePty call; we accumulate trailing
	// jamo in jamoBuf and emit completed syllables to the PTY as soon
	// as the streaming composer can prove they're final.
	jamoMu    sync.Mutex
	jamoBuf   []byte
	jamoTimer *time.Timer
}

type PtyManager struct {
	mu       sync.Mutex
	sessions map[string]*ptySession
	app      *App
}

func NewPtyManager(app *App) *PtyManager {
	return &PtyManager{
		sessions: make(map[string]*ptySession),
		app:      app,
	}
}

// StartPty spawns a new PTY running the given command in dir.
// Returns a generated tabId used for subsequent calls.
// Output is streamed via Wails event "pty:output:<tabId>".
// Exit is signaled via "pty:exit:<tabId>".
func (a *App) StartPty(tabId, command, dir string, args []string, cols, rows int) error {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	a.ptyMgr.mu.Lock()
	defer a.ptyMgr.mu.Unlock()

	if _, exists := a.ptyMgr.sessions[tabId]; exists {
		return fmt.Errorf("tab %s already running", tabId)
	}

	p, err := gopty.New()
	if err != nil {
		return fmt.Errorf("pty new: %w", err)
	}
	if err := p.Resize(cols, rows); err != nil {
		p.Close()
		return fmt.Errorf("pty resize: %w", err)
	}

	// Augment PATH with common locations not present in macOS GUI env
	augmentPATH()

	resolved := command
	if lp, lerr := exec.LookPath(command); lerr == nil {
		resolved = lp
	} else if rp := resolveViaShell(command); rp != "" {
		resolved = rp
	}

	// Windows: if resolved is an npm .cmd shim, spawn node directly so
	// cmd.exe's brief console window doesn't flash on each launch.
	finalCmd := resolved
	finalArgs := args
	if nodeExe, jsPath, ok := resolveNpmShim(resolved); ok {
		finalCmd = nodeExe
		finalArgs = append([]string{jsPath}, args...)
	}

	cmd := p.Command(finalCmd, finalArgs...)
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		}
	}
	cmd.Dir = dir

	env := os.Environ()
	hasTerm := false
	for i, e := range env {
		if strings.HasPrefix(e, "TERM=") {
			env[i] = "TERM=xterm-256color"
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		env = append(env, "TERM=xterm-256color")
	}
	cmd.Env = env
	hidePtyChild(cmd)

	if err := cmd.Start(); err != nil {
		p.Close()
		return fmt.Errorf("pty start: %w", err)
	}

	sess := &ptySession{
		id:   tabId,
		pty:  p,
		cmd:  cmd,
		done: make(chan struct{}),
	}
	a.ptyMgr.sessions[tabId] = sess

	go a.ptyReadLoop(sess)
	go a.ptyWaitLoop(sess)

	return nil
}

func (a *App) ptyReadLoop(s *ptySession) {
	buf := make([]byte, 8192)
	eventName := "pty:output:" + s.id
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			wruntime.EventsEmit(a.ctx, eventName, string(buf[:n]))
		}
		if err != nil {
			if err != io.EOF {
				// surface read errors as event
				wruntime.EventsEmit(a.ctx, "pty:error:"+s.id, err.Error())
			}
			return
		}
	}
}

func (a *App) ptyWaitLoop(s *ptySession) {
	if s.cmd != nil {
		s.cmd.Wait()
	}
	// Cancel any pending jamo flush timer so it can't fire against a
	// closed pty after this session is torn down.
	s.jamoMu.Lock()
	if s.jamoTimer != nil {
		s.jamoTimer.Stop()
		s.jamoTimer = nil
	}
	s.jamoBuf = s.jamoBuf[:0]
	s.jamoMu.Unlock()
	a.ptyMgr.mu.Lock()
	delete(a.ptyMgr.sessions, s.id)
	a.ptyMgr.mu.Unlock()
	if s.pty != nil {
		s.pty.Close()
	}
	close(s.done)
	wruntime.EventsEmit(a.ctx, "pty:exit:"+s.id)
}

// Idle window after which an in-progress Hangul syllable is flushed
// even though it could still be extended. Kept short so the user does
// not feel typing latency. Completed syllables flush instantly through
// the streaming composer; this timer only applies to the trailing
// syllable that has not yet been disambiguated by a following keystroke.
const jamoFlushMS = 60

// isHangulJamo reports whether r is a Hangul jamo that should be
// coalesced and composed. Includes:
//   - Conjoining Jamo (U+1100..U+11FF) — composed by NFC
//   - Jamo Extended-A / -B (U+A960..U+A97F, U+D7B0..U+D7FF)
//   - Compatibility Jamo (U+3131..U+318E) — NOT composable by NFC, but
//     decomposable by NFKC into conjoining jamo which then NFC-compose.
//     macOS WKWebView sometimes hands us this range when forwarding
//     keystrokes from the system Korean IME via the textarea path.
func isHangulJamo(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x11FF:
		return true
	case r >= 0xA960 && r <= 0xA97F:
		return true
	case r >= 0xD7B0 && r <= 0xD7FF:
		return true
	case r >= 0x3131 && r <= 0x318E:
		return true
	}
	return false
}

// splitTrailingJamo returns (head, tail) where tail is the maximal
// suffix of b composed only of Hangul jamo runes. The tail is the part
// we hold back, expecting more jamo runes to follow so the whole run
// can NFC-compose into syllables.
func splitTrailingJamo(b []byte) (head, tail []byte) {
	i := len(b)
	for i > 0 {
		r, size := utf8.DecodeLastRune(b[:i])
		if r == utf8.RuneError || !isHangulJamo(r) {
			break
		}
		i -= size
	}
	return b[:i], b[i:]
}

// flushJamoLocked drains the entire buffer as a best-effort composed
// string. Use only on session teardown / non-jamo arrival / timer
// expiry. Caller must hold s.jamoMu.
func (s *ptySession) flushJamoLocked() {
	if len(s.jamoBuf) == 0 {
		return
	}
	out := composeJamo(string(s.jamoBuf))
	s.jamoBuf = s.jamoBuf[:0]
	if s.jamoTimer != nil {
		s.jamoTimer.Stop()
		s.jamoTimer = nil
	}
	io.WriteString(s.pty, out)
}

// streamFlushLocked emits every syllable the streaming composer can
// prove is final, keeping the trailing in-progress syllable in the
// buffer. Caller must hold s.jamoMu.
func (s *ptySession) streamFlushLocked() {
	if len(s.jamoBuf) == 0 {
		return
	}
	composed, pending := composeHangulJamoStreaming(string(s.jamoBuf))
	if composed != "" {
		io.WriteString(s.pty, composed)
	}
	s.jamoBuf = append(s.jamoBuf[:0], []byte(pending)...)
}

// WritePty sends input bytes to the PTY's stdin. Hangul jamo are
// accumulated in a per-session buffer and emitted as composed syllables
// the moment the streaming composer can prove a syllable is final
// (next consonant arrived). The trailing half-syllable is held for at
// most jamoFlushMS so a user pausing on the last keystroke still sees
// it appear without waiting for the next character.
func (a *App) WritePty(tabId string, data string) error {
	a.ptyMgr.mu.Lock()
	s, ok := a.ptyMgr.sessions[tabId]
	a.ptyMgr.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %s not found", tabId)
	}

	payload := []byte(nfc(data))
	head, tail := splitTrailingJamo(payload)

	s.jamoMu.Lock()
	defer s.jamoMu.Unlock()

	if len(head) > 0 {
		// Non-jamo bytes finalise the trailing syllable. Flush
		// everything in the buffer (as composed text) then write head
		// immediately so control chars / Enter keep zero latency.
		if len(s.jamoBuf) > 0 {
			out := composeJamo(string(s.jamoBuf))
			s.jamoBuf = s.jamoBuf[:0]
			if s.jamoTimer != nil {
				s.jamoTimer.Stop()
				s.jamoTimer = nil
			}
			if _, err := io.WriteString(s.pty, out); err != nil {
				return err
			}
		}
		if _, err := s.pty.Write(head); err != nil {
			return err
		}
	}

	if len(tail) > 0 {
		s.jamoBuf = append(s.jamoBuf, tail...)
		// Flush any syllable the composer is already certain of; only
		// the trailing in-progress syllable remains in the buffer.
		s.streamFlushLocked()
		if s.jamoTimer != nil {
			s.jamoTimer.Stop()
		}
		if len(s.jamoBuf) > 0 {
			s.jamoTimer = time.AfterFunc(jamoFlushMS*time.Millisecond, func() {
				s.jamoMu.Lock()
				defer s.jamoMu.Unlock()
				s.flushJamoLocked()
			})
		}
	}
	return nil
}

// ResizePty informs the PTY about the new viewport size.
func (a *App) ResizePty(tabId string, cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid size %dx%d", cols, rows)
	}
	a.ptyMgr.mu.Lock()
	s, ok := a.ptyMgr.sessions[tabId]
	a.ptyMgr.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %s not found", tabId)
	}
	return s.pty.Resize(cols, rows)
}

// KillPty terminates the PTY process.
func (a *App) KillPty(tabId string) error {
	a.ptyMgr.mu.Lock()
	s, ok := a.ptyMgr.sessions[tabId]
	a.ptyMgr.mu.Unlock()
	if !ok {
		return nil
	}
	s.jamoMu.Lock()
	s.flushJamoLocked()
	s.jamoMu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

var pathAugmented bool

func augmentPATH() {
	if pathAugmented {
		return
	}
	pathAugmented = true

	home, _ := os.UserHomeDir()
	current := os.Getenv("PATH")
	candidates := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		home + "/.local/bin",
		home + "/.npm-global/bin",
		home + "/.volta/bin",
		home + "/.fnm/aliases/default/bin",
		home + "/.bun/bin",
		home + "/.cargo/bin",
	}
	// add nvm node bins
	nvmDir := home + "/.nvm/versions/node"
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, nvmDir+"/"+e.Name()+"/bin")
			}
		}
	}
	parts := []string{current}
	have := map[string]bool{}
	for _, p := range strings.Split(current, string(os.PathListSeparator)) {
		have[p] = true
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil && !have[c] {
			parts = append(parts, c)
			have[c] = true
		}
	}
	os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}

func resolveViaShell(command string) string {
	if runtime.GOOS == "windows" {
		return ""
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	out, err := exec.Command(shell, "-lic", "command -v "+command).Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return ""
	}
	return p
}

// ListPtyTabs returns the IDs of all running PTY tabs.
func (a *App) ListPtyTabs() []string {
	a.ptyMgr.mu.Lock()
	defer a.ptyMgr.mu.Unlock()
	out := make([]string, 0, len(a.ptyMgr.sessions))
	for id := range a.ptyMgr.sessions {
		out = append(out, id)
	}
	return out
}
