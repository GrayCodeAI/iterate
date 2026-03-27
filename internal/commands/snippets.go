package commands

import (
	"fmt"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/suggest"
)

// RegisterSnippetCommands adds code snippet commands
func RegisterSnippetCommands(r *Registry) {
	r.Register(Command{
		Name:        "/snippet",
		Aliases:     []string{"/snip"},
		Description: "insert code snippet (func, for, iferr, etc.)",
		Category:    "productivity",
		Handler:     cmdSnippet,
	})

	r.Register(Command{
		Name:        "/complete",
		Description: "show completions for current word",
		Category:    "productivity",
		Handler:     cmdComplete,
	})
}

func cmdSnippet(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("📦 Available snippets:")
		fmt.Println()
		for name, snippet := range suggest.CommonGoSnippets {
			fmt.Printf("  %-10s - %s\n", name, snippet.Description)
		}
		fmt.Println()
		fmt.Println("Usage: /snippet <name>")
		fmt.Println("Example: /snippet func")
		return Result{Handled: true}
	}

	name := ctx.Arg(1)
	if snippet, ok := suggest.GetSnippet(name); ok {
		// Copy to clipboard or output
		fmt.Println()
		fmt.Println("📋 Snippet:")
		fmt.Println("```go")
		fmt.Println(snippet.Text)
		fmt.Println("```")
		fmt.Println()
		fmt.Println("💡 Tip: Use /copy to copy to clipboard")
	} else {
		PrintError("Unknown snippet: %s", name)
		fmt.Println("Use /snippet to see available snippets")
	}

	return Result{Handled: true}
}

func cmdComplete(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /complete <partial-word>")
		return Result{Handled: true}
	}

	line := ctx.Arg(1)
	for i := 2; ctx.HasArg(i); i++ {
		line += " " + ctx.Arg(i)
	}

	word, _ := suggest.CompleteWord(line, len(line))

	// Show completions for common patterns
	var completions []string

	prefix := strings.ToLower(word)
	for name := range suggest.CommonGoSnippets {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			completions = append(completions, name)
		}
	}

	// Add file completions if it looks like a path
	if strings.Contains(line, "/") || strings.HasSuffix(line, ".") {
		completions = append(completions, "@filename", "@filepath")
	}

	if len(completions) > 0 {
		fmt.Printf("\n💡 Completions for '%s':\n", word)
		for _, c := range completions {
			fmt.Printf("  • %s\n", c)
		}
		fmt.Println()
	} else {
		fmt.Printf("No completions found for '%s'\n", word)
	}

	return Result{Handled: true}
}
