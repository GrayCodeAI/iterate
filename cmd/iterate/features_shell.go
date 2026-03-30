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
	"sync"

	"golang.org/x/sync/errgroup"
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

// countLines counts lines per file extension in repoPath.
// Uses concurrent processing for large repos (100+ files).
func countLines(repoPath string) map[string]int {
	counts := map[string]int{}
	var mu sync.Mutex

	// Channel to collect files that need counting
	type fileInfo struct {
		path string
		ext  string
	}
	files := make(chan fileInfo, 100)

	// Start worker goroutines - use GOMAXPROCS workers
	var g errgroup.Group
	// Limit concurrency to avoid overwhelming the system
	g.SetLimit(4)

	// Worker function
	worker := func() error {
		for fi := range files {
			data, err := os.ReadFile(fi.path)
			if err != nil {
				continue
			}
			// Count newlines without string allocation
			lines := bytes.Count(data, []byte{'\n'}) + 1
			mu.Lock()
			counts[fi.ext] += lines
			mu.Unlock()
		}
		return nil
	}

	// Start workers
	for i := 0; i < 4; i++ {
		g.Go(worker)
	}

	// Walk directory and send files to workers
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
		files <- fileInfo{path: path, ext: ext}
		return nil
	})

	close(files)
	_ = g.Wait()

	return counts
}
