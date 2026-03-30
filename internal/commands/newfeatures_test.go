package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ── /autofix ─────────────────────────────────────────────────────────────────

func TestDetectTestCmd_GoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, args := detectTestCmd(dir)
	if cmd != "go" {
		t.Errorf("expected go, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "test" {
		t.Errorf("expected [test ./...], got %v", args)
	}
}

func TestCmdAutofix_NoAgent(t *testing.T) {
	ctx := Context{
		Parts: []string{"/autofix"},
		REPL:  REPLCallbacks{StreamAndPrint: nil},
	}
	result := cmdAutofix(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestTailLines_ShortInput(t *testing.T) {
	input := "line1\nline2\nline3"
	out := tailLines(input, 10)
	if out != input {
		t.Errorf("expected passthrough for short input, got %q", out)
	}
}

func TestTailLines_TruncatesLong(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "line")
	}
	input := strings.Join(lines, "\n")
	out := tailLines(input, 10)
	if !strings.Contains(out, "omitted") {
		t.Error("expected omission notice in truncated output")
	}
}

// ── /profile ──────────────────────────────────────────────────────────────────

func TestProfileSaveLoad(t *testing.T) {
	// Point profiles to a temp dir.
	home := t.TempDir()
	t.Setenv("HOME", home)

	profiles := loadProfiles()
	if len(profiles) != 0 {
		t.Fatalf("expected empty profiles, got %d", len(profiles))
	}

	p := Profile{Name: "test", Model: "claude-3", Temperature: 0.7, MaxTokens: 4096}
	profiles["test"] = p
	if err := saveProfiles(profiles); err != nil {
		t.Fatalf("saveProfiles: %v", err)
	}

	loaded := loadProfiles()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(loaded))
	}
	lp := loaded["test"]
	if lp.Model != "claude-3" {
		t.Errorf("expected model claude-3, got %s", lp.Model)
	}
	if lp.Temperature != 0.7 {
		t.Errorf("expected temp 0.7, got %f", lp.Temperature)
	}
	if lp.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", lp.MaxTokens)
	}
}

func TestCmdProfile_NoArgs_Handled(t *testing.T) {
	ctx := Context{
		Parts:         []string{"/profile"},
		RuntimeConfig: &RuntimeConfig{},
	}
	result := cmdProfile(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdProfileLoad_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	ctx := Context{
		Parts:         []string{"/profile", "load", "nonexistent"},
		RuntimeConfig: &RuntimeConfig{},
	}
	result := cmdProfileLoad(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdProfileDelete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	profiles := map[string]Profile{"myprof": {Name: "myprof", Model: "gpt-4"}}
	if err := saveProfiles(profiles); err != nil {
		t.Fatal(err)
	}

	ctx := Context{Parts: []string{"/profile", "delete", "myprof"}}
	result := cmdProfileDelete(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}

	remaining := loadProfiles()
	if _, ok := remaining["myprof"]; ok {
		t.Error("expected profile to be deleted")
	}
}

// ── /chain ────────────────────────────────────────────────────────────────────

func TestCmdChain_NoAgent(t *testing.T) {
	ctx := Context{
		Parts: []string{"/chain", "do", "something"},
		REPL:  REPLCallbacks{},
	}
	result := cmdChain(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdChain_NoArgs(t *testing.T) {
	called := 0
	ctx := Context{
		Parts: []string{"/chain"},
		Agent: &iteragent.Agent{},
		REPL: REPLCallbacks{
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, _ string, _ string) {
				called++
			},
		},
	}
	result := cmdChain(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if called != 0 {
		t.Errorf("expected no calls with empty args, got %d", called)
	}
}

func TestCmdChain_TwoSteps(t *testing.T) {
	var prompts []string
	ctx := Context{
		Parts:    []string{"/chain", "step one ;; step two"},
		Line:     "/chain step one ;; step two",
		Agent:    &iteragent.Agent{},
		RepoPath: t.TempDir(),
		REPL: REPLCallbacks{
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, prompt string, _ string) {
				prompts = append(prompts, prompt)
			},
		},
	}
	result := cmdChain(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if len(prompts) != 2 {
		t.Errorf("expected 2 prompts, got %d: %v", len(prompts), prompts)
	}
	if prompts[0] != "step one" {
		t.Errorf("expected 'step one', got %q", prompts[0])
	}
	if prompts[1] != "step two" {
		t.Errorf("expected 'step two', got %q", prompts[1])
	}
}

// ── /search-sessions ──────────────────────────────────────────────────────────

func TestCmdSearchSessions_NoSessions(t *testing.T) {
	ctx := Context{
		Parts: []string{"/search-sessions", "auth"},
		Session: SessionCallbacks{
			ListSessions: func() []string { return nil },
			LoadSession:  func(name string) ([]iteragent.Message, error) { return nil, nil },
		},
	}
	result := cmdSearchSessions(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdSearchSessions_NoQuery(t *testing.T) {
	ctx := Context{
		Parts: []string{"/search-sessions"},
		Session: SessionCallbacks{
			ListSessions: func() []string { return []string{"s1"} },
		},
	}
	result := cmdSearchSessions(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdSearchSessions_FindsMatch(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "how do I fix the auth bug?"},
		{Role: "assistant", Content: "check the middleware"},
	}
	ctx := Context{
		Parts: []string{"/search-sessions", "auth"},
		Session: SessionCallbacks{
			ListSessions: func() []string { return []string{"work"} },
			LoadSession:  func(name string) ([]iteragent.Message, error) { return msgs, nil },
		},
	}
	result := cmdSearchSessions(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdSearchSessions_NoMatch(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "refactor the database layer"},
	}
	ctx := Context{
		Parts: []string{"/search-sessions", "zxcqwerty"},
		Session: SessionCallbacks{
			ListSessions: func() []string { return []string{"work"} },
			LoadSession:  func(name string) ([]iteragent.Message, error) { return msgs, nil },
		},
	}
	result := cmdSearchSessions(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

// ── /trim ─────────────────────────────────────────────────────────────────────

func TestCmdTrim_NoAgent(t *testing.T) {
	ctx := Context{Parts: []string{"/trim", "3"}}
	result := cmdTrim(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdTrim_EmptyContext(t *testing.T) {
	ag := &iteragent.Agent{}
	ctx := Context{
		Parts: []string{"/trim", "3"},
		Agent: ag,
	}
	result := cmdTrim(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdTrim_KeepsLastTurns(t *testing.T) {
	ag := &iteragent.Agent{}
	// 8 messages = 4 turns
	for i := 0; i < 4; i++ {
		ag.Messages = append(ag.Messages,
			iteragent.Message{Role: "user", Content: "q"},
			iteragent.Message{Role: "assistant", Content: "a"},
		)
	}
	ctx := Context{
		Parts: []string{"/trim", "2"},
		Agent: ag,
	}
	result := cmdTrim(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if len(ag.Messages) > 4 {
		t.Errorf("expected ≤4 messages after trim(2 turns), got %d", len(ag.Messages))
	}
}

// TestCmdTrim_NoUserMessageInTail covers the out-of-bounds case where every
// message in the kept tail has a non-user role (pathological but must not panic).
func TestCmdTrim_NoUserMessageInTail(t *testing.T) {
	ag := &iteragent.Agent{}
	// 10 assistant-only messages — no user messages anywhere.
	for i := 0; i < 10; i++ {
		ag.Messages = append(ag.Messages, iteragent.Message{Role: "assistant", Content: "a"})
	}
	ctx := Context{
		Parts: []string{"/trim", "2"},
		Agent: ag,
	}
	// Must not panic.
	result := cmdTrim(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdTrim_AlreadySmall(t *testing.T) {
	ag := &iteragent.Agent{}
	ag.Messages = []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	ctx := Context{
		Parts: []string{"/trim", "10"},
		Agent: ag,
	}
	result := cmdTrim(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	// Messages should be unchanged.
	if len(ag.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(ag.Messages))
	}
}

// ── /multi nil-context fix ────────────────────────────────────────────────────

func TestCmdMulti_NoReadMultiLine(t *testing.T) {
	ctx := Context{
		Parts: []string{"/multi"},
		Agent: &iteragent.Agent{},
		REPL:  REPLCallbacks{},
	}
	result := cmdMulti(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdMulti_CancelledInput(t *testing.T) {
	ctx := Context{
		Parts: []string{"/multi"},
		Agent: &iteragent.Agent{},
		REPL: REPLCallbacks{
			ReadMultiLine: func() (string, bool) { return "", false },
		},
	}
	result := cmdMulti(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdMulti_SendsPrompt(t *testing.T) {
	var got string
	ctx := Context{
		Parts: []string{"/multi"},
		Agent: &iteragent.Agent{},
		REPL: REPLCallbacks{
			ReadMultiLine: func() (string, bool) { return "line one\nline two", true },
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, prompt, _ string) {
				got = prompt
			},
		},
	}
	result := cmdMulti(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if got != "line one\nline two" {
		t.Errorf("expected prompt forwarded, got %q", got)
	}
}
