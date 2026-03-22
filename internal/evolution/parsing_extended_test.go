package evolution

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// planTask struct
// ---------------------------------------------------------------------------

func TestPlanTask_Fields(t *testing.T) {
	task := planTask{
		Number:      1,
		Title:       "Test Task",
		Description: "A test description",
	}
	if task.Number != 1 {
		t.Errorf("expected Number 1, got %d", task.Number)
	}
	if task.Title != "Test Task" {
		t.Errorf("expected Title 'Test Task', got %q", task.Title)
	}
	if task.Description != "A test description" {
		t.Errorf("expected Description 'A test description', got %q", task.Description)
	}
}

// ---------------------------------------------------------------------------
// issueResponse struct
// ---------------------------------------------------------------------------

func TestIssueResponse_Fields(t *testing.T) {
	resp := issueResponse{
		IssueNum: 42,
		Status:   "implement",
		Reason:   "add streaming",
	}
	if resp.IssueNum != 42 {
		t.Errorf("expected IssueNum 42, got %d", resp.IssueNum)
	}
	if resp.Status != "implement" {
		t.Errorf("expected Status 'implement', got %q", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// parseSessionPlanTasks additional edge cases
// ---------------------------------------------------------------------------

func TestParseSessionPlanTasks_NoColonAdditional(t *testing.T) {
	plan := `### Task 7 Without colon here
Description of task.
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Number != 7 {
		t.Errorf("expected task number 7, got %d", tasks[0].Number)
	}
}

// ---------------------------------------------------------------------------
// parseIssueResponses additional edge cases
// ---------------------------------------------------------------------------

func TestParseIssueResponses_IgnoreNonIssueLines(t *testing.T) {
	plan := `### Issue Responses
Some text that's not an issue line
- #1 implement
Another non-issue line
- #2 wontfix
`
	responses := parseIssueResponses(plan)
	if len(responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(responses))
	}
}

// ---------------------------------------------------------------------------
// extractIssueNumbers additional edge cases
// ---------------------------------------------------------------------------

func TestExtractIssueNumbers_OnlyComment(t *testing.T) {
	plan := `### Issue Responses
- #1 some comment
- #2 another comment
`
	nums := extractIssueNumbers(plan)
	if len(nums) != 0 {
		t.Errorf("expected 0 numbers (only comments), got %d", len(nums))
	}
}

// ---------------------------------------------------------------------------
// buildPRBody additional edge cases
// ---------------------------------------------------------------------------

func TestBuildPRBody_EmptyPlanHasSections(t *testing.T) {
	body := buildPRBody("", "some output")
	if !strings.Contains(body, "Changes") {
		t.Error("expected Changes section even for empty plan")
	}
	if !strings.Contains(body, "Tasks") {
		t.Error("expected Tasks section even for empty plan")
	}
}

func TestBuildPRBody_MultipleCommitLines(t *testing.T) {
	plan := "### Task 1: Task one\n"
	output := "feat: first\nfix: second\ndocs: third"
	body := buildPRBody(plan, output)

	if !strings.Contains(body, "feat: first") {
		t.Error("expected first commit line")
	}
	if !strings.Contains(body, "fix: second") {
		t.Error("expected second commit line")
	}
	if !strings.Contains(body, "docs: third") {
		t.Error("expected third commit line")
	}
}

// ---------------------------------------------------------------------------
// extractSessionTitle additional edge cases
// ---------------------------------------------------------------------------

func TestExtractSessionTitle_NoTitleLine(t *testing.T) {
	plan := "Some content\nNo title here"
	title := extractSessionTitle(plan)
	if title != "" {
		t.Errorf("expected empty, got %q", title)
	}
}
