// Package autonomous - Task 17: Agent Debug Mode for transparency in autonomous mode
package autonomous

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// DebugLevel represents the verbosity level of debug output
type DebugLevel int

const (
	DebugLevelNone  DebugLevel = 0
	DebugLevelError DebugLevel = 1
	DebugLevelWarn  DebugLevel = 2
	DebugLevelInfo  DebugLevel = 3
	DebugLevelDebug DebugLevel = 4
	DebugLevelTrace DebugLevel = 5
)

// DebugCategory represents a category of debug output
type DebugCategory string

const (
	DebugCategoryPlanning     DebugCategory = "planning"
	DebugCategoryExecution    DebugCategory = "execution"
	DebugCategoryVerification DebugCategory = "verification"
	DebugCategoryRetry        DebugCategory = "retry"
	DebugCategoryState        DebugCategory = "state"
	DebugCategoryContext      DebugCategory = "context"
	DebugCategoryAll          DebugCategory = "all"
)

// DebugEvent represents a single debug event
type DebugEvent struct {
	Timestamp  time.Time      `json:"timestamp"`
	Level      DebugLevel     `json:"level"`
	Category   DebugCategory  `json:"category"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`
	StackTrace string         `json:"stack_trace,omitempty"`
	TaskID     string         `json:"task_id,omitempty"`
	StepID     string         `json:"step_id,omitempty"`
	Iteration  int            `json:"iteration,omitempty"`
}

// DebugSession represents a debug session for an autonomous run
type DebugSession struct {
	ID        string       `json:"id"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at,omitempty"`
	Task      string       `json:"task"`
	Events    []DebugEvent `json:"events"`
	Status    string       `json:"status"`
	Config    DebugConfig  `json:"config"`
	mu        sync.RWMutex
}

// DebugConfig configures the debug mode
type DebugConfig struct {
	Enabled        bool            `json:"enabled"`
	Level          DebugLevel      `json:"level"`
	Categories     []DebugCategory `json:"categories"`
	OutputFormat   string          `json:"output_format"` // "text", "json", "markdown"
	IncludeStack   bool            `json:"include_stack"`
	MaxEvents      int             `json:"max_events"`
	SlowThreshold  time.Duration   `json:"slow_threshold"`
	PauseOnError   bool            `json:"pause_on_error"`
	LogFile        string          `json:"log_file"`
	RealTimeOutput bool            `json:"real_time_output"`
}

// DefaultDebugConfig returns default debug configuration
func DefaultDebugConfig() DebugConfig {
	return DebugConfig{
		Enabled:        false,
		Level:          DebugLevelInfo,
		Categories:     []DebugCategory{DebugCategoryAll},
		OutputFormat:   "text",
		IncludeStack:   false,
		MaxEvents:      10000,
		SlowThreshold:  5 * time.Second,
		PauseOnError:   false,
		RealTimeOutput: false,
	}
}

// DebugMode provides transparency for autonomous agent operations
type DebugMode struct {
	mu           sync.RWMutex
	config       DebugConfig
	session      *DebugSession
	eventChan    chan DebugEvent
	outputWriter *os.File
	pauseChan    chan struct{}
	resumeChan   chan struct{}
}

// NewDebugMode creates a new debug mode instance
func NewDebugMode(config DebugConfig) *DebugMode {
	dm := &DebugMode{
		config:     config,
		eventChan:  make(chan DebugEvent, 1000),
		pauseChan:  make(chan struct{}),
		resumeChan: make(chan struct{}),
	}

	if config.LogFile != "" {
		f, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			dm.outputWriter = f
		}
	}

	if dm.outputWriter == nil {
		dm.outputWriter = os.Stderr
	}

	return dm
}

// StartSession starts a new debug session
func (dm *DebugMode) StartSession(task string) *DebugSession {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	session := &DebugSession{
		ID:        fmt.Sprintf("debug_%d", time.Now().UnixNano()),
		StartedAt: time.Now(),
		Task:      task,
		Events:    make([]DebugEvent, 0),
		Status:    "running",
		Config:    dm.config,
	}

	dm.session = session

	// Start event processor if real-time output is enabled
	if dm.config.RealTimeOutput {
		go dm.processEvents()
	}

	// Use logLocked since we already hold the mutex
	dm.logLocked(DebugEvent{
		Timestamp: time.Now(),
		Level:     DebugLevelInfo,
		Category:  DebugCategoryState,
		Message:   "Debug session started",
		Details:   map[string]any{"task": task, "id": session.ID},
	})

	return session
}

// EndSession ends the current debug session
func (dm *DebugMode) EndSession(status string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.session == nil {
		return
	}

	dm.session.EndedAt = time.Now()
	dm.session.Status = status

	// Use logLocked since we already hold the mutex
	dm.logLocked(DebugEvent{
		Timestamp: time.Now(),
		Level:     DebugLevelInfo,
		Category:  DebugCategoryState,
		Message:   "Debug session ended",
		Details: map[string]any{
			"status":   status,
			"duration": dm.session.EndedAt.Sub(dm.session.StartedAt).String(),
			"events":   len(dm.session.Events),
		},
	})
}

// Log logs a debug event
func (dm *DebugMode) Log(level DebugLevel, category DebugCategory, message string, details map[string]any) {
	dm.log(level, category, message, details)
}

// log is the internal logging method (acquires lock)
func (dm *DebugMode) log(level DebugLevel, category DebugCategory, message string, details map[string]any) {
	if !dm.config.Enabled || level > dm.config.Level {
		return
	}

	// Check if category is enabled
	if !dm.isCategoryEnabled(category) {
		return
	}

	event := DebugEvent{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
		Details:   details,
	}

	if dm.config.IncludeStack && level <= DebugLevelError {
		event.StackTrace = getStackTrace()
	}

	dm.mu.Lock()
	dm.logLocked(event)
	dm.mu.Unlock()

	// Send to channel for real-time output or write directly
	if dm.config.RealTimeOutput {
		select {
		case dm.eventChan <- event:
		default:
			// Channel full, skip
		}
	} else {
		dm.writeEvent(event)
	}

	// Pause on error if configured
	if dm.config.PauseOnError && level == DebugLevelError {
		dm.Pause()
		<-dm.resumeChan
	}
}

// logLocked logs an event assuming the mutex is already held
func (dm *DebugMode) logLocked(event DebugEvent) {
	if dm.session != nil {
		// Trim old events if max reached
		if len(dm.session.Events) >= dm.config.MaxEvents {
			// Remove oldest 10% of events or at least 1
			trimCount := dm.config.MaxEvents / 10
			if trimCount < 1 {
				trimCount = 1
			}
			if trimCount > len(dm.session.Events) {
				trimCount = len(dm.session.Events)
			}
			dm.session.Events = dm.session.Events[trimCount:]
		}
		dm.session.Events = append(dm.session.Events, event)
	}
}

// isCategoryEnabled checks if a category is enabled
func (dm *DebugMode) isCategoryEnabled(category DebugCategory) bool {
	for _, c := range dm.config.Categories {
		if c == DebugCategoryAll || c == category {
			return true
		}
	}
	return false
}

// processEvents processes events from the channel for real-time output
func (dm *DebugMode) processEvents() {
	for event := range dm.eventChan {
		dm.writeEvent(event)
	}
}

// writeEvent writes an event to the output
func (dm *DebugMode) writeEvent(event DebugEvent) {
	var output string

	switch dm.config.OutputFormat {
	case "json":
		output = dm.formatJSON(event)
	case "markdown":
		output = dm.formatMarkdown(event)
	default:
		output = dm.formatText(event)
	}

	dm.outputWriter.WriteString(output + "\n")
}

// formatText formats an event as plain text
func (dm *DebugMode) formatText(event DebugEvent) string {
	levelStr := levelToString(event.Level)
	categoryStr := string(event.Category)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s/%s: %s",
		event.Timestamp.Format("15:04:05.000"),
		levelStr,
		categoryStr,
		event.Message,
	))

	if event.Duration > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", event.Duration))
	}

	if event.TaskID != "" {
		sb.WriteString(fmt.Sprintf(" [task:%s]", event.TaskID))
	}

	if event.StepID != "" {
		sb.WriteString(fmt.Sprintf(" [step:%s]", event.StepID))
	}

	if event.Iteration > 0 {
		sb.WriteString(fmt.Sprintf(" [iter:%d]", event.Iteration))
	}

	return sb.String()
}

// formatJSON formats an event as JSON
func (dm *DebugMode) formatJSON(event DebugEvent) string {
	return fmt.Sprintf(`{"timestamp":"%s","level":"%s","category":"%s","message":"%s"}`,
		event.Timestamp.Format(time.RFC3339Nano),
		levelToString(event.Level),
		event.Category,
		escapeJSON(event.Message),
	)
}

// formatMarkdown formats an event as Markdown
func (dm *DebugMode) formatMarkdown(event DebugEvent) string {
	levelEmoji := levelToEmoji(event.Level)
	return fmt.Sprintf("- %s **%s** [%s]: %s",
		levelEmoji,
		event.Timestamp.Format("15:04:05"),
		event.Category,
		event.Message,
	)
}

// LogPlanning logs a planning phase event
func (dm *DebugMode) LogPlanning(message string, plan interface{}) {
	details := map[string]any{}
	if plan != nil {
		details["plan"] = plan
	}
	dm.log(DebugLevelInfo, DebugCategoryPlanning, message, details)
}

// LogExecution logs an execution phase event
func (dm *DebugMode) LogExecution(stepID, action string, duration time.Duration, err error) {
	details := map[string]any{
		"action":   action,
		"duration": duration.String(),
	}

	level := DebugLevelInfo
	if err != nil {
		level = DebugLevelError
		details["error"] = err.Error()
	}

	if duration > dm.config.SlowThreshold {
		level = DebugLevelWarn
		details["slow"] = true
	}

	event := DebugEvent{
		Timestamp: time.Now(),
		Level:     level,
		Category:  DebugCategoryExecution,
		Message:   fmt.Sprintf("Executed: %s", action),
		Details:   details,
		Duration:  duration,
		StepID:    stepID,
	}

	dm.logEvent(event)
}

// LogVerification logs a verification phase event
func (dm *DebugMode) LogVerification(stepID string, passed bool, details map[string]any) {
	level := DebugLevelInfo
	message := "Verification passed"
	if !passed {
		level = DebugLevelWarn
		message = "Verification failed"
	}

	details["passed"] = passed

	event := DebugEvent{
		Timestamp: time.Now(),
		Level:     level,
		Category:  DebugCategoryVerification,
		Message:   message,
		Details:   details,
		StepID:    stepID,
	}

	dm.logEvent(event)
}

// LogRetry logs a retry event
func (dm *DebugMode) LogRetry(iteration int, reason string, willRetry bool) {
	level := DebugLevelWarn
	message := fmt.Sprintf("Retry %d: %s", iteration, reason)
	if !willRetry {
		level = DebugLevelError
		message = fmt.Sprintf("Retry %d exhausted: %s", iteration, reason)
	}

	dm.log(level, DebugCategoryRetry, message, map[string]any{
		"iteration":  iteration,
		"reason":     reason,
		"will_retry": willRetry,
	})
}

// LogState logs a state change event
func (dm *DebugMode) LogState(from, to string, details map[string]any) {
	if details == nil {
		details = make(map[string]any)
	}
	details["from_state"] = from
	details["to_state"] = to
	dm.log(DebugLevelDebug, DebugCategoryState, fmt.Sprintf("State: %s -> %s", from, to), details)
}

// LogContext logs a context-related event
func (dm *DebugMode) LogContext(operation string, tokens int, files []string) {
	dm.log(DebugLevelDebug, DebugCategoryContext, operation, map[string]any{
		"tokens": tokens,
		"files":  files,
	})
}

// logEvent is an internal method to log a pre-built event
func (dm *DebugMode) logEvent(event DebugEvent) {
	if !dm.config.Enabled || event.Level > dm.config.Level {
		return
	}
	if !dm.isCategoryEnabled(event.Category) {
		return
	}

	dm.mu.Lock()
	if dm.session != nil {
		if len(dm.session.Events) >= dm.config.MaxEvents {
			dm.session.Events = dm.session.Events[1000:]
		}
		dm.session.Events = append(dm.session.Events, event)
	}
	dm.mu.Unlock()

	if dm.config.RealTimeOutput {
		select {
		case dm.eventChan <- event:
		default:
		}
	} else {
		dm.writeEvent(event)
	}
}

// Pause pauses the debug session (for interactive debugging)
func (dm *DebugMode) Pause() {
	dm.pauseChan <- struct{}{}
}

// Resume resumes the debug session
func (dm *DebugMode) Resume() {
	dm.resumeChan <- struct{}{}
}

// GetSession returns the current session
func (dm *DebugMode) GetSession() *DebugSession {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.session
}

// GetEvents returns all events from the current session
func (dm *DebugMode) GetEvents() []DebugEvent {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return nil
	}
	return dm.session.Events
}

// GetEventsByLevel returns events filtered by level
func (dm *DebugMode) GetEventsByLevel(level DebugLevel) []DebugEvent {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return nil
	}

	var result []DebugEvent
	for _, e := range dm.session.Events {
		if e.Level == level {
			result = append(result, e)
		}
	}
	return result
}

// GetEventsByCategory returns events filtered by category
func (dm *DebugMode) GetEventsByCategory(category DebugCategory) []DebugEvent {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return nil
	}

	var result []DebugEvent
	for _, e := range dm.session.Events {
		if e.Category == category {
			result = append(result, e)
		}
	}
	return result
}

// GetErrors returns all error-level events
func (dm *DebugMode) GetErrors() []DebugEvent {
	return dm.GetEventsByLevel(DebugLevelError)
}

// GetWarnings returns all warning-level events
func (dm *DebugMode) GetWarnings() []DebugEvent {
	return dm.GetEventsByLevel(DebugLevelWarn)
}

// GetSlowOperations returns events that exceeded the slow threshold
func (dm *DebugMode) GetSlowOperations() []DebugEvent {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return nil
	}

	var result []DebugEvent
	for _, e := range dm.session.Events {
		if e.Duration > dm.config.SlowThreshold {
			result = append(result, e)
		}
	}
	return result
}

// GenerateReport generates a debug report for the session
func (dm *DebugMode) GenerateReport() string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return "No debug session"
	}

	var sb strings.Builder

	sb.WriteString("# Debug Session Report\n\n")
	sb.WriteString(fmt.Sprintf("**Session ID:** %s\n", dm.session.ID))
	sb.WriteString(fmt.Sprintf("**Task:** %s\n", dm.session.Task))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", dm.session.Status))
	sb.WriteString(fmt.Sprintf("**Started:** %s\n", dm.session.StartedAt.Format(time.RFC3339)))

	if !dm.session.EndedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("**Ended:** %s\n", dm.session.EndedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("**Duration:** %s\n", dm.session.EndedAt.Sub(dm.session.StartedAt)))
	}

	sb.WriteString("\n## Statistics\n\n")

	errors := dm.GetErrors()
	warnings := dm.GetWarnings()
	slowOps := dm.GetSlowOperations()

	sb.WriteString(fmt.Sprintf("- **Total Events:** %d\n", len(dm.session.Events)))
	sb.WriteString(fmt.Sprintf("- **Errors:** %d\n", len(errors)))
	sb.WriteString(fmt.Sprintf("- **Warnings:** %d\n", len(warnings)))
	sb.WriteString(fmt.Sprintf("- **Slow Operations:** %d\n", len(slowOps)))

	if len(errors) > 0 {
		sb.WriteString("\n## Errors\n\n")
		for _, e := range errors {
			sb.WriteString(fmt.Sprintf("- %s\n", e.Message))
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("\n## Warnings\n\n")
		for _, e := range warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", e.Message))
		}
	}

	if len(slowOps) > 0 {
		sb.WriteString("\n## Slow Operations\n\n")
		for _, e := range slowOps {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", e.Message, e.Duration))
		}
	}

	return sb.String()
}

// ExportJSON exports the session as JSON
func (dm *DebugMode) ExportJSON() string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.session == nil {
		return "{}"
	}

	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"id\": \"%s\",\n", dm.session.ID))
	sb.WriteString(fmt.Sprintf("  \"task\": \"%s\",\n", escapeJSON(dm.session.Task)))
	sb.WriteString(fmt.Sprintf("  \"status\": \"%s\",\n", dm.session.Status))
	sb.WriteString(fmt.Sprintf("  \"started_at\": \"%s\",\n", dm.session.StartedAt.Format(time.RFC3339)))

	if !dm.session.EndedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("  \"ended_at\": \"%s\",\n", dm.session.EndedAt.Format(time.RFC3339)))
	}

	sb.WriteString("  \"events\": [\n")
	for i, e := range dm.session.Events {
		sb.WriteString("    " + dm.formatJSON(e))
		if i < len(dm.session.Events)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ]\n")
	sb.WriteString("}")

	return sb.String()
}

// SetLevel sets the debug level
func (dm *DebugMode) SetLevel(level DebugLevel) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.config.Level = level
}

// Enable enables debug mode
func (dm *DebugMode) Enable() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.config.Enabled = true
}

// Disable disables debug mode
func (dm *DebugMode) Disable() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.config.Enabled = false
}

// IsEnabled returns whether debug mode is enabled
func (dm *DebugMode) IsEnabled() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.config.Enabled
}

// SetCategories sets the debug categories
func (dm *DebugMode) SetCategories(categories []DebugCategory) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.config.Categories = categories
}

// Helper functions

func levelToString(level DebugLevel) string {
	switch level {
	case DebugLevelNone:
		return "NONE"
	case DebugLevelError:
		return "ERROR"
	case DebugLevelWarn:
		return "WARN"
	case DebugLevelInfo:
		return "INFO"
	case DebugLevelDebug:
		return "DEBUG"
	case DebugLevelTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

func levelToEmoji(level DebugLevel) string {
	switch level {
	case DebugLevelError:
		return "❌"
	case DebugLevelWarn:
		return "⚠️"
	case DebugLevelInfo:
		return "ℹ️"
	case DebugLevelDebug:
		return "🔍"
	case DebugLevelTrace:
		return "📍"
	default:
		return "•"
	}
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func getStackTrace() string {
	return "" // Simplified - would use runtime.Callers in production
}

// DebugModeBuilder helps create debug configurations
type DebugModeBuilder struct {
	config DebugConfig
}

// NewDebugModeBuilder creates a new debug mode builder
func NewDebugModeBuilder() *DebugModeBuilder {
	return &DebugModeBuilder{
		config: DefaultDebugConfig(),
	}
}

// WithEnabled sets whether debug is enabled
func (b *DebugModeBuilder) WithEnabled(enabled bool) *DebugModeBuilder {
	b.config.Enabled = enabled
	return b
}

// WithLevel sets the debug level
func (b *DebugModeBuilder) WithLevel(level DebugLevel) *DebugModeBuilder {
	b.config.Level = level
	return b
}

// WithCategories sets the debug categories
func (b *DebugModeBuilder) WithCategories(categories ...DebugCategory) *DebugModeBuilder {
	b.config.Categories = categories
	return b
}

// WithOutputFormat sets the output format
func (b *DebugModeBuilder) WithOutputFormat(format string) *DebugModeBuilder {
	b.config.OutputFormat = format
	return b
}

// WithIncludeStack sets whether to include stack traces
func (b *DebugModeBuilder) WithIncludeStack(include bool) *DebugModeBuilder {
	b.config.IncludeStack = include
	return b
}

// WithMaxEvents sets the maximum number of events
func (b *DebugModeBuilder) WithMaxEvents(max int) *DebugModeBuilder {
	b.config.MaxEvents = max
	return b
}

// WithSlowThreshold sets the slow operation threshold
func (b *DebugModeBuilder) WithSlowThreshold(threshold time.Duration) *DebugModeBuilder {
	b.config.SlowThreshold = threshold
	return b
}

// WithPauseOnError sets whether to pause on errors
func (b *DebugModeBuilder) WithPauseOnError(pause bool) *DebugModeBuilder {
	b.config.PauseOnError = pause
	return b
}

// WithLogFile sets the log file path
func (b *DebugModeBuilder) WithLogFile(path string) *DebugModeBuilder {
	b.config.LogFile = path
	return b
}

// WithRealTimeOutput sets whether to output in real-time
func (b *DebugModeBuilder) WithRealTimeOutput(realtime bool) *DebugModeBuilder {
	b.config.RealTimeOutput = realtime
	return b
}

// Build returns the configured DebugMode
func (b *DebugModeBuilder) Build() *DebugMode {
	return NewDebugMode(b.config)
}

// BuildConfig returns the configuration
func (b *DebugModeBuilder) BuildConfig() DebugConfig {
	return b.config
}
