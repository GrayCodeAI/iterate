package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// WriteLearningsToMemory is the public entry point for the synthesize workflow.
func (e *Engine) WriteLearningsToMemory(title, context, takeaway string) error {
	return e.appendLearningJSONL(title, "evolution", context, takeaway)
}

func (e *Engine) appendLearningJSONL(title, source, context, takeaway string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	dayBytes, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	dayStr := strings.TrimSpace(string(dayBytes))
	day, err := strconv.Atoi(dayStr)
	if err != nil && dayStr != "" {
		e.logger.Warn("DAY_COUNT is not a valid integer, defaulting to 0", "value", dayStr)
	}

	entry := map[string]interface{}{
		"type":     "lesson",
		"day":      day,
		"ts":       time.Now().UTC().Format(time.RFC3339),
		"source":   source,
		"title":    title,
		"context":  context,
		"takeaway": takeaway,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal learning: %w", err)
	}

	path := filepath.Join(memDir, "learnings.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open learnings.jsonl: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Trim entries older than 90 days (learnings have longer value than failures).
	trimFailuresJSONL(path, 90*24*time.Hour)
	return nil
}

// appendFailureJSONL records a failed task to memory/failures.jsonl so the
// planner can avoid repeating the same approach in future cycles.
func (e *Engine) appendFailureJSONL(taskTitle, reason string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	dayBytes, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	dayStr := strings.TrimSpace(string(dayBytes))
	day, err := strconv.Atoi(dayStr)
	if err != nil && dayStr != "" {
		e.logger.Warn("DAY_COUNT is not a valid integer, defaulting to 0", "value", dayStr)
	}

	entry := map[string]interface{}{
		"type":   "failure",
		"day":    day,
		"ts":     time.Now().UTC().Format(time.RFC3339),
		"task":   taskTitle,
		"reason": reason,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal failure: %w", err)
	}

	path := filepath.Join(memDir, "failures.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open failures.jsonl: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Trim entries older than 30 days to prevent unbounded growth.
	trimFailuresJSONL(path, 30*24*time.Hour)
	return nil
}

// trimFailuresJSONL rewrites failures.jsonl keeping only entries newer than maxAge.
func trimFailuresJSONL(path string, maxAge time.Duration) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}

	cutoff := time.Now().UTC().Add(-maxAge)
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			TS string `json:"ts"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			kept = append(kept, line) // keep unparseable lines
			continue
		}
		ts, err := time.Parse(time.RFC3339, entry.TS)
		if err != nil || ts.After(cutoff) {
			kept = append(kept, line)
		}
	}

	if len(kept) == 0 {
		return
	}
	// Write atomically: temp file then rename, so a crash mid-write can't corrupt the file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// recentFailures reads memory/failures.jsonl and returns the last N entries
// from the past 30 days as a formatted string for inclusion in the planner prompt.
func recentFailures(repoPath string, limit int) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "memory", "failures.jsonl"))
	if err != nil || len(data) == 0 {
		return ""
	}

	type failEntry struct {
		Day    int    `json:"day"`
		Task   string `json:"task"`
		Reason string `json:"reason"`
		TS     string `json:"ts"`
	}

	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	var entries []failEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e failEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		// Skip entries older than 30 days.
		if ts, err := time.Parse(time.RFC3339, e.TS); err == nil && ts.Before(cutoff) {
			continue
		}
		entries = append(entries, e)
	}

	// Keep only the last `limit` entries.
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Recent Failures (avoid repeating these)\n\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- Day %d — %s", e.Day, e.Task))
		if e.Reason != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Reason))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *Engine) newAgent(p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet) *iteragent.Agent {
	a := iteragent.New(p, tools, e.logger).
		WithSystemPrompt(systemPrompt).
		WithSkillSet(skills)
	if e.thinkingLevel != "" && e.thinkingLevel != iteragent.ThinkingLevelOff {
		a = a.WithThinkingLevel(e.thinkingLevel)
	}
	return a
}
