package evolution

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type SafetyCheck struct {
	LintCheck             bool
	TestModificationCheck bool
	SmokeTestCheck        bool
	RequireHumanReview    bool
}

func DefaultSafetyCheck() *SafetyCheck {
	return &SafetyCheck{
		LintCheck:             true,
		TestModificationCheck: true,
		SmokeTestCheck:        true,
		RequireHumanReview:    false,
	}
}

func (e *Engine) RunSafetyChecks(ctx context.Context, prFiles []string) (bool, string, error) {
	safety := DefaultSafetyCheck()

	// Check 1: Test modification - block if tests were modified
	if safety.TestModificationCheck {
		blocked, reason := e.checkTestModification(prFiles)
		if blocked {
			if safety.RequireHumanReview {
				return false, reason + " - Requires human review before merge", nil
			}
			return false, reason, nil
		}
	}

	// Check 2: Diff size sanity - warn if too many files changed
	if blocked, reason := e.checkDiffSize(ctx, prFiles); blocked {
		e.logger.Warn("Large diff detected", "reason", reason)
		// Just warn, don't block - but could require human review
	}

	// Check 3: Sensitive files - be extra careful with security-sensitive files
	if blocked, reason := e.checkSensitiveFiles(prFiles); blocked {
		e.logger.Warn("Sensitive files modified", "files", reason)
		// Could require human review for sensitive changes
	}

	// Check 4: Lint check
	if safety.LintCheck {
		passed, output, err := e.runLintCheck(ctx)
		if err != nil {
			return false, "", fmt.Errorf("lint check failed: %w", err)
		}
		if !passed {
			return false, "Lint check failed:\n" + output, nil
		}
	}

	// Check 5: Smoke tests (fast subset)
	if safety.SmokeTestCheck {
		passed, output, err := e.runSmokeTests(ctx)
		if err != nil {
			return false, "", fmt.Errorf("smoke tests failed: %w", err)
		}
		if !passed {
			return false, "Smoke tests failed:\n" + output, nil
		}
	}

	return true, "All safety checks passed", nil
}

func (e *Engine) checkTestModification(prFiles []string) (bool, string) {
	testFilesModified := []string{}

	for _, f := range prFiles {
		// Check for test files
		if strings.HasSuffix(f, "_test.go") ||
			strings.HasSuffix(f, ".test.go") ||
			strings.Contains(f, "_test/") {
			testFilesModified = append(testFilesModified, f)
		}
	}

	if len(testFilesModified) > 0 {
		reason := fmt.Sprintf("Test files modified: %s\nAI agents should not modify tests - only production code.",
			strings.Join(testFilesModified, ", "))
		e.logger.Warn("Test modification detected", "files", testFilesModified)
		return true, reason
	}

	return false, ""
}

func (e *Engine) runLintCheck(ctx context.Context) (bool, string, error) {
	// Run go vet
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = e.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output), nil
	}

	// Run gofmt check
	cmd = exec.CommandContext(ctx, "gofmt", "-l", ".")
	cmd.Dir = e.repoPath

	output, err = cmd.CombinedOutput()
	if err != nil && len(output) > 0 {
		return false, "Formatting issues:\n" + string(output), nil
	}

	return true, "", nil
}

func (e *Engine) runSmokeTests(ctx context.Context) (bool, string, error) {
	// Run fast tests only - core packages
	// Skip tests that need external resources (PRs, network) or are flaky
	cmd := exec.CommandContext(ctx, "go", "test", "-short", "-timeout", "60s",
		"-skip", "TestSaveAndLoadPRState|TestCreatePR|TestMergePR",
		"./internal/evolution/...",
		"./internal/autonomous/...",
	)
	cmd.Dir = e.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output), nil
	}

	return true, "", nil
}

func (e *Engine) checkDiffSize(ctx context.Context, prFiles []string) (bool, string) {
	// Check if too many files changed - could indicate runaway AI
	maxFiles := 20
	if len(prFiles) > maxFiles {
		return true, fmt.Sprintf("Too many files changed: %d (max: %d). This could indicate the AI is making unintended changes.",
			len(prFiles), maxFiles)
	}

	// Check total lines changed
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = e.repoPath
	output, _ := cmd.CombinedOutput()

	// Just warn if lots of changes - don't block
	_ = output

	return false, ""
}

func (e *Engine) checkSensitiveFiles(prFiles []string) (bool, string) {
	// Files that should rarely be modified by AI
	sensitivePatterns := []string{
		".github/workflows/",
		"scripts/evolution/",
		"docs/IDENTITY.md",
		".env",
		"credentials",
		"secret",
		"password",
	}

	var sensitive []string
	for _, f := range prFiles {
		lower := strings.ToLower(f)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(lower, strings.ToLower(pattern)) {
				sensitive = append(sensitive, f)
				break
			}
		}
	}

	if len(sensitive) > 0 {
		return true, strings.Join(sensitive, ", ")
	}

	return false, ""
}
