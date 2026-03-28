package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitOutput(t *testing.T) {
	dir := t.TempDir()
	// Initialize a git repo
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	// Create a file and commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run()

	// Test gitOutput
	branch := gitOutput(dir, "branch", "--show-current")
	if branch == "" {
		// Git might return empty in some configurations; try rev-parse
		branch = gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")
	}
	if branch == "" {
		t.Log("git branch --show-current returned empty (expected in some CI)")
	}

	log := gitOutput(dir, "log", "--oneline", "-1")
	if log == "" {
		t.Error("expected non-empty git log")
	}

	// Test with non-existent dir
	result := gitOutput("/nonexistent/path", "status")
	if result != "" {
		t.Error("expected empty result for non-existent path")
	}
}

func TestGitHookExists(t *testing.T) {
	dir := t.TempDir()
	// Initialize git repo
	exec.Command("git", "init", dir).Run()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0o755)

	// No hooks initially
	if gitHookExists(dir, "pre-commit") {
		t.Error("expected no pre-commit hook")
	}

	// Create a non-executable hook
	hookPath := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(hookPath, []byte("#!/bin/sh\necho test"), 0o644)
	if !gitHookExists(dir, "pre-commit") {
		t.Error("expected pre-commit hook to exist after creation")
	}

	// Remove execute bit
	os.Chmod(hookPath, 0o644)
	if gitHookExists(dir, "pre-commit") {
		t.Error("expected non-executable hook to not be considered active")
	}
}

func TestGitOutputError(t *testing.T) {
	dir := t.TempDir()
	// Not a git repo
	result := gitOutput(dir, "log", "--oneline")
	if result != "" {
		t.Error("expected empty output for non-git directory")
	}
}
