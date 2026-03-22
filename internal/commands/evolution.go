package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RegisterEvolutionCommands adds evolution-related commands.
func RegisterEvolutionCommands(r *Registry) {
	registerEvolutionAnalysisCommands(r)
	registerEvolutionLifecycleCommands(r)
	registerEvolutionGenerationCommands(r)
}

func registerEvolutionAnalysisCommands(r *Registry) {
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
}

func registerEvolutionLifecycleCommands(r *Registry) {
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

func registerEvolutionCreationCommands(r *Registry) {
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
		Name:        "/changelog",
		Aliases:     []string{},
		Description: "generate changelog from git log",
		Category:    "evolution",
		Handler:     cmdChangelog,
	})

	r.Register(Command{
		Name:        "/docs",
		Aliases:     []string{},
		Description: "generate documentation",
		Category:    "evolution",
		Handler:     cmdDocs,
	})
}

func registerEvolutionGenerationCommands(r *Registry) {
	registerEvolutionCreationCommands(r)

	r.Register(Command{
		Name:        "/skill-create",
		Aliases:     []string{},
		Description: "scaffold a new skill",
		Category:    "evolution",
		Handler:     cmdSkillCreate,
	})

	r.Register(Command{
		Name:        "/diagram",
		Aliases:     []string{},
		Description: "generate architecture diagram",
		Category:    "evolution",
		Handler:     cmdDiagram,
	})

	r.Register(Command{
		Name:        "/generate-readme",
		Aliases:     []string{},
		Description: "generate README.md",
		Category:    "evolution",
		Handler:     cmdGenerateReadme,
	})
}

// EvolutionContext provides additional context for evolution commands.
type EvolutionContext struct {
	Logger         *slog.Logger
	StreamAndPrint func(ctx context.Context, a *iteragent.Agent, prompt, repoPath string)
	EventSink      chan iteragent.Event
}

// EvolutionHandler wraps a function that needs evolution context.
func EvolutionHandler(fn func(Context, EvolutionContext) Result, evoCtx EvolutionContext) func(Context) Result {
	return func(ctx Context) Result {
		return fn(ctx, evoCtx)
	}
}

func cmdCoverage(ctx Context) Result {
	fmt.Printf("%sRunning tests with coverage…%s\n", ColorDim, ColorReset)
	if ctx.REPL.RunShell != nil {
		ctx.REPL.RunShell(ctx.RepoPath, "go", "test", "-coverprofile=coverage.out", "./...")
		ctx.REPL.RunShell(ctx.RepoPath, "go", "tool", "cover", "-func=coverage.out")
	} else {
		cmd := exec.Command("go", "test", "-coverprofile=coverage.out", "./...")
		cmd.Dir = ctx.RepoPath
		cmd.Stdout = Stdout
		cmd.Stderr = Stdout
		cmd.Run()
		cmd2 := exec.Command("go", "tool", "cover", "-func=coverage.out")
		cmd2.Dir = ctx.RepoPath
		cmd2.Stdout = Stdout
		cmd2.Stderr = Stdout
		cmd2.Run()
	}
	return Result{Handled: true}
}

func cmdMutants(ctx Context) Result {
	fmt.Printf("%sRunning mutation tests…%s\n", ColorDim, ColorReset)
	fmt.Printf("%sThis finds untested code paths by mutating code and checking if tests catch it.%s\n\n", ColorDim, ColorReset)
	prompt := "Analyze this Go codebase for mutation testing opportunities. " +
		"Identify: boolean conditions that could be flipped, return values that could be changed, " +
		"boundary conditions (>= to >), and arithmetic operations. " +
		"For each mutation found, suggest which existing test should catch it, " +
		"or if a new test is needed. Be specific with file names and line numbers."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
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
	journalPath := filepath.Join(ctx.RepoPath, "JOURNAL.md")
	data, err := os.ReadFile(journalPath)
	if err != nil {
		PrintError("JOURNAL.md not found: %v", err)
		return Result{Handled: true}
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	fmt.Printf("%s── JOURNAL.md (last %d lines) ───────%s\n", ColorDim, n, ColorReset)
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdPhase(ctx Context) Result {
	if !ctx.HasArg(1) {
		dayFile := filepath.Join(ctx.RepoPath, "DAY_COUNT")
		day := "1"
		if data, err := os.ReadFile(dayFile); err == nil {
			day = strings.TrimSpace(string(data))
		}
		fmt.Printf("Current day: %s\n", day)
		fmt.Println("Usage: /phase plan|implement|communicate")
		return Result{Handled: true}
	}

	phase := ctx.Arg(1)
	switch phase {
	case "plan":
		prompt := "You are in the PLAN phase of evolution. Read the source code, JOURNAL.md, " +
			"and any open GitHub issues. Create a SESSION_PLAN.md with 3-5 concrete tasks " +
			"for today's evolution cycle. Each task should be specific and testable."
		if ctx.REPL.StreamAndPrint != nil {
			ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
		}
	case "implement":
		prompt := "You are in the IMPLEMENT phase. Read SESSION_PLAN.md and implement each task. " +
			"For each task: implement the change, run `go test ./...` to verify, " +
			"then move to the next task. Commit after each successful task."
		if ctx.REPL.StreamAndPrint != nil {
			ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
		}
	case "communicate":
		prompt := "You are in the COMMUNICATE phase. Read SESSION_PLAN.md for any issue responses, " +
			"then post comments on the relevant GitHub issues summarizing what was done. " +
			"Also update JOURNAL.md with today's progress."
		if ctx.REPL.StreamAndPrint != nil {
			ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
		}
	default:
		PrintError("unknown phase: %s (use plan, implement, or communicate)", phase)
	}
	return Result{Handled: true}
}

func cmdSnapshot(ctx Context) Result {
	name := ctx.Args()
	if name == "" {
		name = time.Now().Format("20060102-150405")
	}
	if ctx.Snapshots.SaveSnapshot == nil {
		PrintError("snapshot system not available")
		return Result{Handled: true}
	}
	if ctx.Agent == nil {
		PrintError("no agent available")
		return Result{Handled: true}
	}
	if err := ctx.Snapshots.SaveSnapshot(name, ctx.Agent.Messages); err != nil {
		PrintError("snapshot failed: %v", err)
	} else {
		PrintSuccess("snapshot saved: %s", name)
	}
	return Result{Handled: true}
}

func cmdSnapshots(ctx Context) Result {
	if ctx.Snapshots.ListSnapshots == nil {
		PrintError("snapshot system not available")
		return Result{Handled: true}
	}
	snaps := ctx.Snapshots.ListSnapshots()
	if len(snaps) == 0 {
		fmt.Println("No snapshots saved. Use /snapshot [name] to create one.")
		return Result{Handled: true}
	}
	fmt.Printf("%s── Snapshots ──────────────────────%s\n", ColorDim, ColorReset)
	for _, s := range snaps {
		fmt.Printf("  %-30s  %s  (%d msgs)\n", s.Name, s.CreatedAt.Format("01-02 15:04"), len(s.Messages))
	}
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

func cmdEvolveNow(ctx Context) Result {
	dayFile := filepath.Join(ctx.RepoPath, "DAY_COUNT")
	day := "1"
	if data, err := os.ReadFile(dayFile); err == nil {
		day = strings.TrimSpace(string(data))
	}
	prompt := fmt.Sprintf("Run the full evolution loop for day %s:\n\n"+
		"1. PLAN: Read the codebase, JOURNAL.md, and open issues. Write SESSION_PLAN.md with 3-5 tasks.\n"+
		"2. IMPLEMENT: For each task in SESSION_PLAN.md, implement it, test with `go test ./...`, commit.\n"+
		"3. COMMUNICATE: Post issue responses, update JOURNAL.md with today's progress.\n"+
		"4. Update DAY_COUNT to the next day.\n\n"+
		"Start with the PLAN phase now.", day)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdSelfImprove(ctx Context) Result {
	prompt := "Analyze this codebase (an AI coding agent called 'iterate') and suggest improvements. " +
		"Look for: incomplete implementations (TODOs, stubs, 'not yet wired'), code duplication, " +
		"missing error handling, performance issues, and architectural weaknesses. " +
		"For each suggestion: rate priority (high/medium/low), estimate effort, " +
		"and describe the specific change needed. Be actionable."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintError("agent stream not available")
	}
	return Result{Handled: true}
}

func cmdChangelog(ctx Context) Result {
	since := ""
	if ctx.HasArg(1) {
		since = ctx.Arg(1)
	}

	var prompt string
	if since != "" {
		prompt = fmt.Sprintf("Generate a changelog from git log since %s. "+
			"Group by: Features, Bug Fixes, Refactors, Docs. "+
			"Use conventional commit format. Be concise.", since)
	} else {
		prompt = "Generate a changelog from the recent git history. " +
			"Group by: Features, Bug Fixes, Refactors, Docs. " +
			"Use conventional commit format. Be concise."
	}

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintSuccess("changelog generation requires agent stream")
	}
	return Result{Handled: true}
}

func cmdDocs(ctx Context) Result {
	target := "."
	if ctx.HasArg(1) {
		target = ctx.Args()
	}
	prompt := fmt.Sprintf(
		"Generate comprehensive documentation for %s. Include: overview, function signatures, "+
			"parameters, return values, and usage examples. Format as markdown.", target)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	} else {
		PrintSuccess("docs generation requires agent stream")
	}
	return Result{Handled: true}
}

func cmdSkillCreate(ctx Context) Result {
	name := ""
	desc := "A new iterate skill."
	if ctx.HasArg(1) {
		name = ctx.Arg(1)
	}
	if ctx.HasArg(2) {
		desc = strings.Join(ctx.Parts[2:], " ")
	}
	if name == "" {
		if ctx.REPL.PromptLine != nil {
			var ok bool
			name, ok = ctx.REPL.PromptLine("Skill name:")
			if !ok || name == "" {
				return Result{Handled: true}
			}
		} else {
			fmt.Print("Skill name: ")
			fmt.Scanln(&name)
		}
	}
	skillDir := filepath.Join(ctx.RepoPath, "skills", name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		PrintError("%s", err)
		return Result{Handled: true}
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\ntools: [bash, read_file, write_file, edit_file]\n---\n\n# %s\n\n## Steps\n\n1. TODO: define steps\n", name, desc, name)
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		PrintError("%s", err)
		return Result{Handled: true}
	}
	PrintSuccess("skill scaffolded at %s", skillPath)
	if ctx.REPL.StreamAndPrint != nil {
		prompt := fmt.Sprintf(
			"Read the skill file at %s and improve it: fill in realistic steps, "+
				"add good examples, and make the description compelling. Save the improved version.", skillPath)
		fmt.Printf("%sRefining skill with AI…%s\n", ColorDim, ColorReset)
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdDiagram(ctx Context) Result {
	prompt := "Analyze the codebase structure and generate an architecture diagram in ASCII art or Mermaid syntax. " +
		"Show: main packages, key types/interfaces, data flow between components, and external dependencies."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdGenerateReadme(ctx Context) Result {
	prompt := "Analyze this repository and generate a comprehensive README.md. Include: " +
		"project name and description, features list, installation instructions, usage examples, " +
		"architecture overview, and contribution guidelines. Use the existing code and config files for context."
	fmt.Printf("%sGenerate and write README.md? (y/N): %s", ColorYellow, ColorReset)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		prompt += " Write the result directly to README.md."
	}
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}
