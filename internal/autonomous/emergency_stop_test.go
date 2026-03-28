// Package autonomous provides emergency stop functionality tests.
// Task 35: Emergency Stop mechanism for runaway agents.

package autonomous

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestNewEmergencyStopManager(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	
	if !mgr.config.Enabled {
		t.Error("expected enabled by default")
	}
	
	if mgr.IsStopped() {
		t.Error("should not be stopped initially")
	}
}

func TestEmergencyStopManager_Trigger(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	// Trigger emergency stop
	err := mgr.Trigger(
		EmergencyStopManual,
		SeverityCritical,
		"User requested stop",
		"user",
		map[string]interface{}{"reason": "test"},
	)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !mgr.IsStopped() {
		t.Error("expected stopped to be true")
	}
	
	trigger := mgr.GetTrigger()
	if trigger == nil {
		t.Fatal("expected trigger to be set")
	}
	
	if trigger.Reason != EmergencyStopManual {
		t.Errorf("expected reason %s, got %s", EmergencyStopManual, trigger.Reason)
	}
	
	if trigger.Severity != SeverityCritical {
		t.Errorf("expected severity %d, got %d", SeverityCritical, trigger.Severity)
	}
	
	// Second trigger should fail
	err = mgr.Trigger(
		EmergencyStopManual,
		SeverityCritical,
		"Another stop",
		"user",
		nil,
	)
	
	if err == nil {
		t.Error("expected error when triggering already stopped manager")
	}
}

func TestEmergencyStopManager_Reset(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	// Trigger and then reset
	mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test", "test", nil)
	
	if !mgr.IsStopped() {
		t.Fatal("expected stopped")
	}
	
	mgr.Reset()
	
	if mgr.IsStopped() {
		t.Error("expected not stopped after reset")
	}
	
	if mgr.GetTrigger() != nil {
		t.Error("expected trigger to be nil after reset")
	}
}

func TestEmergencyStopManager_RecordAction_LoopDetection(t *testing.T) {
	config := EmergencyStopConfig{
		Enabled:                true,
		AutoRecovery:           false,
		LoopDetectionWindow:    5 * time.Second,
		LoopDetectionThreshold: 3,
	}
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Record same action 3 times
	for i := 0; i < 3; i++ {
		mgr.RecordAction("read_file")
	}
	
	// Should trigger loop detection
	if !mgr.IsStopped() {
		t.Error("expected loop detection to trigger stop")
	}
	
	trigger := mgr.GetTrigger()
	if trigger == nil {
		t.Fatal("expected trigger")
	}
	
	if trigger.Reason != EmergencyStopInfiniteLoop {
		t.Errorf("expected infinite_loop reason, got %s", trigger.Reason)
	}
}

func TestEmergencyStopManager_RecordAction_NoLoop(t *testing.T) {
	config := EmergencyStopConfig{
		Enabled:                true,
		LoopDetectionWindow:    5 * time.Second,
		LoopDetectionThreshold: 5,
	}
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Record different actions
	actions := []string{"read_file", "edit_file", "run_test", "read_file", "edit_file"}
	for _, action := range actions {
		mgr.RecordAction(action)
	}
	
	// Should NOT trigger loop detection
	if mgr.IsStopped() {
		t.Error("should not trigger stop with different actions")
	}
}

func TestEmergencyStopManager_RecordError(t *testing.T) {
	config := EmergencyStopConfig{
		Enabled:              true,
		AutoRecovery:         false,
		MaxConsecutiveErrors: 3,
	}
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Record non-critical errors
	for i := 0; i < 2; i++ {
		mgr.RecordError(context.DeadlineExceeded, false)
	}
	
	if mgr.IsStopped() {
		t.Error("should not stop with errors below threshold")
	}
	
	// Third error should trigger stop
	mgr.RecordError(context.DeadlineExceeded, false)
	
	if !mgr.IsStopped() {
		t.Error("expected stop after max consecutive errors")
	}
	
	trigger := mgr.GetTrigger()
	if trigger == nil {
		t.Fatal("expected trigger")
	}
	
	if trigger.Reason != EmergencyStopCriticalError {
		t.Errorf("expected critical_error reason, got %s", trigger.Reason)
	}
}

func TestEmergencyStopManager_RecordError_Critical(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	config.AutoRecovery = false
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Single critical error should trigger immediately
	mgr.RecordError(context.DeadlineExceeded, true)
	
	if !mgr.IsStopped() {
		t.Error("expected stop after critical error")
	}
	
	trigger := mgr.GetTrigger()
	if trigger == nil {
		t.Fatal("expected trigger")
	}
	
	if trigger.Severity != SeverityFatal {
		t.Errorf("expected fatal severity, got %d", trigger.Severity)
	}
}

func TestEmergencyStopManager_ClearErrorCount(t *testing.T) {
	config := EmergencyStopConfig{
		Enabled:              true,
		MaxConsecutiveErrors: 5,
	}
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Record some errors
	for i := 0; i < 3; i++ {
		mgr.RecordError(context.DeadlineExceeded, false)
	}
	
	// Clear count
	mgr.ClearErrorCount()
	
	// Record more errors - should not trigger yet
	for i := 0; i < 3; i++ {
		mgr.RecordError(context.DeadlineExceeded, false)
	}
	
	if mgr.IsStopped() {
		t.Error("should not trigger after clear with only 3 errors")
	}
}

func TestEmergencyStopManager_CheckResourceUsage(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	config.AutoRecovery = false
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	// Normal usage - should not trigger
	mgr.CheckResourceUsage(1024, 50.0, 100) // 1GB, 50% CPU, 100 goroutines
	
	if mgr.IsStopped() {
		t.Error("should not trigger with normal usage")
	}
	
	// High memory - should trigger
	mgr.CheckResourceUsage(5000, 50.0, 100) // 5GB
	
	if !mgr.IsStopped() {
		t.Error("expected stop with high memory usage")
	}
	
	trigger := mgr.GetTrigger()
	if trigger.Reason != EmergencyStopResourceExhaust {
		t.Errorf("expected resource_exhaust reason, got %s", trigger.Reason)
	}
}

func TestEmergencyStopManager_TriggerSafetyViolation(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	config.AutoRecovery = false
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	
	mgr.TriggerSafetyViolation("unauthorized_file_access", "Attempted to modify /etc/passwd")
	
	if !mgr.IsStopped() {
		t.Error("expected stop after safety violation")
	}
	
	trigger := mgr.GetTrigger()
	if trigger == nil {
		t.Fatal("expected trigger")
	}
	
	if trigger.Reason != EmergencyStopSafetyViolation {
		t.Errorf("expected safety_violation reason, got %s", trigger.Reason)
	}
}

func TestEmergencyStopManager_AttemptRecovery(t *testing.T) {
	t.Run("infinite_loop_recovery", func(t *testing.T) {
		config := EmergencyStopConfig{
			Enabled:                true,
			AutoRecovery:           true,
			LoopDetectionThreshold: 3,
			LoopDetectionWindow:    5 * time.Second,
		}
		logger := slog.Default()
		
		mgr := NewEmergencyStopManager(config, logger)
		
		// Trigger via loop detection
		for i := 0; i < 3; i++ {
			mgr.RecordAction("read_file")
		}
		
		if !mgr.IsStopped() {
			t.Fatal("expected stopped")
		}
		
		// Attempt recovery
		ctx := context.Background()
		err := mgr.AttemptRecovery(ctx)
		
		if err != nil {
			t.Fatalf("unexpected recovery error: %v", err)
		}
		
		if mgr.IsStopped() {
			t.Error("expected recovered (not stopped)")
		}
	})
	
	t.Run("critical_error_no_recovery", func(t *testing.T) {
		config := DefaultEmergencyStopConfig()
		config.AutoRecovery = true
		logger := slog.Default()
		
		mgr := NewEmergencyStopManager(config, logger)
		
		// Trigger critical error
		mgr.RecordError(context.DeadlineExceeded, true)
		
		// Recovery should fail for critical errors
		ctx := context.Background()
		err := mgr.AttemptRecovery(ctx)
		
		if err == nil {
			t.Error("expected recovery to fail for critical error")
		}
	})
	
	t.Run("auto_recovery_disabled", func(t *testing.T) {
		config := EmergencyStopConfig{
			Enabled:      true,
			AutoRecovery: false,
		}
		logger := slog.Default()
		
		mgr := NewEmergencyStopManager(config, logger)
		
		// Trigger
		mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test", "test", nil)
		
		// Recovery should fail
		ctx := context.Background()
		err := mgr.AttemptRecovery(ctx)
		
		if err == nil {
			t.Error("expected recovery to fail when auto recovery disabled")
		}
	})
}

func TestEmergencyStopManager_GetStatus(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	// Initial status
	status := mgr.GetStatus()
	
	if enabled, ok := status["enabled"].(bool); !ok || !enabled {
		t.Error("expected enabled=true")
	}
	
	if stopped, ok := status["stopped"].(bool); !ok || stopped {
		t.Error("expected stopped=false")
	}
	
	// After trigger
	mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test status", "test", nil)
	
	status = mgr.GetStatus()
	
	if reason, ok := status["trigger_reason"].(string); !ok || reason != "manual" {
		t.Errorf("expected trigger_reason=manual, got %v", reason)
	}
}

func TestEmergencyStopManager_StopChan(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	stopChan := mgr.StopChan()
	if stopChan == nil {
		t.Fatal("expected non-nil stop channel")
	}
	
	// Trigger in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test", "test", nil)
	}()
	
	// Should receive from channel
	select {
	case <-stopChan:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for stop channel")
	}
}

func TestEmergencyStopManager_AlertChan(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	alertChan := mgr.AlertChan()
	if alertChan == nil {
		t.Fatal("expected non-nil alert channel")
	}
	
	// Trigger
	mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test alert", "test", nil)
	
	// Should receive from channel
	select {
	case trigger := <-alertChan:
		if trigger.Reason != EmergencyStopManual {
			t.Errorf("expected manual reason, got %s", trigger.Reason)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for alert channel")
	}
}

func TestEmergencyStopManager_NotifyCallback(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	var receivedTrigger *EmergencyStopTrigger
	var mu sync.Mutex
	
	mgr.SetNotifyCallback(func(trigger EmergencyStopTrigger) {
		mu.Lock()
		defer mu.Unlock()
		receivedTrigger = &trigger
	})
	
	// Trigger
	mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test notify", "test", nil)
	
	// Wait for callback
	time.Sleep(50 * time.Millisecond)
	
	mu.Lock()
	defer mu.Unlock()
	
	if receivedTrigger == nil {
		t.Fatal("expected callback to receive trigger")
	}
	
	if receivedTrigger.Reason != EmergencyStopManual {
		t.Errorf("expected manual reason, got %s", receivedTrigger.Reason)
	}
}

func TestEmergencyStopManager_WithAuditLogger(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	
	// Create audit logger
	auditLogger := NewAuditLogger(DefaultAuditConfig())
	mgr.SetAuditLogger(auditLogger)
	
	// Trigger
	mgr.Trigger(EmergencyStopManual, SeverityCritical, "Test audit", "test", nil)
	
	// Check audit log
	entries := auditLogger.GetRecentEvents(10)
	
	found := false
	for _, entry := range entries {
		if entry.Type == AuditEventSecurity {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("expected emergency stop to be logged in audit log")
	}
}

func TestEmergencyStopHandler_HandleTrigger(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	config.AutoRecovery = false
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	handler := NewEmergencyStopHandler(mgr, logger)
	
	err := handler.HandleTrigger("manual", "Handler test", "api")
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !mgr.IsStopped() {
		t.Error("expected stopped")
	}
}

func TestEmergencyStopHandler_HandleStatus(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	mgr := NewEmergencyStopManager(config, slog.Default())
	handler := NewEmergencyStopHandler(mgr, slog.Default())
	
	status := handler.HandleStatus()
	
	if _, ok := status["enabled"]; !ok {
		t.Error("expected enabled in status")
	}
}

func TestEmergencyStopHandler_HandleReset(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	config.AutoRecovery = false
	logger := slog.Default()
	
	mgr := NewEmergencyStopManager(config, logger)
	handler := NewEmergencyStopHandler(mgr, logger)
	
	// Trigger then reset
	handler.HandleTrigger("manual", "Test", "test")
	
	if !mgr.IsStopped() {
		t.Fatal("expected stopped")
	}
	
	err := handler.HandleReset()
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if mgr.IsStopped() {
		t.Error("expected not stopped after reset")
	}
}

func TestEmergencyStopReasons(t *testing.T) {
	reasons := []EmergencyStopReason{
		EmergencyStopManual,
		EmergencyStopInfiniteLoop,
		EmergencyStopResourceExhaust,
		EmergencyStopCriticalError,
		EmergencyStopTimeout,
		EmergencyStopUserAbort,
		EmergencyStopSafetyViolation,
		EmergencyStopExternalSignal,
	}
	
	for _, reason := range reasons {
		if string(reason) == "" {
			t.Errorf("reason should not be empty string")
		}
	}
}

func TestEmergencyStopSeverity(t *testing.T) {
	severities := []EmergencyStopSeverity{
		SeverityWarning,
		SeverityCritical,
		SeverityFatal,
	}
	
	for i, sev := range severities {
		if int(sev) != i {
			t.Errorf("expected severity index %d, got %d", i, sev)
		}
	}
}

func TestDefaultEmergencyStopConfig(t *testing.T) {
	config := DefaultEmergencyStopConfig()
	
	if !config.Enabled {
		t.Error("expected enabled by default")
	}
	
	if !config.AutoRecovery {
		t.Error("expected auto recovery by default")
	}
	
	if config.MaxConsecutiveErrors != 5 {
		t.Errorf("expected 5 max errors, got %d", config.MaxConsecutiveErrors)
	}
	
	if config.LoopDetectionThreshold != 5 {
		t.Errorf("expected 5 loop threshold, got %d", config.LoopDetectionThreshold)
	}
	
	if config.GracefulTimeout != 5*time.Second {
		t.Errorf("expected 5s graceful timeout, got %v", config.GracefulTimeout)
	}
}
