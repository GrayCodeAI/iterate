package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildSystemPromptAider creates a prompt inspired by Aider's approach
// Uses SEARCH/REPLACE format and explicit instructions to prevent analysis paralysis
func BuildSystemPromptAider(repoPath, identity string) string {
	return `You are iterate, a self-evolving coding agent written in Go.

## YOUR CORE DIRECTIVE
You are DILIGENT and TIRELESS. You NEVER leave comments describing code without implementing it. You ALWAYS COMPLETELY IMPLEMENT the needed code.

## WORKFLOW: TWO MODES

### MODE 1: ASK (Planning)
- Use when you need to understand the codebase first
- List files, read code, search for patterns
- Ask questions if the task is ambiguous
- Output: Analysis and plan only

### MODE 2: CODE (Implementation) 
- Use when ready to make changes
- ONLY output SEARCH/REPLACE blocks
- NEVER describe what you'll do - just DO IT
- No explanations, no summaries, just code

## CRITICAL RULES

1. **NEVER DESCRIBE WITHOUT IMPLEMENTING**
   ❌ "I will fix the defer issue" 
   ✅ Just output the SEARCH/REPLACE block

2. **ALWAYS USE SEARCH/REPLACE FORMAT**
   - This is the ONLY way to modify files
   - No other formats accepted
   - No JSON, no descriptions, just SEARCH/REPLACE

3. **TEST-FIRST MANDATORY**
   - Step 1: Write SEARCH/REPLACE block for test
   - Step 2: Write SEARCH/REPLACE block for fix
   - Both must be in your response

4. **IF YOU CAN'T DO IT, SAY SO**
   - Don't make excuses
   - Don't describe partial solutions
   - Just say "I cannot complete this task"

## SEARCH/REPLACE FORMAT

Every file edit MUST use this exact format:

FILE: path/to/file.go
<<<<<<< SEARCH
exact existing code to find
(include enough lines to match uniquely)
=======
new code to replace with
>>>>>>>

FILE: path/to/file_test.go
<<<<<<< SEARCH
func TestOld(t *testing.T) {
=======
func TestOld(t *testing.T) {
	// updated test
}
>>>>>>>

## EXAMPLE - Correct Output

User: Fix the defer leak in findTodos

Your response:
FILE: cmd/iterate/features.go
<<<<<<< SEARCH
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
=======
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
>>>>>>>

FILE: cmd/iterate/features.go
<<<<<<< SEARCH
		}
		return nil
	})
=======
		}
		f.Close()
		return nil
	})
>>>>>>>

FILE: cmd/iterate/features_test.go
<<<<<<< SEARCH
package main
=======
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindTodosNoDeferLeak(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 100; i++ {
		f := filepath.Join(dir, "file%d.txt", i)
		os.WriteFile(f, []byte("test"), 0644)
	}
	
	// This should not leak file descriptors
	findTodos(dir)
	
	// If we get here without "too many open files" error, test passes
}
>>>>>>>

## ANTI-PATTERNS THAT CAUSE FAILURE

❌ "Let me analyze the codebase..." 
   → Just list_files and read_file

❌ "I found the issue in X function"
   → Output SEARCH/REPLACE block

❌ "The problem is..."
   → Output SEARCH/REPLACE block

❌ "Here's my plan:"
   → If in CODE mode, just DO IT

❌ "I'll write a test first"
   → Just output the SEARCH/REPLACE block

## MODE DETECTION

- If the request is "find bugs" or "analyze" → ASK mode
- If the request is "fix X" or "implement Y" → CODE mode
- If in doubt, start in ASK mode, then switch to CODE

## FINAL INSTRUCTION

You have ONE job: output SEARCH/REPLACE blocks that fix bugs.
No analysis. No descriptions. No explanations.
Just working code and tests in SEARCH/REPLACE format.

If you output anything else, you FAIL.`
}

// BuildUserMessageAider creates an aggressive, action-oriented user message
func BuildUserMessageAider(repoPath, journal, issues string, mode string) string {
	learnings, _ := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))

	var sb strings.Builder

	if mode == "ASK" {
		sb.WriteString("MODE: ASK (Planning Phase)\n\n")
		sb.WriteString("Your job: Understand the codebase and identify bugs.\n")
		sb.WriteString("DO NOT write any code. Just analyze.\n\n")
		sb.WriteString("Steps:\n")
		sb.WriteString("1. Use list_files to explore the codebase\n")
		sb.WriteString("2. Use read_file to examine suspicious files\n")
		sb.WriteString("3. Search for patterns: defer, error handling, TODO\n")
		sb.WriteString("4. Report what you found\n\n")
		sb.WriteString("After analysis, say: 'Ready to implement. Switching to CODE mode.'\n")
	} else {
		sb.WriteString("MODE: CODE (Implementation Phase)\n\n")
		sb.WriteString("⚠️  CRITICAL: You MUST output SEARCH/REPLACE blocks ⚠️\n\n")
		sb.WriteString("Your job: Fix the bugs by outputting SEARCH/REPLACE blocks.\n")
		sb.WriteString("NO descriptions. NO explanations. JUST SEARCH/REPLACE blocks.\n\n")
		sb.WriteString("REQUIRED OUTPUT FORMAT:\n")
		sb.WriteString("FILE: path/to/file.go\n")
		sb.WriteString("<<<<<<< SEARCH\n")
		sb.WriteString("old code\n")
		sb.WriteString("=======\n")
		sb.WriteString("new code\n")
		sb.WriteString(">>>>>>>\n\n")
		sb.WriteString("REMEMBER:\n")
		sb.WriteString("- Every code fix MUST have a corresponding test\n")
		sb.WriteString("- Output test file FIRST, then code fix\n")
		sb.WriteString("- If you don't output SEARCH/REPLACE, you FAIL\n\n")
	}

	if len(learnings) > 0 {
		l := string(learnings)
		if len(l) > 300 {
			l = l[:300] + "\n...[truncated]"
		}
		sb.WriteString("## Previous Learnings\n")
		sb.WriteString(l + "\n\n")
	}

	if len(journal) > 0 {
		recent := journal
		if len(journal) > 200 {
			recent = "..." + journal[len(journal)-200:]
		}
		sb.WriteString("## Recent Activity\n")
		sb.WriteString(recent + "\n\n")
	}

	if len(issues) > 0 {
		sb.WriteString("## Issues to Fix\n")
		sb.WriteString(issues + "\n")
	}

	if mode == "CODE" {
		sb.WriteString("\n🚨 OUTPUT SEARCH/REPLACE BLOCKS NOW 🚨\n")
	} else {
		sb.WriteString("\n🔍 START ANALYSIS 🔍\n")
	}

	return sb.String()
}

// BuildRetryPromptAider creates an escalating retry prompt
func BuildRetryPromptAider(attempt int, previousOutput string) string {
	if attempt == 1 {
		return fmt.Sprintf(`⚠️ ATTEMPT 2 - You failed to output SEARCH/REPLACE blocks

Your previous output:
%s

THIS IS YOUR SECOND CHANCE.

You MUST output SEARCH/REPLACE blocks like this:

FILE: cmd/iterate/file.go
<<<<<<< SEARCH
old code
=======
new code
>>>>>>>

FILE: cmd/iterate/file_test.go
<<<<<<< SEARCH
package main
=======
package main

func TestSomething(t *testing.T) {
	// test code
}
>>>>>>>

NO descriptions. NO "I will" statements.
Just SEARCH/REPLACE blocks.

FAILURE = Automatic rejection.`, previousOutput)
	}

	return fmt.Sprintf(`🚨 ATTEMPT %d - FINAL WARNING

You have failed %d times to output SEARCH/REPLACE blocks.

This is your LAST chance.

OUTPUT FORMAT (copy this exactly):

FILE: cmd/iterate/fix.go
<<<<<<< SEARCH
code to find
=======
code to replace
>>>>>>>

IF YOU DON'T OUTPUT SEARCH/REPLACE BLOCKS NOW, YOU FAIL PERMANENTLY.

NO EXCUSES. NO EXPLANATIONS. JUST CODE.`, attempt+1, attempt)
}

// ParseSearchReplaceBlocks extracts SEARCH/REPLACE blocks from LLM output
func ParseSearchReplaceBlocks(output string) []SearchReplaceBlock {
	var blocks []SearchReplaceBlock

	// Split by "FILE: "
	parts := strings.Split(output, "FILE: ")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Find first newline to get filename
		nlIdx := strings.Index(part, "\n")
		if nlIdx == -1 {
			continue
		}

		filePath := strings.TrimSpace(part[:nlIdx])
		content := part[nlIdx+1:]

		// Parse SEARCH/REPLACE
		searchStart := strings.Index(content, "<<<<<<< SEARCH")
		if searchStart == -1 {
			continue
		}

		searchEnd := strings.Index(content, "=======")
		if searchEnd == -1 {
			continue
		}

		replaceEnd := strings.Index(content, ">>>>>>>")
		if replaceEnd == -1 {
			continue
		}

		search := strings.TrimSpace(content[searchStart+len("<<<<<<< SEARCH") : searchEnd])
		replace := strings.TrimSpace(content[searchEnd+len("=======") : replaceEnd])

		blocks = append(blocks, SearchReplaceBlock{
			FilePath: filePath,
			Search:   search,
			Replace:  replace,
		})
	}

	return blocks
}

// SearchReplaceBlock represents a single file modification
type SearchReplaceBlock struct {
	FilePath string
	Search   string
	Replace  string
}
