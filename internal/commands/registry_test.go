package commands

import (
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	r.Register(Command{
		Name:        "/test",
		Aliases:     []string{"/t"},
		Description: "test command",
		Category:    "test",
		Handler:     func(ctx Context) Result { return Result{Handled: true} },
	})

	// Lookup by name
	cmd, ok := r.Lookup("/test")
	if !ok {
		t.Fatal("expected to find /test")
	}
	if cmd.Name != "/test" {
		t.Errorf("expected name /test, got %s", cmd.Name)
	}

	// Lookup by alias
	cmd, ok = r.Lookup("/t")
	if !ok {
		t.Fatal("expected to find alias /t")
	}
	if cmd.Name != "/test" {
		t.Errorf("expected alias to resolve to /test, got %s", cmd.Name)
	}
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry()
	executed := false
	r.Register(Command{
		Name:    "/test",
		Handler: func(ctx Context) Result { executed = true; return Result{Handled: true} },
	})

	result := r.Execute("/test", Context{})
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if !executed {
		t.Error("expected handler to be called")
	}
}

func TestRegistry_ExecuteUnknown(t *testing.T) {
	r := NewRegistry()
	result := r.Execute("/unknown", Context{})
	if result.Handled {
		t.Error("expected unknown command to not be handled")
	}
}

func TestRegisterAll(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	// Verify some key commands are registered
	required := []string{"/quit", "/clear", "/save", "/load", "/test", "/build", "/commit", "/swarm"}
	for _, name := range required {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}

	// Should have all commands registered
	all := r.All()
	if len(all) < 30 {
		t.Errorf("expected at least 30 commands, got %d", len(all))
	}
	t.Logf("Total commands registered: %d", len(all))
}

func TestByCategory(t *testing.T) {
	r := DefaultRegistry()
	cats := r.ByCategory()

	// Verify expected categories exist
	expectedCats := []string{"session", "safety", "dev", "git", "agent"}
	for _, cat := range expectedCats {
		if _, ok := cats[cat]; !ok {
			t.Errorf("expected category %s to exist", cat)
		}
	}
}

func TestQuitReturnsDone(t *testing.T) {
	r := DefaultRegistry()
	result := r.Execute("/quit", Context{})
	if !result.Done {
		t.Error("expected /quit to return Done=true")
	}
}
