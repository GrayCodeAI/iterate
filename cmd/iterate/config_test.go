package main

import (
	"strings"
	"testing"
)

func TestConfigPath(t *testing.T) {
	path := configPath()
	if !strings.Contains(path, ".iterate") {
		t.Errorf("config path should contain .iterate, got %s", path)
	}
	if !strings.HasSuffix(path, "config.json") {
		t.Errorf("config path should end with config.json, got %s", path)
	}
}

func TestSaveLoadConfig(t *testing.T) {
	cfg := iterConfig{
		Provider: "anthropic",
		Model:    "claude-opus-4-6",
	}

	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", cfg.Provider)
	}
}
