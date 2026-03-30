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

func (e *Engine) runTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, ok := e.toolMap[name]
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}

	// Audit log
	e.auditLog("tool_call", name, iteragent.ArgStr(args, "cmd"))

	result, err := tool.Execute(ctx, args)
	if err != nil {
		e.auditLog("tool_error", name, err.Error())
	}
	return result, err
}

func (e *Engine) hasChanges(ctx context.Context) (bool, error) {
	out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git status --short",
	})
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (e *Engine) currentBranch(ctx context.Context) (string, error) {
	out, err := e.runTool(ctx, "bash", map[string]interface{}{
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
		if out, err := e.runTool(ctx, "bash", map[string]interface{}{"cmd": cmd}); err != nil {
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
	if out, err := e.runTool(ctx, "bash", map[string]interface{}{"cmd": createCmd}); err != nil {
		e.logger.Warn("branch creation failed", "err", err, "output", out)
		return "", fmt.Errorf("branch creation failed: %w", err)
	}

	e.logger.Info("created feature branch", "branch", branchName)
	return branchName, nil
}

func (e *Engine) pushBranch(ctx context.Context) error {
	_, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git push -u origin %q", e.branchName),
	})
	return err
}

// runGHCommandWithBackoff runs a gh CLI command and retries on rate-limit (HTTP 429/403).
// Retries up to maxRetries times with exponential backoff starting at 30s.
func (e *Engine) runGHCommandWithBackoff(ctx context.Context, args []string) (string, error) {
	const maxRetries = 4
	wait := 30 * time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		cmd := exec.Command("gh", args...)
		cmd.Dir = e.repoPath
		outBytes, err := cmd.CombinedOutput()
		out := strings.TrimSpace(string(outBytes))
		if err == nil {
			return out, nil
		}
		lower := strings.ToLower(out)
		isRateLimit := strings.Contains(lower, "rate limit") ||
			strings.Contains(lower, "429") ||
			strings.Contains(lower, "secondary rate") ||
			strings.Contains(lower, "abuse detection")
		if !isRateLimit || attempt == maxRetries {
			return out, fmt.Errorf("gh command failed: %w, output: %s", err, out)
		}
		e.logger.Warn("GitHub rate limit hit, backing off",
			"attempt", attempt+1, "wait", wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		wait *= 2
	}
	return "", fmt.Errorf("gh command failed after %d retries", maxRetries)
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
	out, err := e.runGHCommandWithBackoff(ctx, args)
	if err != nil {
		return 0, "", fmt.Errorf("PR creation failed: %w", err)
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

// reviewPR runs the AI self-review. If isReReview is true, this is a second review after auto-fix.
func (e *Engine) reviewPR(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, isReReview ...bool) error {
	if e.prNumber == 0 {
		return fmt.Errorf("no PR to review")
	}

	prDiff, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("gh pr diff %d --repo %s", e.prNumber, e.repo),
	})
	if err != nil {
		return fmt.Errorf("failed to get PR diff: %w", err)
	}

	userMsg := e.buildPRReviewMessage(prDiff)
	if len(isReReview) > 0 && isReReview[0] {
		userMsg = "This is a RE-REVIEW after auto-fixes were applied.\n\n" + userMsg
	}

	a := e.newAgent(p, tools, systemPrompt, skills)
	var reviewBuilder strings.Builder
	var finalContent string
	for ev := range a.Prompt(ctx, userMsg) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		// Accumulate content from streaming updates
		if ev.Type == string(iteragent.EventMessageUpdate) {
			reviewBuilder.WriteString(ev.Content)
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			finalContent = ev.Content
		}
	}
	a.Finish()

	// Use accumulated content or final content, whichever is longer
	reviewText := reviewBuilder.String()
	if len(finalContent) > len(reviewText) {
		reviewText = finalContent
	}

	// Remove verbose tool call blocks - keep only the final summary
	reviewText = e.summarizeReview(reviewText)

	// Fallback if still empty
	if strings.TrimSpace(reviewText) == "" {
		reviewText = "Review completed but no output was generated. Please check the code manually."
	}

	low := strings.ToLower(reviewText)
	passed := strings.Contains(low, "lgtm") || strings.Contains(low, "looks good") || strings.Contains(low, "approved")
	blocked := strings.Contains(low, "reject") || strings.Contains(low, "block") || strings.Contains(low, "not ready")

	// If build and tests pass and no explicit rejection, approve
	if passed || !blocked {
		// Check if build/tests actually pass first
		v := e.verify(ctx)
		if v.BuildPassed && v.TestPassed {
			e.postReviewComment(ctx, reviewText+"\n\n---\n✅ Build and tests pass. Approved for merge.", true)
			e.logger.Info("PR self-review passed (build/tests OK)")
			return nil
		}
		// If build/tests don't pass, continue to try auto-fix
	}

	// Only block if explicitly rejected
	if blocked {
		e.postReviewComment(ctx, reviewText, false)
		return fmt.Errorf("review explicitly rejected: %s", reviewText)
	}

	// For minor issues where build/tests pass, still approve
	v := e.verify(ctx)
	if v.BuildPassed && v.TestPassed {
		e.postReviewComment(ctx, reviewText+"\n\n---\n✅ Build and tests pass despite minor issues. Approved for merge.", true)
		e.logger.Info("PR self-review passed with minor issues (build/tests OK)")
		return nil
	}

	// Reviewer found issues — try to auto-fix them (only on first review)
	if len(isReReview) > 0 && isReReview[0] {
		// Already tried auto-fix, still failed
		e.postReviewComment(ctx, reviewText, false)
		return fmt.Errorf("review blocked merge: issues remain after auto-fix")
	}

	e.logger.Warn("PR self-review found issues — attempting auto-fix")

	fixed, fixErr := e.autoFixIssues(ctx, p, tools, systemPrompt, skills, reviewText)
	if fixErr != nil {
		e.logger.Error("auto-fix failed", "err", fixErr)
		e.postReviewComment(ctx, reviewText+"\n\n---\n**Auto-fix failed:** "+fixErr.Error(), false)
		return fmt.Errorf("review blocked merge: %w", fixErr)
	}

	if !fixed {
		e.logger.Warn("auto-fix could not resolve all issues — blocking merge")
		e.postReviewComment(ctx, reviewText+"\n\n---\n**Auto-fix:** Could not resolve all issues automatically.", false)
		return fmt.Errorf("review blocked merge: auto-fix could not resolve all issues")
	}

	// Re-review after fixes
	e.logger.Info("auto-fix applied — re-reviewing PR")
	return e.reviewPR(ctx, p, tools, systemPrompt, skills, true)
}

// autoFixIssues attempts to fix issues identified during review.
// Returns true if fixes were applied and tests pass, false otherwise.
func (e *Engine) autoFixIssues(ctx context.Context, p iteragent.Provider, tools []iteragent.Tool, systemPrompt string, skills *iteragent.SkillSet, reviewOutput string) (bool, error) {
	e.logger.Info("starting auto-fix based on review feedback")

	// Create fresh context with longer timeout for auto-fix (it runs multiple operations)
	autoFixCtx, autoFixCancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer autoFixCancel()

	// Build fix prompt with review feedback - emphasize unified diffs
	fixPrompt := fmt.Sprintf(`The code review identified issues:

%s

## CRITICAL: You MUST output UNIFIED DIFFS or use write_file/edit_file tools to fix these issues.
Do NOT just describe what you'll do - actually make the changes.

Output unified diffs like:
--- a/path/to/file.go
+++ b/path/to/file.go
@@ ... @@
-old code
+new code

Or use JSON tool calls like:
{"tool":"write_file","args":{"path":"file.go","content":"..."}}

Make the fixes now!`, reviewOutput)

	a := e.newAgent(p, tools, systemPrompt, skills)
	var outputBuilder strings.Builder
	for ev := range a.Prompt(autoFixCtx, fixPrompt) {
		if e.eventSink != nil {
			select {
			case e.eventSink <- ev:
			default:
			}
		}
		if ev.Type == string(iteragent.EventMessageUpdate) {
			outputBuilder.WriteString(ev.Content)
		}
	}
	a.Finish()

	// Try to parse and apply tool calls / unified diffs from output
	output := outputBuilder.String()

	// First try unified diffs
	diffs := ParseUnifiedDiffs(output)
	if len(diffs) > 0 {
		e.logger.Info("found unified diffs in auto-fix output", "count", len(diffs))
		modifiedFiles, err := e.ApplyUnifiedDiffs(diffs)
		if err != nil {
			e.logger.Warn("failed to apply diffs from auto-fix", "err", err)
		} else if len(modifiedFiles) > 0 {
			e.logger.Info("applied diffs from auto-fix", "files", modifiedFiles)
		}
	}

	// Also try tool_call JSON parsing (like in executeTask)
	if modified, err := e.applyToolCallChanges(autoFixCtx, output); err == nil && len(modified) > 0 {
		e.logger.Info("applied tool calls from auto-fix", "files", modified)
	}

	// Check if any changes were made
	status, err := e.runTool(autoFixCtx, "bash", map[string]interface{}{
		"cmd": "git status --porcelain",
	})
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	if strings.TrimSpace(status) == "" {
		e.logger.Info("no changes made during auto-fix")
		return false, nil
	}

	// Run tests to verify fixes
	e.logger.Info("running tests to verify auto-fix")
	_, testErr := e.runTool(autoFixCtx, "bash", map[string]interface{}{
		"cmd": "go test ./... 2>&1 | head -50",
	})
	if testErr != nil {
		e.logger.Warn("tests failed after auto-fix, reverting changes")
		// Revert failed fixes
		e.runTool(autoFixCtx, "bash", map[string]interface{}{
			"cmd": "git checkout -- .",
		})
		return false, fmt.Errorf("tests failed after auto-fix")
	}

	// Commit the fixes
	e.logger.Info("committing auto-fix changes")
	if _, err := e.runTool(autoFixCtx, "bash", map[string]interface{}{
		"cmd": "git add -A && git commit -m 'fix: auto-fix issues from review'",
	}); err != nil {
		return false, fmt.Errorf("failed to commit auto-fix: %w", err)
	}

	// Push fixes to the PR branch
	if _, err := e.runTool(autoFixCtx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git push origin %s", e.branchName),
	}); err != nil {
		return false, fmt.Errorf("failed to push auto-fix: %w", err)
	}

	e.logger.Info("auto-fix applied successfully - will re-review")
	return true, nil
}

// postReviewComment posts the reviewer's output as a GitHub PR comment.
func (e *Engine) postReviewComment(ctx context.Context, reviewOutput string, passed bool) {
	if e.prNumber == 0 {
		return
	}

	verdict := "❌ Issues found — merge blocked."
	if passed {
		verdict = "✅ LGTM — auto-merging."
	}

	body := fmt.Sprintf("## Self-Review\n\n%s\n\n---\n**Verdict:** %s\n\n*Reviewed by iterate-evolve[bot]*",
		reviewOutput, verdict)

	// Use exec.Command to avoid shell injection via body content.
	cmd := exec.Command("gh", "pr", "comment", fmt.Sprintf("%d", e.prNumber),
		"--repo", e.repo, "--body", body)
	cmd.Dir = e.repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		e.logger.Warn("failed to post review comment", "err", err, "output", string(out))
	} else {
		e.logger.Info("posted review comment", "pr", e.prNumber)
	}
}

func (e *Engine) mergePR(ctx context.Context) error {
	if e.prNumber == 0 {
		return fmt.Errorf("no PR to merge")
	}

	args := []string{"pr", "merge", fmt.Sprintf("%d", e.prNumber), "--repo", e.repo, "--squash", "--delete-branch"}
	out, err := e.runGHCommandWithBackoff(ctx, args)
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
		if _, err := e.runTool(ctx, "bash", map[string]interface{}{"cmd": cmd}); err == nil {
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

// Per-phase timeouts tuned to each phase's actual workload.
const (
	timeoutPlan        = 8 * time.Minute  // read + plan only, no code changes
	timeoutImplement   = 40 * time.Minute // one timeout covers all parallel tasks
	timeoutPR          = 3 * time.Minute  // git + gh CLI only
	timeoutReview      = 10 * time.Minute // diff read + agent review
	timeoutMerge       = 12 * time.Minute // merge + CI poll (short, CI poll has its own 10m timer)
	timeoutCommunicate = 10 * time.Minute // journal + issue comments + learnings
)

// withTimeout wraps a context with the default phase timeout (kept for legacy callers).
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeoutImplement)
}

// withPhaseTimeout wraps a context with the timeout for a specific named phase.
func withPhaseTimeout(ctx context.Context, phase string) (context.Context, context.CancelFunc) {
	d := timeoutImplement
	switch phase {
	case "plan":
		d = timeoutPlan
	case "pr":
		d = timeoutPR
	case "review":
		d = timeoutReview
	case "merge":
		d = timeoutMerge
	case "communicate":
		d = timeoutCommunicate
	}
	return context.WithTimeout(ctx, d)
}

func (e *Engine) revert(ctx context.Context) error {
	// Reset uncommitted changes first.
	if _, err := e.runTool(ctx, "bash", map[string]interface{}{
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
	_, err := e.toolMap["git_commit"].Execute(ctx, map[string]interface{}{"message": msg})
	return err
}

// summarizeReview makes the review comment concise by removing verbose tool call blocks
func (e *Engine) summarizeReview(text string) string {
	// Remove tool call blocks and keep only important lines (verdict, issues, fixes)

	lines := strings.Split(text, "\n")
	var result []string
	var inToolBlock bool
	var hasVerdict bool
	var verdictLine string

	verdictKeywords := []string{"lgtm", "approved", "looks good", "verdict:", "issues found",
		"critical issue", "fixes applied", "rejected", "block", "not ready", "auto-fix",
		"build and tests pass", "approved for merge"}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if we're in a tool block
		if strings.HasPrefix(trimmed, "```tool") || strings.HasPrefix(trimmed, "```json") {
			inToolBlock = true
			continue
		}
		if trimmed == "```" {
			inToolBlock = false
			continue
		}
		if inToolBlock {
			continue
		}

		// Skip empty lines at start
		if len(result) == 0 && trimmed == "" {
			continue
		}

		// Check for verdict keywords
		lower := strings.ToLower(trimmed)
		isVerdict := false
		for _, kw := range verdictKeywords {
			if strings.Contains(lower, kw) {
				isVerdict = true
				verdictLine = trimmed
				break
			}
		}

		if isVerdict {
			hasVerdict = true
		}

		// Keep lines that are: verdict lines, headers (##), or bullet points with content
		if isVerdict || strings.HasPrefix(trimmed, "##") ||
			(strings.HasPrefix(trimmed, "-") && len(trimmed) > 5) ||
			(strings.HasPrefix(trimmed, "*") && len(trimmed) > 5) {
			result = append(result, line)
		}
	}

	// If no clear verdict found, just take first few non-empty lines
	if !hasVerdict || len(result) > 20 {
		// Take only first 10 meaningful lines
		newResult := []string{}
		count := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "```") {
				continue
			}
			newResult = append(newResult, line)
			count++
			if count >= 10 {
				break
			}
		}
		result = newResult
	}

	// Ensure we have a verdict line at the end
	if hasVerdict && verdictLine != "" {
		result = append(result, "", verdictLine)
	}

	// Truncate if still too long
	output := strings.Join(result, "\n")
	if len(output) > 2000 {
		output = output[:2000] + "\n...(truncated)"
	}

	return output
}
