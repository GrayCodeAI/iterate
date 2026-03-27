package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
)

// RegisterAgentCommands adds agent control commands.
func RegisterAgentCommands(r *Registry) {
	registerAgentCoreCommands(r)
	registerAgentAdvancedCommands(r)
}

func registerAgentCoreCommands(r *Registry) {
	registerMany(r, "agent",
		"/model", "switch provider/model", cmdModel,
		"/thinking", "set thinking level", cmdThinking,
		"/cache", "toggle Anthropic prompt caching on/off", cmdCache,
		"/tools", "list available tools", cmdTools,
		"/skills", "list available skills", cmdSkills,
	)
}

func registerAgentAdvancedCommands(r *Registry) {
	registerMany(r, "agent",
		"/cost", "show token usage and cost", cmdCost,
		"/tokens", "show detailed token usage", cmdTokens,
		"/spawn", "delegate to subagent (context-efficient)", cmdSpawn,
		"/swarm", "launch N agents with rate limiting (max 100)", cmdSwarm,
	)
}

// registerMany registers a batch of commands with the given category.
func registerMany(r *Registry, category string, args ...interface{}) {
	for i := 0; i < len(args); i += 3 {
		name := args[i].(string)
		desc := args[i+1].(string)
		handler := args[i+2].(func(Context) Result)
		r.Register(Command{
			Name:        name,
			Description: desc,
			Category:    category,
			Handler:     handler,
		})
	}
}

func cmdModel(ctx Context) Result {
	fmt.Printf("%s── Current Model ───────────────────%s\n", ColorDim, ColorReset)
	if ctx.Provider == nil {
		fmt.Println("  No provider configured.")
		fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
		return Result{Handled: true}
	}

	name := ctx.Provider.Name()
	fmt.Printf("  %sProvider:%s  %s\n", ColorBold, ColorReset, name)

	// Context window
	if cw, ok := ctx.Provider.(interface{ ContextWindow() int }); ok {
		w := cw.ContextWindow()
		if w >= 1_000_000 {
			fmt.Printf("  %sContext:%s   %.1fM tokens\n", ColorBold, ColorReset, float64(w)/1_000_000)
		} else {
			fmt.Printf("  %sContext:%s   %dk tokens\n", ColorBold, ColorReset, w/1_000)
		}
	}

	// Thinking support
	if ctx.Thinking != nil {
		level := string(*ctx.Thinking)
		if level == "" || level == "off" {
			fmt.Printf("  %sThinking:%s  off\n", ColorBold, ColorReset)
		} else {
			fmt.Printf("  %sThinking:%s  %s\n", ColorBold, ColorReset, level)
		}
	}

	// Session token usage
	if ctx.SessionInputTokens != nil && ctx.SessionOutputTokens != nil {
		total := *ctx.SessionInputTokens + *ctx.SessionOutputTokens
		if total > 0 {
			if ctx.ContextWindow != nil && *ctx.ContextWindow > 0 {
				pct := float64(total) * 100 / float64(*ctx.ContextWindow)
				fmt.Printf("  %sUsed:%s      %d tokens (%.1f%%)\n", ColorBold, ColorReset, total, pct)
			} else {
				fmt.Printf("  %sUsed:%s      %d tokens\n", ColorBold, ColorReset, total)
			}
		}
	}

	fmt.Println()
	fmt.Printf("  Use %s/provider <name>%s to switch.\n", ColorBold, ColorReset)
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
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

func cmdCache(ctx Context) Result {
	if ctx.Agent == nil {
		fmt.Println("No agent available.")
		return Result{Handled: true}
	}
	// With no argument: show current state.
	if !ctx.HasArg(1) {
		enabled := ctx.RuntimeConfig.CacheEnabled != nil && *ctx.RuntimeConfig.CacheEnabled
		fmt.Printf("Prompt caching: %s\n", map[bool]string{true: "on", false: "off"}[enabled])
		fmt.Println("Usage: /cache on|off")
		return Result{Handled: true}
	}
	val := strings.ToLower(ctx.Arg(1))
	switch val {
	case "on", "true", "1", "yes":
		on := true
		ctx.RuntimeConfig.CacheEnabled = &on
		ctx.Agent.WithCacheEnabled(true)
		fmt.Printf("%sPrompt caching enabled.%s\n", ColorLime, ColorReset)
	case "off", "false", "0", "no":
		off := false
		ctx.RuntimeConfig.CacheEnabled = &off
		ctx.Agent.WithCacheEnabled(false)
		fmt.Printf("%sPrompt caching disabled.%s\n", ColorDim, ColorReset)
	default:
		fmt.Printf("Unknown value %q. Usage: /cache on|off\n", val)
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
	skillsDir := filepath.Join(ctx.RepoPath, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		fmt.Println("No skills directory found.")
		return Result{Handled: true}
	}
	fmt.Printf("%s── Skills ─────────────────────────%s\n", ColorDim, ColorReset)
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			fmt.Printf("  %s\n", entry.Name())
			count++
		}
	}
	if count == 0 {
		fmt.Println("  No skills found.")
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
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

	if ctx.Pool != nil {
		ag, err := ctx.Pool.Acquire(context.Background())
		if err != nil {
			PrintError("failed to acquire agent: %s", err)
			return Result{Handled: true}
		}
		defer ctx.Pool.Release(ag)

		fmt.Printf("%s── subagent output ─────────────────%s\n", ColorDim, ColorReset)
		var full strings.Builder
		events := ag.Prompt(context.Background(), task)
		for e := range events {
			switch iteragent.EventType(e.Type) {
			case iteragent.EventTokenUpdate:
				fmt.Print(e.Content)
				full.WriteString(e.Content)
			case iteragent.EventError:
				PrintError(e.Content)
			}
		}
		ag.Finish()
		fmt.Printf("\n%s────────────────────────────────────%s\n", ColorDim, ColorReset)

		// Inject subagent result into the parent conversation so the model can reason over it.
		if ctx.Agent != nil && full.Len() > 0 {
			injected := fmt.Sprintf("[Subagent result for task: %q]\n\n%s", task, full.String())
			ctx.Agent.Messages = append(ctx.Agent.Messages, iteragent.NewUserMessage(injected))
			PrintDim("subagent result injected into conversation")
		}
	} else if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, task, ctx.RepoPath)
	} else if ctx.Agent != nil {
		resp, err := ctx.Agent.Run(context.Background(), "", task)
		if err != nil {
			PrintError("agent failed: %s", err)
		} else {
			fmt.Println(resp)
		}
	} else {
		PrintError("no agent available")
	}
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

	if ctx.Pool == nil {
		PrintError("Agent pool not configured. Pool must be provided via Context.")
		return Result{Handled: true}
	}

	results := executeSwarmAgents(ctx.Pool, n, task)
	printSwarmResults(results, n)

	return Result{Handled: true}
}

type swarmAgentResult struct {
	idx     int
	content string
	isError bool
}

func executeSwarmAgents(pool *agent.Pool, n int, task string) []string {
	resultCh := make(chan swarmAgentResult, n)
	sem := make(chan struct{}, 10)
	var doneCount int32

	for i := 0; i < n; i++ {
		go func(idx int) {
			sem <- struct{}{}
			defer func() { <-sem }()

			ag, err := pool.Acquire(context.Background())
			if err != nil {
				resultCh <- swarmAgentResult{idx: idx, content: fmt.Sprintf("error: %s", err), isError: true}
				return
			}
			defer pool.Release(ag)

			// Stream the agent, collecting output silently.
			var full strings.Builder
			events := ag.Prompt(context.Background(), task)
			for e := range events {
				if iteragent.EventType(e.Type) == iteragent.EventTokenUpdate {
					full.WriteString(e.Content)
				}
			}
			ag.Finish()

			done := int(atomic.AddInt32(&doneCount, 1))
			fmt.Printf("\r%sSwarm: %d/%d completed…%s", ColorDim, done, n, ColorReset)

			resultCh <- swarmAgentResult{idx: idx, content: full.String()}
		}(i)
	}

	results := make([]string, 0, n)
	for i := 0; i < n; i++ {
		r := <-resultCh
		snippet := r.content
		if len(snippet) > 120 {
			snippet = snippet[:120] + "…"
		}
		results = append(results, fmt.Sprintf("Agent %d: %s", r.idx, snippet))
	}
	fmt.Printf("\r\033[K") // clear progress line
	return results
}

func printSwarmResults(results []string, n int) {
	errCount := 0
	successCount := 0
	for _, r := range results {
		if strings.Contains(r, "error:") {
			errCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("\n%s── Swarm Results ──────────────────%s\n", ColorDim, ColorReset)
	for _, r := range results {
		if strings.Contains(r, "error:") {
			fmt.Printf("  %s✗%s %s\n", ColorRed, ColorReset, r)
		} else {
			fmt.Printf("  %s✓%s %s\n", ColorLime, ColorReset, r)
		}
	}
	fmt.Printf("%s──────────────────────────────────%s\n", ColorDim, ColorReset)
	fmt.Printf("  %s%d/%d succeeded%s", ColorLime, successCount, n, ColorReset)
	if errCount > 0 {
		fmt.Printf("  %s%d failed%s", ColorRed, errCount, ColorReset)
	}
	fmt.Println()
}
