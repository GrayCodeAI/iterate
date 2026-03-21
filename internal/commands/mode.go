package commands

import (
	"fmt"
)

// RegisterModeCommands adds agent mode and display commands.
func RegisterModeCommands(r *Registry) {
	r.Register(Command{
		Name:        "/help",
		Aliases:     []string{"/?"},
		Description: "show available commands",
		Category:    "mode",
		Handler:     cmdHelp,
	})

	r.Register(Command{
		Name:        "/version",
		Aliases:     []string{"/v"},
		Description: "show version",
		Category:    "mode",
		Handler:     cmdVersion,
	})

	r.Register(Command{
		Name:        "/code",
		Aliases:     []string{},
		Description: "full mode with all tools (default)",
		Category:    "mode",
		Handler:     cmdCode,
	})

	r.Register(Command{
		Name:        "/ask",
		Aliases:     []string{},
		Description: "read-only mode",
		Category:    "mode",
		Handler:     cmdAsk,
	})

	r.Register(Command{
		Name:        "/architect",
		Aliases:     []string{},
		Description: "planning mode (no tools)",
		Category:    "mode",
		Handler:     cmdArchitect,
	})

	r.Register(Command{
		Name:        "/summarize",
		Aliases:     []string{},
		Description: "summarize conversation",
		Category:    "mode",
		Handler:     cmdSummarize,
	})

	r.Register(Command{
		Name:        "/review",
		Aliases:     []string{},
		Description: "review current changes",
		Category:    "mode",
		Handler:     cmdReview,
	})

	r.Register(Command{
		Name:        "/explain",
		Aliases:     []string{},
		Description: "explain code in path",
		Category:    "mode",
		Handler:     cmdExplain,
	})

	r.Register(Command{
		Name:        "/view",
		Aliases:     []string{},
		Description: "view file with line numbers",
		Category:    "mode",
		Handler:     cmdView,
	})

	r.Register(Command{
		Name:        "/show",
		Aliases:     []string{},
		Description: "show file or symbol",
		Category:    "mode",
		Handler:     cmdShow,
	})

	r.Register(Command{
		Name:        "/tree",
		Aliases:     []string{},
		Description: "show directory tree",
		Category:    "mode",
		Handler:     cmdTree,
	})

	r.Register(Command{
		Name:        "/stats",
		Aliases:     []string{},
		Description: "show session statistics",
		Category:    "mode",
		Handler:     cmdStats,
	})

	r.Register(Command{
		Name:        "/theme",
		Aliases:     []string{},
		Description: "set color theme",
		Category:    "mode",
		Handler:     cmdTheme,
	})
}

func cmdHelp(ctx Context) Result {
	fmt.Print(`
Available commands:
  /help                  — show this help
  /clear                 — reset conversation history
  /compact               — compact conversation history
  /tools                 — list available tools
  /skills                — list available skills
  /thinking <level>      — set thinking level: off|minimal|low|medium|high
  /model                 — switch provider/model interactively
  /cost                  — show approximate token usage this session

  /save [name]           — save session
  /load [name]           — load saved session
  /bookmark [name]       — save current conversation as a bookmark
  /bookmarks             — list and restore bookmarks
  /history               — show recent input history

  /test / /build / /lint — run tests, build, linter
  /commit <msg>          — git add -A && git commit
  /diff / /status        — git diff / status

  /phase plan|implement  — run evolution phase
  /journal               — view JOURNAL.md
  /day [n]               — show/set evolution day

  /swarm <n> <task>      — launch N agents with rate limiting (max 100)
  /spawn <task>          — delegate to subagent
  /issues                — list open GitHub issues
  /pr                    — create pull request

  /quit                  — exit (auto-saves session)
`)
	return Result{Handled: true}
}

func cmdVersion(ctx Context) Result {
	fmt.Println("iterate version dev")
	return Result{Handled: true}
}

func cmdCode(ctx Context) Result {
	// TODO: wire up mode switching
	PrintSuccess("switched to code mode (all tools enabled)")
	return Result{Handled: true}
}

func cmdAsk(ctx Context) Result {
	// TODO: wire up mode switching
	PrintSuccess("switched to ask mode (read-only tools)")
	return Result{Handled: true}
}

func cmdArchitect(ctx Context) Result {
	// TODO: wire up mode switching
	PrintSuccess("switched to architect mode (no tools)")
	return Result{Handled: true}
}

func cmdSummarize(ctx Context) Result {
	// TODO: wire up via agent stream
	fmt.Println("Summarize command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdReview(ctx Context) Result {
	// TODO: wire up via agent stream
	fmt.Println("Review command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdExplain(ctx Context) Result {
	path := ctx.Args()
	if path == "" {
		path = "."
	}
	// TODO: wire up via agent stream
	fmt.Printf("Explain %s not yet wired in modular commands.\n", path)
	return Result{Handled: true}
}

func cmdView(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /view <file>")
		return Result{Handled: true}
	}
	// TODO: wire up file viewing
	fmt.Println("View command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdShow(ctx Context) Result {
	// TODO: wire up show functionality
	fmt.Println("Show command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdTree(ctx Context) Result {
	// TODO: wire up directory tree
	fmt.Println("Tree command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdStats(ctx Context) Result {
	fmt.Printf("%s── Session Statistics ─────────────%s\n", ColorDim, ColorReset)
	if ctx.SessionInputTokens != nil {
		fmt.Printf("  Input tokens:  ~%d\n", *ctx.SessionInputTokens)
	}
	if ctx.SessionOutputTokens != nil {
		fmt.Printf("  Output tokens: ~%d\n", *ctx.SessionOutputTokens)
	}
	if ctx.Agent != nil {
		fmt.Printf("  Messages:      %d\n", len(ctx.Agent.Messages))
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdTheme(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Themes: default, dark, light, mono")
		return Result{Handled: true}
	}
	// TODO: wire up theme switching
	PrintSuccess("theme set to %s", ctx.Arg(1))
	return Result{Handled: true}
}
