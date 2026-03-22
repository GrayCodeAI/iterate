package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterSessionCommands adds session management commands.
func RegisterSessionCommands(r *Registry) {
	registerSessionCRUDCommands(r)
	registerSessionBookmarkCommands(r)
	registerSessionUtilityCommands(r)
}

func registerSessionCRUDCommands(r *Registry) {
	r.Register(Command{
		Name:        "/quit",
		Aliases:     []string{"/exit", "/q"},
		Description: "exit REPL (auto-saves session)",
		Category:    "session",
		Handler:     cmdQuit,
	})

	r.Register(Command{
		Name:        "/clear",
		Aliases:     []string{},
		Description: "clear conversation history",
		Category:    "session",
		Handler:     cmdClear,
	})

	r.Register(Command{
		Name:        "/save",
		Aliases:     []string{},
		Description: "save session [name]",
		Category:    "session",
		Handler:     cmdSave,
	})

	r.Register(Command{
		Name:        "/load",
		Aliases:     []string{},
		Description: "load session [name]",
		Category:    "session",
		Handler:     cmdLoad,
	})
}

func registerSessionBookmarkCommands(r *Registry) {
	r.Register(Command{
		Name:        "/bookmark",
		Aliases:     []string{},
		Description: "bookmark current state",
		Category:    "session",
		Handler:     cmdBookmark,
	})

	r.Register(Command{
		Name:        "/bookmarks",
		Aliases:     []string{},
		Description: "list and restore bookmarks",
		Category:    "session",
		Handler:     cmdBookmarks,
	})

	r.Register(Command{
		Name:        "/history",
		Aliases:     []string{},
		Description: "show command history",
		Category:    "session",
		Handler:     cmdHistory,
	})
}

func registerSessionUtilityCommands(r *Registry) {
	registerSessionUtilityA(r)
	registerSessionUtilityB(r)
}

func registerSessionTemplateCommands(r *Registry) {
	r.Register(Command{
		Name:        "/templates",
		Aliases:     []string{},
		Description: "list saved templates",
		Category:    "session",
		Handler:     cmdTemplates,
	})

	r.Register(Command{
		Name:        "/save-template",
		Aliases:     []string{},
		Description: "save last prompt as template",
		Category:    "session",
		Handler:     cmdSaveTemplate,
	})

	r.Register(Command{
		Name:        "/template",
		Aliases:     []string{},
		Description: "use a saved template",
		Category:    "session",
		Handler:     cmdTemplate,
	})
}

func registerSessionUtilityA(r *Registry) {
	registerSessionTemplateCommands(r)

	r.Register(Command{
		Name:        "/multi",
		Aliases:     []string{},
		Description: "multi-line input mode",
		Category:    "session",
		Handler:     cmdMulti,
	})

	r.Register(Command{
		Name:        "/compact-hard",
		Aliases:     []string{},
		Description: "hard compact keeping last N",
		Category:    "session",
		Handler:     cmdCompactHard,
	})

	r.Register(Command{
		Name:        "/pin-list",
		Aliases:     []string{},
		Description: "list pinned messages",
		Category:    "session",
		Handler:     cmdPinList,
	})

	r.Register(Command{
		Name:        "/pair",
		Aliases:     []string{},
		Description: "activate pair programming mode",
		Category:    "session",
		Handler:     cmdPair,
	})
}

func registerSessionUtilityB(r *Registry) {
	r.Register(Command{
		Name:        "/changes",
		Aliases:     []string{},
		Description: "show files changed this session",
		Category:    "session",
		Handler:     cmdChanges,
	})

	r.Register(Command{
		Name:        "/iterate-init",
		Aliases:     []string{},
		Description: "generate ITERATE.md context file",
		Category:    "session",
		Handler:     cmdIterateInit,
	})
}

func cmdQuit(ctx Context) Result {
	if ctx.StopWatch != nil {
		ctx.StopWatch()
	}
	if ctx.Agent != nil && len(ctx.Agent.Messages) > 0 && ctx.Session.SaveSession != nil {
		_ = ctx.Session.SaveSession("autosave", ctx.Agent.Messages) // best-effort cleanup
	}
	fmt.Printf("%sbye 🌱%s\n", ColorLime, ColorReset)
	return Result{Done: true, Handled: true}
}

func cmdClear(ctx Context) Result {
	if ctx.Agent != nil {
		ctx.Agent.Reset()
	}
	fmt.Println("Conversation cleared.")
	return Result{Handled: true}
}

func cmdSave(ctx Context) Result {
	name := "default"
	if ctx.HasArg(1) {
		name = ctx.Arg(1)
	}
	if ctx.Session.SaveSession == nil {
		PrintError("save not available")
		return Result{Handled: true}
	}
	if err := ctx.Session.SaveSession(name, ctx.Agent.Messages); err != nil {
		PrintError("%s", err)
	} else {
		PrintSuccess("session saved as \"%s\"", name)
	}
	return Result{Handled: true}
}

func cmdLoad(ctx Context) Result {
	if ctx.Session.ListSessions == nil || ctx.Session.LoadSession == nil {
		PrintError("load not available")
		return Result{Handled: true}
	}
	sessions := ctx.Session.ListSessions()
	if len(sessions) == 0 {
		fmt.Println("No saved sessions. Use /save to create one.")
		return Result{Handled: true}
	}
	var pick string
	if ctx.HasArg(1) {
		pick = ctx.Arg(1)
	} else if ctx.Session.SelectItem != nil {
		var ok bool
		pick, ok = ctx.Session.SelectItem("Select session to load", sessions)
		if !ok {
			return Result{Handled: true}
		}
	} else {
		PrintError("no session name provided")
		return Result{Handled: true}
	}
	msgs, err := ctx.Session.LoadSession(pick)
	if err != nil {
		PrintError("%s", err)
		return Result{Handled: true}
	}
	ctx.Agent.Messages = msgs
	PrintSuccess("loaded session \"%s\" (%d messages)", pick, len(msgs))
	return Result{Handled: true}
}

func cmdBookmark(ctx Context) Result {
	name := time.Now().Format("2006-01-02T15:04")
	if ctx.HasArg(1) {
		name = ctx.Args()
	}
	if ctx.Session.AddBookmark == nil {
		PrintError("bookmark not available")
		return Result{Handled: true}
	}
	ctx.Session.AddBookmark(name, ctx.Agent.Messages)
	PrintSuccess("bookmark \"%s\" saved", name)
	return Result{Handled: true}
}

func cmdBookmarks(ctx Context) Result {
	if ctx.Session.LoadBookmarks == nil || ctx.Session.SelectItem == nil {
		PrintError("bookmarks not available")
		return Result{Handled: true}
	}
	bms := ctx.Session.LoadBookmarks()
	if len(bms) == 0 {
		fmt.Println("No bookmarks. Use /bookmark [name] to save one.")
		return Result{Handled: true}
	}
	labels := make([]string, len(bms))
	for i, b := range bms {
		labels[i] = fmt.Sprintf("%-30s  %s  (%d msgs)", b.Name, b.CreatedAt.Format("01-02 15:04"), len(b.Messages))
	}
	choice, ok := ctx.Session.SelectItem("Select bookmark to restore", labels)
	if !ok {
		return Result{Handled: true}
	}
	for i, label := range labels {
		if label == choice {
			ctx.Agent.Messages = bms[i].Messages
			PrintSuccess("restored bookmark \"%s\"", bms[i].Name)
			break
		}
	}
	return Result{Handled: true}
}

func cmdHistory(ctx Context) Result {
	if ctx.InputHistory == nil || len(*ctx.InputHistory) == 0 {
		fmt.Println("No history yet.")
		return Result{Handled: true}
	}
	start := 0
	if len(*ctx.InputHistory) > 20 {
		start = len(*ctx.InputHistory) - 20
	}
	for i, h := range (*ctx.InputHistory)[start:] {
		fmt.Printf("  %s%3d%s  %s\n", ColorDim, start+i+1, ColorReset, h)
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdTemplates(ctx Context) Result {
	if ctx.Templates.LoadTemplates == nil {
		PrintError("template system not available")
		return Result{Handled: true}
	}

	ts := ctx.Templates.LoadTemplates()
	if len(ts) == 0 {
		fmt.Println("No templates. Use: /save-template <name> (saves last prompt)")
		return Result{Handled: true}
	}

	labels := make([]string, len(ts))
	for i, t := range ts {
		labels[i] = fmt.Sprintf("%-20s  %s", t.Name, t.Created.Format("01-02"))
	}

	if ctx.Session.SelectItem == nil {
		fmt.Printf("%s── Templates ──────────────────────%s\n", ColorDim, ColorReset)
		for _, label := range labels {
			fmt.Printf("  %s\n", label)
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	choice, ok := ctx.Session.SelectItem("Select template", labels)
	if !ok {
		return Result{Handled: true}
	}
	for i, label := range labels {
		if label == choice {
			PrintSuccess("using template: %s", ts[i].Name)
			if ctx.REPL.StreamAndPrint != nil {
				ctx.REPL.StreamAndPrint(nil, ctx.Agent, ts[i].Prompt, ctx.RepoPath)
			}
			break
		}
	}
	return Result{Handled: true}
}

func cmdSaveTemplate(ctx Context) Result {
	if ctx.LastPrompt == nil || *ctx.LastPrompt == "" {
		PrintError("no previous prompt to save")
		return Result{Handled: true}
	}
	if ctx.Templates.AddTemplate == nil {
		PrintError("template system not available")
		return Result{Handled: true}
	}

	name := ""
	if ctx.HasArg(1) {
		name = strings.Join(ctx.Parts[1:], " ")
	}
	if name == "" {
		name = "unnamed"
	}
	ctx.Templates.AddTemplate(name, *ctx.LastPrompt)
	PrintSuccess("template %q saved", name)
	return Result{Handled: true}
}

func cmdTemplate(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /template <name>  (use /templates to browse)")
		return Result{Handled: true}
	}
	if ctx.Templates.LoadTemplates == nil {
		PrintError("template system not available")
		return Result{Handled: true}
	}

	name := strings.Join(ctx.Parts[1:], " ")
	ts := ctx.Templates.LoadTemplates()
	for _, t := range ts {
		if strings.EqualFold(t.Name, name) {
			if ctx.REPL.StreamAndPrint != nil {
				ctx.REPL.StreamAndPrint(nil, ctx.Agent, t.Prompt, ctx.RepoPath)
			} else {
				PrintSuccess("template found: %s", t.Name)
			}
			return Result{Handled: true}
		}
	}
	fmt.Printf("Template %q not found. Use /templates to browse.\n", name)
	return Result{Handled: true}
}

func cmdMulti(ctx Context) Result {
	if ctx.REPL.ReadMultiLine == nil {
		PrintError("multi-line input not available")
		return Result{Handled: true}
	}

	text, ok := ctx.REPL.ReadMultiLine()
	if !ok || strings.TrimSpace(text) == "" {
		fmt.Println("Cancelled.")
		return Result{Handled: true}
	}
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, text, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCompactHard(ctx Context) Result {
	if ctx.Agent == nil {
		PrintError("agent not available")
		return Result{Handled: true}
	}
	keep := 6
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &keep)
	}
	before := len(ctx.Agent.Messages)
	if before <= keep {
		fmt.Printf("Only %d messages, nothing to compact.\n", before)
		return Result{Handled: true}
	}
	ctx.Agent.Messages = ctx.Agent.Messages[len(ctx.Agent.Messages)-keep:]
	fmt.Printf("%s✓ hard compacted: %d → %d messages%s\n\n", ColorLime, before, len(ctx.Agent.Messages), ColorReset)
	return Result{Handled: true}
}

func cmdPinList(ctx Context) Result {
	var msgs []iteragent.Message
	if ctx.State.GetPinnedMessages != nil {
		msgs = ctx.State.GetPinnedMessages()
	}
	if len(msgs) == 0 {
		fmt.Println("No pinned messages.")
		return Result{Handled: true}
	}
	fmt.Printf("%s── Pinned Messages ─────────────────%s\n", ColorDim, ColorReset)
	for i, m := range msgs {
		snippet := m.Content
		if len(snippet) > 80 {
			snippet = snippet[:80] + "…"
		}
		fmt.Printf("  [%d] %s: %s\n", i+1, m.Role, snippet)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPair(ctx Context) Result {
	if ctx.Agent == nil || ctx.REPL.StreamAndPrint == nil {
		PrintError("agent not available")
		return Result{Handled: true}
	}
	pairPrompt := "You are now in pair programming mode. For every change you make, " +
		"explain your reasoning first, then show the code. Ask for confirmation before " +
		"making destructive changes. Suggest alternatives when appropriate."
	ctx.REPL.StreamAndPrint(nil, ctx.Agent, pairPrompt, ctx.RepoPath)
	PrintSuccess("pair programming mode activated")
	return Result{Handled: true}
}

func cmdChanges(ctx Context) Result {
	if ctx.Templates.FormatSessionChanges == nil {
		fmt.Println("No session change tracking available.")
		return Result{Handled: true}
	}
	fmt.Printf("%s── File changes this session ────────%s\n", ColorDim, ColorReset)
	fmt.Println(ctx.Templates.FormatSessionChanges())
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdIterateInit(ctx Context) Result {
	iterateMDPath := filepath.Join(ctx.RepoPath, "ITERATE.md")
	if _, err := os.Stat(iterateMDPath); err == nil {
		if ctx.REPL.PromptLine != nil {
			confirm, ok := ctx.REPL.PromptLine("ITERATE.md already exists. Overwrite? (y/N): ")
			if !ok || strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Cancelled.")
				return Result{Handled: true}
			}
		} else {
			fmt.Printf("%sITERATE.md already exists. Overwrite? (y/N): %s", ColorYellow, ColorReset)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Cancelled.")
				return Result{Handled: true}
			}
		}
	}
	content := GenerateIterateMD(ctx.RepoPath)
	if err := os.WriteFile(iterateMDPath, []byte(content), 0o644); err != nil {
		PrintError("%s", err)
	} else {
		PrintSuccess("ITERATE.md generated — edit it to add project-specific context")
	}
	return Result{Handled: true}
}
