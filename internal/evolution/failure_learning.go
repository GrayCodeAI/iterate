package evolution

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type FailurePattern struct {
	Pattern      string   `json:"pattern"`
	Category     string   `json:"category"`
	Count        int      `json:"count"`
	FirstSeen    string   `json:"first_seen"`
	LastSeen     string   `json:"last_seen"`
	Suggestions  []string `json:"suggestions"`
	ExampleTasks []string `json:"example_tasks"`
}

type FailureAnalyzer struct {
	repoPath string
	logger   *slog.Logger
	patterns map[string]*FailurePattern
}

func NewFailureAnalyzer(repoPath string, logger *slog.Logger) *FailureAnalyzer {
	return &FailureAnalyzer{
		repoPath: repoPath,
		logger:   logger,
		patterns: make(map[string]*FailurePattern),
	}
}

var (
	categoryPatterns = map[string][]string{
		"build_error": {
			"build failed",
			"compilation error",
			"cannot find package",
			"undefined:",
			"syntax error",
			"undefined reference",
		},
		"test_failure": {
			"test failed",
			"FAIL:",
			"Assertion failed",
			"expected:",
			"got:",
		},
		"api_error": {
			"rate limit",
			"api error",
			"429",
			"authentication failed",
			"unauthorized",
		},
		"timeout": {
			"timeout",
			"timed out",
			"context deadline exceeded",
			"deadline exceeded",
		},
		"git_error": {
			"git conflict",
			"merge conflict",
			"CONFLICT",
			"failed to push",
			"rejected",
		},
		"parsing_error": {
			"parse error",
			"invalid format",
			"malformed",
			"unexpected token",
		},
		"diff_error": {
			"failed to apply",
			"patch failed",
			"unified diff",
			"hunk failed",
		},
	}
)

func (fa *FailureAnalyzer) AnalyzeFailures() ([]*FailurePattern, error) {
	failures, err := fa.loadFailures()
	if err != nil {
		return nil, err
	}

	if len(failures) == 0 {
		return []*FailurePattern{}, nil
	}

	fa.identifyPatterns(failures)
	fa.generateSuggestions()

	var patterns []*FailurePattern
	for _, p := range fa.patterns {
		patterns = append(patterns, p)
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})

	return patterns, nil
}

func (fa *FailureAnalyzer) loadFailures() ([]FailureEntry, error) {
	path := filepath.Join(fa.repoPath, "memory", "failures.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var failures []FailureEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}

		var entry FailureEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fa.logger.Warn("Failed to parse failure entry", "line", line[:min(len(line), 100)], "err", err)
			continue
		}
		failures = append(failures, entry)
	}

	return failures, nil
}

func (fa *FailureAnalyzer) identifyPatterns(failures []FailureEntry) {
	fa.patterns = make(map[string]*FailurePattern)

	for _, failure := range failures {
		key := fa.categorizeFailure(failure.Reason)
		if key == "" {
			key = "unknown"
		}

		reasonLower := strings.ToLower(failure.Reason)
		taskLower := strings.ToLower(failure.Task)

		combinedText := reasonLower + " " + taskLower
		pattern := fa.extractPattern(combinedText)

		if pattern == "" {
			pattern = key
		}

		if existing, ok := fa.patterns[pattern]; ok {
			existing.Count++
			if len(existing.ExampleTasks) < 3 {
				existing.ExampleTasks = append(existing.ExampleTasks, failure.Task)
			}
			if failure.TS < existing.FirstSeen {
				existing.FirstSeen = failure.TS
			}
			if failure.TS > existing.LastSeen {
				existing.LastSeen = failure.TS
			}
		} else {
			fa.patterns[pattern] = &FailurePattern{
				Pattern:      pattern,
				Category:     key,
				Count:        1,
				FirstSeen:    failure.TS,
				LastSeen:     failure.TS,
				Suggestions:  []string{},
				ExampleTasks: []string{failure.Task},
			}
		}
	}
}

func (fa *FailureAnalyzer) categorizeFailure(reason string) string {
	reasonLower := strings.ToLower(reason)

	for category, keywords := range categoryPatterns {
		for _, keyword := range keywords {
			if strings.Contains(reasonLower, keyword) {
				return category
			}
		}
	}

	return ""
}

func (fa *FailureAnalyzer) extractPattern(text string) string {
	patterns := []struct {
		regex *regexp.Regexp
	}{
		{regexp.MustCompile(`(\w+Error):\s*(.+)`)},
		{regexp.MustCompile(`failed to (\w+)`)},
		{regexp.MustCompile(`error.*?(\w+)`)},
		{regexp.MustCompile(`cannot (\w+)`)},
	}

	for _, p := range patterns {
		match := p.regex.FindStringSubmatch(text)
		if len(match) > 1 {
			return match[1]
		}
	}

	words := strings.Fields(text)
	if len(words) > 0 {
		return words[0]
	}

	return ""
}

func (fa *FailureAnalyzer) generateSuggestions() {
	for _, pattern := range fa.patterns {
		switch pattern.Category {
		case "build_error":
			pattern.Suggestions = []string{
				"Run `go build ./...` before committing to catch compilation errors",
				"Check for missing imports or type errors",
				"Use go vet to catch common issues",
			}
		case "test_failure":
			pattern.Suggestions = []string{
				"Run tests locally before pushing: `go test ./...`",
				"Check test assertions match actual behavior",
				"Ensure test setup/teardown is correct",
			}
		case "api_error":
			pattern.Suggestions = []string{
				"Implement exponential backoff for API calls",
				"Add rate limiting to prevent 429 errors",
				"Cache responses when possible",
			}
		case "timeout":
			pattern.Suggestions = []string{
				"Increase timeout values for long operations",
				"Implement retry with backoff",
				"Break large operations into smaller chunks",
			}
		case "git_error":
			pattern.Suggestions = []string{
				"Pull latest changes before pushing",
				"Resolve merge conflicts carefully",
				"Use `git status` to check for uncommitted changes",
			}
		case "diff_error":
			pattern.Suggestions = []string{
				"Use unified diff format for all changes",
				"Verify diffs apply cleanly before submitting",
				"Check for whitespace changes that may cause issues",
			}
		default:
			pattern.Suggestions = []string{
				"Analyze the specific error message for clues",
				"Search for similar issues in the codebase",
				"Break down the task into smaller steps",
			}
		}
	}
}

func (fa *FailureAnalyzer) GetPatternSummary() string {
	patterns, err := fa.AnalyzeFailures()
	if err != nil {
		return ""
	}

	if len(patterns) == 0 {
		return "No failure patterns detected yet."
	}

	var sb strings.Builder
	sb.WriteString("## Failure Pattern Analysis\n\n")

	for i, p := range patterns {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("### %s (%d occurrences)\n", p.Pattern, p.Count))
		sb.WriteString(fmt.Sprintf("Category: %s\n", p.Category))
		sb.WriteString(fmt.Sprintf("First seen: %s\n", p.FirstSeen))
		sb.WriteString(fmt.Sprintf("Last seen: %s\n\n", p.LastSeen))

		if len(p.Suggestions) > 0 {
			sb.WriteString("Suggestions:\n")
			for _, s := range p.Suggestions {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (fa *FailureAnalyzer) SavePatterns() error {
	patterns, err := fa.AnalyzeFailures()
	if err != nil {
		return err
	}

	path := filepath.Join(fa.repoPath, "memory", "failure_patterns.json")
	data, err := json.MarshalIndent(patterns, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (fa *FailureAnalyzer) GetAvoidanceGuidelines() string {
	patterns, err := fa.AnalyzeFailures()
	if err != nil || len(patterns) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Guidelines Based on Past Failures\n\n")

	for _, p := range patterns {
		if p.Count < 2 {
			continue
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", p.Pattern, p.Suggestions[0]))
	}

	return sb.String()
}

type FailureEntry struct {
	Day    int    `json:"day"`
	Task   string `json:"task"`
	Reason string `json:"reason"`
	TS     string `json:"ts"`
	Type   string `json:"type"`
}

func (e *Engine) AnalyzeFailures() ([]*FailurePattern, error) {
	fa := NewFailureAnalyzer(e.repoPath, e.logger)
	return fa.AnalyzeFailures()
}

func (e *Engine) GetFailurePatternSummary() string {
	fa := NewFailureAnalyzer(e.repoPath, e.logger)
	return fa.GetPatternSummary()
}

func (e *Engine) GetAvoidanceGuidelines() string {
	fa := NewFailureAnalyzer(e.repoPath, e.logger)
	return fa.GetAvoidanceGuidelines()
}

func (e *Engine) RecordAndAnalyzeFailure(taskTitle, reason string) error {
	if err := e.appendFailureJSONL(taskTitle, reason); err != nil {
		e.logger.Warn("Failed to record failure", "err", err)
	}

	fa := NewFailureAnalyzer(e.repoPath, e.logger)
	if err := fa.SavePatterns(); err != nil {
		e.logger.Warn("Failed to save failure patterns", "err", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
