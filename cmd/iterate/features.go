package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/ui/highlight"
)

// ---------------------------------------------------------------------------
// /context — show current context stats
// ---------------------------------------------------------------------------

func contextStats(messages []iteragent.Message) string {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	approxTokens := totalChars / 4
	win := highlight.ContextWindow
	if win == 0 {
		win = 200000
	}
	pct := float64(approxTokens) / float64(win) * 100
	return fmt.Sprintf("Messages: %d  |  ~%d tokens  |  ~%.0f%% of context window",
		len(messages), approxTokens, pct)
}

// ---------------------------------------------------------------------------
// /export — export conversation to markdown
// ---------------------------------------------------------------------------
// /export — export conversation to markdown
// ---------------------------------------------------------------------------

func exportConversation(messages []iteragent.Message, path string) error {
	var b strings.Builder
	b.WriteString("# iterate — Conversation Export\n\n")
	b.WriteString(fmt.Sprintf("Exported: %s\n\n---\n\n", time.Now().Format("2006-01-02 15:04:05")))
	for _, m := range messages {
		var role string
		if len(m.Role) > 0 {
			role = strings.ToUpper(m.Role[:1]) + m.Role[1:]
		} else {
			role = "unknown"
		}
		b.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", role, m.Content))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ---------------------------------------------------------------------------
// /todos — find TODO/FIXME/HACK comments in the codebase
// ---------------------------------------------------------------------------

func findTodos(repoPath string) []string {
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
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".md" && ext != ".sh" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		lineNum := 0
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			lineNum++
			line := sc.Text()
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") || strings.Contains(upper, "HACK") {
				trimmed := strings.TrimSpace(line)
				results = append(results, fmt.Sprintf("%s:%d  %s", rel, lineNum, trimmed))
			}
		}
		f.Close()
		return nil
	})
	return results
}

// ---------------------------------------------------------------------------
// /search-replace — find and replace text across all Go files
// ---------------------------------------------------------------------------

func searchReplace(repoPath, oldText, newText string) (int, error) {
	count := 0
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".md" && ext != ".sh" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(data), oldText) {
			return nil
		}
		newData := strings.ReplaceAll(string(data), oldText, newText)
		if err := os.WriteFile(path, []byte(newData), 0o644); err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoPath, path)
		fmt.Printf("  %s%s%s\n", colorDim, rel, colorReset)
		count++
		return nil
	})
	return count, err
}

// ---------------------------------------------------------------------------
// Memory helpers — /learn and /memories
// ---------------------------------------------------------------------------

func appendLearning(repoPath, fact string) error {
	path := filepath.Join(repoPath, "memory", "learnings.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry := map[string]string{
		"fact":       fact,
		"created_at": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(entry)
	_, err = fmt.Fprintln(f, string(data))
	return err
}

// ---------------------------------------------------------------------------
// /memo — append to JOURNAL.md
// ---------------------------------------------------------------------------

func appendMemo(repoPath, text string) error {
	path := filepath.Join(repoPath, "docs/JOURNAL.md")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## Memo — %s\n\n%s\n", time.Now().Format("2006-01-02 15:04"), text)
	return err
}

// ---------------------------------------------------------------------------
// /compact-hard — very aggressive context compaction
// ---------------------------------------------------------------------------

func compactHard(messages []iteragent.Message, keepLast int) []iteragent.Message {
	if len(messages) <= keepLast {
		return messages
	}
	// Keep system context (first 2) + last keepLast messages.
	// Ensure tail start is past the head to avoid duplicates when keepLast overlaps.
	var out []iteragent.Message
	if len(messages) > 2 {
		out = append(out, messages[:2]...)
	}
	tailStart := len(messages) - keepLast
	if tailStart < 2 {
		tailStart = 2
	}
	tail := messages[tailStart:]
	out = append(out, tail...)
	return out
}

// ---------------------------------------------------------------------------
// /pin-list
// ---------------------------------------------------------------------------

func formatPinnedMessages(msgs []iteragent.Message) string {
	if len(msgs) == 0 {
		return "No pinned messages."
	}
	var lines []string
	for i, m := range msgs {
		snippet := m.Content
		if len(snippet) > 80 {
			snippet = snippet[:80] + "…"
		}
		lines = append(lines, fmt.Sprintf("  %d  [%s] %s", i+1, m.Role, snippet))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /init — scaffold a new iterate-enabled project
// ---------------------------------------------------------------------------

func initProject(repoPath, projectName string) []string {
	files := map[string]string{
		"docs/IDENTITY.md":       fmt.Sprintf("# %s\n\nA self-evolving project powered by iterate.\n", projectName),
		"docs/PERSONALITY.md":    "Helpful, concise, and direct.\n",
		"docs/JOURNAL.md":        fmt.Sprintf("# iterate Evolution Journal\n\n## Day 1 — %s\n\nProject initialized.\n", time.Now().Format("2006-01-02")),
		"DAY_COUNT":              "1",
		"memory/learnings.jsonl": "",
		"skills/.keep":           "",
	}
	var created []string
	for path, content := range files {
		full := filepath.Join(repoPath, path)
		if _, err := os.Stat(full); err == nil {
			continue // already exists
		}
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err == nil {
			created = append(created, path)
		}
	}
	return created
}
