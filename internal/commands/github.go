package commands

import (
	"fmt"
	"os/exec"
	"strings"
)

// RegisterGitHubCommands adds GitHub-related commands.
func RegisterGitHubCommands(r *Registry) {
	registerGitHubPRCommands(r)
	registerGitHubUtilityCommands(r)
}

func registerGitHubPRCommands(r *Registry) {
	registerMany(r, "github",
		"/issues", "list open GitHub issues", cmdIssues,
		"/pr", "create pull request", cmdPR,
		"/pr-list", "list pull requests", cmdPRList,
		"/pr-review", "review current PR", cmdPRReview,
	)
}

func registerGitHubUtilityCommands(r *Registry) {
	registerMany(r, "github",
		"/pr-checkout", "checkout a PR", cmdPRCheckout,
		"/release", "create a release", cmdRelease,
		"/ci", "check CI status", cmdCI,
		"/gist", "create a gist", cmdGist,
	)
}

func cmdIssues(ctx Context) Result {
	limit := 10
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &limit)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		PrintError("gh CLI not installed — install from https://cli.github.com")
		return Result{Handled: true}
	}
	cmd := exec.Command("gh", "issue", "list", "--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,state,author",
		"--template", "{{range .}}#{{.number}}\t{{.state}}\t{{.title}}\t({{.author.login}})\n{{end}}")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Printf("%s── Open Issues ────────────────────%s\n", ColorDim, ColorReset)
	if err != nil {
		PrintError("gh issue list failed: %s", strings.TrimSpace(string(output)))
	} else if len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("  No open issues.")
	} else {
		fmt.Print(string(output))
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPR(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /pr <create|list|view|checkout|diff|review>")
		return Result{Handled: true}
	}
	sub := ctx.Arg(1)
	switch sub {
	case "create":
		return cmdPRCreate(ctx)
	case "list":
		return cmdPRListInline(ctx)
	case "view":
		return cmdPRView(ctx)
	case "checkout":
		return cmdPRCheckoutInline(ctx)
	case "diff":
		return cmdPRDiff(ctx)
	case "review":
		return cmdPRReviewInline(ctx)
	default:
		fmt.Printf("Unknown /pr subcommand: %s\n", sub)
		fmt.Println("Available: create, list, view, checkout, diff, review")
	}
	return Result{Handled: true}
}

func cmdPRCreate(ctx Context) Result {
	title := ""
	if ctx.HasArg(2) {
		title = strings.Join(ctx.Parts[2:], " ")
	}
	args := []string{"pr", "create", "--fill"}
	if title != "" {
		args = []string{"pr", "create", "--title", title, "--body", "Created via iterate"}
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("PR creation failed")
	}
	return Result{Handled: true}
}

func cmdPRListInline(ctx Context) Result {
	cmd := exec.Command("gh", "pr", "list", "--limit", "20")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("PR list failed")
	}
	return Result{Handled: true}
}

func cmdPRView(ctx Context) Result {
	prNum := ""
	if ctx.HasArg(2) {
		prNum = ctx.Arg(2)
	}
	args := []string{"pr", "view"}
	if prNum != "" {
		args = append(args, prNum)
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("PR view failed")
	}
	return Result{Handled: true}
}

func cmdPRCheckoutInline(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /pr checkout <number>")
		return Result{Handled: true}
	}
	cmd := exec.Command("gh", "pr", "checkout", ctx.Arg(2))
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("PR checkout failed")
	}
	return Result{Handled: true}
}

func cmdPRDiff(ctx Context) Result {
	prNum := ""
	if ctx.HasArg(2) {
		prNum = ctx.Arg(2)
	}
	args := []string{"pr", "diff"}
	if prNum != "" {
		args = append(args, prNum)
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("PR diff failed")
	}
	return Result{Handled: true}
}

func cmdPRReviewInline(ctx Context) Result {
	prNum := ""
	if ctx.HasArg(2) {
		prNum = ctx.Arg(2)
	}
	args := []string{"pr", "diff"}
	if prNum != "" {
		args = append(args, prNum)
	}
	diffCmd := exec.Command("gh", args...)
	diffCmd.Dir = ctx.RepoPath
	diffOut, err := diffCmd.CombinedOutput()
	if err != nil {
		PrintError("failed to get PR diff: %s", err)
		return Result{Handled: true}
	}
	prompt := fmt.Sprintf("Review this PR diff. Look for: bugs, security issues, "+
		"missing tests, and style problems. Be concise.\n\n```diff\n%s\n```",
		string(diffOut))
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdPRList(ctx Context) Result {
	cmd := exec.Command("gh", "pr", "list", "--limit", "20")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("gh pr list failed: %s", strings.TrimSpace(string(output)))
		return Result{Handled: true}
	}
	fmt.Printf("%s── Open PRs ───────────────────────%s\n", ColorDim, ColorReset)
	if len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("  No open PRs.")
	} else {
		fmt.Print(string(output))
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPRReview(ctx Context) Result {
	prNum := ""
	if ctx.HasArg(1) {
		prNum = ctx.Arg(1)
	}
	args := []string{"pr", "diff"}
	if prNum != "" {
		args = append(args, prNum)
	}
	diffCmd := exec.Command("gh", args...)
	diffCmd.Dir = ctx.RepoPath
	diffOut, err := diffCmd.CombinedOutput()
	if err != nil {
		PrintError("failed to get PR diff: %s", strings.TrimSpace(string(diffOut)))
		return Result{Handled: true}
	}
	prompt := fmt.Sprintf("Review this PR diff. Look for: bugs, security issues, "+
		"missing tests, and style problems. Be concise.\n\n```diff\n%s\n```", string(diffOut))
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdPRCheckout(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /pr-checkout <number>")
		return Result{Handled: true}
	}
	prNum := ctx.Arg(1)
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "gh", "pr", "checkout", prNum)
	} else {
		cmd := exec.Command("gh", "pr", "checkout", prNum)
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		if err := cmd.Run(); err != nil {
			PrintError("PR checkout failed: %s", err)
		}
	}
	return Result{Handled: true}
}

func cmdRelease(ctx Context) Result {
	tag := ""
	if ctx.HasArg(1) {
		tag = ctx.Arg(1)
	}
	args := []string{"release", "create"}
	if tag != "" {
		args = append(args, tag, "--generate-notes")
	} else {
		args = append(args, "--generate-notes")
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.TrimSpace(string(output)))
	if err != nil {
		PrintError("release creation failed")
	}
	return Result{Handled: true}
}

func cmdCI(ctx Context) Result {
	cmd := exec.Command("gh", "run", "list", "--limit", "10")
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("gh run list failed: %s", strings.TrimSpace(string(output)))
		return Result{Handled: true}
	}
	fmt.Printf("%s── CI Runs ────────────────────────%s\n", ColorDim, ColorReset)
	if len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("  No CI runs found.")
	} else {
		fmt.Print(string(output))
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdGist(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /gist <file>")
		return Result{Handled: true}
	}
	file := ctx.Args()
	cmd := exec.Command("gh", "gist", "create", file)
	cmd.Dir = ctx.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("gist creation failed: %s", strings.TrimSpace(string(output)))
	} else {
		fmt.Println(strings.TrimSpace(string(output)))
		PrintSuccess("gist created")
	}
	return Result{Handled: true}
}
