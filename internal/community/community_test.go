package community

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// FormatIssuesByType
// ---------------------------------------------------------------------------

func TestFormatIssuesByType_Empty(t *testing.T) {
	result := FormatIssuesByType(map[IssueType][]Issue{})
	if result != "" {
		t.Errorf("expected empty string for empty map, got %q", result)
	}
}

func TestFormatIssuesByType_NilMap(t *testing.T) {
	result := FormatIssuesByType(nil)
	if result != "" {
		t.Errorf("expected empty string for nil map, got %q", result)
	}
}

func TestFormatIssuesByType_AgentInput(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeInput: {
			{Number: 42, Title: "Add dark mode", Body: "Please add dark mode support.", NetVotes: 5, URL: "https://github.com/example/repo/issues/42"},
		},
	}
	result := FormatIssuesByType(issues)

	if !strings.Contains(result, "Community Suggestions") {
		t.Error("expected 'Community Suggestions' section header")
	}
	if !strings.Contains(result, "#42") {
		t.Error("expected issue number #42")
	}
	if !strings.Contains(result, "Add dark mode") {
		t.Error("expected issue title")
	}
	if !strings.Contains(result, "+5") {
		t.Error("expected net votes +5")
	}
}

func TestFormatIssuesByType_AgentSelf(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeSelf: {
			{Number: 7, Title: "Refactor auth", Body: "Cleanup auth module.", NetVotes: 0, URL: "https://github.com/example/repo/issues/7"},
		},
	}
	result := FormatIssuesByType(issues)

	if !strings.Contains(result, "Self-Generated TODOs") {
		t.Error("expected 'Self-Generated TODOs' section header")
	}
	if !strings.Contains(result, "#7") {
		t.Error("expected issue number #7")
	}
}

func TestFormatIssuesByType_HelpWanted(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeHelpWanted: {
			{Number: 99, Title: "Add CI tests", Body: "Need CI pipeline.", NetVotes: 3, URL: "https://github.com/example/repo/issues/99"},
		},
	}
	result := FormatIssuesByType(issues)

	if !strings.Contains(result, "Help Wanted") {
		t.Error("expected 'Help Wanted' section header")
	}
	if !strings.Contains(result, "#99") {
		t.Error("expected issue number #99")
	}
}

func TestFormatIssuesByType_AllTypes(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeInput:      {{Number: 1, Title: "Suggestion", NetVotes: 2}},
		IssueTypeSelf:       {{Number: 2, Title: "Self TODO", NetVotes: 0}},
		IssueTypeHelpWanted: {{Number: 3, Title: "Help!", NetVotes: 1}},
	}
	result := FormatIssuesByType(issues)

	for _, expected := range []string{"Community Suggestions", "Self-Generated TODOs", "Help Wanted"} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected %q in output", expected)
		}
	}
}

func TestFormatIssuesByType_EmptySliceForType(t *testing.T) {
	// A type key present but with empty slice — should produce no section.
	issues := map[IssueType][]Issue{
		IssueTypeInput: {},
	}
	result := FormatIssuesByType(issues)
	if strings.Contains(result, "Community Suggestions") {
		t.Error("empty issue list should not produce a section header")
	}
}

func TestFormatIssuesByType_MultipleIssues(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeInput: {
			{Number: 1, Title: "First", NetVotes: 10},
			{Number: 2, Title: "Second", NetVotes: 5},
			{Number: 3, Title: "Third", NetVotes: 1},
		},
	}
	result := FormatIssuesByType(issues)
	for _, n := range []string{"#1", "#2", "#3"} {
		if !strings.Contains(result, n) {
			t.Errorf("expected %q in output", n)
		}
	}
}

// ---------------------------------------------------------------------------
// FormatDiscussions
// ---------------------------------------------------------------------------

func TestFormatDiscussions_Empty(t *testing.T) {
	result := FormatDiscussions(nil)
	if result != "" {
		t.Errorf("expected empty string for nil discussions, got %q", result)
	}
	result = FormatDiscussions([]Discussion{})
	if result != "" {
		t.Errorf("expected empty string for empty discussions, got %q", result)
	}
}

func TestFormatDiscussions_Single(t *testing.T) {
	d := []Discussion{
		{
			Number:   5,
			Title:    "How to use streaming?",
			Body:     "I want to learn about streaming tokens.",
			Category: "Q&A",
			URL:      "https://github.com/example/repo/discussions/5",
			Comments: 3,
		},
	}
	result := FormatDiscussions(d)

	if !strings.Contains(result, "GitHub Discussions") {
		t.Error("expected section header")
	}
	if !strings.Contains(result, "#5") {
		t.Error("expected discussion number")
	}
	if !strings.Contains(result, "How to use streaming?") {
		t.Error("expected discussion title")
	}
	if !strings.Contains(result, "Q&A") {
		t.Error("expected category")
	}
	if !strings.Contains(result, "3 comments") {
		t.Error("expected comment count")
	}
}

func TestFormatDiscussions_Multiple(t *testing.T) {
	discussions := []Discussion{
		{Number: 1, Title: "Alpha", Category: "General", Comments: 10},
		{Number: 2, Title: "Beta", Category: "Ideas", Comments: 5},
	}
	result := FormatDiscussions(discussions)

	if !strings.Contains(result, "#1") || !strings.Contains(result, "#2") {
		t.Error("expected both discussion numbers")
	}
	if !strings.Contains(result, "Alpha") || !strings.Contains(result, "Beta") {
		t.Error("expected both discussion titles")
	}
}

// ---------------------------------------------------------------------------
// Issue vote-sort (via sort logic reflected in FormatIssuesByType order)
// ---------------------------------------------------------------------------

func TestFormatIssuesByType_VoteOrder(t *testing.T) {
	// FetchIssues sorts by NetVotes descending before returning.
	// We can't call FetchIssues directly (needs token), but we can verify
	// that FormatIssuesByType preserves the slice order it receives.
	// Callers are responsible for sorting; we verify format preserves order.
	issues := map[IssueType][]Issue{
		IssueTypeInput: {
			{Number: 10, Title: "High votes", NetVotes: 100},
			{Number: 20, Title: "Low votes", NetVotes: 1},
		},
	}
	result := FormatIssuesByType(issues)

	idxHigh := strings.Index(result, "#10")
	idxLow := strings.Index(result, "#20")
	if idxHigh < 0 || idxLow < 0 {
		t.Fatal("expected both issues in output")
	}
	if idxHigh > idxLow {
		t.Error("expected high-vote issue (#10) to appear before low-vote issue (#20)")
	}
}

// ---------------------------------------------------------------------------
// Discussion struct fields
// ---------------------------------------------------------------------------

func TestDiscussion_Fields(t *testing.T) {
	d := Discussion{
		Number:     42,
		Title:      "Test",
		Body:       "body text",
		Category:   "Ideas",
		Author:     "octocat",
		URL:        "https://github.com",
		Comments:   7,
		IsAnswered: true,
	}
	if d.Number != 42 || d.Author != "octocat" || !d.IsAnswered {
		t.Errorf("unexpected discussion fields: %+v", d)
	}
}

// ---------------------------------------------------------------------------
// Issue struct fields
// ---------------------------------------------------------------------------

func TestIssue_Fields(t *testing.T) {
	i := Issue{
		Number:   5,
		Title:    "Bug",
		Body:     "body",
		NetVotes: -1,
		URL:      "https://github.com",
		Type:     IssueTypeInput,
	}
	if i.NetVotes != -1 || i.Type != IssueTypeInput {
		t.Errorf("unexpected issue fields: %+v", i)
	}
}
