package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestBuildRepoIndex(t *testing.T) {
	index := buildRepoIndex(".")
	if len(index) == 0 {
		t.Errorf("repo index should not be empty")
	}
}

func TestContextBar(t *testing.T) {
	bar := contextBar([]iteragent.Message{}, 4000)
	if len(bar) == 0 {
		t.Errorf("context bar should not be empty")
	}
}

func TestBuildPrompts(t *testing.T) {
	prompts := []string{
		buildFixPrompt("error: undefined variable"),
		buildExplainErrorPrompt("panic: nil pointer"),
		buildGenerateCommitPrompt("."),
		buildReviewPrompt("."),
		buildSummarizePrompt([]iteragent.Message{}),
	}

	for i, prompt := range prompts {
		if len(prompt) == 0 {
			t.Errorf("prompt %d should not be empty", i)
		}
	}
}

func TestBuildDiagramPrompt(t *testing.T) {
	prompt := buildDiagramPrompt(".")
	if !strings.Contains(prompt, "diagram") && !strings.Contains(prompt, "ASCII") {
		t.Errorf("diagram prompt should mention diagrams")
	}
}

func TestBuildReadmePrompt(t *testing.T) {
	prompt := buildReadmePrompt(".")
	if !strings.Contains(prompt, "README") {
		t.Errorf("readme prompt should mention README")
	}
}

// ---------------------------------------------------------------------------
// wrapToolsWithPermissions integration tests
// ---------------------------------------------------------------------------

// withTempConfig writes a TOML config to a temp XDG_CONFIG_HOME directory and
// returns a cleanup function that restores the original env var.
func withTempConfig(t *testing.T, tomlContent string) func() {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "iterate")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	prev := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	return func() {
		if prev == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", prev)
		}
	}
}

// TestWrapToolsWithPermissions_DirDenied verifies that a file tool targeting a
// denied directory returns "Access denied" without invoking the original Execute.
func TestWrapToolsWithPermissions_DirDenied(t *testing.T) {
	restore := withTempConfig(t, `deny_dirs = ["/secret"]`)
	defer restore()

	called := false
	tool := iteragent.Tool{
		Name:        "write_file",
		Description: "write",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			called = true
			return "wrote", nil
		},
	}

	wrapped := wrapToolsWithPermissions([]iteragent.Tool{tool})
	result, err := wrapped[0].Execute(context.Background(), map[string]string{"path": "/secret/file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Access denied") {
		t.Errorf("expected Access denied, got %q", result)
	}
	if called {
		t.Error("original Execute should not have been called for a denied path")
	}
}

// TestWrapToolsWithPermissions_DirAllowed verifies that a file tool targeting an
// allowed directory proceeds to call the original Execute.
func TestWrapToolsWithPermissions_DirAllowed(t *testing.T) {
	restore := withTempConfig(t, `allow_dirs = ["/allowed"]`)
	defer restore()

	called := false
	tool := iteragent.Tool{
		Name:        "read_file",
		Description: "read",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			called = true
			return "content", nil
		},
	}

	wrapped := wrapToolsWithPermissions([]iteragent.Tool{tool})
	result, err := wrapped[0].Execute(context.Background(), map[string]string{"path": "/allowed/file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("original Execute should have been called for an allowed path")
	}
	if result != "content" {
		t.Errorf("expected %q, got %q", "content", result)
	}
}

// TestWrapToolsWithPermissions_NoDirRestriction verifies that without any
// directory config, all paths are permitted.
func TestWrapToolsWithPermissions_NoDirRestriction(t *testing.T) {
	restore := withTempConfig(t, ``) // empty config
	defer restore()

	called := false
	tool := iteragent.Tool{
		Name:        "write_file",
		Description: "write",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			called = true
			return "ok", nil
		},
	}

	wrapped := wrapToolsWithPermissions([]iteragent.Tool{tool})
	_, err := wrapped[0].Execute(context.Background(), map[string]string{"path": "/anywhere/file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("original Execute should have been called when no dir restrictions are configured")
	}
}
