package commands

import (
	"fmt"
)

// RegisterAnalysisCommands adds repository analysis commands.
func RegisterAnalysisCommands(r *Registry) {
	r.Register(Command{
		Name:        "/count-lines",
		Aliases:     []string{},
		Description: "count lines of code by language",
		Category:    "analysis",
		Handler:     cmdCountLines,
	})

	r.Register(Command{
		Name:        "/hotspots",
		Aliases:     []string{},
		Description: "most changed files in git",
		Category:    "analysis",
		Handler:     cmdHotspots,
	})

	r.Register(Command{
		Name:        "/contributors",
		Aliases:     []string{},
		Description: "show git contributors",
		Category:    "analysis",
		Handler:     cmdContributors,
	})

	r.Register(Command{
		Name:        "/languages",
		Aliases:     []string{},
		Description: "language breakdown",
		Category:    "analysis",
		Handler:     cmdLanguages,
	})
}

func cmdCountLines(ctx Context) Result {
	fmt.Printf("%sCounting lines…%s\n", ColorDim, ColorReset)
	prompt := "Run a line count analysis on this repository. For each programming language found, " +
		"count the number of files and lines of code. Present results in a table format " +
		"with columns: Language, Files, Lines. Include a total row at the bottom."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdHotspots(ctx Context) Result {
	n := 15
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
	}
	prompt := fmt.Sprintf("Analyze git log to find the %d most frequently changed files in this repository. "+
		"Use 'git log --pretty=format: --name-only' and count occurrences. "+
		"Present results as a ranked table with columns: Rank, File, Changes.", n)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdContributors(ctx Context) Result {
	prompt := "Analyze git contributors in this repository. Use 'git shortlog -sne HEAD' to get " +
		"commit counts per author. Present results as a ranked table with columns: Rank, Author, Commits. " +
		"Sort by commit count descending."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdLanguages(ctx Context) Result {
	prompt := "Analyze the programming languages used in this repository. For each language, " +
		"count files and lines. Present as a table sorted by lines descending. " +
		"Also calculate percentage of total for each language."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}
