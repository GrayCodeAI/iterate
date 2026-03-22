package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

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
