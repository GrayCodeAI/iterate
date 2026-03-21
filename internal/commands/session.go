package commands

import (
	"fmt"
	"time"
)

// RegisterSessionCommands adds session management commands.
func RegisterSessionCommands(r *Registry) {
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

func cmdQuit(ctx Context) Result {
	if ctx.StopWatch != nil {
		ctx.StopWatch()
	}
	if ctx.Agent != nil && len(ctx.Agent.Messages) > 0 && ctx.SaveSession != nil {
		_ = ctx.SaveSession("autosave", ctx.Agent.Messages)
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
	if ctx.SaveSession == nil {
		PrintError("save not available")
		return Result{Handled: true}
	}
	if err := ctx.SaveSession(name, ctx.Agent.Messages); err != nil {
		PrintError("%s", err)
	} else {
		PrintSuccess("session saved as \"%s\"", name)
	}
	return Result{Handled: true}
}

func cmdLoad(ctx Context) Result {
	if ctx.ListSessions == nil || ctx.LoadSession == nil {
		PrintError("load not available")
		return Result{Handled: true}
	}
	sessions := ctx.ListSessions()
	if len(sessions) == 0 {
		fmt.Println("No saved sessions. Use /save to create one.")
		return Result{Handled: true}
	}
	var pick string
	if ctx.HasArg(1) {
		pick = ctx.Arg(1)
	} else if ctx.SelectItem != nil {
		var ok bool
		pick, ok = ctx.SelectItem("Select session to load", sessions)
		if !ok {
			return Result{Handled: true}
		}
	} else {
		PrintError("no session name provided")
		return Result{Handled: true}
	}
	msgs, err := ctx.LoadSession(pick)
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
	if ctx.AddBookmark == nil {
		PrintError("bookmark not available")
		return Result{Handled: true}
	}
	ctx.AddBookmark(name, ctx.Agent.Messages)
	PrintSuccess("bookmark \"%s\" saved", name)
	return Result{Handled: true}
}

func cmdBookmarks(ctx Context) Result {
	if ctx.LoadBookmarks == nil || ctx.SelectItem == nil {
		PrintError("bookmarks not available")
		return Result{Handled: true}
	}
	bms := ctx.LoadBookmarks()
	if len(bms) == 0 {
		fmt.Println("No bookmarks. Use /bookmark [name] to save one.")
		return Result{Handled: true}
	}
	labels := make([]string, len(bms))
	for i, b := range bms {
		labels[i] = fmt.Sprintf("%-30s  %s  (%d msgs)", b.Name, b.CreatedAt.Format("01-02 15:04"), len(b.Messages))
	}
	choice, ok := ctx.SelectItem("Select bookmark to restore", labels)
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
