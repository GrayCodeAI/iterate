package evolution

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// issueAlreadyCommented
// ---------------------------------------------------------------------------

// fakeToolRunner replaces e.runTool output for testing without real gh CLI.
// We test issueAlreadyCommented by directly calling it with a stubbed engine
// that overrides the bash tool via a temp script.

func TestIssueAlreadyCommented_AlreadyPosted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "DAY_COUNT"), []byte("5\n"), 0o644)

	// Create a fake `gh` that returns a comment body containing "Day 5"
	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	os.WriteFile(fakeGH, []byte("#!/bin/sh\necho 'Thanks for the fix! — Day 5'\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	result := e.issueAlreadyCommented(context.Background(), 42, "5")
	if !result {
		t.Error("expected issueAlreadyCommented to return true when 'Day 5' found in comments")
	}
}

func TestIssueAlreadyCommented_NotYetPosted(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	// Comments exist but from a different day
	os.WriteFile(fakeGH, []byte("#!/bin/sh\necho 'Thanks! — Day 3'\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	result := e.issueAlreadyCommented(context.Background(), 42, "5")
	if result {
		t.Error("expected issueAlreadyCommented to return false when day doesn't match")
	}
}

func TestIssueAlreadyCommented_NoComments(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	// gh returns empty (no comments)
	os.WriteFile(fakeGH, []byte("#!/bin/sh\necho ''\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	result := e.issueAlreadyCommented(context.Background(), 1, "5")
	if result {
		t.Error("expected issueAlreadyCommented to return false when no comments exist")
	}
}

func TestIssueAlreadyCommented_GHFails(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	// gh exits non-zero (e.g. no auth)
	os.WriteFile(fakeGH, []byte("#!/bin/sh\nexit 1\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	// Should fail safe — don't block posting if gh check fails
	result := e.issueAlreadyCommented(context.Background(), 1, "5")
	if result {
		t.Error("expected issueAlreadyCommented to return false (fail-safe) when gh errors")
	}
}

// ---------------------------------------------------------------------------
// postIssueComments deduplication — unit test via persistJournalEntry
// to confirm day-based sign-off matching works end-to-end
// ---------------------------------------------------------------------------

func TestIssueAlreadyCommented_DayZeroEdgeCase(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	// Comment contains "Day 0" — birth day
	os.WriteFile(fakeGH, []byte("#!/bin/sh\necho 'Born. — Day 0'\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	result := e.issueAlreadyCommented(context.Background(), 1, "0")
	if !result {
		t.Error("expected issueAlreadyCommented to return true for Day 0 match")
	}
}

func TestIssueAlreadyCommented_PartialDayMatch(t *testing.T) {
	dir := t.TempDir()

	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	fakeGH := filepath.Join(binDir, "gh")
	// "Day 1" should NOT match when current day is "10"
	os.WriteFile(fakeGH, []byte("#!/bin/sh\necho 'Done — Day 1'\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	e := New(dir, slog.Default())
	e.repo = "owner/repo"

	// Day 10 — "Day 1" is a substring of "Day 10" so this tests for false positives
	result := e.issueAlreadyCommented(context.Background(), 1, "10")
	if result {
		t.Error("'Day 1' should not match when current day is '10' — substring false positive")
	}
}
