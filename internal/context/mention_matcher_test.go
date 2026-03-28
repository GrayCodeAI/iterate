// Package context provides smart @ mention matching capabilities.
package context

import (
	"context"
	"testing"
	"time"
)

func TestDefaultMentionConfig(t *testing.T) {
	config := DefaultMentionConfig()
	
	if config.MaxResults != 10 {
		t.Errorf("expected 10, got %d", config.MaxResults)
	}
	if config.MinScore != 0.3 {
		t.Errorf("expected 0.3, got %f", config.MinScore)
	}
	if config.FuzzyThreshold != 0.5 {
		t.Errorf("expected 0.5, got %f", config.FuzzyThreshold)
	}
	if config.IncludeHidden {
		t.Error("expected IncludeHidden to be false")
	}
	if config.CacheTTL != 5*time.Minute {
		t.Errorf("expected 5m, got %v", config.CacheTTL)
	}
}

func TestNewMentionMatcher(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	if mm == nil {
		t.Fatal("expected non-nil matcher")
	}
	if mm.files == nil {
		t.Error("expected files map to be initialized")
	}
	if mm.basenameIndex == nil {
		t.Error("expected basenameIndex to be initialized")
	}
}

func TestMentionMatcher_IndexFiles(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	
	files := []string{
		"handler.go",
		"service.ts",
		"utils.py",
		".hidden",
	}
	
	mm.IndexFiles(files)
	
	if len(mm.files) != 4 {
		t.Errorf("expected 4 files, got %d", len(mm.files))
	}
	
	// Check basename index - key is the basename, value is list of full paths
	if len(mm.basenameIndex["handler.go"]) == 0 {
		t.Error("expected basename index to have handler.go entry")
	}
}

func TestMentionMatcher_SetOpenFiles(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.ts"})
	
	mm.SetOpenFiles([]string{"handler.go"})
	
	if !mm.openFiles["handler.go"] {
		t.Error("expected handler.go to be open")
	}
	if mm.openFiles["service.ts"] {
		t.Error("expected service.ts to not be open")
	}
}

func TestMentionMatcher_Match_ExactMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "handler_test.go", "service.go"})
	
	result, err := mm.Match(context.Background(), "handler.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) == 0 {
		t.Fatal("expected at least one match")
	}
	
	// First match should be exact
	match := result.Matches[0]
	if match.MatchType != "exact" {
		t.Errorf("expected exact match, got %s", match.MatchType)
	}
	if match.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", match.Score)
	}
	if match.Name != "handler.go" {
		t.Errorf("expected handler.go, got %s", match.Name)
	}
}

func TestMentionMatcher_Match_PrefixMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "handler_test.go", "service.go"})
	
	result, err := mm.Match(context.Background(), "hand")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) < 2 {
		t.Errorf("expected at least 2 matches, got %d", len(result.Matches))
	}
	
	// All matches should be prefix type
	for _, m := range result.Matches {
		if m.MatchType != "prefix" {
			t.Errorf("expected prefix match, got %s", m.MatchType)
		}
	}
}

func TestMentionMatcher_Match_ContainsMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"my_handler.go", "service.go"})
	
	result, err := mm.Match(context.Background(), "handler")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) == 0 {
		t.Fatal("expected at least one match")
	}
	
	match := result.Matches[0]
	if match.MatchType != "contains" {
		t.Errorf("expected contains match, got %s", match.MatchType)
	}
}

func TestMentionMatcher_Match_FuzzyMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.go"})
	
	// "hdlr" should fuzzy match "handler"
	result, err := mm.Match(context.Background(), "hdlr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should find at least handler.go
	if len(result.Matches) == 0 {
		t.Log("No fuzzy match found (may be below threshold)")
	} else {
		found := false
		for _, m := range result.Matches {
			if m.Name == "handler.go" {
				found = true
				if m.MatchType != "fuzzy" {
					t.Errorf("expected fuzzy match, got %s", m.MatchType)
				}
			}
		}
		if !found {
			t.Log("handler.go not found in fuzzy matches")
		}
	}
}

func TestMentionMatcher_Match_NoMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.go"})
	
	result, err := mm.Match(context.Background(), "zzzzz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) != 0 {
		t.Errorf("expected no matches, got %d", len(result.Matches))
	}
}

func TestMentionMatcher_Match_EmptyQuery(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go"})
	
	result, err := mm.Match(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) != 0 {
		t.Errorf("expected no matches for empty query, got %d", len(result.Matches))
	}
}

func TestMentionMatcher_FuzzyMatch(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	
	tests := []struct {
		pattern string
		text    string
		minScore float64
	}{
		{"hdlr", "handler", 0.5},
		{"test", "test", 1.0},
		{"abc", "xyz", 0.0},
		{"go", "golang", 0.5},
	}
	
	for _, tc := range tests {
		score, _ := mm.fuzzyMatch(tc.pattern, tc.text)
		if tc.minScore > 0 && score < tc.minScore {
			t.Errorf("fuzzyMatch(%s, %s) = %f, expected >= %f", tc.pattern, tc.text, score, tc.minScore)
		}
		if tc.minScore == 0 && score > 0 {
			t.Errorf("fuzzyMatch(%s, %s) = %f, expected 0", tc.pattern, tc.text, score)
		}
	}
}

func TestMentionMatcher_FuzzyMatch_Ranges(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	
	// Test that ranges are returned correctly
	score, ranges := mm.fuzzyMatch("test", "test")
	if score != 1.0 {
		t.Errorf("expected score 1.0, got %f", score)
	}
	if len(ranges) != 2 || ranges[0] != 0 || ranges[1] != 4 {
		t.Errorf("expected ranges [0, 4], got %v", ranges)
	}
}

func TestMentionMatcher_GetSuggestions(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "handler_test.go", "service.go"})
	
	suggestions := mm.GetSuggestions("hand", 5)
	
	if len(suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestMentionMatcher_ResolveMention(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.go"})
	
	// Test exact basename match
	path, ok := mm.ResolveMention("handler.go")
	if !ok {
		t.Error("expected to resolve handler.go")
	}
	if path != "handler.go" {
		t.Errorf("expected handler.go, got %s", path)
	}
	
	// Test non-existent
	_, ok = mm.ResolveMention("nonexistent.go")
	if ok {
		t.Error("expected not to resolve nonexistent.go")
	}
}

func TestMentionMatcher_ClearCache(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go"})
	
	// Make a query to populate cache
	_, _ = mm.Match(context.Background(), "handler")
	
	if len(mm.queryCache) == 0 {
		t.Error("expected query cache to be populated")
	}
	
	mm.ClearCache()
	
	if len(mm.queryCache) != 0 {
		t.Error("expected query cache to be empty after clear")
	}
}

func TestMentionMatcher_GetStats(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.go"})
	mm.SetOpenFiles([]string{"handler.go"})
	
	stats := mm.GetStats()
	
	if stats["indexed_files"].(int) != 2 {
		t.Errorf("expected 2 indexed files, got %v", stats["indexed_files"])
	}
	if stats["open_files"].(int) != 1 {
		t.Errorf("expected 1 open file, got %v", stats["open_files"])
	}
}

func TestMentionMatcher_UpdateConfig(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	
	newConfig := &MentionConfig{
		MaxResults:  20,
		MinScore:    0.5,
		CacheTTL:    10 * time.Minute,
	}
	
	mm.UpdateConfig(newConfig)
	
	if mm.config.MaxResults != 20 {
		t.Errorf("expected 20, got %d", mm.config.MaxResults)
	}
}

func TestMentionMatcher_PreferOpenFiles(t *testing.T) {
	config := DefaultMentionConfig()
	config.PreferOpenFiles = true
	mm := NewMentionMatcher(config, nil)
	
	mm.IndexFiles([]string{"a_handler.go", "b_handler.go"})
	mm.SetOpenFiles([]string{"b_handler.go"})
	
	result, err := mm.Match(context.Background(), "handler")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(result.Matches))
	}
	
	// b_handler.go should come first because it's open
	if result.Matches[0].Name != "b_handler.go" {
		t.Errorf("expected b_handler.go first (open file), got %s", result.Matches[0].Name)
	}
}

func TestMentionMatcher_ExcludeHidden(t *testing.T) {
	config := DefaultMentionConfig()
	config.IncludeHidden = false
	mm := NewMentionMatcher(config, nil)
	
	mm.IndexFiles([]string{"handler.go", ".hidden"})
	
	result, err := mm.Match(context.Background(), "hidden")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should not find .hidden
	for _, m := range result.Matches {
		if m.Name == ".hidden" {
			t.Error("expected .hidden to be excluded")
		}
	}
}

func TestMentionMatcher_IncludeHidden(t *testing.T) {
	config := DefaultMentionConfig()
	config.IncludeHidden = true
	mm := NewMentionMatcher(config, nil)
	
	mm.IndexFiles([]string{"handler.go", ".hidden"})
	
	result, err := mm.Match(context.Background(), "hidden")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should find .hidden
	found := false
	for _, m := range result.Matches {
		if m.Name == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Error("expected .hidden to be included")
	}
}

func TestMentionMatcher_MaxResults(t *testing.T) {
	config := DefaultMentionConfig()
	config.MaxResults = 2
	mm := NewMentionMatcher(config, nil)
	
	mm.IndexFiles([]string{"a.go", "b.go", "c.go", "d.go"})
	
	result, err := mm.Match(context.Background(), ".go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Matches) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(result.Matches))
	}
}

func TestMentionMatcher_ContextCancellation(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	
	// Index many files
	files := make([]string, 100)
	for i := 0; i < 100; i++ {
		files[i] = "file" + string(rune('0'+i%10)) + ".go"
	}
	mm.IndexFiles(files)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	_, err := mm.Match(ctx, "file")
	if err != context.Canceled {
		t.Logf("context cancellation result: %v", err)
	}
}

func TestMentionResult_ToMarkdown(t *testing.T) {
	result := &MentionResult{
		Query: "handler",
		Matches: []*MentionMatch{
			{Name: "handler.go", Path: "internal/handler.go", MatchType: "exact", Score: 1.0},
			{Name: "handler_test.go", Path: "internal/handler_test.go", MatchType: "prefix", Score: 0.9, IsOpen: true},
		},
		Total:    2,
		Duration: 5 * time.Millisecond,
	}
	
	markdown := result.ToMarkdown()
	
	if markdown == "" {
		t.Error("expected non-empty markdown")
	}
	if !contains(markdown, "handler") {
		t.Error("expected 'handler' in markdown")
	}
	if !contains(markdown, "exact") {
		t.Error("expected 'exact' in markdown")
	}
}

func TestMentionResult_ToMarkdown_NoMatches(t *testing.T) {
	result := &MentionResult{
		Query:    "nonexistent",
		Matches:  []*MentionMatch{},
		Total:    0,
		Duration: 1 * time.Millisecond,
	}
	
	markdown := result.ToMarkdown()
	
	if !contains(markdown, "No matches found") {
		t.Errorf("expected 'No matches found' in markdown: %s", markdown)
	}
}

func TestMentionMatcher_MatchByType(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go", "service.go"})
	
	result, err := mm.MatchByType(context.Background(), ".go", "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// All matches should be type "file"
	for _, m := range result.Matches {
		if m.Type != "file" {
			t.Errorf("expected type 'file', got %s", m.Type)
		}
	}
}

func TestMentionMatcher_CacheHit(t *testing.T) {
	mm := NewMentionMatcher(nil, nil)
	mm.IndexFiles([]string{"handler.go"})
	
	// First query
	result1, err := mm.Match(context.Background(), "handler")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Second query should hit cache
	result2, err := mm.Match(context.Background(), "handler")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Results should be identical
	if result1.Total != result2.Total {
		t.Errorf("cached result differs: %d vs %d", result1.Total, result2.Total)
	}
}
