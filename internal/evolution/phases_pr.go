package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// RunPRPhase creates a feature branch from the current HEAD, pushes it, and opens a PR.
// It is designed to run after RunImplementPhase has already committed changes to main.
func (e *Engine) RunPRPhase(ctx context.Context) error {
	ctx, cancel := withPhaseTimeout(ctx, "pr")
	defer cancel()

	day := e.readDayCount()

	// Sync with remote before branching.
	if out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git fetch origin main",
	}); err != nil {
		e.logger.Warn("fetch failed, continuing", "err", err, "output", out)
	}

	// Check if there are any uncommitted changes to include.
	hasUncommitted := false
	if out, _ := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git status --short",
	}); strings.TrimSpace(out) != "" {
		hasUncommitted = true
		e.logger.Info("found uncommitted changes, committing before PR")
		if _, err := e.runTool(ctx, "bash", map[string]interface{}{
			"cmd": "git add -A && git diff --cached --quiet || git commit -m 'chore: evolution day " + day + " changes'",
		}); err != nil {
			e.logger.Warn("failed to commit uncommitted changes", "err", err)
		}
	}

	// Skip if nothing has changed relative to origin/main.
	out, _ := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git diff origin/main HEAD --stat",
	})
	if strings.TrimSpace(out) == "" && !hasUncommitted {
		e.logger.Info("no changes vs origin/main and no uncommitted changes — skipping PR creation")
		return nil
	}

	// Create feature branch at current HEAD (implement commits are already here).
	branchName := fmt.Sprintf("evolution/day-%s", day)
	e.branchName = branchName

	// Delete any stale branch with the same name.
	_ = e.deleteBranch(ctx, branchName)

	if out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git checkout -b %s", branchName),
	}); err != nil {
		return fmt.Errorf("failed to create feature branch %s: %w (output: %s)", branchName, err, out)
	}
	e.logger.Info("created feature branch", "branch", branchName)

	// Push with lease; fall back to a plain push on failure.
	if out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git push -u origin %q --force-with-lease 2>/dev/null || git push -u origin %q", branchName, branchName),
	}); err != nil {
		return fmt.Errorf("failed to push branch: %w (output: %s)", err, out)
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
	ctx, cancel := withPhaseTimeout(ctx, "review")
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
		return fmt.Errorf("review phase: %w", err)
	}
	return nil
}

// RunMergePhase merges the open PR, clears state, and returns to main.
func (e *Engine) RunMergePhase(ctx context.Context) error {
	ctx, cancel := withPhaseTimeout(ctx, "merge")
	defer cancel()

	if e.prNumber == 0 {
		e.logger.Info("no PR to merge — skipping merge phase")
		return nil
	}

	// Check if PR is still open before attempting merge.
	prState, _ := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("gh pr view %d --repo %s --json state --jq .state 2>/dev/null || echo UNKNOWN", e.prNumber, e.repo),
	})
	prState = strings.TrimSpace(prState)
	switch prState {
	case "MERGED":
		e.logger.Info("PR already merged", "number", e.prNumber)
	case "CLOSED":
		e.logger.Info("PR was closed without merge", "number", e.prNumber)
		e.clearPRState()
		e.clearSessionPlan()
		return nil
	}

	if err := e.mergePR(ctx); err != nil {
		// Check if PR was already merged by someone else
		if strings.Contains(err.Error(), "already merged") || strings.Contains(err.Error(), "closed") {
			e.logger.Info("PR already merged or closed, cleaning up state")
			e.clearPRState()
			e.clearSessionPlan()
			if err := e.switchToMain(ctx); err != nil {
				e.logger.Warn("failed to switch to main after merge", "err", err)
			}
			e.waitForCIAndRecord(ctx)
			return nil
		}
		return fmt.Errorf("merge failed: %w", err)
	}
	e.logger.Info("PR merged", "number", e.prNumber)

	e.clearPRState()
	e.clearSessionPlan()

	if err := e.switchToMain(ctx); err != nil {
		e.logger.Warn("failed to switch to main after merge", "err", err)
	}

	// Pull to make sure local main is up-to-date.
	if out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git pull origin main 2>/dev/null || git pull origin main --rebase 2>/dev/null || true",
	}); err != nil {
		e.logger.Warn("pull after merge failed", "err", err, "output", out)
	}

	// Wait for CI and write result to .iterate/ci_status.txt for the next cycle's planner.
	e.waitForCIAndRecord(ctx)

	return nil
}

// waitForCIAndRecord polls GitHub Actions for the CI run triggered by the merge.
// It writes a priority instruction to .iterate/ci_status.txt so the next
// planning cycle treats a CI failure as its top priority.
func (e *Engine) waitForCIAndRecord(ctx context.Context) {
	ciCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ciPath := filepath.Join(e.repoPath, ".iterate", "ci_status.txt")
	_ = os.MkdirAll(filepath.Dir(ciPath), 0o755)

	e.logger.Info("waiting for CI to complete after merge...")

	// Give GitHub a moment to register the CI run after merge.
	time.Sleep(15 * time.Second)

	var conclusion string
	var failedLog string
	pollCount := 0
	const maxPolls = 40 // 40 * 30s = 20 minutes max

	for pollCount < maxPolls {
		select {
		case <-ciCtx.Done():
			e.logger.Warn("CI wait timed out", "polls", pollCount)
			_ = os.WriteFile(ciPath, []byte("CI TIMED OUT after merge. Check GitHub Actions manually."), 0o644)
			return
		default:
		}

		out, err := e.runTool(ciCtx, "bash", map[string]interface{}{
			"cmd": fmt.Sprintf(
				"gh run list --repo %s --workflow ci.yml --branch main --limit 1 --json conclusion,status --jq '.[0]|.conclusion+\":\"+.status' 2>/dev/null || echo 'error:'",
				e.repo,
			),
		})
		if err != nil {
			e.logger.Warn("CI poll failed", "err", err, "poll", pollCount)
			pollCount++
			time.Sleep(30 * time.Second)
			continue
		}

		out = strings.TrimSpace(out)
		if out == "" || out == "error:" || out == ":" {
			e.logger.Info("CI run not yet visible, waiting...", "poll", pollCount)
			pollCount++
			time.Sleep(30 * time.Second)
			continue
		}

		parts := strings.SplitN(out, ":", 2)
		conclusion = parts[0]
		status := ""
		if len(parts) > 1 {
			status = parts[1]
		}

		// "in_progress" or "queued" means still running
		if status == "completed" || conclusion == "success" || conclusion == "failure" {
			e.logger.Info("CI run completed", "conclusion", conclusion)
			break
		}

		e.logger.Info("CI still running", "status", status, "conclusion", conclusion, "poll", pollCount)
		pollCount++
		select {
		case <-ciCtx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}

	switch conclusion {
	case "success":
		e.logger.Info("CI passed after merge")
		_ = os.Remove(ciPath)
	case "failure":
		e.logger.Warn("CI failed after merge — recording for next planner cycle")
		// Fetch the failed step logs for context.
		failedLog, _ = e.runTool(ciCtx, "bash", map[string]interface{}{
			"cmd": fmt.Sprintf(
				"gh run list --repo %s --workflow ci.yml --branch main --limit 1 --json databaseId --jq '.[0].databaseId' | xargs -I{} gh run view {} --repo %s --log-failed 2>/dev/null | head -60",
				e.repo, e.repo,
			),
		})
		msg := "PREVIOUS CI FAILED after merge. Fix the broken tests FIRST before any other task.\n"
		if failedLog != "" {
			msg += "\nFailed output:\n```\n" + failedLog + "\n```\n"
		}
		_ = os.WriteFile(ciPath, []byte(msg), 0o644)
	default:
		// cancelled, skipped, neutral — clear the file
		_ = os.Remove(ciPath)
	}
}
