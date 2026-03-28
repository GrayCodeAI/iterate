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
	r.Register(Command{
		Name:        "/git-context",
		Aliases:     []string{"/gc"},
		Description: "inject git state into agent context",
		Category:    "git",
		Handler:     cmdGitContext,
	})
	r.Register(Command{
		Name:        "/ahead-behind",
		Aliases:     []string{"/ab"},
		Description: "show ahead/behind counts vs remote",
		Category:    "git",
		Handler:     cmdAheadBehind,
	})
}

func cmdBranchInfo(ctx Context) Result {
	repoPath := ctx.RepoPath

	// Current branch
	branch := gitOutput(repoPath, "branch", "--show-current")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		fmt.Println("Not on a branch (detached HEAD)")
		return Result{Handled: true}
	}

	fmt.Printf("%s── Branch Info ────────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  %sBranch:%s %s\n", ColorBold, ColorReset, branch)

	// Remote tracking
	remote := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.remote", branch)))
	mergeRef := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.merge", branch)))

	if remote != "" && mergeRef != "" {
		fmt.Printf("  %sRemote:%s %s → %s\n", ColorDim, ColorReset, remote, mergeRef)

		// Ahead/behind counts
		remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
		ab := gitOutput(repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s/%s...HEAD", remote, remoteBranch))
		ab = strings.TrimSpace(ab)
		parts := strings.Fields(ab)
		if len(parts) == 2 {
			behind := parts[0]
			ahead := parts[1]
			fmt.Printf("  %sAhead:%s %s  %sBehind:%s %s\n", ColorGreen, ColorReset, ahead, ColorYellow, ColorReset, behind)
			if behind != "0" {
				fmt.Printf("  %s⚠ Your branch is %s commits behind %s/%s%s\n", ColorYellow, behind, remote, remoteBranch, ColorReset)
			}
			if ahead != "0" {
				fmt.Printf("  %s↑ Your branch is %s commits ahead of %s/%s%s\n", ColorGreen, ahead, remote, remoteBranch, ColorReset)
			}
		}

		// Divergence indicator
		lastMerge := gitOutput(repoPath, "log", "--oneline", "-1", fmt.Sprintf("merge-base HEAD %s/%s", remote, remoteBranch))
		lastMerge = strings.TrimSpace(lastMerge)
		if lastMerge != "" {
			fmt.Printf("  %sMerge base:%s %s\n", ColorDim, ColorReset, lastMerge)
		}
	} else {
		fmt.Printf("  %sRemote:%s none (local-only branch)\n", ColorDim, ColorReset)
	}

	// Uncommitted changes
	status := gitOutput(repoPath, "status", "--short")
	statusLines := strings.Split(strings.TrimSpace(status), "\n")
	staged := 0
	unstaged := 0
	untracked := 0
	for _, line := range statusLines {
		if line == "" {
			continue
		}
		if len(line) >= 2 {
			if line[0] != ' ' && line[0] != '?' {
				staged++
			}
			if line[1] != ' ' {
				unstaged++
			}
			if line[0] == '?' {
				untracked++
			}
		}
	}

	if staged+unstaged+untracked > 0 {
		fmt.Println()
		fmt.Printf("  %sWorking tree:%s", ColorDim, ColorReset)
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

	// Last 3 commits
	fmt.Println()
	fmt.Printf("  %sRecent commits:%s\n", ColorDim, ColorReset)
	log := gitOutput(repoPath, "log", "--oneline", "-3")
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if line != "" {
			fmt.Printf("    %s\n", line)
		}
	}

	// Stash count
	stashList := gitOutput(repoPath, "stash", "list")
	stashLines := strings.Split(strings.TrimSpace(stashList), "\n")
	stashCount := 0
	for _, l := range stashLines {
		if l != "" {
			stashCount++
		}
	}
	if stashCount > 0 {
		fmt.Printf("\n  %sStash:%s %d entries\n", ColorDim, ColorReset, stashCount)
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdGitContext(ctx Context) Result {
	repoPath := ctx.RepoPath

	// Build a structured git context string for the agent
	var sb strings.Builder

	branch := strings.TrimSpace(gitOutput(repoPath, "branch", "--show-current"))
	sb.WriteString(fmt.Sprintf("Branch: %s\n", branch))

	// Remote tracking
	remote := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.remote", branch)))
	if remote != "" {
		sb.WriteString(fmt.Sprintf("Remote: %s\n", remote))
		remoteBranch := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.merge", branch)))
		remoteBranch = strings.TrimPrefix(remoteBranch, "refs/heads/")
		ab := strings.TrimSpace(gitOutput(repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s/%s...HEAD", remote, remoteBranch)))
		parts := strings.Fields(ab)
		if len(parts) == 2 {
			sb.WriteString(fmt.Sprintf("Ahead: %s, Behind: %s\n", parts[1], parts[0]))
		}
	}

	// Changes summary
	status := gitOutput(repoPath, "diff", "--stat")
	if status != "" {
		sb.WriteString(fmt.Sprintf("Uncommitted changes:\n%s\n", status))
	}

	staged := gitOutput(repoPath, "diff", "--cached", "--stat")
	if staged != "" {
		sb.WriteString(fmt.Sprintf("Staged changes:\n%s\n", staged))
	}

	// Last 5 commits
	log := gitOutput(repoPath, "log", "--oneline", "-5")
	sb.WriteString(fmt.Sprintf("Recent commits:\n%s\n", log))

	// Tags
	tags := gitOutput(repoPath, "tag", "--sort=-creatordate", "-5")
	if tags != "" {
		sb.WriteString(fmt.Sprintf("Recent tags:\n%s\n", tags))
	}

	contextStr := sb.String()
	fmt.Printf("%s── Git Context ────────────────────%s\n", ColorDim, ColorReset)
	fmt.Print(contextStr)
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)

	fmt.Printf("%sGit context has been captured. Include this in your next prompt for better git-aware responses.%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdAheadBehind(ctx Context) Result {
	repoPath := ctx.RepoPath

	branch := strings.TrimSpace(gitOutput(repoPath, "branch", "--show-current"))
	if branch == "" {
		fmt.Println("Not on a branch (detached HEAD)")
		return Result{Handled: true}
	}

	remote := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.remote", branch)))
	if remote == "" {
		fmt.Printf("Branch %s has no remote tracking.\n", branch)
		return Result{Handled: true}
	}

	mergeRef := strings.TrimSpace(gitOutput(repoPath, "config", "--get", fmt.Sprintf("branch.%s.merge", branch)))
	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")

	ab := strings.TrimSpace(gitOutput(repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s/%s...HEAD", remote, remoteBranch)))
	parts := strings.Fields(ab)

	fmt.Printf("%s── Ahead / Behind ─────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  %sBranch:%s %s → %s/%s\n", ColorBold, ColorReset, branch, remote, remoteBranch)
	if len(parts) == 2 {
		fmt.Printf("  %s↑ Ahead: %s%s\n", ColorGreen, parts[1], ColorReset)
		fmt.Printf("  %s↓ Behind: %s%s\n", ColorYellow, parts[0], ColorReset)
	} else {
		fmt.Println("  Unable to determine ahead/behind counts")
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
