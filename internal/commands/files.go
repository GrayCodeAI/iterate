package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

// RegisterFileCommands adds file and search commands.
func RegisterFileCommands(r *Registry) {
	r.Register(Command{
		Name:        "/add",
		Aliases:     []string{},
		Description: "inject file into context",
		Category:    "files",
		Handler:     cmdAdd,
	})

	r.Register(Command{
		Name:        "/find",
		Aliases:     []string{},
		Description: "fuzzy file search",
		Category:    "files",
		Handler:     cmdFind,
	})

	r.Register(Command{
		Name:        "/web",
		Aliases:     []string{},
		Description: "fetch URL into context",
		Category:    "files",
		Handler:     cmdWeb,
	})

	r.Register(Command{
		Name:        "/grep",
		Aliases:     []string{},
		Description: "search code in repo",
		Category:    "files",
		Handler:     cmdGrep,
	})

	r.Register(Command{
		Name:        "/todos",
		Aliases:     []string{},
		Description: "list TODO/FIXME in codebase",
		Category:    "files",
		Handler:     cmdTodos,
	})

	r.Register(Command{
		Name:        "/deps",
		Aliases:     []string{},
		Description: "show go.mod dependencies",
		Category:    "files",
		Handler:     cmdDeps,
	})

	r.Register(Command{
		Name:        "/search",
		Aliases:     []string{},
		Description: "search web or code",
		Category:    "files",
		Handler:     cmdSearch,
	})

	r.Register(Command{
		Name:        "/pwd",
		Aliases:     []string{},
		Description: "show current directory",
		Category:    "files",
		Handler:     cmdPwd,
	})

	r.Register(Command{
		Name:        "/cd",
		Aliases:     []string{},
		Description: "change directory",
		Category:    "files",
		Handler:     cmdCd,
	})

	r.Register(Command{
		Name:        "/ls",
		Aliases:     []string{},
		Description: "list directory",
		Category:    "files",
		Handler:     cmdLs,
	})
}

func cmdAdd(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /add <filepath>")
		return Result{Handled: true}
	}
	filePath := ctx.Args()
	
	// Resolve path
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(ctx.RepoPath, filePath)
	}
	
	data, err := os.ReadFile(absPath)
	if err != nil {
		PrintError("failed to read file: %v", err)
		return Result{Handled: true}
	}
	
	// TODO: inject into agent context
	fmt.Printf("%s✓ read %s (%d bytes) — injecting into context%s\n\n", ColorLime, filePath, len(data), ColorReset)
	return Result{Handled: true}
}

func cmdFind(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /find <pattern>")
		return Result{Handled: true}
	}
	// TODO: wire up fuzzy file search
	fmt.Println("Find command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdWeb(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /web <url>")
		return Result{Handled: true}
	}
	url := ctx.Arg(1)
	fmt.Printf("%sfetching %s…%s\n", ColorDim, url, ColorReset)
	// TODO: wire up URL fetching
	fmt.Println("Web command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdGrep(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /grep <pattern>")
		return Result{Handled: true}
	}
	pattern := ctx.Args()
	fmt.Printf("%s── grep: %s ──%s\n", ColorDim, pattern, ColorReset)
	// TODO: wire up repo grep
	fmt.Println("Grep command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdTodos(ctx Context) Result {
	// TODO: wire up TODO/FIXME scanning
	fmt.Printf("%s── TODOs ──────────────────────────%s\n", ColorDim, ColorReset)
	fmt.Println("Todos command not yet wired in modular commands.")
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdDeps(ctx Context) Result {
	// TODO: wire up go.mod parsing
	fmt.Println("Deps command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdSearch(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /search <query>")
		return Result{Handled: true}
	}
	// TODO: wire up web/code search
	fmt.Println("Search command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdPwd(ctx Context) Result {
	fmt.Println(ctx.RepoPath)
	return Result{Handled: true}
}

func cmdCd(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println(ctx.RepoPath)
		return Result{Handled: true}
	}
	// Note: This doesn't actually change directory, just shows what would happen
	fmt.Printf("Note: /cd is informational only in modular commands.\nTarget: %s\n", ctx.Arg(1))
	return Result{Handled: true}
}

func cmdLs(ctx Context) Result {
	// TODO: wire up directory listing
	fmt.Println("Ls command not yet wired in modular commands.")
	return Result{Handled: true}
}
