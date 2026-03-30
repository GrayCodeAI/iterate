package suggest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestSuggester_GetSuggestions_FuncPrefix(t *testing.T) {
	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	suggestions, err := suggester.GetSuggestions(ctx, Context{
		Prefix:   "func ",
		RepoPath: t.TempDir(),
	})

	if err != nil {
		t.Fatalf("GetSuggestions failed: %v", err)
	}

	found := false
	for _, s := range suggestions {
		if strings.Contains(s.Label, "main") || strings.Contains(s.Text, "main") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected suggestions to contain main function for 'func ' prefix, got: %v", suggestions)
	}
}

func TestSuggester_GetSuggestions_IfPrefix(t *testing.T) {
	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	suggestions, err := suggester.GetSuggestions(ctx, Context{
		Prefix:   "if ",
		RepoPath: t.TempDir(),
	})

	if err != nil {
		t.Fatalf("GetSuggestions failed: %v", err)
	}

	found := false
	for _, s := range suggestions {
		if strings.Contains(s.Text, "err != nil") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected suggestions to contain err check for 'if ' prefix, got: %v", suggestions)
	}
}

func TestSuggester_GetSuggestions_AtPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	suggestions, err := suggester.GetSuggestions(ctx, Context{
		Prefix:   "@",
		RepoPath: tmpDir,
	})

	if err != nil {
		t.Fatalf("GetSuggestions failed: %v", err)
	}

	found := false
	for _, s := range suggestions {
		if s.Kind == "file" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected suggestions to contain file suggestions for '@' prefix, got: %v", suggestions)
	}
}

func TestSuggester_GetSuggestions_EmptyPrefix(t *testing.T) {
	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	suggestions, err := suggester.GetSuggestions(ctx, Context{
		Prefix:   "",
		RepoPath: t.TempDir(),
	})

	if err != nil {
		t.Fatalf("GetSuggestions failed: %v", err)
	}

	if suggestions == nil {
		t.Error("Expected non-nil suggestions for empty prefix")
	}
}

func TestListGoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"main.go",
		"utils.go",
		"utils_test.go",
		"README.md",
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.WriteFile(path, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	goFiles, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listGoFiles failed: %v", err)
	}

	if len(goFiles) != 2 {
		t.Errorf("Expected 2 Go files (excluding test files), got %d: %v", len(goFiles), goFiles)
	}

	for _, f := range goFiles {
		if strings.HasSuffix(f, "_test.go") {
			t.Errorf("Should not include test files, but found: %s", f)
		}
		if filepath.Ext(f) != ".go" {
			t.Errorf("Should only include .go files, but found: %s", f)
		}
	}
}

func TestListGoFiles_ExcludesVendor(t *testing.T) {
	tmpDir := t.TempDir()

	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vendorDir, "vendored.go"), []byte("package vendor"), 0644); err != nil {
		t.Fatalf("Failed to create vendored.go: %v", err)
	}

	goFiles, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listGoFiles failed: %v", err)
	}

	for _, f := range goFiles {
		if strings.Contains(f, "vendor") {
			t.Errorf("Should exclude vendor directory, but found: %s", f)
		}
	}
}

func TestListGoFiles_ExcludesHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("Failed to create hidden dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.go"), []byte("package hidden"), 0644); err != nil {
		t.Fatalf("Failed to create hidden.go: %v", err)
	}

	goFiles, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listGoFiles failed: %v", err)
	}

	for _, f := range goFiles {
		if strings.Contains(f, ".hidden") {
			t.Errorf("Should exclude hidden directories, but found: %s", f)
		}
	}
}

func TestListGoFiles_NonExistentDir(t *testing.T) {
	_, err := listGoFiles("/non/existent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent directory")
	}
}

func TestGetSnippet(t *testing.T) {
	snippet, ok := GetSnippet("func")
	if !ok {
		t.Fatal("Expected to find 'func' snippet")
	}

	if snippet.Text == "" {
		t.Error("Expected non-empty snippet text for 'func'")
	}

	if snippet.Label != "function" {
		t.Errorf("Expected label 'function', got: %s", snippet.Label)
	}

	if snippet.Kind != "snippet" {
		t.Errorf("Expected kind 'snippet', got: %s", snippet.Kind)
	}
}

func TestGetSnippet_Unknown(t *testing.T) {
	_, ok := GetSnippet("unknown_snippet_12345")
	if ok {
		t.Error("Expected false for unknown snippet")
	}
}

func TestGetSnippet_AllSnippets(t *testing.T) {
	snippets := []string{"func", "for", "forr", "if", "ifer", "struct", "method", "test", "main", "ctx"}

	for _, name := range snippets {
		snippet, ok := GetSnippet(name)
		if !ok {
			t.Errorf("Expected to find %q snippet", name)
			continue
		}
		if snippet.Text == "" {
			t.Errorf("Expected non-empty text for %q snippet", name)
		}
		if snippet.Label == "" {
			t.Errorf("Expected non-empty label for %q snippet", name)
		}
	}
}

func TestShowSuggestions(t *testing.T) {
	suggestions := []Suggestion{
		{Text: "func main() {}", Label: "main", Description: "Main function", Kind: "snippet"},
		{Text: "if err != nil {}", Label: "if err", Description: "Error check", Kind: "snippet"},
		{Text: "for i := 0; i < n; i++ {}", Label: "for loop", Description: "For loop", Kind: "snippet"},
	}

	ShowSuggestions(suggestions)
}

func TestShowSuggestions_Empty(t *testing.T) {
	ShowSuggestions([]Suggestion{})
}

func TestShowSuggestions_Nil(t *testing.T) {
	ShowSuggestions(nil)
}

func TestShowSuggestions_Many(t *testing.T) {
	suggestions := make([]Suggestion, 10)
	for i := range suggestions {
		suggestions[i] = Suggestion{
			Text:        "suggestion",
			Label:       "label",
			Description: "desc",
			Kind:        "snippet",
		}
	}

	ShowSuggestions(suggestions)
}

func TestSuggester_GetSuggestions_ContextBased(t *testing.T) {
	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	tests := []struct {
		name   string
		prefix string
		check  func(Suggestion) bool
	}{
		{
			name:   "function definition context",
			prefix: "func ",
			check:  func(s Suggestion) bool { return strings.Contains(s.Label, "main") },
		},
		{
			name:   "if statement context",
			prefix: "if ",
			check:  func(s Suggestion) bool { return strings.Contains(s.Text, "err != nil") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, err := suggester.GetSuggestions(ctx, Context{
				Prefix:   tt.prefix,
				RepoPath: t.TempDir(),
			})

			if err != nil {
				t.Fatalf("GetSuggestions failed: %v", err)
			}

			if len(suggestions) == 0 {
				t.Errorf("Expected suggestions for prefix %q, got none", tt.prefix)
			}

			found := false
			for _, s := range suggestions {
				if tt.check(s) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected to find matching suggestion for prefix %q", tt.prefix)
			}
		})
	}
}

func TestListGoFiles_Recursive(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create root.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "sub.go"), []byte("package sub"), 0644); err != nil {
		t.Fatalf("Failed to create sub.go: %v", err)
	}

	goFiles, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listGoFiles failed: %v", err)
	}

	if len(goFiles) != 2 {
		t.Errorf("Expected 2 Go files recursively, got %d: %v", len(goFiles), goFiles)
	}

	foundRoot, foundSub := false, false
	for _, f := range goFiles {
		if filepath.Base(f) == "root.go" {
			foundRoot = true
		}
		if filepath.Base(f) == "sub.go" {
			foundSub = true
		}
	}

	if !foundRoot {
		t.Error("Expected to find root.go")
	}
	if !foundSub {
		t.Error("Expected to find sub.go in subdirectory")
	}
}

func TestListGoFiles_RespectsGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	goFiles, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listGoFiles failed: %v", err)
	}

	found := false
	for _, f := range goFiles {
		if filepath.Base(f) == "main.go" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find main.go even with go.mod present")
	}
}

func TestSuggestion_KindIcons(t *testing.T) {
	suggestions := []Suggestion{
		{Text: "func", Label: "func", Description: "Function", Kind: "function"},
		{Text: "snippet", Label: "snippet", Description: "Snippet", Kind: "snippet"},
		{Text: "file.go", Label: "file.go", Description: "File", Kind: "file"},
		{Text: "other", Label: "other", Description: "Other", Kind: "other"},
	}

	ShowSuggestions(suggestions)
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("Expected min(1, 2) = 1")
	}
	if min(2, 1) != 1 {
		t.Error("Expected min(2, 1) = 1")
	}
	if min(5, 5) != 5 {
		t.Error("Expected min(5, 5) = 5")
	}
}

func TestSuggester_NewSuggester(t *testing.T) {
	var provider iteragent.Provider
	tools := []iteragent.Tool{}

	s := NewSuggester(provider, tools)
	if s == nil {
		t.Fatal("Expected non-nil Suggester")
	}
}