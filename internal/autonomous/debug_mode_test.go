// Package autonomous - Task 17: Agent Debug Mode tests
package autonomous

import (
	"testing"
	"time"
)

func TestNewDebugMode(t *testing.T) {
	config := DefaultDebugConfig()
	dm := NewDebugMode(config)

	if dm == nil {
		t.Fatal("expected debug mode, got nil")
	}

	if dm.IsEnabled() != false {
		t.Error("expected debug mode to be disabled by default")
	}
}

func TestDefaultDebugConfig(t *testing.T) {
	config := DefaultDebugConfig()

	if config.Enabled != false {
		t.Error("expected enabled to be false")
	}

	if config.Level != DebugLevelInfo {
		t.Errorf("expected level Info, got %d", config.Level)
	}

	if config.MaxEvents != 10000 {
		t.Errorf("expected max events 10000, got %d", config.MaxEvents)
	}
}

func TestDebugModeEnableDisable(t *testing.T) {
	dm := NewDebugMode(DefaultDebugConfig())

	dm.Enable()
	if !dm.IsEnabled() {
		t.Error("expected debug mode enabled")
	}

	dm.Disable()
	if dm.IsEnabled() {
		t.Error("expected debug mode disabled")
	}
}

func TestStartEndSession(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	session := dm.StartSession("test task")

	if session == nil {
		t.Fatal("expected session, got nil")
	}

	if session.Task != "test task" {
		t.Errorf("expected task 'test task', got '%s'", session.Task)
	}

	if session.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", session.Status)
	}

	dm.EndSession("completed")

	if dm.session.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", dm.session.Status)
	}

	if dm.session.EndedAt.IsZero() {
		t.Error("expected ended_at to be set")
	}
}

func TestLogEvent(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.Log(DebugLevelInfo, DebugCategoryExecution, "test message", map[string]any{"key": "value"})

	events := dm.GetEvents()
	if len(events) != 2 { // start session + log event
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestLogLevelFiltering(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	config.Level = DebugLevelWarn
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.Log(DebugLevelInfo, DebugCategoryExecution, "info message", nil)
	dm.Log(DebugLevelWarn, DebugCategoryExecution, "warn message", nil)
	dm.Log(DebugLevelError, DebugCategoryExecution, "error message", nil)

	events := dm.GetEvents()
	// Should have start session + warn + error (info filtered out)
	if len(events) != 3 {
		t.Errorf("expected 3 events (start + warn + error), got %d", len(events))
	}
}

func TestLogCategoryFiltering(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	config.Categories = []DebugCategory{DebugCategoryExecution}
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.Log(DebugLevelInfo, DebugCategoryExecution, "exec message", nil)
	dm.Log(DebugLevelInfo, DebugCategoryPlanning, "plan message", nil) // should be filtered

	events := dm.GetEvents()
	// Should have start session + execution message only
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestLogExecution(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogExecution("step-1", "read file", 100*time.Millisecond, nil)

	events := dm.GetEvents()
	found := false
	for _, e := range events {
		if e.StepID == "step-1" && e.Duration == 100*time.Millisecond {
			found = true
		}
	}

	if !found {
		t.Error("expected to find execution event")
	}
}

func TestLogExecutionSlow(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	config.SlowThreshold = 100 * time.Millisecond
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogExecution("step-1", "slow operation", 500*time.Millisecond, nil)

	slowOps := dm.GetSlowOperations()
	if len(slowOps) == 0 {
		t.Error("expected slow operation to be detected")
	}
}

func TestLogExecutionError(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogExecution("step-1", "failed operation", 100*time.Millisecond, assertError{})

	errors := dm.GetErrors()
	if len(errors) == 0 {
		t.Error("expected error to be logged")
	}
}

type assertError struct{}

func (assertError) Error() string { return "test error" }

func TestLogVerification(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogVerification("step-1", true, map[string]any{"output": "success"})
	dm.LogVerification("step-2", false, map[string]any{"output": "failure"})

	events := dm.GetEvents()
	if len(events) != 3 { // start + 2 verifications
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestLogRetry(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogRetry(1, "timeout", true)
	dm.LogRetry(3, "timeout", false)

	events := dm.GetEvents()
	// start + retry warn + retry error
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestLogState(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	config.Level = DebugLevelDebug
	dm := NewDebugMode(config)

	dm.StartSession("test")

	dm.LogState("idle", "running", nil)

	events := dm.GetEvents()
	found := false
	for _, e := range events {
		if e.Category == DebugCategoryState && e.Message == "State: idle -> running" {
			found = true
		}
	}

	if !found {
		t.Error("expected to find state change event")
	}
}

func TestGetEventsByLevel(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")
	dm.Log(DebugLevelError, DebugCategoryExecution, "error1", nil)
	dm.Log(DebugLevelWarn, DebugCategoryExecution, "warn1", nil)
	dm.Log(DebugLevelError, DebugCategoryExecution, "error2", nil)

	errors := dm.GetEventsByLevel(DebugLevelError)
	if len(errors) != 2 {
		t.Errorf("expected 2 error events, got %d", len(errors))
	}
}

func TestGetEventsByCategory(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test")
	dm.Log(DebugLevelInfo, DebugCategoryExecution, "exec1", nil)
	dm.Log(DebugLevelInfo, DebugCategoryPlanning, "plan1", nil)
	dm.Log(DebugLevelInfo, DebugCategoryExecution, "exec2", nil)

	execEvents := dm.GetEventsByCategory(DebugCategoryExecution)
	if len(execEvents) != 2 {
		t.Errorf("expected 2 execution events, got %d", len(execEvents))
	}
}

func TestGenerateReport(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test task")
	dm.Log(DebugLevelInfo, DebugCategoryExecution, "test event", nil)
	dm.Log(DebugLevelError, DebugCategoryExecution, "test error", nil)
	dm.EndSession("completed")

	report := dm.GenerateReport()

	if report == "" {
		t.Fatal("expected report, got empty string")
	}

	if !containsDebugStr(report, "test task") {
		t.Error("report missing task")
	}

	if !containsDebugStr(report, "completed") {
		t.Error("report missing status")
	}
}

func containsDebugStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsDebugStrHelper(s, substr))
}

func containsDebugStrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDebugModeExportJSON(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	dm := NewDebugMode(config)

	dm.StartSession("test task")
	dm.Log(DebugLevelInfo, DebugCategoryExecution, "test event", nil)
	dm.EndSession("completed")

	json := dm.ExportJSON()

	if json == "" {
		t.Fatal("expected JSON, got empty string")
	}

	if !containsDebugStr(json, `"task":`) {
		t.Error("JSON missing task field")
	}

	if !containsDebugStr(json, `"events":`) {
		t.Error("JSON missing events field")
	}
}

func TestSetLevel(t *testing.T) {
	dm := NewDebugMode(DefaultDebugConfig())

	dm.SetLevel(DebugLevelTrace)

	if dm.config.Level != DebugLevelTrace {
		t.Errorf("expected level Trace, got %d", dm.config.Level)
	}
}

func TestSetCategories(t *testing.T) {
	dm := NewDebugMode(DefaultDebugConfig())

	cats := []DebugCategory{DebugCategoryExecution, DebugCategoryPlanning}
	dm.SetCategories(cats)

	if len(dm.config.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(dm.config.Categories))
	}
}

func TestDebugModeBuilder(t *testing.T) {
	dm := NewDebugModeBuilder().
		WithEnabled(true).
		WithLevel(DebugLevelDebug).
		WithCategories(DebugCategoryExecution, DebugCategoryPlanning).
		WithOutputFormat("json").
		WithIncludeStack(true).
		WithMaxEvents(5000).
		WithSlowThreshold(2 * time.Second).
		WithPauseOnError(true).
		WithRealTimeOutput(true).
		Build()

	if dm == nil {
		t.Fatal("expected debug mode, got nil")
	}

	if !dm.IsEnabled() {
		t.Error("expected enabled")
	}

	if dm.config.Level != DebugLevelDebug {
		t.Errorf("expected Debug level, got %d", dm.config.Level)
	}

	if dm.config.OutputFormat != "json" {
		t.Errorf("expected json format, got %s", dm.config.OutputFormat)
	}
}

func TestDebugModeBuilderConfig(t *testing.T) {
	config := NewDebugModeBuilder().
		WithEnabled(true).
		WithLevel(DebugLevelTrace).
		BuildConfig()

	if !config.Enabled {
		t.Error("expected enabled")
	}

	if config.Level != DebugLevelTrace {
		t.Errorf("expected Trace level, got %d", config.Level)
	}
}

func TestLevelToString(t *testing.T) {
	tests := []struct {
		level    DebugLevel
		expected string
	}{
		{DebugLevelNone, "NONE"},
		{DebugLevelError, "ERROR"},
		{DebugLevelWarn, "WARN"},
		{DebugLevelInfo, "INFO"},
		{DebugLevelDebug, "DEBUG"},
		{DebugLevelTrace, "TRACE"},
	}

	for _, tt := range tests {
		result := levelToString(tt.level)
		if result != tt.expected {
			t.Errorf("levelToString(%d) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}

func TestLevelToEmoji(t *testing.T) {
	tests := []struct {
		level    DebugLevel
		expected string
	}{
		{DebugLevelError, "❌"},
		{DebugLevelWarn, "⚠️"},
		{DebugLevelInfo, "ℹ️"},
		{DebugLevelDebug, "🔍"},
		{DebugLevelTrace, "📍"},
	}

	for _, tt := range tests {
		result := levelToEmoji(tt.level)
		if result != tt.expected {
			t.Errorf("levelToEmoji(%d) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{`hello "world"`, `hello \"world\"`},
		{"line1\nline2", "line1\\nline2"},
		{"tab\there", "tab\\there"},
	}

	for _, tt := range tests {
		result := escapeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDebugLevelConstants(t *testing.T) {
	levels := []DebugLevel{
		DebugLevelNone,
		DebugLevelError,
		DebugLevelWarn,
		DebugLevelInfo,
		DebugLevelDebug,
		DebugLevelTrace,
	}

	// Verify ordering
	for i := 1; i < len(levels); i++ {
		if levels[i] <= levels[i-1] {
			t.Errorf("level ordering incorrect: %d should be > %d", levels[i], levels[i-1])
		}
	}
}

func TestDebugCategoryConstants(t *testing.T) {
	categories := []DebugCategory{
		DebugCategoryPlanning,
		DebugCategoryExecution,
		DebugCategoryVerification,
		DebugCategoryRetry,
		DebugCategoryState,
		DebugCategoryContext,
		DebugCategoryAll,
	}

	for _, c := range categories {
		if c == "" {
			t.Error("category should not be empty")
		}
	}
}

func TestMaxEventsLimit(t *testing.T) {
	config := DefaultDebugConfig()
	config.Enabled = true
	config.MaxEvents = 100
	dm := NewDebugMode(config)

	dm.StartSession("test")

	// Log more events than the max
	for i := 0; i < 200; i++ {
		dm.Log(DebugLevelInfo, DebugCategoryExecution, "event", nil)
	}

	events := dm.GetEvents()
	// Should be trimmed
	if len(events) > config.MaxEvents {
		t.Errorf("expected events to be trimmed to %d, got %d", config.MaxEvents, len(events))
	}
}

func TestTask17FullIntegration(t *testing.T) {
	// Create debug mode with full config
	config := NewDebugModeBuilder().
		WithEnabled(true).
		WithLevel(DebugLevelDebug).
		WithCategories(DebugCategoryAll).
		WithOutputFormat("text").
		WithSlowThreshold(1 * time.Second).
		BuildConfig()

	dm := NewDebugMode(config)

	// Start session
	session := dm.StartSession("Implement user authentication feature")

	if session == nil {
		t.Fatal("expected session, got nil")
	}

	// Log planning phase
	dm.LogPlanning("Creating plan for authentication", map[string]any{"steps": 5})

	// Log execution phases
	dm.LogExecution("step-1", "Read auth module", 50*time.Millisecond, nil)
	dm.LogExecution("step-2", "Write auth handler", 150*time.Millisecond, nil)
	dm.LogExecution("step-3", "Slow database migration", 2*time.Second, nil)

	// Log verifications
	dm.LogVerification("step-1", true, map[string]any{"files_read": 3})
	dm.LogVerification("step-2", true, map[string]any{"files_written": 2})
	dm.LogVerification("step-3", false, map[string]any{"error": "migration failed"})

	// Log retry
	dm.LogRetry(1, "Migration timeout", true)
	dm.LogExecution("step-3", "Retry database migration", 500*time.Millisecond, nil)
	dm.LogVerification("step-3", true, map[string]any{"success": true})

	// Log state changes
	dm.LogState("planning", "executing", nil)
	dm.LogState("executing", "verifying", nil)
	dm.LogState("verifying", "completed", nil)

	// End session
	dm.EndSession("completed")

	// Verify statistics
	events := dm.GetEvents()
	errors := dm.GetErrors()
	warnings := dm.GetWarnings()
	slowOps := dm.GetSlowOperations()

	if len(events) == 0 {
		t.Error("expected events to be logged")
	}

	if len(warnings) == 0 {
		t.Error("expected warnings for slow operation and failed verification")
	}

	if len(slowOps) == 0 {
		t.Error("expected slow operation to be detected")
	}

	// Generate report
	report := dm.GenerateReport()
	if !containsDebugStr(report, "Implement user authentication feature") {
		t.Error("report missing task name")
	}

	// Export JSON
	json := dm.ExportJSON()
	if !containsDebugStr(json, `"events"`) {
		t.Error("JSON export missing events")
	}

	t.Logf("✅ Task 17: Agent Debug Mode - Full integration PASSED")
	t.Logf("Events: %d, Errors: %d, Warnings: %d, Slow: %d", len(events), len(errors), len(warnings), len(slowOps))
}
