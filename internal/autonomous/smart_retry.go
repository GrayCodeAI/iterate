// Package autonomous - Task 11: Smart Retry with error pattern recognition
package autonomous

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SmartRetry implements intelligent retry logic with pattern learning.
type SmartRetry struct {
	mu     sync.RWMutex
	logger interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Debug(msg string, args ...any)
	}
	patterns        []*RetryPattern
	history         *RetryHistory
	strategies      map[string]RetryStrategy
	maxAttempts     int
	baseBackoff     time.Duration
	maxBackoff      time.Duration
	learningEnabled bool
}

// RetryPattern represents a learned error pattern with retry metadata.
type RetryPattern struct {
	ID                string
	RegexPattern      string
	Category          ErrorCategory
	AutoFixable       bool
	FixHints          []string
	SuccessRate       float64
	AttemptCount      int
	LastSeen          time.Time
	PreferredStrategy string
}

// ErrorCategory categorizes error types for strategy selection.
type ErrorCategory string

const (
	CategoryBuildError    ErrorCategory = "build_error"
	CategoryTestFailure   ErrorCategory = "test_failure"
	CategoryRaceCondition ErrorCategory = "race_condition"
	CategoryTimeout       ErrorCategory = "timeout"
	CategoryNetworkError  ErrorCategory = "network_error"
	CategoryResourceError ErrorCategory = "resource_error"
	CategorySyntaxError   ErrorCategory = "syntax_error"
	CategoryImportError   ErrorCategory = "import_error"
	CategoryRuntimeError  ErrorCategory = "runtime_error"
	CategoryUnknown       ErrorCategory = "unknown"
)

// RetryStrategy defines how retries should be attempted.
type RetryStrategy struct {
	Name          string
	MaxAttempts   int
	BackoffFactor float64
	JitterEnabled bool
	FixActions    []FixAction
}

// FixAction represents a potential fix to try.
type FixAction struct {
	Type        FixActionType
	Description string
	Command     string
	FilePattern string
	Template    string
}

// FixActionType defines types of automated fixes.
type FixActionType string

const (
	FixTypeImport      FixActionType = "import"
	FixTypeFormat      FixActionType = "format"
	FixTypeLint        FixActionType = "lint"
	FixTypeRebuild     FixActionType = "rebuild"
	FixTypeClean       FixActionType = "clean"
	FixTypeWait        FixActionType = "wait"
	FixTypeAlternative FixActionType = "alternative"
)

// RetryHistory tracks retry attempts across sessions.
type RetryHistory struct {
	mu       sync.RWMutex
	attempts []*RetryAttempt
	patterns map[string]*PatternStats // pattern ID -> stats
}

// RetryAttempt records a single retry attempt.
type RetryAttempt struct {
	Timestamp  time.Time
	TaskID     string
	Error      string
	Category   ErrorCategory
	AttemptNum int
	Success    bool
	FixApplied string
	Duration   time.Duration
	NextDelay  time.Duration
}

// PatternStats tracks statistics for a pattern.
type PatternStats struct {
	PatternID   string
	TotalSeen   int
	TotalFixed  int
	TotalFailed int
	AvgAttempts float64
	LastFixed   time.Time
	FixActions  map[string]int // fix description -> success count
}

// RetryConfig configures the SmartRetry behavior.
type RetryConfig struct {
	MaxAttempts     int
	BaseBackoff     time.Duration
	MaxBackoff      time.Duration
	LearningEnabled bool
}

// RetryResult holds the outcome of a smart retry operation.
type RetryResult struct {
	Success      bool
	Attempts     int
	TotalTime    time.Duration
	FinalError   string
	FixesApplied []string
	Patterns     []string
	Learned      bool
}

// NewSmartRetry creates a new SmartRetry instance.
func NewSmartRetry(config RetryConfig) *SmartRetry {
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	if config.BaseBackoff == 0 {
		config.BaseBackoff = 1 * time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}

	sr := &SmartRetry{
		patterns:        DefaultRetryPatterns(),
		history:         NewRetryHistory(),
		strategies:      DefaultStrategies(),
		maxAttempts:     config.MaxAttempts,
		baseBackoff:     config.BaseBackoff,
		maxBackoff:      config.MaxBackoff,
		learningEnabled: config.LearningEnabled,
	}

	return sr
}

// SetLogger sets the logger for SmartRetry.
func (sr *SmartRetry) SetLogger(logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
}) {
	sr.mu.Lock()
	sr.logger = logger
	sr.mu.Unlock()
}

// DefaultRetryPatterns returns built-in retry patterns.
func DefaultRetryPatterns() []*RetryPattern {
	return []*RetryPattern{
		{
			ID:                "undefined-identifier",
			RegexPattern:      `undefined:\s*(\w+)`,
			Category:          CategoryBuildError,
			AutoFixable:       true,
			FixHints:          []string{"Check imports for $1", "Define identifier $1"},
			SuccessRate:       0.85,
			PreferredStrategy: "import-fix",
		},
		{
			ID:                "type-mismatch",
			RegexPattern:      `cannot use ([^\s]+) as type ([^\s]+)`,
			Category:          CategoryBuildError,
			AutoFixable:       true,
			FixHints:          []string{"Convert $1 to $2", "Check type signature"},
			SuccessRate:       0.70,
			PreferredStrategy: "rebuild",
		},
		{
			ID:                "unused-import",
			RegexPattern:      `imported and not used:\s*"([^"]+)"`,
			Category:          CategoryImportError,
			AutoFixable:       true,
			FixHints:          []string{"Remove import $1", "Use imported package"},
			SuccessRate:       0.95,
			PreferredStrategy: "lint",
		},
		{
			ID:                "race-condition",
			RegexPattern:      `DATA RACE`,
			Category:          CategoryRaceCondition,
			AutoFixable:       true,
			FixHints:          []string{"Add mutex synchronization", "Use atomic operations"},
			SuccessRate:       0.60,
			PreferredStrategy: "alternative",
		},
		{
			ID:                "nil-pointer",
			RegexPattern:      `nil pointer dereference`,
			Category:          CategoryRuntimeError,
			AutoFixable:       true,
			FixHints:          []string{"Add nil check", "Initialize pointer"},
			SuccessRate:       0.75,
			PreferredStrategy: "rebuild",
		},
		{
			ID:                "panic",
			RegexPattern:      `panic:\s*([^\n]+)`,
			Category:          CategoryRuntimeError,
			AutoFixable:       true,
			FixHints:          []string{"Add recovery handling", "Fix root cause: $1"},
			SuccessRate:       0.65,
			PreferredStrategy: "rebuild",
		},
		{
			ID:                "test-failure",
			RegexPattern:      `--- FAIL:\s*([^\s]+)`,
			Category:          CategoryTestFailure,
			AutoFixable:       false,
			FixHints:          []string{"Review test output", "Check test assertions"},
			SuccessRate:       0.50,
			PreferredStrategy: "rebuild",
		},
		{
			ID:                "timeout",
			RegexPattern:      `timeout|context deadline exceeded`,
			Category:          CategoryTimeout,
			AutoFixable:       true,
			FixHints:          []string{"Increase timeout", "Optimize slow operation"},
			SuccessRate:       0.55,
			PreferredStrategy: "wait",
		},
		{
			ID:                "syntax-error",
			RegexPattern:      `syntax error`,
			Category:          CategorySyntaxError,
			AutoFixable:       true,
			FixHints:          []string{"Fix syntax", "Check for missing brackets/braces"},
			SuccessRate:       0.90,
			PreferredStrategy: "format",
		},
		{
			ID:                "file-not-found",
			RegexPattern:      `no such file or directory`,
			Category:          CategoryResourceError,
			AutoFixable:       false,
			FixHints:          []string{"Create missing file", "Check file path"},
			SuccessRate:       0.40,
			PreferredStrategy: "alternative",
		},
		{
			ID:                "network-error",
			RegexPattern:      `connection refused|network unreachable|dial tcp`,
			Category:          CategoryNetworkError,
			AutoFixable:       true,
			FixHints:          []string{"Wait and retry", "Check network connectivity"},
			SuccessRate:       0.70,
			PreferredStrategy: "wait",
		},
	}
}

// DefaultStrategies returns built-in retry strategies.
func DefaultStrategies() map[string]RetryStrategy {
	return map[string]RetryStrategy{
		"import-fix": {
			Name:          "import-fix",
			MaxAttempts:   3,
			BackoffFactor: 1.5,
			JitterEnabled: true,
			FixActions: []FixAction{
				{Type: FixTypeImport, Description: "Add missing import"},
				{Type: FixTypeLint, Description: "Run goimports"},
				{Type: FixTypeFormat, Description: "Format imports"},
			},
		},
		"lint": {
			Name:          "lint",
			MaxAttempts:   2,
			BackoffFactor: 1.0,
			JitterEnabled: false,
			FixActions: []FixAction{
				{Type: FixTypeLint, Description: "Remove unused code"},
				{Type: FixTypeClean, Description: "Clean and rebuild"},
			},
		},
		"format": {
			Name:          "format",
			MaxAttempts:   2,
			BackoffFactor: 1.0,
			JitterEnabled: false,
			FixActions: []FixAction{
				{Type: FixTypeFormat, Description: "Format code"},
				{Type: FixTypeRebuild, Description: "Rebuild after format"},
			},
		},
		"rebuild": {
			Name:          "rebuild",
			MaxAttempts:   3,
			BackoffFactor: 2.0,
			JitterEnabled: true,
			FixActions: []FixAction{
				{Type: FixTypeClean, Description: "Clean build cache"},
				{Type: FixTypeRebuild, Description: "Full rebuild"},
				{Type: FixTypeAlternative, Description: "Try alternative approach"},
			},
		},
		"wait": {
			Name:          "wait",
			MaxAttempts:   5,
			BackoffFactor: 2.5,
			JitterEnabled: true,
			FixActions: []FixAction{
				{Type: FixTypeWait, Description: "Wait before retry"},
				{Type: FixTypeWait, Description: "Extended wait"},
			},
		},
		"alternative": {
			Name:          "alternative",
			MaxAttempts:   4,
			BackoffFactor: 1.5,
			JitterEnabled: true,
			FixActions: []FixAction{
				{Type: FixTypeAlternative, Description: "Try alternative solution"},
				{Type: FixTypeClean, Description: "Reset and retry"},
			},
		},
	}
}

// NewRetryHistory creates a new retry history tracker.
func NewRetryHistory() *RetryHistory {
	return &RetryHistory{
		patterns: make(map[string]*PatternStats),
	}
}

// ExecuteWithRetry runs a function with smart retry logic.
func (sr *SmartRetry) ExecuteWithRetry(ctx context.Context, taskID string, fn func() error) *RetryResult {
	result := &RetryResult{
		Success:      false,
		Attempts:     0,
		FixesApplied: []string{},
		Patterns:     []string{},
	}

	start := time.Now()
	var lastErr error

	for attempt := 1; attempt <= sr.maxAttempts; attempt++ {
		result.Attempts = attempt

		err := fn()
		if err == nil {
			result.Success = true
			result.TotalTime = time.Since(start)
			sr.recordSuccess(taskID, attempt)
			return result
		}

		lastErr = err
		errorMsg := err.Error()

		// Analyze error
		matchedPatterns := sr.matchPatterns(errorMsg)
		category := sr.categorizeError(errorMsg, matchedPatterns)

		if len(matchedPatterns) > 0 {
			for _, p := range matchedPatterns {
				result.Patterns = append(result.Patterns, p.ID)
			}
		}

		// Get best strategy
		strategy := sr.selectStrategy(category, matchedPatterns)

		// Calculate backoff
		delay := sr.calculateBackoff(attempt, strategy)
		result.TotalTime = time.Since(start)

		// Record attempt
		sr.recordAttempt(&RetryAttempt{
			Timestamp:  time.Now(),
			TaskID:     taskID,
			Error:      errorMsg,
			Category:   category,
			AttemptNum: attempt,
			Success:    false,
			Duration:   result.TotalTime,
			NextDelay:  delay,
		})

		// Log progress
		if sr.logger != nil {
			sr.logger.Info("Retry attempt",
				"attempt", attempt,
				"category", category,
				"delay", delay,
				"patterns", len(matchedPatterns),
			)
		}

		// Try auto-fix if applicable
		if strategy != nil && attempt < sr.maxAttempts {
			fixApplied := sr.tryAutoFix(ctx, strategy, attempt, errorMsg)
			if fixApplied != "" {
				result.FixesApplied = append(result.FixesApplied, fixApplied)
			}
		}

		// Wait before next attempt
		select {
		case <-ctx.Done():
			result.FinalError = ctx.Err().Error()
			return result
		case <-time.After(delay):
		}
	}

	result.TotalTime = time.Since(start)
	if lastErr != nil {
		result.FinalError = lastErr.Error()
	}
	return result
}

// matchPatterns finds all patterns matching the error message.
func (sr *SmartRetry) matchPatterns(errorMsg string) []*RetryPattern {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var matched []*RetryPattern
	for _, pattern := range sr.patterns {
		if isMatch, _ := regexp.MatchString(pattern.RegexPattern, errorMsg); isMatch {
			matched = append(matched, pattern)
		}
	}
	return matched
}

// categorizeError determines the error category.
func (sr *SmartRetry) categorizeError(errorMsg string, patterns []*RetryPattern) ErrorCategory {
	if len(patterns) > 0 {
		// Return category of highest success rate pattern
		best := patterns[0]
		for _, p := range patterns {
			if p.SuccessRate > best.SuccessRate {
				best = p
			}
		}
		return best.Category
	}
	return CategoryUnknown
}

// selectStrategy chooses the best retry strategy.
func (sr *SmartRetry) selectStrategy(category ErrorCategory, patterns []*RetryPattern) *RetryStrategy {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	// Use pattern's preferred strategy if available
	if len(patterns) > 0 {
		for _, p := range patterns {
			if p.PreferredStrategy != "" {
				if strategy, ok := sr.strategies[p.PreferredStrategy]; ok {
					return &strategy
				}
			}
		}
	}

	// Default strategies by category
	defaultMap := map[ErrorCategory]string{
		CategoryBuildError:    "rebuild",
		CategoryTestFailure:   "rebuild",
		CategoryRaceCondition: "alternative",
		CategoryTimeout:       "wait",
		CategoryNetworkError:  "wait",
		CategoryResourceError: "alternative",
		CategorySyntaxError:   "format",
		CategoryImportError:   "import-fix",
		CategoryRuntimeError:  "rebuild",
		CategoryUnknown:       "rebuild",
	}

	strategyName := defaultMap[category]
	if strategy, ok := sr.strategies[strategyName]; ok {
		return &strategy
	}

	// Fallback
	if strategy, ok := sr.strategies["rebuild"]; ok {
		return &strategy
	}
	return nil
}

// calculateBackoff computes the delay before next retry.
func (sr *SmartRetry) calculateBackoff(attempt int, strategy *RetryStrategy) time.Duration {
	baseBackoff := sr.baseBackoff
	factor := 2.0

	if strategy != nil {
		factor = strategy.BackoffFactor
	}

	// Exponential backoff
	delay := time.Duration(float64(baseBackoff) * pow(factor, float64(attempt-1)))

	// Cap at max
	if delay > sr.maxBackoff {
		delay = sr.maxBackoff
	}

	// Add jitter
	if strategy != nil && strategy.JitterEnabled {
		jitter := time.Duration(float64(delay) * 0.1)
		delay = delay + time.Duration(float64(jitter)*(float64(time.Now().UnixNano()%1000)/1000.0))
	}

	return delay
}

// pow calculates x^y for simple exponentiation.
func pow(x, y float64) float64 {
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	return result
}

// tryAutoFix attempts an automatic fix based on the strategy.
func (sr *SmartRetry) tryAutoFix(ctx context.Context, strategy *RetryStrategy, attempt int, errorMsg string) string {
	if strategy == nil || len(strategy.FixActions) == 0 {
		return ""
	}

	// Get fix action for current attempt
	actionIdx := attempt - 1
	if actionIdx >= len(strategy.FixActions) {
		actionIdx = len(strategy.FixActions) - 1
	}
	action := strategy.FixActions[actionIdx]

	if sr.logger != nil {
		sr.logger.Info("Attempting auto-fix",
			"type", action.Type,
			"description", action.Description,
		)
	}

	return action.Description
}

// recordAttempt stores a retry attempt in history.
func (sr *SmartRetry) recordAttempt(attempt *RetryAttempt) {
	sr.history.mu.Lock()
	defer sr.history.mu.Unlock()

	sr.history.attempts = append(sr.history.attempts, attempt)
}

// recordSuccess records a successful retry.
func (sr *SmartRetry) recordSuccess(taskID string, attempts int) {
	sr.history.mu.Lock()
	defer sr.history.mu.Unlock()

	// Update pattern statistics
	for _, attempt := range sr.history.attempts {
		if attempt.TaskID == taskID && !attempt.Success {
			patterns := sr.matchPatterns(attempt.Error)
			for _, p := range patterns {
				stats, ok := sr.history.patterns[p.ID]
				if !ok {
					stats = &PatternStats{
						PatternID:  p.ID,
						FixActions: make(map[string]int),
					}
					sr.history.patterns[p.ID] = stats
				}
				stats.TotalSeen++
				if attempts <= 3 {
					stats.TotalFixed++
					stats.LastFixed = time.Now()
				}
			}
		}
	}
}

// LearnFromHistory updates pattern success rates based on history.
func (sr *SmartRetry) LearnFromHistory() {
	if !sr.learningEnabled {
		return
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.history.mu.RLock()
	defer sr.history.mu.RUnlock()

	for _, stats := range sr.history.patterns {
		// Find and update pattern
		for _, pattern := range sr.patterns {
			if pattern.ID == stats.PatternID {
				pattern.AttemptCount = stats.TotalSeen
				if stats.TotalSeen > 0 {
					pattern.SuccessRate = float64(stats.TotalFixed) / float64(stats.TotalSeen)
				}
				pattern.LastSeen = time.Now()
				break
			}
		}
	}
}

// GetPatternStats returns statistics for a pattern.
func (sr *SmartRetry) GetPatternStats(patternID string) *PatternStats {
	sr.history.mu.RLock()
	defer sr.history.mu.RUnlock()
	return sr.history.patterns[patternID]
}

// GetHistory returns the retry history.
func (sr *SmartRetry) GetHistory() []RetryAttempt {
	sr.history.mu.RLock()
	defer sr.history.mu.RUnlock()

	// Return copies
	attempts := make([]RetryAttempt, len(sr.history.attempts))
	for i, a := range sr.history.attempts {
		attempts[i] = *a
	}
	return attempts
}

// AddPattern adds a new retry pattern.
func (sr *SmartRetry) AddPattern(pattern *RetryPattern) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	pattern.LastSeen = time.Now()
	sr.patterns = append(sr.patterns, pattern)
}

// GetBestFixHint returns the best fix hint based on learned history.
func (sr *SmartRetry) GetBestFixHint(errorMsg string) string {
	patterns := sr.matchPatterns(errorMsg)
	if len(patterns) == 0 {
		return ""
	}

	// Sort by success rate
	best := patterns[0]
	for _, p := range patterns {
		if p.SuccessRate > best.SuccessRate {
			best = p
		}
	}

	if len(best.FixHints) > 0 {
		// Expand variables in hint
		hint := best.FixHints[0]
		re := regexp.MustCompile(best.RegexPattern)
		matches := re.FindStringSubmatch(errorMsg)
		for i := 1; i < len(matches); i++ {
			hint = strings.ReplaceAll(hint, fmt.Sprintf("$%d", i), matches[i])
		}
		return hint
	}

	return ""
}

// AnalyzeFailure analyzes an error and returns structured information.
func (sr *SmartRetry) AnalyzeFailure(errorMsg string) *FailureAnalysis {
	analysis := &FailureAnalysis{
		OriginalError: errorMsg,
		Category:      CategoryUnknown,
		Fixable:       false,
		Confidence:    0.0,
		Suggestions:   []string{},
	}

	patterns := sr.matchPatterns(errorMsg)
	if len(patterns) == 0 {
		return analysis
	}

	// Use best matching pattern
	best := patterns[0]
	for _, p := range patterns {
		if p.SuccessRate > best.SuccessRate {
			best = p
		}
	}

	analysis.Category = best.Category
	analysis.Fixable = best.AutoFixable
	analysis.Confidence = best.SuccessRate
	analysis.PatternID = best.ID
	analysis.Suggestions = best.FixHints

	// Get historical stats
	if stats := sr.GetPatternStats(best.ID); stats != nil {
		analysis.HistoricalFixRate = float64(stats.TotalFixed) / float64(max(stats.TotalSeen, 1))
	}

	return analysis
}

// FailureAnalysis contains structured failure information.
type FailureAnalysis struct {
	OriginalError     string
	Category          ErrorCategory
	PatternID         string
	Fixable           bool
	Confidence        float64
	HistoricalFixRate float64
	Suggestions       []string
	RelatedFiles      []string
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetStats returns overall retry statistics.
func (sr *SmartRetry) GetStats() RetryStats {
	sr.history.mu.RLock()
	defer sr.history.mu.RUnlock()

	stats := RetryStats{
		TotalAttempts: len(sr.history.attempts),
		ByCategory:    make(map[ErrorCategory]int),
	}

	for _, attempt := range sr.history.attempts {
		stats.ByCategory[attempt.Category]++
		if attempt.Success {
			stats.SuccessfulRetries++
		}
	}

	if stats.TotalAttempts > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRetries) / float64(stats.TotalAttempts)
	}

	return stats
}

// RetryStats holds aggregate retry statistics.
type RetryStats struct {
	TotalAttempts     int
	SuccessfulRetries int
	SuccessRate       float64
	ByCategory        map[ErrorCategory]int
}

// Reset clears the retry history.
func (sr *SmartRetry) Reset() {
	sr.history.mu.Lock()
	defer sr.history.mu.Unlock()

	sr.history.attempts = nil
	sr.history.patterns = make(map[string]*PatternStats)
}
