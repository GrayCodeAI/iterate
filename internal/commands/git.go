package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

// RegisterGitCommands adds git commands.
func RegisterGitCommands(r *Registry) {
	registerGitBasicCommands(r)
	registerGitNetworkCommands(r)
	registerGitAdvancedCommands(r)
}

func registerGitBasicCommands(r *Registry) {
	r.Register(Command{Name: "/diff", Description: "show git diff", Category: "git", Handler: cmdDiff})
	r.Register(Command{Name: "/status", Aliases: []string{"/st"}, Description: "show git status", Category: "git", Handler: cmdStatus})
	r.Register(Command{Name: "/log", Description: "show git log [n]", Category: "git", Handler: cmdLog})
	r.Register(Command{Name: "/branch", Aliases: []string{"/br"}, Description: "list or create branch", Category: "git", Handler: cmdBranch})
	r.Register(Command{Name: "/commit", Description: "commit all changes", Category: "git", Handler: cmdCommit})
	r.Register(Command{Name: "/diff-staged", Description: "show staged diff", Category: "git", Handler: cmdDiffStaged})
	r.Register(Command{Name: "/undo", Description: "undo last commit (git reset HEAD~1)", Category: "git", Handler: cmdUndo})
	r.Register(Command{Name: "/git", Description: "git passthrough command", Category: "git", Handler: cmdGit})
	r.Register(Command{Name: "/generate-commit", Description: "AI-generated commit message", Category: "git", Handler: cmdGenerateCommit})
	r.Register(Command{Name: "/auto-commit", Description: "toggle auto-commit", Category: "git", Handler: cmdAutoCommit})
	r.Register(Command{Name: "/revert-file", Description: "revert file to HEAD", Category: "git", Handler: cmdRevertFile})
	r.Register(Command{Name: "/blame", Description: "git blame on file", Category: "git", Handler: cmdBlame})
	r.Register(Command{Name: "/cherry-pick", Description: "git cherry-pick commit", Category: "git", Handler: cmdCherryPick})
	r.Register(Command{Name: "/checkout", Description: "checkout branch (interactive picker)", Category: "git", Handler: cmdCheckout})
	r.Register(Command{Name: "/merge", Description: "merge branch into current", Category: "git", Handler: cmdMerge})
}

func registerGitNetworkCommands(r *Registry) {
	r.Register(Command{Name: "/push", Description: "push to remote", Category: "git", Handler: cmdPush})
	r.Register(Command{Name: "/pull", Description: "pull from remote", Category: "git", Handler: cmdPull})
	r.Register(Command{Name: "/fetch", Description: "git fetch --all --prune", Category: "git", Handler: cmdFetch})
	r.Register(Command{Name: "/rebase", Description: "git rebase onto branch", Category: "git", Handler: cmdRebase})
}

func registerGitAdvancedCommands(r *Registry) {
	r.Register(Command{Name: "/amend", Description: "amend last commit", Category: "git", Handler: cmdAmend})
	r.Register(Command{Name: "/squash", Description: "squash last N commits", Category: "git", Handler: cmdSquash})
	r.Register(Command{Name: "/stash", Description: "git stash / stash pop", Category: "git", Handler: cmdStash})
	r.Register(Command{Name: "/stash-list", Description: "list stash entries", Category: "git", Handler: cmdStashList})
	r.Register(Command{Name: "/clean", Description: "clean untracked files", Category: "git", Handler: cmdClean})
	r.Register(Command{Name: "/tag", Description: "list or create git tag", Category: "git", Handler: cmdTag})
}

func cmdDiff(ctx Context) Result {
	viewer := NewDiffViewer()

	// Get list of changed files first
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("Git diff failed: %s", err)
		return Result{Handled: true}
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 0 || (len(files) == 1 && files[0] == "") {
		fmt.Println("✨ No changes to show")
		return Result{Handled: true}
	}

	// Show colored diff for each file
	for _, file := range files {
		if file == "" {
			continue
		}
		if err := viewer.ShowGitDiff(file); err != nil {
			PrintError("Could not show diff for %s: %v", file, err)
		}
	}

	return Result{Handled: true}
}

func cmdStatus(ctx Context) Result {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Git status failed: %s", err)
	}
	return Result{Handled: true}
}

func cmdLog(ctx Context) Result {
	n := "15"
	if ctx.HasArg(1) {
		n = ctx.Arg(1)
	}
	cmd := exec.Command("git", "--no-pager", "log", "--oneline", "-n"+n)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Git log failed: %s", err)
	}
	return Result{Handled: true}
}

func cmdBranch(ctx Context) Result {
	if ctx.HasArg(1) {
		// Create new branch
		name := ctx.Arg(1)
		cmd := exec.Command("git", "checkout", "-b", name)
		cmd.Dir = ctx.RepoPath
		output, err := cmd.CombinedOutput()
		fmt.Println(string(output))
		if err != nil {
			PrintError("Branch create failed: %s", err)
		} else {
			PrintSuccess("Created branch %s", name)
		}
	} else {
		// List branches
		cmd := exec.Command("git", "branch", "-a")
		cmd.Dir = ctx.RepoPath
		output, err := cmd.CombinedOutput()
		fmt.Println(string(output))
		if err != nil {
			PrintError("Git branch failed: %s", err)
		}
	}
	return Result{Handled: true}
}

func cmdCommit(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /commit <message>")
		return Result{Handled: true}
	}
	msg := ctx.Args()

	// git add -A
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = ctx.RepoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		PrintError("Git add failed: %s\n%s", err, string(output))
		return Result{Handled: true}
	}

	// git commit
	commitCmd := exec.Command("git", "commit", "-m", msg)
	commitCmd.Dir = ctx.RepoPath
	output, err := commitCmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Commit failed: %s", err)
	} else {
		PrintSuccess("Committed: %s", msg)
	}
	return Result{Handled: true}
}

func cmdRevertFile(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /revert-file <filepath>")
		return Result{Handled: true}
	}
	file := ctx.Arg(1)
	fmt.Printf("%sRevert %s to HEAD? (y/N): %s", ColorYellow, file, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "checkout", "HEAD", "--", file)
		} else {
			cmd := exec.Command("git", "checkout", "HEAD", "--", file)
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			cmd.Run()
		}
	} else {
		fmt.Println("Cancelled.")
	}
	return Result{Handled: true}
}

func cmdBlame(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /blame <file>")
		return Result{Handled: true}
	}
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "blame", "--color-lines", ctx.Arg(1))
	} else {
		cmd := exec.Command("git", "blame", "--color-lines", ctx.Arg(1))
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdCherryPick(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /cherry-pick <commit-hash>")
		return Result{Handled: true}
	}
	commit := ctx.Arg(1)
	// Validate commit hash doesn't start with '-' to prevent flag injection
	if len(commit) > 0 && commit[0] == '-' {
		fmt.Println("Error: invalid commit hash (cannot start with '-')")
		return Result{Handled: true}
	}
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "cherry-pick", commit)
	} else {
		cmd := exec.Command("git", "cherry-pick", commit)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdDiffStaged(ctx Context) Result {
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "diff", "--cached", "--color")
	} else {
		cmd := exec.Command("git", "diff", "--cached", "--color")
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdUndo(ctx Context) Result {
	fmt.Printf("%sUndo last commit? (y/N): %s", ColorYellow, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "reset", "HEAD~1")
		} else {
			cmd := exec.Command("git", "reset", "HEAD~1")
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			cmd.Run()
		}
		PrintSuccess("undone")
	} else {
		fmt.Println("Cancelled.")
	}
	return Result{Handled: true}
}

func cmdAutoCommit(ctx Context) Result {
	if ctx.AutoCommitEnabled == nil {
		PrintError("auto-commit state not available")
		return Result{Handled: true}
	}
	*ctx.AutoCommitEnabled = !*ctx.AutoCommitEnabled
	state := "disabled"
	if *ctx.AutoCommitEnabled {
		state = "enabled"
	}
	PrintSuccess("auto-commit %s", state)
	return Result{Handled: true}
}

func cmdGit(ctx Context) Result {
	if !ctx.HasArg(1) {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "status")
		} else {
			cmd := exec.Command("git", "status")
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			cmd.Run()
		}
		return Result{Handled: true}
	}
	gitArgs := ctx.Parts[1:]
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", gitArgs...)
	} else {
		cmd := exec.Command("git", gitArgs...)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	return Result{Handled: true}
}

func cmdGenerateCommit(ctx Context) Result {
	prompt := "Analyze the current git diff (run 'git diff --staged') and generate a concise, " +
		"descriptive commit message following conventional commit format. " +
		"Output only the commit message, nothing else."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}
