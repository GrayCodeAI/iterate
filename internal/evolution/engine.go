package evolution

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/session"
)

// Engine runs one evolution session.
type Engine struct {
	repoPath string
	store    *session.Store
	logger   *slog.Logger
}

// New creates a new evolution engine.
func New(repoPath string, store *session.Store, logger *slog.Logger) *Engine {
	return &Engine{
		repoPath: repoPath,
		store:    store,
		logger:   logger,
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

	// Create agent with all tools
	tools := iteragent.DefaultTools(e.repoPath)
	a := iteragent.New(p, tools, e.logger)
	sess.Events = a.Events

	// Run the agent loop
	output, err := a.Run(ctx, systemPrompt, userMessage)
	sess.FinishedAt = time.Now()
	sess.RawOutput = output

	if err != nil {
		sess.Status = "error"
		sess.Error = err.Error()
		e.appendJournal(sess, false)
		_ = e.store.Save(sess)
		return sess, err
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
		}
		e.appendJournal(sess, true)
	}

	_ = e.store.Save(sess)
	return sess, nil
}

func buildSystemPrompt(repoPath, identity string) string {
	personality, _ := os.ReadFile(filepath.Join(repoPath, "PERSONALITY.md"))
	selfAssess, _ := os.ReadFile(filepath.Join(repoPath, "skills/self-assess.md"))
	evolveSkill, _ := os.ReadFile(filepath.Join(repoPath, "skills/evolve.md"))
	researchSkill, _ := os.ReadFile(filepath.Join(repoPath, "skills/research.md"))
	releaseSkill, _ := os.ReadFile(filepath.Join(repoPath, "skills/release.md"))

	return fmt.Sprintf(`You are iterate, a self-evolving coding agent written in Go.

## Identity
%s

## Personality
%s

## Self-assessment skill
%s

## Evolution skill
%s

## Research skill
%s

## Release skill
%s

## Tool call format
Wrap tool calls in triple backtick blocks:
`+"```"+`tool
{"tool":"tool_name","args":{"key":"value"}}
`+"```",
		identity,
		string(personality),
		string(selfAssess),
		string(evolveSkill),
		string(researchSkill),
		string(releaseSkill),
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

	status := "FAILED (reverted)"
	if success {
		status = "SUCCESS (committed)"
	}

	entry := fmt.Sprintf("\n## Session %s — %s\n\n**Status:** %s\n**Provider:** %s\n**Duration:** %s\n\n%s\n\n---\n",
		sess.StartedAt.Format("2006-01-02 15:04"),
		status,
		status,
		sess.Provider,
		sess.FinishedAt.Sub(sess.StartedAt).Round(time.Second),
		truncate(sess.RawOutput, 1000),
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		e.logger.Warn("failed to write journal", "err", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(entry)
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
