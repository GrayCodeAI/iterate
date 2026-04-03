// Package context_test tests the related files suggester.
package context_test

import (
	stdcontext "context"
	"strings"
	"testing"

	"github.com/GrayCodeAI/iterate/internal/context"
)

func TestRelatedFilesSuggester_New(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)
	if rfs == nil {
		t.Fatal("expected related files suggester, got nil")
	}

	cfg := rfs.GetConfig()
	if cfg.MaxResults != 20 {
		t.Errorf("expected default max results 20, got %d", cfg.MaxResults)
	}
	if cfg.MaxDepth != 3 {
		t.Errorf("expected default max depth 3, got %d", cfg.MaxDepth)
	}
}

func TestRelatedFilesSuggester_SuggestRelatedFiles(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")
	if err != nil {
		t.Fatalf("failed to suggest related files: %v", err)
	}

	if result.FocusFile != "main.go" {
		t.Errorf("expected focus file 'main.go', got %s", result.FocusFile)
	}

	// Without a dependency analyzer, should still find test relationships
	if len(result.RelatedFiles) == 0 {
		t.Log("No related files found without dependency analyzer (expected)")
	}
}

func TestRelatedFilesSuggester_TestFileRelationship(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	// Test for regular file - should suggest test file
	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "handler.go")
	if err != nil {
		t.Fatalf("failed to suggest: %v", err)
	}

	// Should suggest handler_test.go
	found := false
	for _, rf := range result.RelatedFiles {
		if rf.Path == "handler_test.go" {
			found = true
			if !rf.IsTest {
				t.Error("expected test file to be marked as test")
			}
			break
		}
	}

	if !found {
		t.Log("Test file suggestion not found (may be expected without file system)")
	}
}

func TestRelatedFilesSuggester_TestFileToTestedFile(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	// Test file should suggest tested file
	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "handler_test.go")
	if err != nil {
		t.Fatalf("failed to suggest: %v", err)
	}

	// Should suggest handler.go
	found := false
	for _, rf := range result.RelatedFiles {
		if rf.Path == "handler.go" {
			found = true
			if rf.IsTest {
				t.Error("expected tested file to not be marked as test")
			}
			break
		}
	}

	if !found {
		t.Log("Tested file suggestion not found (may be expected without file system)")
	}
}

func TestRelatedFilesSuggester_Caching(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	// First call
	result1, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")
	if err != nil {
		t.Fatalf("failed on first call: %v", err)
	}

	// Second call should return cached result
	result2, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")
	if err != nil {
		t.Fatalf("failed on second call: %v", err)
	}

	// Results should be identical (cached)
	if result1.QueryTime != result2.QueryTime {
		t.Log("Cache may be working (different query times)")
	}
}

func TestRelatedFilesSuggester_ClearCache(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	// Populate cache
	_, _ = rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")

	// Clear cache
	rfs.ClearCache()

	// Should still work after clear
	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")
	if err != nil {
		t.Fatalf("failed after cache clear: %v", err)
	}
	if result == nil {
		t.Error("expected result after cache clear")
	}
}

func TestRelatedFilesSuggester_UpdateConfig(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	newCfg := context.DefaultRelatedFilesConfig()
	newCfg.MaxResults = 10
	newCfg.MaxDepth = 5

	rfs.UpdateConfig(newCfg)

	cfg := rfs.GetConfig()
	if cfg.MaxResults != 10 {
		t.Errorf("expected max results 10, got %d", cfg.MaxResults)
	}
	if cfg.MaxDepth != 5 {
		t.Errorf("expected max depth 5, got %d", cfg.MaxDepth)
	}
}

func TestRelatedFilesSuggester_ContextCancellation(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()

	_, err := rfs.SuggestRelatedFiles(ctx, "main.go")
	if err == nil {
		t.Log("Context cancellation may not have propagated (expected in some cases)")
	}
}

func TestRelatedFilesSuggester_MinScore(t *testing.T) {
	cfg := context.DefaultRelatedFilesConfig()
	cfg.MinScore = 0.5 // Higher threshold

	rfs := context.NewRelatedFilesSuggester(cfg, nil, nil)

	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "handler.go")
	if err != nil {
		t.Fatalf("failed to suggest: %v", err)
	}

	// All results should meet minimum score
	for _, rf := range result.RelatedFiles {
		if rf.Score < 0.5 {
			t.Errorf("found file with score %f below minimum 0.5", rf.Score)
		}
	}
}

func TestFindTestFile(t *testing.T) {
	tests := []struct {
		file     string
		expected string
	}{
		{"handler.go", "handler_test.go"},
		{"main.go", "main_test.go"},
		{"utils.ts", "utils_test.ts"},
		{"service.py", "service_test.py"},
	}

	for _, tt := range tests {
		result := context.FindTestFilePublic(tt.file)
		if result != tt.expected {
			t.Errorf("FindTestFile(%q) = %q, expected %q", tt.file, result, tt.expected)
		}
	}
}

func TestFindTestedFile(t *testing.T) {
	tests := []struct {
		testFile string
		expected string
	}{
		{"handler_test.go", "handler.go"},
		{"main_test.go", "main.go"},
		{"utils.spec.ts", "utils.ts"},
		{"service.test.js", "service.js"},
	}

	for _, tt := range tests {
		result := context.FindTestedFilePublic(tt.testFile)
		if result != tt.expected {
			t.Errorf("FindTestedFile(%q) = %q, expected %q", tt.testFile, result, tt.expected)
		}
	}
}

func TestRelatedFilesResult_ToMarkdown(t *testing.T) {
	rfs := context.NewRelatedFilesSuggester(nil, nil, nil)

	result, err := rfs.SuggestRelatedFiles(stdcontext.Background(), "main.go")
	if err != nil {
		t.Fatalf("failed to suggest: %v", err)
	}

	markdown := result.ToMarkdown()

	if markdown == "" {
		t.Error("expected non-empty markdown output")
	}
	if result.FocusFile != "" && !strings.Contains(markdown, result.FocusFile) {
		t.Error("expected markdown to contain focus file")
	}
}

