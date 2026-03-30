package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"log/slog"
)

// TestRetryConfigDefaults verifies default values for RetryConfig
func TestRetryConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", cfg.MaxRetries)
	}

	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay to be 1s, got %v", cfg.BaseDelay)
	}

	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", cfg.MaxDelay)
	}

	if cfg.BackoffMult != 2.0 {
		t.Errorf("Expected BackoffMult to be 2.0, got %f", cfg.BackoffMult)
	}
}

// TestIsRetryableNilError verifies nil errors are not retryable
func TestIsRetryableNilError(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("Expected nil error to not be retryable")
	}
}

// TestIsRetryableRetryableError verifies retryable error patterns are detected
func TestIsRetryableRetryableError(t *testing.T) {
	retryableErrors := []string{
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

	for _, pattern := range retryableErrors {
		err := errors.New(pattern)
		if !IsRetryable(err) {
			t.Errorf("Expected '%s' to be retryable", pattern)
		}
	}
}

// TestIsRetryableNonRetryableError verifies non-retryable errors are not retryable
func TestIsRetryableNonRetryableError(t *testing.T) {
	err := errors.New("permanent failure")
	if IsRetryable(err) {
		t.Error("Expected regular error to not be retryable")
	}
}

// TestWithRetrySuccess verifies successful function returns immediately
func TestWithRetrySuccess(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	cfg := RetryConfig{
		MaxRetries:  3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	err := WithRetry(context.Background(), cfg, operation, logger)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestWithRetryEventualSuccess verifies retry succeeds eventually
func TestWithRetryEventualSuccess(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("timeout") // retryable error
		}
		return nil
	}

	cfg := RetryConfig{
		MaxRetries:  5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	err := WithRetry(context.Background(), cfg, operation, logger)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestWithRetryExhausted verifies error when max retries exhausted
func TestWithRetryExhausted(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("timeout") // retryable error
	}

	cfg := RetryConfig{
		MaxRetries:  3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	err := WithRetry(context.Background(), cfg, operation, logger)
	if err == nil {
		t.Error("Expected error when max retries exhausted")
	}

	if attempts != 4 { // initial + 3 retries
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}
}

// TestWithRetryNonRetryableError verifies non-retryable errors stop immediately
func TestWithRetryNonRetryableError(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("permanent failure")
	}

	cfg := RetryConfig{
		MaxRetries:  5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	err := WithRetry(context.Background(), cfg, operation, logger)
	if err == nil {
		t.Error("Expected error for non-retryable failure")
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

// TestWithRetryContextCancellation verifies context cancellation stops retry
func TestWithRetryContextCancellation(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	operation := func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("timeout")
	}

	cfg := RetryConfig{
		MaxRetries:  10,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	err := WithRetry(ctx, cfg, operation, logger)
	if err == nil {
		t.Error("Expected error when context cancelled")
	}

	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts before cancellation, got %d", attempts)
	}
}

// TestExecuteWithTrackingResultValidation verifies result tracking on success
func TestExecuteWithTrackingResultValidation(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("timeout")
		}
		return nil
	}

	cfg := RetryConfig{
		MaxRetries:  5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	result := ExecuteWithTracking(context.Background(), cfg, operation, logger)

	if !result.Success {
		t.Error("Expected result.Success to be true")
	}

	if result.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", result.Attempts)
	}

	if !result.WasRetried {
		t.Error("Expected WasRetried to be true")
	}

	if result.Error != nil {
		t.Errorf("Expected no error, got %v", result.Error)
	}

	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}
}

// TestExecuteWithTrackingFailure verifies tracking on complete failure
func TestExecuteWithTrackingFailure(t *testing.T) {
	logger := slog.Default()
	operation := func() error {
		return errors.New("timeout")
	}

	cfg := RetryConfig{
		MaxRetries:  2,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	result := ExecuteWithTracking(context.Background(), cfg, operation, logger)

	if result.Success {
		t.Error("Expected result.Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected error when all attempts fail")
	}
}

// TestExecuteWithTrackingNonRetryable verifies tracking with non-retryable error
func TestExecuteWithTrackingNonRetryable(t *testing.T) {
	logger := slog.Default()
	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("permanent failure")
	}

	cfg := RetryConfig{
		MaxRetries:  5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	result := ExecuteWithTracking(context.Background(), cfg, operation, logger)

	if result.Success {
		t.Error("Expected result.Success to be false")
	}

	if result.Attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", result.Attempts)
	}

	if result.WasRetried {
		t.Error("Expected WasRetried to be false")
	}
}

// TestExecuteWithTrackingContextCancellation verifies tracking with cancelled context
func TestExecuteWithTrackingContextCancellation(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	operation := func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("timeout")
	}

	cfg := RetryConfig{
		MaxRetries:  10,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		BackoffMult: 2.0,
	}

	result := ExecuteWithTracking(ctx, cfg, operation, logger)

	if result.Success {
		t.Error("Expected result.Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected error when context cancelled")
	}
}

// TestTryAutoFixErrorEnhancements verifies error wrapping with hints
func TestTryAutoFixErrorEnhancements(t *testing.T) {
	testCases := []struct {
		pattern string
		errStr  string
	}{
		{"no such file or directory", "no such file or directory"},
		{"permission denied", "permission denied"},
		{"cannot find module", "cannot find module"},
	}

	for _, tc := range testCases {
		err := errors.New(tc.errStr)
		enhanced := TryAutoFix(err)

		if enhanced == nil {
			t.Fatal("Expected enhanced error, got nil")
		}

		if enhanced == err {
			t.Error("Expected new error wrapping original")
		}

		if !strings.Contains(enhanced.Error(), "hint:") {
			t.Errorf("Expected hint in error message, got: %s", enhanced.Error())
		}
	}
}

// TestTryAutoFixNilError verifies TryAutoFix handles nil
func TestTryAutoFixNilError(t *testing.T) {
	result := TryAutoFix(nil)
	if result != nil {
		t.Error("Expected nil when passing nil error")
	}
}

// TestTryAutoFixUnknownError verifies unknown errors pass through unchanged
func TestTryAutoFixUnknownError(t *testing.T) {
	err := errors.New("unknown error")
	result := TryAutoFix(err)

	if result != err {
		t.Error("Expected unknown error to pass through unchanged")
	}
}

// TestRetryableError verifies error message formatting
func TestRetryableError(t *testing.T) {
	inner := errors.New("inner error")
	retryable := &RetryableError{
		Err:        inner,
		Context:    "test operation",
		Suggestion: "try again",
	}

	msg := retryable.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}

	if !strings.Contains(msg, "test operation") {
		t.Errorf("Expected context in error message, got: %s", msg)
	}

	if !strings.Contains(msg, "try again") {
		t.Errorf("Expected suggestion in error message, got: %s", msg)
	}
}

// TestCalculateDelay verifies backoff calculation
func TestCalculateDelay(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:  5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    10 * time.Second,
		BackoffMult: 2.0,
	}

	delays := []time.Duration{
		calculateDelay(cfg, 1),
		calculateDelay(cfg, 2),
		calculateDelay(cfg, 3),
		calculateDelay(cfg, 4),
	}

	// Verify exponential backoff
	if delays[0] >= delays[1] {
		t.Error("Expected delay to increase with attempt")
	}

	if delays[1] >= delays[2] {
		t.Error("Expected delay to increase with attempt")
	}

	// Verify max delay cap
	if delays[3] > cfg.MaxDelay {
		t.Errorf("Expected delay to be capped at MaxDelay, got %v", delays[3])
	}
}

// TestCalculateDelayAtMax verifies delay at max retries
func TestCalculateDelayAtMax(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:  3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		BackoffMult: 2.0,
	}

	// High attempt numbers should be capped
	delay := calculateDelay(cfg, 100)
	if delay > cfg.MaxDelay {
		t.Errorf("Expected delay to be capped, got %v", delay)
	}
}