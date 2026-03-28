package commands

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RegisterContextTemplateCommands adds context template commands.
// Task 48: Context Templates for common workflows
// Task 50: Context Analytics
func RegisterContextTemplateCommands(r *Registry) {
	r.Register(Command{
		Name:        "/ctx-bugfix",
		Aliases:     []string{},
		Description: "context template for bug fixing",
		Category:    "context",
		Handler:     cmdCtxBugfix,
	})
	r.Register(Command{
		Name:        "/ctx-feature",
		Aliases:     []string{},
		Description: "context template for feature work",
		Category:    "context",
		Handler:     cmdCtxFeature,
	})
	r.Register(Command{
		Name:        "/ctx-refactor",
		Aliases:     []string{},
		Description: "context template for refactoring",
		Category:    "context",
		Handler:     cmdCtxRefactor,
	})
	r.Register(Command{
		Name:        "/ctx-test",
		Aliases:     []string{},
		Description: "context template for test writing",
		Category:    "context",
		Handler:     cmdCtxTest,
	})
	r.Register(Command{
		Name:        "/ctx-debug",
		Aliases:     []string{},
		Description: "context template for debugging",
		Category:    "context",
		Handler:     cmdCtxDebug,
	})
	r.Register(Command{
		Name:        "/ctx-review",
		Aliases:     []string{},
		Description: "context template for code review",
		Category:    "context",
		Handler:     cmdCtxReview,
	})
	r.Register(Command{
		Name:        "/ctx-analytics",
		Aliases:     []string{},
		Description: "show context usage analytics",
		Category:    "context",
		Handler:     cmdCtxAnalytics,
	})
}

func cmdCtxBugfix(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are fixing a bug in %s.

Before starting:
1. Reproduce the bug — understand the exact failure
2. Read the relevant source files
3. Check git log for when the bug was introduced
4. Write a failing test that captures the bug

Then fix the bug and verify the test passes.

Focus: one bug, one fix, one commit. Don't refactor while fixing.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxFeature(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are implementing a new feature in %s.

Before starting:
1. Read existing code to understand architecture and patterns
2. Identify where the feature fits
3. Check for similar existing features to follow patterns
4. Plan the implementation (files to create/modify)

Then implement:
1. Write tests first (TDD)
2. Implement the feature
3. Run all tests
4. Refactor if needed

Focus: one feature, clean implementation, comprehensive tests.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxRefactor(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are refactoring code in %s.

Rules:
1. Do NOT change behavior — refactoring means same input, same output
2. Run tests before and after — they must pass both times
3. Make small, incremental changes — one refactor per commit
4. Preserve all public APIs

Steps:
1. Identify the code to refactor
2. Understand current behavior (read tests, trace execution)
3. Plan the refactoring steps
4. Apply changes incrementally, testing after each step

Focus: clean, readable, maintainable code. No feature changes.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxTest(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are writing tests for %s.

Before starting:
1. Check existing test coverage (go test -cover ./...)
2. Identify untested functions and edge cases
3. Read existing tests for patterns and conventions

Test strategy:
1. Test public APIs first
2. Cover edge cases: empty inputs, nil, boundary values
3. Test error paths — what happens when things go wrong?
4. Use table-driven tests where appropriate
5. Keep tests independent — no shared state

Focus: high-quality tests that catch real bugs.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxDebug(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are debugging an issue in %s.

Systematic approach:
1. Reproduce the issue — exact steps to trigger
2. Narrow down — which component is failing?
3. Read the relevant source code carefully
4. Add diagnostic output (logging, print statements)
5. Form a hypothesis
6. Test the hypothesis
7. Fix the root cause (not the symptom)
8. Write a test to prevent regression

Never guess. Read the code. Trace the execution. Find the actual cause.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxReview(ctx Context) Result {
	repoName := filepath.Base(ctx.RepoPath)
	prompt := fmt.Sprintf(`You are reviewing code changes in %s.

Review checklist:
1. Read the diff carefully — what changed and why?
2. Check for bugs: logic errors, off-by-one, nil dereferences, race conditions
3. Check for security: input validation, injection, secrets exposure
4. Check for performance: unnecessary allocations, N+1 queries, missing caching
5. Check for maintainability: naming, complexity, duplication
6. Check for tests: are changes covered? Are edge cases tested?
7. Check for documentation: are public APIs documented?

Be specific. Reference file:line for each issue. Be honest but constructive.`, repoName)

	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdCtxAnalytics(ctx Context) Result {
	// Show context usage statistics
	fmt.Printf("%s── Context Analytics ──────────────%s\n", ColorDim, ColorReset)

	// Count learnings by category
	learnings := loadLearnings(ctx.RepoPath)
	if len(learnings) > 0 {
		categories := categorizeLearnings(learnings)
		fmt.Printf("\n  %sLearning Categories Used:%s\n", ColorBold, ColorReset)
		for cat, count := range categories {
			fmt.Printf("    %-20s %d entries\n", cat, count)
		}
	}

	// Show available context templates
	fmt.Printf("\n  %sAvailable Context Templates:%s\n", ColorBold, ColorReset)
	templates := []struct {
		cmd  string
		desc string
	}{
		{"/ctx-bugfix", "Bug fixing workflow"},
		{"/ctx-feature", "Feature development"},
		{"/ctx-refactor", "Refactoring (no behavior change)"},
		{"/ctx-test", "Test writing strategy"},
		{"/ctx-debug", "Systematic debugging"},
		{"/ctx-review", "Code review checklist"},
	}
	for _, t := range templates {
		fmt.Printf("    %-18s %s\n", t.cmd, t.desc)
	}

	fmt.Printf("\n  %sTip:%s Use these templates at the start of a task to prime the agent context.\n", ColorCyan, ColorReset)
	fmt.Printf("%s──────────────────────────────────%s\n\n", ColorDim, ColorReset)
	return Result{Handled: true}
}

// generateContextSummary builds a brief context string for the current repo state.
func generateContextSummary(repoPath string) string {
	var parts []string

	branch := strings.TrimSpace(gitOutput(repoPath, "branch", "--show-current"))
	if branch != "" {
		parts = append(parts, fmt.Sprintf("branch: %s", branch))
	}

	status := gitOutput(repoPath, "status", "--short")
	if status != "" {
		lines := strings.Split(strings.TrimSpace(status), "\n")
		parts = append(parts, fmt.Sprintf("%d uncommitted changes", len(lines)))
	}

	log := gitOutput(repoPath, "log", "--oneline", "-1")
	if log != "" {
		parts = append(parts, fmt.Sprintf("last commit: %s", strings.TrimSpace(log)))
	}

	return strings.Join(parts, " | ")
}
