package commands

import (
	"context"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// mock provider
// ---------------------------------------------------------------------------

type mockProvider struct {
	name string
}

func (m *mockProvider) Complete(_ context.Context, msgs []iteragent.Message, _ ...iteragent.CompletionOptions) (string, error) {
	return "mock response", nil
}

func (m *mockProvider) Name() string { return m.name }

// ---------------------------------------------------------------------------
// cmdModel
// ---------------------------------------------------------------------------

func TestCmdModel_WithProvider(t *testing.T) {
	p := &mockProvider{name: "claude-sonnet-4"}
	ctx := Context{Provider: p}
	result := cmdModel(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdModel_NilProvider(t *testing.T) {
	ctx := Context{Provider: nil}
	result := cmdModel(ctx)
	if !result.Handled {
		t.Error("expected Handled=true even with nil provider")
	}
}

// ---------------------------------------------------------------------------
// cmdCost
// ---------------------------------------------------------------------------

func TestCmdCost_Basic(t *testing.T) {
	input := 1000
	output := 500
	ctx := Context{
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
	}
	result := cmdCost(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdCost_WithCache(t *testing.T) {
	input := 1000
	output := 500
	cacheRead := 200
	cacheWrite := 100
	ctx := Context{
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
		SessionCacheRead:    &cacheRead,
		SessionCacheWrite:   &cacheWrite,
	}
	result := cmdCost(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdCost_NilTokenPointers(t *testing.T) {
	ctx := Context{}
	result := cmdCost(ctx)
	if !result.Handled {
		t.Error("expected Handled=true with nil token pointers")
	}
}

func TestCmdCost_WithProvider(t *testing.T) {
	p := &mockProvider{name: "gpt-4o"}
	input := 100
	output := 50
	ctx := Context{
		Provider:            p,
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
	}
	result := cmdCost(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdTokens
// ---------------------------------------------------------------------------

func TestCmdTokens_Basic(t *testing.T) {
	input := 2000
	output := 1000
	ctx := Context{
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
	}
	result := cmdTokens(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTokens_WithCache(t *testing.T) {
	input := 2000
	output := 1000
	cacheRead := 500
	cacheWrite := 250
	ctx := Context{
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
		SessionCacheRead:    &cacheRead,
		SessionCacheWrite:   &cacheWrite,
	}
	result := cmdTokens(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTokens_NilPointers(t *testing.T) {
	ctx := Context{}
	result := cmdTokens(ctx)
	if !result.Handled {
		t.Error("expected Handled=true with nil pointers")
	}
}

// ---------------------------------------------------------------------------
// cmdTools
// ---------------------------------------------------------------------------

func TestCmdTools_NilAgent(t *testing.T) {
	ctx := Context{Agent: nil}
	result := cmdTools(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTools_WithAgent(t *testing.T) {
	agent := iteragent.New(&mockProvider{name: "test"}, []iteragent.Tool{
		{Name: "read_file", Description: "read a file"},
		{Name: "write_file", Description: "write a file"},
	}, nil)
	defer agent.Close()
	ctx := Context{Agent: agent}
	result := cmdTools(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTools_EmptyTools(t *testing.T) {
	agent := iteragent.New(&mockProvider{name: "test"}, nil, nil)
	defer agent.Close()
	ctx := Context{Agent: agent}
	result := cmdTools(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdThinking
// ---------------------------------------------------------------------------

func TestCmdThinking_NoArg(t *testing.T) {
	level := iteragent.ThinkingLevel("medium")
	ctx := Context{
		Thinking: &level,
		Parts:    []string{"/thinking"},
	}
	result := cmdThinking(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdThinking_WithArg(t *testing.T) {
	level := iteragent.ThinkingLevel("low")
	ctx := Context{
		Thinking: &level,
		Parts:    []string{"/thinking", "high"},
	}
	result := cmdThinking(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if *ctx.Thinking != "high" {
		t.Errorf("expected thinking level 'high', got %q", *ctx.Thinking)
	}
}

func TestCmdThinking_OffLevel(t *testing.T) {
	level := iteragent.ThinkingLevel("medium")
	ctx := Context{
		Thinking: &level,
		Parts:    []string{"/thinking", "off"},
	}
	result := cmdThinking(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if *ctx.Thinking != "off" {
		t.Errorf("expected 'off', got %q", *ctx.Thinking)
	}
}

// ---------------------------------------------------------------------------
// cmdSwarm
// ---------------------------------------------------------------------------

func TestCmdSwarm_Usage(t *testing.T) {
	ctx := Context{Parts: []string{"/swarm"}}
	result := cmdSwarm(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdSwarm_InvalidNumber(t *testing.T) {
	ctx := Context{Parts: []string{"/swarm", "abc", "task"}}
	result := cmdSwarm(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdSwarm_LimitsTo100(t *testing.T) {
	ctx := Context{Parts: []string{"/swarm", "200", "do something"}}
	result := cmdSwarm(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdSwarm_NoPool(t *testing.T) {
	ctx := Context{Parts: []string{"/swarm", "1", "task"}}
	result := cmdSwarm(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdSpawn
// ---------------------------------------------------------------------------

func TestCmdSpawn_NoArg(t *testing.T) {
	ctx := Context{Parts: []string{"/spawn"}}
	result := cmdSpawn(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdSpawn_NoAgent(t *testing.T) {
	ctx := Context{
		Parts: []string{"/spawn", "do", "task"},
	}
	result := cmdSpawn(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// RegisterAgentCommands
// ---------------------------------------------------------------------------

func TestRegisterAgentCommands(t *testing.T) {
	r := NewRegistry()
	RegisterAgentCommands(r)

	expected := []string{"/model", "/thinking", "/tools", "/skills", "/cost", "/tokens", "/spawn", "/swarm"}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

// ---------------------------------------------------------------------------
// cmdSkills
// ---------------------------------------------------------------------------

func TestCmdSkills_NoDir(t *testing.T) {
	ctx := Context{RepoPath: "/nonexistent/path"}
	result := cmdSkills(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

func TestContext_HasArg(t *testing.T) {
	ctx := Context{Parts: []string{"/cmd", "a1", "a2"}}
	if !ctx.HasArg(1) {
		t.Error("expected HasArg(1)=true")
	}
	if !ctx.HasArg(2) {
		t.Error("expected HasArg(2)=true")
	}
	if ctx.HasArg(3) {
		t.Error("expected HasArg(3)=false")
	}
}

func TestContext_Arg(t *testing.T) {
	ctx := Context{Parts: []string{"/cmd", "first", "second"}}
	if ctx.Arg(1) != "first" {
		t.Errorf("expected Arg(1)='first', got %q", ctx.Arg(1))
	}
	if ctx.Arg(2) != "second" {
		t.Errorf("expected Arg(2)='second', got %q", ctx.Arg(2))
	}
	if ctx.Arg(5) != "" {
		t.Errorf("expected Arg(5)='', got %q", ctx.Arg(5))
	}
}

func TestContext_Args(t *testing.T) {
	ctx := Context{Parts: []string{"/cmd", "arg1", "arg2", "arg3"}}
	got := ctx.Args()
	if got != "arg1 arg2 arg3" {
		t.Errorf("expected 'arg1 arg2 arg3', got %q", got)
	}
}

func TestContext_Args_Empty(t *testing.T) {
	ctx := Context{Parts: []string{"/cmd"}}
	got := ctx.Args()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestContext_Write(t *testing.T) {
	var sb strings.Builder
	ctx := Context{Writer: &sb}
	ctx.Write("hello %s", "world")
	if sb.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", sb.String())
	}
}

func TestContext_WriteLn(t *testing.T) {
	var sb strings.Builder
	ctx := Context{Writer: &sb}
	ctx.WriteLn("line %d", 1)
	if sb.String() != "line 1\n" {
		t.Errorf("expected 'line 1\\n', got %q", sb.String())
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{Name: "/a", Handler: func(Context) Result { return Result{} }})
	r.Register(Command{Name: "/b", Handler: func(Context) Result { return Result{} }})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 commands, got %d", len(all))
	}
}

func TestRegistry_DuplicateName(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{Name: "/a", Handler: func(Context) Result { return Result{Handled: true} }})
	r.Register(Command{Name: "/a", Handler: func(Context) Result { return Result{Handled: false} }})
	cmd, ok := r.Lookup("/a")
	if !ok {
		t.Fatal("expected /a to be registered")
	}
	result := cmd.Handler(Context{})
	if result.Handled {
		t.Error("expected last registration to overwrite (Handled=false)")
	}
}
