package evolution

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// readDayCount
// ---------------------------------------------------------------------------

func TestReadDayCount_WithFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("42\n"), 0o644)
	e := New(dir, slog.Default())

	count := e.readDayCount()
	if count != "42" {
		t.Errorf("expected '42', got %q", count)
	}
}

func TestReadDayCount_MissingFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	count := e.readDayCount()
	if count != "" {
		t.Errorf("expected empty string for missing file, got %q", count)
	}
}

func TestReadDayCount_Whitespace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("  7  \n"), 0o644)
	e := New(dir, slog.Default())

	count := e.readDayCount()
	if count != "7" {
		t.Errorf("expected '7', got %q", count)
	}
}

// ---------------------------------------------------------------------------
// appendJournal
// ---------------------------------------------------------------------------

func TestAppendJournal_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	now := time.Now()
	result := &RunResult{
		StartedAt:  now,
		FinishedAt: now.Add(5 * time.Second),
		Status:     "committed",
	}

	e.appendJournal(result, "feat: add streaming", "anthropic", true)

	data, err := os.ReadFile(filepath.Join(dir, "docs/docs/JOURNAL.md"))
	if err != nil {
		t.Fatalf("JOURNAL.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "iterate Evolution Journal") {
		t.Error("expected journal header")
	}
	if !strings.Contains(content, "add streaming") {
		t.Error("expected journal title")
	}
}

func TestAppendJournal_WithDayCount(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("5\n"), 0o644)
	e := New(dir, slog.Default())

	now := time.Now()
	result := &RunResult{
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
	}

	e.appendJournal(result, "some output", "test", true)

	data, _ := os.ReadFile(filepath.Join(dir, "docs/docs/JOURNAL.md"))
	if !strings.Contains(string(data), "Day 5") {
		t.Error("expected 'Day 5' in journal")
	}
}

func TestAppendJournal_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "docs/docs/JOURNAL.md"),
		[]byte("# iterate Evolution Journal\n\n## Day 1 — existing entry\n"), 0o644)
	e := New(dir, slog.Default())

	now := time.Now()
	result := &RunResult{
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
	}

	e.appendJournal(result, "feat: new entry", "test", true)

	data, _ := os.ReadFile(filepath.Join(dir, "docs/docs/JOURNAL.md"))
	content := string(data)
	if !strings.Contains(content, "existing entry") {
		t.Error("should preserve existing journal content")
	}
	if !strings.Contains(content, "new entry") {
		t.Error("should add new entry")
	}
}

func TestAppendJournal_FailureStatus(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	now := time.Now()
	result := &RunResult{
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
	}

	e.appendJournal(result, "no changes", "test", false)

	data, _ := os.ReadFile(filepath.Join(dir, "docs/docs/JOURNAL.md"))
	if !strings.Contains(string(data), "no changes committed") {
		t.Error("expected failure title for unsuccessful run")
	}
}
