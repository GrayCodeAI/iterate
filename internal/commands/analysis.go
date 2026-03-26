package commands

import (
	"context"
	"fmt"
	"sort"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterAnalysisCommands adds repository analysis and benchmarking commands.
func RegisterAnalysisCommands(r *Registry) {
	r.Register(Command{
		Name:        "/latency",
		Aliases:     []string{"/ping"},
		Description: "measure provider round-trip latency (usage: /latency [rounds])",
		Category:    "analysis",
		Handler:     cmdLatency,
	})
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

// cmdLatency measures provider round-trip latency across N rounds.
func cmdLatency(ctx Context) Result {
	if ctx.Provider == nil {
		PrintError("no provider available")
		return Result{Handled: true}
	}

	rounds := 3
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &rounds)
	}
	if rounds < 1 || rounds > 20 {
		rounds = 3
	}

	probe := []iteragent.Message{{Role: "user", Content: "Reply with the single word: pong"}}
	opts := iteragent.CompletionOptions{MaxTokens: 10}

	fmt.Printf("%s── Benchmark: %s — %d rounds ──%s\n", ColorDim, ctx.Provider.Name(), rounds, ColorReset)

	latencies := make([]time.Duration, 0, rounds)
	var totalTokens int

	for i := 0; i < rounds; i++ {
		start := time.Now()
		resp, err := ctx.Provider.Complete(context.Background(), probe, opts)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  round %d: %s✗ %v%s\n", i+1, ColorRed, err, ColorReset)
			continue
		}
		latencies = append(latencies, elapsed)
		totalTokens += len(resp) / 4 // rough token estimate
		fmt.Printf("  round %d: %s%s%s  (%s)\n", i+1, ColorDim, elapsed.Round(time.Millisecond), ColorReset, resp)
	}

	if len(latencies) == 0 {
		PrintError("all rounds failed")
		return Result{Handled: true}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	avg := sum / time.Duration(len(latencies))
	min := latencies[0]
	max := latencies[len(latencies)-1]
	p50 := latencies[len(latencies)/2]

	fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  min:  %s%s%s\n", ColorLime, min.Round(time.Millisecond), ColorReset)
	fmt.Printf("  avg:  %s%s%s\n", ColorLime, avg.Round(time.Millisecond), ColorReset)
	fmt.Printf("  p50:  %s%s%s\n", ColorDim, p50.Round(time.Millisecond), ColorReset)
	fmt.Printf("  max:  %s%s%s\n", ColorYellow, max.Round(time.Millisecond), ColorReset)
	fmt.Printf("  ok:   %d/%d rounds\n", len(latencies), rounds)
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	_ = totalTokens
	return Result{Handled: true}
}
