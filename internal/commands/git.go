package commands

import (
	"fmt"
	"os/exec"
)

// RegisterGitCommands adds git commands.
func RegisterGitCommands(r *Registry) {
	r.Register(Command{
		Name:        "/diff",
		Aliases:     []string{},
		Description: "show git diff",
		Category:    "git",
		Handler:     cmdDiff,
	})

	r.Register(Command{
		Name:        "/status",
		Aliases:     []string{"/st"},
		Description: "show git status",
		Category:    "git",
		Handler:     cmdStatus,
	})

	r.Register(Command{
		Name:        "/log",
		Aliases:     []string{},
		Description: "show git log [n]",
		Category:    "git",
		Handler:     cmdLog,
	})

	r.Register(Command{
		Name:        "/branch",
		Aliases:     []string{"/br"},
		Description: "list or create branch",
		Category:    "git",
		Handler:     cmdBranch,
	})

	r.Register(Command{
		Name:        "/commit",
		Aliases:     []string{"/ci"},
		Description: "commit all changes",
		Category:    "git",
		Handler:     cmdCommit,
	})

	r.Register(Command{
		Name:        "/push",
		Aliases:     []string{},
		Description: "push to remote",
		Category:    "git",
		Handler:     cmdPush,
	})

	r.Register(Command{
		Name:        "/pull",
		Aliases:     []string{},
		Description: "pull from remote",
		Category:    "git",
		Handler:     cmdPull,
	})
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
