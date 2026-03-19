package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// Unified /pr subcommand dispatcher
// ---------------------------------------------------------------------------

type prSubcommand int

const (
	prSubList     prSubcommand = iota
	prSubView
	prSubDiff
	prSubComment
	prSubCheckout
	prSubCreate
	prSubHelp
)

type parsedPR struct {
	sub    prSubcommand
	number string
	text   string
	draft  bool
}

func parsePRArgs(args string) parsedPR {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		return parsedPR{sub: prSubList}
	}
	switch parts[0] {
	case "list", "ls":
		return parsedPR{sub: prSubList}
	case "view":
		num := ""
		if len(parts) > 1 {
			num = parts[1]
		}
		return parsedPR{sub: prSubView, number: num}
	case "diff":
		num := ""
		if len(parts) > 1 {
			num = parts[1]
		}
		return parsedPR{sub: prSubDiff, number: num}
	case "comment":
		num, body := "", ""
		if len(parts) > 1 {
			num = parts[1]
		}
		if len(parts) > 2 {
			body = strings.Join(parts[2:], " ")
		}
		return parsedPR{sub: prSubComment, number: num, text: body}
	case "checkout", "co":
		num := ""
		if len(parts) > 1 {
			num = parts[1]
		}
		return parsedPR{sub: prSubCheckout, number: num}
	case "create", "new":
		draft := false
		for _, p := range parts[1:] {
			if p == "--draft" || p == "-d" {
				draft = true
			}
		}
		return parsedPR{sub: prSubCreate, draft: draft}
	default:
		// If first arg is a bare number, treat as "view <n>"
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return parsedPR{sub: prSubView, number: parts[0]}
		}
		return parsedPR{sub: prSubHelp}
	}
}

func handlePR(ctx context.Context, line string, a *iteragent.Agent, repoPath string) {
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "/pr"), " "))
	parsed := parsePRArgs(arg)

	switch parsed.sub {
	case prSubList:
		cmd := exec.Command("gh", "pr", "list", "--limit", "20")
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%serror: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Println()

	case prSubView:
		if parsed.number == "" {
			var ok bool
			parsed.number, ok = promptLine("PR number:")
			if !ok || parsed.number == "" {
				return
			}
		}
		cmd := exec.Command("gh", "pr", "view", parsed.number)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		fmt.Println()

	case prSubDiff:
		if parsed.number == "" {
			var ok bool
			parsed.number, ok = promptLine("PR number:")
			if !ok || parsed.number == "" {
				return
			}
		}
		out, err := exec.Command("gh", "pr", "diff", parsed.number).Output()
		if err != nil {
			fmt.Printf("%scould not fetch PR diff: %v%s\n", colorRed, err, colorReset)
			return
		}
		diff := string(out)
		if len(diff) > 8000 {
			diff = diff[:8000] + "\n…[truncated]"
		}
		fmt.Printf("%s── PR #%s diff ────────────────────%s\n%s%s──────────────────────────────────%s\n\n",
			colorDim, parsed.number, colorReset, diff, colorDim, colorReset)

	case prSubComment:
		if parsed.number == "" {
			var ok bool
			parsed.number, ok = promptLine("PR number:")
			if !ok || parsed.number == "" {
				return
			}
		}
		if parsed.text == "" {
			var ok bool
			parsed.text, ok = promptLine("Comment:")
			if !ok || parsed.text == "" {
				return
			}
		}
		cmd := exec.Command("gh", "pr", "comment", parsed.number, "--body", parsed.text)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%serror: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ comment posted on PR #%s%s\n\n", colorLime, parsed.number, colorReset)
		}

	case prSubCheckout:
		if parsed.number == "" {
			// List PRs and let user pick
			out, _ := exec.Command("gh", "pr", "list", "--json", "number,title,headRefName",
				"--template", `{{range .}}#{{.number}} {{.title}} ({{.headRefName}}){{"\n"}}{{end}}`).Output()
			prs := strings.Split(strings.TrimSpace(string(out)), "\n")
			var nonEmpty []string
			for _, pr := range prs {
				if strings.TrimSpace(pr) != "" {
					nonEmpty = append(nonEmpty, pr)
				}
			}
			if len(nonEmpty) == 0 {
				fmt.Println("No open PRs found.")
				return
			}
			choice, ok := selectItem("Select PR to checkout", nonEmpty)
			if !ok {
				return
			}
			// Extract number from "#123 ..."
			if len(choice) > 1 && choice[0] == '#' {
				end := strings.Index(choice[1:], " ")
				if end >= 0 {
					parsed.number = choice[1 : end+1]
				}
			}
			if parsed.number == "" {
				return
			}
		}
		cmd := exec.Command("gh", "pr", "checkout", parsed.number)
		cmd.Dir = repoPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%serror: %v%s\n", colorRed, err, colorReset)
		} else {
			fmt.Printf("%s✓ checked out PR #%s%s\n\n", colorLime, parsed.number, colorReset)
		}

	case prSubCreate:
		branchOut, _ := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
		branch := strings.TrimSpace(string(branchOut))
		if branch == "" || branch == "main" || branch == "master" {
			fmt.Printf("%screate a feature branch first. current: %s%s\n", colorRed, branch, colorReset)
			return
		}
		title, ok := promptLine("PR title:")
		if !ok || title == "" {
			fmt.Println("cancelled.")
			return
		}
		body, ok := promptLine("PR body (Enter for auto):")
		if !ok {
			return
		}
		if body == "" {
			body = fmt.Sprintf("Created by iterate from branch `%s`.", branch)
		}
		// Push branch first
		pushCmd := exec.Command("git", "-C", repoPath, "push", "-u", "origin", branch)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		pushCmd.Run()
		// Create PR
		args := []string{"pr", "create", "--title", title, "--body", body}
		if parsed.draft {
			args = append(args, "--draft")
		}
		prCmd := exec.Command("gh", args...)
		prCmd.Dir = repoPath
		prCmd.Stdout = os.Stdout
		prCmd.Stderr = os.Stderr
		prCmd.Run()
		fmt.Println()

	case prSubHelp:
		fmt.Printf(`%s/pr subcommands:%s
  /pr list              — list open PRs
  /pr view [N]          — view PR details
  /pr diff [N]          — show PR diff
  /pr comment N <text>  — post a comment
  /pr checkout [N]      — checkout PR branch
  /pr create [--draft]  — create a new PR
`, colorDim, colorReset)
	}
}

// ---------------------------------------------------------------------------
// Enhanced /diff with stat summary
// ---------------------------------------------------------------------------

func showGitDiffEnhanced(repoPath string) {
	// Get stat
	statOut, _ := exec.Command("git", "-C", repoPath, "diff", "--stat", "HEAD").Output()
	if strings.TrimSpace(string(statOut)) == "" {
		statOut, _ = exec.Command("git", "-C", repoPath, "diff", "--stat").Output()
	}
	// Get full colored diff
	diffOut, err := exec.Command("git", "-C", repoPath, "diff", "--color=always", "HEAD").Output()
	if err != nil || len(strings.TrimSpace(string(diffOut))) == 0 {
		diffOut, _ = exec.Command("git", "-C", repoPath, "diff", "--color=always").Output()
	}
	if len(strings.TrimSpace(string(diffOut))) == 0 {
		fmt.Printf("%s(no changes)%s\n\n", colorDim, colorReset)
		return
	}
	fmt.Printf("\n%s── diff ──────────────────────────%s\n", colorDim, colorReset)
	if stat := strings.TrimSpace(string(statOut)); stat != "" {
		fmt.Println(stat)
		fmt.Printf("%s──────────────────────────────────%s\n", colorDim, colorReset)
	}
	fmt.Print(string(diffOut))
	fmt.Printf("%s──────────────────────────────────%s\n\n", colorDim, colorReset)
}
