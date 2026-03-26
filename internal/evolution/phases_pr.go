package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RunPRPhase creates a feature branch from the current HEAD, pushes it, and opens a PR.
// It is designed to run after RunImplementPhase has already committed changes to main.
func (e *Engine) RunPRPhase(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	day := e.readDayCount()

	// Sync with remote before branching.
	if out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git pull --rebase origin main",
	}); err != nil {
		e.logger.Warn("pull rebase failed, continuing", "err", err, "output", out)
	}

	// Skip if nothing has changed relative to origin/main.
	out, _ := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git diff origin/main HEAD --stat",
	})
	if strings.TrimSpace(out) == "" {
		e.logger.Info("no changes vs origin/main — skipping PR creation")
		return nil
	}

	// Create feature branch at current HEAD (implement commits are already here).
	branchName := fmt.Sprintf("evolution/day-%s", day)
	e.branchName = branchName

	// Delete any stale branch with the same name.
	_ = e.deleteBranch(ctx, branchName)

	if out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git checkout -b %s", branchName),
	}); err != nil {
		return fmt.Errorf("failed to create feature branch %s: %w (output: %s)", branchName, err, out)
	}
	e.logger.Info("created feature branch", "branch", branchName)

	// Push with lease; fall back to a plain push on failure.
	if out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git push -u origin %q --force-with-lease", branchName),
	}); err != nil {
		e.logger.Warn("push with lease failed, retrying", "err", err, "output", out)
		if out2, err2 := e.runTool(ctx, "bash", map[string]string{
			"cmd": fmt.Sprintf("git push -u origin %q", branchName),
		}); err2 != nil {
			return fmt.Errorf("failed to push branch: %w (output: %s)", err2, out2)
		}
	}
	e.logger.Info("pushed branch", "branch", branchName)

	// Build PR title/body from the session plan.
	planBytes, _ := os.ReadFile(filepath.Join(e.repoPath, "SESSION_PLAN.md"))
	plan := string(planBytes)
	issueNums := extractIssueNumbers(plan)

	sessionTitle := extractSessionTitle(plan)
	prTitle := fmt.Sprintf("iterate: Day %s evolution session", day)
	if sessionTitle != "" {
		prTitle = sessionTitle
	}
	prBody := buildPRBody(plan, "")

	prNum, prURL, err := e.createPR(ctx, prTitle, prBody, issueNums)
	if err != nil {
		return fmt.Errorf("PR creation failed: %w", err)
	}
	e.logger.Info("PR created", "number", prNum, "url", prURL)

	return e.savePRState()
}

// RunReviewPhase runs an AI self-review of the open PR.
func (e *Engine) RunReviewPhase(ctx context.Context, p iteragent.Provider) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	if e.prNumber == 0 {
		e.logger.Info("no PR to review — skipping review phase")
		return nil
	}

	identity, err := os.ReadFile(filepath.Join(e.repoPath, "docs/IDENTITY.md"))
	if err != nil {
		e.logger.Warn("failed to load IDENTITY.md for review phase", "err", err)
	}
	systemPrompt := buildSystemPrompt(e.repoPath, string(identity))

	if err := e.reviewPR(ctx, p, e.tools, systemPrompt, e.skills); err != nil {
		e.logger.Warn("PR review encountered issues", "err", err)
	}
	return nil
}

// RunMergePhase merges the open PR, clears state, and returns to main.
func (e *Engine) RunMergePhase(ctx context.Context) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	if e.prNumber == 0 {
		e.logger.Info("no PR to merge — skipping merge phase")
		return nil
	}

	if err := e.mergePR(ctx); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}
	e.logger.Info("PR merged", "number", e.prNumber)

	e.clearPRState()

	if err := e.switchToMain(ctx); err != nil {
		e.logger.Warn("failed to switch to main after merge", "err", err)
	}

	// Pull to make sure local main is up-to-date.
	if out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git pull origin main",
	}); err != nil {
		e.logger.Warn("pull after merge failed", "err", err, "output", out)
	}

	return nil
}
