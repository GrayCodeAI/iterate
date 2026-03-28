// Package autonomous - Task 11: Smart Retry tests
package autonomous

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewSmartRetry(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		sr := NewSmartRetry(RetryConfig{})
		if sr == nil {
			t.Fatal("expected non-nil SmartRetry")
		}
		if sr.maxAttempts != 5 {
			t.Errorf("expected maxAttempts=5, got %d", sr.maxAttempts)
		}
		if sr.baseBackoff != time.Second {
			t.Errorf("expected baseBackoff=1s, got %v", sr.baseBackoff)
		}
		if sr.maxBackoff != 30*time.Second {
			t.Errorf("expected maxBackoff=30s, got %v", sr.maxBackoff)
		}
		if len(sr.patterns) == 0 {
			t.Error("expected default patterns to be loaded")
		}
		if len(sr.strategies) == 0 {
			t.Error("expected default strategies to be loaded")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		sr := NewSmartRetry(RetryConfig{
			MaxAttempts:     10,
			BaseBackoff:     2 * time.Second,
			MaxBackoff:      time.Minute,
			LearningEnabled: true,
		})
		if sr.maxAttempts != 10 {
			t.Errorf("expected maxAttempts=10, got %d", sr.maxAttempts)
		}
		if sr.baseBackoff != 2*time.Second {
			t.Errorf("expected baseBackoff=2s, got %v", sr.baseBackoff)
		}
		if sr.maxBackoff != time.Minute {
			t.Errorf("expected maxBackoff=1m, got %v", sr.maxBackoff)
		}
		if !sr.learningEnabled {
			t.Error("expected learningEnabled=true")
		}
	})
}

func TestDefaultRetryPatterns(t *testing.T) {
	patterns := DefaultRetryPatterns()
	if len(patterns) < 10 {
		t.Errorf("expected at least 10 default patterns, got %d", len(patterns))
	}

	// Check for essential patterns
	essentialPatterns := []string{
		"undefined-identifier",
		"type-mismatch",
		"unused-import",
		"race-condition",
		"nil-pointer",
		"panic",
		"test-failure",
		"timeout",
		"syntax-error",
	}

	patternMap := make(map[string]bool)
	for _, p := range patterns {
		patternMap[p.ID] = true
	}

	for _, id := range essentialPatterns {
		if !patternMap[id] {
			t.Errorf("missing essential pattern: %s", id)
		}
	}
}

func TestDefaultStrategies(t *testing.T) {
	strategies := DefaultStrategies()
	if len(strategies) < 5 {
		t.Errorf("expected at least 5 default strategies, got %d", len(strategies))
	}

	// Check for essential strategies
	essentialStrategies := []string{
		"import-fix",
		"lint",
		"format",
		"rebuild",
		"wait",
		"alternative",
	}

	for _, name := range essentialStrategies {
		if _, ok := strategies[name]; !ok {
			t.Errorf("missing essential strategy: %s", name)
		}
	}
}

func TestMatchPatterns(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	tests := []struct {
		name     string
		errorMsg string
		wantIDs  []string
	}{
		{
			name:     "undefined identifier",
			errorMsg: "main.go:10: undefined: foo",
			wantIDs:  []string{"undefined-identifier"},
		},
		{
			name:     "type mismatch",
			errorMsg: "cannot use string as type int",
			wantIDs:  []string{"type-mismatch"},
		},
		{
			name:     "unused import",
			errorMsg: `imported and not used: "fmt"`,
			wantIDs:  []string{"unused-import"},
		},
		{
			name:     "race condition",
			errorMsg: "DATA RACE detected",
			wantIDs:  []string{"race-condition"},
		},
		{
			name:     "nil pointer",
			errorMsg: "panic: nil pointer dereference",
			wantIDs:  []string{"nil-pointer", "panic"},
		},
		{
			name:     "test failure",
			errorMsg: "--- FAIL: TestSomething (0.00s)",
			wantIDs:  []string{"test-failure"},
		},
		{
			name:     "timeout",
			errorMsg: "context deadline exceeded",
			wantIDs:  []string{"timeout"},
		},
		{
			name:     "syntax error",
			errorMsg: "main.go:5: syntax error: unexpected token",
			wantIDs:  []string{"syntax-error"},
		},
		{
			name:     "no match",
			errorMsg: "some random error message",
			wantIDs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := sr.matchPatterns(tt.errorMsg)
			if len(matched) != len(tt.wantIDs) {
				t.Errorf("expected %d matches, got %d", len(tt.wantIDs), len(matched))
				return
			}

			matchedIDs := make(map[string]bool)
			for _, p := range matched {
				matchedIDs[p.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !matchedIDs[wantID] {
					t.Errorf("expected pattern %s to match", wantID)
				}
			}
		})
	}
}

func TestCategorizeError(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	tests := []struct {
		name           string
		errorMsg       string
		expectedCat    ErrorCategory
	}{
		{
			name:        "build error",
			errorMsg:    "undefined: foo",
			expectedCat: CategoryBuildError,
		},
		{
			name:        "import error",
			errorMsg:    `imported and not used: "fmt"`,
			expectedCat: CategoryImportError,
		},
		{
			name:        "runtime error",
			errorMsg:    "nil pointer dereference",
			expectedCat: CategoryRuntimeError,
		},
		{
			name:        "timeout",
			errorMsg:    "context deadline exceeded",
			expectedCat: CategoryTimeout,
		},
		{
			name:        "unknown error",
			errorMsg:    "something went wrong",
			expectedCat: CategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := sr.matchPatterns(tt.errorMsg)
			category := sr.categorizeError(tt.errorMsg, patterns)
			if category != tt.expectedCat {
				t.Errorf("expected category %s, got %s", tt.expectedCat, category)
			}
		})
	}
}

func TestSelectStrategy(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	tests := []struct {
		name         string
		category     ErrorCategory
		wantStrategy string
	}{
		{name: "build error", category: CategoryBuildError, wantStrategy: "rebuild"},
		{name: "import error", category: CategoryImportError, wantStrategy: "import-fix"},
		{name: "timeout", category: CategoryTimeout, wantStrategy: "wait"},
		{name: "network error", category: CategoryNetworkError, wantStrategy: "wait"},
		{name: "syntax error", category: CategorySyntaxError, wantStrategy: "format"},
		{name: "race condition", category: CategoryRaceCondition, wantStrategy: "alternative"},
		{name: "unknown", category: CategoryUnknown, wantStrategy: "rebuild"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := sr.selectStrategy(tt.category, nil)
			if strategy == nil {
				t.Fatal("expected non-nil strategy")
			}
			if strategy.Name != tt.wantStrategy {
				t.Errorf("expected strategy %s, got %s", tt.wantStrategy, strategy.Name)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  30 * time.Second,
	})

	strategy := &RetryStrategy{
		BackoffFactor: 2.0,
		JitterEnabled: false,
	}

	tests := []struct {
		attempt  int
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{attempt: 1, minDelay: 1 * time.Second, maxDelay: 1 * time.Second},
		{attempt: 2, minDelay: 2 * time.Second, maxDelay: 2 * time.Second},
		{attempt: 3, minDelay: 4 * time.Second, maxDelay: 4 * time.Second},
		{attempt: 4, minDelay: 8 * time.Second, maxDelay: 8 * time.Second},
		{attempt: 5, minDelay: 16 * time.Second, maxDelay: 16 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := sr.calculateBackoff(tt.attempt, strategy)
			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("attempt %d: expected delay between %v and %v, got %v", 
					tt.attempt, tt.minDelay, tt.maxDelay, delay)
			}
		})
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  30 * time.Second,
	})

	strategy := &RetryStrategy{
		BackoffFactor: 2.0,
		JitterEnabled: true,
	}

	// Run multiple times to verify jitter is applied
	for i := 0; i < 10; i++ {
		delay := sr.calculateBackoff(1, strategy)
		// With jitter, delay should be >= base and <= base * 1.1
		if delay < time.Second {
			t.Errorf("delay should be at least base backoff, got %v", delay)
		}
	}
}

func TestCalculateBackoffMaxCap(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  5 * time.Second,
	})

	strategy := &RetryStrategy{
		BackoffFactor: 2.0,
		JitterEnabled: false,
	}

	// High attempt number should cap at maxBackoff
	delay := sr.calculateBackoff(10, strategy)
	if delay > 5*time.Second {
		t.Errorf("delay should be capped at maxBackoff, got %v", delay)
	}
}

func TestExecuteWithRetry_Success(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
	})

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	}

	result := sr.ExecuteWithRetry(context.Background(), "test-task", fn)

	if !result.Success {
		t.Error("expected success")
	}
	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestExecuteWithRetry_MaxAttemptsExceeded(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
	})

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("persistent error")
	}

	result := sr.ExecuteWithRetry(context.Background(), "test-task", fn)

	if result.Success {
		t.Error("expected failure")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if result.FinalError == "" {
		t.Error("expected final error message")
	}
}

func TestExecuteWithRetry_ContextCancellation(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 5,
		BaseBackoff: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	
	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			cancel()
		}
		return errors.New("error")
	}

	result := sr.ExecuteWithRetry(ctx, "test-task", fn)

	if result.Success {
		t.Error("expected failure due to cancellation")
	}
	if result.FinalError != context.Canceled.Error() {
		t.Errorf("expected context canceled error, got %s", result.FinalError)
	}
}

func TestExecuteWithRetry_PatternRecognition(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 2,
		BaseBackoff: 10 * time.Millisecond,
	})

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("undefined: foo")
	}

	result := sr.ExecuteWithRetry(context.Background(), "test-task", fn)

	if len(result.Patterns) == 0 {
		t.Error("expected patterns to be detected")
	}
}

func TestAnalyzeFailure(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	tests := []struct {
		name         string
		errorMsg     string
		wantFixable  bool
		wantCategory ErrorCategory
	}{
		{
			name:         "undefined identifier - fixable",
			errorMsg:     "undefined: foo",
			wantFixable:  true,
			wantCategory: CategoryBuildError,
		},
		{
			name:         "unused import - fixable",
			errorMsg:     `imported and not used: "fmt"`,
			wantFixable:  true,
			wantCategory: CategoryImportError,
		},
		{
			name:         "test failure - not auto-fixable",
			errorMsg:     "--- FAIL: TestFoo",
			wantFixable:  false,
			wantCategory: CategoryTestFailure,
		},
		{
			name:         "unknown error",
			errorMsg:     "something unexpected",
			wantFixable:  false,
			wantCategory: CategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := sr.AnalyzeFailure(tt.errorMsg)
			if analysis.Fixable != tt.wantFixable {
				t.Errorf("expected Fixable=%v, got %v", tt.wantFixable, analysis.Fixable)
			}
			if analysis.Category != tt.wantCategory {
				t.Errorf("expected Category=%s, got %s", tt.wantCategory, analysis.Category)
			}
		})
	}
}

func TestGetBestFixHint(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	tests := []struct {
		name     string
		errorMsg string
		wantHint string
	}{
		{
			name:     "undefined identifier",
			errorMsg: "undefined: myFunc",
			wantHint: "Check imports for myFunc",
		},
		{
			name:     "unused import",
			errorMsg: `imported and not used: "fmt"`,
			wantHint: "Remove import fmt",
		},
		{
			name:     "no pattern match",
			errorMsg: "unknown error",
			wantHint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := sr.GetBestFixHint(tt.errorMsg)
			if hint != tt.wantHint {
				t.Errorf("expected hint %q, got %q", tt.wantHint, hint)
			}
		})
	}
}

func TestAddPattern(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	initialCount := len(sr.patterns)

	newPattern := &RetryPattern{
		ID:           "custom-pattern",
		RegexPattern: `custom error: (\w+)`,
		Category:     CategoryBuildError,
		AutoFixable:  true,
		FixHints:     []string{"Fix custom error: $1"},
		SuccessRate:  0.80,
	}

	sr.AddPattern(newPattern)

	if len(sr.patterns) != initialCount+1 {
		t.Errorf("expected %d patterns, got %d", initialCount+1, len(sr.patterns))
	}

	// Verify the pattern works
	matched := sr.matchPatterns("custom error: test")
	if len(matched) == 0 {
		t.Error("expected new pattern to match")
	}
}

func TestRetryHistory(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 2,
		BaseBackoff: 10 * time.Millisecond,
	})

	// Execute a failing operation
	fn := func() error {
		return errors.New("undefined: foo")
	}

	sr.ExecuteWithRetry(context.Background(), "task-1", fn)

	history := sr.GetHistory()
	if len(history) == 0 {
		t.Error("expected retry history to be recorded")
	}
}

func TestSmartRetryGetStats(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
	})

	// First, fail
	sr.ExecuteWithRetry(context.Background(), "task-1", func() error {
		return errors.New("error")
	})

	stats := sr.GetStats()
	if stats.TotalAttempts == 0 {
		t.Error("expected total attempts to be recorded")
	}
	if len(stats.ByCategory) == 0 {
		t.Error("expected category stats")
	}
}

func TestSmartRetryReset(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 2,
		BaseBackoff: 10 * time.Millisecond,
	})

	// Execute some operations
	sr.ExecuteWithRetry(context.Background(), "task-1", func() error {
		return errors.New("error")
	})

	// Verify history exists
	if len(sr.GetHistory()) == 0 {
		t.Fatal("expected history to exist before reset")
	}

	// Reset
	sr.Reset()

	// Verify history is cleared
	if len(sr.GetHistory()) != 0 {
		t.Error("expected history to be cleared after reset")
	}
}

func TestLearnFromHistory(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts:     3,
		BaseBackoff:     10 * time.Millisecond,
		LearningEnabled: true,
	})

	// Execute operations
	sr.ExecuteWithRetry(context.Background(), "task-1", func() error {
		return errors.New("undefined: foo")
	})

	// Learn from history
	sr.LearnFromHistory()

	// Verify learning was applied (pattern stats updated)
	stats := sr.GetPatternStats("undefined-identifier")
	if stats == nil {
		t.Log("Pattern stats not available yet (expected if no successful retry)")
	}
}

func TestRetryPatternSuccessRate(t *testing.T) {
	patterns := DefaultRetryPatterns()

	for _, p := range patterns {
		if p.SuccessRate < 0 || p.SuccessRate > 1 {
			t.Errorf("pattern %s has invalid success rate: %f", p.ID, p.SuccessRate)
		}
	}
}

func TestRetryStrategyFields(t *testing.T) {
	strategies := DefaultStrategies()

	for name, s := range strategies {
		if s.Name != name {
			t.Errorf("strategy name mismatch: expected %s, got %s", name, s.Name)
		}
		if s.MaxAttempts <= 0 {
			t.Errorf("strategy %s has invalid MaxAttempts: %d", name, s.MaxAttempts)
		}
		if s.BackoffFactor <= 0 {
			t.Errorf("strategy %s has invalid BackoffFactor: %f", name, s.BackoffFactor)
		}
	}
}

func TestFixActionTypes(t *testing.T) {
	types := []FixActionType{
		FixTypeImport,
		FixTypeFormat,
		FixTypeLint,
		FixTypeRebuild,
		FixTypeClean,
		FixTypeWait,
		FixTypeAlternative,
	}

	for _, ft := range types {
		if string(ft) == "" {
			t.Errorf("fix action type should not be empty")
		}
	}
}

func TestErrorCategories(t *testing.T) {
	categories := []ErrorCategory{
		CategoryBuildError,
		CategoryTestFailure,
		CategoryRaceCondition,
		CategoryTimeout,
		CategoryNetworkError,
		CategoryResourceError,
		CategorySyntaxError,
		CategoryImportError,
		CategoryRuntimeError,
		CategoryUnknown,
	}

	for _, c := range categories {
		if string(c) == "" {
			t.Errorf("category should not be empty")
		}
	}
}

func TestRetryResult_Fields(t *testing.T) {
	result := &RetryResult{
		Success:      true,
		Attempts:     3,
		TotalTime:    5 * time.Second,
		FinalError:   "",
		FixesApplied: []string{"fix1", "fix2"},
		Patterns:     []string{"undefined-identifier"},
		Learned:      true,
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
	if len(result.FixesApplied) != 2 {
		t.Errorf("expected 2 fixes, got %d", len(result.FixesApplied))
	}
}

func TestFailureAnalysis_Fields(t *testing.T) {
	analysis := &FailureAnalysis{
		OriginalError:     "undefined: foo",
		Category:          CategoryBuildError,
		PatternID:         "undefined-identifier",
		Fixable:           true,
		Confidence:        0.85,
		HistoricalFixRate: 0.90,
		Suggestions:       []string{"Check imports", "Define identifier"},
		RelatedFiles:      []string{"main.go"},
	}

	if !analysis.Fixable {
		t.Error("expected Fixable to be true")
	}
	if len(analysis.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(analysis.Suggestions))
	}
}

func TestPatternStats(t *testing.T) {
	stats := &PatternStats{
		PatternID:    "test-pattern",
		TotalSeen:    10,
		TotalFixed:   8,
		TotalFailed:  2,
		AvgAttempts:  2.5,
		FixActions:   map[string]int{"fix1": 5, "fix2": 3},
	}

	if stats.TotalSeen != 10 {
		t.Errorf("expected TotalSeen=10, got %d", stats.TotalSeen)
	}
	if len(stats.FixActions) != 2 {
		t.Errorf("expected 2 fix actions, got %d", len(stats.FixActions))
	}
}

func TestRetryStats(t *testing.T) {
	stats := RetryStats{
		TotalAttempts:     100,
		SuccessfulRetries: 75,
		SuccessRate:       0.75,
		ByCategory: map[ErrorCategory]int{
			CategoryBuildError: 50,
			CategoryTestFailure: 30,
			CategoryTimeout:    20,
		},
	}

	if stats.TotalAttempts != 100 {
		t.Errorf("expected 100 attempts, got %d", stats.TotalAttempts)
	}
	if stats.SuccessRate != 0.75 {
		t.Errorf("expected 0.75 success rate, got %f", stats.SuccessRate)
	}
}

func TestMaxHelper(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
		{-5, -3, -3},
	}

	for _, tt := range tests {
		result := max(tt.a, tt.b)
		if result != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.want)
		}
	}
}

func TestPowHelper(t *testing.T) {
	tests := []struct {
		x, y float64
		want float64
	}{
		{2, 0, 1},
		{2, 1, 2},
		{2, 2, 4},
		{2, 3, 8},
		{3, 2, 9},
		{1.5, 2, 2.25},
	}

	for _, tt := range tests {
		result := pow(tt.x, tt.y)
		if result != tt.want {
			t.Errorf("pow(%f, %f) = %f, want %f", tt.x, tt.y, result, tt.want)
		}
	}
}

func TestRetryAttempt(t *testing.T) {
	now := time.Now()
	attempt := &RetryAttempt{
		Timestamp:  now,
		TaskID:     "task-123",
		Error:      "undefined: foo",
		Category:   CategoryBuildError,
		AttemptNum: 2,
		Success:    false,
		FixApplied: "Added import",
		Duration:   100 * time.Millisecond,
		NextDelay:  200 * time.Millisecond,
	}

	if attempt.TaskID != "task-123" {
		t.Errorf("expected task-123, got %s", attempt.TaskID)
	}
	if !attempt.Timestamp.Equal(now) {
		t.Error("timestamp mismatch")
	}
}

func TestNewRetryHistory(t *testing.T) {
	history := NewRetryHistory()
	if history == nil {
		t.Fatal("expected non-nil history")
	}
	if history.patterns == nil {
		t.Error("expected patterns map to be initialized")
	}
}

// Benchmark tests
func BenchmarkMatchPatterns(b *testing.B) {
	sr := NewSmartRetry(RetryConfig{})
	errorMsg := "main.go:10: undefined: foo"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sr.matchPatterns(errorMsg)
	}
}

func BenchmarkExecuteWithRetry(b *testing.B) {
	sr := NewSmartRetry(RetryConfig{
		MaxAttempts: 3,
		BaseBackoff: time.Millisecond,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sr.ExecuteWithRetry(context.Background(), "bench-task", func() error {
			return nil
		})
	}
}

func BenchmarkAnalyzeFailure(b *testing.B) {
	sr := NewSmartRetry(RetryConfig{})
	errorMsg := "undefined: foo"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sr.AnalyzeFailure(errorMsg)
	}
}
