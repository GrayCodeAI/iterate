package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// maxParallelTasks controls how many task agents run concurrently.
// Each task gets its own git worktree so file edits never conflict.
const maxParallelTasks = 3

// Pre-compiled regexes for tool call parsing (avoids recompilation on every call).
var (
	jsonToolCallRe = regexp.MustCompile(`\{[^{}]*"(?:tool|function_call|action|name)"[^{}]*\}`)
	fileContentRe  = regexp.MustCompile("(?s)([a-zA-Z0-9_/.-]+\\.go)\\n```(?:diff)?\\n(.*?)```")
)

// RunPlanPhase runs the planning phase. Creates SESSION_PLAN.md via agent or fallback.
func (e *Engine) RunPlanPhase(ctx context.Context, p iteragent.Provider, issues string) error {
	ctx, cancel := withPhaseTimeout(ctx, "plan")
	defer cancel()

	identity, journal, day := readPlanContext(e.repoPath)
	userMessage := buildPlanPrompt(e.repoPath, string(journal), day, issues)

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	a := e.newAgent(p, e.tools, systemPrompt, e.skills)

	var contentBuilder strings.Builder
	var lastContent string
	for ev := range a.Prompt(ctx, userMessage) {
		if ev.Type == string(iteragent.EventMessageUpdate) {
			contentBuilder.WriteString(ev.Content)
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			lastContent = ev.Content
		}
	}
	a.Finish()

	// Use accumulated content or final content, whichever is longer
	accumulatedContent := contentBuilder.String()
	if len(accumulatedContent) > len(lastContent) {
		lastContent = accumulatedContent
	}

	// Extract plan from agent output if it didn't write the file via tool call.
	planPath := filepath.Join(e.repoPath, "SESSION_PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) && lastContent != "" {
		extracted := extractPlan(lastContent)
		if extracted != "" {
			if err := os.WriteFile(planPath, []byte(extracted), 0o644); err == nil {
				e.logger.Info("extracted SESSION_PLAN.md from agent output")
			}
		} else {
			// Accept ANY output as a plan — the model may not follow exact format
			if err := os.WriteFile(planPath, []byte(lastContent), 0o644); err == nil {
				e.logger.Info("wrote SESSION_PLAN.md from raw agent output", "len", len(lastContent))
			}
		}
	}

	// If model returned nothing, write a minimal fallback plan
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		e.logger.Info("model returned empty output — writing fallback plan")
		fallback := fmt.Sprintf(`## Session Plan

Session Title: Day %s evolution — code quality and reliability

### Task 1: Fix error handling gaps
Files: cmd/iterate/, internal/
Description: Find functions that ignore errors. Add proper error handling with descriptive messages.

### Task 2: Add missing tests
Files: internal/
Description: Find exported functions without corresponding tests. Write at least one test per function.

### Task 3: Clean up code smells
Files: cmd/iterate/, internal/
Description: Look for defer in loops, unused variables/imports, hardcoded values. Fix one issue.
`, day)
		if err := os.WriteFile(planPath, []byte(fallback), 0o644); err != nil {
			return fmt.Errorf("failed to write fallback plan: %w", err)
		}
		e.logger.Info("wrote fallback SESSION_PLAN.md")
	}

	return nil
}

// extractPlan tries multiple patterns to extract a plan from agent text output.
func extractPlan(output string) string {
	prefixes := []string{
		"## Session Plan", "## Session plan", "# Session Plan", "## Plan",
		"# Plan", "## SESSION PLAN", "# SESSION PLAN",
		"Session Title:", "### Task 1", "## Task 1",
	}
	for _, prefix := range prefixes {
		if idx := strings.Index(output, prefix); idx >= 0 {
			return strings.TrimSpace(output[idx:])
		}
	}
	// Accept any output with task-like structure
	if strings.Contains(output, "Task") && (strings.Contains(output, "Files:") || strings.Contains(output, "Description:")) {
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
	if _, err := e.runTool(ctx, "bash", map[string]interface{}{
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
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("task goroutine panicked", "number", task.Number, "panic", r)
					results[i] = taskResult{task: task, success: false}
				}
			}()
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
			if _, err := e.runTool(ctx, "bash", map[string]interface{}{
				"cmd": fmt.Sprintf("git cherry-pick %s", hash),
			}); err != nil {
				e.logger.Warn("cherry-pick failed, aborting task commits", "number", r.task.Number, "hash", hash, "err", err)
				_, _ = e.runTool(ctx, "bash", map[string]interface{}{"cmd": "git cherry-pick --abort 2>/dev/null || true"})
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
	_, _ = e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git worktree remove --force %q 2>/dev/null; git branch -D %q 2>/dev/null; true", worktreeDir, branchName),
	})

	// Record base commit before task so we can collect new commits afterward.
	baseHash, err := e.runTool(ctx, "bash", map[string]interface{}{"cmd": "git rev-parse HEAD"})
	if err != nil {
		e.logger.Warn("failed to get base commit hash", "err", err)
		return nil, false
	}
	baseHash = strings.TrimSpace(baseHash)

	// Create the worktree branched at current HEAD.
	if out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git worktree add -b %q %q HEAD", branchName, worktreeDir),
	}); err != nil {
		e.logger.Warn("failed to create worktree", "task", task.Number, "err", err, "output", out)
		return nil, false
	}

	// Always remove the worktree when done.
	defer func() {
		_, _ = e.runTool(context.Background(), "bash", map[string]interface{}{
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
	out, err := subEngine.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git log %s..HEAD --format=%%H --reverse", baseHash),
	})

	// If no commits, check if there are uncommitted changes and commit them
	if err != nil || strings.TrimSpace(out) == "" {
		e.logger.Info("no commits found, checking for uncommitted changes", "number", task.Number)

		// Check for any changes
		statusOut, _ := subEngine.runTool(ctx, "bash", map[string]interface{}{
			"cmd": "git status --short",
		})

		if strings.TrimSpace(statusOut) != "" {
			// There are changes - commit them
			e.logger.Info("found uncommitted changes, auto-committing", "number", task.Number, "status", statusOut)
			commitOut, commitErr := subEngine.runTool(ctx, "bash", map[string]interface{}{
				"cmd": "git add -A && git commit -m 'task: auto-commit changes from agent'",
			})
			if commitErr != nil {
				e.logger.Warn("auto-commit failed", "number", task.Number, "err", commitErr, "output", commitOut)
				return nil, false
			}

			// Now get the commit
			out, err = subEngine.runTool(ctx, "bash", map[string]interface{}{
				"cmd": fmt.Sprintf("git log %s..HEAD --format=%%H --reverse", baseHash),
			})
			if err != nil || strings.TrimSpace(out) == "" {
				e.logger.Warn("still no commits after auto-commit", "number", task.Number)
				return nil, false
			}
		} else {
			e.logger.Warn("no new commits from task", "number", task.Number)
			return nil, false
		}
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
// Uses Aider-style SEARCH/REPLACE format for better reliability.
func (e *Engine) runTaskAttempt(ctx context.Context, p iteragent.Provider, task planTask, systemPrompt string, tools []iteragent.Tool, skills *iteragent.SkillSet, protectedWarning, extraContext string) (bool, string) {
	// Use Aider-style aggressive prompt
	userMsg := BuildUserMessageAider(e.repoPath, "", "", "CODE")

	// Add task-specific context
	userMsg += fmt.Sprintf("\n\n## YOUR TASK\n\nTask %d: %s\n\n%s\n\n%s\n\n",
		task.Number, task.Description, protectedWarning, extraContext)

	if extraContext == "" {
		userMsg = strings.Replace(userMsg, "\n\n\n", "\n\n", 1)
	}

	a := e.newAgent(p, tools, systemPrompt, skills)
	var outputBuilder strings.Builder
	var taskOutput string
	var taskErr error

	for ev := range a.Prompt(ctx, userMsg) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}

		if ev.Type == string(iteragent.EventMessageUpdate) {
			outputBuilder.WriteString(ev.Content)
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			taskOutput = ev.Content
		}
		if ev.Type == string(iteragent.EventError) {
			taskErr = fmt.Errorf("%s", ev.Content)
		}
	}
	a.Finish()

	// Use accumulated content or final content, whichever is longer
	accumulatedOutput := outputBuilder.String()
	if len(accumulatedOutput) > len(taskOutput) {
		taskOutput = accumulatedOutput
	}

	if taskErr != nil {
		e.logger.Warn("task error", "number", task.Number, "err", taskErr)
		_ = e.revert(ctx)
		return false, fmt.Sprintf("Agent error: %s", taskErr)
	}

	// CRITICAL: Parse and apply unified diffs (like git diff)
	diffs := ParseUnifiedDiffs(taskOutput)

	// Debug: log what we received
	sampleLen := 500
	if len(taskOutput) < sampleLen {
		sampleLen = len(taskOutput)
	}
	e.logger.Info("Agent output analysis", "number", task.Number,
		"output_length", len(taskOutput),
		"diffs_found", len(diffs),
		"has_tool_calls", strings.Contains(taskOutput, "tool"),
		"sample_output", strings.ReplaceAll(taskOutput[:sampleLen], "\n", "\\n"))

	// FALLBACK: If no unified diffs, try to parse tool_call JSON output
	if len(diffs) == 0 {
		e.logger.Info("No unified diffs found, trying tool_call JSON fallback", "number", task.Number)
		modifiedFiles, toolErr := e.applyToolCallChanges(ctx, taskOutput)
		if toolErr != nil {
			e.logger.Error("Tool call fallback also failed", "number", task.Number, "err", toolErr)
			_ = e.revert(ctx)
			return false, fmt.Sprintf("CRITICAL FAILURE: Neither unified diffs nor tool calls found.\n\nUnified diffs format:\n--- a/path/to/file.go\n+++ b/path/to/file.go\n@@ ... @@\n-old code\n+new code\n\nOr use write_file tool calls in JSON format.")
		}

		// Verify the tool call changes worked
		if len(modifiedFiles) == 0 {
			e.logger.Error("No files modified via tool calls", "number", task.Number)
			_ = e.revert(ctx)
			return false, "CRITICAL FAILURE: Agent did not produce any file changes."
		}

		e.logger.Info("Applied changes via tool call fallback", "number", task.Number, "files", len(modifiedFiles), "modified", modifiedFiles)

		// Verify build/tests pass
		v := e.verify(ctx)
		if !v.BuildPassed || !v.TestPassed {
			errCtx := fmt.Sprintf("Build passed: %v, Test passed: %v.\n\nOutput:\n%s", v.BuildPassed, v.TestPassed, v.Output)
			e.logger.Warn("verification failed, reverting", "number", task.Number, "build", v.BuildPassed, "test", v.TestPassed)
			_ = e.revert(ctx)
			return false, errCtx
		}

		// Verify actual code changes
		if !e.hasCodeChanges(ctx) {
			e.logger.Error("no code changes detected", "number", task.Number)
			_ = e.revert(ctx)
			return false, "FAILURE: Only updated documentation/stats. You MUST modify .go source files."
		}

		if !e.hasTestChanges(ctx) {
			e.logger.Error("code changes without tests", "number", task.Number)
			_ = e.revert(ctx)
			return false, "FAILURE: Code changes MUST include tests. Write *_test.go files for your changes."
		}

		if err := e.appendLearningJSONL("tool_call_fallback", "evolution", task.Description, ""); err != nil {
			e.logger.Warn("failed to record task learning", "task", task.Title, "err", err)
		}
		return true, ""
	}

	// Skip validation - just try to apply whatever diffs we found
	// Apply the diffs
	modifiedFiles, applyErr := e.ApplyUnifiedDiffs(diffs)
	if applyErr != nil {
		e.logger.Error("Failed to apply unified diffs", "number", task.Number, "err", applyErr)
		_ = e.revert(ctx)
		return false, fmt.Sprintf("Failed to apply changes: %v", applyErr)
	}

	e.logger.Info("Applied unified diffs", "number", task.Number, "files", len(modifiedFiles), "modified", modifiedFiles)

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

	// CRITICAL: Verify actual code changes were made (not just docs/stats)
	if !e.hasCodeChanges(ctx) {
		e.logger.Error("no code changes detected - only docs/stats updated", "number", task.Number)
		_ = e.revert(ctx)
		return false, "FAILURE: Only updated documentation/stats. You MUST modify .go source files."
	}

	// CRITICAL: Verify tests were added for code changes
	if !e.hasTestChanges(ctx) {
		e.logger.Error("code changes without tests", "number", task.Number, "files", e.getModifiedFiles(ctx))
		_ = e.revert(ctx)
		return false, "FAILURE: Code changes MUST include tests. Write *_test.go files for your changes."
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

// applyToolCallChanges parses tool_call JSON output and applies file changes
func (e *Engine) applyToolCallChanges(ctx context.Context, output string) ([]string, error) {
	var modifiedFiles []string

	// Strategy 1: Parse JSON tool calls like {"tool":"write_file","args":{"path":"...","content":"..."}}
	// Also handles: tool_call, tool_calls, or assistant messages with tool calls

	// Find all JSON objects in the output - more flexible regex
	matches := jsonToolCallRe.FindAllString(output, -1)

	for _, match := range matches {
		// Validate it's proper JSON before processing
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(match), &raw); err != nil {
			continue
		}
		// Try to parse as tool call
		var toolCall map[string]interface{}
		if err := json.Unmarshal([]byte(match), &toolCall); err != nil {
			continue
		}

		// Get tool name from various possible fields
		toolName := ""
		for _, key := range []string{"tool", "name", "function", "action", "type"} {
			if v, ok := toolCall[key]; ok {
				if s, ok := v.(string); ok {
					toolName = strings.ToLower(s)
					break
				}
			}
		}

		if toolName == "" {
			continue
		}

		// Get args from various possible fields
		var args map[string]interface{}
		for _, key := range []string{"args", "arguments", "input", "parameters"} {
			if v, ok := toolCall[key]; ok {
				if m, ok := v.(map[string]interface{}); ok {
					args = m
					break
				}
				// Handle string args
				if s, ok := v.(string); ok {
					var parsed map[string]interface{}
					if json.Unmarshal([]byte(s), &parsed) == nil {
						args = parsed
						break
					}
				}
			}
		}

		if args == nil {
			continue
		}

		// Handle write_file tool (various name formats)
		if toolName == "write_file" || toolName == "write" || toolName == "writefile" || toolName == "file_write" {
			path, ok := args["path"].(string)
			if !ok {
				// Try file or filename
				if p, ok := args["file"].(string); ok {
					path = p
				} else if p, ok := args["filename"].(string); ok {
					path = p
				}
			}
			if path == "" {
				continue
			}

			content, ok := args["content"].(string)
			if !ok {
				// Try body or text
				if c, ok := args["body"].(string); ok {
					content = c
				} else if c, ok := args["text"].(string); ok {
					content = c
				}
			}
			if content == "" {
				continue
			}

			// Apply the file write
			fullPath := filepath.Join(e.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				e.logger.Warn("failed to create directory", "path", fullPath, "err", err)
				continue
			}

			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				e.logger.Warn("failed to write file", "path", fullPath, "err", err)
				continue
			}

			modifiedFiles = append(modifiedFiles, path)
			e.logger.Info("Applied write_file via tool call", "file", path)
		}

		// Handle edit_file tool
		if toolName == "edit_file" || toolName == "edit" || toolName == "editfile" || toolName == "file_edit" || toolName == "str_replace_editor" {
			path, ok := args["path"].(string)
			if !ok {
				if p, ok := args["file"].(string); ok {
					path = p
				}
			}
			if path == "" {
				continue
			}

			// Try to extract old/new content
			var oldContent, newContent string
			for _, oldKey := range []string{"old_string", "old", "before", "search"} {
				if v, ok := args[oldKey]; ok {
					if s, ok := v.(string); ok {
						oldContent = s
						break
					}
				}
			}
			for _, newKey := range []string{"new_string", "new", "after", "replace"} {
				if v, ok := args[newKey]; ok {
					if s, ok := v.(string); ok {
						newContent = s
						break
					}
				}
			}

			if oldContent == "" || newContent == "" {
				continue
			}

			// Apply the edit
			fullPath := filepath.Join(e.repoPath, path)
			existing, err := os.ReadFile(fullPath)
			if err != nil {
				e.logger.Warn("failed to read file for edit", "path", fullPath, "err", err)
				continue
			}

			// Simple string replace
			replacedContent := strings.Replace(string(existing), oldContent, newContent, 1)
			if string(existing) == replacedContent {
				e.logger.Warn("edit_file: content unchanged", "path", fullPath)
				continue
			}

			if err := os.WriteFile(fullPath, []byte(replacedContent), 0644); err != nil {
				e.logger.Warn("failed to write file after edit", "path", fullPath, "err", err)
				continue
			}

			modifiedFiles = append(modifiedFiles, path)
			e.logger.Info("Applied edit_file via tool call", "file", path)
		}
	}

	// Strategy 2: Look for file paths with content after them (markdown style)
	contentMatches := fileContentRe.FindAllStringSubmatch(output, -1)
	for _, match := range contentMatches {
		if len(match) > 2 {
			path := match[1]
			content := match[2]

			// Skip if already modified
			alreadyModified := false
			for _, f := range modifiedFiles {
				if f == path {
					alreadyModified = true
					break
				}
			}
			if alreadyModified {
				continue
			}

			fullPath := filepath.Join(e.repoPath, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				continue
			}

			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				continue
			}

			modifiedFiles = append(modifiedFiles, path)
			e.logger.Info("Applied file from code block", "file", path)
		}
	}

	return modifiedFiles, nil
}
