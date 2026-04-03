package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Per-project structured memory — .iterate/memory.json
//
// This is distinct from the evolution memory in memory/learnings.jsonl.
// It stores short notes that persist across REPL sessions for a given project
// and are injected into the agent's system prompt.
// ---------------------------------------------------------------------------

type projectMemoryEntry struct {
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type projectMemory struct {
	Entries []projectMemoryEntry `json:"entries"`
}

func projectMemoryPath(repoPath string) string {
	return filepath.Join(repoPath, ".iterate", "memory.json")
}

func loadProjectMemory(repoPath string) projectMemory {
	data, err := os.ReadFile(projectMemoryPath(repoPath))
	if err != nil {
		return projectMemory{}
	}
	var m projectMemory
	if err := json.Unmarshal(data, &m); err != nil {
		return projectMemory{}
	}
	return m
}

func saveProjectMemory(repoPath string, m projectMemory) error {
	path := projectMemoryPath(repoPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o644)
}

func addProjectMemoryNote(repoPath, note string) error {
	m := loadProjectMemory(repoPath)
	m.Entries = append(m.Entries, projectMemoryEntry{
		Note:      note,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	return saveProjectMemory(repoPath, m)
}

func removeProjectMemoryEntry(repoPath string, idx int) (projectMemoryEntry, bool) {
	m := loadProjectMemory(repoPath)
	if idx < 0 || idx >= len(m.Entries) {
		return projectMemoryEntry{}, false
	}
	entry := m.Entries[idx]
	m.Entries = append(m.Entries[:idx], m.Entries[idx+1:]...)
	if err := saveProjectMemory(repoPath, m); err != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to save project memory after removal: %v\n", err)
	}
	return entry, true
}

// formatProjectMemoryForPrompt returns a compact string for injection into the system prompt.
func formatProjectMemoryForPrompt(m projectMemory) string {
	if len(m.Entries) == 0 {
		return ""
	}
	var lines []string
	for _, e := range m.Entries {
		lines = append(lines, "- "+e.Note)
	}
	return "## Project Notes\n\n" + strings.Join(lines, "\n") + "\n"
}

// ---------------------------------------------------------------------------
// Active learnings reader
// ---------------------------------------------------------------------------

func readActiveLearnings(repoPath string) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))
	if err != nil {
		raw, err2 := os.ReadFile(filepath.Join(repoPath, "memory", "learnings.jsonl"))
		if err2 != nil {
			return ""
		}
		trimmed := strings.TrimSpace(string(raw))
		if trimmed == "" {
			return ""
		}
		lines := strings.Split(trimmed, "\n")
		if len(lines) > 10 {
			lines = lines[len(lines)-10:]
		}
		return strings.Join(lines, "\n")
	}
	return string(data)
}
