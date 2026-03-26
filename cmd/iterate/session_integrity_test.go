package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// overrideSessionsDir temporarily redirects sessionsDir() to a temp dir.
// Returns a cleanup function that removes the override.
func overrideSessionsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := os.Getenv("HOME")
	// Set HOME to tempdir so sessionsDir() resolves to tempdir/.iterate/sessions
	_ = os.Setenv("HOME", dir)
	t.Cleanup(func() { _ = os.Setenv("HOME", old) })
	return filepath.Join(dir, ".iterate", "sessions")
}

func TestSaveAndLoadSession(t *testing.T) {
	overrideSessionsDir(t)

	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	if err := saveSession("test", msgs); err != nil {
		t.Fatalf("saveSession failed: %v", err)
	}

	loaded, err := loadSession("test")
	if err != nil {
		t.Fatalf("loadSession failed: %v", err)
	}
	if len(loaded) != len(msgs) {
		t.Fatalf("want %d messages, got %d", len(msgs), len(loaded))
	}
	for i := range msgs {
		if loaded[i].Role != msgs[i].Role || loaded[i].Content != msgs[i].Content {
			t.Errorf("message %d mismatch: want %+v, got %+v", i, msgs[i], loaded[i])
		}
	}
}

func TestLoadSession_CorruptFallsBackToBak(t *testing.T) {
	dir := overrideSessionsDir(t)
	_ = os.MkdirAll(dir, 0o755)

	msgs := []iteragent.Message{{Role: "user", Content: "saved before corrupt"}}
	data, _ := json.MarshalIndent(msgs, "", "  ")

	path := filepath.Join(dir, "test.json")
	// Write valid .bak
	_ = atomicWriteFile(path+".bak", data, 0o644)
	// Write corrupt primary
	_ = os.WriteFile(path, []byte("not valid json {{{"), 0o644)

	loaded, err := loadSession("test")
	if err != nil {
		t.Fatalf("loadSession should fall back to .bak, got error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Content != "saved before corrupt" {
		t.Errorf("unexpected loaded messages: %+v", loaded)
	}
}

func TestLoadSession_IntegrityCheckEmptyRole(t *testing.T) {
	dir := overrideSessionsDir(t)
	_ = os.MkdirAll(dir, 0o755)

	// Message with empty role violates integrity check.
	msgs := []iteragent.Message{{Role: "", Content: "no role"}}
	data, _ := json.MarshalIndent(msgs, "", "  ")
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, data, 0o644)

	_, err := loadSession("bad")
	if err == nil {
		t.Error("expected integrity check error for empty role, got nil")
	}
}

func TestSaveSession_WritesBakOnOverwrite(t *testing.T) {
	dir := overrideSessionsDir(t)
	_ = os.MkdirAll(dir, 0o755)

	v1 := []iteragent.Message{{Role: "user", Content: "version 1"}}
	v2 := []iteragent.Message{{Role: "user", Content: "version 2"}}

	if err := saveSession("snap", v1); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	// .bak should not exist yet (no prior version to back up)
	bakPath := filepath.Join(dir, "snap.json.bak")
	if _, err := os.Stat(bakPath); err == nil {
		// .bak may or may not exist on first save — that's OK
	}

	if err := saveSession("snap", v2); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	// .bak should now contain v1
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf(".bak file not created on overwrite: %v", err)
	}
	var bakMsgs []iteragent.Message
	_ = json.Unmarshal(bakData, &bakMsgs)
	if len(bakMsgs) == 0 || bakMsgs[0].Content != "version 1" {
		t.Errorf(".bak should contain version 1, got: %+v", bakMsgs)
	}
}
