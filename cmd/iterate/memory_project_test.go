package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func tempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// ---------------------------------------------------------------------------
// projectMemoryPath
// ---------------------------------------------------------------------------

func TestProjectMemoryPath(t *testing.T) {
	path := projectMemoryPath("/some/repo")
	want := filepath.Join("/some/repo", ".iterate", "memory.json")
	if path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
}

// ---------------------------------------------------------------------------
// loadProjectMemory
// ---------------------------------------------------------------------------

func TestLoadProjectMemory_MissingFile(t *testing.T) {
	dir := tempRepo(t)
	m := loadProjectMemory(dir)
	if len(m.Entries) != 0 {
		t.Errorf("expected empty entries for missing file, got %d", len(m.Entries))
	}
}

func TestLoadProjectMemory_InvalidJSON(t *testing.T) {
	dir := tempRepo(t)
	p := projectMemoryPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 0 {
		t.Errorf("expected empty entries for invalid JSON, got %d", len(m.Entries))
	}
}

func TestLoadProjectMemory_ValidFile(t *testing.T) {
	dir := tempRepo(t)
	pm := projectMemory{
		Entries: []projectMemoryEntry{
			{Note: "first note", CreatedAt: "2024-01-01T00:00:00Z"},
			{Note: "second note", CreatedAt: "2024-01-02T00:00:00Z"},
		},
	}
	p := projectMemoryPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(pm, "", "  ")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := loadProjectMemory(dir)
	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Entries))
	}
	if m.Entries[0].Note != "first note" {
		t.Errorf("expected 'first note', got %q", m.Entries[0].Note)
	}
	if m.Entries[1].Note != "second note" {
		t.Errorf("expected 'second note', got %q", m.Entries[1].Note)
	}
}

// ---------------------------------------------------------------------------
// saveProjectMemory
// ---------------------------------------------------------------------------

func TestSaveProjectMemory_CreatesDir(t *testing.T) {
	dir := tempRepo(t)
	pm := projectMemory{
		Entries: []projectMemoryEntry{{Note: "test", CreatedAt: "2024-01-01T00:00:00Z"}},
	}
	if err := saveProjectMemory(dir, pm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := projectMemoryPath(dir)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestSaveProjectMemory_RoundTrip(t *testing.T) {
	dir := tempRepo(t)
	pm := projectMemory{
		Entries: []projectMemoryEntry{
			{Note: "round-trip note", CreatedAt: "2024-03-15T10:30:00Z"},
		},
	}
	if err := saveProjectMemory(dir, pm); err != nil {
		t.Fatalf("save error: %v", err)
	}
	loaded := loadProjectMemory(dir)
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Note != "round-trip note" {
		t.Errorf("expected 'round-trip note', got %q", loaded.Entries[0].Note)
	}
	if loaded.Entries[0].CreatedAt != "2024-03-15T10:30:00Z" {
		t.Errorf("expected timestamp preserved, got %q", loaded.Entries[0].CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// addProjectMemoryNote
// ---------------------------------------------------------------------------

func TestAddProjectMemoryNote_FirstNote(t *testing.T) {
	dir := tempRepo(t)
	if err := addProjectMemoryNote(dir, "my first note"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Entries))
	}
	if m.Entries[0].Note != "my first note" {
		t.Errorf("expected 'my first note', got %q", m.Entries[0].Note)
	}
	if m.Entries[0].CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
}

func TestAddProjectMemoryNote_MultipleNotes(t *testing.T) {
	dir := tempRepo(t)
	for _, note := range []string{"alpha", "beta", "gamma"} {
		if err := addProjectMemoryNote(dir, note); err != nil {
			t.Fatalf("add %q: %v", note, err)
		}
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m.Entries))
	}
	if m.Entries[0].Note != "alpha" || m.Entries[1].Note != "beta" || m.Entries[2].Note != "gamma" {
		t.Errorf("unexpected order: %+v", m.Entries)
	}
}

func TestAddProjectMemoryNote_AppendsToPrevious(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "existing")
	_ = addProjectMemoryNote(dir, "new note")
	m := loadProjectMemory(dir)
	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Entries))
	}
}

// ---------------------------------------------------------------------------
// removeProjectMemoryEntry
// ---------------------------------------------------------------------------

func TestRemoveProjectMemoryEntry_ValidIndex(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "keep")
	_ = addProjectMemoryNote(dir, "remove me")
	_ = addProjectMemoryNote(dir, "also keep")

	entry, ok := removeProjectMemoryEntry(dir, 1)
	if !ok {
		t.Fatal("expected ok=true for valid index")
	}
	if entry.Note != "remove me" {
		t.Errorf("expected 'remove me', got %q", entry.Note)
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(m.Entries))
	}
	if m.Entries[0].Note != "keep" || m.Entries[1].Note != "also keep" {
		t.Errorf("unexpected entries after removal: %+v", m.Entries)
	}
}

func TestRemoveProjectMemoryEntry_IndexZero(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "first")
	_ = addProjectMemoryNote(dir, "second")

	entry, ok := removeProjectMemoryEntry(dir, 0)
	if !ok {
		t.Fatal("expected ok=true for index 0")
	}
	if entry.Note != "first" {
		t.Errorf("expected 'first', got %q", entry.Note)
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 1 || m.Entries[0].Note != "second" {
		t.Errorf("unexpected state after removal: %+v", m.Entries)
	}
}

func TestRemoveProjectMemoryEntry_LastIndex(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "a")
	_ = addProjectMemoryNote(dir, "b")

	_, ok := removeProjectMemoryEntry(dir, 1)
	if !ok {
		t.Fatal("expected ok=true for last index")
	}
	m := loadProjectMemory(dir)
	if len(m.Entries) != 1 || m.Entries[0].Note != "a" {
		t.Errorf("unexpected state: %+v", m.Entries)
	}
}

func TestRemoveProjectMemoryEntry_NegativeIndex(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "note")

	_, ok := removeProjectMemoryEntry(dir, -1)
	if ok {
		t.Error("expected ok=false for negative index")
	}
}

func TestRemoveProjectMemoryEntry_OutOfBounds(t *testing.T) {
	dir := tempRepo(t)
	_ = addProjectMemoryNote(dir, "only note")

	_, ok := removeProjectMemoryEntry(dir, 5)
	if ok {
		t.Error("expected ok=false for out-of-bounds index")
	}
}

func TestRemoveProjectMemoryEntry_EmptyMemory(t *testing.T) {
	dir := tempRepo(t)
	_, ok := removeProjectMemoryEntry(dir, 0)
	if ok {
		t.Error("expected ok=false on empty memory")
	}
}

// ---------------------------------------------------------------------------
// formatProjectMemoryForPrompt
// ---------------------------------------------------------------------------

func TestFormatProjectMemoryForPrompt_Empty(t *testing.T) {
	m := projectMemory{}
	result := formatProjectMemoryForPrompt(m)
	if result != "" {
		t.Errorf("expected empty string for empty memory, got %q", result)
	}
}

func TestFormatProjectMemoryForPrompt_SingleEntry(t *testing.T) {
	m := projectMemory{
		Entries: []projectMemoryEntry{{Note: "use context7 for docs"}},
	}
	result := formatProjectMemoryForPrompt(m)
	if !strings.Contains(result, "## Project Notes") {
		t.Error("expected '## Project Notes' header")
	}
	if !strings.Contains(result, "- use context7 for docs") {
		t.Errorf("expected note in output, got %q", result)
	}
}

func TestFormatProjectMemoryForPrompt_MultipleEntries(t *testing.T) {
	m := projectMemory{
		Entries: []projectMemoryEntry{
			{Note: "alpha"},
			{Note: "beta"},
			{Note: "gamma"},
		},
	}
	result := formatProjectMemoryForPrompt(m)
	if !strings.Contains(result, "- alpha") {
		t.Error("expected '- alpha'")
	}
	if !strings.Contains(result, "- beta") {
		t.Error("expected '- beta'")
	}
	if !strings.Contains(result, "- gamma") {
		t.Error("expected '- gamma'")
	}
}

func TestFormatProjectMemoryForPrompt_StartsWithHeader(t *testing.T) {
	m := projectMemory{
		Entries: []projectMemoryEntry{{Note: "note"}},
	}
	result := formatProjectMemoryForPrompt(m)
	if !strings.HasPrefix(result, "## Project Notes") {
		t.Errorf("expected result to start with '## Project Notes', got %q", result)
	}
}

func TestFormatProjectMemoryForPrompt_EndsWithNewline(t *testing.T) {
	m := projectMemory{
		Entries: []projectMemoryEntry{{Note: "note"}},
	}
	result := formatProjectMemoryForPrompt(m)
	if !strings.HasSuffix(result, "\n") {
		t.Errorf("expected result to end with newline, got %q", result)
	}
}
