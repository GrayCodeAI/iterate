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

func (e *Engine) newAgent(p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet) *iteragent.Agent {
	a := iteragent.New(p, tools, e.logger).
		WithSystemPrompt(systemPrompt).
		WithSkillSet(skills)
	if e.thinkingLevel != "" && e.thinkingLevel != iteragent.ThinkingLevelOff {
		a = a.WithThinkingLevel(e.thinkingLevel)
	}
	return a
}
