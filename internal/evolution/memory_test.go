package evolution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"
)

func TestAppendLearningJSONL_Success(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	// Write DAY_COUNT file
	_ = os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("5"), 0644)

	err := e.appendLearningJSONL("Test Lesson", "evolution", "test context", "test takeaway")
	if err != nil {
		t.Fatalf("appendLearningJSONL failed: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatalf("failed to read learnings.jsonl: %v", err)
	}

	if !strings.Contains(string(content), "Test Lesson") {
		t.Errorf("expected content to contain 'Test Lesson', got: %s", string(content))
	}

	if !strings.Contains(string(content), "test takeaway") {
		t.Errorf("expected content to contain 'test takeaway', got: %s", string(content))
	}
}

func TestAppendFailureJSONL_Success(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	// Write DAY_COUNT file
	_ = os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("3"), 0644)

	err := e.appendFailureJSONL("Test Task", "Test failure reason")
	if err != nil {
		t.Fatalf("appendFailureJSONL failed: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(dir, "memory", "failures.jsonl"))
	if err != nil {
		t.Fatalf("failed to read failures.jsonl: %v", err)
	}

	if !strings.Contains(string(content), "Test Task") {
		t.Errorf("expected content to contain 'Test Task', got: %s", string(content))
	}

	if !strings.Contains(string(content), "Test failure reason") {
		t.Errorf("expected content to contain failure reason, got: %s", string(content))
	}
}

func TestAppendLearningJSONL_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	_ = os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("1"), 0644)

	// Ensure memory directory doesn't exist
	_ = os.RemoveAll(filepath.Join(dir, "memory"))

	err := e.appendLearningJSONL("Test", "evolution", "", "")
	if err != nil {
		t.Fatalf("appendLearningJSONL failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Join(dir, "memory")); os.IsNotExist(err) {
		t.Error("memory directory should have been created")
	}
}

func TestAppendLearningJSONL_DefaultsDayToZero(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	// No DAY_COUNT file - should default to 0

	err := e.appendLearningJSONL("Test", "evolution", "", "")
	if err != nil {
		t.Fatalf("appendLearningJSONL failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if !strings.Contains(string(content), `"day":0`) {
		t.Errorf("expected day to default to 0, got: %s", string(content))
	}
}

func TestAppendLearningJSONL_HandlesInvalidDayCount(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	_ = os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("invalid"), 0644)

	err := e.appendLearningJSONL("Test", "evolution", "", "")
	if err != nil {
		t.Fatalf("appendLearningJSONL should handle invalid DAY_COUNT: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if !strings.Contains(string(content), `"day":0`) {
		t.Errorf("expected day to default to 0 with invalid DAY_COUNT, got: %s", string(content))
	}
}

func TestRecentFailures_LimitsEntries(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	_ = os.MkdirAll(memDir, 0755)

	// Create multiple entries
	var entries []string
	for i := 0; i < 10; i++ {
		entry := `{"day":1,"ts":"2099-01-0"` + string(rune('1'+i)) + `T00:00:00Z","task":"task ` + string(rune('0'+i)) + `","reason":"reason"}`
		entries = append(entries, entry)
	}
	content := strings.Join(entries, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(memDir, "failures.jsonl"), []byte(content), 0644)

	result := recentFailures(dir, 5)
	count := strings.Count(result, "task")
	if count > 5 {
		t.Errorf("expected at most 5 tasks in output, got %d", count)
	}
}

func TestRecentFailures_NoFile(t *testing.T) {
	dir := t.TempDir()
	result := recentFailures(dir, 5)
	if result != "" {
		t.Error("expected empty string when no failures file exists")
	}
}

func TestRecentFailures_SkipsOldEntries(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	_ = os.MkdirAll(memDir, 0755)

	// Create entries - one old, one new
	oldEntry := `{"day":1,"ts":"2020-01-01T00:00:00Z","task":"old task","reason":"old"}`
	newEntry := `{"day":2,"ts":"2099-01-01T00:00:00Z","task":"new task","reason":"new"}`
	content := oldEntry + "\n" + newEntry + "\n"
	_ = os.WriteFile(filepath.Join(memDir, "failures.jsonl"), []byte(content), 0644)

	result := recentFailures(dir, 5)
	if strings.Contains(result, "old task") {
		t.Error("old entry should have been skipped")
	}
}
