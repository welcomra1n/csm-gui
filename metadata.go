package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ── Config ────────────────────────────────────────────────────────────────────

type Config struct {
	ExpiryDays      int    `json:"expiry_days"`
	RefreshInterval int    `json:"refresh_interval"`
	DefaultSort     string `json:"default_sort"`
	DefaultTerminal string `json:"default_terminal"`
	CompactMode     bool   `json:"compact_mode"`
}

func defaultConfig() Config {
	return Config{
		ExpiryDays:      30,
		RefreshInterval: 10,
		DefaultSort:     "date",
		DefaultTerminal: "auto",
		CompactMode:     false,
	}
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "csm-config.json")
}

func loadConfig() Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	if cfg.ExpiryDays <= 0 {
		cfg.ExpiryDays = 30
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 10
	}
	return cfg
}

func saveConfig(cfg Config) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath(), data, 0644)
}

// ── Metadata ──────────────────────────────────────────────────────────────────

type Metadata struct {
	Folders         []string            `json:"folders"`
	SessionFolders  map[string]string   `json:"session_folders"`
	SessionTags     map[string][]string `json:"session_tags"`
	FolderCollapsed map[string]bool     `json:"folder_collapsed"`
	FolderColors    map[string]string   `json:"folder_colors"`
	TempSessions    map[string]bool     `json:"temp_sessions"`
	Recaps          map[string]string   `json:"recaps"`
	Prefs           map[string]bool     `json:"prefs"`
}

func metadataFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "csm-metadata.json")
}

func loadMetadata() *Metadata {
	m := &Metadata{
		Folders:         []string{},
		SessionFolders:  make(map[string]string),
		SessionTags:     make(map[string][]string),
		FolderCollapsed: make(map[string]bool),
		FolderColors:    make(map[string]string),
		TempSessions:    make(map[string]bool),
		Recaps:          make(map[string]string),
		Prefs:           make(map[string]bool),
	}
	data, err := os.ReadFile(metadataFilePath())
	if err != nil {
		return m
	}
	if json.Unmarshal(data, m) != nil {
		return m
	}
	if m.SessionFolders == nil {
		m.SessionFolders = make(map[string]string)
	}
	if m.SessionTags == nil {
		m.SessionTags = make(map[string][]string)
	}
	if m.FolderCollapsed == nil {
		m.FolderCollapsed = make(map[string]bool)
	}
	if m.FolderColors == nil {
		m.FolderColors = make(map[string]string)
	}
	if m.TempSessions == nil {
		m.TempSessions = make(map[string]bool)
	}
	if m.Recaps == nil {
		m.Recaps = make(map[string]string)
	}
	if m.Prefs == nil {
		m.Prefs = make(map[string]bool)
	}
	return m
}

func saveMetadata(m *Metadata) {
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(metadataFilePath(), data, 0644)
}

// ── Aliases ───────────────────────────────────────────────────────────────────

func aliasFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "session-aliases.json")
}

func loadAliases() map[string]string {
	data, err := os.ReadFile(aliasFilePath())
	if err != nil {
		return make(map[string]string)
	}
	var aliases map[string]string
	if json.Unmarshal(data, &aliases) != nil {
		return make(map[string]string)
	}
	return aliases
}

func saveAliases(aliases map[string]string) error {
	data, err := json.MarshalIndent(aliases, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(aliasFilePath(), data, 0644)
}

// ── Project aliases ───────────────────────────────────────────────────────────

func projectAliasFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "project-aliases.json")
}

func loadProjectAliases() map[string]string {
	data, err := os.ReadFile(projectAliasFilePath())
	if err != nil {
		return make(map[string]string)
	}
	var aliases map[string]string
	if json.Unmarshal(data, &aliases) != nil {
		return make(map[string]string)
	}
	return aliases
}

func saveProjectAliases(aliases map[string]string) {
	data, _ := json.MarshalIndent(aliases, "", "  ")
	os.WriteFile(projectAliasFilePath(), data, 0644)
}

// ── Pins ──────────────────────────────────────────────────────────────────────

func pinFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "session-pins.json")
}

func unpinFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "session-unpins.json")
}

func loadUnpins() map[string]bool {
	unpins := make(map[string]bool)
	data, err := os.ReadFile(unpinFilePath())
	if err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for _, id := range ids {
				unpins[id] = true
			}
		}
	}
	return unpins
}

func saveUnpins(unpins map[string]bool) {
	var ids []string
	for id := range unpins {
		ids = append(ids, id)
	}
	data, _ := json.MarshalIndent(ids, "", "  ")
	os.WriteFile(unpinFilePath(), data, 0644)
}

func loadCodexPins() map[string]bool {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".codex", ".codex-global-state.json"))
	if err != nil {
		return nil
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(data, &state) != nil {
		return nil
	}
	raw, ok := state["pinned-thread-ids"]
	if !ok {
		return nil
	}
	var ids []string
	if json.Unmarshal(raw, &ids) != nil {
		return nil
	}
	pins := make(map[string]bool)
	for _, id := range ids {
		pins[id] = true
	}
	return pins
}

func saveCodexPins(pins map[string]bool) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".codex", ".codex-global-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(data, &state) != nil {
		return
	}
	var ids []string
	for id := range pins {
		ids = append(ids, id)
	}
	raw, _ := json.Marshal(ids)
	state["pinned-thread-ids"] = raw
	out, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, out, 0644)
}

func loadPins() map[string]bool {
	pins := make(map[string]bool)
	data, err := os.ReadFile(pinFilePath())
	if err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for _, id := range ids {
				pins[id] = true
			}
		}
	}
	unpins := loadUnpins()
	codexPins := loadCodexPins()
	for id := range codexPins {
		if !unpins[id] {
			pins[id] = true
		}
	}
	return pins
}

func savePins(pins map[string]bool) error {
	var ids []string
	for id := range pins {
		ids = append(ids, id)
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pinFilePath(), data, 0644)
}

// ── Trash / deletion ──────────────────────────────────────────────────────────

func trashDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "session-trash")
}

func deleteSession(s *Session) error {
	trash := trashDir()
	os.MkdirAll(trash, 0755)

	destName := filepath.Base(s.SessionFile)
	destPath := filepath.Join(trash, destName)
	if err := os.Rename(s.SessionFile, destPath); err != nil {
		data, readErr := os.ReadFile(s.SessionFile)
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
			return writeErr
		}
		os.Remove(s.SessionFile)
	}

	meta := map[string]string{
		"originalPath": s.SessionFile,
		"provider":     fmt.Sprintf("%d", s.Provider),
		"id":           s.ID,
		"deletedAt":    fmt.Sprintf("%s", s.ModTime.Format("2006-01-02T15:04:05Z07:00")),
	}
	metaData, _ := json.Marshal(meta)
	os.WriteFile(destPath+".meta", metaData, 0644)

	if s.Provider == ProviderCodex {
		removeFromCodexIndex(s.ID)
	}
	return nil
}

func removeFromCodexIndex(id string) {
	home, _ := os.UserHomeDir()
	indexPath := filepath.Join(home, ".codex", "session_index.jsonl")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}

	var kept []string
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var entry codexIndexEntry
		if json.Unmarshal([]byte(line), &entry) == nil && entry.ID == id {
			continue
		}
		kept = append(kept, line)
	}

	result := ""
	if len(kept) > 0 {
		result = joinLines(kept) + "\n"
	}
	os.WriteFile(indexPath, []byte(result), 0644)
}

// splitLines / joinLines — thin wrappers to avoid strings import in metadata.go
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// ── UUID helper ───────────────────────────────────────────────────────────────

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
