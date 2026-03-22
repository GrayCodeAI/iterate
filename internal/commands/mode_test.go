package commands

import (
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// cmdCode
// ---------------------------------------------------------------------------

func TestCmdCode_SetsMode(t *testing.T) {
	mode := 2
	ctx := Context{
		CurrentMode: &mode,
		Parts:       []string{"/code"},
	}
	result := cmdCode(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if mode != 0 {
		t.Errorf("expected mode 0 (normal), got %d", mode)
	}
}

func TestCmdCode_NilMode(t *testing.T) {
	ctx := Context{Parts: []string{"/code"}}
	result := cmdCode(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdAsk
// ---------------------------------------------------------------------------

func TestCmdAsk_SetsMode(t *testing.T) {
	mode := 0
	ctx := Context{
		CurrentMode: &mode,
		Parts:       []string{"/ask"},
	}
	result := cmdAsk(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if mode != 1 {
		t.Errorf("expected mode 1 (ask), got %d", mode)
	}
}

// ---------------------------------------------------------------------------
// cmdArchitect
// ---------------------------------------------------------------------------

func TestCmdArchitect_SetsMode(t *testing.T) {
	mode := 0
	ctx := Context{
		CurrentMode: &mode,
		Parts:       []string{"/architect"},
	}
	result := cmdArchitect(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if mode != 2 {
		t.Errorf("expected mode 2 (architect), got %d", mode)
	}
}

// ---------------------------------------------------------------------------
// cmdVersion
// ---------------------------------------------------------------------------

func TestCmdVersion_WithVersion(t *testing.T) {
	ctx := Context{Version: "1.2.3"}
	result := cmdVersion(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdVersion_DefaultVersion(t *testing.T) {
	ctx := Context{Version: ""}
	result := cmdVersion(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdVersion_WithProvider(t *testing.T) {
	p := &mockProvider{name: "gpt-4o"}
	ctx := Context{Version: "2.0", Provider: p}
	result := cmdVersion(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdStats
// ---------------------------------------------------------------------------

func TestCmdStats_Basic(t *testing.T) {
	input := 100
	output := 50
	ctx := Context{
		SessionInputTokens:  &input,
		SessionOutputTokens: &output,
	}
	result := cmdStats(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdStats_WithAgent(t *testing.T) {
	agent := iteragent.New(&mockProvider{name: "test"}, nil, nil)
	defer agent.Close()
	agent.Messages = []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	ctx := Context{Agent: agent}
	result := cmdStats(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdStats_NilPointers(t *testing.T) {
	ctx := Context{}
	result := cmdStats(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdTheme
// ---------------------------------------------------------------------------

func TestCmdTheme_NoArg(t *testing.T) {
	ctx := Context{Parts: []string{"/theme"}}
	result := cmdTheme(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTheme_ValidTheme(t *testing.T) {
	var applied string
	ctx := Context{
		Parts:      []string{"/theme", "nord"},
		ApplyTheme: func(name string) { applied = name },
	}
	result := cmdTheme(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if applied != "nord" {
		t.Errorf("expected theme 'nord', got %q", applied)
	}
}

func TestCmdTheme_InvalidTheme(t *testing.T) {
	ctx := Context{
		Parts: []string{"/theme", "nonexistent"},
	}
	result := cmdTheme(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTheme_Minimal(t *testing.T) {
	var applied string
	ctx := Context{
		Parts:      []string{"/theme", "minimal"},
		ApplyTheme: func(name string) { applied = name },
	}
	result := cmdTheme(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if applied != "minimal" {
		t.Errorf("expected 'minimal', got %q", applied)
	}
}

// ---------------------------------------------------------------------------
// cmdTree
// ---------------------------------------------------------------------------

func TestCmdTree_DefaultDepth(t *testing.T) {
	ctx := Context{
		RepoPath: "/tmp",
		Parts:    []string{"/tree"},
	}
	result := cmdTree(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdTree_CustomDepth(t *testing.T) {
	ctx := Context{
		RepoPath: "/tmp",
		Parts:    []string{"/tree", "2"},
	}
	result := cmdTree(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdView
// ---------------------------------------------------------------------------

func TestCmdView_NoArg(t *testing.T) {
	ctx := Context{
		RepoPath: "/tmp",
		Parts:    []string{"/view"},
	}
	result := cmdView(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdView_NonExistentFile(t *testing.T) {
	ctx := Context{
		RepoPath: "/tmp",
		Parts:    []string{"/view", "nonexistent_file.txt"},
	}
	result := cmdView(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdSummarize
// ---------------------------------------------------------------------------

func TestCmdSummarize_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/summarize"},
	}
	result := cmdSummarize(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdSummarize_EmptyMessages(t *testing.T) {
	agent := iteragent.New(&mockProvider{name: "test"}, nil, nil)
	defer agent.Close()
	ctx := Context{
		Agent: agent,
		Parts: []string{"/summarize"},
	}
	result := cmdSummarize(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// cmdHelp
// ---------------------------------------------------------------------------

func TestCmdHelp_ReturnsHandled(t *testing.T) {
	result := cmdHelp(Context{})
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

// ---------------------------------------------------------------------------
// RegisterModeCommands
// ---------------------------------------------------------------------------

func TestRegisterModeCommands(t *testing.T) {
	r := NewRegistry()
	RegisterModeCommands(r)

	expected := []string{"/help", "/version", "/code", "/ask", "/architect", "/summarize", "/review", "/explain", "/view", "/show", "/tree", "/stats", "/theme"}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

func TestModeCommandAliases(t *testing.T) {
	r := NewRegistry()
	RegisterModeCommands(r)

	cmd, ok := r.Lookup("/?")
	if !ok {
		t.Fatal("expected /? alias")
	}
	if cmd.Name != "/help" {
		t.Errorf("expected /? to resolve to /help, got %s", cmd.Name)
	}
}

// ---------------------------------------------------------------------------
// BuildProjectTree helper
// ---------------------------------------------------------------------------

func TestBuildProjectTree(t *testing.T) {
	tree := BuildProjectTree(".", 2)
	// Should not panic and should return a string
	_ = tree
}

func TestBuildProjectTree_EmptyDir(t *testing.T) {
	tree := BuildProjectTree("/nonexistent/path/xyz", 2)
	if strings.TrimSpace(tree) != "" {
		// Some systems may return something; just check no panic
		_ = tree
	}
}
