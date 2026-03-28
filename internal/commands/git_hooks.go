package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RegisterGitHooksCommands adds git hooks integration commands.
// Task 70: Git Hooks Integration (pre-commit, post-commit)
func RegisterGitHooksCommands(r *Registry) {
	r.Register(Command{
		Name:        "/hooks",
		Aliases:     []string{},
		Description: "list or install git hooks",
		Category:    "git",
		Handler:     cmdHooks,
	})
	r.Register(Command{
		Name:        "/hooks-install",
		Aliases:     []string{},
		Description: "install iterate git hooks",
		Category:    "git",
		Handler:     cmdHooksInstall,
	})
	r.Register(Command{
		Name:        "/hooks-remove",
		Aliases:     []string{},
		Description: "remove iterate git hooks",
		Category:    "git",
		Handler:     cmdHooksRemove,
	})
}

const preCommitHook = `#!/bin/sh
# iterate pre-commit hook — run go vet and go fmt check
if command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
    echo "iterate: running go vet..."
    go vet ./... 2>&1
    if [ $? -ne 0 ]; then
        echo "iterate: go vet failed — fix issues before committing"
        exit 1
    fi

    echo "iterate: checking go fmt..."
    UNFMT=$(gofmt -l .)
    if [ -n "$UNFMT" ]; then
        echo "iterate: unformatted files:"
        echo "$UNFMT"
        echo "run: go fmt ./..."
        exit 1
    fi
fi
`

const postCommitHook = `#!/bin/sh
# iterate post-commit hook — log commit to session
COMMIT_MSG=$(git log -1 --pretty=%B)
COMMIT_HASH=$(git log -1 --pretty=%H)
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $COMMIT_HASH $COMMIT_MSG" >> .iterate/commit_log.txt
`

func cmdHooks(ctx Context) Result {
	hooksDir := filepath.Join(ctx.RepoPath, ".git", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		PrintError("cannot read hooks dir: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("%s── Git Hooks ──────────────────────%s\n", ColorDim, ColorReset)
	found := false
	for _, e := range entries {
		if e.IsDir() || e.Name() == "update" || strings.HasSuffix(e.Name(), ".sample") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		isExec := info.Mode()&0o111 != 0
		status := fmt.Sprintf("%s✗ not executable%s", ColorRed, ColorReset)
		if isExec {
			status = fmt.Sprintf("%s✓ active%s", ColorLime, ColorReset)
		}
		fmt.Printf("  %-25s %s\n", e.Name(), status)
		found = true
	}
	if !found {
		fmt.Println("  No custom hooks installed.")
		fmt.Printf("\n  Use /hooks-install to add iterate's pre-commit and post-commit hooks.\n")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdHooksInstall(ctx Context) Result {
	hooksDir := filepath.Join(ctx.RepoPath, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		PrintError("not a git repository (.git/hooks not found)")
		return Result{Handled: true}
	}

	// Install pre-commit hook
	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(preCommitPath, []byte(preCommitHook), 0o755); err != nil {
		PrintError("failed to write pre-commit hook: %v", err)
		return Result{Handled: true}
	}
	fmt.Printf("  %s✓%s Installed pre-commit hook (go vet + go fmt)\n", ColorLime, ColorReset)

	// Install post-commit hook
	postCommitPath := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(postCommitHook), 0o755); err != nil {
		PrintError("failed to write post-commit hook: %v", err)
		return Result{Handled: true}
	}
	fmt.Printf("  %s✓%s Installed post-commit hook (commit log)\n", ColorLime, ColorReset)

	// Ensure .iterate directory exists
	_ = os.MkdirAll(filepath.Join(ctx.RepoPath, ".iterate"), 0o755)

	fmt.Println()
	PrintSuccess("hooks installed")
	return Result{Handled: true}
}

func cmdHooksRemove(ctx Context) Result {
	hooksDir := filepath.Join(ctx.RepoPath, ".git", "hooks")
	removed := 0
	for _, name := range []string{"pre-commit", "post-commit"} {
		path := filepath.Join(hooksDir, name)
		if _, err := os.Stat(path); err == nil {
			// Check if it's our hook
			data, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(data), "iterate") {
				os.Remove(path)
				fmt.Printf("  %s✓%s Removed %s\n", ColorLime, ColorReset, name)
				removed++
			}
		}
	}
	if removed == 0 {
		fmt.Println("No iterate hooks found to remove.")
	} else {
		PrintSuccess("removed %d hooks", removed)
	}
	return Result{Handled: true}
}

// gitHookExists checks if a specific hook is installed and executable.
func gitHookExists(repoPath, hookName string) bool {
	path := filepath.Join(repoPath, ".git", "hooks", hookName)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

// runGitHook runs a git hook and returns its output.
func runGitHook(repoPath, hookName string) (string, error) {
	path := filepath.Join(repoPath, ".git", "hooks", hookName)
	cmd := exec.Command(path)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}
