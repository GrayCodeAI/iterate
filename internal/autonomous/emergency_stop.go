// Package autonomous provides emergency stop functionality for the autonomous engine.
// Task 35: Emergency Stop mechanism for runaway agents.

package autonomous

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// EmergencyStopReason defines why an emergency stop was triggered.
type EmergencyStopReason string

const (
	EmergencyStopManual          EmergencyStopReason = "manual"            // User-triggered
	EmergencyStopInfiniteLoop    EmergencyStopReason = "infinite_loop"     // Loop detection
	EmergencyStopResourceExhaust EmergencyStopReason = "resource_exhaust"  // Memory/CPU limits
	EmergencyStopCriticalError   EmergencyStopReason = "critical_error"    // Unrecoverable error
	EmergencyStopTimeout         EmergencyStopReason = "timeout"           // Execution timeout
	EmergencyStopUserAbort       EmergencyStopReason = "user_abort"        // User requested abort
	EmergencyStopSafetyViolation EmergencyStopReason = "safety_violation"  // Safety policy breach
	EmergencyStopExternalSignal  EmergencyStopReason = "external_signal"   // External monitoring trigger
)

// EmergencyStopSeverity defines the severity level of the stop.
type EmergencyStopSeverity int

const (
	SeverityWarning EmergencyStopSeverity = iota // Can continue with caution
	SeverityCritical                             // Must stop immediately
	SeverityFatal                                // System-level issue, full halt
)

// EmergencyStopTrigger represents what triggered the stop.
type EmergencyStopTrigger struct {
	Reason      EmergencyStopReason
	Severity    EmergencyStopSeverity
	Message     string
	Source      string    // "user", "system", "monitor", "engine"
	Timestamp   time.Time
	Context     map[string]interface{} // Additional context
	Recovered   bool                   // Whether recovery was attempted
	Stacktrace  string                 // Optional stack trace
}

// EmergencyStopConfig holds configuration for emergency stop behavior.
type EmergencyStopConfig struct {
	Enabled                bool          // Enable emergency stop system
	AutoRecovery           bool          // Attempt automatic recovery
	MaxConsecutiveErrors   int           // Errors before triggering stop (default: 5)
	MaxRepeatedActions     int           // Same action repeated before stop (default: 3)
	LoopDetectionWindow    time.Duration // Window for loop detection (default: 30s)
	LoopDetectionThreshold int           // Max identical actions in window (default: 5)
	GracefulTimeout        time.Duration // Time for graceful shutdown (default: 5s)
	RollbackOnStop         bool          // Rollback to last snapshot on stop
	NotifyOnStop           bool          // Send notification on emergency stop
}

// DefaultEmergencyStopConfig returns the default configuration.
func DefaultEmergencyStopConfig() EmergencyStopConfig {
	return EmergencyStopConfig{
		Enabled:                true,
		AutoRecovery:           true,
		MaxConsecutiveErrors:   5,
		MaxRepeatedActions:     3,
		LoopDetectionWindow:    30 * time.Second,
		LoopDetectionThreshold: 5,
		GracefulTimeout:        5 * time.Second,
		RollbackOnStop:         true,
		NotifyOnStop:           true,
	}
}

// EmergencyStopManager manages emergency stop functionality.
type EmergencyStopManager struct {
	config         EmergencyStopConfig
	mu             sync.RWMutex
	stopped        atomic.Bool
	trigger        *EmergencyStopTrigger
	actionHistory  []actionRecord
	errorCount     int
	logger         *slog.Logger
	engine         *Engine
	auditLogger    *AuditLogger
	notifyCallback func(trigger EmergencyStopTrigger)
	
	// Channels for control
	stopChan     chan struct{}
	alertChan    chan EmergencyStopTrigger
	recoveryChan chan struct{}
}

// actionRecord tracks actions for loop detection.
type actionRecord struct {
	Action    string
	Timestamp time.Time
}

// NewEmergencyStopManager creates a new emergency stop manager.
func NewEmergencyStopManager(config EmergencyStopConfig, logger *slog.Logger) *EmergencyStopManager {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &EmergencyStopManager{
		config:       config,
		logger:       logger.With("component", "emergency_stop"),
		actionHistory: make([]actionRecord, 0, 100),
		stopChan:     make(chan struct{}),
		alertChan:    make(chan EmergencyStopTrigger, 10),
		recoveryChan: make(chan struct{}),
	}
}

// SetEngine links the manager to an autonomous engine.
func (m *EmergencyStopManager) SetEngine(engine *Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engine = engine
}

// SetAuditLogger sets the audit logger for emergency events.
func (m *EmergencyStopManager) SetAuditLogger(logger *AuditLogger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogger = logger
}

// SetNotifyCallback sets a callback for emergency stop notifications.
func (m *EmergencyStopManager) SetNotifyCallback(callback func(trigger EmergencyStopTrigger)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCallback = callback
}

// Trigger initiates an emergency stop.
func (m *EmergencyStopManager) Trigger(reason EmergencyStopReason, severity EmergencyStopSeverity, message, source string, context map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.stopped.Load() {
		return fmt.Errorf("emergency stop already triggered")
	}
	
	trigger := EmergencyStopTrigger{
		Reason:    reason,
		Severity:  severity,
		Message:   message,
		Source:    source,
		Timestamp: time.Now(),
		Context:   context,
	}
	
	m.trigger = &trigger
	m.stopped.Store(true)
	
	// Log the emergency stop
	m.logger.Error("EMERGENCY STOP TRIGGERED",
		"reason", reason,
		"severity", severity,
		"message", message,
		"source", source,
	)
	
	// Log to audit log if available
	if m.auditLogger != nil {
		m.auditLogger.LogSecurity(
			fmt.Sprintf("EMERGENCY_STOP_%s", reason),
			fmt.Sprintf("%s (source: %s)", message, source),
			AuditSeverityCritical,
		)
	}
	
	// Notify via callback
	if m.notifyCallback != nil {
		go m.notifyCallback(trigger)
	}
	
	// Send alert
	select {
	case m.alertChan <- trigger:
	default:
		// Channel full, log warning
		m.logger.Warn("Alert channel full, notification may be delayed")
	}
	
	// Close stop channel to signal all listeners
	close(m.stopChan)
	
	// Trigger engine stop if linked
	if m.engine != nil {
		m.engine.Stop()
	}
	
	return nil
}

// IsStopped returns whether an emergency stop has been triggered.
func (m *EmergencyStopManager) IsStopped() bool {
	return m.stopped.Load()
}

// GetTrigger returns the emergency stop trigger details.
func (m *EmergencyStopManager) GetTrigger() *EmergencyStopTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trigger
}

// Reset clears the emergency stop state for a new session.
func (m *EmergencyStopManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.stopped.Store(false)
	m.trigger = nil
	m.actionHistory = make([]actionRecord, 0, 100)
	m.errorCount = 0
	
	// Recreate channels
	m.stopChan = make(chan struct{})
	m.alertChan = make(chan EmergencyStopTrigger, 10)
	m.recoveryChan = make(chan struct{})
	
	m.logger.Info("Emergency stop manager reset")
}

// StopChan returns the stop channel for listening.
func (m *EmergencyStopManager) StopChan() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopChan
}

// AlertChan returns the alert channel for notifications.
func (m *EmergencyStopManager) AlertChan() <-chan EmergencyStopTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertChan
}

// RecordAction records an action for loop detection.
func (m *EmergencyStopManager) RecordAction(action string) {
	m.mu.Lock()
	
	// Add to history
	m.actionHistory = append(m.actionHistory, actionRecord{
		Action:    action,
		Timestamp: time.Now(),
	})
	
	// Trim old entries
	cutoff := time.Now().Add(-m.config.LoopDetectionWindow)
	newHistory := make([]actionRecord, 0, len(m.actionHistory))
	for _, record := range m.actionHistory {
		if record.Timestamp.After(cutoff) {
			newHistory = append(newHistory, record)
		}
	}
	m.actionHistory = newHistory
	
	// Check for loops
	detectedLoop := m.detectLoop()
	
	if detectedLoop {
		m.logger.Warn("Loop detected, triggering emergency stop",
			"action", action,
			"window", m.config.LoopDetectionWindow,
		)
	}
	
	m.mu.Unlock()
	
	// Trigger outside of lock to avoid deadlock
	if detectedLoop {
		m.Trigger(
			EmergencyStopInfiniteLoop,
			SeverityCritical,
			fmt.Sprintf("Detected infinite loop: action '%s' repeated %d times in %v", action, m.config.LoopDetectionThreshold, m.config.LoopDetectionWindow),
			"system",
			map[string]interface{}{
				"action":     action,
				"threshold":  m.config.LoopDetectionThreshold,
				"window":     m.config.LoopDetectionWindow.String(),
			},
		)
	}
}

// detectLoop checks if the same action is repeated too many times.
func (m *EmergencyStopManager) detectLoop() bool {
	if len(m.actionHistory) < m.config.LoopDetectionThreshold {
		return false
	}
	
	// Count occurrences of recent actions
	actionCounts := make(map[string]int)
	for _, record := range m.actionHistory {
		actionCounts[record.Action]++
		if actionCounts[record.Action] >= m.config.LoopDetectionThreshold {
			return true
		}
	}
	
	return false
}

// RecordError records an error and checks for error threshold.
func (m *EmergencyStopManager) RecordError(err error, isCritical bool) {
	m.mu.Lock()
	
	shouldTrigger := false
	triggerSeverity := SeverityCritical
	triggerMessage := ""
	triggerSource := "system"
	triggerContext := map[string]interface{}{}
	
	if isCritical {
		m.logger.Error("Critical error recorded, triggering emergency stop", "error", err)
		shouldTrigger = true
		triggerSeverity = SeverityFatal
		triggerMessage = fmt.Sprintf("Critical error: %v", err)
		triggerSource = "engine"
		triggerContext = map[string]interface{}{"error": err.Error()}
	} else {
		m.errorCount++
		
		if m.errorCount >= m.config.MaxConsecutiveErrors {
			m.logger.Error("Max consecutive errors reached, triggering emergency stop",
				"errorCount", m.errorCount,
				"maxErrors", m.config.MaxConsecutiveErrors,
			)
			shouldTrigger = true
			triggerMessage = fmt.Sprintf("Too many consecutive errors: %d (max: %d)", m.errorCount, m.config.MaxConsecutiveErrors)
			triggerContext = map[string]interface{}{
				"errorCount": m.errorCount,
				"maxErrors":  m.config.MaxConsecutiveErrors,
			}
		}
	}
	
	m.mu.Unlock()
	
	// Trigger outside of lock to avoid deadlock
	if shouldTrigger {
		m.Trigger(
			EmergencyStopCriticalError,
			triggerSeverity,
			triggerMessage,
			triggerSource,
			triggerContext,
		)
	}
}

// ClearErrorCount resets the error counter after a successful operation.
func (m *EmergencyStopManager) ClearErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount = 0
}

// CheckResourceUsage checks resource usage against limits.
func (m *EmergencyStopManager) CheckResourceUsage(memoryMB int64, cpuPercent float64, goroutineCount int) {
	m.mu.RLock()
	
	var violations []string
	shouldTrigger := false
	
	// Memory check (warn at 80%, critical at 95%)
	if memoryMB > 2048 { // 2GB warning threshold
		violations = append(violations, fmt.Sprintf("high memory usage: %d MB", memoryMB))
	}
	if memoryMB > 4096 { // 4GB critical threshold
		m.logger.Error("Memory exhaustion detected", "memoryMB", memoryMB)
		shouldTrigger = true
	}
	
	// CPU check (warn at 90%, critical at 100% for extended period)
	if cpuPercent > 90 {
		violations = append(violations, fmt.Sprintf("high CPU usage: %.1f%%", cpuPercent))
	}
	
	// Goroutine check (potential leak detection)
	if goroutineCount > 1000 {
		m.logger.Warn("High goroutine count, potential leak", "count", goroutineCount)
		violations = append(violations, fmt.Sprintf("high goroutine count: %d", goroutineCount))
	}
	
	m.mu.RUnlock()
	
	// Log warnings outside of lock
	if len(violations) > 0 && m.logger != nil {
		m.logger.Warn("Resource usage warnings", "violations", violations)
	}
	
	// Trigger outside of lock to avoid deadlock
	if shouldTrigger {
		m.Trigger(
			EmergencyStopResourceExhaust,
			SeverityCritical,
			fmt.Sprintf("Memory exhaustion: %d MB used", memoryMB),
			"system",
			map[string]interface{}{"memory_mb": memoryMB},
		)
	}
}

// TriggerSafetyViolation triggers a stop due to safety policy breach.
func (m *EmergencyStopManager) TriggerSafetyViolation(violation, details string) {
	m.Trigger(
		EmergencyStopSafetyViolation,
		SeverityCritical,
		fmt.Sprintf("Safety violation: %s - %s", violation, details),
		"safety_system",
		map[string]interface{}{
			"violation": violation,
			"details":   details,
		},
	)
}

// AttemptRecovery attempts to recover from an emergency stop.
func (m *EmergencyStopManager) AttemptRecovery(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.stopped.Load() {
		return fmt.Errorf("no emergency stop to recover from")
	}
	
	if !m.config.AutoRecovery {
		return fmt.Errorf("auto recovery disabled")
	}
	
	m.logger.Info("Attempting recovery from emergency stop",
		"reason", m.trigger.Reason,
	)
	
	// Recovery strategy depends on the trigger reason
	switch m.trigger.Reason {
	case EmergencyStopInfiniteLoop:
		// Clear action history and reset
		m.actionHistory = make([]actionRecord, 0, 100)
		m.errorCount = 0
		m.stopped.Store(false)
		m.trigger = nil
		m.stopChan = make(chan struct{})
		m.logger.Info("Recovery successful: loop cleared")
		return nil
		
	case EmergencyStopResourceExhaust:
		// Wait for resources to free up
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			m.stopped.Store(false)
			m.trigger = nil
			m.stopChan = make(chan struct{})
			m.logger.Info("Recovery successful: resources freed")
			return nil
		}
		
	case EmergencyStopCriticalError:
		// Critical errors require manual intervention
		return fmt.Errorf("critical errors require manual intervention")
		
	default:
		return fmt.Errorf("recovery not supported for reason: %s", m.trigger.Reason)
	}
}

// GetStatus returns the current status of the emergency stop manager.
func (m *EmergencyStopManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := map[string]interface{}{
		"enabled":     m.config.Enabled,
		"stopped":     m.stopped.Load(),
		"error_count": m.errorCount,
	}
	
	if m.trigger != nil {
		status["trigger_reason"] = string(m.trigger.Reason)
		status["trigger_severity"] = m.trigger.Severity
		status["trigger_message"] = m.trigger.Message
		status["trigger_source"] = m.trigger.Source
		status["trigger_time"] = m.trigger.Timestamp
	}
	
	return status
}

// EmergencyStopHandler provides HTTP/WebSocket handler support.
type EmergencyStopHandler struct {
	manager *EmergencyStopManager
	logger  *slog.Logger
}

// NewEmergencyStopHandler creates a new handler.
func NewEmergencyStopHandler(manager *EmergencyStopManager, logger *slog.Logger) *EmergencyStopHandler {
	return &EmergencyStopHandler{
		manager: manager,
		logger:  logger,
	}
}

// HandleTrigger handles an external trigger request.
func (h *EmergencyStopHandler) HandleTrigger(reason, message, source string) error {
	return h.manager.Trigger(
		EmergencyStopReason(reason),
		SeverityCritical,
		message,
		source,
		map[string]interface{}{
			"external_trigger": true,
		},
	)
}

// HandleStatus returns status for external monitoring.
func (h *EmergencyStopHandler) HandleStatus() map[string]interface{} {
	return h.manager.GetStatus()
}

// HandleReset handles reset requests.
func (h *EmergencyStopHandler) HandleReset() error {
	h.manager.Reset()
	return nil
}
