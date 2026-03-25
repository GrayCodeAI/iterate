package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func buildSystemPrompt(repoPath, identity string) string {
	personality, _ := os.ReadFile(filepath.Join(repoPath, "docs/PERSONALITY.md"))

	return fmt.Sprintf(`You are iterate, a self-evolving coding agent written in Go.

## Identity
%s

## Personality
%s

## Tool call format

To call a tool, output a fenced code block with language "tool":

`+"```"+`tool
{"tool":"tool_name","args":{"key":"value"}}
`+"```"+`

Examples:

Read a file:
`+"```"+`tool
{"tool":"read_file","args":{"path":"internal/evolution/engine.go"}}
`+"```"+`

Write a file:
`+"```"+`tool
{"tool":"write_file","args":{"path":"SESSION_PLAN.md","content":"## Session Plan\n\nSession Title: Fix nil pointer\n\n### Task 1: Fix nil check\nFiles: cmd/iterate/repl.go\nDescription: Add nil check on line 47\nIssue: none\n\n### Issue Responses\n"}}
`+"```"+`

Run a bash command:
`+"```"+`tool
{"tool":"bash","args":{"cmd":"go test ./..."}}
`+"```"+`

	CRITICAL: You MUST use this format to call tools. Do NOT just describe what you would do.`,
		identity,
		string(personality),
	)
}

func buildUserMessage(repoPath, journal, issues string) string {
	learnings, _ := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))

	var sb strings.Builder
	sb.WriteString("## Your task\n\n")
	sb.WriteString("Assess your codebase, find one meaningful improvement, implement it, test it, and commit it.\n\n")

	if len(learnings) > 0 {
		l := string(learnings)
		if len(l) > 1000 {
			l = l[:1000] + "\n...[truncated]"
		}
		sb.WriteString("## What you have learned so far\n\n")
		sb.WriteString(l + "\n\n")
	}

	if len(journal) > 0 {
		recent := journal
		if len(journal) > 500 {
			recent = "...\n" + journal[len(journal)-500:]
		}
		sb.WriteString("## Recent journal\n\n")
		sb.WriteString(recent + "\n\n")
	}

	if len(issues) > 0 {
		sb.WriteString("## Community input\n\n")
		sb.WriteString(issues + "\n")
	}

	sb.WriteString("Begin your self-assessment now.")
	return sb.String()
}
