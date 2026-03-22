package commands

import (
	"testing"
)

func TestVersionCommand_Handled(t *testing.T) {
	r := DefaultRegistry()
	cmd, ok := r.Lookup("/version")
	if !ok {
		t.Fatal("expected /version to be registered")
	}

	ctx := Context{Version: "1.2.3"}
	result := cmd.Handler(ctx)
	if !result.Handled {
		t.Error("expected /version to return Handled=true")
	}
}

func TestVersionCommand_DefaultVersion(t *testing.T) {
	r := DefaultRegistry()
	cmd, ok := r.Lookup("/version")
	if !ok {
		t.Fatal("expected /version to be registered")
	}

	ctx := Context{} // empty version
	result := cmd.Handler(ctx)
	if !result.Handled {
		t.Error("expected /version with empty version to return Handled=true")
	}
}

func TestHelpCommand_Handled(t *testing.T) {
	r := DefaultRegistry()
	cmd, ok := r.Lookup("/help")
	if !ok {
		t.Fatal("expected /help to be registered")
	}

	result := cmd.Handler(Context{})
	if !result.Handled {
		t.Error("expected /help to return Handled=true")
	}
}

func TestHelpCommand_Alias(t *testing.T) {
	r := DefaultRegistry()
	cmd, ok := r.Lookup("/?")
	if !ok {
		t.Fatal("expected /? alias to be registered")
	}
	if cmd.Name != "/help" {
		t.Errorf("expected /? to resolve to /help, got %s", cmd.Name)
	}
}

func TestQuitCommand_ReturnsDone(t *testing.T) {
	r := DefaultRegistry()
	result := r.Execute("/quit", Context{})
	if !result.Done {
		t.Error("expected /quit to return Done=true")
	}
}

func TestClearCommand_Handled(t *testing.T) {
	r := DefaultRegistry()
	result := r.Execute("/clear", Context{})
	if !result.Handled {
		t.Error("expected /clear to return Handled=true")
	}
}

func TestCommandRegistry_HasExpectedCommands(t *testing.T) {
	r := DefaultRegistry()
	expected := []string{
		"/help", "/version", "/quit", "/clear",
		"/test", "/build", "/lint",
		"/save", "/load",
		"/diff", "/status", "/commit",
	}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

func TestCommandRegistry_CategoryGrouping(t *testing.T) {
	r := DefaultRegistry()
	cats := r.ByCategory()

	// mode category should contain /help and /version
	modeCmds := cats["mode"]
	found := map[string]bool{}
	for _, cmd := range modeCmds {
		found[cmd.Name] = true
	}
	if !found["/help"] {
		t.Error("expected /help in mode category")
	}
	if !found["/version"] {
		t.Error("expected /version in mode category")
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	r := DefaultRegistry()
	result := r.Execute("/nonexistent_command_xyz", Context{})
	if result.Handled {
		t.Error("expected unknown command to return Handled=false")
	}
}
