package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// categorizeJournalEntry returns an emoji based on content analysis.
// 🚀 for feat/implement/add, 🐛 for fix/bug/broken, 📝 for doc/journal, 🔧 for refactor/improve.
func categorizeJournalEntry(content string) string {
	lower := strings.ToLower(content)

	// Check for fix/bug/broken keywords first (high priority)
	if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") ||
		strings.Contains(lower, "broken") || strings.Contains(lower, "revert") {
		return "🐛"
	}

	// Check for feat/implement/add keywords
	if strings.Contains(lower, "feat") || strings.Contains(lower, "implement") ||
		strings.Contains(lower, "add ") || strings.Contains(lower, "feature") {
		return "🚀"
	}

	// Check for doc/journal keywords
	if strings.Contains(lower, "doc") || strings.Contains(lower, "journal") ||
		strings.Contains(lower, "readme") || strings.Contains(lower, "comment") {
		return "📝"
	}

	// Check for refactor/improve keywords
	if strings.Contains(lower, "refactor") || strings.Contains(lower, "improve") ||
		strings.Contains(lower, "cleanup") || strings.Contains(lower, "clean up") ||
		strings.Contains(lower, "optimize") || strings.Contains(lower, "enhance") {
		return "🔧"
	}

	// Default: no emoji
	return ""
}

func (e *Engine) appendJournal(result *RunResult, output, provider string, success bool) {
	path := filepath.Join(e.repoPath, "docs/JOURNAL.md")

	dayCount := 0
	if data, err := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT")); err == nil {
		dayCount, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	title := extractJournalTitle(output, success)
	body := buildJournalBody(output, provider, result.FinishedAt.Sub(result.StartedAt))

	// Determine emoji based on content analysis
	emoji := categorizeJournalEntry(title + " " + body)
	if emoji != "" {
		title = emoji + " " + title
	}

	entry := fmt.Sprintf("\n## Day %d — %s — %s\n\n%s\n",
		dayCount,
		result.StartedAt.Format("15:04"),
		title,
		body,
	)

	existing, _ := os.ReadFile(path)
	header := "# iterate Evolution Journal\n"
	rest := strings.TrimPrefix(string(existing), header)
	newContent := header + entry + rest

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		e.logger.Warn("failed to write journal", "err", err)
	}
}

func extractJournalTitle(output string, success bool) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"feat:", "fix:", "refactor:", "chore:", "docs:", "test:"} {
			if strings.HasPrefix(strings.ToLower(line), prefix) && len(line) < 80 {
				return line
			}
		}
	}
	if success {
		return "evolution session"
	}
	return "session (no changes committed)"
}

func buildJournalBody(output, provider string, duration time.Duration) string {
	lines := []string{fmt.Sprintf("Provider: %s · Duration: %s", provider, duration.Round(time.Second))}

	count := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			continue
		}
		if len(line) > 20 && len(line) < 200 {
			lines = append(lines, line)
			count++
			if count >= 3 {
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractCommitMessage(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"commit:", "feat:", "fix:", "refactor:", "chore:", "docs:"} {
			if strings.HasPrefix(strings.ToLower(line), prefix) {
				return line
			}
		}
	}
	return fmt.Sprintf("iterate: session %s", time.Now().Format("2006-01-02"))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

func extractCommitLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"feat:", "fix:", "refactor:", "chore:", "docs:", "test:"} {
			if strings.HasPrefix(strings.ToLower(line), prefix) && len(line) < 120 {
				lines = append(lines, line)
				break
			}
		}
	}
	return lines
}
