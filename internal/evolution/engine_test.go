package evolution

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// Instantiation and builder methods
// ---------------------------------------------------------------------------

func TestNew_NonNil(t *testing.T) {
	e := New("/tmp/repo", slog.Default())
	if e == nil {
		t.Fatal("New returned nil")
	}
	if e.repoPath != "/tmp/repo" {
		t.Errorf("expected repoPath /tmp/repo, got %q", e.repoPath)
	}
}

func TestWithEventSink(t *testing.T) {
	ch := make(chan iteragent.Event, 1)
	e := New("/tmp/repo", slog.Default()).WithEventSink(ch)
	if e.eventSink == nil {
		t.Error("eventSink should be set")
	}
}

func TestWithThinking(t *testing.T) {
	e := New("/tmp/repo", slog.Default()).WithThinking(iteragent.ThinkingLevelMedium)
	if e.thinkingLevel != iteragent.ThinkingLevelMedium {
		t.Errorf("expected thinking medium, got %q", e.thinkingLevel)
	}
}

// ---------------------------------------------------------------------------
// parseSessionPlanTasks
// ---------------------------------------------------------------------------

func TestParseSessionPlanTasks_Empty(t *testing.T) {
	tasks := parseSessionPlanTasks("")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for empty plan, got %d", len(tasks))
	}
}

func TestParseSessionPlanTasks_SingleTask(t *testing.T) {
	plan := `## Session Plan

### Task 1: Add logging
Files: main.go
Description: Add structured logging
Issue: none
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Number != 1 {
		t.Errorf("expected task number 1, got %d", tasks[0].Number)
	}
	if tasks[0].Title != "Add logging" {
		t.Errorf("expected title 'Add logging', got %q", tasks[0].Title)
	}
	if !strings.Contains(tasks[0].Description, "Add logging") {
		t.Errorf("description should contain task header, got %q", tasks[0].Description)
	}
}

func TestParseSessionPlanTasks_MultipleTasks(t *testing.T) {
	plan := `## Session Plan

### Task 1: Fix bug
Files: bug.go
Description: Fix the bug.

### Task 2: Add feature
Files: feature.go
Description: Add the feature.

### Issue Responses
- #1: implement — will do it
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d: %+v", len(tasks), tasks)
	}
	if tasks[0].Number != 1 || tasks[1].Number != 2 {
		t.Errorf("unexpected task numbers: %d, %d", tasks[0].Number, tasks[1].Number)
	}
}

func TestParseSessionPlanTasks_StopsAtIssueResponses(t *testing.T) {
	plan := `### Task 1: Something

### Issue Responses
- #5: wontfix — out of scope

### Task 2: Should not be parsed
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task (stopped at Issue Responses), got %d", len(tasks))
	}
}

func TestParseSessionPlanTasks_NoColonInTitle(t *testing.T) {
	plan := `### Task 3 Title Without Colon
Description here.
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Number != 3 {
		t.Errorf("expected task number 3, got %d", tasks[0].Number)
	}
}

// ---------------------------------------------------------------------------
// parseIssueResponses
// ---------------------------------------------------------------------------

func TestParseIssueResponses_Empty(t *testing.T) {
	resp := parseIssueResponses("")
	if len(resp) != 0 {
		t.Errorf("expected 0 responses, got %d", len(resp))
	}
}

func TestParseIssueResponses_Implement(t *testing.T) {
	plan := `### Issue Responses
- #42: implement — will add it this sprint
`
	resp := parseIssueResponses(plan)
	if len(resp) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resp))
	}
	if resp[0].IssueNum != 42 {
		t.Errorf("expected issue #42, got #%d", resp[0].IssueNum)
	}
	if resp[0].Status != "implement" {
		t.Errorf("expected status 'implement', got %q", resp[0].Status)
	}
	if resp[0].Reason != "will add it this sprint" {
		t.Errorf("expected reason 'will add it this sprint', got %q", resp[0].Reason)
	}
}

func TestParseIssueResponses_Wontfix(t *testing.T) {
	plan := `### Issue Responses
- #7: wontfix — out of scope
`
	resp := parseIssueResponses(plan)
	if len(resp) != 1 || resp[0].Status != "wontfix" {
		t.Errorf("expected wontfix response, got %+v", resp)
	}
}

func TestParseIssueResponses_Multiple(t *testing.T) {
	plan := `### Issue Responses
- #1: implement — yes
- #2: wontfix — no
- #3: partial — maybe
`
	resp := parseIssueResponses(plan)
	if len(resp) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(resp))
	}
	statuses := map[int]string{}
	for _, r := range resp {
		statuses[r.IssueNum] = r.Status
	}
	if statuses[1] != "implement" || statuses[2] != "wontfix" || statuses[3] != "partial" {
		t.Errorf("unexpected statuses: %v", statuses)
	}
}

func TestParseIssueResponses_StopsAtNextSection(t *testing.T) {
	plan := `### Issue Responses
- #1: implement — yes

### Something Else
- #2: implement — should not be parsed
`
	resp := parseIssueResponses(plan)
	if len(resp) != 1 {
		t.Errorf("expected 1 response (stopped at next section), got %d", len(resp))
	}
}

func TestParseIssueResponses_LowercaseSection(t *testing.T) {
	plan := `### Issue responses
- #5: wontfix — no
`
	resp := parseIssueResponses(plan)
	if len(resp) != 1 {
		t.Errorf("expected lowercase section header to be recognized, got %d", len(resp))
	}
}

// ---------------------------------------------------------------------------
// extractCommitMessage
// ---------------------------------------------------------------------------

func TestExtractCommitMessage_FeatLine(t *testing.T) {
	output := "Some output\nfeat: add streaming support\nmore output"
	msg := extractCommitMessage(output)
	if msg != "feat: add streaming support" {
		t.Errorf("expected feat: line, got %q", msg)
	}
}

func TestExtractCommitMessage_FixLine(t *testing.T) {
	output := "fix: handle nil pointer"
	msg := extractCommitMessage(output)
	if msg != "fix: handle nil pointer" {
		t.Errorf("expected fix: line, got %q", msg)
	}
}

func TestExtractCommitMessage_FallbackDate(t *testing.T) {
	output := "no conventional commit here"
	msg := extractCommitMessage(output)
	if !strings.HasPrefix(msg, "iterate: session ") {
		t.Errorf("expected date fallback, got %q", msg)
	}
}

func TestExtractCommitMessage_CaseInsensitive(t *testing.T) {
	output := "FEAT: uppercase commit"
	msg := extractCommitMessage(output)
	if msg != "FEAT: uppercase commit" {
		t.Errorf("expected case-insensitive match, got %q", msg)
	}
}

// ---------------------------------------------------------------------------
// extractJournalTitle
// ---------------------------------------------------------------------------

func TestExtractJournalTitle_FeatLine(t *testing.T) {
	output := "Some text\nfeat: new feature added\nmore text"
	title := extractJournalTitle(output, true)
	if title != "feat: new feature added" {
		t.Errorf("expected feat line as title, got %q", title)
	}
}

func TestExtractJournalTitle_SuccessFallback(t *testing.T) {
	title := extractJournalTitle("no commit message here", true)
	if title != "evolution session" {
		t.Errorf("expected 'evolution session' fallback, got %q", title)
	}
}

func TestExtractJournalTitle_FailureFallback(t *testing.T) {
	title := extractJournalTitle("", false)
	if !strings.Contains(title, "no changes") {
		t.Errorf("expected failure fallback, got %q", title)
	}
}

// ---------------------------------------------------------------------------
// firstLine / truncate helpers
// ---------------------------------------------------------------------------

func TestFirstLine_MultiLine(t *testing.T) {
	s := "line one\nline two\nline three"
	if got := firstLine(s); got != "line one" {
		t.Errorf("expected 'line one', got %q", got)
	}
}

func TestFirstLine_SingleLine(t *testing.T) {
	if got := firstLine("only line"); got != "only line" {
		t.Errorf("expected 'only line', got %q", got)
	}
}

func TestFirstLine_Empty(t *testing.T) {
	if got := firstLine(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncate_Short(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should not be truncated, got %q", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := truncate(long, 10)
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("long string should end with ...[truncated], got %q", got)
	}
	if len(got) > 30 {
		t.Errorf("truncated string too long: %d chars", len(got))
	}
}

// ---------------------------------------------------------------------------
// WriteLearningsToMemory (disk I/O smoke test)
// ---------------------------------------------------------------------------

func TestWriteLearningsToMemory_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	if err := e.WriteLearningsToMemory("test title", "test context", "test takeaway"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatalf("learnings.jsonl not created: %v", err)
	}
	if !strings.Contains(string(data), "test title") {
		t.Errorf("learnings.jsonl should contain the title, got: %s", string(data))
	}
}

func TestWriteLearningsToMemory_Appends(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	for i := 0; i < 3; i++ {
		if err := e.WriteLearningsToMemory("entry", "ctx", ""); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d", len(lines))
	}
}
