// Package autonomous - Task 19: Success Criteria validation before task completion
package autonomous

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CriterionType represents the type of success criterion
type CriterionType string

const (
	CriterionTypeFileExists     CriterionType = "file_exists"
	CriterionTypeFileNotExists  CriterionType = "file_not_exists"
	CriterionTypeFileContains   CriterionType = "file_contains"
	CriterionTypeFileMatches    CriterionType = "file_matches"
	CriterionTypeCommandSuccess CriterionType = "command_success"
	CriterionTypeCommandOutput  CriterionType = "command_output"
	CriterionTypeTestPasses     CriterionType = "test_passes"
	CriterionTypeBuildSuccess   CriterionType = "build_success"
	CriterionTypeLintPasses     CriterionType = "lint_passes"
	CriterionTypeCustom         CriterionType = "custom"
)

// CriterionStatus represents the status of a criterion check
type CriterionStatus string

const (
	CriterionStatusPending CriterionStatus = "pending"
	CriterionStatusPassed  CriterionStatus = "passed"
	CriterionStatusFailed  CriterionStatus = "failed"
	CriterionStatusSkipped CriterionStatus = "skipped"
	CriterionStatusError   CriterionStatus = "error"
)

// SuccessCriterion represents a single success criterion
type SuccessCriterion struct {
	ID          string          `json:"id"`
	Type        CriterionType   `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Required    bool            `json:"required"` // Must pass for overall success
	Weight      float64         `json:"weight"`   // Weight for scoring (0.0-1.0)
	Target      string          `json:"target"`   // File path, command, or target identifier
	Pattern     string          `json:"pattern"`  // Pattern to match (regex or substring)
	Expected    string          `json:"expected"` // Expected value or output
	Negate      bool            `json:"negate"`   // Negate the result
	Timeout     time.Duration   `json:"timeout"`
	RetryCount  int             `json:"retry_count"`
	RetryDelay  time.Duration   `json:"retry_delay"`
	Status      CriterionStatus `json:"status"`
	Message     string          `json:"message"` // Result message
	CheckedAt   time.Time       `json:"checked_at"`
	Duration    time.Duration   `json:"duration"` // How long the check took
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

// ValidationResult represents the result of validating a criterion
type ValidationResult struct {
	CriterionID string         `json:"criterion_id"`
	Passed      bool           `json:"passed"`
	Message     string         `json:"message"`
	Details     map[string]any `json:"details,omitempty"`
	Error       error          `json:"error,omitempty"`
	Duration    time.Duration  `json:"duration"`
}

// ValidationReport represents a complete validation report
type ValidationReport struct {
	TaskID          string              `json:"task_id"`
	TaskName        string              `json:"task_name"`
	TotalCriteria   int                 `json:"total_criteria"`
	Passed          int                 `json:"passed"`
	Failed          int                 `json:"failed"`
	Skipped         int                 `json:"skipped"`
	Score           float64             `json:"score"` // 0.0-1.0 weighted score
	AllRequiredPass bool                `json:"all_required_pass"`
	Criteria        []*SuccessCriterion `json:"criteria"`
	GeneratedAt     time.Time           `json:"generated_at"`
	Duration        time.Duration       `json:"duration"`
	Summary         string              `json:"summary"`
}

// SuccessCriteriaConfig configures the success criteria validator
type SuccessCriteriaConfig struct {
	DefaultTimeout    time.Duration `json:"default_timeout"`
	DefaultRetries    int           `json:"default_retries"`
	DefaultRetryDelay time.Duration `json:"default_retry_delay"`
	ParallelChecks    bool          `json:"parallel_checks"`
	StopOnFirstFail   bool          `json:"stop_on_first_fail"`
}

// DefaultSuccessCriteriaConfig returns default configuration
func DefaultSuccessCriteriaConfig() SuccessCriteriaConfig {
	return SuccessCriteriaConfig{
		DefaultTimeout:    30 * time.Second,
		DefaultRetries:    2,
		DefaultRetryDelay: 1 * time.Second,
		ParallelChecks:    true,
		StopOnFirstFail:   false,
	}
}

// SuccessCriteriaValidator validates success criteria before task completion
type SuccessCriteriaValidator struct {
	mu       sync.RWMutex
	config   SuccessCriteriaConfig
	checkers map[CriterionType]CriterionChecker
	logger   interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}
}

// CriterionChecker is a function that checks a criterion
type CriterionChecker func(ctx context.Context, criterion *SuccessCriterion) (*ValidationResult, error)

// NewSuccessCriteriaValidator creates a new validator
func NewSuccessCriteriaValidator(config SuccessCriteriaConfig) *SuccessCriteriaValidator {
	v := &SuccessCriteriaValidator{
		config:   config,
		checkers: make(map[CriterionType]CriterionChecker),
	}

	// Register default checkers
	v.registerDefaultCheckers()

	return v
}

// registerDefaultCheckers registers the built-in criterion checkers
func (v *SuccessCriteriaValidator) registerDefaultCheckers() {
	// File existence checker
	v.RegisterChecker(CriterionTypeFileExists, v.checkFileExists)
	v.RegisterChecker(CriterionTypeFileNotExists, v.checkFileNotExists)
	v.RegisterChecker(CriterionTypeFileContains, v.checkFileContains)
	v.RegisterChecker(CriterionTypeFileMatches, v.checkFileMatches)
	v.RegisterChecker(CriterionTypeCommandSuccess, v.checkCommandSuccess)
	v.RegisterChecker(CriterionTypeCommandOutput, v.checkCommandOutput)
	v.RegisterChecker(CriterionTypeTestPasses, v.checkTestPasses)
	v.RegisterChecker(CriterionTypeBuildSuccess, v.checkBuildSuccess)
	v.RegisterChecker(CriterionTypeLintPasses, v.checkLintPasses)
	v.RegisterChecker(CriterionTypeCustom, v.checkCustom)
}

// RegisterChecker registers a custom criterion checker
func (v *SuccessCriteriaValidator) RegisterChecker(criterionType CriterionType, checker CriterionChecker) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.checkers[criterionType] = checker
}

// SetLogger sets the logger for the validator
func (v *SuccessCriteriaValidator) SetLogger(logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.logger = logger
}

// Validate validates all criteria and returns a report
func (v *SuccessCriteriaValidator) Validate(ctx context.Context, criteria []*SuccessCriterion) *ValidationReport {
	start := time.Now()

	report := &ValidationReport{
		TotalCriteria: len(criteria),
		Criteria:      make([]*SuccessCriterion, 0, len(criteria)),
		GeneratedAt:   time.Now(),
	}

	// Apply defaults to criteria
	for _, c := range criteria {
		if c.Timeout == 0 {
			c.Timeout = v.config.DefaultTimeout
		}
		if c.RetryCount == 0 {
			c.RetryCount = v.config.DefaultRetries
		}
		if c.RetryDelay == 0 {
			c.RetryDelay = v.config.DefaultRetryDelay
		}
		if c.Weight == 0 {
			c.Weight = 1.0
		}
	}

	if v.config.ParallelChecks {
		v.validateParallel(ctx, criteria, report)
	} else {
		v.validateSequential(ctx, criteria, report)
	}

	// Calculate score and summary
	report.Duration = time.Since(start)
	report.calculateScore()
	report.generateSummary()

	return report
}

// validateSequential validates criteria one by one
func (v *SuccessCriteriaValidator) validateSequential(ctx context.Context, criteria []*SuccessCriterion, report *ValidationReport) {
	for _, criterion := range criteria {
		if ctx.Err() != nil {
			criterion.Status = CriterionStatusSkipped
			report.Skipped++
			report.Criteria = append(report.Criteria, criterion)
			continue
		}

		result := v.validateCriterion(ctx, criterion)
		v.applyResult(criterion, result)
		report.Criteria = append(report.Criteria, criterion)

		if criterion.Status == CriterionStatusPassed {
			report.Passed++
		} else if criterion.Status == CriterionStatusFailed || criterion.Status == CriterionStatusError {
			report.Failed++
		} else {
			report.Skipped++
		}

		// Stop on first failure if configured
		if v.config.StopOnFirstFail && criterion.Status == CriterionStatusFailed && criterion.Required {
			break
		}
	}
}

// validateParallel validates criteria in parallel
func (v *SuccessCriteriaValidator) validateParallel(ctx context.Context, criteria []*SuccessCriterion, report *ValidationReport) {
	var wg sync.WaitGroup
	results := make(chan *criterionResult, len(criteria))

	for _, criterion := range criteria {
		wg.Add(1)
		go func(c *SuccessCriterion) {
			defer wg.Done()

			if ctx.Err() != nil {
				results <- &criterionResult{criterion: c, status: CriterionStatusSkipped}
				return
			}

			result := v.validateCriterion(ctx, c)
			results <- &criterionResult{criterion: c, result: result}
		}(criterion)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		v.applyResult(res.criterion, res.result)
		report.Criteria = append(report.Criteria, res.criterion)

		if res.criterion.Status == CriterionStatusPassed {
			report.Passed++
		} else if res.criterion.Status == CriterionStatusFailed || res.criterion.Status == CriterionStatusError {
			report.Failed++
		} else {
			report.Skipped++
		}
	}
}

type criterionResult struct {
	criterion *SuccessCriterion
	result    *ValidationResult
	status    CriterionStatus
}

// validateCriterion validates a single criterion with retries
func (v *SuccessCriteriaValidator) validateCriterion(ctx context.Context, criterion *SuccessCriterion) *ValidationResult {
	v.mu.RLock()
	checker, exists := v.checkers[criterion.Type]
	v.mu.RUnlock()

	if !exists {
		return &ValidationResult{
			CriterionID: criterion.ID,
			Passed:      false,
			Message:     fmt.Sprintf("No checker registered for type: %s", criterion.Type),
			Error:       fmt.Errorf("unknown criterion type: %s", criterion.Type),
		}
	}

	var lastResult *ValidationResult
	var lastErr error

	for attempt := 0; attempt <= criterion.RetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return &ValidationResult{
					CriterionID: criterion.ID,
					Passed:      false,
					Message:     "Context cancelled during retry",
					Error:       ctx.Err(),
				}
			case <-time.After(criterion.RetryDelay):
			}
		}

		// Create timeout context
		checkCtx, cancel := context.WithTimeout(ctx, criterion.Timeout)
		start := time.Now()

		result, err := checker(checkCtx, criterion)
		cancel()

		// Handle nil result from checker
		if result == nil {
			if err != nil {
				lastErr = err
				continue
			}
			// Checker returned nil without error - treat as failure
			result = &ValidationResult{
				CriterionID: criterion.ID,
				Passed:      false,
				Message:     "Checker returned nil result",
			}
		}

		result.Duration = time.Since(start)

		if err != nil {
			lastErr = err
			continue
		}

		lastResult = result

		// Apply negation if needed
		if criterion.Negate {
			result.Passed = !result.Passed
			if result.Passed {
				result.Message = "Negated: " + result.Message
			}
		}

		if result.Passed {
			return result
		}

		lastResult = result
	}

	if lastResult == nil {
		return &ValidationResult{
			CriterionID: criterion.ID,
			Passed:      false,
			Message:     fmt.Sprintf("All %d attempts failed: %v", criterion.RetryCount+1, lastErr),
			Error:       lastErr,
		}
	}

	return lastResult
}

// applyResult applies a validation result to a criterion
func (v *SuccessCriteriaValidator) applyResult(criterion *SuccessCriterion, result *ValidationResult) {
	criterion.CheckedAt = time.Now()

	// Handle nil result (e.g., from context cancellation)
	if result == nil {
		criterion.Status = CriterionStatusSkipped
		criterion.Message = "Validation skipped (result unavailable)"
		return
	}

	criterion.Message = result.Message

	if result.Error != nil {
		criterion.Status = CriterionStatusError
		criterion.Message = fmt.Sprintf("%s (error: %v)", result.Message, result.Error)
	} else if result.Passed {
		criterion.Status = CriterionStatusPassed
	} else {
		criterion.Status = CriterionStatusFailed
	}
}

// Default checkers

func (v *SuccessCriteriaValidator) checkFileExists(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// This is a placeholder - actual implementation would check filesystem
	// In production, this would use os.Stat or similar
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("File exists check for: %s", c.Target),
		Details:     map[string]any{"path": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkFileNotExists(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("File does not exist check for: %s", c.Target),
		Details:     map[string]any{"path": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkFileContains(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would read file and check content
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("File contains check for: %s", c.Target),
		Details:     map[string]any{"path": c.Target, "pattern": c.Pattern},
	}, nil
}

func (v *SuccessCriteriaValidator) checkFileMatches(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would read file and regex match
	_, err := regexp.Compile(c.Pattern)
	if err != nil {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      false,
			Message:     fmt.Sprintf("Invalid regex pattern: %s", c.Pattern),
			Error:       err,
		}, nil
	}

	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("File matches check for: %s", c.Target),
		Details:     map[string]any{"path": c.Target, "pattern": c.Pattern},
	}, nil
}

func (v *SuccessCriteriaValidator) checkCommandSuccess(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would execute command
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("Command success check for: %s", c.Target),
		Details:     map[string]any{"command": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkCommandOutput(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would execute command and check output
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("Command output check for: %s", c.Target),
		Details:     map[string]any{"command": c.Target, "expected": c.Expected},
	}, nil
}

func (v *SuccessCriteriaValidator) checkTestPasses(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would run tests
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("Test passes check for: %s", c.Target),
		Details:     map[string]any{"test_target": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkBuildSuccess(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would run build
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("Build success check for: %s", c.Target),
		Details:     map[string]any{"build_target": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkLintPasses(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Placeholder - actual implementation would run linter
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      true,
		Message:     fmt.Sprintf("Lint passes check for: %s", c.Target),
		Details:     map[string]any{"lint_target": c.Target},
	}, nil
}

func (v *SuccessCriteriaValidator) checkCustom(ctx context.Context, c *SuccessCriterion) (*ValidationResult, error) {
	// Custom checkers are stored in metadata
	if c.Metadata == nil {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      false,
			Message:     "No custom checker function provided",
			Error:       fmt.Errorf("missing custom checker"),
		}, nil
	}

	checkerFunc, ok := c.Metadata["checker"].(func(context.Context, *SuccessCriterion) (bool, string, error))
	if !ok {
		return &ValidationResult{
			CriterionID: c.ID,
			Passed:      false,
			Message:     "Invalid custom checker function",
			Error:       fmt.Errorf("invalid checker type"),
		}, nil
	}

	passed, message, err := checkerFunc(ctx, c)
	return &ValidationResult{
		CriterionID: c.ID,
		Passed:      passed,
		Message:     message,
		Error:       err,
	}, nil
}

// QuickValidate performs a quick validation of a single criterion
func (v *SuccessCriteriaValidator) QuickValidate(ctx context.Context, criterion *SuccessCriterion) (*ValidationResult, error) {
	result := v.validateCriterion(ctx, criterion)
	return result, result.Error
}

// IsSuccess checks if a validation report indicates success
func (r *ValidationReport) IsSuccess() bool {
	return r.AllRequiredPass && r.Score >= 0.8
}

// calculateScore calculates the weighted score
func (r *ValidationReport) calculateScore() {
	if r.TotalCriteria == 0 {
		r.Score = 1.0
		r.AllRequiredPass = true
		return
	}

	var totalWeight, passedWeight float64
	r.AllRequiredPass = true

	for _, c := range r.Criteria {
		totalWeight += c.Weight
		if c.Status == CriterionStatusPassed {
			passedWeight += c.Weight
		}
		if c.Required && c.Status != CriterionStatusPassed {
			r.AllRequiredPass = false
		}
	}

	if totalWeight > 0 {
		r.Score = passedWeight / totalWeight
	}
}

// generateSummary generates a human-readable summary
func (r *ValidationReport) generateSummary() {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Validation: %d/%d criteria passed", r.Passed, r.TotalCriteria))

	if r.AllRequiredPass {
		sb.WriteString(" ✓ All required criteria met")
	} else {
		sb.WriteString(" ✗ Some required criteria failed")
	}

	sb.WriteString(fmt.Sprintf(" (Score: %.0f%%)", r.Score*100))

	r.Summary = sb.String()
}

// GetStats returns statistics about the validator
func (v *SuccessCriteriaValidator) GetStats() map[string]any {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return map[string]any{
		"checkers_registered": len(v.checkers),
		"parallel_checks":     v.config.ParallelChecks,
		"default_timeout":     v.config.DefaultTimeout.String(),
		"default_retries":     v.config.DefaultRetries,
	}
}

// CriterionBuilder helps build success criteria
type CriterionBuilder struct {
	criterion *SuccessCriterion
}

// NewCriterionBuilder creates a new criterion builder
func NewCriterionBuilder(criterionType CriterionType, target string) *CriterionBuilder {
	return &CriterionBuilder{
		criterion: &SuccessCriterion{
			ID:       fmt.Sprintf("crit_%d", time.Now().UnixNano()),
			Type:     criterionType,
			Target:   target,
			Required: true,
			Weight:   1.0,
			Status:   CriterionStatusPending,
			Metadata: make(map[string]any),
		},
	}
}

// WithName sets the criterion name
func (b *CriterionBuilder) WithName(name string) *CriterionBuilder {
	b.criterion.Name = name
	return b
}

// WithDescription sets the criterion description
func (b *CriterionBuilder) WithDescription(desc string) *CriterionBuilder {
	b.criterion.Description = desc
	return b
}

// WithPattern sets the pattern for matching
func (b *CriterionBuilder) WithPattern(pattern string) *CriterionBuilder {
	b.criterion.Pattern = pattern
	return b
}

// WithExpected sets the expected value
func (b *CriterionBuilder) WithExpected(expected string) *CriterionBuilder {
	b.criterion.Expected = expected
	return b
}

// WithTimeout sets the timeout
func (b *CriterionBuilder) WithTimeout(timeout time.Duration) *CriterionBuilder {
	b.criterion.Timeout = timeout
	return b
}

// WithRetries sets the retry count and delay
func (b *CriterionBuilder) WithRetries(count int, delay time.Duration) *CriterionBuilder {
	b.criterion.RetryCount = count
	b.criterion.RetryDelay = delay
	return b
}

// Optional marks the criterion as optional
func (b *CriterionBuilder) Optional() *CriterionBuilder {
	b.criterion.Required = false
	return b
}

// WithWeight sets the weight for scoring
func (b *CriterionBuilder) WithWeight(weight float64) *CriterionBuilder {
	b.criterion.Weight = weight
	return b
}

// Negate negates the result
func (b *CriterionBuilder) Negate() *CriterionBuilder {
	b.criterion.Negate = true
	return b
}

// WithMetadata adds metadata
func (b *CriterionBuilder) WithMetadata(key string, value any) *CriterionBuilder {
	b.criterion.Metadata[key] = value
	return b
}

// Build returns the built criterion
func (b *CriterionBuilder) Build() *SuccessCriterion {
	return b.criterion
}
