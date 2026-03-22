package commands

import (
	"testing"
)

// ---------------------------------------------------------------------------
// cmdDiff
// ---------------------------------------------------------------------------

func TestCmdDiff_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/diff"},
	}
	result := cmdDiff(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdStatus
// ---------------------------------------------------------------------------

func TestCmdStatus_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/status"},
	}
	result := cmdStatus(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdLog
// ---------------------------------------------------------------------------

func TestCmdLog_DefaultCount(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/log"},
	}
	result := cmdLog(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdLog_CustomCount(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/log", "5"},
	}
	result := cmdLog(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdBranch
// ---------------------------------------------------------------------------

func TestCmdBranch_ListBranches(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/branch"},
	}
	result := cmdBranch(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdCommit
// ---------------------------------------------------------------------------

func TestCmdCommit_NoMessage(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/commit"},
	}
	result := cmdCommit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdCommit_WithMessage(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/commit", "test", "message"},
	}
	result := cmdCommit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdPush
// ---------------------------------------------------------------------------

func TestCmdPush_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/push"},
	}
	result := cmdPush(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdPull
// ---------------------------------------------------------------------------

func TestCmdPull_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/pull"},
	}
	result := cmdPull(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdRevertFile
// ---------------------------------------------------------------------------

func TestCmdRevertFile_NoArg(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/revert-file"},
	}
	result := cmdRevertFile(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdBlame
// ---------------------------------------------------------------------------

func TestCmdBlame_NoArg(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/blame"},
	}
	result := cmdBlame(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdCherryPick
// ---------------------------------------------------------------------------

func TestCmdCherryPick_NoArg(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/cherry-pick"},
	}
	result := cmdCherryPick(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdAutoCommit
// ---------------------------------------------------------------------------

func TestCmdAutoCommit_Toggle(t *testing.T) {
	enabled := false
	ctx := Context{
		AutoCommitEnabled: &enabled,
		Parts:             []string{"/auto-commit"},
	}
	result := cmdAutoCommit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if !enabled {
		t.Error("expected auto-commit to be enabled after toggle")
	}
}

func TestCmdAutoCommit_Disable(t *testing.T) {
	enabled := true
	ctx := Context{
		AutoCommitEnabled: &enabled,
		Parts:             []string{"/auto-commit"},
	}
	result := cmdAutoCommit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if enabled {
		t.Error("expected auto-commit to be disabled after toggle")
	}
}

func TestCmdAutoCommit_NilPointer(t *testing.T) {
	ctx := Context{
		Parts: []string{"/auto-commit"},
	}
	result := cmdAutoCommit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdGit
// ---------------------------------------------------------------------------

func TestCmdGit_NoArgs(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/git"},
	}
	result := cmdGit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdGit_WithArgs(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/git", "log", "--oneline", "-1"},
	}
	result := cmdGit(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdTag
// ---------------------------------------------------------------------------

func TestCmdTag_ListTags(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/tag"},
	}
	result := cmdTag(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdFetch
// ---------------------------------------------------------------------------

func TestCmdFetch_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/fetch"},
	}
	result := cmdFetch(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdRebase
// ---------------------------------------------------------------------------

func TestCmdRebase_DefaultTarget(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/rebase"},
	}
	result := cmdRebase(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdRebase_CustomTarget(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/rebase", "develop"},
	}
	result := cmdRebase(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdStash
// ---------------------------------------------------------------------------

func TestCmdStash_Stash(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/stash"},
	}
	result := cmdStash(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdStash_Pop(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/stash", "pop"},
	}
	result := cmdStash(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdAmend
// ---------------------------------------------------------------------------

func TestCmdAmend_NoEdit(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/amend"},
	}
	result := cmdAmend(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdAmend_WithMessage(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/amend", "fix", "typo"},
		Line:     "/amend fix typo",
	}
	result := cmdAmend(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdDiffStaged
// ---------------------------------------------------------------------------

func TestCmdDiffStaged_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/diff-staged"},
	}
	result := cmdDiffStaged(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdStashList
// ---------------------------------------------------------------------------

func TestCmdStashList_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/stash-list"},
	}
	result := cmdStashList(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdClean
// ---------------------------------------------------------------------------

func TestCmdClean_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/clean"},
	}
	result := cmdClean(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdUndo
// ---------------------------------------------------------------------------

func TestCmdUndo_ReturnsHandled(t *testing.T) {
	ctx := Context{
		RepoPath: ".",
		Parts:    []string{"/undo"},
	}
	result := cmdUndo(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// RegisterGitCommands
// ---------------------------------------------------------------------------

func TestRegisterGitCommands(t *testing.T) {
	r := NewRegistry()
	RegisterGitCommands(r)

	expected := []string{
		"/diff", "/status", "/log", "/branch", "/commit",
		"/push", "/pull", "/revert-file", "/blame",
		"/cherry-pick", "/checkout", "/merge", "/stash",
		"/tag", "/fetch", "/rebase", "/amend", "/squash",
		"/diff-staged", "/stash-list", "/clean", "/undo",
		"/auto-commit", "/git", "/generate-commit",
	}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

func TestGitCommandAliases(t *testing.T) {
	r := NewRegistry()
	RegisterGitCommands(r)

	cmd, ok := r.Lookup("/st")
	if !ok {
		t.Fatal("expected /st alias")
	}
	if cmd.Name != "/status" {
		t.Errorf("expected /st to resolve to /status, got %s", cmd.Name)
	}

	cmd, ok = r.Lookup("/br")
	if !ok {
		t.Fatal("expected /br alias")
	}
	if cmd.Name != "/branch" {
		t.Errorf("expected /br to resolve to /branch, got %s", cmd.Name)
	}
}
