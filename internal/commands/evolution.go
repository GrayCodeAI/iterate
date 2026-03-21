package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterEvolutionCommands adds evolution-related commands.
func RegisterEvolutionCommands(r *Registry) {
	r.Register(Command{
		Name:        "/coverage",
		Aliases:     []string{},
		Description: "run tests with coverage",
		Category:    "evolution",
		Handler:     cmdCoverage,
	})

	r.Register(Command{
		Name:        "/mutants",
		Aliases:     []string{},
		Description: "run mutation tests",
		Category:    "evolution",
		Handler:     cmdMutants,
	})

	r.Register(Command{
		Name:        "/day",
		Aliases:     []string{},
		Description: "show/set evolution day count",
		Category:    "evolution",
		Handler:     cmdDay,
	})

	r.Register(Command{
		Name:        "/journal",
		Aliases:     []string{},
		Description: "view JOURNAL.md",
		Category:    "evolution",
		Handler:     cmdJournal,
	})

	r.Register(Command{
		Name:        "/phase",
		Aliases:     []string{},
		Description: "run evolution phase (plan|implement|communicate)",
		Category:    "evolution",
		Handler:     cmdPhase,
	})

	r.Register(Command{
		Name:        "/snapshot",
		Aliases:     []string{},
		Description: "save conversation snapshot",
		Category:    "evolution",
		Handler:     cmdSnapshot,
	})

	r.Register(Command{
		Name:        "/snapshots",
		Aliases:     []string{},
		Description: "list saved snapshots",
		Category:    "evolution",
		Handler:     cmdSnapshots,
	})

	r.Register(Command{
		Name:        "/evolve-now",
		Aliases:     []string{},
		Description: "run full evolution loop",
		Category:    "evolution",
		Handler:     cmdEvolveNow,
	})

	r.Register(Command{
		Name:        "/self-improve",
		Aliases:     []string{},
		Description: "analyze and improve own code",
		Category:    "evolution",
		Handler:     cmdSelfImprove,
	})
}

// EvolutionContext provides additional context for evolution commands.
type EvolutionContext struct {
	Logger        *slog.Logger
	StreamAndPrint func(ctx context.Context, a *iteragent.Agent, prompt, repoPath string)
	EventSink     chan iteragent.Event
}

// EvolutionHandler wraps a function that needs evolution context.
func EvolutionHandler(fn func(Context, EvolutionContext) Result, evoCtx EvolutionContext) func(Context) Result {
	return func(ctx Context) Result {
		return fn(ctx, evoCtx)
	}
}

func cmdCoverage(ctx Context) Result {
	fmt.Printf("%sRunning tests with coverage…%s\n", ColorDim, ColorReset)
	// TODO: wire up runCoverage function
	fmt.Println("Coverage command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdMutants(ctx Context) Result {
	fmt.Printf("%sRunning mutation tests…%s\n", ColorDim, ColorReset)
	fmt.Printf("%sThis finds untested code paths by mutating code and checking if tests catch it.%s\n\n", ColorDim, ColorReset)
	// TODO: wire up via agent stream
	fmt.Println("Mutants command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdDay(ctx Context) Result {
	dayFile := filepath.Join(ctx.RepoPath, "DAY_COUNT")
	currentDay := "1"
	if data, err := os.ReadFile(dayFile); err == nil && len(data) > 0 {
		currentDay = strings.TrimSpace(string(data))
	}
	
	if !ctx.HasArg(1) {
		fmt.Printf("%sCurrent day: %s%s\n\n", ColorLime, currentDay, ColorReset)
		return Result{Handled: true}
	}
	
	newDay := ctx.Arg(1)
	if err := os.WriteFile(dayFile, []byte(newDay), 0644); err != nil {
		PrintError("Failed to update day: %v", err)
	} else {
		fmt.Printf("%sDay updated: %s → %s%s\n\n", ColorLime, currentDay, newDay, ColorReset)
	}
	return Result{Handled: true}
}

func cmdJournal(ctx Context) Result {
	n := 50
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
	}
	// TODO: wire up viewJournal function
	fmt.Printf("%s── JOURNAL.md (last %d lines) ───────%s\n", ColorDim, n, ColorReset)
	fmt.Println("Journal command not yet wired in modular commands.")
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPhase(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /phase plan|implement|communicate")
		return Result{Handled: true}
	}
	
	phase := ctx.Arg(1)
	fmt.Printf("Running phase: %s\n", phase)
	
	// TODO: wire up evolution engine with event sink
	// This requires Provider, Logger, and event channel
	
	fmt.Printf("Phase %s command not yet fully wired.\n", phase)
	return Result{Handled: true}
}

func cmdSnapshot(ctx Context) Result {
	name := ctx.Args()
	if name == "" {
		name = time.Now().Format("20060102-150405")
	}
	// TODO: wire up saveSnapshot function
	PrintSuccess("snapshot saved: %s", name)
	return Result{Handled: true}
}

func cmdSnapshots(ctx Context) Result {
	// TODO: wire up listSnapshots function
	fmt.Println("Snapshots command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdEvolveNow(ctx Context) Result {
	// TODO: wire up via agent stream
	fmt.Println("Evolve-now command not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdSelfImprove(ctx Context) Result {
	// TODO: wire up via agent stream
	fmt.Println("Self-improve command not yet wired in modular commands.")
	return Result{Handled: true}
}
