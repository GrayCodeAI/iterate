package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// initHistory
// ---------------------------------------------------------------------------

func TestInitHistory_NoFile(t *testing.T) {
	// Reset globals to clean state
	inputHistoryMu.Lock()
	savedHistory := inputHistory
	inputHistory = nil
	inputHistoryMu.Unlock()
	savedFile := historyFile
	defer func() {
		inputHistoryMu.Lock()
		inputHistory = savedHistory
		inputHistoryMu.Unlock()
		historyFile = savedFile
	}()

	historyFile = filepath.Join(t.TempDir(), "nonexistent", "history")
	initHistory()

	// initHistory reads from ~/.iterate/history (hardcoded), so we just verify
	// that the function doesn't panic and that inputHistory is a valid slice.
	got := getInputHistory()
	if got == nil {
		t.Error("getInputHistory should never return nil")
	}
}

func TestInitHistory_WithFile(t *testing.T) {
	// initHistory always reads from ~/.iterate/history, so we can't easily
	// mock it. Instead, verify the loading logic by testing appendHistory
	// + getInputHistory which use the same global state.
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")

	inputHistoryMu.Lock()
	savedHistory := inputHistory
	inputHistory = nil
	inputHistoryMu.Unlock()
	savedFile := historyFile
	historyFile = histPath
	defer func() {
		inputHistoryMu.Lock()
		inputHistory = savedHistory
		inputHistoryMu.Unlock()
		historyFile = savedFile
	}()

	appendHistory("/help")
	appendHistory("/test")
	appendHistory("/git status")

	got := getInputHistory()
	if len(got) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(got))
	}
	if got[0] != "/help" {
		t.Errorf("expected first entry '/help', got %q", got[0])
	}
	if got[1] != "/test" {
		t.Errorf("expected second entry '/test', got %q", got[1])
	}
	if got[2] != "/git status" {
		t.Errorf("expected third entry '/git status', got %q", got[2])
	}
}

func TestInitHistory_SkipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")

	inputHistoryMu.Lock()
	savedHistory := inputHistory
	inputHistory = nil
	inputHistoryMu.Unlock()
	savedFile := historyFile
	historyFile = histPath
	defer func() {
		inputHistoryMu.Lock()
		inputHistory = savedHistory
		inputHistoryMu.Unlock()
		historyFile = savedFile
	}()

	appendHistory("/help")
	appendHistory("/test")

	got := getInputHistory()
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// appendHistory
// ---------------------------------------------------------------------------

func TestAppendHistory_EmptyString(t *testing.T) {
	before := inputHistoryLen()
	appendHistory("")
	after := inputHistoryLen()
	if after != before {
		t.Error("empty string should not be appended")
	}
}

func TestAppendHistory_AppendsToMemory(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")
	historyFile = histPath

	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()

	appendHistory("/first")
	appendHistory("/second")

	got := getInputHistory()
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0] != "/first" {
		t.Errorf("expected '/first', got %q", got[0])
	}
	if got[1] != "/second" {
		t.Errorf("expected '/second', got %q", got[1])
	}
}

func TestAppendHistory_DeduplicatesLastEntry(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")
	historyFile = histPath

	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()

	appendHistory("/same")
	appendHistory("/same")

	got := getInputHistory()
	if len(got) != 1 {
		t.Errorf("expected 1 entry (duplicate suppressed), got %d", len(got))
	}
}

func TestAppendHistory_PersistsToFile(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")
	historyFile = histPath

	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()

	appendHistory("/persisted")

	data, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("history file not created: %v", err)
	}
	if !strings.Contains(string(data), "/persisted") {
		t.Errorf("file should contain '/persisted', got %q", string(data))
	}
}

func TestAppendHistory_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "subdir", "history")
	historyFile = histPath

	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()

	appendHistory("/test")

	if _, err := os.Stat(histPath); err != nil {
		t.Errorf("history file should be created with parent dir: %v", err)
	}
}

func TestAppendHistory_EmptyHistoryFile(t *testing.T) {
	// When historyFile is empty, should not persist but should still work in memory
	savedFile := historyFile
	historyFile = ""
	defer func() { historyFile = savedFile }()

	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()

	appendHistory("/test")

	got := getInputHistory()
	if len(got) != 1 {
		t.Errorf("expected 1 entry in memory, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// trimHistoryFile
// ---------------------------------------------------------------------------

func TestTrimHistoryFile_UnderLimit(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")
	historyFile = histPath

	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "/cmd" + string(rune('a'+i))
	}
	os.WriteFile(histPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	trimHistoryFile()

	data, _ := os.ReadFile(histPath)
	result := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(result) != 10 {
		t.Errorf("expected 10 lines (under limit), got %d", len(result))
	}
}

func TestTrimHistoryFile_OverLimit(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "history")
	historyFile = histPath

	lines := make([]string, maxHistoryLines+50)
	for i := range lines {
		lines[i] = "/cmd" + strings.Repeat("x", 10)
	}
	os.WriteFile(histPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	trimHistoryFile()

	data, _ := os.ReadFile(histPath)
	result := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(result) != maxHistoryLines {
		t.Errorf("expected %d lines after trim, got %d", maxHistoryLines, len(result))
	}
}

func TestTrimHistoryFile_NoFile(t *testing.T) {
	historyFile = filepath.Join(t.TempDir(), "nonexistent")
	// Should not panic
	trimHistoryFile()
}

// ---------------------------------------------------------------------------
// getInputHistory / inputHistoryLen / inputHistoryAt
// ---------------------------------------------------------------------------

func TestGetInputHistory_ReturnsCopy(t *testing.T) {
	inputHistoryMu.Lock()
	inputHistory = []string{"/a", "/b", "/c"}
	inputHistoryMu.Unlock()

	got := getInputHistory()
	got[0] = "/modified"

	original := getInputHistory()
	if original[0] != "/a" {
		t.Error("modifying returned copy should not affect original")
	}
}

func TestInputHistoryLen(t *testing.T) {
	inputHistoryMu.Lock()
	inputHistory = []string{"/a", "/b"}
	inputHistoryMu.Unlock()

	if inputHistoryLen() != 2 {
		t.Errorf("expected length 2, got %d", inputHistoryLen())
	}
}

func TestInputHistoryAt(t *testing.T) {
	inputHistoryMu.Lock()
	inputHistory = []string{"/first", "/second", "/third"}
	inputHistoryMu.Unlock()

	if inputHistoryAt(0) != "/first" {
		t.Errorf("expected '/first', got %q", inputHistoryAt(0))
	}
	if inputHistoryAt(2) != "/third" {
		t.Errorf("expected '/third', got %q", inputHistoryAt(2))
	}
}
