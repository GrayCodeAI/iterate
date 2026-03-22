package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterFileCommands adds file and search commands.
func RegisterFileCommands(r *Registry) {
	registerFileContentCommands(r)
	registerFileSearchCommands(r)
	registerFileNavigationCommands(r)
}

func registerFileContentCommands(r *Registry) {
	r.Register(Command{
		Name:        "/add",
		Aliases:     []string{},
		Description: "inject file into context",
		Category:    "files",
		Handler:     cmdAdd,
	})

	r.Register(Command{
		Name:        "/web",
		Aliases:     []string{},
		Description: "fetch URL into context",
		Category:    "files",
		Handler:     cmdWeb,
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
}

func registerFileSearchCommands(r *Registry) {
	r.Register(Command{
		Name:        "/find",
		Aliases:     []string{},
		Description: "fuzzy file search",
		Category:    "files",
		Handler:     cmdFind,
	})

	r.Register(Command{
		Name:        "/grep",
		Aliases:     []string{},
		Description: "search code in repo",
		Category:    "files",
		Handler:     cmdGrep,
	})

	r.Register(Command{
		Name:        "/search",
		Aliases:     []string{},
		Description: "search web or code",
		Category:    "files",
		Handler:     cmdSearch,
	})

	r.Register(Command{
		Name:        "/search-replace",
		Aliases:     []string{},
		Description: "find and replace across repo",
		Category:    "files",
		Handler:     cmdSearchReplace,
	})
}

func registerFileNavigationCommands(r *Registry) {
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

	r.Register(Command{
		Name:        "/paste",
		Aliases:     []string{},
		Description: "paste from clipboard",
		Category:    "files",
		Handler:     cmdPaste,
	})

	r.Register(Command{
		Name:        "/open",
		Aliases:     []string{},
		Description: "open file in editor",
		Category:    "files",
		Handler:     cmdOpen,
	})
}

func cmdAdd(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /add <filepath>")
		return Result{Handled: true}
	}
	filePath := ctx.Args()

	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(ctx.RepoPath, filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		PrintError("failed to read file: %v", err)
		return Result{Handled: true}
	}

	if ctx.Agent != nil {
		content := fmt.Sprintf("Here is the content of the file `%s`:\n\n```\n%s\n```", filePath, string(data))
		ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.Message{
			Role:    "user",
			Content: content,
		})
	}

	fmt.Printf("%s✓ read %s (%d bytes) — injected into context%s\n\n", ColorLime, filePath, len(data), ColorReset)
	return Result{Handled: true}
}

func cmdFind(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /find <pattern>")
		return Result{Handled: true}
	}
	pattern := strings.ToLower(ctx.Args())

	var matches []string
	err := filepath.Walk(ctx.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(strings.ToLower(info.Name()), pattern) {
			rel, _ := filepath.Rel(ctx.RepoPath, path)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		PrintError("walk failed: %v", err)
		return Result{Handled: true}
	}

	sort.Strings(matches)
	if len(matches) == 0 {
		fmt.Println("No matches found.")
	} else {
		fmt.Printf("%s── %d matches ──%s\n", ColorDim, len(matches), ColorReset)
		for _, m := range matches {
			fmt.Printf("  %s\n", m)
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdWeb(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /web <url>")
		return Result{Handled: true}
	}
	url := ctx.Arg(1)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	fmt.Printf("%sfetching %s…%s\n", ColorDim, url, ColorReset)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		PrintError("fetch failed: %v", err)
		return Result{Handled: true}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		PrintError("HTTP %d", resp.StatusCode)
		return Result{Handled: true}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	if err != nil {
		PrintError("read failed: %v", err)
		return Result{Handled: true}
	}

	if ctx.Agent != nil {
		content := fmt.Sprintf("Here is the content fetched from `%s`:\n\n%s", url, string(body))
		ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.Message{
			Role:    "user",
			Content: content,
		})
		PrintSuccess("fetched %d bytes — injected into context", len(body))
	} else {
		fmt.Println(string(body))
	}
	return Result{Handled: true}
}

func cmdGrep(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /grep <pattern>")
		return Result{Handled: true}
	}
	pattern := ctx.Args()
	fmt.Printf("%s── grep: %s ──%s\n", ColorDim, pattern, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "50", pattern)
	} else {
		cmd = exec.Command("grep", "-rn", "--include=*", "-m", "50", pattern)
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No matches found.")
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdTodos(ctx Context) Result {
	fmt.Printf("%s── TODOs ──────────────────────────%s\n", ColorDim, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "100", "(TODO|FIXME|HACK|XXX)")
	} else {
		cmd = exec.Command("grep", "-rn", "-E", "-m", "100", "(TODO|FIXME|HACK|XXX)")
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No TODOs found.")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdDeps(ctx Context) Result {
	goModPath := filepath.Join(ctx.RepoPath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		PrintError("failed to read go.mod: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("%s── Dependencies ───────────────────%s\n", ColorDim, ColorReset)
	lines := strings.Split(string(data), "\n")
	inRequire := false
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "require") {
			if strings.Contains(trimmed, "(") {
				inRequire = true
			} else {
				dep := strings.TrimPrefix(trimmed, "require ")
				if dep != trimmed && dep != "" {
					fmt.Printf("  %s\n", dep)
					count++
				}
			}
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		if inRequire && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			fmt.Printf("  %s\n", trimmed)
			count++
		}
	}
	if count == 0 {
		fmt.Println("  No dependencies found in go.mod")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdSearch(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /search <query>")
		return Result{Handled: true}
	}
	query := ctx.Args()
	fmt.Printf("%s── search: %s ──%s\n", ColorDim, query, ColorReset)

	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.Command("rg", "--no-heading", "-n", "-S", "--max-count", "50", "-i", query)
	} else {
		cmd = exec.Command("grep", "-rn", "--include=*", "-i", "-m", "50", query)
	}
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil && len(output) == 0 {
		fmt.Println("No matches found.")
	}
	fmt.Println()
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
	target := ctx.Arg(1)
	resolved := target
	if !filepath.IsAbs(target) {
		resolved = filepath.Join(ctx.RepoPath, target)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		PrintError("path not found: %s", target)
		return Result{Handled: true}
	}
	if !info.IsDir() {
		PrintError("not a directory: %s", target)
		return Result{Handled: true}
	}
	fmt.Printf("Note: /cd is informational only.\nResolved: %s\n", resolved)
	return Result{Handled: true}
}

func cmdLs(ctx Context) Result {
	dir := ctx.RepoPath
	if ctx.HasArg(1) {
		target := ctx.Arg(1)
		if !filepath.IsAbs(target) {
			dir = filepath.Join(ctx.RepoPath, target)
		} else {
			dir = target
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		PrintError("failed to read directory: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("%s── %s ──%s\n", ColorDim, dir, ColorReset)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			fmt.Printf("  %s%s/%s\n", ColorCyan, name, ColorReset)
		} else {
			info, _ := entry.Info()
			size := ""
			if info != nil {
				size = fmt.Sprintf(" (%d bytes)", info.Size())
			}
			fmt.Printf("  %s%s\n", name, size)
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdSearchReplace(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /search-replace <old> <new>")
		return Result{Handled: true}
	}

	oldText := ctx.Arg(1)
	newText := ctx.Arg(2)
	fmt.Printf("%sReplace all occurrences of %q with %q? (y/N): %s", ColorYellow, oldText, newText, ColorReset)

	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		fmt.Println("Cancelled.")
		return Result{Handled: true}
	}

	// Use sed via shell for actual replacement
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "bash", "-c",
			fmt.Sprintf("find . -type f -name '*.go' -exec sed -i '' 's/%s/%s/g' {} +",
				oldText, newText))
		PrintSuccess("replaced occurrences of %q with %q", oldText, newText)
	} else {
		PrintError("shell execution not available")
	}
	return Result{Handled: true}
}

func cmdPaste(ctx Context) Result {
	var text string
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		out, err = cmd.Output()
	}
	if err != nil {
		PrintError("clipboard not available: %s", err)
		return Result{Handled: true}
	}
	text = string(out)
	if strings.TrimSpace(text) == "" {
		fmt.Println("Clipboard is empty.")
		return Result{Handled: true}
	}
	fmt.Printf("%s✓ pasting %d chars from clipboard%s\n\n", ColorLime, len(text), ColorReset)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, text, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdOpen(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /open <file>")
		return Result{Handled: true}
	}
	filePath := strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(ctx.RepoPath, filePath)
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, absPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError("%s", err)
	}
	return Result{Handled: true}
}
