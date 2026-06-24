package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	gopty "github.com/aymanbagabas/go-pty"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ptySession struct {
	id   string
	pty  gopty.Pty
	cmd  *gopty.Cmd
	done chan struct{}
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

	cmd := p.Command(resolved, args...)
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
	a.ptyMgr.mu.Lock()
	delete(a.ptyMgr.sessions, s.id)
	a.ptyMgr.mu.Unlock()
	if s.pty != nil {
		s.pty.Close()
	}
	close(s.done)
	wruntime.EventsEmit(a.ctx, "pty:exit:"+s.id)
}

// WritePty sends input bytes to the PTY's stdin.
func (a *App) WritePty(tabId string, data string) error {
	a.ptyMgr.mu.Lock()
	s, ok := a.ptyMgr.sessions[tabId]
	a.ptyMgr.mu.Unlock()
	if !ok {
		return fmt.Errorf("tab %s not found", tabId)
	}
	_, err := io.WriteString(s.pty, data)
	return err
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
