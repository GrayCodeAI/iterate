package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitOutput(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial").Run()

	log := gitOutput(dir, "log", "--oneline", "-1")
	if log == "" {
		t.Error("expected non-empty git log")
	}

	result := gitOutput("/nonexistent/path", "status")
	if result != "" {
		t.Error("expected empty result for non-existent path")
	}
}

func TestGitHookExists(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0o755)

	if gitHookExists(dir, "pre-commit") {
		t.Error("expected no pre-commit hook initially")
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(hookPath, []byte("#!/bin/sh\necho test"), 0o755)
	if !gitHookExists(dir, "pre-commit") {
		t.Error("expected executable hook to exist")
	}

	os.Chmod(hookPath, 0o644)
	if gitHookExists(dir, "pre-commit") {
		t.Error("expected non-executable hook to not be active")
	}
}
