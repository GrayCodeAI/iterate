// Package autonomous - Task 20: Learning from Autonomous Failures tests
package autonomous

import (
	"path/filepath"
	"testing"
)

func TestNewFailureLearner(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	if fl == nil {
		t.Fatal("expected failure learner, got nil")
	}

	if !fl.enabled {
		t.Error("expected learner to be enabled")
	}
}

func TestRecordFailure(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	if learning == nil {
		t.Fatal("expected learning, got nil")
	}

	if learning.TaskType != "build" {
		t.Errorf("expected task type 'build', got '%s'", learning.TaskType)
	}

	if learning.ErrorMessage != "undefined: foo" {
		t.Errorf("expected error message 'undefined: foo', got '%s'", learning.ErrorMessage)
	}

	stats := fl.GetStats()
	if stats.TotalFailures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.TotalFailures)
	}
}

func TestRecordSimilarFailures(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	// Record same error twice
	fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordFailure("build", "task-2", "undefined: foo", nil)

	stats := fl.GetStats()
	if stats.TotalFailures != 2 {
		t.Errorf("expected 2 failures, got %d", stats.TotalFailures)
	}

	learnings := fl.GetRecentLearnings(10)
	if len(learnings) != 1 {
		t.Errorf("expected 1 unique learning, got %d", len(learnings))
	}

	if learnings[0].Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", learnings[0].Attempts)
	}
}

func TestRecordSolution(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	actions := []LearningAction{
		{Type: "code_change", Description: "Added import for foo", Success: true},
	}

	fl.RecordSolution(learning.ID, "Add missing import", actions)

	learnings := fl.GetRecentLearnings(1)
	if len(learnings) == 0 {
		t.Fatal("expected learning, got none")
	}

	if learnings[0].Solution != "Add missing import" {
		t.Errorf("expected solution 'Add missing import', got '%s'", learnings[0].Solution)
	}

	if learnings[0].SuccessRate != 1.0 {
		t.Errorf("expected success rate 1.0, got %f", learnings[0].SuccessRate)
	}
}

func TestGetRecommendation(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	// Record a failure and solution
	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordSolution(learning.ID, "Add import", []LearningAction{
		{Type: "code_change", Description: "Add import", Success: true},
	})

	// Get recommendation for similar error
	rec := fl.GetRecommendation("undefined: foo")

	if rec == nil {
		t.Fatal("expected recommendation, got nil")
	}

	if len(rec.SimilarCases) == 0 {
		t.Error("expected similar cases")
	}

	if len(rec.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestGetRecommendationWithSmartRetry(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{LearningEnabled: true})
	fl := NewFailureLearner(FailureLearnerConfig{
		Enabled:    true,
		SmartRetry: sr,
	})

	// Get recommendation for an error SmartRetry knows
	rec := fl.GetRecommendation("undefined: myFunc")

	if rec.Category != CategoryBuildError {
		t.Errorf("expected category build_error, got %s", rec.Category)
	}
}

func TestApplyLearning(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	// Record failure and solution
	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordSolution(learning.ID, "Add import", []LearningAction{
		{Type: "code_change", Description: "Add import", Success: true},
	})

	// Get recommendation and apply
	rec := fl.GetRecommendation("undefined: foo")
	applied := fl.ApplyLearning(rec)

	if !applied {
		t.Error("expected learning to be applied")
	}

	stats := fl.GetStats()
	if stats.SuccessfulApplies != 1 {
		t.Errorf("expected 1 successful apply, got %d", stats.SuccessfulApplies)
	}
}

func TestLearnedPatterns(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	// Record failures with pattern
	fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordFailure("build", "task-2", "undefined: bar", nil)

	patterns := fl.GetPatterns()
	if len(patterns) == 0 {
		t.Error("expected patterns to be learned")
	}
}

func TestFailureLearnerBuilder(t *testing.T) {
	sr := NewSmartRetry(RetryConfig{})

	fl := NewFailureLearnerBuilder().
		WithEnabled(true).
		WithMaxLearnings(5000).
		WithSmartRetry(sr).
		WithStoragePath("/tmp/test.json").
		Build()

	if fl == nil {
		t.Fatal("expected failure learner, got nil")
	}

	if !fl.enabled {
		t.Error("expected enabled")
	}

	if fl.maxLearnings != 5000 {
		t.Errorf("expected max learnings 5000, got %d", fl.maxLearnings)
	}
}

func TestExportLearnings(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	json, err := fl.ExportLearnings()
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if json == "" {
		t.Error("expected JSON output")
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "learnings.json")

	// Create and save
	fl1 := NewFailureLearner(FailureLearnerConfig{
		Enabled:     true,
		StoragePath: storagePath,
	})

	fl1.RecordFailure("build", "task-1", "undefined: foo", nil)

	if err := fl1.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load in new instance
	fl2 := NewFailureLearner(FailureLearnerConfig{
		Enabled:     true,
		StoragePath: storagePath,
	})

	stats := fl2.GetStats()
	if stats.TotalFailures != 1 {
		t.Errorf("expected 1 failure after load, got %d", stats.TotalFailures)
	}
}

func TestVerifyLearning(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	fl.VerifyLearning(learning.ID)

	unverified := fl.GetUnverifiedLearnings()
	if len(unverified) != 0 {
		t.Error("expected no unverified learnings")
	}
}

func TestGetRecentLearnings(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	// Record multiple failures
	for i := 0; i < 5; i++ {
		fl.RecordFailure("build", "task-"+string(rune('0'+i)), "undefined: foo"+string(rune('0'+i)), nil)
	}

	recent := fl.GetRecentLearnings(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent learnings, got %d", len(recent))
	}
}

func TestClearFailureLearner(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	fl.Clear()

	stats := fl.GetStats()
	if stats.TotalFailures != 0 {
		t.Errorf("expected 0 failures after clear, got %d", stats.TotalFailures)
	}
}

func TestFailureLearnerGenerateReport(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordFailure("test", "task-2", "--- FAIL: TestBar", nil)

	report := fl.GenerateReport()

	if report == "" {
		t.Fatal("expected report, got empty string")
	}

	if !containsFLStr(report, "Total Failures") {
		t.Error("report missing failure count")
	}
}

func containsFLStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDisabledLearner(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: false})

	learning := fl.RecordFailure("build", "task-1", "undefined: foo", nil)

	if learning != nil {
		t.Error("expected nil when disabled")
	}
}

func TestMaxLearningsLimit(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{
		Enabled:      true,
		MaxLearnings: 10,
	})

	// Record more than max
	for i := 0; i < 20; i++ {
		fl.RecordFailure("build", "task-"+string(rune('0'+i)), "error"+string(rune('0'+i)), nil)
	}

	learnings := fl.GetRecentLearnings(100)
	if len(learnings) > 10 {
		t.Errorf("expected at most 10 learnings, got %d", len(learnings))
	}
}

func TestCategoryStats(t *testing.T) {
	fl := NewFailureLearner(FailureLearnerConfig{Enabled: true})

	fl.RecordFailure("build", "task-1", "undefined: foo", nil)
	fl.RecordFailure("test", "task-2", "--- FAIL: TestBar", nil)
	fl.RecordFailure("build", "task-3", "undefined: baz", nil)

	stats := fl.GetStats()
	if stats.ByTaskType["build"] != 2 {
		t.Errorf("expected 2 build failures, got %d", stats.ByTaskType["build"])
	}
	if stats.ByTaskType["test"] != 1 {
		t.Errorf("expected 1 test failure, got %d", stats.ByTaskType["test"])
	}
}

func TestTask20FullIntegration(t *testing.T) {
	// Create integrated system with SmartRetry
	sr := NewSmartRetry(RetryConfig{LearningEnabled: true})

	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "learnings.json")

	fl := NewFailureLearnerBuilder().
		WithEnabled(true).
		WithSmartRetry(sr).
		WithStoragePath(storagePath).
		Build()

	// Simulate autonomous failure workflow
	// 1. Record failure
	learning1 := fl.RecordFailure("autonomous", "auto-task-1", "undefined: ProcessData", map[string]any{
		"files": []string{"processor.go", "main.go"},
		"step":  "code_generation",
	})

	if learning1 == nil {
		t.Fatal("expected learning, got nil")
	}

	// 2. Record solution after fix
	fl.RecordSolution(learning1.ID, "Add import for ProcessData", []LearningAction{
		{Type: "code_change", Description: "Added import statement", File: "main.go", Success: true},
	})

	// 3. Record another similar failure
	learning2 := fl.RecordFailure("autonomous", "auto-task-2", "undefined: HandleRequest", map[string]any{
		"files": []string{"handler.go"},
		"step":  "code_generation",
	})

	// 4. Get recommendation for new similar error
	rec := fl.GetRecommendation("undefined: NewFunction")

	if rec.Category != CategoryBuildError {
		t.Errorf("expected build_error category, got %s", rec.Category)
	}

	// 5. Apply learning
	if len(rec.SimilarCases) > 0 {
		applied := fl.ApplyLearning(rec)
		t.Logf("Applied learning: %v", applied)
	}

	// 6. Verify and save
	fl.VerifyLearning(learning1.ID)
	fl.VerifyLearning(learning2.ID)

	if err := fl.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// 7. Generate report
	report := fl.GenerateReport()
	t.Logf("Report:\n%s", report)

	// Verify stats
	stats := fl.GetStats()
	if stats.TotalFailures != 2 {
		t.Errorf("expected 2 failures, got %d", stats.TotalFailures)
	}

	if stats.PatternsLearned == 0 {
		t.Error("expected patterns to be learned")
	}

	t.Logf("✅ Task 20: Learning from Autonomous Failures - Full integration PASSED")
	t.Logf("Failures: %d, Learnings: %d, Patterns: %d",
		stats.TotalFailures, stats.TotalLearnings, stats.PatternsLearned)
}

func TestErrorNormalization(t *testing.T) {
	tests := []struct {
		name     string
		error1   string
		error2   string
		sameHash bool
	}{
		{
			name:     "same error",
			error1:   "undefined: foo",
			error2:   "undefined: foo",
			sameHash: true,
		},
		{
			name:     "different error",
			error1:   "undefined: foo",
			error2:   "undefined: bar",
			sameHash: false,
		},
		{
			name:     "different line numbers",
			error1:   "error at line 42",
			error2:   "error at line 100",
			sameHash: true,
		},
		{
			name:     "different file paths",
			error1:   "/home/user/project/main.go: undefined",
			error2:   "/home/other/project/main.go: undefined",
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := hashError(tt.error1)
			hash2 := hashError(tt.error2)

			if tt.sameHash && hash1 != hash2 {
				t.Errorf("expected same hash for %q and %q", tt.error1, tt.error2)
			}
			if !tt.sameHash && hash1 == hash2 {
				t.Errorf("expected different hash for %q and %q", tt.error1, tt.error2)
			}
		})
	}
}

func TestPatternExtraction(t *testing.T) {
	tests := []struct {
		errorMsg   string
		shouldFind bool
	}{
		{"undefined: myFunc", true},
		{"cannot use string as type int", true},
		{"nil pointer dereference", true},
		{"--- FAIL: TestMain", true},
		{"syntax error: unexpected token", true},
		{"random error message", false},
	}

	for _, tt := range tests {
		pattern := extractErrorPattern(tt.errorMsg)
		if tt.shouldFind && pattern == "" {
			t.Errorf("expected pattern for %q", tt.errorMsg)
		}
		if !tt.shouldFind && pattern != "" {
			t.Errorf("unexpected pattern for %q: %s", tt.errorMsg, pattern)
		}
	}
}
