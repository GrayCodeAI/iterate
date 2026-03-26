package evolution

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// groupTasksByFileOverlap
// ---------------------------------------------------------------------------

func TestGroupTasksByFileOverlap_NoTasks(t *testing.T) {
	waves := groupTasksByFileOverlap(nil)
	if len(waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(waves))
	}
}

func TestGroupTasksByFileOverlap_SingleTask(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Title: "T1", Files: []string{"foo.go"}},
	}
	waves := groupTasksByFileOverlap(tasks)
	if len(waves) != 1 || len(waves[0]) != 1 {
		t.Errorf("expected 1 wave with 1 task, got %v", waves)
	}
}

func TestGroupTasksByFileOverlap_NonOverlappingInSameWave(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Files: []string{"a.go"}},
		{Number: 2, Files: []string{"b.go"}},
		{Number: 3, Files: []string{"c.go"}},
	}
	waves := groupTasksByFileOverlap(tasks)
	if len(waves) != 1 {
		t.Errorf("expected 1 wave (no overlaps), got %d", len(waves))
	}
	if len(waves[0]) != 3 {
		t.Errorf("expected 3 tasks in wave, got %d", len(waves[0]))
	}
}

func TestGroupTasksByFileOverlap_OverlapForcesNewWave(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Files: []string{"shared.go"}},
		{Number: 2, Files: []string{"other.go"}},
		{Number: 3, Files: []string{"shared.go"}}, // conflicts with task 1
	}
	waves := groupTasksByFileOverlap(tasks)
	if len(waves) != 2 {
		t.Errorf("expected 2 waves (task 3 conflicts with task 1), got %d", len(waves))
	}
	// Wave 1 should have tasks 1 and 2; wave 2 should have task 3.
	if len(waves[0]) != 2 {
		t.Errorf("wave 1: expected 2 tasks, got %d", len(waves[0]))
	}
	if len(waves[1]) != 1 || waves[1][0].Number != 3 {
		t.Errorf("wave 2: expected task 3, got %v", waves[1])
	}
}

func TestGroupTasksByFileOverlap_NoFilesForcesOwnWave(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Files: []string{"a.go"}},
		{Number: 2, Files: nil}, // no files → own wave
		{Number: 3, Files: []string{"b.go"}},
	}
	waves := groupTasksByFileOverlap(tasks)
	// Task 1 in wave 1, task 2 alone in wave 2, task 3 starts wave 3.
	if len(waves) != 3 {
		t.Errorf("expected 3 waves, got %d", len(waves))
	}
	if waves[1][0].Number != 2 {
		t.Errorf("expected task 2 alone in wave 2, got task %d", waves[1][0].Number)
	}
}

func TestGroupTasksByFileOverlap_MultipleFileOverlap(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Files: []string{"a.go", "b.go"}},
		{Number: 2, Files: []string{"c.go", "b.go"}}, // b.go overlaps
	}
	waves := groupTasksByFileOverlap(tasks)
	if len(waves) != 2 {
		t.Errorf("expected 2 waves, got %d", len(waves))
	}
}

func TestGroupTasksByFileOverlap_OrderPreserved(t *testing.T) {
	tasks := []planTask{
		{Number: 1, Files: []string{"x.go"}},
		{Number: 2, Files: []string{"y.go"}},
		{Number: 3, Files: []string{"x.go"}},
		{Number: 4, Files: []string{"z.go"}},
	}
	waves := groupTasksByFileOverlap(tasks)
	// Wave 1: tasks 1, 2 (no overlap). Wave 2: tasks 3, 4 (3 conflicts with 1; 4 is fine with 3).
	if len(waves) != 2 {
		t.Errorf("expected 2 waves, got %d", len(waves))
	}
	if waves[0][0].Number != 1 || waves[0][1].Number != 2 {
		t.Errorf("wave 1 order wrong: %v", waves[0])
	}
	if waves[1][0].Number != 3 || waves[1][1].Number != 4 {
		t.Errorf("wave 2 order wrong: %v", waves[1])
	}
}

// ---------------------------------------------------------------------------
// planTask Files parsing (via parseSessionPlanTasks)
// ---------------------------------------------------------------------------

func TestParseSessionPlanTasks_FilesField(t *testing.T) {
	plan := `## Session Plan

Session Title: Fix things

### Task 1: Fix parser
Files: internal/parser/parser.go, internal/parser/lexer.go
Description: Fix off-by-one error.

### Task 2: Add auth
Files: internal/auth/middleware.go
Description: Add JWT middleware.

### Issue Responses
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if len(tasks[0].Files) != 2 {
		t.Errorf("task 1: expected 2 files, got %v", tasks[0].Files)
	}
	if tasks[0].Files[0] != "internal/parser/parser.go" {
		t.Errorf("task 1 file 0: got %q", tasks[0].Files[0])
	}
	if len(tasks[1].Files) != 1 || tasks[1].Files[0] != "internal/auth/middleware.go" {
		t.Errorf("task 2 files: got %v", tasks[1].Files)
	}
}

func TestParseSessionPlanTasks_NoFilesField(t *testing.T) {
	plan := `## Session Plan

### Task 1: Improve something
Description: Do the thing.
`
	tasks := parseSessionPlanTasks(plan)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if len(tasks[0].Files) != 0 {
		t.Errorf("expected no files, got %v", tasks[0].Files)
	}
}

// ---------------------------------------------------------------------------
// recentFailures
// ---------------------------------------------------------------------------

func TestRecentFailures_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result := recentFailures(dir, 10)
	if result != "" {
		t.Errorf("expected empty string for missing failures.jsonl, got %q", result)
	}
}

func TestRecentFailures_ReturnsFormatted(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	_ = os.MkdirAll(memDir, 0o755)

	entries := []map[string]interface{}{
		{"type": "failure", "day": 3, "task": "Fix parser", "reason": "build failed"},
		{"type": "failure", "day": 4, "task": "Add auth", "reason": "test failed"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	_ = os.WriteFile(filepath.Join(memDir, "failures.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	result := recentFailures(dir, 10)
	if !strings.Contains(result, "Fix parser") {
		t.Errorf("expected 'Fix parser' in output, got %q", result)
	}
	if !strings.Contains(result, "Add auth") {
		t.Errorf("expected 'Add auth' in output, got %q", result)
	}
	if !strings.Contains(result, "Day 3") {
		t.Errorf("expected 'Day 3' in output, got %q", result)
	}
}

func TestRecentFailures_RespectsLimit(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	_ = os.MkdirAll(memDir, 0o755)

	var lines []string
	for i := 1; i <= 15; i++ {
		e := map[string]interface{}{"type": "failure", "day": i, "task": "Task", "reason": "fail"}
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	_ = os.WriteFile(filepath.Join(memDir, "failures.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	result := recentFailures(dir, 5)
	// Should contain days 11-15 (last 5), not day 1.
	if strings.Contains(result, "Day 1\n") || strings.Contains(result, "Day 1 —") {
		t.Errorf("expected day 1 to be trimmed by limit, got %q", result)
	}
	if !strings.Contains(result, "Day 15") {
		t.Errorf("expected day 15 in output, got %q", result)
	}
}

func TestRecentFailures_CorruptLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	_ = os.MkdirAll(memDir, 0o755)

	content := `{"type":"failure","day":1,"task":"Good","reason":"ok"}
not valid json
{"type":"failure","day":2,"task":"Also good","reason":"ok"}
`
	_ = os.WriteFile(filepath.Join(memDir, "failures.jsonl"), []byte(content), 0o644)

	result := recentFailures(dir, 10)
	if !strings.Contains(result, "Good") {
		t.Errorf("expected 'Good' in output, got %q", result)
	}
	if !strings.Contains(result, "Also good") {
		t.Errorf("expected 'Also good' in output, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// appendFailureJSONL
// ---------------------------------------------------------------------------

func TestAppendFailureJSONL_WritesEntry(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("7"), 0o644)

	e := New(dir, slog.Default())
	if err := e.appendFailureJSONL("My task", "build error"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "failures.jsonl"))
	if err != nil {
		t.Fatalf("failures.jsonl not created: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("invalid JSON written: %v", err)
	}
	if entry["task"] != "My task" {
		t.Errorf("task mismatch: got %v", entry["task"])
	}
	if entry["reason"] != "build error" {
		t.Errorf("reason mismatch: got %v", entry["reason"])
	}
	if int(entry["day"].(float64)) != 7 {
		t.Errorf("day mismatch: got %v", entry["day"])
	}
}

func TestAppendFailureJSONL_AppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())

	_ = e.appendFailureJSONL("Task A", "reason A")
	_ = e.appendFailureJSONL("Task B", "reason B")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "failures.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), string(data))
	}
}

// ---------------------------------------------------------------------------
// clearSessionPlan
// ---------------------------------------------------------------------------

func TestClearSessionPlan_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "SESSION_PLAN.md")
	_ = os.WriteFile(planPath, []byte("## plan"), 0o644)

	e := New(dir, slog.Default())
	e.clearSessionPlan()

	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Error("expected SESSION_PLAN.md to be deleted")
	}
}

func TestClearSessionPlan_NoErrorIfMissing(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	// Should not panic or error when file doesn't exist.
	e.clearSessionPlan()
}

// ---------------------------------------------------------------------------
// trimFailuresJSONL
// ---------------------------------------------------------------------------

func TestTrimFailuresJSONL_RemovesOldEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failures.jsonl")

	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour) // 40 days ago — should be trimmed
	recent := now.Add(-5 * 24 * time.Hour) // 5 days ago — should be kept

	lines := []string{
		`{"type":"failure","day":1,"task":"Old task","reason":"old","ts":"` + old.Format(time.RFC3339) + `"}`,
		`{"type":"failure","day":2,"task":"Recent task","reason":"new","ts":"` + recent.Format(time.RFC3339) + `"}`,
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	trimFailuresJSONL(path, 30*24*time.Hour)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read trimmed file: %v", err)
	}
	if strings.Contains(string(data), "Old task") {
		t.Error("expected old entry to be trimmed, but it is still present")
	}
	if !strings.Contains(string(data), "Recent task") {
		t.Error("expected recent entry to be kept, but it is missing")
	}
}

func TestTrimFailuresJSONL_KeepsAllIfAllRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "failures.jsonl")

	recent := time.Now().UTC().Add(-1 * 24 * time.Hour)
	lines := []string{
		`{"type":"failure","day":1,"task":"A","reason":"x","ts":"` + recent.Format(time.RFC3339) + `"}`,
		`{"type":"failure","day":2,"task":"B","reason":"y","ts":"` + recent.Format(time.RFC3339) + `"}`,
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	trimFailuresJSONL(path, 30*24*time.Hour)

	data, _ := os.ReadFile(path)
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(got) != 2 {
		t.Errorf("expected 2 lines, got %d", len(got))
	}
}

func TestTrimFailuresJSONL_NoopOnMissingFile(t *testing.T) {
	// Should not panic or error.
	trimFailuresJSONL(filepath.Join(t.TempDir(), "nonexistent.jsonl"), 30*24*time.Hour)
}
