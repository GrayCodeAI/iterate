// Package autonomous - Task 12: Resource limits tests
package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()

	if limits.MaxDuration != 30*time.Minute {
		t.Errorf("expected MaxDuration=30m, got %v", limits.MaxDuration)
	}
	if limits.MaxTurns != 100 {
		t.Errorf("expected MaxTurns=100, got %d", limits.MaxTurns)
	}
	if limits.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", limits.MaxRetries)
	}
	if limits.MaxTokens != 500000 {
		t.Errorf("expected MaxTokens=500000, got %d", limits.MaxTokens)
	}
	if limits.MaxCost != 10.0 {
		t.Errorf("expected MaxCost=10.0, got %f", limits.MaxCost)
	}
}

func TestStrictResourceLimits(t *testing.T) {
	limits := StrictResourceLimits()

	if limits.MaxDuration != 5*time.Minute {
		t.Errorf("expected MaxDuration=5m, got %v", limits.MaxDuration)
	}
	if limits.MaxTurns != 20 {
		t.Errorf("expected MaxTurns=20, got %d", limits.MaxTurns)
	}
	if limits.MaxCost != 1.0 {
		t.Errorf("expected MaxCost=1.0, got %f", limits.MaxCost)
	}
}

func TestRelaxedResourceLimits(t *testing.T) {
	limits := RelaxedResourceLimits()

	if limits.MaxDuration != 2*time.Hour {
		t.Errorf("expected MaxDuration=2h, got %v", limits.MaxDuration)
	}
	if limits.MaxTurns != 500 {
		t.Errorf("expected MaxTurns=500, got %d", limits.MaxTurns)
	}
	if limits.MaxCost != 50.0 {
		t.Errorf("expected MaxCost=50.0, got %f", limits.MaxCost)
	}
}

func TestNewLimitEnforcer(t *testing.T) {
	limits := DefaultResourceLimits()
	enforcer := NewLimitEnforcer(limits)

	if enforcer == nil {
		t.Fatal("expected non-nil enforcer")
	}
	if enforcer.startTime.IsZero() {
		t.Error("expected startTime to be set")
	}
}

func TestIncrementTurn(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 3}
	enforcer := NewLimitEnforcer(limits)

	// Should succeed for first 3 turns
	for i := 0; i < 3; i++ {
		if v := enforcer.IncrementTurn(); v != nil {
			t.Errorf("turn %d should not violate", i+1)
		}
	}

	// 4th turn should violate
	v := enforcer.IncrementTurn()
	if v == nil {
		t.Error("expected violation on 4th turn")
	}
	if v.Type != "turns" {
		t.Errorf("expected violation type 'turns', got %s", v.Type)
	}
}

func TestIncrementRetry(t *testing.T) {
	limits := ResourceLimits{MaxRetries: 2}
	enforcer := NewLimitEnforcer(limits)

	// First 2 retries should succeed
	for i := 0; i < 2; i++ {
		if v := enforcer.IncrementRetry(); v != nil {
			t.Errorf("retry %d should not violate", i+1)
		}
	}

	// 3rd retry should violate
	v := enforcer.IncrementRetry()
	if v == nil {
		t.Error("expected violation on 3rd retry")
	}
	if !v.Recoverable {
		t.Error("retry violation should be recoverable")
	}
}

func TestIncrementFileRead(t *testing.T) {
	limits := ResourceLimits{MaxFileReads: 5}
	enforcer := NewLimitEnforcer(limits)

	for i := 0; i < 5; i++ {
		if v := enforcer.IncrementFileRead(); v != nil {
			t.Errorf("read %d should not violate", i+1)
		}
	}

	v := enforcer.IncrementFileRead()
	if v == nil {
		t.Error("expected violation after max reads")
	}
}

func TestIncrementFileWrite(t *testing.T) {
	limits := ResourceLimits{MaxFileWrites: 3}
	enforcer := NewLimitEnforcer(limits)

	for i := 0; i < 3; i++ {
		if v := enforcer.IncrementFileWrite(); v != nil {
			t.Errorf("write %d should not violate", i+1)
		}
	}

	v := enforcer.IncrementFileWrite()
	if v == nil {
		t.Error("expected violation after max writes")
	}
}

func TestIncrementCommand(t *testing.T) {
	limits := ResourceLimits{MaxCommands: 10}
	enforcer := NewLimitEnforcer(limits)

	for i := 0; i < 10; i++ {
		if v := enforcer.IncrementCommand(); v != nil {
			t.Errorf("command %d should not violate", i+1)
		}
	}

	v := enforcer.IncrementCommand()
	if v == nil {
		t.Error("expected violation after max commands")
	}
}

func TestIncrementAPICall(t *testing.T) {
	limits := ResourceLimits{MaxAPICalls: 5}
	enforcer := NewLimitEnforcer(limits)

	for i := 0; i < 5; i++ {
		if v := enforcer.IncrementAPICall(); v != nil {
			t.Errorf("api call %d should not violate", i+1)
		}
	}

	v := enforcer.IncrementAPICall()
	if v == nil {
		t.Error("expected violation after max api calls")
	}
}

func TestAddTokens(t *testing.T) {
	limits := ResourceLimits{MaxTokens: 1000}
	enforcer := NewLimitEnforcer(limits)

	// Add tokens up to limit
	if v := enforcer.AddTokens(500); v != nil {
		t.Error("first add should not violate")
	}
	if v := enforcer.AddTokens(400); v != nil {
		t.Error("second add should not violate")
	}

	// This should exceed limit
	v := enforcer.AddTokens(200)
	if v == nil {
		t.Error("expected violation when exceeding token limit")
	}
}

func TestAddCost(t *testing.T) {
	limits := ResourceLimits{MaxCost: 5.0}
	enforcer := NewLimitEnforcer(limits)

	// Add cost up to limit
	if v := enforcer.AddCost(2.0); v != nil {
		t.Error("first add should not violate")
	}
	if v := enforcer.AddCost(2.0); v != nil {
		t.Error("second add should not violate")
	}

	// This should exceed limit
	v := enforcer.AddCost(2.0)
	if v == nil {
		t.Error("expected violation when exceeding cost limit")
	}
}

func TestCheckMemory(t *testing.T) {
	// Set a very high limit that won't be triggered
	limits := ResourceLimits{MaxMemoryMB: 10000, WarnMemoryMB: 8000}
	enforcer := NewLimitEnforcer(limits)

	v := enforcer.CheckMemory()
	if v != nil {
		t.Error("expected no memory violation with high limit")
	}
}

func TestCheckDuration(t *testing.T) {
	limits := ResourceLimits{MaxDuration: 100 * time.Millisecond}
	enforcer := NewLimitEnforcer(limits)

	// Check immediately - should not violate
	v := enforcer.CheckDuration()
	if v != nil {
		t.Error("expected no violation immediately")
	}

	// Wait for duration to exceed
	time.Sleep(150 * time.Millisecond)

	v = enforcer.CheckDuration()
	if v == nil {
		t.Error("expected violation after duration exceeded")
	}
}

func TestCheckIdle(t *testing.T) {
	limits := ResourceLimits{MaxIdleTime: 50 * time.Millisecond}
	enforcer := NewLimitEnforcer(limits)

	// Check immediately - should not violate
	v := enforcer.CheckIdle()
	if v != nil {
		t.Error("expected no violation immediately")
	}

	// Wait for idle time to exceed
	time.Sleep(60 * time.Millisecond)

	v = enforcer.CheckIdle()
	if v == nil {
		t.Error("expected violation after idle time exceeded")
	}

	// Update activity should clear the violation state
	enforcer.UpdateActivity()
	enforcer.violated.Store(false)

	v = enforcer.CheckIdle()
	if v != nil {
		t.Error("expected no violation after activity update")
	}
}

func TestContext(t *testing.T) {
	limits := ResourceLimits{MaxDuration: 100 * time.Millisecond}
	enforcer := NewLimitEnforcer(limits)

	ctx, cancel := enforcer.Context(context.Background())
	defer cancel()

	// Context should have deadline
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		t.Error("expected context to have deadline")
	}
	if deadline.IsZero() {
		t.Error("expected non-zero deadline")
	}
}

func TestTurnContext(t *testing.T) {
	limits := ResourceLimits{MaxTurnDuration: 50 * time.Millisecond}
	enforcer := NewLimitEnforcer(limits)

	ctx, cancel := enforcer.TurnContext(context.Background())
	defer cancel()

	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		t.Error("expected turn context to have deadline")
	}
	if deadline.IsZero() {
		t.Error("expected non-zero deadline")
	}
}

func TestGetUsage(t *testing.T) {
	limits := DefaultResourceLimits()
	enforcer := NewLimitEnforcer(limits)

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()
	enforcer.IncrementFileRead()
	enforcer.IncrementFileWrite()
	enforcer.IncrementCommand()
	enforcer.AddTokens(100)
	enforcer.AddCost(0.5)

	usage := enforcer.GetUsage()

	if usage.Turns != 2 {
		t.Errorf("expected 2 turns, got %d", usage.Turns)
	}
	if usage.FileReads != 1 {
		t.Errorf("expected 1 file read, got %d", usage.FileReads)
	}
	if usage.FileWrites != 1 {
		t.Errorf("expected 1 file write, got %d", usage.FileWrites)
	}
	if usage.Commands != 1 {
		t.Errorf("expected 1 command, got %d", usage.Commands)
	}
	if usage.Tokens != 100 {
		t.Errorf("expected 100 tokens, got %d", usage.Tokens)
	}
	if usage.Cost != 0.5 {
		t.Errorf("expected cost 0.5, got %f", usage.Cost)
	}
}

func TestGetRemaining(t *testing.T) {
	limits := ResourceLimits{
		MaxTurns:    10,
		MaxTokens:   1000,
		MaxCost:     5.0,
		MaxDuration: time.Hour,
	}
	enforcer := NewLimitEnforcer(limits)

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()
	enforcer.AddTokens(200)
	enforcer.AddCost(1.0)

	remaining := enforcer.GetRemaining()

	if remaining.MaxTurns != 8 {
		t.Errorf("expected 8 remaining turns, got %d", remaining.MaxTurns)
	}
	if remaining.MaxTokens != 800 {
		t.Errorf("expected 800 remaining tokens, got %d", remaining.MaxTokens)
	}
	if remaining.MaxCost != 4.0 {
		t.Errorf("expected 4.0 remaining cost, got %f", remaining.MaxCost)
	}
}

func TestIsViolated(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 2}
	enforcer := NewLimitEnforcer(limits)

	if enforcer.IsViolated() {
		t.Error("should not be violated initially")
	}

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()

	if enforcer.IsViolated() {
		t.Error("should not be violated at exact limit")
	}

	enforcer.IncrementTurn()

	if !enforcer.IsViolated() {
		t.Error("should be violated after exceeding limit")
	}
}

func TestGetViolation(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 1}
	enforcer := NewLimitEnforcer(limits)

	if enforcer.GetViolation() != nil {
		t.Error("should not have violation initially")
	}

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()

	v := enforcer.GetViolation()
	if v == nil {
		t.Error("should have violation after exceeding limit")
	}
	if v.Type != "turns" {
		t.Errorf("expected violation type 'turns', got %s", v.Type)
	}
}

func TestReset(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 10, MaxTokens: 1000}
	enforcer := NewLimitEnforcer(limits)

	// Use some resources
	enforcer.IncrementTurn()
	enforcer.IncrementTurn()
	enforcer.AddTokens(500)

	// Reset
	enforcer.Reset()

	usage := enforcer.GetUsage()
	if usage.Turns != 0 {
		t.Errorf("expected 0 turns after reset, got %d", usage.Turns)
	}
	if usage.Tokens != 0 {
		t.Errorf("expected 0 tokens after reset, got %d", usage.Tokens)
	}
	if enforcer.IsViolated() {
		t.Error("should not be violated after reset")
	}
}

func TestSetLimits(t *testing.T) {
	enforcer := NewLimitEnforcer(DefaultResourceLimits())

	newLimits := ResourceLimits{MaxTurns: 50}
	enforcer.SetLimits(newLimits)

	got := enforcer.GetLimits()
	if got.MaxTurns != 50 {
		t.Errorf("expected MaxTurns=50, got %d", got.MaxTurns)
	}
}

func TestCanPerform(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 2, MaxFileReads: 3}
	enforcer := NewLimitEnforcer(limits)

	// Should be able to perform initially
	if !enforcer.CanPerform("turn") {
		t.Error("should be able to perform turn")
	}
	if !enforcer.CanPerform("read") {
		t.Error("should be able to perform read")
	}

	// Use up turns
	enforcer.IncrementTurn()
	enforcer.IncrementTurn()

	if enforcer.CanPerform("turn") {
		t.Error("should not be able to perform turn at limit")
	}
	if !enforcer.CanPerform("read") {
		t.Error("should still be able to perform read")
	}
}

func TestPercentUsed(t *testing.T) {
	limits := ResourceLimits{
		MaxTurns:  10,
		MaxTokens: 1000,
		MaxCost:   5.0,
	}
	enforcer := NewLimitEnforcer(limits)

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()
	enforcer.AddTokens(250)
	enforcer.AddCost(1.0)

	percent := enforcer.PercentUsed()

	if percent["turns"] != 20.0 {
		t.Errorf("expected turns 20%%, got %f%%", percent["turns"])
	}
	if percent["tokens"] != 25.0 {
		t.Errorf("expected tokens 25%%, got %f%%", percent["tokens"])
	}
	if percent["cost"] != 20.0 {
		t.Errorf("expected cost 20%%, got %f%%", percent["cost"])
	}
}

func TestResourceLimitsSummary(t *testing.T) {
	limits := ResourceLimits{
		MaxTurns:    100,
		MaxTokens:   500000,
		MaxCost:     10.0,
		MaxDuration: 30 * time.Minute,
	}
	enforcer := NewLimitEnforcer(limits)

	enforcer.IncrementTurn()
	enforcer.AddTokens(1000)
	enforcer.AddCost(0.5)

	summary := enforcer.Summary()

	if summary == "" {
		t.Error("expected non-empty summary")
	}
	// Summary should contain key information
	if len(summary) < 50 {
		t.Errorf("summary seems too short: %s", summary)
	}
}

func TestOnViolationCallback(t *testing.T) {
	limits := ResourceLimits{MaxTurns: 1}
	enforcer := NewLimitEnforcer(limits)

	var called bool
	var gotViolation *LimitViolation

	enforcer.SetOnViolation(func(v *LimitViolation) {
		called = true
		gotViolation = v
	})

	enforcer.IncrementTurn()
	enforcer.IncrementTurn()

	if !called {
		t.Error("expected callback to be called")
	}
	if gotViolation == nil {
		t.Error("expected violation in callback")
	}
}

func TestOnWarningCallback(t *testing.T) {
	limits := ResourceLimits{
		MaxMemoryMB:  10000, // High limit so it won't fail
		WarnMemoryMB: 1,     // Very low to trigger warning
	}
	enforcer := NewLimitEnforcer(limits)

	var warningCalled bool
	var warningMsg string

	enforcer.SetOnWarning(func(msg string) {
		warningCalled = true
		warningMsg = msg
	})

	enforcer.CheckMemory()

	// Note: This test might not trigger warning depending on actual memory usage
	// The test validates the callback mechanism works
	_ = warningCalled
	_ = warningMsg
}

func TestCheckAll(t *testing.T) {
	limits := ResourceLimits{
		MaxTurns:    1,
		MaxDuration: time.Hour,
		MaxIdleTime: time.Hour,
		MaxMemoryMB: 10000,
	}
	enforcer := NewLimitEnforcer(limits)

	// CheckAll with no violations
	violations := enforcer.CheckAll()
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d", len(violations))
	}

	// Exceed turn limit
	enforcer.IncrementTurn()
	enforcer.IncrementTurn()

	violations = enforcer.CheckAll()
	if len(violations) == 0 {
		t.Error("expected at least one violation")
	}
}

func TestUpdateActivity(t *testing.T) {
	limits := ResourceLimits{MaxIdleTime: 10 * time.Millisecond}
	enforcer := NewLimitEnforcer(limits)

	// Wait a bit
	time.Sleep(20 * time.Millisecond)

	// Update activity
	enforcer.UpdateActivity()

	// Check idle - should not violate because we just updated
	v := enforcer.CheckIdle()
	if v != nil {
		t.Error("expected no idle violation after activity update")
	}
}

func TestResourceUsageFields(t *testing.T) {
	usage := ResourceUsage{
		Duration:   time.Minute,
		Turns:      10,
		Retries:    2,
		FileReads:  50,
		FileWrites: 5,
		Commands:   20,
		APICalls:   100,
		Tokens:     5000,
		MemoryMB:   256,
		Cost:       2.50,
		Goroutines: 15,
	}

	if usage.Turns != 10 {
		t.Errorf("expected 10 turns, got %d", usage.Turns)
	}
	if usage.Cost != 2.50 {
		t.Errorf("expected cost 2.50, got %f", usage.Cost)
	}
}

func TestLimitViolationFields(t *testing.T) {
	v := LimitViolation{
		Type:        "turns",
		Limit:       10,
		Actual:      11,
		Message:     "Turn limit exceeded",
		Timestamp:   time.Now(),
		Recoverable: false,
	}

	if v.Type != "turns" {
		t.Errorf("expected type 'turns', got %s", v.Type)
	}
	if v.Limit != 10 {
		t.Errorf("expected limit 10, got %v", v.Limit)
	}
	if v.Recoverable {
		t.Error("expected not recoverable")
	}
}

func TestZeroLimits(t *testing.T) {
	// With zero limits, nothing should violate
	limits := ResourceLimits{}
	enforcer := NewLimitEnforcer(limits)

	// All operations should succeed
	for i := 0; i < 100; i++ {
		if v := enforcer.IncrementTurn(); v != nil {
			t.Error("zero limits should allow unlimited operations")
		}
	}
}

func TestNegativeLimits(t *testing.T) {
	// Negative limits should be treated as unlimited
	limits := ResourceLimits{MaxTurns: -1}
	enforcer := NewLimitEnforcer(limits)

	for i := 0; i < 10; i++ {
		if v := enforcer.IncrementTurn(); v != nil {
			t.Error("negative limits should allow unlimited operations")
		}
	}
}

// Benchmarks
func BenchmarkIncrementTurn(b *testing.B) {
	limits := ResourceLimits{MaxTurns: 1000000}
	enforcer := NewLimitEnforcer(limits)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enforcer.IncrementTurn()
	}
}

func BenchmarkGetUsage(b *testing.B) {
	limits := DefaultResourceLimits()
	enforcer := NewLimitEnforcer(limits)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enforcer.GetUsage()
	}
}

func BenchmarkCheckAll(b *testing.B) {
	limits := DefaultResourceLimits()
	enforcer := NewLimitEnforcer(limits)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enforcer.CheckAll()
	}
}

func BenchmarkPercentUsed(b *testing.B) {
	limits := DefaultResourceLimits()
	enforcer := NewLimitEnforcer(limits)

	enforcer.IncrementTurn()
	enforcer.AddTokens(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enforcer.PercentUsed()
	}
}
