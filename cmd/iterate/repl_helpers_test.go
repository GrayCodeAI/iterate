package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// containsString
// ---------------------------------------------------------------------------

func TestContainsString_Found(t *testing.T) {
	ss := []string{"apple", "banana", "cherry"}
	if !containsString(ss, "banana") {
		t.Error("expected containsString to return true for 'banana'")
	}
}

func TestContainsString_NotFound(t *testing.T) {
	ss := []string{"apple", "banana", "cherry"}
	if containsString(ss, "grape") {
		t.Error("expected containsString to return false for 'grape'")
	}
}

func TestContainsString_Empty(t *testing.T) {
	if containsString(nil, "anything") {
		t.Error("expected containsString to return false for nil slice")
	}
	if containsString([]string{}, "anything") {
		t.Error("expected containsString to return false for empty slice")
	}
}

func TestContainsString_ExactMatch(t *testing.T) {
	ss := []string{"hello world"}
	if !containsString(ss, "hello world") {
		t.Error("expected exact match")
	}
	if containsString(ss, "hello") {
		t.Error("should not match partial string")
	}
}

func TestContainsString_CaseSensitive(t *testing.T) {
	ss := []string{"Hello"}
	if containsString(ss, "hello") {
		t.Error("should be case-sensitive")
	}
}

// ---------------------------------------------------------------------------
// detectTestCommand
// ---------------------------------------------------------------------------

func TestDetectTestCommand_GoProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "go test ./... -v" {
		t.Errorf("expected 'go test ./... -v', got %q", cmd)
	}
}

func TestDetectTestCommand_RustProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "cargo test" {
		t.Errorf("expected 'cargo test', got %q", cmd)
	}
}

func TestDetectTestCommand_PythonWithPyproject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"test\""), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "pytest" {
		t.Errorf("expected 'pytest', got %q", cmd)
	}
}

func TestDetectTestCommand_PythonWithSetupPy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "setup.py"), []byte("from setuptools import setup"), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "python -m pytest" {
		t.Errorf("expected 'python -m pytest', got %q", cmd)
	}
}

func TestDetectTestCommand_NodeWithTestScript(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{"name": "test", "scripts": {"test": "jest"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "npm test" {
		t.Errorf("expected 'npm test', got %q", cmd)
	}
}

func TestDetectTestCommand_NodeWithoutTestScript(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{"name": "myapp", "version": "1.0.0"}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "node --test" {
		t.Errorf("expected 'node --test', got %q", cmd)
	}
}

func TestDetectTestCommand_Makefile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("test:\n\tgo test"), 0o644)

	cmd := detectTestCommand(dir)
	if cmd != "make test" {
		t.Errorf("expected 'make test', got %q", cmd)
	}
}

func TestDetectTestCommand_Default(t *testing.T) {
	dir := t.TempDir()
	cmd := detectTestCommand(dir)
	if cmd != "go test ./..." {
		t.Errorf("expected 'go test ./...' as default, got %q", cmd)
	}
}

// ---------------------------------------------------------------------------
// replSystemPrompt
// ---------------------------------------------------------------------------

func TestReplSystemPrompt_ContainsBaseText(t *testing.T) {
	dir := t.TempDir()
	prompt := replSystemPrompt(dir)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !containsString([]string{prompt}, "iterate") && len(prompt) == 0 {
		t.Error("prompt should contain 'iterate'")
	}
}

func TestReplSystemPrompt_WithPersonality(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs/PERSONALITY.md"), []byte("Be helpful and concise."), 0o644)

	prompt := replSystemPrompt(dir)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

func TestReplSystemPrompt_WithIterateMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ITERATE.md"), []byte("# Project Context\nThis is a test project."), 0o644)

	prompt := replSystemPrompt(dir)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

func TestReplSystemPrompt_WithProjectMemory(t *testing.T) {
	dir := t.TempDir()
	addProjectMemoryNote(dir, "important note about the project")

	prompt := replSystemPrompt(dir)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

func TestReplSystemPrompt_WithActiveLearnings(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "active_learnings.md"), []byte("Always use Go modules."), 0o644)

	prompt := replSystemPrompt(dir)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

// ---------------------------------------------------------------------------
// fetchOllamaModels (integration - may fail if no Ollama running)
// ---------------------------------------------------------------------------

func TestFetchOllamaModels_InvalidURL(t *testing.T) {
	_, err := fetchOllamaModels("http://localhost:99999/api/tags")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ---------------------------------------------------------------------------
// readActiveLearnings
// ---------------------------------------------------------------------------

func TestReadActiveLearnings_NoMemoryDir(t *testing.T) {
	dir := t.TempDir()
	result := readActiveLearnings(dir)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestReadActiveLearnings_WithActiveLearnings(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "active_learnings.md"), []byte("learning 1\nlearning 2"), 0o644)

	result := readActiveLearnings(dir)
	if result != "learning 1\nlearning 2" {
		t.Errorf("expected 'learning 1\\nlearning 2', got %q", result)
	}
}

func TestReadActiveLearnings_FallbackToLearningsJSONL(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "learnings.jsonl"), []byte(`{"fact":"a"}
{"fact":"b"}
{"fact":"c"}`), 0o644)

	result := readActiveLearnings(dir)
	if result == "" {
		t.Error("expected non-empty result from fallback")
	}
}
