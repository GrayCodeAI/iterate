package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

func cmdPush(ctx Context) Result {
	cmd := exec.Command("git", "push")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	// Limit output for verbose push
	if len(output) > 500 {
		fmt.Println(string(output[:500]) + "...")
	} else {
		fmt.Println(string(output))
	}
	if err != nil {
		PrintError("Push failed: %s", err)
	} else {
		PrintSuccess("Pushed")
	}
	return Result{Handled: true}
}

func cmdPull(ctx Context) Result {
	cmd := exec.Command("git", "pull")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Pull failed: %s", err)
	} else {
		PrintSuccess("Pulled")
	}
	return Result{Handled: true}
}

func cmdFetch(ctx Context) Result {
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "fetch", "--all", "--prune")
	} else {
		cmd := exec.Command("git", "fetch", "--all", "--prune")
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdRebase(ctx Context) Result {
	target := "main"
	if ctx.HasArg(1) {
		target = ctx.Arg(1)
	}
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "rebase", target)
	} else {
		cmd := exec.Command("git", "rebase", target)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdCheckout(ctx Context) Result {
	// Get branches
	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("failed to list branches: %s", err)
		return Result{Handled: true}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" && !strings.Contains(line, "HEAD ->") {
			branches = append(branches, line)
		}
	}

	if len(branches) == 0 {
		fmt.Println("No branches found.")
		return Result{Handled: true}
	}

	var target string
	if ctx.HasArg(1) {
		target = ctx.Arg(1)
	} else if ctx.Session.SelectItem != nil {
		var ok bool
		target, ok = ctx.Session.SelectItem("Checkout branch", branches)
		if !ok {
			return Result{Handled: true}
		}
	} else {
		PrintError("no branch specified and interactive picker not available")
		return Result{Handled: true}
	}

	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "checkout", target)
	} else {
		cmd := exec.Command("git", "checkout", target)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdMerge(ctx Context) Result {
	// Get branches
	cmd := exec.Command("git", "branch")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("failed to list branches: %s", err)
		return Result{Handled: true}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}

	var target string
	if ctx.HasArg(1) {
		target = ctx.Arg(1)
	} else if ctx.Session.SelectItem != nil {
		var ok bool
		target, ok = ctx.Session.SelectItem("Merge branch into current", branches)
		if !ok {
			return Result{Handled: true}
		}
	} else {
		PrintError("no branch specified and interactive picker not available")
		return Result{Handled: true}
	}

	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "merge", target)
	} else {
		cmd := exec.Command("git", "merge", target)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}
