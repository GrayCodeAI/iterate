package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/ui/highlight"
)

// RegisterModeCommands adds agent mode and display commands.
func RegisterModeCommands(r *Registry) {
	registerModeCoreCommands(r)
	registerAgentModeCommands(r)
	registerDisplayCommands(r)
}

func registerModeCoreCommands(r *Registry) {
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
}

func registerAgentModeCommands(r *Registry) {
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
}

func registerDisplayCommands(r *Registry) {
	registerMany(r, "mode",
		"/summarize", "summarize conversation", cmdSummarize,
		"/review", "review current changes", cmdReview,
		"/explain", "explain code in path", cmdExplain,
		"/view", "view file with line numbers", cmdView,
	)
	r.Register(Command{
		Name:        "/chain",
		Aliases:     []string{},
		Description: "run prompts sequentially, separated by ;; (e.g. /chain fix tests ;; commit the changes)",
		Category:    "mode",
		Handler:     cmdChain,
	})
	registerDisplayNavCommands(r)
}

func registerDisplayNavCommands(r *Registry) {
	registerMany(r, "mode",
		"/show", "show file or symbol", cmdShow,
		"/tree", "show directory tree", cmdTree,
		"/stats", "show session statistics", cmdStats,
		"/theme", "set color theme", cmdTheme,
		"/shortcuts", "list all keyboard shortcuts", cmdShortcuts,
		"/providers", "list configured providers and status", cmdProviders,
		"/render", "toggle markdown vs raw response rendering", cmdRender,
	)
}

// helpExamples maps command names to their usage examples.
var helpExamples = map[string][][2]string{
	"/cache": {
		{"/cache", "show current cache state"},
		{"/cache on", "enable caching (saves cost on long sessions)"},
		{"/cache off", "disable caching"},
	},
	"/budget": {
		{"/budget", "show current spend vs limit"},
		{"/budget 5.00", "set a $5.00 spending limit"},
		{"/budget 0", "clear the spending limit"},
	},
	"/compare": {
		{"/compare groq what is a monad?", "compare current provider vs groq"},
		{"/compare anthropic explain this code", "side-by-side A/B test"},
	},
	"/template": {
		{"/template save fix-tests", "save last prompt as template 'fix-tests'"},
		{"/template list", "show all saved templates"},
		{"/template use fix-tests", "queue 'fix-tests' for next message"},
		{"/template delete fix-tests", "remove template 'fix-tests'"},
	},
	"/t": {
		{"/t fix-tests", "quick shortcut to queue template 'fix-tests'"},
	},
	"/capabilities": {
		{"/capabilities", "show streaming, tools, context window info"},
		{"/caps", "same via alias"},
	},
	"/thinking": {
		{"/thinking off", "disable extended thinking"},
		{"/thinking minimal", "minimal thinking"},
		{"/thinking high", "maximum thinking depth"},
	},
	"/model": {
		{"/model", "open interactive model picker"},
	},
	"/safe": {
		{"/safe", "show current safe-mode state"},
		{"/safe on", "enable safe mode (confirms destructive ops)"},
		{"/safe off", "disable safe mode"},
	},
	"/image": {
		{"/image path/to/screenshot.png", "attach image to next message"},
	},
}

func cmdHelp(ctx Context) Result {
	// /help <command> — show description for a specific command.
	if ctx.HasArg(1) && ctx.Registry != nil {
		name := ctx.Arg(1)
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		cmd, ok := ctx.Registry.Lookup(name)
		if !ok {
			fmt.Printf("Unknown command: %s\n", name)
			return Result{Handled: true}
		}
		fmt.Printf("\n%s%s%s — %s\n", ColorBold, cmd.Name, ColorReset, cmd.Description)
		if len(cmd.Aliases) > 0 {
			fmt.Printf("  %saliases:%s %s\n", ColorDim, ColorReset, strings.Join(cmd.Aliases, ", "))
		}
		// Print usage examples if available.
		if examples, ok := helpExamples[cmd.Name]; ok {
			fmt.Printf("\n  %sExamples:%s\n", ColorDim, ColorReset)
			for _, ex := range examples {
				fmt.Printf("    %s%-40s%s %s%s%s\n",
					ColorCyan, ex[0], ColorReset,
					ColorDim, "— "+ex[1], ColorReset)
			}
		}
		fmt.Println()
		return Result{Handled: true}
	}

	// Dynamic help generated from the registry when available.
	if ctx.Registry != nil {
		byCategory := ctx.Registry.ByCategory()
		// Sort categories for stable output.
		cats := make([]string, 0, len(byCategory))
		for c := range byCategory {
			cats = append(cats, c)
		}
		sort.Strings(cats)

		fmt.Printf("\n%sAvailable commands%s\n\n", ColorBold, ColorReset)
		for _, cat := range cats {
			cmds := byCategory[cat]
			sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
			fmt.Printf("%s%s%s\n", ColorDim, strings.ToUpper(cat), ColorReset)
			for _, cmd := range cmds {
				alias := ""
				if len(cmd.Aliases) > 0 {
					alias = fmt.Sprintf("  %s(%s)%s", ColorDim, strings.Join(cmd.Aliases, ", "), ColorReset)
				}
				fmt.Printf("  %-22s %s%s\n", cmd.Name, cmd.Description, alias)
			}
			fmt.Println()
		}
		fmt.Printf("Tip: %s/help <command>%s for details on any command.\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	// Fallback static help when registry is unavailable (e.g., in tests).
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
		ctx.REPL.StreamAndPrint(context.Background(), ctx.Agent, prompt, ctx.RepoPath)
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

func cmdShortcuts(ctx Context) Result {
	fmt.Printf("%s── Keyboard Shortcuts ──────────────────────%s\n", ColorDim, ColorReset)
	shortcuts := [][2]string{
		{"Enter", "submit prompt"},
		{"↑ / ↓", "navigate history"},
		{"← / →", "move cursor left / right"},
		{"Ctrl+A", "move to beginning of line"},
		{"Ctrl+E", "move to end of line"},
		{"Ctrl+W", "delete word backward"},
		{"Ctrl+U", "delete to beginning of line"},
		{"Ctrl+K", "delete to end of line"},
		{"Ctrl+Y", "yank (paste) last killed text"},
		{"Tab", "autocomplete command or file path"},
		{"Ctrl+R", "fuzzy search history"},
		{"Ctrl+C (line)", "clear current input"},
		{"Ctrl+C (empty)", "cancel in-progress request"},
		{"Ctrl+D", "exit iterate"},
		{"Delete", "delete character under cursor"},
		{"\\\\<Enter>", "continue input on next line"},
	}
	for _, s := range shortcuts {
		fmt.Printf("  %s%-22s%s %s\n", ColorBold, s[0], ColorReset, s[1])
	}
	fmt.Printf("%s────────────────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdProviders(ctx Context) Result {
	fmt.Printf("%s── Configured Providers ────────────────────%s\n", ColorDim, ColorReset)

	type providerInfo struct {
		name   string
		envKey string
		models string
	}
	providers := []providerInfo{
		{"anthropic", "ANTHROPIC_API_KEY", "claude-sonnet-4-6, claude-3-5-sonnet, claude-3-haiku"},
		{"openai", "OPENAI_API_KEY", "gpt-4o, gpt-4o-mini, gpt-4-turbo"},
		{"openrouter", "OPENROUTER_API_KEY", "anthropic/claude-sonnet-4, openai/gpt-4o, google/gemini-2.5-flash"},
		{"gemini", "GEMINI_API_KEY", "gemini-2.0-flash, gemini-1.5-pro, gemini-2.5-pro"},
		{"groq", "GROQ_API_KEY", "llama-3.3-70b-versatile, llama-3.1-8b-instant"},
		{"ollama", "OLLAMA_BASE_URL", "llama3, codellama, mistral (local)"},
		{"nvidia", "NVIDIA_API_KEY", "meta/llama-3.3-70b-instruct"},
		{"opencode", "OPENCODE_API_KEY", "mimo-v2-pro-free"},
		{"opencode-cli", "(no key needed)", "mimo-v2-pro-free (via CLI)"},
		{"deepseek", "ITERATE_API_KEY", "deepseek-chat, deepseek-reasoner"},
		{"mistral", "ITERATE_API_KEY", "mistral-large, mistral-small"},
		{"azure", "ITERATE_API_KEY", "gpt-4o, gpt-4o-mini (via ITERATE_BASE_URL)"},
	}

	currentProvider := os.Getenv("ITERATE_PROVIDER")
	if currentProvider == "" {
		currentProvider = "gemini"
	}
	currentModel := os.Getenv("ITERATE_MODEL")

	for _, p := range providers {
		status := "  "
		marker := "  "
		if p.name == currentProvider {
			marker = ColorLime + "▶ " + ColorReset
			status = ColorBold
		}
		keySet := os.Getenv(p.envKey) != ""
		keyStatus := ""
		if p.envKey == "(no key needed)" {
			keyStatus = ColorDim + " (no key)" + ColorReset
		} else if keySet {
			keyStatus = ColorLime + " ✓ key set" + ColorReset
		} else {
			keyStatus = ColorDim + " (no key)" + ColorReset
		}
		fmt.Printf("%s%s%s%-14s%s%s%s\n", marker, status, ColorBold, p.name, ColorReset, keyStatus, "")
		if p.name == currentProvider && currentModel != "" {
			fmt.Printf("       model: %s%s%s\n", ColorCyan, currentModel, ColorReset)
		} else {
			fmt.Printf("       %s%s%s\n", ColorDim, p.models, ColorReset)
		}
	}
	fmt.Println()
	fmt.Printf("  Use %s/provider <name>%s to switch providers.\n\n", ColorBold, ColorReset)
	return Result{Handled: true}
}

func cmdRender(ctx Context) Result {
	// Toggle or explicitly set: /format [markdown|raw|on|off]
	if ctx.HasArg(1) {
		arg := strings.ToLower(ctx.Arg(1))
		switch arg {
		case "markdown", "md", "on", "true":
			highlight.MarkdownEnabled = true
		case "raw", "plain", "off", "false":
			highlight.MarkdownEnabled = false
		default:
			fmt.Printf("Usage: /format [markdown|raw]\n")
			return Result{Handled: true}
		}
	} else {
		highlight.MarkdownEnabled = !highlight.MarkdownEnabled
	}
	if highlight.MarkdownEnabled {
		PrintSuccess("format: markdown (syntax highlighting on)")
	} else {
		PrintSuccess("format: raw (plain text output)")
	}
	return Result{Handled: true}
}

// cmdChain runs multiple prompts sequentially, separated by ";;".
// Example: /chain write tests for auth.go ;; run the tests ;; fix any failures
func cmdChain(ctx Context) Result {
	if ctx.REPL.StreamAndPrint == nil || ctx.Agent == nil {
		PrintError("agent not available")
		return Result{Handled: true}
	}

	raw := ctx.Args()
	if raw == "" {
		fmt.Printf("%sUsage: /chain prompt1 ;; prompt2 ;; prompt3%s\n", ColorDim, ColorReset)
		fmt.Printf("%sEach step runs after the previous one completes.%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	steps := strings.Split(raw, ";;")
	var prompts []string
	for _, s := range steps {
		s = strings.TrimSpace(s)
		if s != "" {
			prompts = append(prompts, s)
		}
	}
	if len(prompts) == 0 {
		PrintError("no prompts found — separate them with ;;")
		return Result{Handled: true}
	}

	fmt.Printf("%s── Chain: %d steps ──────────────────────%s\n\n", ColorDim, len(prompts), ColorReset)
	for i, prompt := range prompts {
		fmt.Printf("%s[%d/%d]%s  %s%s%s\n\n", ColorDim, i+1, len(prompts), ColorReset, ColorYellow, prompt, ColorReset)
		ctx.REPL.StreamAndPrint(context.Background(), ctx.Agent, prompt, ctx.RepoPath)
	}
	fmt.Printf("%s── Chain complete ───────────────────────%s\n\n", ColorLime, ColorReset)
	return Result{Handled: true}
}
