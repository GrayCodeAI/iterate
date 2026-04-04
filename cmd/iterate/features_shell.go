package main

import (
	"bufio"
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
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(result), "\n") {
		lines = append(lines, strings.TrimPrefix(l, repoPath+"/"))
	}
	return strings.Join(lines, "\n"), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
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
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lines := 0
		for scanner.Scan() {
			lines++
		}
		f.Close()
		if scanner.Err() == nil {
			counts[ext] += lines
		}
		return nil
	})
	return counts
}
