package commands

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterUtilityCommands adds utility/context management commands.
func RegisterUtilityCommands(r *Registry) {
	registerUtilityContextCommands(r)
	registerUtilityActionCommands(r)
}

func registerUtilityContextCommands(r *Registry) {
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
		Name:        "/compact",
		Aliases:     []string{},
		Description: "compact conversation history",
		Category:    "utility",
		Handler:     cmdCompact,
	})
}

func registerUtilityActionCommands(r *Registry) {
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

	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		PrintError("no conversation to export")
		return Result{Handled: true}
	}

	f, err := os.Create(name)
	if err != nil {
		PrintError("failed to create file: %v", err)
		return Result{Handled: true}
	}
	defer f.Close()

	fmt.Fprintf(f, "# Iterate Export — %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	for i, msg := range ctx.Agent.Messages {
		role := msg.Role
		switch role {
		case "user":
			role = "User"
		case "assistant":
			role = "Assistant"
		case "system":
			role = "System"
		}
		fmt.Fprintf(f, "## %d. %s\n\n%s\n\n---\n\n", i+1, role, msg.Content)
	}

	PrintSuccess("exported to %s", name)
	return Result{Handled: true}
}

func cmdRetry(ctx Context) Result {
	if ctx.Agent == nil || len(ctx.Agent.Messages) < 2 {
		PrintError("no conversation to retry")
		return Result{Handled: true}
	}
	msgs := ctx.Agent.Messages
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" {
		PrintError("last message is not from assistant")
		return Result{Handled: true}
	}
	ctx.Agent.Messages = msgs[:len(msgs)-1]
	PrintSuccess("removed last response — resend your message to retry")
	return Result{Handled: true}
}

func cmdCopy(ctx Context) Result {
	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		PrintError("no messages to copy")
		return Result{Handled: true}
	}
	last := ctx.Agent.Messages[len(ctx.Agent.Messages)-1]
	if last.Role != "assistant" {
		PrintError("last message is not from assistant")
		return Result{Handled: true}
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("pbcopy"); err == nil {
		cmd = exec.Command("pbcopy")
	} else if _, err := exec.LookPath("xclip"); err == nil {
		cmd = exec.Command("xclip", "-selection", "clipboard")
	} else if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd = exec.Command("wl-copy")
	} else {
		PrintError("no clipboard tool found (pbcopy, xclip, wl-copy)")
		return Result{Handled: true}
	}

	cmd.Stdin = strings.NewReader(last.Content)
	if err := cmd.Run(); err != nil {
		PrintError("clipboard copy failed: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("copied last response to clipboard")
	return Result{Handled: true}
}

func pinsPath(repoPath string) string {
	return filepath.Join(repoPath, ".iterate", "pins.json")
}

func loadPins(repoPath string) []iteragent.Message {
	data, err := os.ReadFile(pinsPath(repoPath))
	if err != nil {
		return nil
	}
	var pins []iteragent.Message
	if err := json.Unmarshal(data, &pins); err != nil {
		return nil
	}
	return pins
}

func savePins(repoPath string, pins []iteragent.Message) {
	if err := os.MkdirAll(filepath.Join(repoPath, ".iterate"), 0755); err != nil {
		slog.Warn("failed to create .iterate dir for pins", "err", err)
		return
	}
	data, err := json.MarshalIndent(pins, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal pins", "err", err)
		return
	}
	if err := os.WriteFile(pinsPath(repoPath), data, 0644); err != nil {
		slog.Warn("failed to write pins file", "err", err)
	}
}

func cmdPin(ctx Context) Result {
	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		PrintError("no messages to pin")
		return Result{Handled: true}
	}
	last := ctx.Agent.Messages[len(ctx.Agent.Messages)-1]
	pins := loadPins(ctx.RepoPath)
	pins = append(pins, last)
	savePins(ctx.RepoPath, pins)
	PrintSuccess("message pinned — will survive /compact")
	return Result{Handled: true}
}

func cmdUnpin(ctx Context) Result {
	savePins(ctx.RepoPath, nil)
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
	if ctx.Session.SaveSession != nil && len(ctx.Agent.Messages) > 0 {
		name := fmt.Sprintf("fork-%s", time.Now().Format("20060102-150405"))
		_ = ctx.Session.SaveSession(name, ctx.Agent.Messages) // best-effort cleanup
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
	if ctx.Agent == nil || len(ctx.Agent.Messages) == 0 {
		PrintError("no conversation to compact")
		return Result{Handled: true}
	}

	before := len(ctx.Agent.Messages)
	msgs := ctx.Agent.Messages

	pins := loadPins(ctx.RepoPath)
	pinKeys := make(map[string]bool)
	for _, p := range pins {
		pinKeys[p.Role+":"+p.Content] = true
	}

	var kept []iteragent.Message
	keptKeys := make(map[string]bool)

	for _, msg := range msgs {
		if msg.Role == "system" {
			key := msg.Role + ":" + msg.Content
			if !keptKeys[key] {
				kept = append(kept, msg)
				keptKeys[key] = true
			}
		}
	}

	for _, msg := range pins {
		key := msg.Role + ":" + msg.Content
		if !keptKeys[key] {
			kept = append(kept, msg)
			keptKeys[key] = true
		}
	}

	start := len(msgs) - 20
	if start < 0 {
		start = 0
	}
	for _, msg := range msgs[start:] {
		key := msg.Role + ":" + msg.Content
		if !keptKeys[key] {
			kept = append(kept, msg)
			keptKeys[key] = true
		}
	}

	ctx.Agent.Messages = kept
	after := len(kept)
	PrintSuccess("compacted: %d → %d messages", before, after)
	return Result{Handled: true}
}
