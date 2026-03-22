package evolution

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// extractJournalTitle additional edge cases
// ---------------------------------------------------------------------------

func TestExtractJournalTitle_MultiplePrefixesInOutput(t *testing.T) {
	output := "noise\nfeat: first feature\nfix: second fix\nmore noise"
	title := extractJournalTitle(output, true)
	if title != "feat: first feature" {
		t.Errorf("expected first matching prefix, got %q", title)
	}
}

// ---------------------------------------------------------------------------
// buildJournalBody additional edge cases
// ---------------------------------------------------------------------------

func TestBuildJournalBody_EmptyOutputGivesProvider(t *testing.T) {
	body := buildJournalBody("", "test", 1000000000)
	if !strings.Contains(body, "Provider: test") {
		t.Error("body should contain provider even for empty output")
	}
}

func TestBuildJournalBody_SkipsEmptyLines(t *testing.T) {
	output := "line one is long enough for inclusion\n\nline two is long enough for inclusion"
	body := buildJournalBody(output, "test", 1000000000)
	if strings.Contains(body, "\n\n\n") {
		t.Error("should not have double blank lines")
	}
}

func TestBuildJournalBody_VeryLongLinesSkipped(t *testing.T) {
	longLine := strings.Repeat("x", 300)
	output := longLine + "\nline that is long enough for inclusion in body"
	body := buildJournalBody(output, "test", 1000000000)
	if strings.Contains(body, strings.Repeat("x", 300)) {
		t.Error("very long lines should be excluded")
	}
}

// ---------------------------------------------------------------------------
// extractCommitMessage additional edge cases
// ---------------------------------------------------------------------------

func TestExtractCommitMessage_EmptyOutput(t *testing.T) {
	msg := extractCommitMessage("")
	if !strings.Contains(msg, "iterate: session") {
		t.Errorf("expected default session message, got %q", msg)
	}
}

func TestExtractCommitMessage_NoPrefixFallback(t *testing.T) {
	msg := extractCommitMessage("some random output\nno prefixes here")
	if !strings.Contains(msg, "iterate: session") {
		t.Errorf("expected default session message, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// extractCommitLines additional edge cases
// ---------------------------------------------------------------------------

func TestExtractCommitLines_WhitespaceOnlyInput(t *testing.T) {
	lines := extractCommitLines("   \n  \n   ")
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestExtractCommitLines_UnderMaxLength(t *testing.T) {
	prefix := "feat: "
	content := strings.Repeat("x", 110)
	output := prefix + content
	lines := extractCommitLines(output)
	if len(lines) != 1 {
		t.Errorf("expected 1 line under limit, got %d", len(lines))
	}
}

func TestExtractCommitLines_OverMaxLength(t *testing.T) {
	prefix := "feat: "
	content := strings.Repeat("x", 200)
	output := prefix + content
	lines := extractCommitLines(output)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines over limit, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// firstLine additional edge cases
// ---------------------------------------------------------------------------

func TestFirstLine_NewlineOnlyInput(t *testing.T) {
	got := firstLine("\n")
	if got != "" {
		t.Errorf("expected empty for newline-only, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// categorizeJournalEntry additional edge cases
// ---------------------------------------------------------------------------

func TestCategorizeJournalEntry_WhitespaceOnly(t *testing.T) {
	got := categorizeJournalEntry("   ")
	if got != "" {
		t.Errorf("expected empty for whitespace, got %q", got)
	}
}

func TestCategorizeJournalEntry_Unicode(t *testing.T) {
	got := categorizeJournalEntry("添加新功能")
	_ = got // just verify no panic
}
