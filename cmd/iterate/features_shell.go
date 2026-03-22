package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// /grep — content search across the repo
// ---------------------------------------------------------------------------

func grepRepo(repoPath, pattern string) (string, error) {
	cmd := exec.Command("grep", "-rn", "--include=*.go", "--include=*.md",
		"--include=*.json", "--include=*.sh", "--include=*.yaml", "--include=*.yml",
		"-m", "5", pattern, repoPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run() // ignore exit code (1 = no matches)
	result := out.String()
	if result == "" {
		return fmt.Sprintf("No matches for %q", pattern), nil
	}
	// Make paths relative
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(result), "\n") {
		lines = append(lines, strings.TrimPrefix(l, repoPath+"/"))
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------------------
// /copy — copy text to clipboard (macOS pbcopy, Linux xclip)
// ---------------------------------------------------------------------------

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch {
	case commandExists("pbcopy"):
		cmd = exec.Command("pbcopy")
	case commandExists("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case commandExists("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--input")
	default:
		return fmt.Errorf("no clipboard tool found (pbcopy/xclip/xsel)")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ---------------------------------------------------------------------------
// /coverage — test coverage report
// ---------------------------------------------------------------------------

func runCoverage(repoPath string) (string, error) {
	cmd := exec.Command("go", "test", "-coverprofile=/tmp/iterate-cover.out", "./...")
	cmd.Dir = repoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	// Summary
	summary := exec.Command("go", "tool", "cover", "-func=/tmp/iterate-cover.out")
	summary.Dir = repoPath
	sumOut, err := summary.Output()
	if err != nil {
		return out.String(), nil
	}
	return string(sumOut), nil
}

// ---------------------------------------------------------------------------
// /languages — file extension breakdown
// ---------------------------------------------------------------------------

func languageBreakdown(repoPath string) string {
	counts := countLines(repoPath)
	type entry struct {
		ext   string
		lines int
	}
	var entries []entry
	for ext, n := range counts {
		entries = append(entries, entry{ext, n})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lines > entries[j].lines
	})
	total := 0
	for _, e := range entries {
		total += e.lines
	}
	var lines []string
	for _, e := range entries {
		if e.lines < 10 {
			continue
		}
		pct := float64(e.lines) / float64(total) * 100
		bar := strings.Repeat("█", int(pct/2))
		lines = append(lines, fmt.Sprintf("  %-8s %5d  %s %.0f%%", e.ext, e.lines, bar, pct))
	}
	return fmt.Sprintf("Total: %d lines\n%s", total, strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// /count-lines — lines of code breakdown
// ---------------------------------------------------------------------------

func countLines(repoPath string) map[string]int {
	counts := map[string]int{}
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Count(string(data), "\n") + 1
		counts[ext] += lines
		return nil
	})
	return counts
}

// ---------------------------------------------------------------------------
// /benchmark
// ---------------------------------------------------------------------------

func runBenchmark(repoPath, pkg string) (string, error) {
	args := []string{"test", "-bench=.", "-benchmem", "-run=^$"}
	if pkg != "" {
		args = append(args, pkg)
	} else {
		args = append(args, "./...")
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------------
// /env — show/set environment variables
// ---------------------------------------------------------------------------

func showEnv(filter string) string {
	var lines []string
	for _, e := range os.Environ() {
		if filter == "" || strings.Contains(strings.ToUpper(e), strings.ToUpper(filter)) {
			lines = append(lines, e)
		}
	}
	// Sort
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /paste — paste from clipboard into context
// ---------------------------------------------------------------------------

func pasteFromClipboard() (string, error) {
	var cmd *exec.Cmd
	switch {
	case commandExists("pbpaste"):
		cmd = exec.Command("pbpaste")
	case commandExists("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard", "-out")
	case commandExists("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--output")
	default:
		return "", fmt.Errorf("no clipboard tool found (pbpaste/xclip/xsel)")
	}
	out, err := cmd.Output()
	return string(out), err
}

// ---------------------------------------------------------------------------
// /open — open file in $EDITOR
// ---------------------------------------------------------------------------

func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"nvim", "vim", "nano", "vi"} {
			if commandExists(e) {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found — set $EDITOR")
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /squash — squash last N commits into one
// ---------------------------------------------------------------------------

func squashCommits(repoPath string, n int, msg string) error {
	// Soft reset N commits back
	cmd := exec.Command("git", "-C", repoPath, "reset", "--soft", fmt.Sprintf("HEAD~%d", n))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	commitCmd := exec.Command("git", "-C", repoPath, "commit", "-m", msg)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	return commitCmd.Run()
}

// ---------------------------------------------------------------------------
// /journal — view/tail JOURNAL.md
// ---------------------------------------------------------------------------

func viewJournal(repoPath string, lines int) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "docs/docs/JOURNAL.md"))
	if err != nil {
		return "JOURNAL.md not found."
	}
	all := strings.Split(string(data), "\n")
	if lines > 0 && len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

// ---------------------------------------------------------------------------
// /skill-create — scaffold a new skill file
// ---------------------------------------------------------------------------

func scaffoldSkill(repoPath, name, description string) (string, error) {
	dir := filepath.Join(repoPath, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	content := fmt.Sprintf(`---
name: %s
description: %s
tools: [bash, read_file, write_file, edit_file]
---

# %s

## Overview

Describe what this skill does.

## Steps

1. Step one
2. Step two
3. Step three

## Notes

- Add any special considerations here.
`, name, description, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ---------------------------------------------------------------------------
// /ci — show GitHub Actions run status
// ---------------------------------------------------------------------------

func getCIStatus(repoPath string) (string, error) {
	out, err := exec.Command("gh", "run", "list", "--limit", "5",
		"--json", "status,conclusion,name,createdAt,url",
		"--template", `{{range .}}{{.status}} {{.conclusion}} {{.name}} {{.createdAt}}{{"\n"}}{{end}}`).Output()
	if err != nil {
		return "", fmt.Errorf("gh run list: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------------------
// /view — syntax-highlighted file viewer
// ---------------------------------------------------------------------------

func viewFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// /verify — run all quality checks
// ---------------------------------------------------------------------------

type verifyResult struct {
	name   string
	ok     bool
	output string
}

func runVerify(repoPath string) []verifyResult {
	type check struct {
		name string
		args []string
	}
	checks := []check{
		{"build", []string{"go", "build", "./..."}},
		{"vet", []string{"go", "vet", "./..."}},
		{"test", []string{"go", "test", "./..."}},
		{"fmt", []string{"gofmt", "-l", "."}},
	}
	var results []verifyResult
	for _, c := range checks {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		detail := strings.TrimSpace(string(out))
		// gofmt: non-empty output means files need formatting
		ok := err == nil
		if c.name == "fmt" {
			ok = detail == ""
			if !ok {
				detail = "needs formatting: " + detail
			} else {
				detail = "all files formatted"
			}
		}
		if len(detail) > 100 {
			detail = detail[:100] + "…"
		}
		results = append(results, verifyResult{c.name, ok, detail})
	}
	return results
}

// ---------------------------------------------------------------------------
// /doctor — project health check
// ---------------------------------------------------------------------------

type healthResult struct {
	check  string
	ok     bool
	detail string
}

func runDoctor(repoPath string) []healthResult {
	var results []healthResult
	run := func(check, name string, args ...string) healthResult {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		detail := strings.TrimSpace(string(out))
		if len(detail) > 80 {
			detail = detail[:80] + "…"
		}
		return healthResult{check: check, ok: err == nil, detail: detail}
	}

	results = append(results, run("go build", "go", "build", "./..."))
	results = append(results, run("go vet", "go", "vet", "./..."))
	results = append(results, run("go test", "go", "test", "-count=1", "-timeout=30s", "./..."))

	// Check go.sum is up to date
	modCmd := exec.Command("go", "mod", "verify")
	modCmd.Dir = repoPath
	out, err := modCmd.CombinedOutput()
	results = append(results, healthResult{
		check: "go mod verify", ok: err == nil,
		detail: strings.TrimSpace(string(out)),
	})

	// Git clean?
	statusOut, _ := exec.Command("git", "-C", repoPath, "status", "--short").Output()
	dirty := strings.TrimSpace(string(statusOut)) != ""
	detail := "working tree clean"
	if dirty {
		detail = "uncommitted changes present"
	}
	results = append(results, healthResult{check: "git clean", ok: !dirty, detail: detail})

	return results
}

// ---------------------------------------------------------------------------
// /pr-checkout — checkout a PR locally
// ---------------------------------------------------------------------------

func prCheckout(repoPath string, prNum string) error {
	cmd := exec.Command("gh", "pr", "checkout", prNum)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /gist — create a GitHub gist from a file or stdin
// ---------------------------------------------------------------------------

func createGist(content, filename, description string, public bool) (string, error) {
	tmp, err := os.CreateTemp("", "iterate-gist-*."+filepath.Ext(filename))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(content)
	tmp.Close()

	args := []string{"gist", "create", tmp.Name(),
		"--filename", filename,
		"--desc", description}
	if public {
		args = append(args, "--public")
	}
	out, err := exec.Command("gh", args...).Output()
	return strings.TrimSpace(string(out)), err
}
