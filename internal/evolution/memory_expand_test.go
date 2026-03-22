package evolution

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLearningJSONL_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	err := e.appendLearningJSONL("test", "source", "context", "takeaway")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	memDir := filepath.Join(dir, "memory")
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		t.Error("memory directory should be created")
	}
}

func TestAppendLearningJSONL_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	err := e.appendLearningJSONL("title", "evolution", "some context", "key takeaway")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["title"] != "title" {
		t.Errorf("expected title 'title', got %v", entry["title"])
	}
	if entry["source"] != "evolution" {
		t.Errorf("expected source 'evolution', got %v", entry["source"])
	}
	if entry["context"] != "some context" {
		t.Errorf("expected context, got %v", entry["context"])
	}
	if entry["takeaway"] != "key takeaway" {
		t.Errorf("expected takeaway, got %v", entry["takeaway"])
	}
	if entry["type"] != "lesson" {
		t.Errorf("expected type 'lesson', got %v", entry["type"])
	}
}

func TestAppendLearningJSONL_WithDayCount(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("42\n"), 0o644)
	e := New(dir, slog.Default())
	e.appendLearningJSONL("title", "source", "ctx", "tk")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	var entry map[string]interface{}
	json.Unmarshal(data, &entry)
	if entry["day"] != float64(42) {
		t.Errorf("expected day 42, got %v", entry["day"])
	}
}

func TestAppendLearningJSONL_MultipleAppends(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	for i := 0; i < 5; i++ {
		if err := e.appendLearningJSONL("entry", "src", "ctx", "tk"); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
}

func TestAppendLearningJSONL_HasTimestamp(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.appendLearningJSONL("t", "s", "c", "tk")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	var entry map[string]interface{}
	json.Unmarshal(data, &entry)
	ts, ok := entry["ts"].(string)
	if !ok || ts == "" {
		t.Error("entry should have a non-empty timestamp")
	}
}

func TestWriteLearningsToMemory_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	if err := e.WriteLearningsToMemory("title", "context", "takeaway"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "memory", "learnings.jsonl")); os.IsNotExist(err) {
		t.Error("learnings.jsonl should exist")
	}
}

func TestWriteLearningsToMemory_CorrectContent(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.WriteLearningsToMemory("my title", "my context", "my takeaway")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	content := string(data)
	if !strings.Contains(content, "my title") {
		t.Error("should contain title")
	}
	if !strings.Contains(content, "my context") {
		t.Error("should contain context")
	}
	if !strings.Contains(content, "my takeaway") {
		t.Error("should contain takeaway")
	}
}

func TestWriteLearningsToMemory_SourceIsEvolution(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.WriteLearningsToMemory("t", "c", "tk")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	var entry map[string]interface{}
	json.Unmarshal(data, &entry)
	if entry["source"] != "evolution" {
		t.Errorf("WriteLearningsToMemory should set source='evolution', got %v", entry["source"])
	}
}
