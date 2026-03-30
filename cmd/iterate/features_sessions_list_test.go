package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListSessionsErrorHandling(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test with non-existent directory path
	// We can't easily inject a bad path, so we verify the current behavior
	// handles empty directory gracefully
	sessions := listSessions()
	// Should return empty slice (not nil) when dir doesn't exist or is empty
	if sessions == nil {
		t.Error("listSessions returned nil instead of empty slice")
	}

	// Create a session file and verify it's listed
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Initialize with empty sessions dir
	dir := sessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	// Create a test session file
	testSession := `[{"role":"user","content":"test"}]`
	testPath := filepath.Join(dir, "testsession.json")
	if err := os.WriteFile(testPath, []byte(testSession), 0o644); err != nil {
		t.Fatalf("failed to write test session: %v", err)
	}

	// Should list the session
	sessions = listSessions()
	found := false
	for _, s := range sessions {
		if s == "testsession" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'testsession' in %v", sessions)
	}
}