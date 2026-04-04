package commands

import (
	"testing"
)

func TestRegistry_Registration(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestCommand_HasName(t *testing.T) {
	cmd := Command{Name: "/test", Description: "test command", Category: "test"}
	if cmd.Name != "/test" {
		t.Errorf("expected /test, got %q", cmd.Name)
	}
}

func TestResult_HandledState(t *testing.T) {
	r1 := Result{Handled: true}
	if !r1.Handled {
		t.Error("expected Handled to be true")
	}

	r2 := Result{Handled: false}
	if r2.Handled {
		t.Error("expected Handled to be false")
	}
}

func TestNewRegistry_DefaultState(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("registry should not be nil")
	}
}

func TestCmdContext_HasArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/test", "arg1"},
	}

	if !ctx.HasArg(1) {
		t.Error("expected arg at index 1 to exist")
	}
	if ctx.HasArg(2) {
		t.Error("expected arg at index 2 to not exist")
	}
}

func TestCmdContext_Arg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/test", "first", "second"},
	}

	if got := ctx.Arg(1); got != "first" {
		t.Errorf("expected first arg, got %q", got)
	}
	if got := ctx.Arg(2); got != "second" {
		t.Errorf("expected second arg, got %q", got)
	}
}

func TestCmdContext_Args(t *testing.T) {
	ctx := Context{
		Parts: []string{"/test", "arg1", "arg2"},
	}

	got := ctx.Args()
	expected := "arg1 arg2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCmdContext_Command(t *testing.T) {
	ctx := Context{
		Parts: []string{"/help", "section"},
		Line:  "/help section",
	}

	if ctx.Parts[0] != "/help" {
		t.Errorf("expected /help, got %q", ctx.Parts[0])
	}
}
