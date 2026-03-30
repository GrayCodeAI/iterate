// Package autonomous - Task 29: Tests for Audit Log
package autonomous

import (
	"errors"
	"testing"
	"time"
)

func TestAuditEventType_Constants(t *testing.T) {
	if AuditEventCommand != "command" {
		t.Error("AuditEventCommand should be 'command'")
	}
	if AuditEventFileRead != "file_read" {
		t.Error("AuditEventFileRead should be 'file_read'")
	}
	if AuditEventFileWrite != "file_write" {
		t.Error("AuditEventFileWrite should be 'file_write'")
	}
	if AuditEventApproval != "approval" {
		t.Error("AuditEventApproval should be 'approval'")
	}
	if AuditEventSecurity != "security" {
		t.Error("AuditEventSecurity should be 'security'")
	}
}

func TestAuditSeverity_Constants(t *testing.T) {
	if AuditSeverityInfo != "info" {
		t.Error("AuditSeverityInfo should be 'info'")
	}
	if AuditSeverityWarning != "warning" {
		t.Error("AuditSeverityWarning should be 'warning'")
	}
	if AuditSeverityError != "error" {
		t.Error("AuditSeverityError should be 'error'")
	}
	if AuditSeverityCritical != "critical" {
		t.Error("AuditSeverityCritical should be 'critical'")
	}
}

func TestDefaultAuditConfig(t *testing.T) {
	config := DefaultAuditConfig()

	if !config.Enabled {
		t.Error("Default config should be enabled")
	}
	if config.MaxEvents != 10000 {
		t.Errorf("Expected 10000 max events, got: %d", config.MaxEvents)
	}
	if config.SeverityFilter != AuditSeverityInfo {
		t.Error("Default severity filter should be info")
	}
}

func TestNewAuditLogger(t *testing.T) {
	config := DefaultAuditConfig()
	logger := NewAuditLogger(config)

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if logger.events == nil {
		t.Error("Events slice should be initialized")
	}
}

func TestAuditLogger_Log(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test command",
		WithOperation("test"),
		WithTarget("test.txt"),
		WithResult("success"),
	)

	stats := logger.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("Expected 1 event, got: %d", stats.TotalEvents)
	}
}

func TestAuditLogger_LogCommand(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.LogCommand("ls", []string{"-la", "/home"}, "success", 100*time.Millisecond, nil)

	events := logger.GetRecentEvents(1)
	if len(events) != 1 {
		t.Fatal("Expected 1 event")
	}

	if events[0].Type != AuditEventCommand {
		t.Error("Event type should be command")
	}
	if events[0].Duration != 100*time.Millisecond {
		t.Error("Duration should be set")
	}
}

func TestAuditLogger_LogCommand_WithError(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	testErr := errors.New("command failed")
	logger.LogCommand("rm", []string{"-rf", "/important"}, "error", 0, testErr)

	events := logger.GetRecentEvents(1)
	if events[0].Severity != AuditSeverityError {
		t.Error("Failed command should have error severity")
	}
	if events[0].Error == "" {
		t.Error("Error message should be set")
	}
}

func TestAuditLogger_LogFileOperation(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	// Read operation
	logger.LogFileOperation("read", "/etc/passwd", "success", nil)
	events := logger.GetRecentEvents(1)
	if events[0].Type != AuditEventFileRead {
		t.Error("Read operation should have file_read type")
	}

	// Write operation
	logger.LogFileOperation("write", "/tmp/test.txt", "success", nil)
	events = logger.GetRecentEvents(1)
	if events[0].Type != AuditEventFileWrite {
		t.Error("Write operation should have file_write type")
	}

	// Delete operation
	logger.LogFileOperation("delete", "/tmp/old.txt", "success", nil)
	events = logger.GetRecentEvents(1)
	if events[0].Type != AuditEventFileDelete {
		t.Error("Delete operation should have file_delete type")
	}
	if events[0].Severity != AuditSeverityWarning {
		t.Error("Delete operation should have warning severity")
	}
}

func TestAuditLogger_LogApproval(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	// Approved
	logger.LogApproval("req-123", "rm -rf /test", true, "admin", "")
	events := logger.GetRecentEvents(1)
	if events[0].Severity != AuditSeverityInfo {
		t.Error("Approved should have info severity")
	}
	if *events[0].Approved != true {
		t.Error("Approved should be true")
	}

	// Denied
	logger.LogApproval("req-124", "rm -rf /", false, "admin", "Too risky")
	events = logger.GetRecentEvents(1)
	if events[0].Severity != AuditSeverityWarning {
		t.Error("Denied should have warning severity")
	}
	if *events[0].Approved != false {
		t.Error("Approved should be false")
	}
}

func TestAuditLogger_LogSecurity(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.LogSecurity("blocked_path_access", "Attempted to access /etc/shadow", AuditSeverityWarning)

	events := logger.GetRecentEvents(1)
	if events[0].Type != AuditEventSecurity {
		t.Error("Event type should be security")
	}
	if events[0].Severity != AuditSeverityWarning {
		t.Error("Severity should be warning")
	}
}

func TestAuditLogger_LogError(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	testErr := errors.New("something went wrong")
	logger.LogError("test_operation", "Test error occurred", testErr)

	events := logger.GetRecentEvents(1)
	if events[0].Type != AuditEventError {
		t.Error("Event type should be error")
	}
	if events[0].Severity != AuditSeverityError {
		t.Error("Severity should be error")
	}
}

func TestAuditLogger_LogSnapshot(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	// Create snapshot
	logger.LogSnapshot("create", "snap-123", nil)
	events := logger.GetRecentEvents(1)
	if events[0].Type != AuditEventSnapshot {
		t.Error("Event type should be snapshot")
	}

	// Failed restore
	logger.LogSnapshot("restore", "snap-123", errors.New("restore failed"))
	events = logger.GetRecentEvents(1)
	if events[0].Severity != AuditSeverityError {
		t.Error("Failed restore should have error severity")
	}
}

func TestAuditLogger_Disabled(t *testing.T) {
	config := DefaultAuditConfig()
	config.Enabled = false
	logger := NewAuditLogger(config)

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test")

	stats := logger.GetStats()
	if stats.TotalEvents != 0 {
		t.Error("Disabled logger should not record events")
	}
}

func TestAuditLogger_SeverityFilter(t *testing.T) {
	config := DefaultAuditConfig()
	config.SeverityFilter = AuditSeverityWarning
	logger := NewAuditLogger(config)

	// Info should be filtered
	logger.Log(AuditEventCommand, AuditSeverityInfo, "Info event")

	// Warning and above should be logged
	logger.Log(AuditEventCommand, AuditSeverityWarning, "Warning event")
	logger.Log(AuditEventCommand, AuditSeverityError, "Error event")

	stats := logger.GetStats()
	if stats.TotalEvents != 2 {
		t.Errorf("Expected 2 events (warning + error), got: %d", stats.TotalEvents)
	}
}

func TestAuditLogger_EventTypeFilter(t *testing.T) {
	config := DefaultAuditConfig()
	config.EventTypes = []AuditEventType{AuditEventCommand, AuditEventSecurity}
	logger := NewAuditLogger(config)

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Command event")
	logger.Log(AuditEventFileRead, AuditSeverityInfo, "File read event")
	logger.Log(AuditEventSecurity, AuditSeverityInfo, "Security event")

	stats := logger.GetStats()
	if stats.TotalEvents != 2 {
		t.Errorf("Expected 2 events (command + security), got: %d", stats.TotalEvents)
	}
}

func TestAuditLogger_MaxEvents(t *testing.T) {
	config := DefaultAuditConfig()
	config.MaxEvents = 5
	logger := NewAuditLogger(config)

	// Add 10 events
	for i := 0; i < 10; i++ {
		logger.Log(AuditEventCommand, AuditSeverityInfo, "Event")
	}

	events := logger.GetRecentEvents(100)
	if len(events) != 5 {
		t.Errorf("Expected 5 events (max), got: %d", len(events))
	}
}

func TestAuditLogger_SetSessionID(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())
	logger.SetSessionID("session-123")

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test")

	events := logger.GetRecentEvents(1)
	if events[0].SessionID != "session-123" {
		t.Error("Session ID should be set")
	}
}

func TestAuditLogger_GetRecentEvents(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	for i := 0; i < 10; i++ {
		logger.Log(AuditEventCommand, AuditSeverityInfo, "Event")
	}

	recent := logger.GetRecentEvents(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 recent events, got: %d", len(recent))
	}
}

func TestAuditLogger_GetEvents_WithFilter(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.LogCommand("ls", []string{}, "success", 0, nil)
	logger.LogFileOperation("write", "/test.txt", "success", nil)
	logger.LogCommand("rm", []string{}, "success", 0, nil)

	filter := AuditFilter{Types: []AuditEventType{AuditEventCommand}}
	events := logger.GetEvents(filter)

	if len(events) != 2 {
		t.Errorf("Expected 2 command events, got: %d", len(events))
	}
}

func TestAuditLogger_Export(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())
	logger.SetSessionID("test-session")

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test event")

	data, err := logger.Export()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Export should produce data")
	}
}

func TestAuditLogger_Clear(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test")
	logger.Clear()

	stats := logger.GetStats()
	if stats.TotalEvents != 0 {
		t.Error("Clear should remove all events")
	}
}

func TestAuditLogger_GetStats(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test 1", WithResult("success"))
	logger.Log(AuditEventFileWrite, AuditSeverityWarning, "Test 2", WithResult("success"))
	logger.LogError("op", "Test 3", errors.New("err"))

	stats := logger.GetStats()

	if stats.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got: %d", stats.TotalEvents)
	}
	if stats.ErrorsLogged != 1 {
		t.Errorf("Expected 1 error logged, got: %d", stats.ErrorsLogged)
	}
	if stats.EventsByType["command"] != 1 {
		t.Error("Should have 1 command event")
	}
	if stats.EventsByResult["success"] != 2 {
		t.Errorf("Should have 2 success results, got: %d", stats.EventsByResult["success"])
	}
}

func TestAuditFilter_Matches(t *testing.T) {
	now := time.Now()
	events := []AuditEvent{
		{Type: AuditEventCommand, Severity: AuditSeverityInfo, SessionID: "s1", Result: "success", Timestamp: now},
		{Type: AuditEventFileRead, Severity: AuditSeverityWarning, SessionID: "s2", Result: "error", Timestamp: now.Add(time.Hour)},
	}

	// Filter by type
	filter := AuditFilter{Types: []AuditEventType{AuditEventCommand}}
	if !filter.Matches(events[0]) || filter.Matches(events[1]) {
		t.Error("Type filter not working")
	}

	// Filter by severity
	filter = AuditFilter{Severity: AuditSeverityWarning}
	if filter.Matches(events[0]) || !filter.Matches(events[1]) {
		t.Error("Severity filter not working")
	}

	// Filter by session
	filter = AuditFilter{SessionID: "s1"}
	if !filter.Matches(events[0]) || filter.Matches(events[1]) {
		t.Error("Session filter not working")
	}

	// Filter by result
	filter = AuditFilter{Result: "error"}
	if filter.Matches(events[0]) || !filter.Matches(events[1]) {
		t.Error("Result filter not working")
	}

	// Filter by time range
	endTime := now.Add(30 * time.Minute)
	filter = AuditFilter{EndTime: &endTime}
	if !filter.Matches(events[0]) || filter.Matches(events[1]) {
		t.Error("Time filter not working")
	}
}

func TestAuditOptions(t *testing.T) {
	logger := NewAuditLogger(DefaultAuditConfig())

	approved := true
	logger.Log(AuditEventCommand, AuditSeverityInfo, "Test",
		WithStepID("step-1"),
		WithOperation("test_op"),
		WithTarget("test_target"),
		WithActor("test_actor"),
		WithResult("test_result"),
		WithError(errors.New("test error")),
		WithDangerLevel(DangerLevelHigh),
		WithApproved(approved),
		WithApprovalID("approval-123"),
		WithDuration(5*time.Second),
		WithMetadata("key", "value"),
	)

	events := logger.GetRecentEvents(1)
	event := events[0]

	if event.StepID != "step-1" {
		t.Error("StepID not set")
	}
	if event.Operation != "test_op" {
		t.Error("Operation not set")
	}
	if event.Target != "test_target" {
		t.Error("Target not set")
	}
	if event.Actor != "test_actor" {
		t.Error("Actor not set")
	}
	if event.Result != "test_result" {
		t.Error("Result not set")
	}
	if event.Error == "" {
		t.Error("Error not set")
	}
	if event.DangerLevel != "high" {
		t.Error("DangerLevel not set")
	}
	if *event.Approved != true {
		t.Error("Approved not set")
	}
	if event.ApprovalID != "approval-123" {
		t.Error("ApprovalID not set")
	}
	if event.Duration != 5*time.Second {
		t.Error("Duration not set")
	}
	if event.Metadata["key"] != "value" {
		t.Error("Metadata not set")
	}
}

func TestTask29AuditLog(t *testing.T) {
	// Comprehensive test for Task 29

	// Test 1: Create logger
	logger := NewAuditLogger(DefaultAuditConfig())
	logger.SetSessionID("test-session-29")

	// Test 2: Log various events
	logger.LogCommand("git", []string{"status"}, "success", 50*time.Millisecond, nil)
	logger.LogFileOperation("write", "/src/main.go", "success", nil)
	logger.LogApproval("req-1", "rm -rf /tmp", true, "user", "")
	logger.LogSecurity("suspicious_file_access", "Attempted access to protected path", AuditSeverityWarning)

	// Test 3: Check stats
	stats := logger.GetStats()
	if stats.TotalEvents != 4 {
		t.Errorf("Expected 4 events, got: %d", stats.TotalEvents)
	}

	// Test 4: Filter events
	filter := AuditFilter{Types: []AuditEventType{AuditEventCommand, AuditEventApproval}}
	filtered := logger.GetEvents(filter)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered events, got: %d", len(filtered))
	}

	// Test 5: Export
	data, err := logger.Export()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Export should produce data")
	}

	// Test 6: Clear
	logger.Clear()
	stats = logger.GetStats()
	if stats.TotalEvents != 0 {
		t.Error("Clear should remove all events")
	}
}
