package evolution

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// categorizeJournalEntry
// ---------------------------------------------------------------------------

func TestCategorizeJournalEntry(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "fix keyword returns bug emoji",
			content: "fix: handle nil pointer dereference",
			want:    "🐛",
		},
		{
			name:    "bug keyword returns bug emoji",
			content: "bug in authentication flow",
			want:    "🐛",
		},
		{
			name:    "broken keyword returns bug emoji",
			content: "broken test suite repaired",
			want:    "🐛",
		},
		{
			name:    "revert keyword returns bug emoji",
			content: "revert previous change",
			want:    "🐛",
		},
		{
			name:    "fix takes priority over feat",
			content: "feat: fix the broken feature",
			want:    "🐛",
		},
		{
			name:    "feat keyword returns rocket emoji",
			content: "feat: add streaming support",
			want:    "🚀",
		},
		{
			name:    "implement keyword returns rocket emoji",
			content: "implement new parser",
			want:    "🚀",
		},
		{
			name:    "add with trailing space returns rocket emoji",
			content: "add new test cases",
			want:    "🚀",
		},
		{
			name:    "feature keyword returns rocket emoji",
			content: "feature: dark mode support",
			want:    "🚀",
		},
		{
			name:    "doc keyword returns memo emoji",
			content: "update doc strings",
			want:    "📝",
		},
		{
			name:    "journal keyword returns memo emoji",
			content: "journal entry for today",
			want:    "📝",
		},
		{
			name:    "readme keyword returns memo emoji",
			content: "readme update with examples",
			want:    "📝",
		},
		{
			name:    "comment keyword returns memo emoji",
			content: "added a comment explaining the algorithm",
			want:    "📝",
		},
		{
			name:    "refactor keyword returns wrench emoji",
			content: "refactor error handling",
			want:    "🔧",
		},
		{
			name:    "improve keyword returns wrench emoji",
			content: "improve performance of search",
			want:    "🔧",
		},
		{
			name:    "cleanup keyword returns wrench emoji",
			content: "cleanup unused imports",
			want:    "🔧",
		},
		{
			name:    "clean up keyword returns wrench emoji",
			content: "clean up temp files",
			want:    "🔧",
		},
		{
			name:    "optimize keyword returns wrench emoji",
			content: "optimize database queries",
			want:    "🔧",
		},
		{
			name:    "enhance keyword returns wrench emoji",
			content: "enhance user experience",
			want:    "🔧",
		},
		{
			name:    "unknown content returns empty string",
			content: "some random update",
			want:    "",
		},
		{
			name:    "empty content returns empty string",
			content: "",
			want:    "",
		},
		{
			name:    "case insensitive matching",
			content: "FEAT: uppercase feature",
			want:    "🚀",
		},
		{
			name:    "case insensitive fix",
			content: "FIX: uppercase bug fix",
			want:    "🐛",
		},
		{
			name:    "case insensitive refactor",
			content: "REFACTOR: uppercase refactor",
			want:    "🔧",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeJournalEntry(tt.content)
			if got != tt.want {
				t.Errorf("categorizeJournalEntry(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractJournalTitle (extended beyond engine_test.go)
// ---------------------------------------------------------------------------

func TestExtractJournalTitle_FixLine(t *testing.T) {
	output := "Something happened\nfix: correct off-by-one error\nmore text"
	title := extractJournalTitle(output, true)
	if title != "fix: correct off-by-one error" {
		t.Errorf("expected fix line as title, got %q", title)
	}
}

func TestExtractJournalTitle_RefactorLine(t *testing.T) {
	output := "refactor: simplify parsing logic"
	title := extractJournalTitle(output, true)
	if title != "refactor: simplify parsing logic" {
		t.Errorf("expected refactor line as title, got %q", title)
	}
}

func TestExtractJournalTitle_DocsLine(t *testing.T) {
	output := "docs: update API documentation"
	title := extractJournalTitle(output, true)
	if title != "docs: update API documentation" {
		t.Errorf("expected docs line as title, got %q", title)
	}
}

func TestExtractJournalTitle_TestLine(t *testing.T) {
	output := "test: add integration tests"
	title := extractJournalTitle(output, true)
	if title != "test: add integration tests" {
		t.Errorf("expected test line as title, got %q", title)
	}
}

func TestExtractJournalTitle_ChoreLine(t *testing.T) {
	output := "chore: update dependencies"
	title := extractJournalTitle(output, true)
	if title != "chore: update dependencies" {
		t.Errorf("expected chore line as title, got %q", title)
	}
}

func TestExtractJournalTitle_LongLineSkipped(t *testing.T) {
	longLine := "feat: " + strings.Repeat("x", 100)
	output := longLine + "\nsome other output"
	title := extractJournalTitle(output, true)
	if title != "evolution session" {
		t.Errorf("expected fallback for long line, got %q", title)
	}
}

func TestExtractJournalTitle_FailureWithNoChanges(t *testing.T) {
	title := extractJournalTitle("some output", false)
	if title != "session (no changes committed)" {
		t.Errorf("expected failure fallback, got %q", title)
	}
}

func TestExtractJournalTitle_FailureWithFeatLine(t *testing.T) {
	// Even on failure, if a feat line is found it's returned
	output := "feat: attempted feature\nbut it failed"
	title := extractJournalTitle(output, false)
	if title != "feat: attempted feature" {
		t.Errorf("expected feat line even on failure, got %q", title)
	}
}

// ---------------------------------------------------------------------------
// extractCommitMessage (extended beyond engine_test.go)
// ---------------------------------------------------------------------------

func TestExtractCommitMessage_ChoreLine(t *testing.T) {
	output := "chore: bump version"
	msg := extractCommitMessage(output)
	if msg != "chore: bump version" {
		t.Errorf("expected chore: line, got %q", msg)
	}
}

func TestExtractCommitMessage_DocsLine(t *testing.T) {
	output := "docs: update README"
	msg := extractCommitMessage(output)
	if msg != "docs: update README" {
		t.Errorf("expected docs: line, got %q", msg)
	}
}

func TestExtractCommitMessage_RefactorLine(t *testing.T) {
	output := "refactor: extract helper function"
	msg := extractCommitMessage(output)
	if msg != "refactor: extract helper function" {
		t.Errorf("expected refactor: line, got %q", msg)
	}
}

func TestExtractCommitMessage_MultipleLinesFirstMatch(t *testing.T) {
	output := "some noise\nfeat: first match\nfix: second match"
	msg := extractCommitMessage(output)
	if msg != "feat: first match" {
		t.Errorf("expected first matching line, got %q", msg)
	}
}

func TestExtractCommitMessage_CommitPrefix(t *testing.T) {
	output := "commit: my commit message"
	msg := extractCommitMessage(output)
	if msg != "commit: my commit message" {
		t.Errorf("expected commit: line, got %q", msg)
	}
}

func TestExtractCommitMessage_CaseInsensitivePrefix(t *testing.T) {
	output := "CHORE: uppercase"
	msg := extractCommitMessage(output)
	if msg != "CHORE: uppercase" {
		t.Errorf("expected case-insensitive match, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// extractCommitLines
// ---------------------------------------------------------------------------

func TestExtractCommitLines_Empty(t *testing.T) {
	lines := extractCommitLines("")
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty input, got %d", len(lines))
	}
}

func TestExtractCommitLines_SingleFeat(t *testing.T) {
	output := "feat: add new feature"
	lines := extractCommitLines(output)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "feat: add new feature" {
		t.Errorf("expected 'feat: add new feature', got %q", lines[0])
	}
}

func TestExtractCommitLines_Multiple(t *testing.T) {
	output := "some noise\nfeat: add streaming\nfix: handle nil\ndocs: update readme\nother line"
	lines := extractCommitLines(output)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "feat: add streaming" {
		t.Errorf("unexpected first line: %q", lines[0])
	}
	if lines[1] != "fix: handle nil" {
		t.Errorf("unexpected second line: %q", lines[1])
	}
	if lines[2] != "docs: update readme" {
		t.Errorf("unexpected third line: %q", lines[2])
	}
}

func TestExtractCommitLines_LongLineSkipped(t *testing.T) {
	longLine := "feat: " + strings.Repeat("x", 200)
	output := longLine + "\nfix: short fix"
	lines := extractCommitLines(output)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (long skipped), got %d", len(lines))
	}
	if lines[0] != "fix: short fix" {
		t.Errorf("expected only the short fix line, got %q", lines[0])
	}
}

func TestExtractCommitLines_TestPrefix(t *testing.T) {
	output := "test: add parser tests"
	lines := extractCommitLines(output)
	if len(lines) != 1 || lines[0] != "test: add parser tests" {
		t.Errorf("expected test prefix match, got %v", lines)
	}
}

func TestExtractCommitLines_RefactorPrefix(t *testing.T) {
	output := "refactor: simplify code"
	lines := extractCommitLines(output)
	if len(lines) != 1 || lines[0] != "refactor: simplify code" {
		t.Errorf("expected refactor prefix match, got %v", lines)
	}
}

func TestExtractCommitLines_NoMatch(t *testing.T) {
	output := "just some output\nno commit prefixes here\nanother line"
	lines := extractCommitLines(output)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines with no match, got %d", len(lines))
	}
}

func TestExtractCommitLines_CaseInsensitive(t *testing.T) {
	output := "FEAT: uppercase\nFIX: uppercase fix"
	lines := extractCommitLines(output)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}

// ---------------------------------------------------------------------------
// firstLine (extended beyond engine_test.go)
// ---------------------------------------------------------------------------

func TestFirstLine_LeadingWhitespace(t *testing.T) {
	got := firstLine("  hello  \nworld")
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestFirstLine_TrailingNewline(t *testing.T) {
	got := firstLine("only line\n")
	if got != "only line" {
		t.Errorf("expected 'only line', got %q", got)
	}
}

func TestFirstLine_OnlyWhitespace(t *testing.T) {
	got := firstLine("   ")
	if got != "" {
		t.Errorf("expected empty for whitespace-only, got %q", got)
	}
}

func TestFirstLine_MultipleNewlines(t *testing.T) {
	got := firstLine("first\n\nthird")
	if got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// buildJournalBody
// ---------------------------------------------------------------------------

func TestBuildJournalBody_BasicFormat(t *testing.T) {
	output := "line that is long enough for inclusion here and visible\ntoo short\nanother line that is long enough to be included in the body output"
	body := buildJournalBody(output, "anthropic", 5_000_000_000) // 5 seconds

	if !strings.Contains(body, "Provider: anthropic") {
		t.Errorf("body should contain provider, got %q", body)
	}
	if !strings.Contains(body, "Duration:") {
		t.Errorf("body should contain duration, got %q", body)
	}
}

func TestBuildJournalBody_SkipsShortLines(t *testing.T) {
	output := "short\na\nthis is a moderately long line that should be included"
	body := buildJournalBody(output, "openai", 1_000_000_000)

	// "short" and "a" are too short (< 20 chars), so they should be excluded
	if strings.Contains(body, "\nshort\n") {
		t.Error("short lines should be excluded from body")
	}
}

func TestBuildJournalBody_SkipsJSONLines(t *testing.T) {
	output := `{"key": "value that makes this line long enough to pass the length filter"}`
	body := buildJournalBody(output, "test", 1_000_000_000)

	if strings.Contains(body, "{") {
		t.Error("JSON lines should be excluded from body")
	}
}

func TestBuildJournalBody_SkipsArrayLines(t *testing.T) {
	output := `["item1", "item2", "item3", "item4 that makes this long enough"]`
	body := buildJournalBody(output, "test", 1_000_000_000)

	if strings.Contains(body, "[") {
		t.Error("Array lines should be excluded from body")
	}
}

func TestBuildJournalBody_MaxThreeContentLines(t *testing.T) {
	lines := []string{
		"line one that is long enough for inclusion in body",
		"line two that is long enough for inclusion in body",
		"line three that is long enough for inclusion in body",
		"line four that is long enough for inclusion in body",
		"line five that is long enough for inclusion in body",
	}
	output := strings.Join(lines, "\n")
	body := buildJournalBody(output, "test", 1_000_000_000)

	count := 0
	for _, line := range lines {
		if strings.Contains(body, line) {
			count++
		}
	}
	if count > 3 {
		t.Errorf("body should contain at most 3 content lines, got %d", count)
	}
}
