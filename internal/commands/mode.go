package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	ver := ctx.Version
	if ver == "" {
		ver = "dev"
	}
	fmt.Printf("iterate  version %s\n", ver)
	fmt.Printf("iteragent (SDK embedded)\n")
	if ctx.Provider != nil {
		fmt.Printf("provider: %s\n\n", ctx.Provider.Name())
	} else {
		fmt.Println()
	}
	return Result{Handled: true}
}

func cmdCode(ctx Context) Result {
	if ctx.CurrentMode != nil {
		*ctx.CurrentMode = 0 // modeNormal
	}
	if ctx.REPL.MakeAgent != nil {
		ctx.REPL.MakeAgent()
	}
	PrintSuccess("switched to code mode (all tools enabled)")
	return Result{Handled: true}
}

func cmdAsk(ctx Context) Result {
	if ctx.CurrentMode != nil {
		*ctx.CurrentMode = 1 // modeAsk (read-only)
	}
	if ctx.REPL.MakeAgent != nil {
		ctx.REPL.MakeAgent()
	}
	PrintSuccess("switched to ask mode (read-only tools)")
	return Result{Handled: true}
}

func cmdArchitect(ctx Context) Result {
	if ctx.CurrentMode != nil {
		*ctx.CurrentMode = 2 // modeArchitect (no tools)
	}
	if ctx.REPL.MakeAgent != nil {
		ctx.REPL.MakeAgent()
	}
	PrintSuccess("switched to architect mode (no tools)")
	return Result{Handled: true}
}

func cmdSummarize(ctx Context) Result {
	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		PrintError("no conversation to summarize")
		return Result{Handled: true}
	}
	msgs := ctx.Agent.Messages
	prompt := fmt.Sprintf(
		"Summarize this conversation in 3-5 bullet points. Focus on: what was asked, "+
			"what was implemented, and any decisions made. Be brief.\n\n"+
			"(Conversation has %d messages)", len(msgs))
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdReview(ctx Context) Result {
	// Get the current diff
	out, _ := exec.Command("git", "-C", ctx.RepoPath, "diff", "HEAD").Output()
	diff := strings.TrimSpace(string(out))
	if diff == "" {
		out, _ = exec.Command("git", "-C", ctx.RepoPath, "diff").Output()
		diff = strings.TrimSpace(string(out))
	}
	prompt := "Review the current code changes. Look for: bugs, security issues, performance problems, " +
		"missing error handling, and style violations. Be concise and actionable.\n\n"
	if diff != "" {
		if len(diff) > 6000 {
			diff = diff[:6000] + "\n…[truncated]"
		}
		prompt += "```diff\n" + diff + "\n```"
	} else {
		prompt += "No diff found — review the overall codebase structure and quality."
	}
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdExplain(ctx Context) Result {
	path := ctx.Args()
	if path == "" {
		path = "."
	}
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(ctx.RepoPath, path)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		PrintError("cannot read %s: %v", path, err)
		return Result{Handled: true}
	}
	prompt := fmt.Sprintf("Explain the following code from %s. Describe: purpose, key functions/types, "+
		"data flow, and any notable patterns.\n\n```\n%s\n```", path, string(data))
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdView(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /view <file>")
		return Result{Handled: true}
	}
	filePath := ctx.Arg(1)
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(ctx.RepoPath, filePath)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		PrintError("cannot read %s: %v", filePath, err)
		return Result{Handled: true}
	}
	lines := strings.Split(string(data), "\n")
	fmt.Printf("%s── %s (%d lines) ──%s\n", ColorDim, filePath, len(lines), ColorReset)
	for i, line := range lines {
		fmt.Printf("%4d │ %s\n", i+1, line)
	}
	fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdShow(ctx Context) Result {
	ref := "HEAD"
	if ctx.HasArg(1) {
		ref = ctx.Args()
	}
	cmd := exec.Command("git", "-C", ctx.RepoPath, "show", "--stat", ref)
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("git show failed: %s", err)
		return Result{Handled: true}
	}
	fmt.Println(strings.TrimSpace(string(output)))
	return Result{Handled: true}
}

func cmdTree(ctx Context) Result {
	maxDepth := 4
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &maxDepth)
	}
	fmt.Printf("%sProject tree (git ls-files):%s\n", ColorDim, ColorReset)
	tree := BuildProjectTree(ctx.RepoPath, maxDepth)
	if tree == "" {
		fmt.Println("  (no files found)")
	} else {
		fmt.Println(tree)
	}
	fmt.Println()
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
	available := []string{"default", "nord", "monokai", "minimal"}
	if !ctx.HasArg(1) {
		fmt.Printf("Available themes: %s\n", strings.Join(available, ", "))
		fmt.Println("Usage: /theme <name>")
		return Result{Handled: true}
	}
	name := ctx.Arg(1)
	found := false
	for _, t := range available {
		if t == name {
			found = true
			break
		}
	}
	if !found {
		PrintError("unknown theme: %s (available: %s)", name, strings.Join(available, ", "))
		return Result{Handled: true}
	}
	if ctx.ApplyTheme != nil {
		ctx.ApplyTheme(name)
	}
	PrintSuccess("theme set to %s", name)
	return Result{Handled: true}
}
