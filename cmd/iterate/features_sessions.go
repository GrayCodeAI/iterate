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
// Audit log (JSON Lines format)
// ---------------------------------------------------------------------------

var auditLogPath string

// auditEntry is one JSON Lines record written to the audit log.
type auditEntry struct {
	Timestamp string                 `json:"ts"`
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Result    string                 `json:"result,omitempty"`
	IsError   bool                   `json:"error,omitempty"`
}

func initAuditLog() {
	home, _ := os.UserHomeDir()
	auditLogPath = filepath.Join(home, ".iterate", "audit.jsonl")
	_ = os.MkdirAll(filepath.Dir(auditLogPath), 0o755)
}

func logAudit(toolName string, args map[string]interface{}, result string) error {
	if auditLogPath == "" {
		return nil
	}
	f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	snippet := result
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	isErr := strings.HasPrefix(strings.ToLower(strings.TrimSpace(result)), "error")

	entry := auditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tool:      toolName,
		Args:      args,
		Result:    snippet,
		IsError:   isErr,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	if _, err := f.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("write audit newline: %w", err)
	}
	return nil
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
	// Write .bak backup of previous version before overwriting.
	if existing, err := os.ReadFile(path); err == nil {
		_ = atomicWriteFile(path+".bak", existing, 0o644)
	}
	return atomicWriteFile(path, data, 0o644)
}

func loadSession(name string) ([]iteragent.Message, error) {
	path := filepath.Join(sessionsDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var msgs []iteragent.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		// Attempt to restore from .bak file
		bakData, bakErr := os.ReadFile(path + ".bak")
		if bakErr == nil {
			var bakMsgs []iteragent.Message
			if json.Unmarshal(bakData, &bakMsgs) == nil {
				return bakMsgs, nil
			}
		}
		return nil, fmt.Errorf("session file corrupt: %w", err)
	}
	// Integrity check: each message must have a non-empty role.
	for i, m := range msgs {
		if m.Role == "" {
			return nil, fmt.Errorf("session integrity check failed: message %d has empty role", i)
		}
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
	if err := atomicWriteFile(bookmarksPath(), data, 0o644); err != nil {
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

func toggleDebugLogging() bool {
	if _, ok := os.LookupEnv("ITERATE_DEBUG"); ok {
		os.Unsetenv("ITERATE_DEBUG")
		return false
	}
	os.Setenv("ITERATE_DEBUG", "1")
	return true
}

func isDebugLogging() bool {
	_, ok := os.LookupEnv("ITERATE_DEBUG")
	return ok
}

func debugLog(msg string, args ...interface{}) {
	if isDebugLogging() {
		fmt.Fprintf(os.Stderr, "[debug] "+msg+"\n", args...)
	}
}
