package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RunPlanPhase runs the planning phase. Creates SESSION_PLAN.md via agent or fallback.
func (e *Engine) RunPlanPhase(ctx context.Context, p iteragent.Provider, issues string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	identity, journal, day := readPlanContext(e.repoPath)
	userMessage := buildPlanPrompt(e.repoPath, string(journal), day, issues)

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})
	a := e.newAgent(p, tools, systemPrompt, skills)

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

	var sb strings.Builder
	appendPlanInstructions(&sb, ciStatus, day)
	appendPlanContext(&sb, learnings, journal, issues)
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
	ctx, cancel := withTimeout(ctx)
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

	systemPrompt, tools, skills := e.loadImplementContext()

	protectedWarning := "\n\nPROTECTED FILES — DO NOT EDIT:\n- internal/evolution/*.go\n- .github/workflows/*.yml\n- cmd/iterate/main.go\n- scripts/evolution/evolve.sh\n\nIf a task requires editing these, skip it.\n"

	for _, task := range tasks {
		e.logger.Info("implementing task", "number", task.Number, "title", task.Title)
		e.executeTask(ctx, p, task, systemPrompt, tools, skills, protectedWarning)
	}

	// Commit any remaining changes
	if _, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git add -A && git diff --cached --quiet || git commit -m 'iterate: implement session changes'",
	}); err != nil {
		e.logger.Warn("final commit failed", "err", err)
	}

	return nil
}

// executeTask runs a single task, reverts on failure.
func (e *Engine) executeTask(ctx context.Context, p iteragent.Provider, task planTask, systemPrompt string, tools []iteragent.Tool, skills *iteragent.SkillSet, protectedWarning string) {
	userMsg := fmt.Sprintf("Implement Task %d: %s\n\n%s\n\nAfter implementing, run: go build ./... && go test ./...\nThen commit your changes.", task.Number, task.Description, protectedWarning)

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
		e.logger.Warn("task failed, reverting", "number", task.Number, "err", taskErr)
		_ = e.revert(ctx)
		return
	}

	if violations, _ := e.verifyProtected(ctx); len(violations) > 0 {
		e.logger.Warn("protected files modified, reverting", "number", task.Number, "files", violations)
		_ = e.revert(ctx)
		return
	}

	v := e.verify(ctx)
	if !v.BuildPassed || !v.TestPassed {
		e.logger.Warn("verification failed, reverting", "number", task.Number, "build", v.BuildPassed, "test", v.TestPassed)
		_ = e.revert(ctx)
		return
	}

	_ = e.appendLearningJSONL(firstLine(extractCommitMessage(taskOutput)), "evolution", task.Description, "")
	_ = taskOutput
}

// loadImplementContext prepares system prompt, tools, and skills for implementation.
func (e *Engine) loadImplementContext() (string, []iteragent.Tool, *iteragent.SkillSet) {
	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})
	return systemPrompt, tools, skills
}
