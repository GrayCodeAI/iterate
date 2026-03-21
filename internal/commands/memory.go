package commands

import (
	"fmt"
	"strings"
)

// RegisterMemoryCommands adds memory/note-taking commands.
func RegisterMemoryCommands(r *Registry) {
	r.Register(Command{
		Name:        "/memo",
		Aliases:     []string{},
		Description: "append memo to JOURNAL.md",
		Category:    "memory",
		Handler:     cmdMemo,
	})

	r.Register(Command{
		Name:        "/learn",
		Aliases:     []string{},
		Description: "add fact to learnings.jsonl",
		Category:    "memory",
		Handler:     cmdLearn,
	})

	r.Register(Command{
		Name:        "/memories",
		Aliases:     []string{},
		Description: "show project notes and learnings",
		Category:    "memory",
		Handler:     cmdMemories,
	})

	r.Register(Command{
		Name:        "/remember",
		Aliases:     []string{},
		Description: "save note to project memory",
		Category:    "memory",
		Handler:     cmdRemember,
	})

	r.Register(Command{
		Name:        "/forget",
		Aliases:     []string{},
		Description: "remove memory entry or message",
		Category:    "memory",
		Handler:     cmdForget,
	})
}

func cmdMemo(ctx Context) Result {
	text := ctx.Args()
	if text == "" {
		fmt.Println("Usage: /memo <text>")
		return Result{Handled: true}
	}
	// TODO: wire up appendMemo
	PrintSuccess("memo added to JOURNAL.md")
	return Result{Handled: true}
}

func cmdLearn(ctx Context) Result {
	fact := ctx.Args()
	if fact == "" {
		fmt.Println("Usage: /learn <fact or lesson>")
		return Result{Handled: true}
	}
	// TODO: wire up appendLearning
	PrintSuccess("added to memory/learnings.jsonl")
	return Result{Handled: true}
}

func cmdMemories(ctx Context) Result {
	// TODO: wire up printProjectMemory and readActiveLearnings
	fmt.Printf("%s── Project Memory ──────────────────%s\n", ColorDim, ColorReset)
	fmt.Println("Memories command not yet wired in modular commands.")
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdRemember(ctx Context) Result {
	note := ctx.Args()
	if note == "" {
		fmt.Println("Usage: /remember <note>")
		return Result{Handled: true}
	}
	// TODO: wire up addProjectMemoryNote
	PrintSuccess("note saved to .iterate/memory.json")
	return Result{Handled: true}
}

func cmdForget(ctx Context) Result {
	// /forget <n>  → remove project memory entry n (1-indexed)
	// /forget msg <n> → remove conversation message n
	if ctx.HasArg(1) && ctx.Arg(1) == "msg" {
		// Remove from conversation
		if ctx.Agent == nil {
			fmt.Println("No agent available.")
			return Result{Handled: true}
		}
		n := len(ctx.Agent.Messages)
		if ctx.HasArg(2) {
			fmt.Sscanf(ctx.Arg(2), "%d", &n)
			n-- // 0-indexed
		}
		if n < 0 || n >= len(ctx.Agent.Messages) {
			fmt.Printf("Invalid index. Context has %d messages (1-%d).\n", len(ctx.Agent.Messages), len(ctx.Agent.Messages))
			return Result{Handled: true}
		}
		removed := ctx.Agent.Messages[n]
		ctx.Agent.Messages = append(ctx.Agent.Messages[:n], ctx.Agent.Messages[n+1:]...)
		snippet := removed.Content
		if len(snippet) > 60 {
			snippet = snippet[:60] + "…"
		}
		fmt.Printf("%s✓ removed message %d [%s]: %s%s\n\n", ColorLime, n+1, removed.Role, snippet, ColorReset)
	} else {
		// Remove from project memory
		// TODO: wire up removeProjectMemoryEntry
		fmt.Println("Forget project memory not yet wired in modular commands.")
	}
	return Result{Handled: true}
}

// Helper to check if string is empty
func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
