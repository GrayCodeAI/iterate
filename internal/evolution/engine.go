package evolution

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/session"
)

// Engine runs one evolution session.
type Engine struct {
	repoPath      string
	store         *session.Store
	logger        *slog.Logger
	eventSink     chan<- iteragent.Event
	thinkingLevel iteragent.ThinkingLevel
}

// New creates a new evolution engine.
func New(repoPath string, store *session.Store, logger *slog.Logger) *Engine {
	return &Engine{
		repoPath: repoPath,
		store:    store,
		logger:   logger,
	}
}

// WithEventSink sets a channel that receives live agent events during evolution.
// Used by the web dashboard to stream real-time activity to connected clients.
func (e *Engine) WithEventSink(sink chan<- iteragent.Event) *Engine {
	e.eventSink = sink
	return e
}

// WithThinking sets the extended thinking level for all agents spawned by this engine.
func (e *Engine) WithThinking(level iteragent.ThinkingLevel) *Engine {
	e.thinkingLevel = level
	return e
}

// forwardEvents drains the given event channel and forwards each event to eventSink.
// It returns when the source channel is closed.
func (e *Engine) forwardEvents(src <-chan iteragent.Event) {
	if e.eventSink == nil {
		// Drain the channel so the agent goroutine doesn't block.
		for range src {
		}
		return
	}
	for ev := range src {
		select {
		case e.eventSink <- ev:
		default:
		}
	}
}

// Run executes one full evolution session.
// It reads the repo, assesses it, implements improvements, tests, and commits or reverts.
func (e *Engine) Run(ctx context.Context, p iteragent.Provider, issues string) (*session.Session, error) {
	sess := &session.Session{
		StartedAt: time.Now(),
		Provider:  p.Name(),
		Status:    "running",
	}

	// Save initial session state
	if err := e.store.Save(sess); err != nil {
		e.logger.Warn("failed to save initial session", "err", err)
	}

	// Build context for the agent
	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "IDENTITY.md"))
	journal, _ := os.ReadFile(filepath.Join(e.repoPath, "JOURNAL.md"))

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	userMessage := buildUserMessage(e.repoPath, string(journal), issues)

	// Create agent with all tools - builder pattern like yoyo-evolve
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})

	a := e.newAgent(p, tools, systemPrompt, skills)

	// Stream events to the web dashboard if a sink is configured.
	eventCh := a.Prompt(ctx, userMessage)
	var output string
	var runErr error
	for ev := range eventCh {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			output = ev.Content
		}
		if ev.Type == string(iteragent.EventError) {
			runErr = fmt.Errorf("%s", ev.Content)
		}
	}
	a.Finish()

	sess.FinishedAt = time.Now()
	sess.RawOutput = output

	if runErr != nil {
		sess.Status = "error"
		sess.Error = runErr.Error()
		e.appendJournal(sess, false)
		_ = e.store.Save(sess)
		return sess, runErr
	}

	// Run tests to verify changes
	testResult, testErr := e.runTests(ctx)
	sess.TestOutput = testResult

	if testErr != nil {
		// Tests failed — revert all changes
		e.logger.Info("tests failed, reverting changes")
		sess.Status = "reverted"
		_ = e.revert(ctx)
		e.appendJournal(sess, false)
	} else {
		// Tests passed — commit
		e.logger.Info("tests passed, committing changes")
		sess.Status = "committed"
		commitMsg := extractCommitMessage(output)
		if err := e.commit(ctx, commitMsg); err != nil {
			sess.Status = "commit_failed"
			sess.Error = err.Error()
		} else {
			// Write learning entry after successful commit
			title := firstLine(commitMsg)
			_ = e.appendLearningJSONL(title, "evolution", buildUserMessage(e.repoPath, "", issues), "")
		}
		e.appendJournal(sess, true)
	}

	_ = e.store.Save(sess)
	return sess, nil
}

// RunPlanPhase runs only the planning phase: reads source, JOURNAL.md, issues,
// writes SESSION_PLAN.md and commits it.
func (e *Engine) RunPlanPhase(ctx context.Context, p iteragent.Provider, issues string) error {
	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "IDENTITY.md"))
	journal, _ := os.ReadFile(filepath.Join(e.repoPath, "JOURNAL.md"))
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	day := strings.TrimSpace(string(dayCount))

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))

	var sb strings.Builder
	sb.WriteString("## Phase: Planning\n\n")
	sb.WriteString("Read your source code (cmd/, internal/), JOURNAL.md, and ISSUES_TODAY.md.\n")
	sb.WriteString("Then write SESSION_PLAN.md with EXACTLY this format:\n\n")
	sb.WriteString("## Session Plan\n\n")
	sb.WriteString("### Task 1: [title]\n")
	sb.WriteString("Files: [files to modify]\n")
	sb.WriteString("Description: [what to do]\n")
	sb.WriteString("Issue: #N (or \"none\")\n\n")
	sb.WriteString("### Task 2: ...\n\n")
	sb.WriteString("### Issue Responses\n")
	sb.WriteString("- #N: implement — [reason]\n")
	sb.WriteString("- #N: wontfix — [reason]\n\n")
	sb.WriteString("After writing SESSION_PLAN.md, commit it:\n")
	sb.WriteString(fmt.Sprintf("git add SESSION_PLAN.md && git commit -m \"Day %s: session plan\"\n\n", day))
	sb.WriteString("Then STOP. Do not implement anything.\n\n")

	if len(string(journal)) > 0 {
		recent := string(journal)
		if len(recent) > 500 {
			recent = "...\n" + recent[len(recent)-500:]
		}
		sb.WriteString("## Recent journal\n\n")
		sb.WriteString(recent)
		sb.WriteString("\n\n")
	}
	if len(issues) > 0 {
		sb.WriteString("## Community input\n\n")
		sb.WriteString(issues)
		sb.WriteString("\n")
	}

	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})
	a := e.newAgent(p, tools, systemPrompt, skills)

	e.forwardEvents(a.Prompt(ctx, sb.String()))
	a.Finish()
	return nil
}

// RunImplementPhase reads SESSION_PLAN.md and runs one agent per task.
func (e *Engine) RunImplementPhase(ctx context.Context, p iteragent.Provider) error {
	planPath := filepath.Join(e.repoPath, "SESSION_PLAN.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("SESSION_PLAN.md not found: %w", err)
	}
	plan := string(planBytes)

	tasks := parseSessionPlanTasks(plan)
	if len(tasks) == 0 {
		e.logger.Warn("no tasks found in SESSION_PLAN.md, running single-shot")
		tasks = []planTask{{Number: 1, Title: "Self-improvement", Description: plan}}
	}

	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "IDENTITY.md"))
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})

	for _, task := range tasks {
		e.logger.Info("implementing task", "number", task.Number, "title", task.Title)

		userMsg := fmt.Sprintf("Your ONLY job: implement Task %d from SESSION_PLAN.md and commit.\n\n%s\n\nAfter implementing, run: go fmt && go vet && go build ./... && go test ./...\nThen commit your changes.",
			task.Number, task.Description)

		a := iteragent.New(p, tools, e.logger).
			WithSystemPrompt(systemPrompt).
			WithSkillSet(skills)

		var taskOutput string
		var taskErr error
		for ev := range a.Prompt(ctx, userMsg) {
			if e.eventSink != nil {
				select {
				case e.eventSink <- ev:
				default:
				}
			}
			if ev.Type == string(iteragent.EventMessageEnd) {
				taskOutput = ev.Content
			}
			if ev.Type == string(iteragent.EventError) {
				taskErr = fmt.Errorf("%s", ev.Content)
			}
		}
		a.Finish()

		if taskErr != nil {
			e.logger.Warn("task failed", "number", task.Number, "err", taskErr)
			continue
		}

		// Write learning for each completed task
		commitMsg := extractCommitMessage(taskOutput)
		_ = e.appendLearningJSONL(firstLine(commitMsg), "evolution", task.Description, "")
	}
	return nil
}

// RunCommunicatePhase parses Issue Responses from SESSION_PLAN.md and posts them.
func (e *Engine) RunCommunicatePhase(ctx context.Context, p iteragent.Provider) error {
	planPath := filepath.Join(e.repoPath, "SESSION_PLAN.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		// SESSION_PLAN.md missing is not fatal for communicate phase
		e.logger.Warn("SESSION_PLAN.md not found, skipping communicate phase")
		return nil
	}
	plan := string(planBytes)

	// Parse Issue Responses section
	responses := parseIssueResponses(plan)
	if len(responses) == 0 {
		e.logger.Info("no issue responses in SESSION_PLAN.md")
		return nil
	}

	identity, _ := os.ReadFile(filepath.Join(e.repoPath, "IDENTITY.md"))
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})

	for _, resp := range responses {
		userMsg := fmt.Sprintf("Post a GitHub issue comment on issue #%d.\nStatus: %s\nReason: %s\n\nUse the gh CLI: gh issue comment %d --repo . --body \"...\"\nKeep the comment brief and friendly.",
			resp.IssueNum, resp.Status, resp.Reason, resp.IssueNum)

		a := iteragent.New(p, tools, e.logger).
			WithSystemPrompt(systemPrompt).
			WithSkillSet(skills)

		e.forwardEvents(a.Prompt(ctx, userMsg))
		a.Finish()
	}
	return nil
}

// WriteLearningsToMemory is the public entry point for the synthesize workflow.
// It writes a generic learning entry for the current session.
func (e *Engine) WriteLearningsToMemory(title, context, takeaway string) error {
	return e.appendLearningJSONL(title, "evolution", context, takeaway)
}

// appendLearningJSONL appends a lesson entry to memory/learnings.jsonl.
func (e *Engine) appendLearningJSONL(title, source, context, takeaway string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	// Read current day number
	dayBytes, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	day, _ := strconv.Atoi(strings.TrimSpace(string(dayBytes)))

	entry := map[string]interface{}{
		"type":     "lesson",
		"day":      day,
		"ts":       time.Now().UTC().Format(time.RFC3339),
		"source":   source,
		"title":    title,
		"context":  context,
		"takeaway": takeaway,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal learning: %w", err)
	}

	path := filepath.Join(memDir, "learnings.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open learnings.jsonl: %w", err)
	}
	defer f.Close()

	_, err = f.Write(append(line, '\n'))
	return err
}

// newAgent creates a configured agent with the engine's thinking level applied.
func (e *Engine) newAgent(p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet) *iteragent.Agent {
	a := e.newAgent(p, tools, systemPrompt, skills)
	if e.thinkingLevel != "" && e.thinkingLevel != iteragent.ThinkingLevelOff {
		a = a.WithThinkingLevel(e.thinkingLevel)
	}
	return a
}

func buildSystemPrompt(repoPath, identity string) string {
	personality, _ := os.ReadFile(filepath.Join(repoPath, "PERSONALITY.md"))

	skills, _ := iteragent.LoadSkills([]string{filepath.Join(repoPath, "skills")})
	skillsPrompt := skills.FormatForPrompt()

	return fmt.Sprintf(`You are iterate, a self-evolving coding agent written in Go.

## Identity
%s

## Personality
%s
%s

## Tool call format
Wrap tool calls in triple backtick blocks:
`+"```"+`tool
{"tool":"tool_name","args":{"key":"value"}}
`+"```",
		identity,
		string(personality),
		skillsPrompt,
	)
}

func buildUserMessage(repoPath, journal, issues string) string {
	var sb strings.Builder

	sb.WriteString("## Your task\n\n")
	sb.WriteString("Assess your codebase, find one meaningful improvement, implement it, test it, and commit it.\n\n")

	// List files for context
	sb.WriteString("Start by listing your files with list_files, then read relevant source files.\n\n")

	// Recent journal context (last 500 chars)
	if len(journal) > 0 {
		recent := journal
		if len(journal) > 500 {
			recent = "...\n" + journal[len(journal)-500:]
		}
		sb.WriteString("## Recent journal\n\n")
		sb.WriteString(recent)
		sb.WriteString("\n\n")
	}

	// Community issues
	if len(issues) > 0 {
		sb.WriteString("## Community input\n\n")
		sb.WriteString(issues)
		sb.WriteString("\n")
	}

	sb.WriteString("Begin your self-assessment now.")
	return sb.String()
}

func (e *Engine) runTests(ctx context.Context) (string, error) {
	tools := iteragent.DefaultTools(e.repoPath)
	tm := iteragent.ToolMap(tools)
	return tm["run_tests"].Execute(ctx, nil)
}

func (e *Engine) revert(ctx context.Context) error {
	tools := iteragent.DefaultTools(e.repoPath)
	tm := iteragent.ToolMap(tools)
	_, err := tm["git_revert"].Execute(ctx, nil)
	return err
}

func (e *Engine) commit(ctx context.Context, msg string) error {
	tools := iteragent.DefaultTools(e.repoPath)
	tm := iteragent.ToolMap(tools)
	_, err := tm["git_commit"].Execute(ctx, map[string]string{"message": msg})
	return err
}

func (e *Engine) appendJournal(sess *session.Session, success bool) {
	path := filepath.Join(e.repoPath, "JOURNAL.md")

	// Read current day count from DAY_COUNT file.
	dayCount := 0
	if data, err := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT")); err == nil {
		dayCount, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	// Build a concise title from the session output.
	title := extractJournalTitle(sess.RawOutput, success)

	// Build body: summary from agent output, stripped of raw tool calls.
	body := buildJournalBody(sess)

	entry := fmt.Sprintf("\n## Day %d — %s — %s\n\n%s\n",
		dayCount,
		sess.StartedAt.Format("15:04"),
		title,
		body,
	)

	// Prepend to keep newest entries at top (after the "# Journal" header).
	existing, _ := os.ReadFile(path)
	header := "# Journal\n"
	rest := strings.TrimPrefix(string(existing), header)
	newContent := header + entry + rest

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		e.logger.Warn("failed to write journal", "err", err)
	}
}

// extractJournalTitle picks a short title from agent output.
func extractJournalTitle(output string, success bool) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Look for conventional commit prefixes.
		for _, prefix := range []string{"feat:", "fix:", "refactor:", "chore:", "docs:", "test:"} {
			if strings.HasPrefix(strings.ToLower(line), prefix) && len(line) < 80 {
				return line
			}
		}
	}
	if success {
		return "evolution session"
	}
	return "session (no changes committed)"
}

// buildJournalBody produces a clean body for the journal entry.
func buildJournalBody(sess *session.Session) string {
	duration := sess.FinishedAt.Sub(sess.StartedAt).Round(time.Second)
	lines := []string{fmt.Sprintf("Provider: %s · Duration: %s", sess.Provider, duration)}

	// Include up to 3 lines of meaningful agent output (not tool calls).
	count := 0
	for _, line := range strings.Split(sess.RawOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			continue
		}
		if len(line) > 20 && len(line) < 200 {
			lines = append(lines, line)
			count++
			if count >= 3 {
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractCommitMessage(output string) string {
	// Look for a line starting with "commit:" or "feat:" or "fix:" etc.
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"commit:", "feat:", "fix:", "refactor:", "chore:", "docs:"} {
			if strings.HasPrefix(strings.ToLower(line), prefix) {
				return line
			}
		}
	}
	return fmt.Sprintf("iterate: session %s", time.Now().Format("2006-01-02"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// planTask holds a single task parsed from SESSION_PLAN.md.
type planTask struct {
	Number      int
	Title       string
	Description string
}

// parseSessionPlanTasks extracts tasks from SESSION_PLAN.md content.
func parseSessionPlanTasks(plan string) []planTask {
	var tasks []planTask
	scanner := bufio.NewScanner(strings.NewReader(plan))

	var current *planTask
	var descLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Detect "### Task N: title"
		if strings.HasPrefix(line, "### Task ") {
			// Save previous task
			if current != nil {
				current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
				tasks = append(tasks, *current)
			}
			// Parse new task header
			rest := strings.TrimPrefix(line, "### Task ")
			var num int
			var title string
			if idx := strings.IndexByte(rest, ':'); idx >= 0 {
				fmt.Sscanf(rest[:idx], "%d", &num)
				title = strings.TrimSpace(rest[idx+1:])
			} else {
				fmt.Sscanf(rest, "%d", &num)
				title = rest
			}
			current = &planTask{Number: num, Title: title}
			descLines = []string{line}
			continue
		}

		// Stop collecting at Issue Responses section
		if strings.HasPrefix(line, "### Issue Responses") || strings.HasPrefix(line, "### Issue responses") {
			if current != nil {
				current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
				tasks = append(tasks, *current)
				current = nil
			}
			break
		}

		if current != nil {
			descLines = append(descLines, line)
		}
	}

	// Save last task
	if current != nil {
		current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
		tasks = append(tasks, *current)
	}

	return tasks
}

// issueResponse holds a parsed issue response from SESSION_PLAN.md.
type issueResponse struct {
	IssueNum int
	Status   string
	Reason   string
}

// parseIssueResponses extracts Issue Responses lines from SESSION_PLAN.md.
func parseIssueResponses(plan string) []issueResponse {
	var responses []issueResponse
	inSection := false

	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "### Issue Responses") || strings.HasPrefix(line, "### Issue responses") {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "### ") {
			break
		}
		if inSection && strings.HasPrefix(line, "- #") {
			// Format: - #N: status — reason
			rest := strings.TrimPrefix(line, "- #")
			var num int
			fmt.Sscanf(rest, "%d", &num)
			if num == 0 {
				continue
			}
			status := "comment"
			reason := rest
			if strings.Contains(rest, "wontfix") {
				status = "wontfix"
			} else if strings.Contains(rest, "implement") {
				status = "implement"
			} else if strings.Contains(rest, "partial") {
				status = "partial"
			}
			if idx := strings.Index(rest, "—"); idx >= 0 {
				reason = strings.TrimSpace(rest[idx+len("—"):])
			} else if idx := strings.Index(rest, "--"); idx >= 0 {
				reason = strings.TrimSpace(rest[idx+2:])
			}
			responses = append(responses, issueResponse{IssueNum: num, Status: status, Reason: reason})
		}
	}
	return responses
}
