// Package retry provides intelligent retry mechanisms for agent operations
package retry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	BackoffMult float64
}

// DefaultConfig returns sensible defaults
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		BackoffMult: 2.0,
	}
}

// RetryableError indicates an error that can be retried
type RetryableError struct {
	Err        error
	Context    string
	Suggestion string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("%s: %v (suggestion: %s)", e.Context, e.Err, e.Suggestion)
}

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network/transient errors
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary",
		"rate limit",
		"too many requests",
		"eof",
		"broken pipe",
		"network",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// WithRetry executes a function with automatic retry logic
func WithRetry(ctx context.Context, config RetryConfig, operation func() error, logger *slog.Logger) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := calculateDelay(config, attempt)
			logger.Info("retrying operation",
				"attempt", attempt,
				"max_retries", config.MaxRetries,
				"delay", delay,
				"error", lastErr)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				logger.Info("operation succeeded after retry", "attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Don't retry if it's not a retryable error
		if !IsRetryable(err) {
			logger.Error("non-retryable error", "error", err)
			return err
		}

		// Last attempt failed
		if attempt == config.MaxRetries {
			logger.Error("all retry attempts exhausted",
				"attempts", config.MaxRetries+1,
				"error", err)
			return fmt.Errorf("failed after %d attempts: %w", config.MaxRetries+1, err)
		}
	}

	return lastErr
}

// calculateDelay computes the delay for a given attempt using exponential backoff
func calculateDelay(config RetryConfig, attempt int) time.Duration {
	delay := config.BaseDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * config.BackoffMult)
	}

	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// AutoFixAttempt tries to automatically fix common errors
type AutoFixAttempt struct {
	ErrorPattern string
	FixFunc      func(error) error
}

// CommonAutoFixes provides automatic fixes for known issues
var CommonAutoFixes = []AutoFixAttempt{
	{
		ErrorPattern: "no such file or directory",
		FixFunc: func(err error) error {
			// Could suggest creating the file or checking path
			return fmt.Errorf("%w (hint: check if file exists or path is correct)", err)
		},
	},
	{
		ErrorPattern: "permission denied",
		FixFunc: func(err error) error {
			return fmt.Errorf("%w (hint: check file permissions or run with appropriate privileges)", err)
		},
	},
	{
		ErrorPattern: "cannot find module",
		FixFunc: func(err error) error {
			return fmt.Errorf("%w (hint: run 'go mod tidy' or check module path)", err)
		},
	},
}

// TryAutoFix attempts to apply automatic fixes to errors
func TryAutoFix(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	for _, fix := range CommonAutoFixes {
		if strings.Contains(errStr, strings.ToLower(fix.ErrorPattern)) {
			return fix.FixFunc(err)
		}
	}

	return err
}

// OperationResult tracks the result of a retried operation
type OperationResult struct {
	Success    bool
	Attempts   int
	Duration   time.Duration
	Error      error
	WasRetried bool
}

// ExecuteWithTracking runs an operation with full tracking
func ExecuteWithTracking(ctx context.Context, config RetryConfig, operation func() error, logger *slog.Logger) OperationResult {
	start := time.Now()
	result := OperationResult{}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			result.WasRetried = true
			delay := calculateDelay(config, attempt)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				result.Error = ctx.Err()
				result.Duration = time.Since(start)
				return result
			}
		}

		err := operation()
		result.Attempts = attempt + 1

		if err == nil {
			result.Success = true
			result.Duration = time.Since(start)
			return result
		}

		lastErr = TryAutoFix(err)

		if !IsRetryable(err) {
			result.Error = lastErr
			result.Duration = time.Since(start)
			return result
		}
	}

	result.Error = lastErr
	result.Duration = time.Since(start)
	return result
}
