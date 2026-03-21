package commands

import (
	"fmt"
)

// RegisterGitHubCommands adds GitHub-related commands.
func RegisterGitHubCommands(r *Registry) {
	r.Register(Command{
		Name:        "/issues",
		Aliases:     []string{},
		Description: "list open GitHub issues",
		Category:    "github",
		Handler:     cmdIssues,
	})

	r.Register(Command{
		Name:        "/pr",
		Aliases:     []string{},
		Description: "create pull request",
		Category:    "github",
		Handler:     cmdPR,
	})

	r.Register(Command{
		Name:        "/pr-list",
		Aliases:     []string{},
		Description: "list pull requests",
		Category:    "github",
		Handler:     cmdPRList,
	})

	r.Register(Command{
		Name:        "/pr-review",
		Aliases:     []string{},
		Description: "review current PR",
		Category:    "github",
		Handler:     cmdPRReview,
	})

	r.Register(Command{
		Name:        "/pr-checkout",
		Aliases:     []string{},
		Description: "checkout a PR",
		Category:    "github",
		Handler:     cmdPRCheckout,
	})

	r.Register(Command{
		Name:        "/release",
		Aliases:     []string{},
		Description: "create a release",
		Category:    "github",
		Handler:     cmdRelease,
	})

	r.Register(Command{
		Name:        "/ci",
		Aliases:     []string{},
		Description: "check CI status",
		Category:    "github",
		Handler:     cmdCI,
	})

	r.Register(Command{
		Name:        "/gist",
		Aliases:     []string{},
		Description: "create a gist",
		Category:    "github",
		Handler:     cmdGist,
	})
}

func cmdIssues(ctx Context) Result {
	limit := 10
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &limit)
	}
	// TODO: wire up listGitHubIssues
	fmt.Printf("%s── Open Issues ────────────────────%s\n", ColorDim, ColorReset)
	fmt.Println("Issues command not yet wired in modular commands.")
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPR(ctx Context) Result {
	// TODO: wire up handlePR
	fmt.Println("PR command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdPRList(ctx Context) Result {
	// TODO: wire up PR listing via gh CLI
	fmt.Println("PR-list command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdPRReview(ctx Context) Result {
	// TODO: wire up PR review
	fmt.Println("PR-review command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdPRCheckout(ctx Context) Result {
	// TODO: wire up PR checkout via gh CLI
	fmt.Println("PR-checkout command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdRelease(ctx Context) Result {
	// TODO: wire up release creation via gh CLI
	fmt.Println("Release command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdCI(ctx Context) Result {
	// TODO: wire up CI status check via gh CLI
	fmt.Println("CI command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdGist(ctx Context) Result {
	// TODO: wire up gist creation via gh CLI
	fmt.Println("Gist command not yet wired in modular commands.")
	return Result{Handled: true}
}
