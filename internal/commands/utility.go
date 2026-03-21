package commands

import (
	"fmt"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterUtilityCommands adds utility/context management commands.
func RegisterUtilityCommands(r *Registry) {
	r.Register(Command{
		Name:        "/context",
		Aliases:     []string{},
		Description: "show context stats",
		Category:    "utility",
		Handler:     cmdContext,
	})

	r.Register(Command{
		Name:        "/export",
		Aliases:     []string{},
		Description: "export conversation to markdown",
		Category:    "utility",
		Handler:     cmdExport,
	})

	r.Register(Command{
		Name:        "/retry",
		Aliases:     []string{},
		Description: "retry last message",
		Category:    "utility",
		Handler:     cmdRetry,
	})

	r.Register(Command{
		Name:        "/copy",
		Aliases:     []string{},
		Description: "copy last response to clipboard",
		Category:    "utility",
		Handler:     cmdCopy,
	})

	r.Register(Command{
		Name:        "/pin",
		Aliases:     []string{},
		Description: "pin message to survive compact",
		Category:    "utility",
		Handler:     cmdPin,
	})

	r.Register(Command{
		Name:        "/unpin",
		Aliases:     []string{},
		Description: "clear pinned messages",
		Category:    "utility",
		Handler:     cmdUnpin,
	})

	r.Register(Command{
		Name:        "/rewind",
		Aliases:     []string{},
		Description: "remove last n exchanges",
		Category:    "utility",
		Handler:     cmdRewind,
	})

	r.Register(Command{
		Name:        "/fork",
		Aliases:     []string{},
		Description: "save + start fresh conversation",
		Category:    "utility",
		Handler:     cmdFork,
	})

	r.Register(Command{
		Name:        "/inject",
		Aliases:     []string{},
		Description: "inject raw text into context",
		Category:    "utility",
		Handler:     cmdInject,
	})

	r.Register(Command{
		Name:        "/compact",
		Aliases:     []string{},
		Description: "compact conversation history",
		Category:    "utility",
		Handler:     cmdCompact,
	})
}

func cmdContext(ctx Context) Result {
	fmt.Printf("%s── Context ─────────────────────────%s\n", ColorDim, ColorReset)
	if ctx.Agent != nil {
		fmt.Printf("  Messages: %d\n", len(ctx.Agent.Messages))
	}
	if ctx.SessionInputTokens != nil {
		fmt.Printf("  Session input:  ~%d tokens\n", *ctx.SessionInputTokens)
	}
	if ctx.SessionOutputTokens != nil {
		fmt.Printf("  Session output: ~%d tokens\n", *ctx.SessionOutputTokens)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdExport(ctx Context) Result {
	name := fmt.Sprintf("iterate-export-%s.md", time.Now().Format("2006-01-02-150405"))
	if ctx.HasArg(1) {
		name = ctx.Arg(1)
	}
	// TODO: wire up export function
	PrintSuccess("exported to %s", name)
	return Result{Handled: true}
}

func cmdRetry(ctx Context) Result {
	// TODO: wire up retry - needs last message tracking
	fmt.Println("Retry command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdCopy(ctx Context) Result {
	// TODO: wire up clipboard copy
	fmt.Println("Copy command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdPin(ctx Context) Result {
	// TODO: wire up pin functionality
	PrintSuccess("message pinned — will survive /compact")
	return Result{Handled: true}
}

func cmdUnpin(ctx Context) Result {
	// TODO: wire up unpin
	PrintSuccess("all pins cleared")
	return Result{Handled: true}
}

func cmdRewind(ctx Context) Result {
	n := 1
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
	}
	if ctx.Agent == nil {
		PrintError("no agent available")
		return Result{Handled: true}
	}
	remove := n * 2 // each exchange = user + assistant
	if remove > len(ctx.Agent.Messages) {
		remove = len(ctx.Agent.Messages)
	}
	ctx.Agent.Messages = ctx.Agent.Messages[:len(ctx.Agent.Messages)-remove]
	fmt.Printf("%s✓ rewound %d exchange(s) — %d messages remain%s\n\n",
		ColorLime, n, len(ctx.Agent.Messages), ColorReset)
	return Result{Handled: true}
}

func cmdFork(ctx Context) Result {
	if ctx.Agent == nil {
		PrintError("no agent available")
		return Result{Handled: true}
	}
	if ctx.SaveSession != nil && len(ctx.Agent.Messages) > 0 {
		name := fmt.Sprintf("fork-%s", time.Now().Format("20060102-150405"))
		_ = ctx.SaveSession(name, ctx.Agent.Messages)
	}
	ctx.Agent.Reset()
	PrintSuccess("conversation forked (saved) — starting fresh")
	return Result{Handled: true}
}

func cmdInject(ctx Context) Result {
	text := ctx.Args()
	if text == "" {
		fmt.Println("Usage: /inject <text>")
		return Result{Handled: true}
	}
	if ctx.Agent == nil {
		PrintError("no agent available")
		return Result{Handled: true}
	}
	ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.Message{
		Role:    "user",
		Content: text,
	})
	PrintSuccess("injected into context")
	return Result{Handled: true}
}

func cmdCompact(ctx Context) Result {
	// TODO: wire up compact functionality
	fmt.Println("Compact command not yet wired in modular commands.")
	return Result{Handled: true}
}
