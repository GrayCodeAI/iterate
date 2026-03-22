package main

import (
	"testing"

	"github.com/GrayCodeAI/iterate/internal/commands"
)

func TestREPLStartup(t *testing.T) {
	cfg := loadConfig()
	if cfg.Provider == "" && cfg.Model == "" {
		// No config file present is fine — just verify loadConfig doesn't panic.
		t.Log("no config file found (expected in CI)")
	}
}

func TestCommandRegistryIntegration(t *testing.T) {
	r := commands.DefaultRegistry()

	for _, name := range []string{"/help", "/quit", "/save", "/load", "/test", "/build", "/commit", "/status"} {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("command %s not found in registry", name)
		}
	}

	cats := r.ByCategory()
	if len(cats) == 0 {
		t.Error("no command categories found")
	}
}

func TestDefaultRegistryCount(t *testing.T) {
	r := commands.DefaultRegistry()
	all := r.All()
	if len(all) < 80 {
		t.Errorf("expected at least 80 commands, got %d", len(all))
	}
}
