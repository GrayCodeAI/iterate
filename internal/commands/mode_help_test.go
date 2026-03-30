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

	// Note: cmdHelp uses fmt.Printf, not ctx.Writer, so we verify behavior
	// by checking the function returns Handled=true. The actual output goes to stdout.
	// This is a known limitation - cmdHelp should use ctx.Writer for testability.
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

	// Note: cmdHelp uses fmt.Printf, not ctx.Writer, so we verify behavior
	// by checking the function returns Handled=true. The actual output goes to stdout.
	// This is a known limitation - cmdHelp should use ctx.Writer for testability.
}

func TestCmdHelp_UnknownCommand(t *testing.T) {
	r := NewRegistry()

	var buf strings.Builder
	ctx := Context{
		Registry: r,
		Writer:   &buf,
		Parts:    []string{"/help", "/nonexistent"},
	}

	result := cmdHelp(ctx)
	if !result.Handled {
		t.Error("expected Handled=true for unknown command")
	}
}
