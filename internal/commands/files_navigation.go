package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
