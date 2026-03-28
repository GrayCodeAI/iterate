// Package context_test tests the file prioritizer.
package context_test

import (
	stdcontext "context"
	"testing"

	"github.com/GrayCodeAI/iterate/internal/context"
)

func TestFilePrioritizer_NewFilePrioritizer(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	if fp == nil {
		t.Fatal("expected file prioritizer, got nil")
	}
	
	cfg := fp.GetConfig()
	if cfg.MaxFiles != 50 {
		t.Errorf("expected default max files 50, got %d", cfg.MaxFiles)
	}
	if cfg.MaxTokens != 100000 {
		t.Errorf("expected default max tokens 100000, got %d", cfg.MaxTokens)
	}
}

func TestFilePrioritizer_PrioritizeForFile(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	files := []string{
		"main.go",
		"handler.go",
		"handler_test.go",
		"utils.go",
		"utils_test.go",
		"README.md",
	}
	
	result, err := fp.PrioritizeForFile(stdcontext.Background(), "handler.go", files)
	if err != nil {
		t.Fatalf("failed to prioritize: %v", err)
	}
	
	if result.FocusFile != "handler.go" {
		t.Errorf("expected focus file 'handler.go', got %s", result.FocusFile)
	}
	
	// Focus file should be first with highest score
	if len(result.Files) == 0 {
		t.Fatal("expected at least one file in result")
	}
	
	if result.Files[0].Path != "handler.go" {
		t.Errorf("expected first file to be 'handler.go', got %s", result.Files[0].Path)
	}
	if result.Files[0].Score != 1.0 {
		t.Errorf("expected focus file score 1.0, got %f", result.Files[0].Score)
	}
}

func TestFilePrioritizer_PrioritizeForQuery(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	files := []string{
		"handler.go",
		"handler_test.go",
		"user_handler.go",
		"utils.go",
		"main.go",
	}
	
	result, err := fp.PrioritizeForQuery(stdcontext.Background(), "handler user", files)
	if err != nil {
		t.Fatalf("failed to prioritize: %v", err)
	}
	
	if result.FocusFile != "" {
		t.Error("expected empty focus file for query-based prioritization")
	}
	
	// Files matching query terms should have higher scores
	if len(result.Files) == 0 {
		t.Fatal("expected at least one file in result")
	}
}

func TestFilePrioritizer_TestFileDetection(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	files := []string{
		"main.go",
		"main_test.go",
		"handler.spec.ts",
		"utils.test.js",
		"service.py",
		"service_test.py",
	}
	
	result, err := fp.PrioritizeForFile(stdcontext.Background(), "main.go", files)
	if err != nil {
		t.Fatalf("failed to prioritize: %v", err)
	}
	
	// Find test files in result
	testFiles := 0
	for _, f := range result.Files {
		if f.IsTest {
			testFiles++
		}
	}
	
	if testFiles == 0 {
		t.Error("expected some test files to be identified")
	}
}

func TestFilePrioritizer_BudgetLimit(t *testing.T) {
	cfg := context.DefaultFilePriorityConfig()
	cfg.MaxTokens = 3000 // Only 3 files worth
	cfg.MaxFiles = 3
	
	fp := context.NewFilePrioritizer(cfg, nil, nil, nil, nil)
	
	files := []string{
		"file1.go",
		"file2.go",
		"file3.go",
		"file4.go",
		"file5.go",
	}
	
	result, err := fp.PrioritizeForFile(stdcontext.Background(), "file1.go", files)
	if err != nil {
		t.Fatalf("failed to prioritize: %v", err)
	}
	
	if result.FilesIncluded > 3 {
		t.Errorf("expected at most 3 files, got %d", result.FilesIncluded)
	}
	if result.FilesSkipped == 0 {
		t.Error("expected some files to be skipped due to budget")
	}
}

func TestFilePrioritizer_ContextCancellation(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel() // Cancel immediately
	
	files := []string{"main.go", "handler.go"}
	
	_, err := fp.PrioritizeForFile(ctx, "main.go", files)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main_test.go", true},
		{"handler.spec.ts", true},
		{"utils.test.js", true},
		{"service_test.py", true},
		{"UserTest.java", true},
		{"main.go", false},
		{"handler.go", false},
		{"README.md", false},
		{"pkg/main_test.go", true},
		{"internal/handler_test.go", true},
	}
	
	for _, tt := range tests {
		result := context.IsTestFilePublic(tt.path)
		if result != tt.expected {
			t.Errorf("IsTestFile(%q) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsTestForFile(t *testing.T) {
	tests := []struct {
		testFile  string
		focusFile string
		expected  bool
	}{
		{"main_test.go", "main.go", true},
		{"handler_test.go", "handler.go", true},
		{"user_handler_test.go", "user_handler.go", true},
		{"handler.spec.ts", "handler.ts", true},
		{"main_test.go", "handler.go", false},
		{"utils_test.go", "main.go", false},
		{"pkg/main_test.go", "main.go", true},
	}
	
	for _, tt := range tests {
		result := context.IsTestForFilePublic(tt.testFile, tt.focusFile)
		if result != tt.expected {
			t.Errorf("IsTestForFile(%q, %q) = %v, expected %v", 
				tt.testFile, tt.focusFile, result, tt.expected)
		}
	}
}

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		query    string
		expected int // minimum expected terms
	}{
		{"implement user authentication", 3},
		{"fix the bug in handler", 3}, // "the", "in" are stop words
		{"add feature to the main module", 4}, // "to", "the" are stop words
		{"create new API endpoint", 4},
		{"", 0},
	}
	
	for _, tt := range tests {
		result := context.ExtractTermsPublic(tt.query)
		if len(result) < tt.expected {
			t.Errorf("ExtractTerms(%q) returned %d terms, expected at least %d",
				tt.query, len(result), tt.expected)
		}
	}
}

func TestFilePrioritizer_UpdateConfig(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	newCfg := context.DefaultFilePriorityConfig()
	newCfg.MaxFiles = 100
	newCfg.MaxTokens = 200000
	
	fp.UpdateConfig(newCfg)
	
	cfg := fp.GetConfig()
	if cfg.MaxFiles != 100 {
		t.Errorf("expected max files 100, got %d", cfg.MaxFiles)
	}
	if cfg.MaxTokens != 200000 {
		t.Errorf("expected max tokens 200000, got %d", cfg.MaxTokens)
	}
}

func TestFilePrioritizer_ToMarkdown(t *testing.T) {
	fp := context.NewFilePrioritizer(nil, nil, nil, nil, nil)
	
	files := []string{"main.go", "handler.go", "handler_test.go"}
	
	result, err := fp.PrioritizeForFile(stdcontext.Background(), "main.go", files)
	if err != nil {
		t.Fatalf("failed to prioritize: %v", err)
	}
	
	markdown := result.ToMarkdown()
	
	if markdown == "" {
		t.Error("expected non-empty markdown output")
	}
	if result.FocusFile != "" && !containsStr(markdown, result.FocusFile) {
		t.Error("expected markdown to contain focus file")
	}
}

// Helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
