package commands

import (
	"context"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ── /run ──────────────────────────────────────────────────────────────────────

func TestCmdRun_NoArgs(t *testing.T) {
	ctx := Context{Parts: []string{"/run"}}
	result := cmdRun(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdRun_CapturesOutput(t *testing.T) {
	lastRunMu.Lock()
	lastRunOutput = ""
	lastRunMu.Unlock()

	ctx := Context{
		Parts:    []string{"/run", "echo", "hello"},
		RepoPath: t.TempDir(),
	}
	result := cmdRun(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	out := LastRunOutput()
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in captured output, got %q", out)
	}
}

func TestCmdRun_AskFlag_NoArg(t *testing.T) {
	ctx := Context{Parts: []string{"/run", "--ask"}}
	result := cmdRun(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdRun_AskFlag_SendsPrompt(t *testing.T) {
	var gotPrompt string
	ctx := Context{
		Parts:    []string{"/run", "--ask", "echo", "world"},
		RepoPath: t.TempDir(),
		Agent:    &iteragent.Agent{},
		REPL: REPLCallbacks{
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, prompt, _ string) {
				gotPrompt = prompt
			},
		},
	}
	lastRunMu.Lock()
	lastRunOutput = ""
	lastRunMu.Unlock()

	result := cmdRun(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if !strings.Contains(gotPrompt, "echo world") {
		t.Errorf("expected command in prompt, got %q", gotPrompt)
	}
	if !strings.Contains(gotPrompt, "world") {
		t.Errorf("expected output in prompt, got %q", gotPrompt)
	}
}

// ── /explain-error ─────────────────────────────────────────────────────────────

func TestCmdExplainError_NoArg_NoLastRun(t *testing.T) {
	lastRunMu.Lock()
	lastRunOutput = ""
	lastRunMu.Unlock()

	ctx := Context{
		Parts: []string{"/explain-error"},
		Line:  "/explain-error",
	}
	result := cmdExplainError(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
}

func TestCmdExplainError_NoArg_UsesLastRun(t *testing.T) {
	lastRunMu.Lock()
	lastRunOutput = "error: undefined: foo"
	lastRunMu.Unlock()
	defer func() {
		lastRunMu.Lock()
		lastRunOutput = ""
		lastRunMu.Unlock()
	}()

	var gotPrompt string
	ctx := Context{
		Parts: []string{"/explain-error"},
		Line:  "/explain-error",
		Agent: &iteragent.Agent{},
		REPL: REPLCallbacks{
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, prompt, _ string) {
				gotPrompt = prompt
			},
		},
	}
	result := cmdExplainError(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if !strings.Contains(gotPrompt, "undefined: foo") {
		t.Errorf("expected last run output in prompt, got %q", gotPrompt)
	}
}

func TestCmdExplainError_WithArg(t *testing.T) {
	var gotPrompt string
	ctx := Context{
		Parts: []string{"/explain-error", "segfault"},
		Line:  "/explain-error segfault",
		Agent: &iteragent.Agent{},
		REPL: REPLCallbacks{
			StreamAndPrint: func(_ context.Context, _ *iteragent.Agent, prompt, _ string) {
				gotPrompt = prompt
			},
		},
	}
	result := cmdExplainError(ctx)
	if !result.Handled {
		t.Error("expected handled")
	}
	if !strings.Contains(gotPrompt, "segfault") {
		t.Errorf("expected error text in prompt, got %q", gotPrompt)
	}
}
