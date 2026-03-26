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
	day, _ := strconv.Atoi(strings.TrimSpace(string(dayBytes)))

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
	defer f.Close()

	_, err = f.Write(append(line, '\n'))
	return err
}

// appendFailureJSONL records a failed task to memory/failures.jsonl so the
// planner can avoid repeating the same approach in future cycles.
func (e *Engine) appendFailureJSONL(taskTitle, reason string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	dayBytes, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	day, _ := strconv.Atoi(strings.TrimSpace(string(dayBytes)))

	entry := map[string]interface{}{
		"type":  "failure",
		"day":   day,
		"ts":    time.Now().UTC().Format(time.RFC3339),
		"task":  taskTitle,
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
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// recentFailures reads memory/failures.jsonl and returns the last N entries
// as a formatted string for inclusion in the planner prompt.
func recentFailures(repoPath string, limit int) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "memory", "failures.jsonl"))
	if err != nil || len(data) == 0 {
		return ""
	}

	type failEntry struct {
		Day    int    `json:"day"`
		Task   string `json:"task"`
		Reason string `json:"reason"`
	}

	var entries []failEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e failEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
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
