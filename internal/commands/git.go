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
	cmd := exec.Command("git", "diff")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		PrintError("Git diff failed: %s", err)
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
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "cherry-pick", ctx.Arg(1))
	} else {
		cmd := exec.Command("git", "cherry-pick", ctx.Arg(1))
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

func cmdStash(ctx Context) Result {
	pop := ctx.HasArg(1) && ctx.Arg(1) == "pop"
	action := "stash"
	if pop {
		action = "stash pop"
	}

	if ctx.REPL.RunShell != nil {
		if pop {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "stash", "pop")
		} else {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "stash")
		}
	} else {
		var cmd *exec.Cmd
		if pop {
			cmd = exec.Command("git", "stash", "pop")
		} else {
			cmd = exec.Command("git", "stash")
		}
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		if err := cmd.Run(); err != nil {
			PrintError("git %s failed: %s", action, err)
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdTag(ctx Context) Result {
	if !ctx.HasArg(1) {
		// List tags
		cmd := exec.Command("git", "tag", "--sort=-creatordate")
		cmd.Dir = ctx.RepoPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			PrintError("git tag failed: %s", err)
			return Result{Handled: true}
		}
		tags := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(tags) == 0 || (len(tags) == 1 && tags[0] == "") {
			fmt.Println("No tags.")
		} else {
			fmt.Printf("%s── Tags ───────────────────────────%s\n", ColorDim, ColorReset)
			for _, t := range tags {
				if t != "" {
					fmt.Printf("  %s\n", t)
				}
			}
			fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		}
		return Result{Handled: true}
	}

	// Create tag
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "git", "tag", ctx.Arg(1))
	} else {
		cmd := exec.Command("git", "tag", ctx.Arg(1))
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
	}
	PrintSuccess("tag %s created", ctx.Arg(1))
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

func cmdAmend(ctx Context) Result {
	msg := ""
	if ctx.HasArg(1) {
		msg = strings.TrimSpace(strings.TrimPrefix(ctx.Line, ctx.Parts[0]))
	}
	if msg != "" {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "commit", "--amend", "-m", msg)
		} else {
			cmd := exec.Command("git", "commit", "--amend", "-m", msg)
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			if err := cmd.Run(); err != nil {
				PrintError("%s", err)
			}
		}
	} else {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "commit", "--amend", "--no-edit")
		} else {
			cmd := exec.Command("git", "commit", "--amend", "--no-edit")
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			if err := cmd.Run(); err != nil {
				PrintError("%s", err)
			}
		}
	}
	fmt.Println()
	return Result{Handled: true}
}

func cmdSquash(ctx Context) Result {
	n := 2
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
	}
	msg := fmt.Sprintf("squash: combine last %d commits", n)
	if ctx.HasArg(2) {
		msg = strings.Join(ctx.Parts[2:], " ")
	}
	fmt.Printf("%sSquash last %d commits into %q? (y/N): %s", ColorYellow, n, msg, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		resetTarget := fmt.Sprintf("HEAD~%d", n-1)
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "reset", "--soft", resetTarget)
			ctx.REPL.RunShell(ctx.RepoPath, "git", "commit", "--amend", "-m", msg)
		} else {
			cmd := exec.Command("git", "reset", "--soft", resetTarget)
			cmd.Dir = ctx.RepoPath
			cmd.Stdout = Stdout
			cmd.Stderr = Stdout
			cmd.Run()
			cmd2 := exec.Command("git", "commit", "--amend", "-m", msg)
			cmd2.Dir = ctx.RepoPath
			cmd2.Stdout = Stdout
			cmd2.Stderr = Stdout
			cmd2.Run()
		}
	} else {
		fmt.Println("Cancelled.")
	}
	fmt.Println()
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

func cmdStashList(ctx Context) Result {
	cmd := exec.Command("git", "-C", ctx.RepoPath, "stash", "list")
	output, err := cmd.Output()
	list := strings.TrimSpace(string(output))
	if err != nil || list == "" {
		fmt.Println("Stash is empty.")
	} else {
		fmt.Printf("%s── Stash ──────────────────────────%s\n%s\n%s──────────────────────────────────%s\n\n",
			ColorDim, ColorReset, list, ColorDim, ColorReset)
	}
	return Result{Handled: true}
}

func cmdClean(ctx Context) Result {
	out, _ := exec.Command("git", "-C", ctx.RepoPath, "clean", "-nd").Output()
	if strings.TrimSpace(string(out)) == "" {
		fmt.Println("Nothing to clean.")
		return Result{Handled: true}
	}
	fmt.Print(string(out))
	fmt.Printf("%sRemove these files? (y/N): %s", ColorYellow, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		if ctx.REPL.RunShell != nil {
			ctx.REPL.RunShell(ctx.RepoPath, "git", "clean", "-fd")
		} else {
			cmd := exec.Command("git", "clean", "-fd")
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
