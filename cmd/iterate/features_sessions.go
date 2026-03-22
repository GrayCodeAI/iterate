package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// Audit log
// ---------------------------------------------------------------------------

var auditLogPath string

func initAuditLog() {
	home, _ := os.UserHomeDir()
	auditLogPath = filepath.Join(home, ".iterate", "audit.log")
	_ = os.MkdirAll(filepath.Dir(auditLogPath), 0o755) // best-effort cleanup
}

func logAudit(toolName string, args map[string]interface{}, result string) {
	if auditLogPath == "" {
		return
	}
	f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	argBytes, _ := json.Marshal(args)
	snippet := result
	if len(snippet) > 120 {
		snippet = snippet[:120] + "…"
	}
	fmt.Fprintf(f, "[%s] %s %s → %s\n", time.Now().Format("2006-01-02 15:04:05"), toolName, argBytes, snippet)
}

// ---------------------------------------------------------------------------
// Session save / load (exposed to REPL)
// ---------------------------------------------------------------------------

func sessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "sessions")
}

func saveSession(name string, messages []iteragent.Message) error {
	dir := sessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}
	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadSession(name string) ([]iteragent.Message, error) {
	path := filepath.Join(sessionsDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var msgs []iteragent.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func listSessions() []string {
	entries, _ := os.ReadDir(sessionsDir())
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// Bookmarks
// ---------------------------------------------------------------------------

type Bookmark struct {
	Name      string              `json:"name"`
	CreatedAt time.Time           `json:"created_at"`
	Messages  []iteragent.Message `json:"messages"`
}

func bookmarksPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "bookmarks.json")
}

func loadBookmarks() []Bookmark {
	data, err := os.ReadFile(bookmarksPath())
	if err != nil {
		return nil
	}
	var bms []Bookmark
	if err := json.Unmarshal(data, &bms); err != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to parse bookmarks: %v\n", err)
		return nil
	}
	return bms
}

func saveBookmarks(bms []Bookmark) {
	data, _ := json.MarshalIndent(bms, "", "  ")
	if err := os.WriteFile(bookmarksPath(), data, 0o644); err != nil {
		slog.Warn("failed to write bookmarks file", "err", err)
	}
}

func addBookmark(name string, messages []iteragent.Message) {
	bms := loadBookmarks()
	// Replace if name exists
	for i, b := range bms {
		if b.Name == name {
			bms[i] = Bookmark{Name: name, CreatedAt: time.Now(), Messages: messages}
			saveBookmarks(bms)
			return
		}
	}
	bms = append(bms, Bookmark{Name: name, CreatedAt: time.Now(), Messages: messages})
	saveBookmarks(bms)
}

// ---------------------------------------------------------------------------
// /stats — session statistics
// ---------------------------------------------------------------------------

// recordToolCall and recordMessage are kept as thin wrappers for backward compatibility.
func recordToolCall() { sess.ToolCalls++ }
func recordMessage()  { sess.Messages++ }

func sessionStats() string {
	return sess.Stats()
}

// ---------------------------------------------------------------------------
// /notify — ring terminal bell when agent finishes
// ---------------------------------------------------------------------------

func maybeNotify() {
	if cfg.NotifyEnabled {
		fmt.Print("\a") // terminal bell
	}
}

// ---------------------------------------------------------------------------
// /debug — toggle debug logging
// ---------------------------------------------------------------------------
