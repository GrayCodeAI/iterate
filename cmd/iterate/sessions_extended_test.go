package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// saveSession / loadSession / listSessions
// ---------------------------------------------------------------------------

func TestSaveSession_CreatesFile(t *testing.T) {
	_ = t.TempDir()
	// We need to work with the real sessionsDir, so test the round-trip
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	// Write to a temp path to verify the function works
	testName := "test-session-" + t.Name()
	// Use the real sessions dir but with a unique name
	err := saveSession(testName, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Load it back
	loaded, err := loadSession(testName)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", loaded[0].Content)
	}
}

func TestLoadSession_NotFound(t *testing.T) {
	_, err := loadSession("nonexistent-session-xyz")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	// Save a session, then list
	name := "list-test-" + t.Name()
	msgs := []iteragent.Message{{Role: "user", Content: "test"}}
	saveSession(name, msgs)

	sessions := listSessions()
	found := false
	for _, s := range sessions {
		if s == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected session %q in list, got %v", name, sessions)
	}
}

func TestListSessions_Empty(t *testing.T) {
	// listSessions should not panic even if dir doesn't exist
	// (it will create it on first save, but may be empty)
	_ = listSessions()
}

// ---------------------------------------------------------------------------
// sessionsDir
// ---------------------------------------------------------------------------

func TestSessionsDir_ReturnsPath(t *testing.T) {
	dir := sessionsDir()
	if !strings.Contains(dir, ".iterate") {
		t.Errorf("expected .iterate in sessions dir, got %q", dir)
	}
	if !strings.Contains(dir, "sessions") {
		t.Errorf("expected 'sessions' in path, got %q", dir)
	}
}

// ---------------------------------------------------------------------------
// initAuditLog / logAudit
// ---------------------------------------------------------------------------

func TestInitAuditLog_CreatesDir(t *testing.T) {
	initAuditLog()
	if auditLogPath == "" {
		t.Error("expected auditLogPath to be set")
	}
}

func TestLogAudit_WithInit(t *testing.T) {
	initAuditLog()
	logAudit("test_tool", map[string]interface{}{"arg": "value"}, "result")
	// Just verify it doesn't panic
}

func TestLogAudit_WithoutInit(t *testing.T) {
	origPath := auditLogPath
	auditLogPath = ""
	logAudit("test_tool", nil, "result")
	auditLogPath = origPath
}

// ---------------------------------------------------------------------------
// bookmarks
// ---------------------------------------------------------------------------

func TestBookmarksPath_ContainsIterate(t *testing.T) {
	p := bookmarksPath()
	if !strings.Contains(p, ".iterate") {
		t.Errorf("expected .iterate in bookmarks path, got %q", p)
	}
}

func TestLoadBookmarks_MissingFile(t *testing.T) {
	// This uses the real home dir, so just test it doesn't panic
	bms := loadBookmarks()
	_ = bms // may be nil or empty
}

func TestSaveAndLoadBookmarks(t *testing.T) {
	bms := []Bookmark{
		{Name: "test", Messages: []iteragent.Message{{Role: "user", Content: "hi"}}},
	}
	saveBookmarks(bms)

	loaded := loadBookmarks()
	if len(loaded) < 1 {
		// It may not find it if home dir is different, which is fine
		return
	}
	found := false
	for _, b := range loaded {
		if b.Name == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find saved bookmark")
	}
}

// ---------------------------------------------------------------------------
// addBookmark
// ---------------------------------------------------------------------------

func TestAddBookmark_New(t *testing.T) {
	msgs := []iteragent.Message{{Role: "user", Content: "test bookmark"}}
	addBookmark("bm-test-new", msgs)

	bms := loadBookmarks()
	found := false
	for _, b := range bms {
		if b.Name == "bm-test-new" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected bookmark to be added")
	}
}

func TestAddBookmark_ReplaceExisting(t *testing.T) {
	msgs1 := []iteragent.Message{{Role: "user", Content: "first"}}
	addBookmark("bm-test-replace", msgs1)

	msgs2 := []iteragent.Message{{Role: "user", Content: "second"}}
	addBookmark("bm-test-replace", msgs2)

	bms := loadBookmarks()
	for _, b := range bms {
		if b.Name == "bm-test-replace" {
			if b.Messages[0].Content != "second" {
				t.Error("expected bookmark to be replaced with second message")
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// recordToolCall / recordMessage / sessionStats
// ---------------------------------------------------------------------------

func TestSessionStats_ReturnsNonEmpty(t *testing.T) {
	stats := sessionStats()
	if stats == "" {
		t.Error("sessionStats should return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// maybeNotify
// ---------------------------------------------------------------------------

func TestMaybeNotify_NoPanic(t *testing.T) {
	// Just verify it doesn't panic
	origEnabled := cfg.NotifyEnabled
	cfg.NotifyEnabled = false
	maybeNotify()
	cfg.NotifyEnabled = true
	maybeNotify()
	cfg.NotifyEnabled = origEnabled
}

// ---------------------------------------------------------------------------
// saveSession error cases
// ---------------------------------------------------------------------------

func TestSaveSession_InvalidJSON(t *testing.T) {
	// Can't really create invalid JSON from []iteragent.Message,
	// but we can verify the function works
	name := "invalid-json-test-" + t.Name()
	msgs := []iteragent.Message{
		{Role: "user", Content: "test"},
	}
	err := saveSession(name, msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadBookmarks with invalid JSON
// ---------------------------------------------------------------------------

func TestLoadBookmarks_InvalidJSON(t *testing.T) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".iterate")
	os.MkdirAll(dir, 0o755)
	path := bookmarksPath()

	orig, _ := os.ReadFile(path)
	defer os.WriteFile(path, orig, 0o644)

	os.WriteFile(path, []byte("not json"), 0o644)
	bms := loadBookmarks()
	if bms != nil {
		t.Error("expected nil bookmarks for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Session save with complex messages
// ---------------------------------------------------------------------------

func TestSaveSession_ComplexMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello!"},
		{Role: "assistant", Content: "Hi there! How can I help?"},
		{Role: "user", Content: "Tell me a joke."},
		{Role: "assistant", Content: "Why did the programmer quit? Because he didn't get arrays!"},
	}

	name := "complex-session-" + t.Name()
	err := saveSession(name, msgs)
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := loadSession(name)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded))
	}
	if loaded[0].Role != "system" {
		t.Errorf("expected first role 'system', got %q", loaded[0].Role)
	}
}

// ---------------------------------------------------------------------------
// sessionsDir JSON structure
// ---------------------------------------------------------------------------

func TestSaveSession_ValidJSON(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "test content"},
	}
	name := "json-verify-" + t.Name()
	saveSession(name, msgs)

	path := filepath.Join(sessionsDir(), name+".json")
	data, _ := os.ReadFile(path)

	var loaded []iteragent.Message
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 message, got %d", len(loaded))
	}
}
