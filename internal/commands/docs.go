package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegisterDocsCommands adds documentation-related commands.
// Task 43: Documentation Summarizer for external dependencies
// Task 49: External Docs Integration (fetch library docs)
func RegisterDocsCommands(r *Registry) {
	r.Register(Command{
		Name:        "/deps-summary",
		Aliases:     []string{},
		Description: "summarize project dependencies",
		Category:    "context",
		Handler:     cmdDepsSummary,
	})
	r.Register(Command{
		Name:        "/pkgdoc",
		Aliases:     []string{"/pd"},
		Description: "look up Go package documentation",
		Category:    "context",
		Handler:     cmdPkgdoc,
	})
	r.Register(Command{
		Name:        "/docs-fetch",
		Aliases:     []string{"/df"},
		Description: "fetch external library docs (AI-assisted)",
		Category:    "context",
		Handler:     cmdDocsFetch,
	})
}

func cmdDepsSummary(ctx Context) Result {
	repoPath := ctx.RepoPath

	// Check for Go
	goMod := filepath.Join(repoPath, "go.mod")
	if data, err := os.ReadFile(goMod); err == nil {
		fmt.Printf("%s── Go Dependencies ────────────────%s\n", ColorDim, ColorReset)
		lines := strings.Split(string(data), "\n")
		inRequire := false
		depCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "require") {
				inRequire = true
				continue
			}
			if line == ")" {
				inRequire = false
				continue
			}
			if inRequire && line != "" && !strings.HasPrefix(line, "//") {
				fmt.Printf("  %s\n", line)
				depCount++
			}
		}
		fmt.Printf("\n  %s%d dependencies%s\n", ColorDim, depCount, ColorReset)
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	// Check for Node
	packageJSON := filepath.Join(repoPath, "package.json")
	if data, err := os.ReadFile(packageJSON); err == nil {
		fmt.Printf("%s── Node Dependencies ──────────────%s\n", ColorDim, ColorReset)
		content := string(data)
		if strings.Contains(content, "\"dependencies\"") {
			fmt.Println("  (dependencies found in package.json — use npm ls for full tree)")
		}
		if strings.Contains(content, "\"devDependencies\"") {
			fmt.Println("  (devDependencies found in package.json)")
		}
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	// Check for Python
	reqTxt := filepath.Join(repoPath, "requirements.txt")
	if data, err := os.ReadFile(reqTxt); err == nil {
		fmt.Printf("%s── Python Dependencies ────────────%s\n", ColorDim, ColorReset)
		lines := strings.Split(string(data), "\n")
		depCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				fmt.Printf("  %s\n", line)
				depCount++
			}
		}
		fmt.Printf("\n  %s%d dependencies%s\n", ColorDim, depCount, ColorReset)
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	fmt.Println("No recognized dependency file found (go.mod, package.json, requirements.txt)")
	return Result{Handled: true}
}

func cmdPkgdoc(ctx Context) Result {
	pkg := ctx.Args()
	if pkg == "" {
		fmt.Println("Usage: /pkgdoc <package-name>")
		fmt.Println("Example: /pkgdoc encoding/json")
		return Result{Handled: true}
	}

	// Use AI to look up and summarize the package
	prompt := fmt.Sprintf(
		"Look up the Go package %q on pkg.go.dev. Summarize:\n"+
			"1. What the package does (1-2 sentences)\n"+
			"2. Key types and functions\n"+
			"3. Common usage patterns\n"+
			"4. Any notable caveats or gotchas\n\n"+
			"Be concise. Focus on what a developer needs to know to use this package.", pkg)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdDocsFetch(ctx Context) Result {
	query := ctx.Args()
	if query == "" {
		fmt.Println("Usage: /docs-fetch <topic or library>")
		fmt.Println("Example: /docs-fetch gorilla/mux routing patterns")
		return Result{Handled: true}
	}

	prompt := fmt.Sprintf(
		"Research and explain: %s\n\n"+
			"Include:\n"+
			"1. What it is and what problem it solves\n"+
			"2. Key API / usage patterns with code examples\n"+
			"3. Best practices and common pitfalls\n"+
			"4. How it relates to the current project (check go.mod for context)\n\n"+
			"Be practical and code-focused.", query)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}
