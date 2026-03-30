// Package autonomous - Task 12: Timeout and resource limits for autonomous operations
package autonomous

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ResourceLimits defines constraints for autonomous operations.
type ResourceLimits struct {
	// Time constraints
	MaxDuration     time.Duration // Maximum total operation time
	MaxTurnDuration time.Duration // Maximum time per turn/step
	MaxIdleTime     time.Duration // Maximum idle time before timeout

	// Operation counts
	MaxTurns      int // Maximum number of turns/steps
	MaxRetries    int // Maximum retries per operation
	MaxFileReads  int // Maximum file reads
	MaxFileWrites int // Maximum file writes
	MaxCommands   int // Maximum shell commands
	MaxAPICalls   int // Maximum API calls

	// Memory constraints
	MaxMemoryMB  int64 // Maximum memory usage in MB
	WarnMemoryMB int64 // Warning threshold for memory

	// Token/Cost constraints
	MaxTokens int64   // Maximum tokens (input + output)
	MaxCost   float64 // Maximum cost in dollars

	// Concurrency constraints
	MaxConcurrentOps int // Maximum concurrent operations
	MaxGoroutines    int // Maximum goroutines
}

// DefaultResourceLimits returns sensible default limits.
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxDuration:      30 * time.Minute,
		MaxTurnDuration:  5 * time.Minute,
		MaxIdleTime:      2 * time.Minute,
		MaxTurns:         100,
		MaxRetries:       5,
		MaxFileReads:     500,
		MaxFileWrites:    100,
		MaxCommands:      200,
		MaxAPICalls:      1000,
		MaxMemoryMB:      512,
		WarnMemoryMB:     400,
		MaxTokens:        500000,
		MaxCost:          10.0,
		MaxConcurrentOps: 10,
		MaxGoroutines:    100,
	}
}

// StrictResourceLimits returns strict limits for untrusted operations.
func StrictResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxDuration:      5 * time.Minute,
		MaxTurnDuration:  30 * time.Second,
		MaxIdleTime:      30 * time.Second,
		MaxTurns:         20,
		MaxRetries:       2,
		MaxFileReads:     50,
		MaxFileWrites:    10,
		MaxCommands:      20,
		MaxAPICalls:      100,
		MaxMemoryMB:      128,
		WarnMemoryMB:     100,
		MaxTokens:        50000,
		MaxCost:          1.0,
		MaxConcurrentOps: 3,
		MaxGoroutines:    20,
	}
}

// RelaxedResourceLimits returns relaxed limits for trusted operations.
func RelaxedResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxDuration:      2 * time.Hour,
		MaxTurnDuration:  15 * time.Minute,
		MaxIdleTime:      10 * time.Minute,
		MaxTurns:         500,
		MaxRetries:       10,
		MaxFileReads:     2000,
		MaxFileWrites:    500,
		MaxCommands:      1000,
		MaxAPICalls:      5000,
		MaxMemoryMB:      2048,
		WarnMemoryMB:     1500,
		MaxTokens:        2000000,
		MaxCost:          50.0,
		MaxConcurrentOps: 20,
		MaxGoroutines:    200,
	}
}

// LimitEnforcer enforces resource limits during operations.
type LimitEnforcer struct {
	mu           sync.RWMutex
	limits       ResourceLimits
	startTime    time.Time
	lastActivity atomic.Int64

	// Counters (atomic for thread-safety)
	turns      atomic.Int64
	retries    atomic.Int64
	fileReads  atomic.Int64
	fileWrites atomic.Int64
	commands   atomic.Int64
	apiCalls   atomic.Int64
	tokens     atomic.Int64

	// Cost tracking
	totalCost atomic.Pointer[float64]

	// State
	violated  atomic.Bool
	violation atomic.Pointer[LimitViolation]

	// Callbacks
	onViolation func(*LimitViolation)
	onWarning   func(string)
	logger      interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
	}
}

// LimitViolation represents a limit breach.
type LimitViolation struct {
	Type        string      // "timeout", "turns", "memory", "tokens", "cost", etc.
	Limit       interface{} // The limit that was exceeded
	Actual      interface{} // The actual value
	Message     string      // Human-readable message
	Timestamp   time.Time   // When it occurred
	Recoverable bool        // Whether operation can continue
}

// ResourceUsage represents current resource consumption.
type ResourceUsage struct {
	Duration      time.Duration
	Turns         int64
	Retries       int64
	FileReads     int64
	FileWrites    int64
	Commands      int64
	APICalls      int64
	Tokens        int64
	MemoryMB      int64
	Cost          float64
	ConcurrentOps int
	Goroutines    int
}

// NewLimitEnforcer creates a new limit enforcer.
func NewLimitEnforcer(limits ResourceLimits) *LimitEnforcer {
	le := &LimitEnforcer{
		limits:    limits,
		startTime: time.Now(),
	}

	// Initialize cost pointer
	cost := 0.0
	le.totalCost.Store(&cost)

	// Initialize activity time
	le.lastActivity.Store(time.Now().UnixNano())

	return le
}

// SetLogger sets the logger for the enforcer.
func (le *LimitEnforcer) SetLogger(logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}) {
	le.mu.Lock()
	le.logger = logger
	le.mu.Unlock()
}

// SetOnViolation sets the callback for limit violations.
func (le *LimitEnforcer) SetOnViolation(callback func(*LimitViolation)) {
	le.mu.Lock()
	le.onViolation = callback
	le.mu.Unlock()
}

// SetOnWarning sets the callback for warnings.
func (le *LimitEnforcer) SetOnWarning(callback func(string)) {
	le.mu.Lock()
	le.onWarning = callback
	le.mu.Unlock()
}

// UpdateActivity updates the last activity timestamp.
func (le *LimitEnforcer) UpdateActivity() {
	le.lastActivity.Store(time.Now().UnixNano())
}

// Context creates a context with timeout based on limits.
func (le *LimitEnforcer) Context(parent context.Context) (context.Context, context.CancelFunc) {
	if le.limits.MaxDuration > 0 {
		return context.WithTimeout(parent, le.limits.MaxDuration)
	}
	return context.WithCancel(parent)
}

// TurnContext creates a context for a single turn.
func (le *LimitEnforcer) TurnContext(parent context.Context) (context.Context, context.CancelFunc) {
	if le.limits.MaxTurnDuration > 0 {
		return context.WithTimeout(parent, le.limits.MaxTurnDuration)
	}
	return context.WithCancel(parent)
}

// CheckTurnLimit checks if the turn limit has been reached.
func (le *LimitEnforcer) CheckTurnLimit() *LimitViolation {
	if le.limits.MaxTurns <= 0 {
		return nil
	}

	turns := le.turns.Load()
	if turns >= int64(le.limits.MaxTurns) {
		violation := &LimitViolation{
			Type:        "turns",
			Limit:       le.limits.MaxTurns,
			Actual:      turns,
			Message:     fmt.Sprintf("Turn limit exceeded: %d >= %d", turns, le.limits.MaxTurns),
			Timestamp:   time.Now(),
			Recoverable: false,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementTurn increments the turn counter and checks limits.
func (le *LimitEnforcer) IncrementTurn() *LimitViolation {
	le.UpdateActivity()
	turns := le.turns.Add(1)

	if le.limits.MaxTurns > 0 && turns > int64(le.limits.MaxTurns) {
		violation := &LimitViolation{
			Type:        "turns",
			Limit:       le.limits.MaxTurns,
			Actual:      turns,
			Message:     fmt.Sprintf("Turn limit exceeded: %d > %d", turns, le.limits.MaxTurns),
			Timestamp:   time.Now(),
			Recoverable: false,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementRetry increments the retry counter and checks limits.
func (le *LimitEnforcer) IncrementRetry() *LimitViolation {
	le.UpdateActivity()
	retries := le.retries.Add(1)

	if le.limits.MaxRetries > 0 && retries > int64(le.limits.MaxRetries) {
		violation := &LimitViolation{
			Type:        "retries",
			Limit:       le.limits.MaxRetries,
			Actual:      retries,
			Message:     fmt.Sprintf("Retry limit exceeded: %d > %d", retries, le.limits.MaxRetries),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementFileRead increments the file read counter.
func (le *LimitEnforcer) IncrementFileRead() *LimitViolation {
	le.UpdateActivity()
	reads := le.fileReads.Add(1)

	if le.limits.MaxFileReads > 0 && reads > int64(le.limits.MaxFileReads) {
		violation := &LimitViolation{
			Type:        "file_reads",
			Limit:       le.limits.MaxFileReads,
			Actual:      reads,
			Message:     fmt.Sprintf("File read limit exceeded: %d > %d", reads, le.limits.MaxFileReads),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementFileWrite increments the file write counter.
func (le *LimitEnforcer) IncrementFileWrite() *LimitViolation {
	le.UpdateActivity()
	writes := le.fileWrites.Add(1)

	if le.limits.MaxFileWrites > 0 && writes > int64(le.limits.MaxFileWrites) {
		violation := &LimitViolation{
			Type:        "file_writes",
			Limit:       le.limits.MaxFileWrites,
			Actual:      writes,
			Message:     fmt.Sprintf("File write limit exceeded: %d > %d", writes, le.limits.MaxFileWrites),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementCommand increments the command counter.
func (le *LimitEnforcer) IncrementCommand() *LimitViolation {
	le.UpdateActivity()
	cmds := le.commands.Add(1)

	if le.limits.MaxCommands > 0 && cmds > int64(le.limits.MaxCommands) {
		violation := &LimitViolation{
			Type:        "commands",
			Limit:       le.limits.MaxCommands,
			Actual:      cmds,
			Message:     fmt.Sprintf("Command limit exceeded: %d > %d", cmds, le.limits.MaxCommands),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// IncrementAPICall increments the API call counter.
func (le *LimitEnforcer) IncrementAPICall() *LimitViolation {
	le.UpdateActivity()
	calls := le.apiCalls.Add(1)

	if le.limits.MaxAPICalls > 0 && calls > int64(le.limits.MaxAPICalls) {
		violation := &LimitViolation{
			Type:        "api_calls",
			Limit:       le.limits.MaxAPICalls,
			Actual:      calls,
			Message:     fmt.Sprintf("API call limit exceeded: %d > %d", calls, le.limits.MaxAPICalls),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// AddTokens adds to the token counter.
func (le *LimitEnforcer) AddTokens(count int64) *LimitViolation {
	le.UpdateActivity()
	total := le.tokens.Add(count)

	if le.limits.MaxTokens > 0 && total > le.limits.MaxTokens {
		violation := &LimitViolation{
			Type:        "tokens",
			Limit:       le.limits.MaxTokens,
			Actual:      total,
			Message:     fmt.Sprintf("Token limit exceeded: %d > %d", total, le.limits.MaxTokens),
			Timestamp:   time.Now(),
			Recoverable: false,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// AddCost adds to the total cost.
func (le *LimitEnforcer) AddCost(cost float64) *LimitViolation {
	le.UpdateActivity()

	// Thread-safe cost update
	for {
		oldPtr := le.totalCost.Load()
		oldCost := *oldPtr
		newCost := oldCost + cost

		if le.limits.MaxCost > 0 && newCost > le.limits.MaxCost {
			violation := &LimitViolation{
				Type:        "cost",
				Limit:       le.limits.MaxCost,
				Actual:      newCost,
				Message:     fmt.Sprintf("Cost limit exceeded: $%.2f > $%.2f", newCost, le.limits.MaxCost),
				Timestamp:   time.Now(),
				Recoverable: false,
			}
			le.recordViolation(violation)
			return violation
		}

		newPtr := &newCost
		if le.totalCost.CompareAndSwap(oldPtr, newPtr) {
			break
		}
	}
	return nil
}

// CheckMemory checks current memory usage against limits.
func (le *LimitEnforcer) CheckMemory() *LimitViolation {
	if le.limits.MaxMemoryMB <= 0 {
		return nil
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryMB := int64(m.Alloc / 1024 / 1024)

	// Check warning threshold
	if le.limits.WarnMemoryMB > 0 && memoryMB >= le.limits.WarnMemoryMB {
		msg := fmt.Sprintf("Memory usage warning: %dMB >= %dMB", memoryMB, le.limits.WarnMemoryMB)
		if le.onWarning != nil {
			le.onWarning(msg)
		}
		if le.logger != nil {
			le.logger.Warn(msg)
		}
	}

	// Check hard limit
	if memoryMB > le.limits.MaxMemoryMB {
		violation := &LimitViolation{
			Type:        "memory",
			Limit:       le.limits.MaxMemoryMB,
			Actual:      memoryMB,
			Message:     fmt.Sprintf("Memory limit exceeded: %dMB > %dMB", memoryMB, le.limits.MaxMemoryMB),
			Timestamp:   time.Now(),
			Recoverable: true,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// CheckIdle checks if the operation has been idle too long.
func (le *LimitEnforcer) CheckIdle() *LimitViolation {
	if le.limits.MaxIdleTime <= 0 {
		return nil
	}

	lastActivity := time.Unix(0, le.lastActivity.Load())
	idleTime := time.Since(lastActivity)

	if idleTime > le.limits.MaxIdleTime {
		violation := &LimitViolation{
			Type:        "idle",
			Limit:       le.limits.MaxIdleTime,
			Actual:      idleTime,
			Message:     fmt.Sprintf("Idle timeout exceeded: %v > %v", idleTime, le.limits.MaxIdleTime),
			Timestamp:   time.Now(),
			Recoverable: false,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// CheckDuration checks if total duration limit has been exceeded.
func (le *LimitEnforcer) CheckDuration() *LimitViolation {
	if le.limits.MaxDuration <= 0 {
		return nil
	}

	duration := time.Since(le.startTime)
	if duration > le.limits.MaxDuration {
		violation := &LimitViolation{
			Type:        "duration",
			Limit:       le.limits.MaxDuration,
			Actual:      duration,
			Message:     fmt.Sprintf("Duration limit exceeded: %v > %v", duration, le.limits.MaxDuration),
			Timestamp:   time.Now(),
			Recoverable: false,
		}
		le.recordViolation(violation)
		return violation
	}
	return nil
}

// CheckAll performs all limit checks.
func (le *LimitEnforcer) CheckAll() []*LimitViolation {
	var violations []*LimitViolation

	if v := le.CheckTurnLimit(); v != nil {
		violations = append(violations, v)
	}
	if v := le.CheckMemory(); v != nil {
		violations = append(violations, v)
	}
	if v := le.CheckIdle(); v != nil {
		violations = append(violations, v)
	}
	if v := le.CheckDuration(); v != nil {
		violations = append(violations, v)
	}

	return violations
}

// IsViolated returns true if any limit has been violated.
func (le *LimitEnforcer) IsViolated() bool {
	return le.violated.Load()
}

// GetViolation returns the most recent violation.
func (le *LimitEnforcer) GetViolation() *LimitViolation {
	return le.violation.Load()
}

// GetUsage returns current resource usage.
func (le *LimitEnforcer) GetUsage() ResourceUsage {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	costPtr := le.totalCost.Load()
	cost := 0.0
	if costPtr != nil {
		cost = *costPtr
	}

	return ResourceUsage{
		Duration:   time.Since(le.startTime),
		Turns:      le.turns.Load(),
		Retries:    le.retries.Load(),
		FileReads:  le.fileReads.Load(),
		FileWrites: le.fileWrites.Load(),
		Commands:   le.commands.Load(),
		APICalls:   le.apiCalls.Load(),
		Tokens:     le.tokens.Load(),
		MemoryMB:   int64(m.Alloc / 1024 / 1024),
		Cost:       cost,
		Goroutines: runtime.NumGoroutine(),
	}
}

// GetRemaining returns remaining resources.
func (le *LimitEnforcer) GetRemaining() ResourceLimits {
	usage := le.GetUsage()

	remaining := ResourceLimits{}

	if le.limits.MaxTurns > 0 {
		remaining.MaxTurns = le.limits.MaxTurns - int(usage.Turns)
	}
	if le.limits.MaxRetries > 0 {
		remaining.MaxRetries = le.limits.MaxRetries - int(usage.Retries)
	}
	if le.limits.MaxFileReads > 0 {
		remaining.MaxFileReads = le.limits.MaxFileReads - int(usage.FileReads)
	}
	if le.limits.MaxFileWrites > 0 {
		remaining.MaxFileWrites = le.limits.MaxFileWrites - int(usage.FileWrites)
	}
	if le.limits.MaxCommands > 0 {
		remaining.MaxCommands = le.limits.MaxCommands - int(usage.Commands)
	}
	if le.limits.MaxAPICalls > 0 {
		remaining.MaxAPICalls = le.limits.MaxAPICalls - int(usage.APICalls)
	}
	if le.limits.MaxTokens > 0 {
		remaining.MaxTokens = le.limits.MaxTokens - usage.Tokens
	}
	if le.limits.MaxCost > 0 {
		remaining.MaxCost = le.limits.MaxCost - usage.Cost
	}
	if le.limits.MaxMemoryMB > 0 {
		remaining.MaxMemoryMB = le.limits.MaxMemoryMB - usage.MemoryMB
	}
	if le.limits.MaxDuration > 0 {
		remaining.MaxDuration = le.limits.MaxDuration - usage.Duration
	}

	return remaining
}

// recordViolation records a limit violation.
func (le *LimitEnforcer) recordViolation(violation *LimitViolation) {
	le.violated.Store(true)
	le.violation.Store(violation)

	if le.onViolation != nil {
		le.onViolation(violation)
	}

	if le.logger != nil {
		le.logger.Warn("Limit violation",
			"type", violation.Type,
			"limit", violation.Limit,
			"actual", violation.Actual,
			"message", violation.Message,
		)
	}
}

// Reset resets all counters and state.
func (le *LimitEnforcer) Reset() {
	le.turns.Store(0)
	le.retries.Store(0)
	le.fileReads.Store(0)
	le.fileWrites.Store(0)
	le.commands.Store(0)
	le.apiCalls.Store(0)
	le.tokens.Store(0)

	cost := 0.0
	le.totalCost.Store(&cost)

	le.violated.Store(false)
	le.violation.Store(nil)

	le.startTime = time.Now()
	le.lastActivity.Store(time.Now().UnixNano())
}

// SetLimits updates the limits.
func (le *LimitEnforcer) SetLimits(limits ResourceLimits) {
	le.mu.Lock()
	le.limits = limits
	le.mu.Unlock()
}

// GetLimits returns current limits.
func (le *LimitEnforcer) GetLimits() ResourceLimits {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.limits
}

// CanPerform checks if an operation can be performed without violating limits.
func (le *LimitEnforcer) CanPerform(opType string) bool {
	le.mu.RLock()
	limits := le.limits
	le.mu.RUnlock()

	switch opType {
	case "turn":
		return limits.MaxTurns <= 0 || le.turns.Load() < int64(limits.MaxTurns)
	case "read":
		return limits.MaxFileReads <= 0 || le.fileReads.Load() < int64(limits.MaxFileReads)
	case "write":
		return limits.MaxFileWrites <= 0 || le.fileWrites.Load() < int64(limits.MaxFileWrites)
	case "command":
		return limits.MaxCommands <= 0 || le.commands.Load() < int64(limits.MaxCommands)
	case "api":
		return limits.MaxAPICalls <= 0 || le.apiCalls.Load() < int64(limits.MaxAPICalls)
	default:
		return true
	}
}

// PercentUsed returns the percentage of each resource used.
func (le *LimitEnforcer) PercentUsed() map[string]float64 {
	usage := le.GetUsage()
	limits := le.GetLimits()

	percent := make(map[string]float64)

	if limits.MaxTurns > 0 {
		percent["turns"] = float64(usage.Turns) / float64(limits.MaxTurns) * 100
	}
	if limits.MaxFileReads > 0 {
		percent["file_reads"] = float64(usage.FileReads) / float64(limits.MaxFileReads) * 100
	}
	if limits.MaxFileWrites > 0 {
		percent["file_writes"] = float64(usage.FileWrites) / float64(limits.MaxFileWrites) * 100
	}
	if limits.MaxCommands > 0 {
		percent["commands"] = float64(usage.Commands) / float64(limits.MaxCommands) * 100
	}
	if limits.MaxAPICalls > 0 {
		percent["api_calls"] = float64(usage.APICalls) / float64(limits.MaxAPICalls) * 100
	}
	if limits.MaxTokens > 0 {
		percent["tokens"] = float64(usage.Tokens) / float64(limits.MaxTokens) * 100
	}
	if limits.MaxCost > 0 {
		percent["cost"] = usage.Cost / limits.MaxCost * 100
	}
	if limits.MaxMemoryMB > 0 {
		percent["memory"] = float64(usage.MemoryMB) / float64(limits.MaxMemoryMB) * 100
	}
	if limits.MaxDuration > 0 {
		percent["duration"] = float64(usage.Duration) / float64(limits.MaxDuration) * 100
	}

	return percent
}

// Summary returns a human-readable summary of resource usage.
func (le *LimitEnforcer) Summary() string {
	usage := le.GetUsage()
	limits := le.GetLimits()
	percent := le.PercentUsed()

	var sb strings.Builder
	sb.WriteString("Resource Usage Summary:\n")
	sb.WriteString(fmt.Sprintf("  Duration: %v / %v (%.1f%%)\n",
		usage.Duration.Round(time.Second), limits.MaxDuration, percent["duration"]))
	sb.WriteString(fmt.Sprintf("  Turns: %d / %d (%.1f%%)\n",
		usage.Turns, limits.MaxTurns, percent["turns"]))
	sb.WriteString(fmt.Sprintf("  Tokens: %d / %d (%.1f%%)\n",
		usage.Tokens, limits.MaxTokens, percent["tokens"]))
	sb.WriteString(fmt.Sprintf("  Cost: $%.2f / $%.2f (%.1f%%)\n",
		usage.Cost, limits.MaxCost, percent["cost"]))
	sb.WriteString(fmt.Sprintf("  Memory: %dMB / %dMB (%.1f%%)\n",
		usage.MemoryMB, limits.MaxMemoryMB, percent["memory"]))
	sb.WriteString(fmt.Sprintf("  File Reads: %d / %d\n", usage.FileReads, limits.MaxFileReads))
	sb.WriteString(fmt.Sprintf("  File Writes: %d / %d\n", usage.FileWrites, limits.MaxFileWrites))
	sb.WriteString(fmt.Sprintf("  Commands: %d / %d\n", usage.Commands, limits.MaxCommands))
	sb.WriteString(fmt.Sprintf("  API Calls: %d / %d\n", usage.APICalls, limits.MaxAPICalls))
	sb.WriteString(fmt.Sprintf("  Goroutines: %d\n", usage.Goroutines))

	return sb.String()
}
