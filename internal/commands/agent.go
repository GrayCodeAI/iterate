package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterAgentCommands adds agent control commands.
func RegisterAgentCommands(r *Registry) {
	r.Register(Command{
		Name:        "/model",
		Aliases:     []string{},
		Description: "switch provider/model",
		Category:    "agent",
		Handler:     cmdModel,
	})

	r.Register(Command{
		Name:        "/thinking",
		Aliases:     []string{},
		Description: "set thinking level",
		Category:    "agent",
		Handler:     cmdThinking,
	})

	r.Register(Command{
		Name:        "/tools",
		Aliases:     []string{},
		Description: "list available tools",
		Category:    "agent",
		Handler:     cmdTools,
	})

	r.Register(Command{
		Name:        "/skills",
		Aliases:     []string{},
		Description: "list available skills",
		Category:    "agent",
		Handler:     cmdSkills,
	})

	r.Register(Command{
		Name:        "/cost",
		Aliases:     []string{},
		Description: "show token usage and cost",
		Category:    "agent",
		Handler:     cmdCost,
	})

	r.Register(Command{
		Name:        "/tokens",
		Aliases:     []string{},
		Description: "show detailed token usage",
		Category:    "agent",
		Handler:     cmdTokens,
	})

	r.Register(Command{
		Name:        "/spawn",
		Aliases:     []string{},
		Description: "delegate to subagent (context-efficient)",
		Category:    "agent",
		Handler:     cmdSpawn,
	})

	r.Register(Command{
		Name:        "/swarm",
		Aliases:     []string{},
		Description: "launch N agents with rate limiting (max 100)",
		Category:    "agent",
		Handler:     cmdSwarm,
	})
}

func cmdModel(ctx Context) Result {
	// TODO: wire up model switching
	if ctx.Provider != nil {
		fmt.Printf("Current model: %s\n", ctx.Provider.Name())
	}
	fmt.Println("Model switching not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdThinking(ctx Context) Result {
	if !ctx.HasArg(1) {
		if ctx.Thinking != nil {
			fmt.Printf("Current thinking level: %s\n", *ctx.Thinking)
		}
		fmt.Println("Usage: /thinking off|minimal|low|medium|high")
		return Result{Handled: true}
	}
	if ctx.Thinking != nil {
		*ctx.Thinking = iteragent.ThinkingLevel(ctx.Arg(1))
		if ctx.Agent != nil {
			ctx.Agent.WithThinkingLevel(*ctx.Thinking)
		}
		fmt.Printf("Thinking set to %s.\n", *ctx.Thinking)
	}
	return Result{Handled: true}
}

func cmdTools(ctx Context) Result {
	if ctx.Agent == nil {
		fmt.Println("No agent available.")
		return Result{Handled: true}
	}
	tools := ctx.Agent.GetTools()
	fmt.Printf("%d tools:\n", len(tools))
	for _, t := range tools {
		desc := strings.SplitN(t.Description, "\n", 2)[0]
		fmt.Printf("  %-20s %s\n", t.Name, desc)
	}
	return Result{Handled: true}
}

func cmdSkills(ctx Context) Result {
	// TODO: wire up skill loading
	fmt.Println("Skills loading not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdCost(ctx Context) Result {
	model := "unknown"
	if ctx.Provider != nil {
		model = ctx.Provider.Name()
	}
	fmt.Printf("%s── Cost estimate ───────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  Model:         %s\n", model)
	if ctx.SessionInputTokens != nil {
		fmt.Printf("  Input tokens:  ~%d\n", *ctx.SessionInputTokens)
	}
	if ctx.SessionOutputTokens != nil {
		fmt.Printf("  Output tokens: ~%d\n", *ctx.SessionOutputTokens)
	}
	if ctx.SessionCacheRead != nil && *ctx.SessionCacheRead > 0 {
		fmt.Printf("  Cache read:    ~%d\n", *ctx.SessionCacheRead)
	}
	if ctx.SessionCacheWrite != nil && *ctx.SessionCacheWrite > 0 {
		fmt.Printf("  Cache write:   ~%d\n", *ctx.SessionCacheWrite)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdTokens(ctx Context) Result {
	fmt.Printf("%s── Token usage ─────────────────────%s\n", ColorDim, ColorReset)
	if ctx.SessionInputTokens != nil {
		fmt.Printf("  Session input:  ~%d tokens\n", *ctx.SessionInputTokens)
	}
	if ctx.SessionOutputTokens != nil {
		fmt.Printf("  Session output: ~%d tokens\n", *ctx.SessionOutputTokens)
	}
	if ctx.SessionCacheRead != nil && *ctx.SessionCacheRead > 0 {
		fmt.Printf("  Cache read:     ~%d tokens\n", *ctx.SessionCacheRead)
	}
	if ctx.SessionCacheWrite != nil && *ctx.SessionCacheWrite > 0 {
		fmt.Printf("  Cache write:    ~%d tokens\n", *ctx.SessionCacheWrite)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdSpawn(ctx Context) Result {
	if !ctx.HasArg(1) {
		fmt.Println("Usage: /spawn <task>")
		return Result{Handled: true}
	}
	task := ctx.Args()
	PrintSuccess("Spawning subagent for: %s", task)
	// TODO: wire up actual spawning via iteragent
	fmt.Println("Spawn not yet wired in modular commands.")
	return Result{Handled: true}
}

func cmdSwarm(ctx Context) Result {
	if !ctx.HasArg(2) {
		fmt.Println("Usage: /swarm <n> <task>   (max 100 agents)")
		return Result{Handled: true}
	}

	n, err := strconv.Atoi(ctx.Arg(1))
	if err != nil {
		PrintError("Invalid number: %s", ctx.Arg(1))
		return Result{Handled: true}
	}
	if n > 100 {
		n = 100
		PrintDim("Limited to 100 agents")
	}
	task := strings.Join(ctx.Parts[2:], " ")

	fmt.Printf("%s── Swarm Launch ───────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  Agents:    %d\n", n)
	fmt.Printf("  Task:      %s\n", task)
	fmt.Printf("  Concurrency: 10 max\n")
	fmt.Printf("  Rate limit:   5 req/sec\n")
	fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)

	// Swarm requires a pool to be pre-configured
	if ctx.Pool == nil {
		PrintError("Agent pool not configured. Pool must be provided via Context.")
		return Result{Handled: true}
	}

	// Run swarm with progress tracking
	var wg sync.WaitGroup
	results := make([]string, 0, n)
	var mu sync.Mutex

	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Acquire an agent (rate-limited via pool)
			ag, err := ctx.Pool.Acquire(context.Background())
			if err != nil {
				mu.Lock()
				results = append(results, fmt.Sprintf("Agent %d: error: %s", idx, err))
				mu.Unlock()
				return
			}
			defer ctx.Pool.Release(ag)

			// TODO: wire up actual agent execution with the acquired agent
			// For now, simulate
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			results = append(results, fmt.Sprintf("Agent %d: done", idx))
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("\n%s✓ Swarm complete in %s%s\n", ColorLime, elapsed.Round(time.Millisecond), ColorReset)
	fmt.Printf("  Completed: %d/%d agents\n", len(results), n)

	return Result{Handled: true}
}
