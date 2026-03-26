package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		Name:        "/history-search",
		Aliases:     []string{"/hs"},
		Description: "search session messages for a keyword",
		Category:    "session",
		Handler:     cmdHistorySearch,
	})

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

	r.Register(Command{
		Name:        "/auditlog",
		Aliases:     []string{"/alog"},
		Description: "show recent audit log entries [n]",
		Category:    "session",
		Handler:     cmdAuditLog,
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

// auditLogRecord mirrors the JSON Lines format written by logAudit in features_sessions.go.
type auditLogRecord struct {
	Timestamp string                 `json:"ts"`
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Result    string                 `json:"result,omitempty"`
	IsError   bool                   `json:"error,omitempty"`
}

func cmdAuditLog(ctx Context) Result {
	n := 20
	if ctx.HasArg(1) {
		if v, err := strconv.Atoi(ctx.Arg(1)); err == nil && v > 0 {
			n = v
		}
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".iterate", "audit.jsonl")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No audit log found.")
		} else {
			PrintError("open audit log: %v", err)
		}
		return Result{Handled: true}
	}
	defer f.Close()

	// Read all lines then take the last n.
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	recent := lines[start:]

	fmt.Printf("%s── Audit log (last %d) ─────────────%s\n", ColorDim, len(recent), ColorReset)
	for _, line := range recent {
		var rec auditLogRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			fmt.Printf("  %s%s%s\n", ColorDim, line, ColorReset)
			continue
		}
		ts := rec.Timestamp
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			ts = t.Local().Format("15:04:05")
		}
		statusCol := ColorLime
		statusIcon := "✓"
		if rec.IsError {
			statusCol = ColorRed
			statusIcon = "✗"
		}
		result := rec.Result
		if len(result) > 60 {
			result = result[:60] + "…"
		}
		result = strings.ReplaceAll(result, "\n", " ")
		fmt.Printf("  %s%s%s %s%-18s%s %s%s%s\n",
			statusCol, statusIcon, ColorReset,
			ColorDim, ts, ColorReset,
			ColorBold, rec.Tool, ColorReset)
		if result != "" {
			fmt.Printf("    %s%s%s\n", ColorDim, result, ColorReset)
		}
	}
	if len(recent) == 0 {
		fmt.Println("  (empty)")
	}
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

// cmdHistorySearch searches the current session history for a keyword (case-insensitive).
// Usage: /history-search <term>
func cmdHistorySearch(ctx Context) Result {
	query := strings.TrimSpace(ctx.Args())
	if query == "" {
		fmt.Println("Usage: /search <term>")
		return Result{Handled: true}
	}

	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		fmt.Println("No messages in current session.")
		return Result{Handled: true}
	}

	lower := strings.ToLower(query)
	type match struct {
		idx     int
		role    string
		excerpt string
	}
	var matches []match

	for i, msg := range ctx.Agent.Messages {
		content := msg.Content
		contentLower := strings.ToLower(content)
		if !strings.Contains(contentLower, lower) {
			continue
		}
		// Find first occurrence and build a short excerpt around it.
		pos := strings.Index(contentLower, lower)
		start := pos - 60
		if start < 0 {
			start = 0
		}
		end := pos + len(query) + 60
		if end > len(content) {
			end = len(content)
		}
		excerpt := strings.ReplaceAll(content[start:end], "\n", " ")
		if start > 0 {
			excerpt = "…" + excerpt
		}
		if end < len(content) {
			excerpt += "…"
		}
		matches = append(matches, match{idx: i + 1, role: msg.Role, excerpt: excerpt})
	}

	if len(matches) == 0 {
		fmt.Printf("No matches for %q in %d messages.\n\n", query, len(ctx.Agent.Messages))
		return Result{Handled: true}
	}

	fmt.Printf("%s── Search: %q — %d match(es) ──%s\n", ColorDim, query, len(matches), ColorReset)
	for _, m := range matches {
		roleColor := ColorDim
		switch m.role {
		case "user":
			roleColor = ColorCyan
		case "assistant":
			roleColor = ColorLime
		}
		fmt.Printf("  %s#%d %-10s%s %s\n", roleColor, m.idx, m.role, ColorReset, m.excerpt)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}
