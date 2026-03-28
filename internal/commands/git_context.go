package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

// RegisterGitContextCommands adds git-aware context commands.
func RegisterGitContextCommands(r *Registry) {
	r.Register(Command{
		Name:        "/branch-info",
		Aliases:     []string{"/bi"},
		Description: "detailed branch state (ahead/behind/divergence)",
		Category:    "git",
		Handler:     cmdBranchInfo,
	})
}

func cmdBranchInfo(ctx Context) Result {
	repoPath := ctx.RepoPath

	branch := strings.TrimSpace(gitOutput(repoPath, "branch", "--show-current"))
	if branch == "" {
		fmt.Println("Not on a branch (detached HEAD)")
		return Result{Handled: true}
	}

	fmt.Printf("%s── Branch Info ────────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  %sBranch:%s %s\n", ColorBold, ColorReset, branch)

	remote := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.remote", branch)))
	mergeRef := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.merge", branch)))

	if remote != "" && mergeRef != "" {
		remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
		fmt.Printf("  %sRemote:%s %s/%s\n", ColorDim, ColorReset, remote, remoteBranch)

		ab := strings.TrimSpace(gitOutput(repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s/%s...HEAD", remote, remoteBranch)))
		parts := strings.Fields(ab)
		if len(parts) == 2 {
			behind, ahead := parts[0], parts[1]
			if ahead != "0" {
				fmt.Printf("  %s↑%s %s ahead  ", ColorGreen, ColorReset, ahead)
			}
			if behind != "0" {
				fmt.Printf("%s↓%s %s behind", ColorYellow, ColorReset, behind)
			}
			if ahead == "0" && behind == "0" {
				fmt.Printf("  %sUp to date%s", ColorLime, ColorReset)
			}
			fmt.Println()
		}
	} else {
		fmt.Printf("  %sRemote:%s none\n", ColorDim, ColorReset)
	}

	// Working tree
	status := gitOutput(repoPath, "status", "--short")
	statusLines := strings.Split(strings.TrimSpace(status), "\n")
	staged, unstaged, untracked := 0, 0, 0
	for _, line := range statusLines {
		if line == "" || len(line) < 2 {
			continue
		}
		if line[0] == '?' {
			untracked++
		} else {
			if line[0] != ' ' {
				staged++
			}
			if line[1] != ' ' {
				unstaged++
			}
		}
	}
	if staged+unstaged+untracked > 0 {
		fmt.Printf("  %sChanges:%s", ColorDim, ColorReset)
		if staged > 0 {
			fmt.Printf("  +%d staged", staged)
		}
		if unstaged > 0 {
			fmt.Printf("  ~%d unstaged", unstaged)
		}
		if untracked > 0 {
			fmt.Printf("  ?%d untracked", untracked)
		}
		fmt.Println()
	}

	// Recent commits
	fmt.Printf("  %sRecent:%s\n", ColorDim, ColorReset)
	log := gitOutput(repoPath, "log", "--oneline", "-3")
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if line != "" {
			fmt.Printf("    %s\n", line)
		}
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func gitOutput(repoPath string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}
