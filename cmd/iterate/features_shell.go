package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	sort.Strings(lines)
	return strings.Join(lines, "\n")
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

	modCmd := exec.Command("go", "mod", "verify")
	modCmd.Dir = repoPath
	out, err := modCmd.CombinedOutput()
	results = append(results, healthResult{
		check: "go mod verify", ok: err == nil,
		detail: strings.TrimSpace(string(out)),
	})

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
// /web — fetch a URL and return readable text
// ---------------------------------------------------------------------------

func fetchURL(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", err
	}
	text := string(body)
	if strings.Contains(resp.Header.Get("Content-Type"), "html") {
		text = stripHTMLTags(text)
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			b.WriteByte(' ')
			continue
		}
		if !inTag {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// /find — fuzzy file search
// ---------------------------------------------------------------------------

func findFiles(repoPath, pattern string) []string {
	pattern = strings.ToLower(pattern)
	var results []string
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
		rel, _ := filepath.Rel(repoPath, path)
		if strings.Contains(strings.ToLower(rel), pattern) {
			results = append(results, rel)
		}
		return nil
	})
	return results
}
