package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// maxParallelTasks controls how many task agents run concurrently.
// Each task gets its own git worktree so file edits never conflict.
const maxParallelTasks = 3

// RunPlanPhase runs the planning phase. Creates SESSION_PLAN.md via agent or fallback.
func (e *Engine) RunPlanPhase(ctx context.Context, p iteragent.Provider, issues string) error {
	ctx, cancel := withPhaseTimeout(ctx, "plan")
	defer cancel()

	identity, journal, day := readPlanContext(e.repoPath)
	userMessage := buildPlanPrompt(e.repoPath, string(journal), day, issues)

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	a := e.newAgent(p, e.tools, systemPrompt, e.skills)

	var lastContent string
	for ev := range a.Prompt(ctx, userMessage) {
		if ev.Type == string(iteragent.EventMessageEnd) {
			lastContent = ev.Content
		}
	}
	a.Finish()

	// Extract plan from agent output if it didn't write the file via tool call.
	planPath := filepath.Join(e.repoPath, "SESSION_PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) && lastContent != "" {
		extracted := extractPlan(lastContent)
		if extracted != "" {
			if err := os.WriteFile(planPath, []byte(extracted), 0o644); err == nil {
				e.logger.Info("extracted SESSION_PLAN.md from agent output")
			}
		}
	}

	// Verify a plan was produced — fail explicitly rather than silently proceeding.
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("planning phase produced no SESSION_PLAN.md")
	}
	return nil
}

// extractPlan tries multiple patterns to extract a plan from agent text output.
func extractPlan(output string) string {
	for _, prefix := range []string{"## Session Plan", "## Session plan", "# Session Plan", "## Plan"} {
		if idx := strings.Index(output, prefix); idx >= 0 {
			return strings.TrimSpace(output[idx:])
		}
	}
	if strings.Contains(output, "Task 1") || strings.Contains(output, "### Task") {
		return strings.TrimSpace(output)
	}
	return ""
}

func readPlanContext(repoPath string) ([]byte, []byte, string) {
	identity, _ := os.ReadFile(filepath.Join(repoPath, "docs/IDENTITY.md"))
	journal, _ := os.ReadFile(filepath.Join(repoPath, "docs/JOURNAL.md"))
	dayCount, _ := os.ReadFile(filepath.Join(repoPath, "DAY_COUNT"))
	return identity, journal, strings.TrimSpace(string(dayCount))
}

func buildPlanPrompt(repoPath, journal, day, issues string) string {
	learnings, _ := os.ReadFile(filepath.Join(repoPath, "memory", "ACTIVE_LEARNINGS.md"))
	ciStatus, _ := os.ReadFile(filepath.Join(repoPath, ".iterate", "ci_status.txt"))

	// Run codebase analysis for smarter task selection
	analysis := AnalyzeCodebase(repoPath)
	analysisStr := analysis.FormatAnalysis()

	var sb strings.Builder
	appendPlanInstructions(&sb, ciStatus, day)
	sb.WriteString("## Codebase Analysis\n\n")
	sb.WriteString(analysisStr)
	appendPlanContext(&sb, learnings, journal, issues)

	// Inject recent failures so the planner avoids repeating bad approaches.
	if failures := recentFailures(repoPath, 10); failures != "" {
		sb.WriteString("\n" + failures)
	}
	return sb.String()
}

func appendPlanInstructions(sb *strings.Builder, ciStatus []byte, day string) {
	if len(ciStatus) > 0 {
		sb.WriteString(strings.TrimSpace(string(ciStatus)) + "\n\n")
	}
	sb.WriteString("## Phase: Planning\n\n")
	sb.WriteString("Read your source code, then write SESSION_PLAN.md.\n\n")
	sb.WriteString("**Step 1 — Read your codebase:**\n")
	sb.WriteString("- list_files on cmd/ and internal/ recursively\n")
	sb.WriteString("- Read key source files\n")
	sb.WriteString("- Run: go build ./... && go test ./... && go vet ./...\n")
	sb.WriteString("- grep -rn 'TODO\\|FIXME\\|panic(' --include='*.go' cmd/ internal/\n\n")
	sb.WriteString("**Step 2 — Pick tasks by priority:**\n")
	sb.WriteString("0. Fix broken builds or failing tests\n")
	sb.WriteString("1. Bugs, crashes, or silent failures\n")
	sb.WriteString("2. Missing tests for existing features\n")
	sb.WriteString("3. Community issues\n")
	sb.WriteString("4. UX improvements\n\n")
	sb.WriteString("**Step 3 — Write SESSION_PLAN.md using the write_file tool:**\n\n")
	sb.WriteString("Format:\n")
	sb.WriteString("```\n## Session Plan\n\n")
	sb.WriteString("Session Title: [short title]\n\n")
	sb.WriteString("### Task 1: [title]\n")
	sb.WriteString("Files: [files to modify]\n")
	sb.WriteString("Description: [specific enough to implement blindly]\n")
	sb.WriteString("Issue: #N (or none)\n\n")
	sb.WriteString("### Issue Responses\n")
	sb.WriteString("- #N: implement — [reason]\n")
	sb.WriteString("```\n\n")
	sb.WriteString("After writing, STOP. Do not implement. Planning only.\n\n")
}

func appendPlanContext(sb *strings.Builder, learnings []byte, journal string, issues string) {
	if len(learnings) > 0 {
		l := string(learnings)
		if len(l) > 1000 {
			l = l[:1000] + "\n...[truncated]"
		}
		sb.WriteString("## What you have learned so far\n\n")
		sb.WriteString(l + "\n\n")
	}
	if len(journal) > 0 {
		recent := journal
		if len(recent) > 800 {
			recent = "...\n" + recent[len(recent)-800:]
		}
		sb.WriteString("## Recent journal\n\n")
		sb.WriteString(recent + "\n\n")
	}
	if len(issues) > 0 {
		sb.WriteString("## Community input\n\n")
		sb.WriteString(issues + "\n")
	}
}

// RunImplementPhase reads SESSION_PLAN.md and executes tasks.
func (e *Engine) RunImplementPhase(ctx context.Context, p iteragent.Provider) error {
	ctx, cancel := withPhaseTimeout(ctx, "implement")
	defer cancel()

	planBytes, err := os.ReadFile(filepath.Join(e.repoPath, "SESSION_PLAN.md"))
	if err != nil {
		return fmt.Errorf("SESSION_PLAN.md not found: %w", err)
	}
	plan := string(planBytes)

	tasks := parseSessionPlanTasks(plan)
	if len(tasks) == 0 {
		e.logger.Warn("no tasks parsed, using whole plan as single task")
		tasks = []planTask{{Number: 1, Title: "Self-improvement", Description: plan}}
	}

	systemPrompt, _, skills := e.loadImplementContext()

	protectedWarning := "\n\nPROTECTED FILES — DO NOT EDIT:\n- internal/evolution/*.go\n- .github/workflows/*.yml\n- cmd/iterate/main.go\n- scripts/evolution/evolve.sh\n\nIf a task requires editing these, skip it.\n"

	e.logger.Info("running tasks in parallel", "count", len(tasks), "max_parallel", maxParallelTasks)
	e.runTasksParallel(ctx, p, tasks, systemPrompt, skills, protectedWarning)

	// Catch-all commit for any tracked changes the agents forgot to commit.
	sessionTitle := extractSessionTitle(plan)
	finalMsg := "iterate: implement session changes"
	if sessionTitle != "" {
		finalMsg = "chore: " + sessionTitle
	}
	if _, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git add -u && git diff --cached --quiet || git commit -m %q", finalMsg),
	}); err != nil {
		e.logger.Warn("final commit failed", "err", err)
	}

	return nil
}

// runTasksParallel groups tasks by file overlap into waves, then runs each wave
// in parallel (up to maxParallelTasks concurrent). Tasks sharing declared files
// are placed in separate waves and run sequentially to prevent cherry-pick conflicts.
func (e *Engine) runTasksParallel(ctx context.Context, p iteragent.Provider, tasks []planTask, systemPrompt string, skills *iteragent.SkillSet, protectedWarning string) {
	waves := groupTasksByFileOverlap(tasks)
	e.logger.Info("task waves planned", "waves", len(waves), "tasks", len(tasks))

	for waveIdx, wave := range waves {
		e.logger.Info("running wave", "wave", waveIdx+1, "tasks", len(wave))
		e.runWave(ctx, p, wave, systemPrompt, skills, protectedWarning)
	}
}

// runWave runs all tasks in a single wave concurrently then cherry-picks their commits.
// Tasks in a wave are guaranteed to have non-overlapping declared files.
func (e *Engine) runWave(ctx context.Context, p iteragent.Provider, tasks []planTask, systemPrompt string, skills *iteragent.SkillSet, protectedWarning string) {
	type taskResult struct {
		task    planTask
		commits []string
		success bool
	}

	results := make([]taskResult, len(tasks))
	sem := make(chan struct{}, maxParallelTasks)
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task planTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			e.logger.Info("starting task in worktree", "number", task.Number, "title", task.Title)
			commits, ok := e.runTaskInWorktree(ctx, p, task, systemPrompt, skills, protectedWarning)
			results[i] = taskResult{task: task, commits: commits, success: ok}
			if ok {
				e.logger.Info("task succeeded", "number", task.Number, "commits", len(commits))
			} else {
				e.logger.Warn("task failed, skipping", "number", task.Number)
			}
		}(i, task)
	}

	wg.Wait()

	// Cherry-pick each task's commits in task order so history is clean.
	for _, r := range results {
		if !r.success || len(r.commits) == 0 {
			continue
		}
		e.logger.Info("cherry-picking task commits", "number", r.task.Number, "commits", len(r.commits))
		for _, hash := range r.commits {
			if _, err := e.runTool(ctx, "bash", map[string]string{
				"cmd": fmt.Sprintf("git cherry-pick %s", hash),
			}); err != nil {
				e.logger.Warn("cherry-pick failed, aborting task commits", "number", r.task.Number, "hash", hash, "err", err)
				_, _ = e.runTool(ctx, "bash", map[string]string{"cmd": "git cherry-pick --abort 2>/dev/null || true"})
				break
			}
		}
	}
}

// runTaskInWorktree creates an isolated git worktree, runs the task inside it,
// and returns the list of new commit hashes produced (in chronological order).
func (e *Engine) runTaskInWorktree(ctx context.Context, p iteragent.Provider, task planTask, systemPrompt string, skills *iteragent.SkillSet, protectedWarning string) ([]string, bool) {
	worktreeDir := filepath.Join(e.repoPath, ".iterate", "worktrees", fmt.Sprintf("task-%d-%s", task.Number, e.traceID[:6]))
	branchName := fmt.Sprintf("wt/task-%d-%s", task.Number, e.traceID[:6])

	// Clean up any leftover worktree from a previous run.
	_, _ = e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git worktree remove --force %q 2>/dev/null; git branch -D %q 2>/dev/null; true", worktreeDir, branchName),
	})

	// Record base commit before task so we can collect new commits afterward.
	baseHash, err := e.runTool(ctx, "bash", map[string]string{"cmd": "git rev-parse HEAD"})
	if err != nil {
		e.logger.Warn("failed to get base commit hash", "err", err)
		return nil, false
	}
	baseHash = strings.TrimSpace(baseHash)

	// Create the worktree branched at current HEAD.
	if out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git worktree add -b %q %q HEAD", branchName, worktreeDir),
	}); err != nil {
		e.logger.Warn("failed to create worktree", "task", task.Number, "err", err, "output", out)
		return nil, false
	}

	// Always remove the worktree when done.
	defer func() {
		_, _ = e.runTool(context.Background(), "bash", map[string]string{
			"cmd": fmt.Sprintf("git worktree remove --force %q 2>/dev/null; git branch -D %q 2>/dev/null; true", worktreeDir, branchName),
		})
	}()

	// Build a sub-engine scoped to the worktree directory.
	wtTools := iteragent.DefaultTools(worktreeDir)
	wtSkills, err := iteragent.LoadSkills([]string{filepath.Join(worktreeDir, "skills")})
	if err != nil || wtSkills == nil {
		wtSkills = skills
	}
	subEngine := &Engine{
		repoPath:      worktreeDir,
		repo:          e.repo,
		logger:        e.logger.With("task", task.Number, "worktree", "yes"),
		traceID:       e.traceID,
		toolMap:       iteragent.ToolMap(wtTools),
		tools:         wtTools,
		skills:        wtSkills,
		thinkingLevel: e.thinkingLevel,
		eventSink:     e.eventSink,
	}

	// Run the task (with retry) inside the worktree.
	subEngine.executeTask(ctx, p, task, systemPrompt, wtTools, wtSkills, protectedWarning)

	// Collect all commits added by the task (between base and new HEAD of worktree branch).
	out, err := subEngine.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git log %s..HEAD --format=%%H --reverse", baseHash),
	})
	if err != nil || strings.TrimSpace(out) == "" {
		e.logger.Warn("no new commits from task", "number", task.Number)
		return nil, false
	}

	var commits []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if h := strings.TrimSpace(line); h != "" {
			commits = append(commits, h)
		}
	}
	return commits, true
}

// executeTask runs a single task. On failure, reverts and retries once with error context.
func (e *Engine) executeTask(ctx context.Context, p iteragent.Provider, task planTask, systemPrompt string, tools []iteragent.Tool, skills *iteragent.SkillSet, protectedWarning string) {
	ok, errCtx := e.runTaskAttempt(ctx, p, task, systemPrompt, tools, skills, protectedWarning, "")
	if ok {
		return
	}

	// First attempt failed. Retry with the actual error context captured before revert.
	e.logger.Info("retrying task after failure", "number", task.Number)
	retryCtx := "Previous attempt failed.\n\n" + errCtx + "\n\nFix the errors and try again."
	if ok, failReason := e.runTaskAttempt(ctx, p, task, systemPrompt, tools, skills, protectedWarning, retryCtx); ok {
		e.logger.Info("task succeeded on retry", "number", task.Number)
	} else {
		e.logger.Warn("task failed after retry, skipping", "number", task.Number)
		if err := e.appendFailureJSONL(task.Title, firstLine(failReason)); err != nil {
			e.logger.Warn("failed to record task failure", "task", task.Title, "err", err)
		}
	}
}

// runTaskAttempt executes one attempt at a task. Returns (success, errorContext).
// errorContext is populated on failure with build/test output captured before reverting.
func (e *Engine) runTaskAttempt(ctx context.Context, p iteragent.Provider, task planTask, systemPrompt string, tools []iteragent.Tool, skills *iteragent.SkillSet, protectedWarning, extraContext string) (bool, string) {
	userMsg := fmt.Sprintf("Implement Task %d: %s\n\n%s", task.Number, task.Description, protectedWarning)
	if extraContext != "" {
		userMsg += "\n\n" + extraContext
	}
	userMsg += "\n\nAfter implementing, run: go build ./... && go test ./...\nThen commit your changes using a conventional commit message (e.g. feat: ..., fix: ..., chore: ..., refactor: ..., test: ..., docs: ...)."

	a := e.newAgent(p, tools, systemPrompt, skills)
	var taskOutput string
	var taskErr error
	for ev := range a.Prompt(ctx, userMsg) {
		if ev.Type == string(iteragent.EventMessageEnd) {
			taskOutput = ev.Content
		}
		if ev.Type == string(iteragent.EventError) {
			taskErr = fmt.Errorf("%s", ev.Content)
		}
	}
	a.Finish()

	if taskErr != nil {
		e.logger.Warn("task error", "number", task.Number, "err", taskErr)
		_ = e.revert(ctx)
		return false, fmt.Sprintf("Agent error: %s", taskErr)
	}

	if violations, err := e.verifyProtected(ctx); err != nil {
		e.logger.Warn("verifyProtected check failed", "err", err)
	} else if len(violations) > 0 {
		e.logger.Warn("protected files modified, reverting", "number", task.Number, "files", violations)
		_ = e.revert(ctx)
		return false, fmt.Sprintf("Protected files were modified (not allowed): %v", violations)
	}

	v := e.verify(ctx)
	if !v.BuildPassed || !v.TestPassed {
		// Capture error output BEFORE reverting so the retry has meaningful context.
		errCtx := fmt.Sprintf("Build passed: %v, Test passed: %v.\n\nOutput:\n%s", v.BuildPassed, v.TestPassed, v.Output)
		e.logger.Warn("verification failed, reverting", "number", task.Number, "build", v.BuildPassed, "test", v.TestPassed)
		_ = e.revert(ctx)
		return false, errCtx
	}

	if err := e.appendLearningJSONL(firstLine(extractCommitMessage(taskOutput)), "evolution", task.Description, ""); err != nil {
		e.logger.Warn("failed to record task learning", "task", task.Title, "err", err)
	}
	return true, ""
}

// loadImplementContext prepares the system prompt for implementation using cached tools and skills.
func (e *Engine) loadImplementContext() (string, []iteragent.Tool, *iteragent.SkillSet) {
	identity, err := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	if err != nil {
		e.logger.Warn("failed to load IDENTITY.md, agent will run without identity context", "err", err)
	}
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	return systemPrompt, e.tools, e.skills
}
