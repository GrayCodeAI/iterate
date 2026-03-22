package main

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GrayCodeAI/iterate/internal/commands"
)

// ---------------------------------------------------------------------------
// /find — fuzzy file search
// ---------------------------------------------------------------------------

// findFiles returns files in repoPath whose relative path contains the pattern (case-insensitive).
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024)) // 256 KB max
	if err != nil {
		return "", err
	}
	text := string(body)
	// Strip HTML tags very simply for readability
	if strings.Contains(resp.Header.Get("Content-Type"), "html") {
		text = commands.StripHTMLTags(text)
	}
	// Trim excessive blank lines
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------------------
// Memory helpers — /learn and /memories
// ---------------------------------------------------------------------------

func readActiveLearnings(repoPath string) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "memory", "active_learnings.md"))
	if err != nil {
		// Fall back to last 10 lines of learnings.jsonl
		raw, err2 := os.ReadFile(filepath.Join(repoPath, "memory", "learnings.jsonl"))
		if err2 != nil {
			return ""
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		if len(lines) > 10 {
			lines = lines[len(lines)-10:]
		}
		return strings.Join(lines, "\n")
	}
	return string(data)
}
