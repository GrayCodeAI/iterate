package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// fetchOllamaModels fetches the list of model names from an Ollama /api/tags endpoint.
func fetchOllamaModels(tagsURL string) ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(tagsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

// showGitDiff runs git diff and prints colored output if there are changes.
func showGitDiff(repoPath string) {
	cmd := exec.Command("git", "diff", "--color=always", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		// Try unstaged diff
		cmd2 := exec.Command("git", "diff", "--color=always")
		cmd2.Dir = repoPath
		out, err = cmd2.Output()
	}
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	fmt.Printf("\n%s── diff ──────────────────────────%s\n", colorDim, colorReset)
	fmt.Print(string(out))
	fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// detectTestCommand returns the appropriate test command based on project type.
// Supports: Go, Rust, Python, Node.js, and falls back to 'make test'.
func detectTestCommand(repoPath string) string {
	// Check for Go project
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		return "go test ./... -v"
	}
	// Check for Rust project
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		return "cargo test"
	}
	// Check for Python project
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		return "pytest"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "setup.py")); err == nil {
		return "python -m pytest"
	}
	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		// Check for npm test script
		pkgJSON, _ := os.ReadFile(filepath.Join(repoPath, "package.json"))
		if bytes.Contains(pkgJSON, []byte(`"test"`)) {
			return "npm test"
		}
		return "node --test"
	}
	// Check for Makefile
	if _, err := os.Stat(filepath.Join(repoPath, "Makefile")); err == nil {
		return "make test"
	}
	// Default fallback
	return "go test ./..."
}

func runShell(repoPath string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("exit: %v\n", err)
	}
}

func replSystemPrompt(repoPath string) string {
	personality, _ := os.ReadFile(filepath.Join(repoPath, "docs/docs/PERSONALITY.md"))

	base := "You are iterate, a self-evolving coding agent built by GrayCodeAI.\n"
	base += "You are a coding agent — never describe yourself as a 'Go coding agent'.\n"
	base += "Help the user with coding tasks, answer questions, and use tools when needed.\n"
	base += "Keep responses concise and direct. Do not add journals, logs, or internal monologue.\n"
	base += "NEVER narrate what you are about to do. Never say 'Let me check', 'I'll look at', 'Let me read' or similar. Answer directly.\n"
	if len(personality) > 0 {
		base += "\n## Personality\n" + string(personality)
	}

	// Inject project memory notes (per-project .iterate/memory.json)
	if mem := loadProjectMemory(repoPath); len(mem.Entries) > 0 {
		base += "\n" + formatProjectMemoryForPrompt(mem)
	}

	// Inject active learnings (evolution memory)
	if learnings := readActiveLearnings(repoPath); learnings != "" {
		base += "\n## Active Learnings\n" + learnings + "\n"
	}

	// Inject ITERATE.md if present
	if iterateMD, err := os.ReadFile(filepath.Join(repoPath, "ITERATE.md")); err == nil {
		base += "\n## Project Context (ITERATE.md)\n" + string(iterateMD)
	}

	if index := buildRepoIndex(repoPath); index != "" {
		base += "\n## Repo structure\n```\n" + index + "\n```\n"
	}
	return base
}
