package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// App struct
type App struct {
	ctx    context.Context
	ptyMgr *PtyManager
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

	out, err := exec.Command(cmd, "-p", prompt, "--dangerously-skip-permissions").Output()
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
