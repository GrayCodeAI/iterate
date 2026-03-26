package evolution

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/GrayCodeAI/iterate/internal/util"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func (e *Engine) runTool(ctx context.Context, name string, args map[string]string) (string, error) {
	tool, ok := e.toolMap[name]
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}

	// Audit log
	e.auditLog("tool_call", name, args["cmd"])

	result, err := tool.Execute(ctx, args)
	if err != nil {
		e.auditLog("tool_error", name, err.Error())
	}
	return result, err
}

func (e *Engine) hasChanges(ctx context.Context) (bool, error) {
	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git status --short",
	})
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (e *Engine) currentBranch(ctx context.Context) (string, error) {
	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git branch --show-current",
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (e *Engine) deleteBranch(ctx context.Context, branch string) error {
	// Use exec.Command directly to avoid shell injection via branch name.
	localCmd := exec.Command("git", "branch", "-D", branch)
	localCmd.Dir = e.repoPath
	_ = localCmd.Run() // best-effort local delete

	remoteCmd := exec.Command("git", "push", "origin", "--delete", branch)
	remoteCmd.Dir = e.repoPath
	_ = remoteCmd.Run() // best-effort remote delete
	return nil
}

func (e *Engine) createFeatureBranch(ctx context.Context, day int) (string, error) {
	branchName := fmt.Sprintf("evolution/day-%d", day)
	e.branchName = branchName

	e.logger.Info("preparing to create feature branch", "branch", branchName)

	// First ensure we're on main and have latest
	cmds := []string{
		"git fetch origin main",
		"git checkout main",
		"git pull origin main",
	}
	for _, cmd := range cmds {
		e.logger.Info("running prep command", "cmd", cmd)
		if out, err := e.runTool(ctx, "bash", map[string]string{"cmd": cmd}); err != nil {
			e.logger.Warn("prep command failed", "cmd", cmd, "err", err, "output", out)
			return "", fmt.Errorf("prep command failed: %w", err)
		}
	}

	// Delete existing branch if it exists
	if err := e.deleteBranch(ctx, branchName); err != nil {
		e.logger.Warn("failed to delete existing branch", "branch", branchName, "err", err)
	}

	// Create and checkout new branch
	createCmd := fmt.Sprintf("git checkout -b %s origin/main", branchName)
	e.logger.Info("creating branch", "cmd", createCmd)
	if out, err := e.runTool(ctx, "bash", map[string]string{"cmd": createCmd}); err != nil {
		e.logger.Warn("branch creation failed", "err", err, "output", out)
		return "", fmt.Errorf("branch creation failed: %w", err)
	}

	e.logger.Info("created feature branch", "branch", branchName)
	return branchName, nil
}

func (e *Engine) pushBranch(ctx context.Context) error {
	_, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("git push -u origin %q", e.branchName),
	})
	return err
}

func (e *Engine) createPR(ctx context.Context, title, body string, issueNums []int) (int, string, error) {
	var linkedIssues []string
	for _, n := range issueNums {
		linkedIssues = append(linkedIssues, fmt.Sprintf("Fixes #%d", n))
	}

	prBody := body
	if len(linkedIssues) > 0 {
		prBody += "\n\n## Related Issues\n"
		for _, issue := range linkedIssues {
			prBody += "- " + issue + "\n"
		}
	}

	// Use gh CLI with proper argument passing to prevent shell injection.
	args := []string{"pr", "create", "--repo", e.repo, "--title", title, "--body", prBody, "--base", "main", "--head", e.branchName}
	cmd := exec.Command("gh", args...)
	cmd.Dir = e.repoPath
	outBytes, err := cmd.CombinedOutput()
	out := string(outBytes)
	if err != nil {
		return 0, "", fmt.Errorf("PR creation failed: %w, output: %s", err, out)
	}

	url := strings.TrimSpace(out)
	var prNum int
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		if _, err := fmt.Sscanf(url[idx+1:], "%d", &prNum); err != nil {
			e.logger.Warn("could not parse PR number from URL", "url", url)
		}
	}

	e.prURL = url
	e.prNumber = prNum
	e.logger.Info("created PR", "number", prNum, "url", url)
	return prNum, url, nil
}

func (e *Engine) buildPRReviewMessage(prDiff string) string {
	return fmt.Sprintf(`Review your own PR #%d changes critically. Check for:
1. Bugs or security issues
2. Missing error handling
3. Test coverage
4. Code style violations
5. Whether the changes actually solve the stated tasks

Be honest and strict. If you find issues, describe them so you can fix them before merge.

## PR Diff:
%s

If you find issues, use write_file/edit_file/bash to fix them, then run tests. 
After fixing, amend your commit: git commit --amend --no-edit
Then push: git push --force-with-lease origin %s

If the changes are good, reply: "LGTM"
`, e.prNumber, util.Truncate(prDiff, 8000), e.branchName)
}

func (e *Engine) reviewPR(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet) error {
	if e.prNumber == 0 {
		return fmt.Errorf("no PR to review")
	}

	prDiff, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("gh pr diff %d --repo %s", e.prNumber, e.repo),
	})
	if err != nil {
		return fmt.Errorf("failed to get PR diff: %w", err)
	}

	userMsg := e.buildPRReviewMessage(prDiff)

	a := e.newAgent(p, tools, systemPrompt, skills)
	var reviewOutput string
	for ev := range a.Prompt(ctx, userMsg) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			reviewOutput = ev.Content
		}
	}
	a.Finish()

	low := strings.ToLower(reviewOutput)
	if strings.Contains(low, "lgtm") || strings.Contains(low, "looks good") {
		e.logger.Info("PR self-review passed")
		return nil
	}

	// Reviewer found issues but didn't say LGTM — block the merge.
	e.logger.Warn("PR self-review did not pass — blocking merge")
	return fmt.Errorf("review blocked merge: reviewer did not say LGTM")
}

func (e *Engine) mergePR(ctx context.Context) error {
	if e.prNumber == 0 {
		return fmt.Errorf("no PR to merge")
	}

	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": fmt.Sprintf("gh pr merge %d --repo %s --squash --delete-branch", e.prNumber, e.repo),
	})
	if err != nil {
		return fmt.Errorf("PR merge failed: %w, output: %s", err, out)
	}

	e.logger.Info("PR merged successfully", "number", e.prNumber)
	return nil
}

func (e *Engine) switchToMain(ctx context.Context) error {
	// Try main first, then master, then recreate from origin/main.
	for _, cmd := range []string{
		"git checkout main",
		"git checkout master",
		"git checkout origin/main -b main",
	} {
		if _, err := e.runTool(ctx, "bash", map[string]string{"cmd": cmd}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not switch to main or master branch")
}

// forwardEvents drains the given event channel and forwards each event to eventSink.
func (e *Engine) forwardEvents(src <-chan iteragent.Event) {
	if e.eventSink == nil {
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

func (e *Engine) runTests(ctx context.Context) (string, error) {
	return e.toolMap["run_tests"].Execute(ctx, nil)
}

// defaultPhaseTimeout is the maximum duration for any evolution phase.
const defaultPhaseTimeout = 30 * time.Minute

// withTimeout wraps a context with the default phase timeout.
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, defaultPhaseTimeout)
}

func (e *Engine) revert(ctx context.Context) error {
	// Reset uncommitted changes first.
	if _, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git checkout -- . && git clean -fd",
	}); err == nil {
		return nil
	}
	// Fall back: use git_revert tool for any committed changes.
	e.logger.Warn("git checkout failed, trying revert tool")
	_, err := e.toolMap["git_revert"].Execute(ctx, nil)
	return err
}

func (e *Engine) commit(ctx context.Context, msg string) error {
	_, err := e.toolMap["git_commit"].Execute(ctx, map[string]string{"message": msg})
	return err
}
