package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestBuildSummarizePrompt_Empty(t *testing.T) {
	result := buildSummarizePrompt(nil)
	if !strings.Contains(result, "empty") {
		t.Errorf("expected 'empty' message, got %q", result)
	}
}

func TestBuildSummarizePrompt_EmptySlice(t *testing.T) {
	result := buildSummarizePrompt([]iteragent.Message{})
	if !strings.Contains(result, "empty") {
		t.Errorf("expected 'empty' message for empty slice, got %q", result)
	}
}

func TestBuildSummarizePrompt_WithMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result := buildSummarizePrompt(msgs)
	if !strings.Contains(result, "2 messages") {
		t.Errorf("should mention message count, got %q", result)
	}
	if !strings.Contains(result, "Summarize") {
		t.Errorf("should contain 'Summarize', got %q", result)
	}
}

func TestBuildSummarizePrompt_ManyMessages(t *testing.T) {
	msgs := make([]iteragent.Message, 10)
	for i := range msgs {
		msgs[i] = iteragent.Message{Role: "user", Content: "msg"}
	}
	result := buildSummarizePrompt(msgs)
	if !strings.Contains(result, "10 messages") {
		t.Errorf("should mention count 10, got %q", result)
	}
}

func TestBuildFixPrompt_ContainsError(t *testing.T) {
	result := buildFixPrompt("undefined: foo")
	if !strings.Contains(result, "undefined: foo") {
		t.Errorf("should contain error text, got %q", result)
	}
	if !strings.Contains(result, "go build") {
		t.Errorf("should mention go build, got %q", result)
	}
	if !strings.Contains(result, "```") {
		t.Errorf("should wrap error in code block, got %q", result)
	}
}

func TestBuildFixPrompt_Multiline(t *testing.T) {
	errText := "error1\nerror2\nerror3"
	result := buildFixPrompt(errText)
	if !strings.Contains(result, "error1") {
		t.Errorf("should contain first error line, got %q", result)
	}
	if !strings.Contains(result, "error3") {
		t.Errorf("should contain last error line, got %q", result)
	}
}

func TestBuildExplainErrorPrompt_Contains(t *testing.T) {
	result := buildExplainErrorPrompt("panic: nil pointer")
	if !strings.Contains(result, "panic: nil pointer") {
		t.Errorf("should contain error text, got %q", result)
	}
	if !strings.Contains(result, "Explain") {
		t.Errorf("should contain 'Explain', got %q", result)
	}
	if !strings.Contains(result, "why it happens") {
		t.Errorf("should ask about cause, got %q", result)
	}
}

func TestBuildExplainErrorPrompt_Empty(t *testing.T) {
	result := buildExplainErrorPrompt("")
	if !strings.Contains(result, "Explain") {
		t.Errorf("should still contain prompt, got %q", result)
	}
}

func TestBuildMockPrompt_ContainsPath(t *testing.T) {
	result := buildMockPrompt("internal/agent/agent.go")
	if !strings.Contains(result, "internal/agent/agent.go") {
		t.Errorf("should contain file path, got %q", result)
	}
	if !strings.Contains(result, "mock") {
		t.Errorf("should mention mocks, got %q", result)
	}
}

func TestBuildMockPrompt_Interface(t *testing.T) {
	result := buildMockPrompt("provider.go")
	if !strings.Contains(result, "interface") {
		t.Errorf("should mention interfaces, got %q", result)
	}
}

func TestBuildDiagramPrompt_WithTempDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
	os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte("package main"), 0o644)

	result := buildDiagramPrompt(dir)
	if !strings.Contains(result, "ASCII") {
		t.Errorf("should mention ASCII, got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("should contain repo index, got %q", result)
	}
}

func TestBuildReadmePrompt_WithTempDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	result := buildReadmePrompt(dir)
	if !strings.Contains(result, "README.md") {
		t.Errorf("should mention README.md, got %q", result)
	}
}

func TestBuildReadmePrompt_WithExistingReadme(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project"), 0o644)

	result := buildReadmePrompt(dir)
	if !strings.Contains(result, "Existing README") {
		t.Errorf("should mention existing README, got %q", result)
	}
	if !strings.Contains(result, "# My Project") {
		t.Errorf("should include existing README content, got %q", result)
	}
}

func TestBuildRepoIndex_WithTempDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme"), 0o644)

	result := buildRepoIndex(dir)
	if !strings.Contains(result, "main.go") {
		t.Errorf("should contain main.go, got %q", result)
	}
	if !strings.Contains(result, "README.md") {
		t.Errorf("should contain README.md, got %q", result)
	}
	if !strings.Contains(result, "src/") {
		t.Errorf("should contain src/ directory, got %q", result)
	}
}

func TestBuildRepoIndex_SkipsHidden(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("config"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	result := buildRepoIndex(dir)
	if strings.Contains(result, ".git") {
		t.Errorf("should skip .git directory, got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("should contain main.go, got %q", result)
	}
}

func TestBuildRepoIndex_SkipsVendor(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "vendor"), 0o755)
	os.WriteFile(filepath.Join(dir, "vendor", "lib.go"), []byte("package lib"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	result := buildRepoIndex(dir)
	if strings.Contains(result, "vendor") {
		t.Errorf("should skip vendor directory, got %q", result)
	}
}

func TestBuildRepoIndex_PreservesIterate(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".iterate"), 0o755)
	os.WriteFile(filepath.Join(dir, ".iterate", "config.json"), []byte("{}"), 0o644)

	result := buildRepoIndex(dir)
	if !strings.Contains(result, ".iterate") {
		t.Errorf("should include .iterate directory, got %q", result)
	}
}

func TestBuildRepoIndex_DepthLimit(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c", "d", "e")
	os.MkdirAll(deep, 0o755)
	os.WriteFile(filepath.Join(deep, "deep.go"), []byte("package deep"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	result := buildRepoIndex(dir)
	if strings.Contains(result, "deep.go") {
		t.Errorf("should skip files deeper than 4 levels, got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("should contain top-level file, got %q", result)
	}
}

func TestBuildChangelogPrompt_NoCommits(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	result := buildChangelogPrompt(dir, "")
	if result == "" {
		t.Error("should return non-empty string")
	}
}

func TestBuildGenerateCommitPrompt_NoDiff(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	result := buildGenerateCommitPrompt(dir)
	if result == "" {
		t.Error("should return non-empty string")
	}
}

func TestBuildReleaseNotes_Empty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	result := buildReleaseNotes(dir, "", "")
	if result == "" {
		t.Error("should return non-empty string")
	}
}
