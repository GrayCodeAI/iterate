package commands

import "fmt"

// RegisterContextTemplateCommands adds context analytics.
func RegisterContextTemplateCommands(r *Registry) {
	r.Register(Command{
		Name:        "/ctx-analytics",
		Aliases:     []string{},
		Description: "show context usage analytics",
		Category:    "context",
		Handler:     cmdCtxAnalytics,
	})
}

func cmdCtxAnalytics(ctx Context) Result {
	fmt.Printf("%s── Context Analytics ──────────────%s\n", ColorDim, ColorReset)

	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) > 0 {
		categories := categorizeLearnings(learnings)
		fmt.Printf("\n  %sLearning Categories:%s\n", ColorBold, ColorReset)
		for cat, count := range categories {
			fmt.Printf("    %-20s %d entries\n", cat, count)
		}
	}

	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}
