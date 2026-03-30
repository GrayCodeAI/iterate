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

## Available Tools

You have these tools. USE THEM. Do not just describe what you would do — actually do it.

### read_file — Read a file
`+"```"+`tool
{"tool":"read_file","args":{"path":"path/to/file.go"}}
`+"```"+`

### write_file — Write/create a file
`+"```"+`tool
{"tool":"write_file","args":{"path":"SESSION_PLAN.md","content":"## Session Plan\n\nSession Title: My task\n\n### Task 1: Do something\nFiles: cmd/foo.go\nDescription: Fix the thing\nIssue: none\n\n### Issue Responses\n"}}
`+"```"+`

### edit_file — Edit part of a file
`+"```"+`tool
{"tool":"edit_file","args":{"path":"cmd/foo.go","old_string":"old code here","new_string":"new code here"}}
`+"```"+`

### bash — Run a shell command
`+"```"+`tool
{"tool":"bash","args":{"cmd":"go test ./..."}}
`+"```"+`

### list_files — List files in a directory
`+"```"+`tool
{"tool":"list_files","args":{"path":"cmd/iterate"}}
`+"```"+`

## Rules

1. ALWAYS use tools to read files before editing them
2. After writing code, ALWAYS run: go build ./... && go test ./...
3. If tests fail, fix the code and try again
4. If you need to create SESSION_PLAN.md, use write_file
5. Be direct. No explanations. Just act.
6. One tool call at a time. Wait for results before next action.
7. CRITICAL: You MUST make actual code changes. Updating metrics/docs alone is FAILURE.
8. Look for real bugs: defer in loops, missing error handling, race conditions, etc.`,
		identity,
		string(personality),
	)
}

func buildUserMessage(repoPath, journal, issues string) string {
	learnings, _ := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))

	var sb strings.Builder
	sb.WriteString("## CRITICAL TASK: Find and Fix Real Bugs\n\n")
	sb.WriteString("You MUST find and fix actual code bugs. This is NOT optional.\n\n")
	sb.WriteString("### Step 1: Find Bugs (REQUIRED)\n")
	sb.WriteString("1. Use list_files to explore cmd/iterate/ and internal/ directories\n")
	sb.WriteString("2. Use read_file to examine .go source files\n")
	sb.WriteString("3. Look for:\n")
	sb.WriteString("   - defer statements inside loops (resource leaks)\n")
	sb.WriteString("   - Missing error handling (ignored errors)\n")
	sb.WriteString("   - Unused variables or imports\n")
	sb.WriteString("   - Functions that should return errors but don't\n")
	sb.WriteString("   - Race conditions in concurrent code\n")
	sb.WriteString("   - TODO/FIXME comments that should be implemented\n\n")
	sb.WriteString("### Step 2: Fix the Bug (REQUIRED)\n")
	sb.WriteString("1. Use edit_file to fix the bug you found\n")
	sb.WriteString("2. Do NOT just update docs/stats - that's FAILURE\n")
	sb.WriteString("3. You MUST modify .go files with actual code changes\n\n")
	sb.WriteString("### Step 3: Test (REQUIRED)\n")
	sb.WriteString("1. Run: go build ./... && go test ./...\n")
	sb.WriteString("2. Fix any failures before committing\n\n")
	sb.WriteString("### Step 4: Commit (REQUIRED)\n")
	sb.WriteString("1. Use bash to run: git add -A && git commit -m 'fix: description'\n")
	sb.WriteString("2. Commit message must describe the actual bug fixed\n\n")

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

	sb.WriteString("Begin now. Use tools. Don't just describe — act.")
	return sb.String()
}
