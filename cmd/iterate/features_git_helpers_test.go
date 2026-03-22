package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandExists(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"git exists", "git", true},
		{"go exists", "go", true},
		{"nonexistent command", "definitely_not_a_real_command_12345", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commandExists(tt.cmd)
			if got != tt.want {
				t.Errorf("commandExists(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestLanguageBreakdown(t *testing.T) {
	dir := t.TempDir()

	// Create some test files
	files := map[string]string{
		"main.go":   "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
		"util.go":   "package util\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n",
		"readme.md": "# Project\n\nThis is a test project.\n",
		"script.sh": "#!/bin/bash\necho hello\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	result := languageBreakdown(dir)
	if result == "" {
		t.Fatal("languageBreakdown returned empty string")
	}
	// Should contain the total count
	if !contains(result, "Total:") {
		t.Error("expected languageBreakdown output to contain 'Total:'")
	}
	// .go files should be counted
	if !contains(result, ".go") {
		t.Error("expected languageBreakdown to include .go files")
	}
}

func TestLanguageBreakdown_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result := languageBreakdown(dir)
	// Empty dir returns "Total: 0 lines" with no per-language lines
	if !contains(result, "Total: 0") {
		t.Errorf("expected Total: 0 for empty dir, got %q", result)
	}
}

func TestGrepRepo(t *testing.T) {
	dir := t.TempDir()

	// Create a file with searchable content
	content := "package main\n\nfunc hello() {\n\tprintln(\"greetings\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	result, err := grepRepo(dir, "hello")
	if err != nil {
		t.Fatalf("grepRepo: %v", err)
	}
	if !contains(result, "main.go") {
		t.Errorf("expected grepRepo result to reference main.go, got %q", result)
	}
}

func TestGrepRepo_NoMatches(t *testing.T) {
	dir := t.TempDir()

	content := "package main\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := grepRepo(dir, "xyznotfound")
	if err != nil {
		t.Fatalf("grepRepo: %v", err)
	}
	if !contains(result, "No matches") {
		t.Errorf("expected no matches message, got %q", result)
	}
}

func TestCountLines(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"a.go":  "package main\n\nfunc main() {}\n",
		"b.go":  "package util\n\nfunc Add(a, b int) int { return a + b }\n",
		"c.txt": "hello world\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	counts := countLines(dir)
	if counts[".go"] <= 0 {
		t.Error("expected .go line count > 0")
	}
	if counts[".txt"] <= 0 {
		t.Error("expected .txt line count > 0")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
