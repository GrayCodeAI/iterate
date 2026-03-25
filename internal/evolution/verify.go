package evolution

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// VerificationResult holds the outcome of a build + test verification run.
type VerificationResult struct {
	BuildPassed bool
	TestPassed  bool
	Output      string
	Error       error
}

// verify runs build + tests and returns the combined result.
// This is called after each implementation task to ensure changes don't break the codebase.
func (e *Engine) verify(ctx context.Context) *VerificationResult {
	result := &VerificationResult{}

	// Step 1: Build
	buildOut, buildErr := e.runTool(ctx, "bash", map[string]string{
		"cmd": "go build ./...",
	})
	result.Output = buildOut
	if buildErr != nil {
		result.Error = fmt.Errorf("build failed: %w", buildErr)
		return result
	}
	result.BuildPassed = true

	// Step 2: Vet
	vetOut, vetErr := e.runTool(ctx, "bash", map[string]string{
		"cmd": "go vet ./...",
	})
	if vetErr != nil {
		result.Output += "\n" + vetOut
		result.Error = fmt.Errorf("vet failed: %w", vetErr)
		return result
	}

	// Step 3: Test
	testOut, testErr := e.runTool(ctx, "bash", map[string]string{
		"cmd": "go test ./... -short",
	})
	result.Output += "\n" + testOut
	if testErr != nil {
		result.Error = fmt.Errorf("tests failed: %w", testErr)
		return result
	}
	result.TestPassed = true

	return result
}

// verifyProtected checks if any modified files are protected.
// Returns a list of protected files that were modified.
func (e *Engine) verifyProtected(ctx context.Context) ([]string, error) {
	out, err := e.runTool(ctx, "bash", map[string]string{
		"cmd": "git diff --name-only HEAD",
	})
	if err != nil {
		return nil, err
	}

	var violations []string
	for _, file := range strings.Split(strings.TrimSpace(out), "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		// Normalize absolute paths to relative before protection check.
		// git diff --name-only returns relative paths, but guard defensively.
		if strings.HasPrefix(file, "/") {
			if rel, err := filepath.Rel(e.repoPath, file); err == nil && !strings.HasPrefix(rel, "..") {
				file = rel
			}
		}
		if isProtected(file) {
			violations = append(violations, file)
		}
	}
	return violations, nil
}
