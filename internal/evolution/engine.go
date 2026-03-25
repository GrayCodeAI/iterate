package evolution

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ProtectedFiles defines patterns for files the agent MUST NOT edit during evolution.
// These are core infrastructure that could break the evolution system itself.
var ProtectedFiles = []string{
	// Evolution engine - modifying this could break self-evolution
	"internal/evolution/engine.go",
	"internal/evolution/*.go",
	// GitHub Actions workflows - could disable evolution triggers
	".github/workflows/evolve.yml",
	".github/workflows/*.yml",
	// Core REPL - the agent's interface
	"cmd/iterate/repl.go",
	"cmd/iterate/main.go",
	// Configuration
	".iterate/config.json",
	// Scripts that run evolution
	"scripts/evolution/evolve.sh",
	"scripts/social/social.sh",
}

// isProtected checks if a file path matches any protected pattern.
func isProtected(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, pattern := range ProtectedFiles {
		// Check exact match
		if cleanPath == filepath.Clean(pattern) {
			return true
		}
		// Check glob pattern match
		if matched, _ := filepath.Match(pattern, filepath.Base(cleanPath)); matched {
			dir := filepath.Dir(cleanPath)
			patternDir := filepath.Dir(pattern)
			if dir == patternDir || patternDir == "." {
				return true
			}
		}
		// Check if path is inside a protected directory
		if strings.HasSuffix(pattern, "/*") || strings.HasSuffix(pattern, "/*.go") {
			protectedDir := strings.TrimSuffix(pattern, "/*")
			protectedDir = strings.TrimSuffix(protectedDir, "/*.go")
			if strings.HasPrefix(cleanPath, protectedDir+"/") {
				return true
			}
		}
	}
	return false
}

// Engine runs one evolution session.
type Engine struct {
	repoPath      string
	repo          string
	logger        *slog.Logger
	eventSink     chan<- iteragent.Event
	thinkingLevel iteragent.ThinkingLevel
	prNumber      int
	prURL         string
	branchName    string
	traceID       string
}

// generateTraceID creates a random hex trace ID for request correlation.
func generateTraceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// RunResult holds the outcome of a completed evolution run.
type RunResult struct {
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	PRNumber   int
	PRURL      string
}

// PRState persists PR info between phases (evolve.sh runs phases as separate CLI invocations).
type PRState struct {
	PRNumber int    `json:"pr_number"`
	PRURL    string `json:"pr_url"`
	Branch   string `json:"branch"`
}

const prStateFile = ".iterate/pr_state.json"

// New creates a new evolution engine.
func New(repoPath string, logger *slog.Logger) *Engine {
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = "GrayCodeAI/iterate"
	}
	tid := generateTraceID()
	e := &Engine{
		repoPath: repoPath,
		repo:     repo,
		logger:   logger.With("traceID", tid),
		traceID:  tid,
	}
	// Load PR state from previous phase if exists
	e.loadPRState()
	return e
}

// TraceID returns the trace ID for this engine instance.
func (e *Engine) TraceID() string {
	return e.traceID
}

// savePRState persists PR info to file for cross-phase communication.
func (e *Engine) savePRState() error {
	if e.prNumber == 0 {
		return nil
	}
	path := filepath.Join(e.repoPath, prStateFile)
	data, err := json.Marshal(PRState{
		PRNumber: e.prNumber,
		PRURL:    e.prURL,
		Branch:   e.branchName,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// loadPRState restores PR info from file at engine creation.
func (e *Engine) loadPRState() {
	path := filepath.Join(e.repoPath, prStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state PRState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	e.prNumber = state.PRNumber
	e.prURL = state.PRURL
	e.branchName = state.Branch
}

// clearPRState removes the PR state file.
func (e *Engine) clearPRState() {
	path := filepath.Join(e.repoPath, prStateFile)
	os.Remove(path)
}

// WithEventSink sets a channel that receives live agent events during evolution.
func (e *Engine) WithEventSink(sink chan<- iteragent.Event) *Engine {
	e.eventSink = sink
	return e
}

// WithThinking sets the extended thinking level for all agents spawned by this engine.
func (e *Engine) WithThinking(level iteragent.ThinkingLevel) *Engine {
	e.thinkingLevel = level
	return e
}

// Run executes one full evolution session.
func (e *Engine) handlePostRunTests(ctx context.Context, day int, output string, p iteragent.Provider, tools []iteragent.Tool, skills *iteragent.SkillSet, result *RunResult) error {
	testResult, testErr := e.runTests(ctx)
	_ = testResult

	if testErr != nil {
		e.logger.Info("tests failed, reverting changes")
		result.Status = "reverted"
		_ = e.revert(ctx)
		return nil
	}

	e.logger.Info("tests passed, creating feature branch")
	if err := e.handleCommitAndPR(ctx, day, output, p, result); err != nil {
		return nil
	}

	e.handlePRReviewAndMerge(ctx, p, tools, skills, output, result)
	return nil
}

func (e *Engine) Run(ctx context.Context, p iteragent.Provider, issues string) (*RunResult, error) {
	e.logger.Info("evolution run started", "repo", e.repo)
	result := &RunResult{
		StartedAt: time.Now(),
		Status:    "running",
	}

	identity, journal, day := e.readContextFiles()

	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))
	userMessage := buildUserMessage(e.repoPath, string(journal), issues)

	tools := iteragent.DefaultTools(e.repoPath)
	skills, _ := iteragent.LoadSkills([]string{filepath.Join(e.repoPath, "skills")})
	a := e.newAgent(p, tools, systemPrompt, skills)

	output, runErr := e.runAgentAndCollectEvents(ctx, a, userMessage)
	a.Finish()
	result.FinishedAt = time.Now()

	if runErr != nil {
		result.Status = "error"
		e.logger.Error("evolution run failed", "error", runErr)
		return result, runErr
	}

	hasChanges, _ := e.hasChanges(ctx)
	if !hasChanges {
		e.logger.Info("no changes detected, skipping PR flow")
		result.Status = "no_changes"
		return result, nil
	}

	if err := e.handlePostRunTests(ctx, day, output, p, tools, skills, result); err != nil {
		return result, err
	}

	e.logger.Info("evolution run completed", "status", result.Status, "duration", result.FinishedAt.Sub(result.StartedAt).String())
	return result, nil
}

func (e *Engine) readContextFiles() ([]byte, []byte, int) {
	identity, err := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	if err != nil {
		e.logger.Warn("failed to read IDENTITY.md", "err", err)
	}
	journal, err := os.ReadFile(filepath.Join(e.repoPath, "docs/JOURNAL.md"))
	if err != nil {
		e.logger.Warn("failed to read JOURNAL.md", "err", err)
	}
	dayBytes, err := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	if err != nil {
		e.logger.Warn("failed to read DAY_COUNT", "err", err)
	}
	day, err := strconv.Atoi(strings.TrimSpace(string(dayBytes)))
	if err != nil {
		e.logger.Warn("failed to parse DAY_COUNT", "err", err, "raw", string(dayBytes))
	}
	return identity, journal, day
}

func (e *Engine) runAgentAndCollectEvents(ctx context.Context, a *iteragent.Agent, userMessage string) (string, error) {
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
	return output, runErr
}

// handleCommitAndPR creates a branch, commits, pushes, and creates a PR.
func (e *Engine) handleCommitAndPR(ctx context.Context, day int, output string, p iteragent.Provider, result *RunResult) error {
	branchName, err := e.createFeatureBranch(ctx, day)
	if err != nil {
		e.logger.Warn("failed to create feature branch, falling back to direct commit", "err", err)
		result.Status = "committed"
		commitMsg := extractCommitMessage(output)
		if err := e.commit(ctx, commitMsg); err != nil {
			result.Status = "commit_failed"
		}
		e.appendJournal(result, output, p.Name(), true)
		return fmt.Errorf("branch creation failed")
	}
	e.logger.Info("feature branch created", "branch", branchName)

	e.logger.Info("committing changes")
	commitMsg := extractCommitMessage(output)
	if err := e.commit(ctx, commitMsg); err != nil {
		e.logger.Warn("commit failed", "err", err)
		_ = e.switchToMain(ctx)
		if err := e.commit(ctx, commitMsg); err != nil {
			result.Status = "commit_failed"
		}
		e.appendJournal(result, output, p.Name(), true)
		return fmt.Errorf("commit failed")
	}
	e.logger.Info("changes committed")

	e.logger.Info("pushing branch")
	if err := e.pushBranch(ctx); err != nil {
		e.logger.Warn("push failed, falling back to direct commit", "err", err)
		_ = e.switchToMain(ctx)
		result.Status = "committed"
		e.appendJournal(result, output, p.Name(), true)
		return fmt.Errorf("push failed")
	}
	e.logger.Info("branch pushed")

	return e.createPRFromBranch(ctx, day, output, commitMsg, p, result)
}

// createPRFromBranch builds PR body and creates the pull request.
func (e *Engine) createPRFromBranch(ctx context.Context, day int, output string, commitMsg string, p iteragent.Provider, result *RunResult) error {
	e.logger.Info("creating PR")

	planBytes, err := os.ReadFile(filepath.Join(e.repoPath, "SESSION_PLAN.md"))
	if err != nil {
		e.logger.Warn("failed to read SESSION_PLAN.md", "err", err)
	}
	issueNums := extractIssueNumbers(string(planBytes))
	prTitle := firstLine(commitMsg)
	prBody := buildPRBody(string(planBytes), output)

	e.logger.Info("creating PR", "title", prTitle, "body_len", len(prBody))
	prNum, prURL, err := e.createPR(ctx, prTitle, prBody, issueNums)
	e.logger.Info("PR creation result", "prNum", prNum, "prURL", prURL, "err", err)
	if err != nil {
		e.logger.Warn("PR creation failed, falling back to direct main commit", "err", err)
		_ = e.switchToMain(ctx)
		result.Status = "committed"
		e.appendJournal(result, output, p.Name(), true)
		return fmt.Errorf("PR creation failed")
	}
	return nil
}

// handlePRReviewAndMerge reviews the PR, merges if approved, and records the result.
func (e *Engine) handlePRReviewAndMerge(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, skills *iteragent.SkillSet, output string, result *RunResult) {
	systemPrompt := buildSystemPrompt(e.repoPath, "")
	if err := e.reviewPR(ctx, p, tools, systemPrompt, skills); err != nil {
		e.logger.Warn("PR review had issues", "err", err)
	}

	if err := e.mergePR(ctx); err != nil {
		e.logger.Warn("PR merge failed, will retry next session", "err", err)
		result.Status = "merge_pending"
		e.appendJournal(result, output, p.Name(), true)
		return
	}

	result.Status = "merged"
	result.PRNumber = e.prNumber
	result.PRURL = e.prURL
	_ = e.appendLearningJSONL(firstLine(extractCommitMessage(output)), "evolution", buildUserMessage(e.repoPath, "", ""), "")
	e.appendJournal(result, output, p.Name(), true)

	_ = e.switchToMain(ctx)
}

// auditLog appends a tool call or error to .iterate/audit.jsonl for debugging.
func (e *Engine) auditLog(eventType, tool, detail string) {
	auditPath := filepath.Join(e.repoPath, ".iterate", "audit.jsonl")
	_ = os.MkdirAll(filepath.Dir(auditPath), 0o755)

	entry := map[string]string{
		"ts":   time.Now().UTC().Format(time.RFC3339),
		"type": eventType,
		"tool": tool,
	}
	if detail != "" {
		// Truncate long details
		if len(detail) > 200 {
			detail = detail[:200] + "..."
		}
		entry["detail"] = detail
	}

	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
