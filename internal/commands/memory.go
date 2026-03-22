package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	journalPath := filepath.Join(ctx.RepoPath, "JOURNAL.md")
	f, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		PrintError("failed to open JOURNAL.md: %v", err)
		return Result{Handled: true}
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04")
	entry := fmt.Sprintf("\n## Memo — %s\n\n%s\n", timestamp, text)
	if _, err := f.WriteString(entry); err != nil {
		PrintError("failed to write: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("memo added to JOURNAL.md")
	return Result{Handled: true}
}

func cmdLearn(ctx Context) Result {
	fact := ctx.Args()
	if fact == "" {
		fmt.Println("Usage: /learn <fact or lesson>")
		return Result{Handled: true}
	}

	learningsDir := filepath.Join(ctx.RepoPath, "memory")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		PrintError("failed to create memory dir: %v", err)
		return Result{Handled: true}
	}

	learningsPath := filepath.Join(learningsDir, "learnings.jsonl")
	f, err := os.OpenFile(learningsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		PrintError("failed to open learnings.jsonl: %v", err)
		return Result{Handled: true}
	}
	defer f.Close()

	entry := map[string]string{
		"timestamp": time.Now().Format(time.RFC3339),
		"fact":      fact,
	}
	jsonData, err := json.Marshal(entry)
	if err != nil {
		PrintError("failed to marshal: %v", err)
		return Result{Handled: true}
	}
	if _, err := f.WriteString(string(jsonData) + "\n"); err != nil {
		PrintError("failed to write: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("added to memory/learnings.jsonl")
	return Result{Handled: true}
}

func cmdMemories(ctx Context) Result {
	fmt.Printf("%s── Project Memory ──────────────────%s\n", ColorDim, ColorReset)

	memoryPath := filepath.Join(ctx.RepoPath, ".iterate", "memory.json")
	if data, err := os.ReadFile(memoryPath); err == nil && len(data) > 0 {
		var notes []map[string]string
		if json.Unmarshal(data, &notes) == nil && len(notes) > 0 {
			fmt.Printf("  %sProject notes:%s\n", ColorBold, ColorReset)
			for i, note := range notes {
				ts := note["timestamp"]
				text := note["note"]
				if len(text) > 80 {
					text = text[:80] + "…"
				}
				fmt.Printf("  %s%d%s  [%s] %s\n", ColorDim, i+1, ColorReset, ts, text)
			}
			fmt.Println()
		}
	}

	activePath := filepath.Join(ctx.RepoPath, "memory", "active_learnings.md")
	if data, err := os.ReadFile(activePath); err == nil && len(data) > 0 {
		fmt.Printf("  %sActive learnings:%s\n", ColorBold, ColorReset)
		fmt.Println("  " + strings.ReplaceAll(string(data), "\n", "\n  "))
		fmt.Println()
	}

	learningsPath := filepath.Join(ctx.RepoPath, "memory", "learnings.jsonl")
	if data, err := os.ReadFile(learningsPath); err == nil && len(data) > 0 {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		fmt.Printf("  %sLearnings: %d entries in learnings.jsonl%s\n", ColorDim, len(lines), ColorReset)
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdRemember(ctx Context) Result {
	note := ctx.Args()
	if note == "" {
		fmt.Println("Usage: /remember <note>")
		return Result{Handled: true}
	}

	iterateDir := filepath.Join(ctx.RepoPath, ".iterate")
	if err := os.MkdirAll(iterateDir, 0755); err != nil {
		PrintError("failed to create .iterate dir: %v", err)
		return Result{Handled: true}
	}

	memoryPath := filepath.Join(iterateDir, "memory.json")
	var notes []map[string]string
	if data, err := os.ReadFile(memoryPath); err == nil {
		json.Unmarshal(data, &notes)
	}

	notes = append(notes, map[string]string{
		"timestamp": time.Now().Format(time.RFC3339),
		"note":      note,
	})

	jsonData, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		PrintError("failed to marshal: %v", err)
		return Result{Handled: true}
	}
	if err := os.WriteFile(memoryPath, jsonData, 0644); err != nil {
		PrintError("failed to write: %v", err)
		return Result{Handled: true}
	}
	PrintSuccess("note saved to .iterate/memory.json")
	return Result{Handled: true}
}

func cmdForget(ctx Context) Result {
	if ctx.HasArg(1) && ctx.Arg(1) == "msg" {
		if ctx.Agent == nil {
			fmt.Println("No agent available.")
			return Result{Handled: true}
		}
		n := len(ctx.Agent.Messages)
		if ctx.HasArg(2) {
			fmt.Sscanf(ctx.Arg(2), "%d", &n)
			n--
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
		if !ctx.HasArg(1) {
			fmt.Println("Usage: /forget <n>  or  /forget msg <n>")
			return Result{Handled: true}
		}
		n := 0
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
		if n < 1 {
			PrintError("invalid index")
			return Result{Handled: true}
		}

		memoryPath := filepath.Join(ctx.RepoPath, ".iterate", "memory.json")
		data, err := os.ReadFile(memoryPath)
		if err != nil {
			PrintError("failed to read memory: %v", err)
			return Result{Handled: true}
		}

		var notes []map[string]string
		if err := json.Unmarshal(data, &notes); err != nil {
			PrintError("failed to parse memory: %v", err)
			return Result{Handled: true}
		}

		if n > len(notes) {
			PrintError("index out of range (have %d notes)", len(notes))
			return Result{Handled: true}
		}

		removed := notes[n-1]
		notes = append(notes[:n-1], notes[n:]...)

		newData, err := json.MarshalIndent(notes, "", "  ")
		if err != nil {
			PrintError("failed to marshal: %v", err)
			return Result{Handled: true}
		}
		if err := os.WriteFile(memoryPath, newData, 0644); err != nil {
			PrintError("failed to write: %v", err)
			return Result{Handled: true}
		}

		text := removed["note"]
		if len(text) > 60 {
			text = text[:60] + "…"
		}
		PrintSuccess("removed note %d: %s", n, text)
	}
	return Result{Handled: true}
}

func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
