package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hideDuration = 3 * 24 * time.Hour // 3 days
const seenThreshold = 2

// HistoryEntry tracks how many times a tool has appeared in results.
type HistoryEntry struct {
	Seen        int       `json:"seen"`
	LastSeen    time.Time `json:"last_seen"`
	HiddenUntil time.Time `json:"hidden_until,omitempty"`
}

// ToolHistory holds the seen-history for all tools.
type ToolHistory struct {
	Tools map[string]*HistoryEntry `json:"tools"`
}

// historyPath returns ~/.config/newtools/history.json.
func historyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "newtools", "history.json"), nil
}

// loadHistory reads history from disk. Returns an empty history if the file
// doesn't exist yet. Expired entries are reset on load.
func loadHistory() *ToolHistory {
	h := &ToolHistory{Tools: make(map[string]*HistoryEntry)}

	path, err := historyPath()
	if err != nil {
		return h
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}

	if err := json.Unmarshal(data, h); err != nil {
		// Corrupt file — start fresh
		h.Tools = make(map[string]*HistoryEntry)
		return h
	}

	if h.Tools == nil {
		h.Tools = make(map[string]*HistoryEntry)
	}

	// Reset expired entries
	now := time.Now()
	for _, entry := range h.Tools {
		if !entry.HiddenUntil.IsZero() && now.After(entry.HiddenUntil) {
			entry.Seen = 0
			entry.HiddenUntil = time.Time{}
		}
	}

	return h
}

// saveHistory writes the history to disk, creating the directory if needed.
func (h *ToolHistory) saveHistory() error {
	path, err := historyPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsHidden returns true if the tool should be filtered from results.
func (h *ToolHistory) IsHidden(name string) bool {
	entry, ok := h.Tools[strings.ToLower(name)]
	if !ok {
		return false
	}
	return !entry.HiddenUntil.IsZero() && time.Now().Before(entry.HiddenUntil)
}

// RecordSeen increments the seen count for each displayed tool name.
// When a tool reaches the threshold, hidden_until is set.
func (h *ToolHistory) RecordSeen(names []string) {
	now := time.Now()
	for _, name := range names {
		key := strings.ToLower(name)
		entry, ok := h.Tools[key]
		if !ok {
			entry = &HistoryEntry{}
			h.Tools[key] = entry
		}
		entry.Seen++
		entry.LastSeen = now
		if entry.Seen >= seenThreshold {
			entry.HiddenUntil = now.Add(hideDuration)
		}
	}
}

// resetHistory deletes the history file.
func resetHistory() error {
	path, err := historyPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
