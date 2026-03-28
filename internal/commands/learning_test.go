package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLearningsEmpty(t *testing.T) {
	dir := t.TempDir()
	learnings := loadLearnings(dir)
	if len(learnings) != 0 {
		t.Errorf("expected 0 learnings from empty dir, got %d", len(learnings))
	}
}

func TestLoadLearningsValid(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	content := `{"type":"lesson","day":1,"ts":"2026-03-25T19:24:40Z","source":"evolution","title":"test learning","context":"some context","takeaway":"a takeaway"}
{"type":"lesson","day":2,"ts":"2026-03-26T10:00:00Z","source":"evolution","title":"another learning","context":"more context","takeaway":"another takeaway"}
`
	os.WriteFile(filepath.Join(memDir, "learnings.jsonl"), []byte(content), 0o644)

	learnings := loadLearnings(dir)
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings, got %d", len(learnings))
	}
	if learnings[0].Title != "test learning" {
		t.Errorf("expected title 'test learning', got %q", learnings[0].Title)
	}
	if learnings[1].Day != 2 {
		t.Errorf("expected day 2, got %d", learnings[1].Day)
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		entry    learningEntry
		expected string
	}{
		{
			entry:    learningEntry{Title: "Added unit tests for the parser", Takeaway: "testing coverage matters"},
			expected: "testing",
		},
		{
			entry:    learningEntry{Title: "Optimized cache invalidation", Takeaway: "memory usage improved with faster responses"},
			expected: "performance",
		},
		{
			entry:    learningEntry{Title: "Fixed git merge conflict resolution", Takeaway: "branch workflow improvement"},
			expected: "git",
		},
		{
			entry:    learningEntry{Title: "Random observation", Takeaway: "nothing specific"},
			expected: "general",
		},
	}

	for _, tt := range tests {
		got := inferCategory(tt.entry)
		if got != tt.expected {
			t.Errorf("inferCategory(%q) = %q, want %q", tt.entry.Title, got, tt.expected)
		}
	}
}

func TestInferConfidence(t *testing.T) {
	// High quality entry
	high := learningEntry{
		Title:    "A meaningful insight about code architecture patterns",
		Takeaway: "Interface-based design improves testability and maintainability significantly",
		Context:  "Discovered while refactoring the command registry to use interfaces instead of concrete types",
		Source:   "evolution",
		Day:      5,
	}
	conf := inferConfidence(high)
	if conf < 0.7 {
		t.Errorf("expected high confidence for quality entry, got %.2f", conf)
	}

	// Low quality entry
	low := learningEntry{
		Title: "Fix",
	}
	conf = inferConfidence(low)
	if conf > 0.5 {
		t.Errorf("expected low confidence for minimal entry, got %.2f", conf)
	}
}

func TestCategorizeLearnings(t *testing.T) {
	entries := []learningEntry{
		{Category: "testing"},
		{Category: "testing"},
		{Category: "git"},
		{Category: ""},
	}
	cats := categorizeLearnings(entries)
	if cats["testing"] != 2 {
		t.Errorf("expected 2 testing entries, got %d", cats["testing"])
	}
	if cats["git"] != 1 {
		t.Errorf("expected 1 git entry, got %d", cats["git"])
	}
	if cats["uncategorized"] != 1 {
		t.Errorf("expected 1 uncategorized entry, got %d", cats["uncategorized"])
	}
}

func TestFilterByConfidence(t *testing.T) {
	entries := []learningEntry{
		{Title: "high", Confidence: 0.8},
		{Title: "medium", Confidence: 0.5},
		{Title: "low", Confidence: 0.2},
	}

	high := filterByConfidence(entries, 0.7)
	if len(high) != 1 {
		t.Errorf("expected 1 high-confidence entry, got %d", len(high))
	}

	all := filterByConfidence(entries, 0.0)
	if len(all) != 3 {
		t.Errorf("expected 3 entries at threshold 0.0, got %d", len(all))
	}
}

func TestFindRecurringThemes(t *testing.T) {
	entries := []learningEntry{
		{Title: "testing improves confidence", Takeaway: "test coverage matters for quality"},
		{Title: "testing edge cases", Takeaway: "test coverage catches bugs early"},
		{Title: "optimize performance", Takeaway: "caching speeds up responses"},
	}
	themes := findRecurringThemes(entries)
	foundTesting := false
	foundCoverage := false
	for _, theme := range themes {
		if strings.Contains(theme, "testing") {
			foundTesting = true
		}
		if strings.Contains(theme, "coverage") {
			foundCoverage = true
		}
	}
	if !foundTesting {
		t.Error("expected 'testing' to be a recurring theme")
	}
	if !foundCoverage {
		t.Error("expected 'coverage' to be a recurring theme")
	}
}

func TestSaveAndLoadLearningsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	entries := []learningEntry{
		{Day: 1, Title: "first", Category: "testing", Confidence: 0.8},
		{Day: 2, Title: "second", Category: "git", Confidence: 0.6},
	}

	saveLearnings(dir, entries)
	loaded := loadLearnings(dir)

	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries after round-trip, got %d", len(loaded))
	}
	if loaded[0].Title != "first" {
		t.Errorf("expected first title 'first', got %q", loaded[0].Title)
	}
	if loaded[1].Category != "git" {
		t.Errorf("expected second category 'git', got %q", loaded[1].Category)
	}
}

func TestLoadLearningsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0o755)

	content := `{"type":"lesson","day":1,"title":"valid"}
NOT_VALID_JSON
{"type":"lesson","day":2,"title":"also valid"}
`
	os.WriteFile(filepath.Join(memDir, "learnings.jsonl"), []byte(content), 0o644)

	learnings := loadLearnings(dir)
	if len(learnings) != 2 {
		t.Errorf("expected 2 valid learnings from mixed file, got %d", len(learnings))
	}
}
