package commands

import (
	"strings"
	"testing"
)

func TestCmdHelp_WithRegistry_ShowsCategory(t *testing.T) {
	r := NewRegistry()
	RegisterModeCommands(r)

	var buf strings.Builder
	ctx := Context{
		Registry: r,
		Writer:   &buf,
		Parts:    []string{"/help", "/code"},
	}

	result := cmdHelp(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}

	output := buf.String()
	// The output should contain the category
	if !strings.Contains(output, "mode") {
		t.Errorf("expected help output to contain category 'mode', got:\n%s", output)
	}
}

func TestCmdHelp_WithoutArgs_ShowsAllCategories(t *testing.T) {
	r := NewRegistry()
	RegisterModeCommands(r)

	var buf strings.Builder
	ctx := Context{
		Registry: r,
		Writer:   &buf,
		Parts:    []string{"/help"},
	}

	result := cmdHelp(ctx)
	if !result.Handled {
		t.Error("expected Handled=true")
	}

	output := buf.String()
	// Should show at least one category header
	if !strings.Contains(output, "MODE") {
		t.Errorf("expected help output to contain category header, got:\n%s", output)
	}
}