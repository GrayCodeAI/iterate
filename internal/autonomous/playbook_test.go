// Package autonomous - Task 16: Agent Playbooks tests
package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestNewPlaybookRegistry(t *testing.T) {
	registry := NewPlaybookRegistry()

	if registry == nil {
		t.Fatal("expected registry, got nil")
	}

	playbooks := registry.List()
	if len(playbooks) == 0 {
		t.Error("expected default playbooks to be registered")
	}
}

func TestRegisterPlaybook(t *testing.T) {
	registry := NewPlaybookRegistry()

	playbook := &Playbook{
		ID:          "test-playbook",
		Name:        "Test Playbook",
		Type:        PlaybookTypeCustom,
		Description: "A test playbook",
		Version:     "1.0.0",
		Author:      "test",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Enabled:     true,
	}

	err := registry.Register(playbook)
	if err != nil {
		t.Fatalf("failed to register playbook: %v", err)
	}

	retrieved, err := registry.Get("test-playbook")
	if err != nil {
		t.Fatalf("failed to get playbook: %v", err)
	}

	if retrieved.Name != "Test Playbook" {
		t.Errorf("expected name 'Test Playbook', got '%s'", retrieved.Name)
	}
}

func TestRegisterPlaybookEmptyID(t *testing.T) {
	registry := NewPlaybookRegistry()

	playbook := &Playbook{
		Name: "No ID Playbook",
		Type: PlaybookTypeCustom,
	}

	err := registry.Register(playbook)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUnregisterPlaybook(t *testing.T) {
	registry := NewPlaybookRegistry()

	// Register a test playbook
	playbook := &Playbook{
		ID:        "to-remove",
		Name:      "To Remove",
		Type:      PlaybookTypeCustom,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	registry.Register(playbook)

	// Verify it exists
	_, err := registry.Get("to-remove")
	if err != nil {
		t.Fatalf("playbook should exist: %v", err)
	}

	// Unregister
	registry.Unregister("to-remove")

	// Verify it's gone
	_, err = registry.Get("to-remove")
	if err == nil {
		t.Error("expected error after unregister")
	}
}

func TestGetByType(t *testing.T) {
	registry := NewPlaybookRegistry()

	fixBugPlaybooks := registry.GetByType(PlaybookTypeFixBug)
	if len(fixBugPlaybooks) == 0 {
		t.Error("expected at least one fix_bug playbook")
	}

	for _, p := range fixBugPlaybooks {
		if p.Type != PlaybookTypeFixBug {
			t.Errorf("expected type fix_bug, got %s", p.Type)
		}
	}
}

func TestListEnabled(t *testing.T) {
	registry := NewPlaybookRegistry()

	enabled := registry.ListEnabled()
	if len(enabled) == 0 {
		t.Error("expected enabled playbooks")
	}

	for _, p := range enabled {
		if !p.Enabled {
			t.Error("expected all listed playbooks to be enabled")
		}
	}
}

func TestEnableDisable(t *testing.T) {
	registry := NewPlaybookRegistry()

	// Disable
	err := registry.Disable("fix-bug")
	if err != nil {
		t.Fatalf("failed to disable: %v", err)
	}

	p, _ := registry.Get("fix-bug")
	if p.Enabled {
		t.Error("expected playbook to be disabled")
	}

	// Enable
	err = registry.Enable("fix-bug")
	if err != nil {
		t.Fatalf("failed to enable: %v", err)
	}

	p, _ = registry.Get("fix-bug")
	if !p.Enabled {
		t.Error("expected playbook to be enabled")
	}
}

func TestMatchPlaybook(t *testing.T) {
	registry := NewPlaybookRegistry()

	tests := []struct {
		task        string
		expectedID  string
		expectMatch bool
	}{
		{"fix the login bug", "fix-bug", true},
		{"add unit tests for auth", "add-test", true},
		{"refactor the database module", "refactor", true},
		{"implement user authentication feature", "add-feature", true},
		{"optimize the query performance", "optimize", true},
		{"add documentation for API", "document", true},
		{"fix security vulnerability in input validation", "security", true},
		{"random task with no match", "", false},
	}

	for _, tt := range tests {
		match := registry.Match(tt.task)
		if tt.expectMatch {
			if match == nil {
				t.Errorf("task '%s': expected match, got nil", tt.task)
				continue
			}
			if match.ID != tt.expectedID {
				t.Errorf("task '%s': expected '%s', got '%s'", tt.task, tt.expectedID, match.ID)
			}
		} else if match != nil {
			t.Errorf("task '%s': expected no match, got '%s'", tt.task, match.ID)
		}
	}
}

func TestMatchAll(t *testing.T) {
	registry := NewPlaybookRegistry()

	matches := registry.MatchAll("fix the bug and add tests")
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches, got %d", len(matches))
	}

	// Verify all returned playbooks are matches
	for _, m := range matches {
		if m.ID != "fix-bug" && m.ID != "add-test" {
			t.Logf("Note: got playbook %s with priority %d", m.ID, m.Triggers.Priority)
		}
	}
}

func TestPlaybookInstance(t *testing.T) {
	playbook := &Playbook{
		ID:          "test-instance",
		Name:        "Test Instance",
		Type:        PlaybookTypeCustom,
		Description: "Test playbook for instances",
		Version:     "1.0.0",
		Author:      "test",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Steps: []PlaybookStep{
			{ID: "step-1", Order: 1, Type: "read", Action: "Read {{file}}", Target: "{{file}}", Required: true},
			{ID: "step-2", Order: 2, Type: "write", Action: "Write to {{output}}", Target: "{{output}}", Required: true},
		},
		Variables: map[string]string{
			"file": "default.txt",
		},
		Enabled: true,
	}

	variables := map[string]string{
		"file":   "custom.txt",
		"output": "result.txt",
	}

	instance := NewPlaybookInstance(playbook, variables)

	if instance == nil {
		t.Fatal("expected instance, got nil")
	}

	if instance.Status != InstanceStatusPending {
		t.Errorf("expected pending status, got %s", instance.Status)
	}

	// Check variable resolution
	if instance.Variables["file"] != "custom.txt" {
		t.Errorf("expected custom.txt, got %s", instance.Variables["file"])
	}

	// Check steps resolved
	if len(instance.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(instance.Steps))
	}

	if instance.Steps[0].ResolvedAction != "Read custom.txt" {
		t.Errorf("expected 'Read custom.txt', got '%s'", instance.Steps[0].ResolvedAction)
	}

	if instance.Steps[1].ResolvedTarget != "result.txt" {
		t.Errorf("expected 'result.txt', got '%s'", instance.Steps[1].ResolvedTarget)
	}
}

func TestPlaybookInstanceExecution(t *testing.T) {
	playbook := &Playbook{
		ID:        "exec-test",
		Name:      "Execution Test",
		Type:      PlaybookTypeCustom,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Steps: []PlaybookStep{
			{ID: "s1", Order: 1, Type: "read", Action: "Step 1", Target: "file1"},
			{ID: "s2", Order: 2, Type: "write", Action: "Step 2", Target: "file2"},
			{ID: "s3", Order: 3, Type: "test", Action: "Step 3", Target: "file3"},
		},
		Enabled: true,
	}

	instance := NewPlaybookInstance(playbook, nil)

	// Start execution
	instance.Start()
	if instance.Status != InstanceStatusRunning {
		t.Errorf("expected running, got %s", instance.Status)
	}

	// Get current step
	step := instance.GetCurrentStep()
	if step == nil {
		t.Fatal("expected current step")
	}
	if step.Type != "read" {
		t.Errorf("expected read step, got %s", step.Type)
	}

	// Mark completed
	instance.MarkStepCompleted("read output")
	if instance.Steps[0].Status != PlaybookStepStatusCompleted {
		t.Error("expected step 1 completed")
	}

	// Advance
	instance.AdvanceStep()
	if instance.CurrentStep != 1 {
		t.Errorf("expected current step 1, got %d", instance.CurrentStep)
	}

	// Progress check
	progress := instance.GetProgress()
	if progress < 30.0 || progress > 40.0 {
		t.Errorf("expected ~33%% progress, got %.0f%%", progress)
	}

	// Complete remaining steps
	instance.MarkStepCompleted("write output")
	instance.AdvanceStep()
	instance.MarkStepCompleted("test output")
	instance.AdvanceStep()

	if !instance.IsComplete() {
		t.Error("expected instance complete")
	}
}

func TestPlaybookInstanceFailure(t *testing.T) {
	playbook := &Playbook{
		ID:        "fail-test",
		Name:      "Failure Test",
		Type:      PlaybookTypeCustom,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Steps: []PlaybookStep{
			{ID: "s1", Order: 1, Type: "read", Action: "Step 1", Target: "file1"},
		},
		Enabled: true,
	}

	instance := NewPlaybookInstance(playbook, nil)
	instance.Start()

	// Simulate failure
	instance.MarkStepFailed(context.DeadlineExceeded)

	if instance.Status != InstanceStatusFailed {
		t.Errorf("expected failed status, got %s", instance.Status)
	}

	if !instance.IsFailed() {
		t.Error("expected IsFailed to be true")
	}
}

func TestPlaybookInstanceSkip(t *testing.T) {
	playbook := &Playbook{
		ID:        "skip-test",
		Name:      "Skip Test",
		Type:      PlaybookTypeCustom,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Steps: []PlaybookStep{
			{ID: "s1", Order: 1, Type: "read", Action: "Step 1", Target: "file1", Required: false},
			{ID: "s2", Order: 2, Type: "write", Action: "Step 2", Target: "file2", Required: true},
		},
		Enabled: true,
	}

	instance := NewPlaybookInstance(playbook, nil)
	instance.Start()

	// Skip first step
	instance.MarkStepSkipped("optional step not needed")
	if instance.Steps[0].Status != PlaybookStepStatusSkipped {
		t.Error("expected step skipped")
	}

	instance.AdvanceStep()
	instance.MarkStepCompleted("done")
	instance.AdvanceStep()

	// Skipped step should count toward progress
	if instance.GetProgress() != 100.0 {
		t.Errorf("expected 100%% progress with skip, got %.0f%%", instance.GetProgress())
	}
}

func TestPlaybookInstanceSummary(t *testing.T) {
	playbook := &Playbook{
		ID:        "summary-test",
		Name:      "Summary Test",
		Type:      PlaybookTypeCustom,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Steps: []PlaybookStep{
			{ID: "s1", Order: 1, Type: "read", Action: "Read file", Target: "file"},
		},
		Enabled: true,
	}

	instance := NewPlaybookInstance(playbook, nil)
	instance.Start()

	summary := instance.GetSummary()

	if !containsAll(summary, "Summary Test", "running", "Read file") {
		t.Errorf("summary missing expected content: %s", summary)
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !pbContainsSubstring(s, substr) {
			return false
		}
	}
	return true
}

func pbContainsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && pbContainsSubstringHelper(s, substr))
}

func pbContainsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPlaybookBuilder(t *testing.T) {
	playbook := NewPlaybookBuilder("custom-pb", "Custom Playbook", PlaybookTypeCustom).
		WithDescription("A custom playbook for testing").
		WithVersion("2.0.0").
		WithAuthor("builder-test").
		AddStep("read", "Read {{input}}", "{{input}}", true).
		AddStep("write", "Write {{output}}", "{{output}}", true).
		AddSuccessCriteria("Input read successfully").
		AddSuccessCriteria("Output written correctly").
		AddTriggerKeywords("custom", "test").
		AddTriggerPatterns(`custom\s+\w+`).
		SetTriggerPriority(5).
		AddVariable("input", "default.txt").
		AddTag("testing").
		SetEnabled(true).
		Build()

	if playbook.ID != "custom-pb" {
		t.Errorf("expected ID 'custom-pb', got '%s'", playbook.ID)
	}

	if playbook.Name != "Custom Playbook" {
		t.Errorf("expected name 'Custom Playbook', got '%s'", playbook.Name)
	}

	if len(playbook.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(playbook.Steps))
	}

	if len(playbook.SuccessCriteria) != 2 {
		t.Errorf("expected 2 success criteria, got %d", len(playbook.SuccessCriteria))
	}

	if len(playbook.Triggers.Keywords) != 2 {
		t.Errorf("expected 2 trigger keywords, got %d", len(playbook.Triggers.Keywords))
	}

	if playbook.Triggers.Priority != 5 {
		t.Errorf("expected priority 5, got %d", playbook.Triggers.Priority)
	}

	if !playbook.Enabled {
		t.Error("expected playbook enabled")
	}
}

func TestRegistryInstantiatePlaybook(t *testing.T) {
	registry := NewPlaybookRegistry()

	variables := map[string]string{
		"source_file": "main.go",
		"test_file":   "main_test.go",
	}

	instance, err := registry.InstantiatePlaybook(context.Background(), "refactor", variables)
	if err != nil {
		t.Fatalf("failed to instantiate: %v", err)
	}

	if instance == nil {
		t.Fatal("expected instance, got nil")
	}

	if instance.Playbook.ID != "refactor" {
		t.Errorf("expected refactor playbook, got %s", instance.Playbook.ID)
	}

	// Verify variables were resolved
	foundResolved := false
	for _, step := range instance.Steps {
		if step.ResolvedTarget == "main.go" || step.ResolvedTarget == "main_test.go" {
			foundResolved = true
			break
		}
	}
	if !foundResolved {
		t.Error("expected at least one step with resolved variable")
	}
}

func TestRegistryInstantiateNotFound(t *testing.T) {
	registry := NewPlaybookRegistry()

	_, err := registry.InstantiatePlaybook(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent playbook")
	}
}

func TestRegistryGetStats(t *testing.T) {
	registry := NewPlaybookRegistry()

	stats := registry.GetStats()

	if stats == nil {
		t.Fatal("expected stats, got nil")
	}

	total, ok := stats["total_playbooks"].(int)
	if !ok {
		t.Fatal("expected total_playbooks to be int")
	}

	if total == 0 {
		t.Error("expected at least one playbook")
	}

	enabled, ok := stats["enabled_playbooks"].(int)
	if !ok {
		t.Fatal("expected enabled_playbooks to be int")
	}

	if enabled == 0 {
		t.Error("expected at least one enabled playbook")
	}
}

func TestDefaultPlaybooksHaveRequiredFields(t *testing.T) {
	registry := NewPlaybookRegistry()

	playbooks := registry.List()

	for _, p := range playbooks {
		if p.ID == "" {
			t.Error("playbook missing ID")
		}
		if p.Name == "" {
			t.Errorf("playbook %s missing name", p.ID)
		}
		if p.Type == "" {
			t.Errorf("playbook %s missing type", p.ID)
		}
		if p.Version == "" {
			t.Errorf("playbook %s missing version", p.ID)
		}
		if len(p.Steps) == 0 {
			t.Errorf("playbook %s has no steps", p.ID)
		}
		for i, step := range p.Steps {
			if step.Type == "" {
				t.Errorf("playbook %s step %d missing type", p.ID, i)
			}
			if step.Action == "" {
				t.Errorf("playbook %s step %d missing action", p.ID, i)
			}
		}
	}
}

func TestPlaybookTypeConstants(t *testing.T) {
	types := []PlaybookType{
		PlaybookTypeRefactor,
		PlaybookTypeAddTest,
		PlaybookTypeFixBug,
		PlaybookTypeAddFeature,
		PlaybookTypeOptimize,
		PlaybookTypeDocument,
		PlaybookTypeMigrate,
		PlaybookTypeSecurity,
		PlaybookTypeCustom,
	}

	for _, pt := range types {
		if pt == "" {
			t.Error("playbook type should not be empty")
		}
	}
}

func TestInstanceStatusConstants(t *testing.T) {
	statuses := []InstanceStatus{
		InstanceStatusPending,
		InstanceStatusRunning,
		InstanceStatusCompleted,
		InstanceStatusFailed,
		InstanceStatusCancelled,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("instance status should not be empty")
		}
	}
}

func TestStepStatusConstants(t *testing.T) {
	statuses := []PlaybookStepStatus{
		PlaybookStepStatusPending,
		PlaybookStepStatusRunning,
		PlaybookStepStatusCompleted,
		PlaybookStepStatusFailed,
		PlaybookStepStatusSkipped,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("step status should not be empty")
		}
	}
}

func TestResolveVariables(t *testing.T) {
	tests := []struct {
		template  string
		variables map[string]string
		expected  string
	}{
		{
			template:  "Read {{file}}",
			variables: map[string]string{"file": "test.txt"},
			expected:  "Read test.txt",
		},
		{
			template:  "{{a}} and {{b}}",
			variables: map[string]string{"a": "first", "b": "second"},
			expected:  "first and second",
		},
		{
			template:  "No variables here",
			variables: map[string]string{"unused": "value"},
			expected:  "No variables here",
		},
		{
			template:  "Missing {{unknown}}",
			variables: map[string]string{"other": "value"},
			expected:  "Missing {{unknown}}",
		},
	}

	for _, tt := range tests {
		result := resolveVariables(tt.template, tt.variables)
		if result != tt.expected {
			t.Errorf("resolveVariables(%q, %v) = %q, want %q", tt.template, tt.variables, result, tt.expected)
		}
	}
}

func TestTask16FullIntegration(t *testing.T) {
	// Create registry
	registry := NewPlaybookRegistry()

	// Match a task to a playbook
	task := "fix the authentication bug causing login failures"
	playbook := registry.Match(task)

	if playbook == nil {
		t.Fatal("expected to match a playbook for bug fix task")
	}

	if playbook.ID != "fix-bug" {
		t.Errorf("expected fix-bug playbook, got %s", playbook.ID)
	}

	// Instantiate the playbook
	variables := map[string]string{
		"error_source":      "logs/auth.log",
		"affected_files":    "auth/login.go",
		"reproduce_command": "go test ./auth/...",
		"test_target":       "./auth/...",
	}

	instance, err := registry.InstantiatePlaybook(context.Background(), playbook.ID, variables)
	if err != nil {
		t.Fatalf("failed to instantiate playbook: %v", err)
	}

	// Execute the playbook instance
	instance.Start()

	// Simulate step execution
	for i := 0; i < len(instance.Steps); i++ {
		step := instance.GetCurrentStep()
		if step == nil {
			t.Fatalf("unexpected nil step at index %d", i)
		}

		// Simulate execution
		instance.MarkStepCompleted("completed successfully")

		if !instance.AdvanceStep() && i < len(instance.Steps)-1 {
			t.Errorf("advance step returned false prematurely at step %d", i)
		}
	}

	// Verify completion
	if !instance.IsComplete() {
		t.Error("expected playbook instance to be complete")
	}

	if instance.GetProgress() != 100.0 {
		t.Errorf("expected 100%% progress, got %.0f%%", instance.GetProgress())
	}

	// Get summary
	summary := instance.GetSummary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	t.Logf("✅ Task 16: Agent Playbooks - Full integration PASSED")
	t.Logf("Playbook: %s, Steps: %d, Final Status: %s", instance.Playbook.Name, len(instance.Steps), instance.Status)
}
