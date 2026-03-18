package main

import (
	"strings"
	"testing"
)

func TestTabCompleteExact(t *testing.T) {
	result := tabComplete("/help")
	if result != "/help " {
		t.Errorf("exact match should return command with space, got %q", result)
	}
}

func TestTabCompletePartial(t *testing.T) {
	result := tabComplete("/hel")
	if !strings.HasPrefix(result, "/hel") {
		t.Errorf("partial match should start with input, got %q", result)
	}
}

func TestTabCompleteMultiMatch(t *testing.T) {
	result := tabComplete("/test")
	// Should return common prefix of /test, /test-file, /test-gen
	if !strings.HasPrefix(result, "/test") {
		t.Errorf("multi-match should return common prefix, got %q", result)
	}
}

func TestTabCompleteNoMatch(t *testing.T) {
	result := tabComplete("/nonexistent")
	if result != "/nonexistent" {
		t.Errorf("no match should return input unchanged, got %q", result)
	}
}

func TestSlashCommandsList(t *testing.T) {
	if len(slashCommands) < 100 {
		t.Errorf("expected 100+ slash commands, got %d", len(slashCommands))
	}

	// Check for some key commands
	hasHelp := false
	hasDiff := false
	hasTest := false
	for _, cmd := range slashCommands {
		if cmd == "/help" {
			hasHelp = true
		}
		if cmd == "/diff" {
			hasDiff = true
		}
		if cmd == "/test" {
			hasTest = true
		}
	}

	if !hasHelp || !hasDiff || !hasTest {
		t.Errorf("slash commands missing expected commands")
	}
}
