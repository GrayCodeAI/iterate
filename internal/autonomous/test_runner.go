// Package autonomous - Task 2: Autonomous test-running with failure analysis and auto-fix
package autonomous

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TestRunner provides autonomous test execution with failure analysis.
type TestRunner struct {
	engine    *Engine
	maxRetries int
	failurePatterns []FailurePattern
}

// FailurePattern represents a known failure pattern and its fix suggestion.
type FailurePattern struct {
	Pattern     string // Regex pattern to match in test output
	Category    string // "build_error", "test_failure", "race_condition", "timeout", "import_error"
	FixHint     string // Hint for the agent on how to fix
	AutoFixable bool   // Whether this can be auto-fixed
}

// TestResult holds the result of a test run.
type TestResult struct {
	Passed      bool
	Output      string
	BuildOutput string
	TestOutput  string
	Duration    time.Duration
	Failures    []TestFailure
	Summary     TestSummary
}

// TestFailure represents a single test failure.
type TestFailure struct {
	TestName    string
	Package     string
	Error       string
	File        string
	Line        int
	Expected    string
	Got         string
	Category    string
	FixSuggestion string
}

// TestSummary holds aggregated test statistics.
type TestSummary struct {
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	Coverage     float64
}

// NewTestRunner creates a new test runner.
func NewTestRunner(engine *Engine) *TestRunner {
	return &TestRunner{
		engine:     engine,
		maxRetries: 3,
		failurePatterns: DefaultFailurePatterns(),
	}
}

// DefaultFailurePatterns returns the built-in failure pattern detection rules.
func DefaultFailurePatterns() []FailurePattern {
	return []FailurePattern{
		{
			Pattern:     `undefined: (\w+)`,
			Category:    "build_error",
			FixHint:     "The identifier '$1' is not defined. Check imports or define the identifier.",
			AutoFixable: true,
		},
		{
			Pattern:     `cannot use ([^\s]+) as type ([^\s]+)`,
			Category:    "build_error",
			FixHint:     "Type mismatch: cannot use $1 as $2. Check type conversion.",
			AutoFixable: true,
		},
		{
			Pattern:     `imported and not used: "([^"]+)"`,
			Category:    "build_error",
			FixHint:     "Unused import '$1'. Remove the import or use it.",
			AutoFixable: true,
		},
		{
			Pattern:     `DATA RACE`,
			Category:    "race_condition",
			FixHint:     "Race condition detected. Use sync.Mutex or sync/atomic for shared data.",
			AutoFixable: true,
		},
		{
			Pattern:     `panic: ([^\n]+)`,
			Category:    "runtime_error",
			FixHint:     "Panic occurred: $1. Add error handling or nil checks.",
			AutoFixable: true,
		},
		{
			Pattern:     `nil pointer dereference`,
			Category:    "runtime_error",
			FixHint:     "Nil pointer dereference. Add nil check before accessing.",
			AutoFixable: true,
		},
		{
			Pattern:     `--- FAIL: ([^\s]+)`,
			Category:    "test_failure",
			FixHint:     "Test '$1' failed. Check the test output for details.",
			AutoFixable: false,
		},
		{
			Pattern:     `timeout`,
			Category:    "timeout",
			FixHint:     "Operation timed out. Consider increasing timeout or optimizing code.",
			AutoFixable: false,
		},
		{
			Pattern:     `no such file or directory`,
			Category:    "file_error",
			FixHint:     "File not found. Check file path or create the file.",
			AutoFixable: false,
		},
		{
			Pattern:     `permission denied`,
			Category:    "permission_error",
			FixHint:     "Permission denied. Check file permissions or run with appropriate privileges.",
			AutoFixable: false,
		},
	}
}

// RunTestsWithAnalysis runs tests and analyzes failures.
func (tr *TestRunner) RunTestsWithAnalysis(ctx context.Context) (*TestResult, error) {
	start := time.Now()
	result := &TestResult{}

	// Step 1: Build first
	buildOutput, buildErr := tr.engine.runCommand(ctx, "go build ./...")
	result.BuildOutput = buildOutput
	
	if buildErr != nil {
		result.Passed = false
		result.Output = buildOutput
		result.Failures = tr.analyzeBuildFailures(buildOutput)
		result.Duration = time.Since(start)
		return result, fmt.Errorf("build failed")
	}

	// Step 2: Run tests with coverage
	testOutput, testErr := tr.engine.runCommand(ctx, "go test -v -cover ./...")
	result.TestOutput = testOutput
	result.Output = testOutput
	result.Summary = tr.parseTestSummary(testOutput)

	if testErr != nil {
		result.Passed = false
		result.Failures = tr.analyzeTestFailures(testOutput)
	} else {
		result.Passed = true
	}

	result.Duration = time.Since(start)
	return result, nil
}

// RunTestsWithAutoFix runs tests and attempts to auto-fix failures.
func (tr *TestRunner) RunTestsWithAutoFix(ctx context.Context) (*TestResult, error) {
	for attempt := 1; attempt <= tr.maxRetries; attempt++ {
		result, err := tr.RunTestsWithAnalysis(ctx)
		
		if err == nil && result.Passed {
			tr.engine.logger.Info("Tests passed", "attempt", attempt)
			return result, nil
		}

		// Analyze failures for auto-fixable issues
		autoFixable := tr.findAutoFixableFailures(result.Failures)
		if len(autoFixable) == 0 {
			tr.engine.logger.Warn("No auto-fixable failures found", "attempt", attempt)
			return result, err
		}

		tr.engine.logger.Info("Attempting auto-fix", "attempt", attempt, "failures", len(autoFixable))

		// Generate fix suggestions
		fixPrompt := tr.buildFixPrompt(result, autoFixable)
		
		// Apply fixes via agent
		if tr.engine.agent != nil {
			for ev := range tr.engine.agent.Prompt(ctx, fixPrompt) {
				if tr.engine.eventSink != nil {
					tr.engine.eventSink <- ev
				}
			}
		}
	}

	return nil, fmt.Errorf("auto-fix failed after %d attempts", tr.maxRetries)
}

// analyzeBuildFailures extracts failures from build output.
func (tr *TestRunner) analyzeBuildFailures(output string) []TestFailure {
	var failures []TestFailure

	for _, pattern := range tr.failurePatterns {
		if pattern.Category != "build_error" {
			continue
		}

		re := regexp.MustCompile(pattern.Pattern)
		matches := re.FindAllStringSubmatch(output, -1)

		for _, match := range matches {
			failure := TestFailure{
				Category:    pattern.Category,
				Error:       match[0],
				FixSuggestion: tr.expandHint(pattern.FixHint, match),
			}

			if len(match) > 1 {
				failure.TestName = match[1]
			}

			failures = append(failures, failure)
		}
	}

	return failures
}

// analyzeTestFailures extracts failures from test output.
func (tr *TestRunner) analyzeTestFailures(output string) []TestFailure {
	var failures []TestFailure

	// Find all failed tests
	failPattern := regexp.MustCompile(`--- FAIL: ([^\s]+) \(([^\)]+)\)`)
	matches := failPattern.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		failure := TestFailure{
			TestName: match[1],
			Package:  match[2],
			Category: "test_failure",
		}

		// Extract error details
		errorPattern := regexp.MustCompile(fmt.Sprintf(`--- FAIL: %s[\s\S]*?Error:[\s]*(.*?)(?:\n|$)`, regexp.QuoteMeta(match[1])))
		if errMatch := errorPattern.FindStringSubmatch(output); len(errMatch) > 1 {
			failure.Error = errMatch[1]
		}

		// Match against known patterns
		for _, pattern := range tr.failurePatterns {
			if matched, _ := regexp.MatchString(pattern.Pattern, output); matched {
				failure.Category = pattern.Category
				failure.FixSuggestion = pattern.FixHint
				break
			}
		}

		failures = append(failures, failure)
	}

	return failures
}

// parseTestSummary extracts test statistics from output.
func (tr *TestRunner) parseTestSummary(output string) TestSummary {
	summary := TestSummary{}

	// Parse coverage
	coverPattern := regexp.MustCompile(`coverage:\s*([\d.]+)%`)
	if match := coverPattern.FindStringSubmatch(output); len(match) > 1 {
		fmt.Sscanf(match[1], "%f", &summary.Coverage)
	}

	// Parse test counts
	// Example: "PASS ok github.com/pkg/module 0.123s"
	// Example: "FAIL github.com/pkg/module [build failed]"
	

	// Count PASS/FAIL lines
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "=== RUN") {
			summary.TotalTests++
		}
		if strings.HasPrefix(line, "--- PASS") {
			summary.PassedTests++
		}
		if strings.HasPrefix(line, "--- FAIL") {
			summary.FailedTests++
		}
		if strings.HasPrefix(line, "--- SKIP") {
			summary.SkippedTests++
		}
	}

	return summary
}

// findAutoFixableFailures filters failures that can be auto-fixed.
func (tr *TestRunner) findAutoFixableFailures(failures []TestFailure) []TestFailure {
	var autoFixable []TestFailure

	for _, f := range failures {
		for _, pattern := range tr.failurePatterns {
			if pattern.AutoFixable && f.Category == pattern.Category {
				autoFixable = append(autoFixable, f)
				break
			}
		}
	}

	return autoFixable
}

// buildFixPrompt generates a prompt for the agent to fix failures.
func (tr *TestRunner) buildFixPrompt(result *TestResult, failures []TestFailure) string {
	var sb strings.Builder

	sb.WriteString("The following test failures need to be fixed:\n\n")

	for i, f := range failures {
		sb.WriteString(fmt.Sprintf("### Failure %d\n", i+1))
		if f.TestName != "" {
			sb.WriteString(fmt.Sprintf("Test: %s\n", f.TestName))
		}
		if f.Package != "" {
			sb.WriteString(fmt.Sprintf("Package: %s\n", f.Package))
		}
		if f.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", f.Error))
		}
		if f.FixSuggestion != "" {
			sb.WriteString(fmt.Sprintf("Suggestion: %s\n", f.FixSuggestion))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Test Output\n\n```\n")
	sb.WriteString(result.Output)
	sb.WriteString("\n```\n\n")

	sb.WriteString("Analyze the failures and make the necessary code changes to fix them. ")
	sb.WriteString("After fixing, run `go build ./... && go test ./...` to verify.\n")

	return sb.String()
}

// expandHint replaces $1, $2, etc. with matched groups.
func (tr *TestRunner) expandHint(hint string, match []string) string {
	result := hint
	for i := 1; i < len(match); i++ {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), match[i])
	}
	return result
}

// RunSpecificTests runs only the specified test functions.
func (tr *TestRunner) RunSpecificTests(ctx context.Context, testNames []string) (*TestResult, error) {
	start := time.Now()
	result := &TestResult{}

	// Build test arguments
	args := "go test -v -run"
	for _, name := range testNames {
		args += fmt.Sprintf(" %s", name)
	}
	args += " ./..."

	output, err := tr.engine.runCommand(ctx, args)
	result.TestOutput = output
	result.Output = output
	result.Passed = err == nil
	result.Duration = time.Since(start)

	if !result.Passed {
		result.Failures = tr.analyzeTestFailures(output)
	}

	return result, err
}

// RunBenchmarks runs benchmark tests.
func (tr *TestRunner) RunBenchmarks(ctx context.Context) (*TestResult, error) {
	start := time.Now()
	result := &TestResult{}

	output, err := tr.engine.runCommand(ctx, "go test -bench=. -benchmem ./...")
	result.TestOutput = output
	result.Output = output
	result.Passed = err == nil
	result.Duration = time.Since(start)

	return result, err
}

// DetectFlakyTests runs tests multiple times to detect flakiness.
func (tr *TestRunner) DetectFlakyTests(ctx context.Context, iterations int) ([]string, error) {
	results := make(map[string]int) // test -> failure count

	for i := 0; i < iterations; i++ {
		output, _ := tr.engine.runCommand(ctx, "go test ./...")
		
		failPattern := regexp.MustCompile(`--- FAIL: ([^\s]+)`)
		matches := failPattern.FindAllStringSubmatch(output, -1)

		for _, match := range matches {
			results[match[1]]++
		}
	}

	var flaky []string
	for test, failures := range results {
		if failures > 0 && failures < iterations {
			flaky = append(flaky, test)
		}
	}

	return flaky, nil
}
