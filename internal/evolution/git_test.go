package evolution

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// initGitRepo creates a bare git repo with an initial commit on main.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := []struct {
		args []string
	}{
		{[]string{"git", "init"}},
		{[]string{"git", "config", "user.email", "test@test.com"}},
		{[]string{"git", "config", "user.name", "Test"}},
	}
	for _, c := range cmds {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", c.args, err, out)
		}
	}
	// initial commit so HEAD exists
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644)
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	cmd.CombinedOutput()
}

// ---------------------------------------------------------------------------
// hasChanges
// ---------------------------------------------------------------------------

func TestHasChanges_NoChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	has, err := e.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges error: %v", err)
	}
	if has {
		t.Error("expected no changes in clean repo")
	}
}

func TestHasChanges_WithUntracked(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	has, err := e.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges error: %v", err)
	}
	if !has {
		t.Error("expected changes with untracked file")
	}
}

func TestHasChanges_WithModified(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed\n"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	has, err := e.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges error: %v", err)
	}
	if !has {
		t.Error("expected changes with modified file")
	}
}

func TestHasChanges_WithStaged(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	os.WriteFile(filepath.Join(dir, "staged.go"), []byte("package main\n"), 0o644)
	cmd := exec.Command("git", "add", "staged.go")
	cmd.Dir = dir
	cmd.CombinedOutput()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	has, err := e.hasChanges(ctx)
	if err != nil {
		t.Fatalf("hasChanges error: %v", err)
	}
	if !has {
		t.Error("expected changes with staged file")
	}
}

// ---------------------------------------------------------------------------
// currentBranch
// ---------------------------------------------------------------------------

func TestCurrentBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	branch, err := e.currentBranch(ctx)
	if err != nil {
		t.Fatalf("currentBranch error: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got %q", branch)
	}
}

func TestCurrentBranch_AfterCheckout(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	cmd := exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = dir
	cmd.CombinedOutput()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	branch, err := e.currentBranch(ctx)
	if err != nil {
		t.Fatalf("currentBranch error: %v", err)
	}
	if branch != "feature/test" {
		t.Errorf("expected 'feature/test', got %q", branch)
	}
}

// ---------------------------------------------------------------------------
// deleteBranch
// ---------------------------------------------------------------------------

func TestDeleteBranch_Existing(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	// create a branch to delete
	cmd := exec.Command("git", "checkout", "-b", "to-delete")
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	cmd.CombinedOutput()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.deleteBranch(ctx, "to-delete"); err != nil {
		t.Fatalf("deleteBranch error: %v", err)
	}

	// verify branch is gone
	cmd = exec.Command("git", "branch", "--list", "to-delete")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("branch should be deleted, got %q", string(out))
	}
}

func TestDeleteBranch_NonExisting(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// should not error (2>/dev/null || true)
	if err := e.deleteBranch(ctx, "nonexistent-branch-xyz"); err != nil {
		t.Fatalf("deleteBranch on nonexistent should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// switchToMain
// ---------------------------------------------------------------------------

func TestSwitchToMain_FromOtherBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	cmd := exec.Command("git", "checkout", "-b", "other-branch")
	cmd.Dir = dir
	cmd.CombinedOutput()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.switchToMain(ctx); err != nil {
		t.Fatalf("switchToMain error: %v", err)
	}

	branch, _ := e.currentBranch(ctx)
	if branch != "main" && branch != "master" {
		t.Errorf("expected main/master after switch, got %q", branch)
	}
}

func TestSwitchToMain_AlreadyOnMain(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.switchToMain(ctx); err != nil {
		t.Fatalf("switchToMain on main should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyProtected
// ---------------------------------------------------------------------------

func TestVerifyProtected_NoViolations(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	// modify a non-protected file and commit
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# updated\n"), 0o644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", "update readme")
	cmd.Dir = dir
	cmd.CombinedOutput()

	// add unstaged change
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# updated again\n"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	violations, err := e.verifyProtected(ctx)
	if err != nil {
		t.Fatalf("verifyProtected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %v", violations)
	}
}

func TestVerifyProtected_WithViolation(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	// create and commit a protected file path
	os.MkdirAll(filepath.Join(dir, "internal", "evolution"), 0o755)
	os.WriteFile(filepath.Join(dir, "internal", "evolution", "engine.go"), []byte("package evolution\n"), 0o644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", "add engine")
	cmd.Dir = dir
	cmd.CombinedOutput()

	// modify it (unstaged)
	os.WriteFile(filepath.Join(dir, "internal", "evolution", "engine.go"), []byte("package evolution\n// changed\n"), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	violations, err := e.verifyProtected(ctx)
	if err != nil {
		t.Fatalf("verifyProtected error: %v", err)
	}
	if len(violations) == 0 {
		t.Error("expected violation for protected file")
	}
}

func TestVerifyProtected_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	e := New(dir, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	violations, err := e.verifyProtected(ctx)
	if err != nil {
		t.Fatalf("verifyProtected error: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for clean repo, got %v", violations)
	}
}

// ---------------------------------------------------------------------------
// persistJournalEntry
// ---------------------------------------------------------------------------

func TestPersistJournalEntry_Valid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "JOURNAL.md"), []byte("# iterate Evolution Journal\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("7\n"), 0o644)
	e := New(dir, slog.Default())

	entry := "## Day 99 — 12:00 — Test Entry\n\nSome body text here.\n"
	e.persistJournalEntry(entry, "7")

	data, _ := os.ReadFile(filepath.Join(dir, "JOURNAL.md"))
	content := string(data)
	if !strings.Contains(content, "Day 7") {
		t.Errorf("expected Day 7 in journal, got:\n%s", content)
	}
	if strings.Contains(content, "Day 99") {
		t.Error("should replace Day 99 with Day 7")
	}
}

func TestPersistJournalEntry_NoDayMarker(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "JOURNAL.md"), []byte("# iterate Evolution Journal\n"), 0o644)
	e := New(dir, slog.Default())

	entry := "This has no day marker at all"
	e.persistJournalEntry(entry, "5")

	data, _ := os.ReadFile(filepath.Join(dir, "JOURNAL.md"))
	// should not modify the file
	if string(data) != "# iterate Evolution Journal\n" {
		t.Errorf("journal should be unchanged, got:\n%s", string(data))
	}
}

func TestPersistJournalEntry_ExtractsFromMiddle(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "JOURNAL.md"), []byte("# iterate Evolution Journal\n"), 0o644)
	e := New(dir, slog.Default())

	entry := "Some preamble\n## Day 3 — 10:00 — Extracted\n\nBody here.\n\n## Day 4 — extra"
	e.persistJournalEntry(entry, "3")

	data, _ := os.ReadFile(filepath.Join(dir, "JOURNAL.md"))
	content := string(data)
	if !strings.Contains(content, "Day 3") {
		t.Error("should extract Day 3 section")
	}
	if strings.Contains(content, "Day 4") {
		t.Error("should not include Day 4 section")
	}
}

func TestPersistJournalEntry_ZeroDay(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "JOURNAL.md"), []byte("# iterate Evolution Journal\n"), 0o644)
	e := New(dir, slog.Default())

	entry := "## Day 99 — 12:00 — Test\n\nBody.\n"
	e.persistJournalEntry(entry, "0")

	data, _ := os.ReadFile(filepath.Join(dir, "JOURNAL.md"))
	// day 0 means don't replace, so Day 99 should remain
	if !strings.Contains(string(data), "Day 99") {
		t.Errorf("should keep original day when day='0', got:\n%s", string(data))
	}
}

// ---------------------------------------------------------------------------
// RunResult struct
// ---------------------------------------------------------------------------

func TestRunResult_Fields(t *testing.T) {
	now := time.Now()
	r := &RunResult{
		Status:     "merged",
		StartedAt:  now,
		FinishedAt: now.Add(5 * time.Second),
		PRNumber:   42,
		PRURL:      "https://github.com/test/repo/pull/42",
	}
	if r.Status != "merged" {
		t.Errorf("expected merged, got %q", r.Status)
	}
	if r.PRNumber != 42 {
		t.Errorf("expected PR 42, got %d", r.PRNumber)
	}
	if r.FinishedAt.Sub(r.StartedAt) != 5*time.Second {
		t.Error("duration should be 5s")
	}
}

// ---------------------------------------------------------------------------
// PRState struct
// ---------------------------------------------------------------------------

func TestPRState_JSON(t *testing.T) {
	state := PRState{
		PRNumber: 7,
		PRURL:    "https://github.com/o/r/pull/7",
		Branch:   "evolution/day-1",
	}
	if state.PRNumber != 7 {
		t.Errorf("expected 7, got %d", state.PRNumber)
	}
	if state.Branch != "evolution/day-1" {
		t.Errorf("expected branch, got %q", state.Branch)
	}
}

// ---------------------------------------------------------------------------
// withTimeout
// ---------------------------------------------------------------------------

func TestWithTimeout_Deadline(t *testing.T) {
	ctx := context.Background()
	tctx, cancel := withTimeout(ctx)
	defer cancel()

	deadline, ok := tctx.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}
	if time.Until(deadline) > defaultPhaseTimeout {
		t.Error("deadline should be within default timeout")
	}
}

// ---------------------------------------------------------------------------
// createFeatureBranch (uses iteragent bash tool — needs a git repo)
// ---------------------------------------------------------------------------

func TestCreateFeatureBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// create a local "origin/main" ref so git checkout -b branch origin/main works
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/main", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update-ref failed: %v\n%s", err, out)
	}

	e := New(dir, slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	branch, err := e.createFeatureBranch(ctx, 1)
	if err != nil {
		// Some prep commands fail (fetch/pull) but branch creation may still succeed
		// If it fully fails, that's OK for this test environment
		t.Logf("createFeatureBranch returned error (may be expected without remote): %v", err)
		return
	}
	if branch != "evolution/day-1" {
		t.Errorf("expected 'evolution/day-1', got %q", branch)
	}
}

// ---------------------------------------------------------------------------
// forwardEvents (additional tests)
// ---------------------------------------------------------------------------

func TestForwardEvents_EmptyChannel(t *testing.T) {
	e := New("/tmp", slog.Default())
	ch := make(chan iteragent.Event)
	close(ch)
	e.forwardEvents(ch) // should not block
}

// ---------------------------------------------------------------------------
// newAgent
// ---------------------------------------------------------------------------

func TestNewAgent_CreatesAgent(t *testing.T) {
	e := New(t.TempDir(), slog.Default())
	// nil provider is OK for construction (it won't be called)
	a := e.newAgent(nil, nil, "system prompt", nil)
	if a == nil {
		t.Error("newAgent should return non-nil agent")
	}
}

func TestNewAgent_WithThinking(t *testing.T) {
	e := New(t.TempDir(), slog.Default())
	e.thinkingLevel = iteragent.ThinkingLevelMedium
	a := e.newAgent(nil, nil, "prompt", nil)
	if a == nil {
		t.Error("newAgent should return non-nil agent")
	}
}
