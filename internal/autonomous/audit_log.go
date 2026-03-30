// Package autonomous - Task 29: Audit Log for all autonomous operations
package autonomous

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	// AuditEventCommand - Command execution
	AuditEventCommand AuditEventType = "command"

	// AuditEventFileRead - File read operation
	AuditEventFileRead AuditEventType = "file_read"

	// AuditEventFileWrite - File write operation
	AuditEventFileWrite AuditEventType = "file_write"

	// AuditEventFileDelete - File delete operation
	AuditEventFileDelete AuditEventType = "file_delete"

	// AuditEventApproval - Approval request/decision
	AuditEventApproval AuditEventType = "approval"

	// AuditEventSnapshot - Snapshot created/restored
	AuditEventSnapshot AuditEventType = "snapshot"

	// AuditEventRollback - Rollback operation
	AuditEventRollback AuditEventType = "rollback"

	// AuditEventError - Error occurred
	AuditEventError AuditEventType = "error"

	// AuditEventSecurity - Security-related event
	AuditEventSecurity AuditEventType = "security"

	// AuditEventSandbox - Sandbox operation
	AuditEventSandbox AuditEventType = "sandbox"

	// AuditEventNetwork - Network operation
	AuditEventNetwork AuditEventType = "network"

	// AuditEventPlan - Plan created/modified
	AuditEventPlan AuditEventType = "plan"

	// AuditEventStep - Step execution
	AuditEventStep AuditEventType = "step"
)

// AuditSeverity represents the severity level of an audit event.
type AuditSeverity string

const (
	// AuditSeverityInfo - Informational event
	AuditSeverityInfo AuditSeverity = "info"

	// AuditSeverityWarning - Warning event
	AuditSeverityWarning AuditSeverity = "warning"

	// AuditSeverityError - Error event
	AuditSeverityError AuditSeverity = "error"

	// AuditSeverityCritical - Critical security event
	AuditSeverityCritical AuditSeverity = "critical"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	// ID is the unique event identifier
	ID string `json:"id"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Type is the event type
	Type AuditEventType `json:"type"`

	// Severity is the event severity
	Severity AuditSeverity `json:"severity"`

	// Message is a human-readable description
	Message string `json:"message"`

	// SessionID is the session this event belongs to
	SessionID string `json:"session_id,omitempty"`

	// StepID is the step this event relates to
	StepID string `json:"step_id,omitempty"`

	// Operation is the specific operation performed
	Operation string `json:"operation,omitempty"`

	// Target is what was operated on (file path, command, etc.)
	Target string `json:"target,omitempty"`

	// Actor is who/what initiated the operation
	Actor string `json:"actor,omitempty"`

	// Result is the outcome (success/failure)
	Result string `json:"result,omitempty"`

	// Error contains error details if applicable
	Error string `json:"error,omitempty"`

	// DangerLevel is the assessed danger level (if applicable)
	DangerLevel string `json:"danger_level,omitempty"`

	// Approved indicates if this was approved
	Approved *bool `json:"approved,omitempty"`

	// ApprovalID references the approval request
	ApprovalID string `json:"approval_id,omitempty"`

	// Duration is how long the operation took
	Duration time.Duration `json:"duration,omitempty"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Checksum for event integrity
	Checksum string `json:"checksum"`
}

// AuditConfig configures the audit log behavior.
type AuditConfig struct {
	// Enabled turns audit logging on/off
	Enabled bool `json:"enabled"`

	// MaxEvents is the maximum events to keep in memory
	MaxEvents int `json:"max_events"`

	// PersistToFile saves audit logs to disk
	PersistToFile bool `json:"persist_to_file"`

	// LogPath is the directory for audit log files
	LogPath string `json:"log_path"`

	// RotateSize is the max size before rotating (bytes)
	RotateSize int64 `json:"rotate_size"`

	// IncludeMetadata includes full metadata in logs
	IncludeMetadata bool `json:"include_metadata"`

	// SeverityFilter only logs events at or above this severity
	SeverityFilter AuditSeverity `json:"severity_filter"`

	// EventTypes filters which event types to log (empty = all)
	EventTypes []AuditEventType `json:"event_types,omitempty"`
}

// DefaultAuditConfig returns the default audit configuration.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:         true,
		MaxEvents:       10000,
		PersistToFile:   false,
		IncludeMetadata: true,
		SeverityFilter:  AuditSeverityInfo,
	}
}

// AuditStats tracks audit log statistics.
type AuditStats struct {
	TotalEvents    int            `json:"total_events"`
	EventsByType   map[string]int `json:"events_by_type"`
	EventsByResult map[string]int `json:"events_by_result"`
	FirstEvent     *time.Time     `json:"first_event,omitempty"`
	LastEvent      *time.Time     `json:"last_event,omitempty"`
	ErrorsLogged   int            `json:"errors_logged"`
	CriticalEvents int            `json:"critical_events"`
}

// AuditLogger manages audit logging for autonomous operations.
type AuditLogger struct {
	mu sync.RWMutex

	// config is the audit configuration
	config AuditConfig

	// events stores all audit events in memory
	events []AuditEvent

	// stats tracks audit statistics
	stats AuditStats

	// sessionID is the current session identifier
	sessionID string

	// file is the current log file handle
	file *os.File

	// currentSize is the current log file size
	currentSize int64

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(config AuditConfig) *AuditLogger {
	logger := &AuditLogger{
		config: config,
		events: make([]AuditEvent, 0, config.MaxEvents),
		stats: AuditStats{
			EventsByType:   make(map[string]int),
			EventsByResult: make(map[string]int),
		},
		timeNow: time.Now,
	}

	// Initialize file logging if enabled
	if config.PersistToFile && config.LogPath != "" {
		logger.initFileLogging()
	}

	return logger
}

// SetSessionID sets the current session identifier.
func (al *AuditLogger) SetSessionID(sessionID string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.sessionID = sessionID
}

// Log records a new audit event.
func (al *AuditLogger) Log(eventType AuditEventType, severity AuditSeverity, message string, opts ...AuditOption) {
	if !al.config.Enabled {
		return
	}

	// Check severity filter
	if !al.shouldLog(severity) {
		return
	}

	// Check event type filter
	if !al.shouldLogType(eventType) {
		return
	}

	// Build the event
	event := AuditEvent{
		ID:        generateEventID(),
		Timestamp: al.timeNow(),
		Type:      eventType,
		Severity:  severity,
		Message:   message,
		SessionID: al.sessionID,
		Metadata:  make(map[string]interface{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(&event)
	}

	// Calculate checksum
	event.Checksum = calculateEventChecksum(event)

	al.mu.Lock()
	defer al.mu.Unlock()

	// Add to events
	al.events = append(al.events, event)

	// Trim if needed
	if len(al.events) > al.config.MaxEvents {
		al.events = al.events[len(al.events)-al.config.MaxEvents:]
	}

	// Update stats
	al.updateStats(event)

	// Persist to file if enabled
	if al.config.PersistToFile && al.file != nil {
		al.persistEvent(event)
	}
}

// AuditOption is a functional option for audit events.
type AuditOption func(*AuditEvent)

// WithStepID sets the step ID.
func WithStepID(stepID string) AuditOption {
	return func(e *AuditEvent) { e.StepID = stepID }
}

// WithOperation sets the operation.
func WithOperation(operation string) AuditOption {
	return func(e *AuditEvent) { e.Operation = operation }
}

// WithTarget sets the target.
func WithTarget(target string) AuditOption {
	return func(e *AuditEvent) { e.Target = target }
}

// WithActor sets the actor.
func WithActor(actor string) AuditOption {
	return func(e *AuditEvent) { e.Actor = actor }
}

// WithResult sets the result.
func WithResult(result string) AuditOption {
	return func(e *AuditEvent) { e.Result = result }
}

// WithError sets the error.
func WithError(err error) AuditOption {
	return func(e *AuditEvent) {
		if err != nil {
			e.Error = err.Error()
		}
	}
}

// WithDangerLevel sets the danger level.
func WithDangerLevel(level DangerLevel) AuditOption {
	return func(e *AuditEvent) { e.DangerLevel = level.String() }
}

// WithApproved sets the approved status.
func WithApproved(approved bool) AuditOption {
	return func(e *AuditEvent) { e.Approved = &approved }
}

// WithApprovalID sets the approval ID.
func WithApprovalID(id string) AuditOption {
	return func(e *AuditEvent) { e.ApprovalID = id }
}

// WithDuration sets the duration.
func WithDuration(d time.Duration) AuditOption {
	return func(e *AuditEvent) { e.Duration = d }
}

// WithMetadata sets metadata.
func WithMetadata(key string, value interface{}) AuditOption {
	return func(e *AuditEvent) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]interface{})
		}
		e.Metadata[key] = value
	}
}

// Convenience logging methods

// LogCommand logs a command execution.
func (al *AuditLogger) LogCommand(command string, args []string, result string, duration time.Duration, err error) {
	severity := AuditSeverityInfo
	if err != nil {
		severity = AuditSeverityError
	}

	al.Log(AuditEventCommand, severity, fmt.Sprintf("Command: %s", command),
		WithOperation("execute"),
		WithTarget(command+" "+joinArgsForAudit(args)),
		WithResult(result),
		WithDuration(duration),
		WithError(err),
	)
}

// LogFileOperation logs a file operation.
func (al *AuditLogger) LogFileOperation(operation, path string, result string, err error) {
	eventType := AuditEventFileRead
	severity := AuditSeverityInfo

	switch operation {
	case "write", "create", "modify":
		eventType = AuditEventFileWrite
	case "delete", "remove":
		eventType = AuditEventFileDelete
		severity = AuditSeverityWarning
	}

	if err != nil {
		severity = AuditSeverityError
	}

	al.Log(eventType, severity, fmt.Sprintf("File %s: %s", operation, path),
		WithOperation(operation),
		WithTarget(path),
		WithResult(result),
		WithError(err),
	)
}

// LogApproval logs an approval event.
func (al *AuditLogger) LogApproval(requestID, command string, approved bool, approvedBy, reason string) {
	severity := AuditSeverityInfo
	result := "approved"
	if !approved {
		severity = AuditSeverityWarning
		result = "denied"
	}

	al.Log(AuditEventApproval, severity, fmt.Sprintf("Approval %s: %s", result, command),
		WithOperation("approval_decision"),
		WithTarget(command),
		WithApproved(approved),
		WithApprovalID(requestID),
		WithActor(approvedBy),
		WithResult(result),
		WithMetadata("reason", reason),
	)
}

// LogSecurity logs a security-related event.
func (al *AuditLogger) LogSecurity(event, details string, severity AuditSeverity) {
	if severity == "" {
		severity = AuditSeverityWarning
	}

	al.Log(AuditEventSecurity, severity, fmt.Sprintf("Security: %s", event),
		WithOperation("security_check"),
		WithMetadata("details", details),
	)
}

// LogError logs an error event.
func (al *AuditLogger) LogError(operation, message string, err error) {
	al.Log(AuditEventError, AuditSeverityError, message,
		WithOperation(operation),
		WithError(err),
		WithResult("error"),
	)
}

// LogSnapshot logs a snapshot operation.
func (al *AuditLogger) LogSnapshot(operation, snapshotID string, err error) {
	severity := AuditSeverityInfo
	if err != nil {
		severity = AuditSeverityError
	}

	al.Log(AuditEventSnapshot, severity, fmt.Sprintf("Snapshot %s: %s", operation, snapshotID),
		WithOperation(operation),
		WithTarget(snapshotID),
		WithError(err),
	)
}

// Query methods

// GetEvents returns all events matching the filter.
func (al *AuditLogger) GetEvents(filter AuditFilter) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	result := make([]AuditEvent, 0)

	for _, event := range al.events {
		if filter.Matches(event) {
			result = append(result, event)
		}
	}

	return result
}

// GetRecentEvents returns the most recent n events.
func (al *AuditLogger) GetRecentEvents(n int) []AuditEvent {
	al.mu.RLock()
	defer al.mu.RUnlock()

	if n >= len(al.events) {
		return append([]AuditEvent{}, al.events...)
	}

	return append([]AuditEvent{}, al.events[len(al.events)-n:]...)
}

// GetStats returns audit statistics.
func (al *AuditLogger) GetStats() AuditStats {
	al.mu.RLock()
	defer al.mu.RUnlock()

	return al.stats
}

// Export exports all events to JSON.
func (al *AuditLogger) Export() ([]byte, error) {
	al.mu.RLock()
	defer al.mu.RUnlock()

	export := struct {
		SessionID string       `json:"session_id"`
		Events    []AuditEvent `json:"events"`
		Stats     AuditStats   `json:"stats"`
		Exported  time.Time    `json:"exported"`
	}{
		SessionID: al.sessionID,
		Events:    al.events,
		Stats:     al.stats,
		Exported:  al.timeNow(),
	}

	return json.MarshalIndent(export, "", "  ")
}

// Clear clears all events from memory.
func (al *AuditLogger) Clear() {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.events = make([]AuditEvent, 0, al.config.MaxEvents)
	al.stats = AuditStats{
		EventsByType:   make(map[string]int),
		EventsByResult: make(map[string]int),
	}
}

// Helper methods

func (al *AuditLogger) shouldLog(severity AuditSeverity) bool {
	severityOrder := map[AuditSeverity]int{
		AuditSeverityInfo:     0,
		AuditSeverityWarning:  1,
		AuditSeverityError:    2,
		AuditSeverityCritical: 3,
	}

	filterLevel := severityOrder[al.config.SeverityFilter]
	eventLevel := severityOrder[severity]

	return eventLevel >= filterLevel
}

func (al *AuditLogger) shouldLogType(eventType AuditEventType) bool {
	if len(al.config.EventTypes) == 0 {
		return true
	}

	for _, t := range al.config.EventTypes {
		if t == eventType {
			return true
		}
	}

	return false
}

func (al *AuditLogger) updateStats(event AuditEvent) {
	al.stats.TotalEvents++
	al.stats.EventsByType[string(event.Type)]++
	al.stats.EventsByResult[event.Result]++

	if al.stats.FirstEvent == nil {
		al.stats.FirstEvent = &event.Timestamp
	}
	al.stats.LastEvent = &event.Timestamp

	if event.Severity == AuditSeverityError {
		al.stats.ErrorsLogged++
	}
	if event.Severity == AuditSeverityCritical {
		al.stats.CriticalEvents++
	}
}

func (al *AuditLogger) initFileLogging() {
	// Create log directory if needed
	if err := os.MkdirAll(al.config.LogPath, 0755); err != nil {
		return
	}

	// Open or create log file
	filename := filepath.Join(al.config.LogPath, "audit.log")
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	al.file = file

	// Get current file size
	if info, err := file.Stat(); err == nil {
		al.currentSize = info.Size()
	}
}

func (al *AuditLogger) persistEvent(event AuditEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	data = append(data, '\n')

	// Check rotation
	if al.config.RotateSize > 0 && al.currentSize+int64(len(data)) > al.config.RotateSize {
		al.rotateFile()
	}

	n, err := al.file.Write(data)
	if err != nil {
		return
	}

	al.currentSize += int64(n)
}

func (al *AuditLogger) rotateFile() {
	if al.file != nil {
		al.file.Close()
	}

	// Rename current file
	oldPath := filepath.Join(al.config.LogPath, "audit.log")
	newPath := filepath.Join(al.config.LogPath, fmt.Sprintf("audit-%s.log", al.timeNow().Format("20060102-150405")))
	os.Rename(oldPath, newPath)

	// Create new file
	al.initFileLogging()
}

// AuditFilter filters audit events.
type AuditFilter struct {
	Types     []AuditEventType
	Severity  AuditSeverity
	SessionID string
	StepID    string
	Result    string
	StartTime *time.Time
	EndTime   *time.Time
}

// Matches checks if an event matches the filter.
func (f AuditFilter) Matches(event AuditEvent) bool {
	// Check type
	if len(f.Types) > 0 {
		found := false
		for _, t := range f.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check severity
	if f.Severity != "" && event.Severity != f.Severity {
		return false
	}

	// Check session ID
	if f.SessionID != "" && event.SessionID != f.SessionID {
		return false
	}

	// Check step ID
	if f.StepID != "" && event.StepID != f.StepID {
		return false
	}

	// Check result
	if f.Result != "" && event.Result != f.Result {
		return false
	}

	// Check time range
	if f.StartTime != nil && event.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && event.Timestamp.After(*f.EndTime) {
		return false
	}

	return true
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

func calculateEventChecksum(event AuditEvent) string {
	data := fmt.Sprintf("%s|%s|%s|%s", event.ID, event.Timestamp, event.Type, event.Message)
	return fmt.Sprintf("%x", sha256Hash(data))
}

func sha256Hash(data string) []byte {
	h := sha256.New()
	h.Write([]byte(data))
	return h.Sum(nil)
}

func joinArgsForAudit(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		if containsSpace(arg) {
			result += "\"" + arg + "\""
		} else {
			result += arg
		}
	}
	return result
}
