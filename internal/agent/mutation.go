package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// MutationTestTool runs go-mutesting to find untested code paths.
// Requires: go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest
func MutationTestTool(repoPath string) Tool {
	return Tool{
		Name:        "mutation_test",
		Description: "Run mutation testing to find untested code paths.\nArgs: {\"pkg\": \"./internal/agent/...\"} (defaults to ./...)",
		Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pkg := iteragent.ArgStr(args, "pkg")
			if pkg == "" {
				pkg = "./..."
			}

			ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
			defer cancel()

			// Check if go-mutesting is installed
			which := exec.CommandContext(ctx, "which", "go-mutesting")
			which.Dir = repoPath
			if err := which.Run(); err != nil {
				// Fall back to basic coverage report
				return runCoverageReport(ctx, repoPath, pkg)
			}

			cmd := exec.CommandContext(ctx, "go-mutesting", pkg)
			cmd.Dir = repoPath
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			_ = cmd.Run()

			result := out.String()
			return summariseMutationResults(result), nil
		},
	}
}

// runCoverageReport is a fallback when go-mutesting is not installed.
// It runs go test with -cover and highlights packages with low coverage.
func runCoverageReport(ctx context.Context, repoPath, pkg string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-cover", "-covermode=atomic", pkg)
	cmd.Dir = repoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()

	lines := strings.Split(out.String(), "\n")
	var sb strings.Builder
	sb.WriteString("=== Coverage report (go-mutesting not installed, using go test -cover) ===\n\n")

	for _, line := range lines {
		if strings.Contains(line, "coverage:") {
			// Highlight low coverage
			if strings.Contains(line, "0.0%") || strings.Contains(line, "[no test") {
				sb.WriteString("LOW  " + line + "\n")
			} else {
				sb.WriteString("     " + line + "\n")
			}
		}
	}

	sb.WriteString("\nTo install mutation testing: go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest\n")
	return sb.String(), nil
}

// summariseMutationResults extracts the key stats from go-mutesting output.
func summariseMutationResults(raw string) string {
	lines := strings.Split(raw, "\n")
	var survived, killed, total int
	var survivedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PASS") {
			killed++
			total++
		} else if strings.HasPrefix(line, "FAIL") {
			survived++
			total++
			survivedLines = append(survivedLines, line)
		}
	}

	var sb strings.Builder
	sb.WriteString("=== Mutation testing results ===\n")
	sb.WriteString(fmt.Sprintf("Total: %d | Killed: %d | Survived: %d\n", total, killed, survived))

	if total > 0 {
		score := float64(killed) / float64(total) * 100
		sb.WriteString(fmt.Sprintf("Mutation score: %.1f%%\n", score))
	}

	if len(survivedLines) > 0 {
		sb.WriteString("\n=== Surviving mutants (untested paths) ===\n")
		// Show at most 20 to keep output manageable
		limit := 20
		if len(survivedLines) < limit {
			limit = len(survivedLines)
		}
		for _, l := range survivedLines[:limit] {
			sb.WriteString(l + "\n")
		}
		if len(survivedLines) > 20 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(survivedLines)-20))
		}
	}

	return sb.String()
}
