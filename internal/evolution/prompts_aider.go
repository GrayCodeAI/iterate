package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BuildSystemPromptAider creates a prompt inspired by Aider's approach
// Uses unified diff format (like git diff) - this is 3X more effective than custom formats
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
- ONLY output UNIFIED DIFFS (like git diff)
- NEVER describe what you'll do - just DO IT

## CRITICAL RULES

1. **NEVER DESCRIBE WITHOUT IMPLEMENTING**
   ❌ "I will fix the defer issue" 
   ✅ Just output the unified diff

2. **ALWAYS USE UNIFIED DIFF FORMAT**
   - Use the same format as "git diff" output
   - It's familiar to LLMs - they've seen millions of diffs in training
   - DO NOT include line numbers in hunk headers

3. **TEST-FIRST MANDATORY**
   - Step 1: Write unified diff for test file FIRST
   - Step 2: Write unified diff for code fix
   - Both must be in your response

4. **IF YOU CAN'T DO IT, SAY SO**

## UNIFIED DIFF FORMAT (LIKE GIT DIFF)

Use the same format that "git diff" outputs:

--- a/path/to/file.go
+++ b/path/to/file.go
@@ ... @@
 func example() {
-    // old code to remove
+    // new code to add
     return
 }

## KEY RULES FOR DIFFS:

1. Start with "--- a/FILENAME" and "+++ b/FILENAME"
2. Use "@@ ... @@" for hunk headers (don't include line numbers)
3. Use "-" prefix for lines to remove
4. Use "+" prefix for lines to add
5. Use " " (space) prefix for unchanged lines
6. Include enough context to uniquely identify the location

## EXAMPLE - Correct Output

User: Fix the bug in findTodos

Your response:
--- a/cmd/iterate/features.go
+++ b/cmd/iterate/features.go
@@ ... @@
 func findTodos(dir string) []string {
-    f, err := os.Open(path)
-    if err != nil {
-        return nil
-    }
-    defer f.Close()
+    f, err := os.Open(path)
+    if err != nil {
+        return nil
+    }
 }

--- a/cmd/iterate/features_test.go
+++ b/cmd/iterate/features_test.go
@@ ... @@
+func TestFindTodosNoDeferLeak(t *testing.T) {
+    // Test code here
+}

## ANTI-PATTERNS THAT CAUSE FAILURE

❌ "Let me analyze the codebase..." → Just list_files and read_file

❌ "I found the issue in X function" → Output unified diff

❌ "Here's my plan:" → If in CODE mode, just DO IT

❌ Any format other than unified diff → WILL FAIL

## FINAL INSTRUCTION

You have ONE job: output unified diffs that fix bugs.
No analysis. No descriptions. No explanations.
Just working code and tests in unified diff format.
`
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
		sb.WriteString("⚠️  CRITICAL: You MUST output UNIFIED DIFFS ⚠️\n\n")
		sb.WriteString("Your job: Fix the bugs by outputting unified diffs.\n")
		sb.WriteString("NO descriptions. NO explanations. JUST UNIFIED DIFFS.\n\n")
		sb.WriteString("REQUIRED OUTPUT FORMAT:\n")
		sb.WriteString("--- a/path/to/file.go\n")
		sb.WriteString("+++ b/path/to/file.go\n")
		sb.WriteString("@@ ... @@\n")
		sb.WriteString("- old code\n")
		sb.WriteString("+ new code\n\n")
		sb.WriteString("REMEMBER:\n")
		sb.WriteString("- Every code fix MUST have a corresponding test\n")
		sb.WriteString("- Output test file diff FIRST, then code fix diff\n")
		sb.WriteString("- If you don't output unified diffs, you FAIL\n\n")
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
		sb.WriteString("\n🚨 OUTPUT UNIFIED DIFFS NOW 🚨\n")
	} else {
		sb.WriteString("\n🔍 START ANALYSIS 🔍\n")
	}

	return sb.String()
}

// BuildRetryPromptAider creates an escalating retry prompt
func BuildRetryPromptAider(attempt int, previousOutput string) string {
	if attempt == 1 {
		return fmt.Sprintf(`⚠️ ATTEMPT 2 - You failed to output unified diffs

Your previous output:
%s

THIS IS YOUR SECOND CHANCE.

You MUST output unified diffs like this:

--- a/cmd/iterate/file.go
+++ b/cmd/iterate/file.go
@@ ... @@
 func oldFunc() {
-    // old
+    // new
 }

--- a/cmd/iterate/file_test.go
+++ b/cmd/iterate/file_test.go
@@ ... @@
+func TestSomething(t *testing.T) {
+    // test code
+}

NO descriptions. NO "I will" statements.
Just unified diffs.

FAILURE = Automatic rejection.`, previousOutput)
	}

	return fmt.Sprintf(`🚨 ATTEMPT %d - FINAL WARNING

You have failed %d times to output unified diffs.

This is your LAST chance.

OUTPUT FORMAT (copy this exactly):

--- a/cmd/iterate/fix.go
+++ b/cmd/iterate/fix.go
@@ ... @@
-code to find
+code to replace
+more code

IF YOU DON'T OUTPUT UNIFIED DIFFS NOW, YOU FAIL PERMANENTLY.

NO EXCUSES. NO EXPLANATIONS. JUST CODE.`, attempt+1, attempt)
}

// UnifiedDiff represents a parsed unified diff
type UnifiedDiff struct {
	OldFile string
	NewFile string
	Hunks   []DiffHunk
}

// DiffHunk represents a single hunk in a unified diff
type DiffHunk struct {
	Context []string
	Added   []string
	Removed []string
}

// ParseUnifiedDiffs extracts unified diffs from LLM output
func ParseUnifiedDiffs(output string) []UnifiedDiff {
	var diffs []UnifiedDiff

	// Split by "--- a/" or "+++ b/" to find diff blocks
	re := regexp.MustCompile(`(?s)(--- a/[^\n]+\n\+{3} a/[^\n]+\n@@.*?@@(?:\n.*?)*?)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		diff := parseSingleDiffBlock(match[1])
		if diff.OldFile != "" {
			diffs = append(diffs, diff)
		}
	}

	return diffs
}

func parseSingleDiffBlock(block string) UnifiedDiff {
	diff := UnifiedDiff{}

	lines := strings.Split(block, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "--- a/") {
			diff.OldFile = strings.TrimPrefix(line, "--- a/")
		} else if strings.HasPrefix(line, "+++ b/") {
			diff.NewFile = strings.TrimPrefix(line, "+++ b/")
		} else if strings.HasPrefix(line, "@@") {
			hunk := parseHunk(line, lines)
			if len(hunk.Added) > 0 || len(hunk.Removed) > 0 {
				diff.Hunks = append(diff.Hunks, hunk)
			}
		}
	}

	// Use NewFile if OldFile is empty
	if diff.OldFile == "" {
		diff.OldFile = diff.NewFile
	}

	return diff
}

func parseHunk(header string, allLines []string) DiffHunk {
	hunk := DiffHunk{}

	// Find where this hunk starts in the lines
	hunkStart := -1
	for i, line := range allLines {
		if strings.HasPrefix(line, "@@") && strings.Contains(header, strings.TrimSpace(line)) {
			hunkStart = i
			break
		}
	}

	if hunkStart == -1 {
		return hunk
	}

	// Parse hunk lines until next hunk or end
	for i := hunkStart + 1; i < len(allLines); i++ {
		line := allLines[i]

		if strings.HasPrefix(line, "@@") {
			break
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			hunk.Added = append(hunk.Added, strings.TrimPrefix(line, "+"))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			hunk.Removed = append(hunk.Removed, strings.TrimPrefix(line, "-"))
		} else if strings.HasPrefix(line, " ") || (line != "" && !strings.HasPrefix(line, "\\")) {
			hunk.Context = append(hunk.Context, strings.TrimPrefix(line, " "))
		}
	}

	return hunk
}

// ApplyUnifiedDiffs applies unified diffs to files with flexible error recovery
func (e *Engine) ApplyUnifiedDiffs(diffs []UnifiedDiff) ([]string, error) {
	var modifiedFiles []string

	for _, diff := range diffs {
		// Try to apply the diff
		filePath := diff.NewFile
		if filePath == "" {
			continue
		}

		fullPath := filepath.Join(e.repoPath, filePath)

		// Strategy 1: Direct patch
		if err := applyDiffStrategy1(fullPath, diff); err == nil {
			modifiedFiles = append(modifiedFiles, filePath)
			e.logger.Info("Applied unified diff (strategy 1)", "file", filePath)
			continue
		}

		// Strategy 2: Flexible whitespace
		if err := applyDiffStrategy2(fullPath, diff); err == nil {
			modifiedFiles = append(modifiedFiles, filePath)
			e.logger.Info("Applied unified diff (strategy 2)", "file", filePath)
			continue
		}

		// Strategy 3: High-level diff (replace entire blocks)
		if err := applyDiffStrategy3(fullPath, diff); err == nil {
			modifiedFiles = append(modifiedFiles, filePath)
			e.logger.Info("Applied unified diff (strategy 3)", "file", filePath)
			continue
		}

		e.logger.Warn("Failed to apply diff with all strategies", "file", filePath)
	}

	if len(modifiedFiles) == 0 && len(diffs) > 0 {
		return nil, fmt.Errorf("failed to apply any diffs")
	}

	return modifiedFiles, nil
}

func applyDiffStrategy1(filePath string, diff UnifiedDiff) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	oldContent := string(content)
	newContent := oldContent

	// For each hunk, try to find and replace
	for _, hunk := range diff.Hunks {
		// Build search pattern from removed lines + context
		searchLines := append(hunk.Context, hunk.Removed...)
		search := strings.Join(searchLines, "\n")

		// Build replace pattern from context + added lines
		replaceLines := append(hunk.Context, hunk.Added...)
		replace := strings.Join(replaceLines, "\n")

		if !strings.Contains(newContent, search) {
			return fmt.Errorf("hunk not found")
		}

		newContent = strings.Replace(newContent, search, replace, 1)
	}

	if newContent == oldContent {
		return fmt.Errorf("no changes made")
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

func applyDiffStrategy2(filePath string, diff UnifiedDiff) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	oldContent := string(content)
	newContent := oldContent

	// Normalize whitespace before matching
	for _, hunk := range diff.Hunks {
		searchLines := normalizeWhitespace(hunk.Context, hunk.Removed)
		search := strings.Join(searchLines, "\n")

		replaceLines := normalizeWhitespace(hunk.Context, hunk.Added)
		replace := strings.Join(replaceLines, "\n")

		// Try to find with normalized whitespace
		normalizedOld := normalizeContent(newContent)
		normalizedSearch := normalizeContent(search)

		idx := strings.Index(normalizedOld, normalizedSearch)
		if idx == -1 {
			continue
		}

		// Find the actual location and replace
		actualStart := findActualLocation(newContent, search, idx)
		if actualStart == -1 {
			continue
		}

		actualEnd := actualStart + len(search)
		newContent = newContent[:actualStart] + replace + newContent[actualEnd:]
	}

	if newContent == oldContent {
		return fmt.Errorf("no changes made")
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

func applyDiffStrategy3(filePath string, diff UnifiedDiff) error {
	// High-level approach: replace entire function/blocks
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	oldContent := string(content)
	newContent := oldContent

	// Build a complete replacement: removed lines replaced with added lines
	for _, hunk := range diff.Hunks {
		oldBlock := strings.Join(append(hunk.Context, hunk.Removed...), "\n")
		newBlock := strings.Join(append(hunk.Context, hunk.Added...), "\n")

		if !strings.Contains(newContent, oldBlock) {
			// Try without context
			oldBlock = strings.Join(hunk.Removed, "\n")
			newBlock = strings.Join(hunk.Added, "\n")
		}

		if strings.Contains(newContent, oldBlock) {
			newContent = strings.Replace(newContent, oldBlock, newBlock, 1)
		}
	}

	if newContent == oldContent {
		return fmt.Errorf("no changes made")
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

func normalizeWhitespace(context, lines []string) []string {
	var result []string
	for _, line := range lines {
		// Collapse multiple spaces to single space
		normalized := strings.Join(strings.Fields(line), " ")
		result = append(result, normalized)
	}
	return result
}

func normalizeContent(content string) string {
	normalized := strings.Join(strings.Fields(content), " ")
	return normalized
}

func findActualLocation(content, search string, _ int) int {
	fields := strings.Fields(content)
	searchFields := strings.Fields(search)

	for i := 0; i < len(fields); i++ {
		if i+len(searchFields) > len(fields) {
			continue
		}

		match := true
		for j := range searchFields {
			if fields[i+j] != searchFields[j] {
				match = false
				break
			}
		}

		if match {
			return strings.Index(content, fields[i])
		}
	}

	return -1
}

// DetectUnifiedDiffs checks if output contains unified diffs
func DetectUnifiedDiffs(output string) bool {
	return strings.Contains(output, "--- a/") &&
		strings.Contains(output, "+++ b/") &&
		strings.Contains(output, "@@")
}

// CountUnifiedDiffs returns the number of unified diff blocks
func CountUnifiedDiffs(output string) int {
	return strings.Count(output, "--- a/")
}

// ValidateUnifiedDiffs checks if diffs are well-formed
func ValidateUnifiedDiffs(diffs []UnifiedDiff) []string {
	var errors []string

	for i, diff := range diffs {
		if diff.NewFile == "" {
			errors = append(errors, fmt.Sprintf("Diff %d: missing file path", i+1))
			continue
		}
		if len(diff.Hunks) == 0 {
			errors = append(errors, fmt.Sprintf("Diff %d (%s): no hunks", i+1, diff.NewFile))
		}
	}

	return errors
}
