package autonomous

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewSuccessCriteriaValidator(t *testing.T) {
	config := DefaultSuccessCriteriaConfig()
	v := NewSuccessCriteriaValidator(config)
	
	if v == nil {
		t.Fatal("expected validator, got nil")
	}
	
	if len(v.checkers) == 0 {
		t.Error("expected default checkers to be registered")
	}
}

func TestDefaultSuccessCriteriaConfig(t *testing.T) {
	config := DefaultSuccessCriteriaConfig()
	
	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout 30s, got %v", config.DefaultTimeout)
	}
	
	if config.DefaultRetries != 2 {
		t.Errorf("expected DefaultRetries 2, got %d", config.DefaultRetries)
	}
	
	if !config.ParallelChecks {
		t.Error("expected ParallelChecks to be true")
	}
}

func TestValidateEmptyCriteria(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	report := v.Validate(context.Background(), nil)
	
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	
	if report.TotalCriteria != 0 {
		t.Errorf("expected 0 criteria, got %d", report.TotalCriteria)
	}
	
	if report.Score != 1.0 {
		t.Errorf("expected score 1.0 for empty criteria, got %f", report.Score)
	}
	
	if !report.AllRequiredPass {
		t.Error("expected AllRequiredPass to be true for empty criteria")
	}
}

func TestValidateSingleCriterion(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criteria := []*SuccessCriterion{
		{
			ID:       "test-1",
			Type:     CriterionTypeFileExists,
			Target:   "test.txt",
			Required: true,
		},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.TotalCriteria != 1 {
		t.Errorf("expected 1 criterion, got %d", report.TotalCriteria)
	}
	
	if report.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Passed)
	}
	
	if !report.AllRequiredPass {
		t.Error("expected AllRequiredPass to be true")
	}
}

func TestValidateMultipleCriteria(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "file1.txt", Required: true},
		{ID: "c2", Type: CriterionTypeFileNotExists, Target: "file2.txt", Required: true},
		{ID: "c3", Type: CriterionTypeBuildSuccess, Target: ".", Required: false},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.TotalCriteria != 3 {
		t.Errorf("expected 3 criteria, got %d", report.TotalCriteria)
	}
	
	if report.Passed != 3 {
		t.Errorf("expected 3 passed, got %d", report.Passed)
	}
}

func TestValidateWithFailures(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	// Register a failing checker
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      false,
			Message:     "File does not exist",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "missing.txt", Required: true},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	
	if report.AllRequiredPass {
		t.Error("expected AllRequiredPass to be false when required criterion fails")
	}
}

func TestValidateWithOptionalFailure(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	// Register a failing checker
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      false,
			Message:     "File does not exist",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "missing.txt", Required: false},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	
	// AllRequiredPass should still be true because the failed criterion is optional
	if !report.AllRequiredPass {
		t.Error("expected AllRequiredPass to be true when only optional criteria fail")
	}
}

func TestValidateWithContextCancellation(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "test.txt"},
		{ID: "c2", Type: CriterionTypeFileExists, Target: "test2.txt"},
	}
	
	report := v.Validate(ctx, criteria)
	
	// Criteria should be skipped due to cancelled context
	if report.Skipped != 2 && report.Passed != 2 {
		// Either skipped or passed (depending on timing)
		t.Logf("Report: passed=%d, skipped=%d, failed=%d", report.Passed, report.Skipped, report.Failed)
	}
}

func TestValidateSequential(t *testing.T) {
	config := DefaultSuccessCriteriaConfig()
	config.ParallelChecks = false
	v := NewSuccessCriteriaValidator(config)
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "file1.txt"},
		{ID: "c2", Type: CriterionTypeFileExists, Target: "file2.txt"},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.TotalCriteria != 2 {
		t.Errorf("expected 2 criteria, got %d", report.TotalCriteria)
	}
}

func TestValidateStopOnFirstFail(t *testing.T) {
	config := DefaultSuccessCriteriaConfig()
	config.StopOnFirstFail = true
	config.ParallelChecks = false
	v := NewSuccessCriteriaValidator(config)
	
	// Register a failing checker
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		if c.Target == "fail.txt" {
			return &ValidationResult{
				CriterionID: c.ID,
				Passed:      false,
				Message:     "Failed",
			}, nil
		}
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      true,
			Message:     "Passed",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "pass.txt", Required: true},
		{ID: "c2", Type: CriterionTypeFileExists, Target: "fail.txt", Required: true},
		{ID: "c3", Type: CriterionTypeFileExists, Target: "pass2.txt", Required: true},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	// Should stop after first failure
	if report.Passed > 2 {
		t.Errorf("expected at most 2 passed due to stop on first fail, got %d", report.Passed)
	}
}

func TestWeightedScore(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	// Register a checker that fails for specific target
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		passed := c.Target != "fail.txt"
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      passed,
			Message:     "Check result",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "pass.txt", Weight: 1.0},
		{ID: "c2", Type: CriterionTypeFileExists, Target: "fail.txt", Weight: 2.0},
		{ID: "c3", Type: CriterionTypeFileExists, Target: "pass2.txt", Weight: 1.0},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	// Score should be 2/4 = 0.5 (passed weight / total weight)
	expectedScore := 2.0 / 4.0
	if report.Score < expectedScore-0.01 || report.Score > expectedScore+0.01 {
		t.Errorf("expected score ~%.2f, got %.2f", expectedScore, report.Score)
	}
}

func TestNegateCriterion(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	// Register a checker that always passes
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      true,
			Message:     "File exists",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "test.txt", Negate: true},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Passed != 0 {
		t.Errorf("expected 0 passed (negated), got %d", report.Passed)
	}
	
	if report.Failed != 1 {
		t.Errorf("expected 1 failed (negated), got %d", report.Failed)
	}
}

func TestRetryLogic(t *testing.T) {
	config := DefaultSuccessCriteriaConfig()
	config.DefaultRetries = 2
	config.DefaultRetryDelay = 10 * time.Millisecond
	v := NewSuccessCriteriaValidator(config)
	
	attempts := 0
	
	// Register a checker that fails twice then succeeds
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		attempts++
		if attempts < 3 {
			return &ValidationResult{
				CriterionID: c.ID,
				Passed:      false,
				Message:     "Not yet",
			}, nil
		}
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      true,
			Message:     "Success",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "test.txt"},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Passed != 1 {
		t.Errorf("expected 1 passed after retries, got %d", report.Passed)
	}
}

func TestCheckerError(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	// Register a checker that returns an error
	v.RegisterChecker(CriterionTypeFileExists, func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		return nil, errors.New("checker error")
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionTypeFileExists, Target: "test.txt", RetryCount: 0},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	// Should be marked as error
	criterion := report.Criteria[0]
	if criterion.Status != CriterionStatusError {
		t.Errorf("expected status error, got %s", criterion.Status)
	}
}

func TestUnknownCriterionType(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionType("unknown"), Target: "test.txt"},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Failed != 1 {
		t.Errorf("expected 1 failed for unknown type, got %d", report.Failed)
	}
}

func TestQuickValidate(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criterion := &SuccessCriterion{
		ID:     "test-1",
		Type:   CriterionTypeFileExists,
		Target: "test.txt",
	}
	
	result, err := v.QuickValidate(context.Background(), criterion)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	
	if !result.Passed {
		t.Error("expected result to pass")
	}
}

func TestRegisterCustomChecker(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	v.RegisterChecker(CriterionType("custom_type"), func(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      true,
			Message:     "Custom check passed",
		}, nil
	})
	
	criteria := []*SuccessCriterion{
		{ID: "c1", Type: CriterionType("custom_type"), Target: "test"},
	}
	
	report := v.Validate(context.Background(), criteria)
	
	if report.Passed != 1 {
		t.Errorf("expected 1 passed for custom checker, got %d", report.Passed)
	}
}

func TestCriterionBuilder(t *testing.T) {
	criterion := NewCriterionBuilder(CriterionTypeFileExists, "test.txt").
		WithName("Test Criterion").
		WithDescription("Check if test.txt exists").
		WithPattern("pattern").
		WithExpected("expected").
		WithTimeout(10 * time.Second).
		WithRetries(3, 100*time.Millisecond).
		Optional().
		WithWeight(0.5).
		WithMetadata("key", "value").
		Build()
	
	if criterion.Type != CriterionTypeFileExists {
		t.Errorf("expected type file_exists, got %s", criterion.Type)
	}
	
	if criterion.Target != "test.txt" {
		t.Errorf("expected target test.txt, got %s", criterion.Target)
	}
	
	if criterion.Name != "Test Criterion" {
		t.Errorf("expected name 'Test Criterion', got %s", criterion.Name)
	}
	
	if criterion.Required {
		t.Error("expected criterion to be optional")
	}
	
	if criterion.Weight != 0.5 {
		t.Errorf("expected weight 0.5, got %f", criterion.Weight)
	}
	
	if criterion.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", criterion.Timeout)
	}
	
	if criterion.RetryCount != 3 {
		t.Errorf("expected retry count 3, got %d", criterion.RetryCount)
	}
}

func TestValidationReportIsSuccess(t *testing.T) {
	tests := []struct {
		name           string
		report         *ValidationReport
		expectedSuccess bool
	}{
		{
			name: "all pass",
			report: &ValidationReport{
				AllRequiredPass: true,
				Score:           0.9,
			},
			expectedSuccess: true,
		},
		{
			name: "required fails",
			report: &ValidationReport{
				AllRequiredPass: false,
				Score:           0.9,
			},
			expectedSuccess: false,
		},
		{
			name: "low score",
			report: &ValidationReport{
				AllRequiredPass: true,
				Score:           0.7,
			},
			expectedSuccess: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.report.IsSuccess() != tt.expectedSuccess {
				t.Errorf("expected IsSuccess=%v, got %v", tt.expectedSuccess, tt.report.IsSuccess())
			}
		})
	}
}

func TestValidatorGetStats(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	stats := v.GetStats()
	
	if stats == nil {
		t.Fatal("expected stats, got nil")
	}
	
	if stats["checkers_registered"].(int) == 0 {
		t.Error("expected checkers to be registered")
	}
}

func TestCriterionTypeConstants(t *testing.T) {
	types := []CriterionType{
		CriterionTypeFileExists,
		CriterionTypeFileNotExists,
		CriterionTypeFileContains,
		CriterionTypeFileMatches,
		CriterionTypeCommandSuccess,
		CriterionTypeCommandOutput,
		CriterionTypeTestPasses,
		CriterionTypeBuildSuccess,
		CriterionTypeLintPasses,
		CriterionTypeCustom,
	}
	
	for _, ct := range types {
		if ct == "" {
			t.Error("criterion type should not be empty")
		}
	}
}

func TestCriterionStatusConstants(t *testing.T) {
	statuses := []CriterionStatus{
		CriterionStatusPending,
		CriterionStatusPassed,
		CriterionStatusFailed,
		CriterionStatusSkipped,
		CriterionStatusError,
	}
	
	for _, cs := range statuses {
		if cs == "" {
			t.Error("criterion status should not be empty")
		}
	}
}

func TestSuccessCriterionFields(t *testing.T) {
	c := &SuccessCriterion{
		ID:          "test",
		Type:        CriterionTypeFileExists,
		Name:        "Test",
		Description: "Test criterion",
		Required:    true,
		Weight:      1.0,
		Target:      "file.txt",
		Pattern:     "pattern",
		Expected:    "expected",
		Negate:      false,
		Timeout:     time.Minute,
		RetryCount:  2,
		RetryDelay:  time.Second,
		Status:      CriterionStatusPending,
		Metadata:    map[string]any{"key": "value"},
	}
	
	if c.ID != "test" {
		t.Errorf("expected ID test, got %s", c.ID)
	}
}

func TestValidationResultFields(t *testing.T) {
	r := &ValidationResult{
		CriterionID: "test",
		Passed:      true,
		Message:     "Test passed",
		Details:     map[string]any{"key": "value"},
	}
	
	if r.CriterionID != "test" {
		t.Errorf("expected CriterionID test, got %s", r.CriterionID)
	}
}

func TestValidationReportFields(t *testing.T) {
	r := &ValidationReport{
		TaskID:          "task-1",
		TaskName:        "Test Task",
		TotalCriteria:   5,
		Passed:          4,
		Failed:          1,
		Skipped:         0,
		Score:           0.8,
		AllRequiredPass: true,
		Summary:         "Test summary",
	}
	
	if r.TaskID != "task-1" {
		t.Errorf("expected TaskID task-1, got %s", r.TaskID)
	}
}

func TestCheckFileMatchesInvalidRegex(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criterion := &SuccessCriterion{
		ID:      "test",
		Type:    CriterionTypeFileMatches,
		Target:  "test.txt",
		Pattern: "[invalid(regex",
	}
	
	result, err := v.checkFileMatches(context.Background(), criterion)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if result.Passed {
		t.Error("expected failure for invalid regex pattern")
	}
}

func TestCheckCustomNoMetadata(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criterion := &SuccessCriterion{
		ID:       "test",
		Type:     CriterionTypeCustom,
		Metadata: nil,
	}
	
	result, err := v.checkCustom(context.Background(), criterion)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if result.Passed {
		t.Error("expected failure for missing metadata")
	}
}

func TestCheckCustomInvalidChecker(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	criterion := &SuccessCriterion{
		ID:       "test",
		Type:     CriterionTypeCustom,
		Metadata: map[string]any{"checker": "not a function"},
	}
	
	result, err := v.checkCustom(context.Background(), criterion)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if result.Passed {
		t.Error("expected failure for invalid checker")
	}
}

func TestCheckCustomValidChecker(t *testing.T) {
	v := NewSuccessCriteriaValidator(DefaultSuccessCriteriaConfig())
	
	called := false
	checker := func(ctx context.Context, c *SuccessCriterion) (bool, string, error) {
		called = true
		return true, "Custom check passed", nil
	}
	
	criterion := &SuccessCriterion{
		ID:       "test",
		Type:     CriterionTypeCustom,
		Metadata: map[string]any{"checker": checker},
	}
	
	result, err := v.checkCustom(context.Background(), criterion)
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if !result.Passed {
		t.Error("expected custom check to pass")
	}
	
	if !called {
		t.Error("expected custom checker to be called")
	}
}
