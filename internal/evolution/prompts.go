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

## TOOL USAGE IS MANDATORY - NOT OPTIONAL

You CANNOT complete tasks by describing what you would do. You MUST use tools.
Failure to use tools = Task Failure.

## Available Tools - USE THEM IMMEDIATELY

### read_file — Read a file
`+"```"+`tool
{"tool":"read_file","args":{"path":"path/to/file.go"}}
`+"```"+`

### edit_file — Edit part of a file (MOST IMPORTANT)
`+"```"+`tool
{"tool":"edit_file","args":{"path":"cmd/foo.go","old_string":"old code","new_string":"new code"}}
`+"```"+`

### write_file — Write/create a file
`+"```"+`tool
{"tool":"write_file","args":{"path":"path/to/file.go","content":"package main\n\nfunc main() {}"}}
`+"```"+`

### bash — Run commands
`+"```"+`tool
{"tool":"bash","args":{"cmd":"go build ./... && go test ./..."}}
`+"```"+`

### list_files — List directory contents
`+"```"+`tool
{"tool":"list_files","args":{"path":"cmd/iterate"}}
`+"```"+`

## CRITICAL RULES - VIOLATION = FAILURE

1. **YOU MUST USE TOOLS** - Describing actions without using tools is FAILURE
2. **edit_file is REQUIRED** - You MUST use edit_file at least once per task
3. **NO EXPLANATIONS** - Don't say "I will fix this" - JUST FIX IT with edit_file
4. **IMMEDIATE ACTION** - Start with list_files, then read_file, then edit_file
5. **TEST AFTER EVERY CHANGE** - Run: go build ./... && go test ./...
6. **COMMIT REQUIRED** - Use bash: git add -A && git commit -m "fix: description"
7. **METRICS UPDATES ARE FAILURE** - Only updating stats/docs = AUTOMATIC REJECTION

## ANTI-PATTERNS THAT CAUSE FAILURE

❌ "I'll analyze the codebase first" → WRONG. Just use list_files
❌ "I found the issue in X" → WRONG. Use edit_file to fix it
❌ "The problem is..." → WRONG. Fix it with edit_file
❌ "Let me search for..." → WRONG. Use search_files tool

## SUCCESS PATTERN

✅ list_files → read_file → edit_file → bash (build/test) → bash (git commit)

## BUGS TO FIX

- defer inside loops (resource leaks)
- Missing error handling
- Functions that should return errors
- Race conditions
- Unused imports/variables

## FINAL WARNING

If you don't use edit_file at least once, the task FAILS automatically.
If you only update docs/stats/dashboard, the task FAILS automatically.
Start NOW. Use tools. Fix bugs.`,
		identity,
		string(personality),
	)
}

func buildUserMessage(repoPath, journal, issues string) string {
	learnings, _ := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))

	var sb strings.Builder
	sb.WriteString("⚠️  MANDATORY CODE FIX REQUIRED - NO EXCEPTIONS ⚠️\n\n")
	sb.WriteString("YOU HAVE 10 MINUTES TO COMPLETE THIS TASK.\n")
	sb.WriteString("FAILURE TO MAKE CODE CHANGES = AUTOMATIC REJECTION\n\n")

	sb.WriteString("## TASK: Fix a Real Bug in 4 Steps\n\n")

	sb.WriteString("### Step 1: EXPLORE (30 seconds)\n")
	sb.WriteString("→ Use list_files on cmd/iterate/\n")
	sb.WriteString("→ Use list_files on internal/\n\n")

	sb.WriteString("### Step 2: FIND BUG (2 minutes)\n")
	sb.WriteString("→ Use read_file to examine .go files\n")
	sb.WriteString("→ Look for: defer inside loops, ignored errors, unused variables\n")
	sb.WriteString("→ Find ONE concrete bug\n\n")

	sb.WriteString("### Step 3: FIX BUG (REQUIRED - 5 minutes)\n")
	sb.WriteString("→ Use edit_file to fix the bug\n")
	sb.WriteString("→ Example: fix defer in loop by changing to explicit close\n")
	sb.WriteString("→ YOU MUST USE edit_file - NO EXCEPTIONS\n\n")

	sb.WriteString("### Step 4: VERIFY & COMMIT (3 minutes)\n")
	sb.WriteString("→ Use bash: go build ./... && go test ./...\n")
	sb.WriteString("→ Use bash: git add -A && git commit -m 'fix: bug description'\n")
	sb.WriteString("→ If tests fail, fix with edit_file and retry\n\n")

	sb.WriteString("## AUTOMATIC FAILURE CONDITIONS\n")
	sb.WriteString("❌ No edit_file usage → FAILURE\n")
	sb.WriteString("❌ Only docs/stats changed → FAILURE\n")
	sb.WriteString("❌ No .go files modified → FAILURE\n")
	sb.WriteString("❌ Taking longer than 10 minutes → FAILURE\n\n")

	if len(learnings) > 0 {
		l := string(learnings)
		if len(l) > 500 {
			l = l[:500] + "\n...[truncated]"
		}
		sb.WriteString("## Previous Learnings\n")
		sb.WriteString(l + "\n\n")
	}

	if len(journal) > 0 {
		recent := journal
		if len(journal) > 300 {
			recent = "..." + journal[len(journal)-300:]
		}
		sb.WriteString("## Recent Activity\n")
		sb.WriteString(recent + "\n\n")
	}

	if len(issues) > 0 {
		sb.WriteString("## Community Issues\n")
		sb.WriteString(issues + "\n")
	}

	sb.WriteString("\n🚨 START NOW - USE list_files THEN read_file THEN edit_file 🚨")
	return sb.String()
}

// BuildRetryPrompt creates an escalating prompt for retry attempts
func BuildRetryPrompt(attempt int, previousOutput string) string {
	urgency := ""
	switch attempt {
	case 1:
		urgency = "⚠️  ATTEMPT 2: You failed to make code changes. This is your second chance."
	case 2:
		urgency = "🚨 ATTEMPT 3: FINAL WARNING. If you don't use edit_file now, you FAIL."
	default:
		urgency = "🔥 CRITICAL: IMMEDIATE ACTION REQUIRED"
	}

	return fmt.Sprintf(`%s

PREVIOUS ATTEMPT FAILED:
%s

YOU MUST:
1. IMMEDIATELY use edit_file on a .go file
2. Make a concrete code fix (not just comments)
3. Run tests to verify
4. Commit the changes

DO NOT:
- Explain what you will do
- Read more files without acting
- Update only docs/stats

YOU HAVE 3 MINUTES. USE edit_file NOW OR FAIL.`, urgency, previousOutput)
}
