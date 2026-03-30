// Package autonomous - Task 13: Progress Dashboard tests
package autonomous

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewProgressDashboard(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test Task",
	})

	if dashboard == nil {
		t.Fatal("expected non-nil dashboard")
	}
	if dashboard.taskName != "Test Task" {
		t.Errorf("expected task name 'Test Task', got %s", dashboard.taskName)
	}
	if dashboard.status != DashboardStatusRunning {
		t.Errorf("expected initial status 'running', got %s", dashboard.status)
	}
}

func TestDashboardConfigDefaults(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test",
	})

	if dashboard.output == nil {
		t.Error("expected default output to be set")
	}
	if dashboard.refreshRate != 500*time.Millisecond {
		t.Errorf("expected default refresh rate 500ms, got %v", dashboard.refreshRate)
	}
}

func TestDashboardAddStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	step := dashboard.AddStep("step1", "First Step", "Description")

	if step == nil {
		t.Fatal("expected non-nil step")
	}
	if step.ID != "step1" {
		t.Errorf("expected step ID 'step1', got %s", step.ID)
	}
	if step.Name != "First Step" {
		t.Errorf("expected step name 'First Step', got %s", step.Name)
	}
	if step.Status != StepPending {
		t.Errorf("expected initial status 'pending', got %s", step.Status)
	}
	if dashboard.totalOperations != 1 {
		t.Errorf("expected total operations 1, got %d", dashboard.totalOperations)
	}
}

func TestStartStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First Step", "Description")

	dashboard.StartStep("step1")

	// Find step and check status
	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Status != StepRunning {
				t.Errorf("expected status 'running', got %s", step.Status)
			}
			if dashboard.currentStep != step {
				t.Error("expected step to be current step")
			}
			return
		}
	}
	t.Error("step not found")
}

func TestCompleteStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First Step", "Description")
	dashboard.StartStep("step1")

	dashboard.CompleteStep("step1")

	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Status != StepCompleted {
				t.Errorf("expected status 'completed', got %s", step.Status)
			}
			if step.Progress != 1.0 {
				t.Errorf("expected progress 1.0, got %f", step.Progress)
			}
			if dashboard.completedOps != 1 {
				t.Errorf("expected completed ops 1, got %d", dashboard.completedOps)
			}
			return
		}
	}
	t.Error("step not found")
}

func TestFailStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First Step", "Description")
	dashboard.StartStep("step1")

	dashboard.FailStep("step1", "Something went wrong")

	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Status != StepFailed {
				t.Errorf("expected status 'failed', got %s", step.Status)
			}
			if step.Error != "Something went wrong" {
				t.Errorf("expected error message, got %s", step.Error)
			}
			if dashboard.failedOps != 1 {
				t.Errorf("expected failed ops 1, got %d", dashboard.failedOps)
			}
			return
		}
	}
	t.Error("step not found")
}

func TestRetryStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First Step", "Description")

	dashboard.RetryStep("step1")

	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Status != StepRetrying {
				t.Errorf("expected status 'retrying', got %s", step.Status)
			}
			if dashboard.retries != 1 {
				t.Errorf("expected retries 1, got %d", dashboard.retries)
			}
			return
		}
	}
	t.Error("step not found")
}

func TestSkipStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First Step", "Description")

	dashboard.SkipStep("step1")

	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Status != StepSkipped {
				t.Errorf("expected status 'skipped', got %s", step.Status)
			}
			return
		}
	}
	t.Error("step not found")
}

func TestAddSubStep(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "Parent Step", "Description")

	subStep := dashboard.AddSubStep("step1", "sub1", "Sub Step")

	if subStep == nil {
		t.Fatal("expected non-nil sub-step")
	}
	if subStep.ID != "sub1" {
		t.Errorf("expected sub-step ID 'sub1', got %s", subStep.ID)
	}

	// Verify it's in parent's sub-steps
	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if len(step.SubSteps) != 1 {
				t.Errorf("expected 1 sub-step, got %d", len(step.SubSteps))
			}
			return
		}
	}
	t.Error("parent step not found")
}

func TestDashboardSetProgress(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.SetProgress(0.5)

	if dashboard.progress != 0.5 {
		t.Errorf("expected progress 0.5, got %f", dashboard.progress)
	}
}

func TestSetStatus(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.SetStatus(DashboardStatusPaused)

	if dashboard.status != DashboardStatusPaused {
		t.Errorf("expected status 'paused', got %s", dashboard.status)
	}
}

func TestSetTokens(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.SetTokens(1000)

	if dashboard.tokensUsed != 1000 {
		t.Errorf("expected tokens 1000, got %d", dashboard.tokensUsed)
	}
}

func TestSetCost(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.SetCost(5.50)

	if dashboard.cost != 5.50 {
		t.Errorf("expected cost 5.50, got %f", dashboard.cost)
	}
}

func TestIncrementRetries(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.IncrementRetries()
	dashboard.IncrementRetries()

	if dashboard.retries != 2 {
		t.Errorf("expected retries 2, got %d", dashboard.retries)
	}
}

func TestRender(t *testing.T) {
	var buf bytes.Buffer
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test Task",
		Output:   &buf,
		Compact:  true,
	})

	dashboard.AddStep("step1", "First Step", "Description")
	dashboard.StartStep("step1")
	dashboard.CompleteStep("step1")

	dashboard.Render()

	output := buf.String()
	if !strings.Contains(output, "Test Task") {
		t.Error("expected output to contain task name")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected output to contain status")
	}
}

func TestDashboardGetStats(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test Task"})

	dashboard.AddStep("step1", "First", "")
	dashboard.AddStep("step2", "Second", "")
	dashboard.StartStep("step1")
	dashboard.CompleteStep("step1")
	dashboard.SetTokens(500)
	dashboard.SetCost(1.5)

	stats := dashboard.GetStats()

	if stats.TaskName != "Test Task" {
		t.Errorf("expected task name 'Test Task', got %s", stats.TaskName)
	}
	if stats.TotalOps != 2 {
		t.Errorf("expected total ops 2, got %d", stats.TotalOps)
	}
	if stats.CompletedOps != 1 {
		t.Errorf("expected completed ops 1, got %d", stats.CompletedOps)
	}
	if stats.TokensUsed != 500 {
		t.Errorf("expected tokens 500, got %d", stats.TokensUsed)
	}
	if stats.Cost != 1.5 {
		t.Errorf("expected cost 1.5, got %f", stats.Cost)
	}
}

func TestDashboardExportJSON(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test Task"})

	dashboard.AddStep("step1", "First", "")
	dashboard.CompleteStep("step1")

	json := dashboard.ExportJSON()

	if !strings.Contains(json, `"task_name": "Test Task"`) {
		t.Error("expected JSON to contain task name")
	}
	if !strings.Contains(json, `"status": "running"`) {
		t.Error("expected JSON to contain status")
	}
	if !strings.Contains(json, `"steps":`) {
		t.Error("expected JSON to contain steps")
	}
}

func TestGetStatusIcon(t *testing.T) {
	dashboard := &ProgressDashboard{}

	tests := []struct {
		status   DashboardStatus
		expected string
	}{
		{DashboardStatusRunning, "🔄"},
		{DashboardStatusPaused, "⏸️"},
		{DashboardStatusCompleted, "✅"},
		{DashboardStatusFailed, "❌"},
		{DashboardStatusCancelled, "🚫"},
	}

	for _, tt := range tests {
		dashboard.status = tt.status
		icon := dashboard.getStatusIcon()
		if icon != tt.expected {
			t.Errorf("status %s: expected icon %s, got %s", tt.status, tt.expected, icon)
		}
	}
}

func TestGetStepIcon(t *testing.T) {
	dashboard := &ProgressDashboard{}

	tests := []struct {
		status   StepStatus
		expected string
	}{
		{StepPending, "⏳"},
		{StepRunning, "🔄"},
		{StepCompleted, "✅"},
		{StepFailed, "❌"},
		{StepSkipped, "⏭️"},
		{StepRetrying, "🔁"},
	}

	for _, tt := range tests {
		icon := dashboard.getStepIcon(tt.status)
		if icon != tt.expected {
			t.Errorf("step status %s: expected icon %s, got %s", tt.status, tt.expected, icon)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"ab", 2, "ab"},
		{"a", 0, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		wantStr  string
	}{
		{0, "-"},
		{100 * time.Millisecond, "100ms"},
		{1500 * time.Millisecond, "1.5s"},
		{65 * time.Second, "1m 5s"},
		{3665 * time.Second, "1h 1m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if !strings.Contains(result, tt.wantStr) && result != tt.wantStr {
			t.Errorf("formatDuration(%v) = %q, want containing %q", tt.duration, result, tt.wantStr)
		}
	}
}

func TestCountCompletedSteps(t *testing.T) {
	steps := []*ProgressStep{
		{ID: "1", Status: StepCompleted, SubSteps: []*ProgressStep{
			{ID: "1.1", Status: StepCompleted},
			{ID: "1.2", Status: StepRunning},
		}},
		{ID: "2", Status: StepPending},
		{ID: "3", Status: StepCompleted},
	}

	count := countCompletedSteps(steps)

	if count != 3 {
		t.Errorf("expected 3 completed steps, got %d", count)
	}
}

func TestComplete(t *testing.T) {
	var buf bytes.Buffer
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test",
		Output:   &buf,
		Compact:  true,
	})

	dashboard.Complete()

	if dashboard.status != DashboardStatusCompleted {
		t.Errorf("expected status 'completed', got %s", dashboard.status)
	}
}

func TestFail(t *testing.T) {
	var buf bytes.Buffer
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test",
		Output:   &buf,
		Compact:  true,
	})

	dashboard.Fail("Test error")

	if dashboard.status != DashboardStatusFailed {
		t.Errorf("expected status 'failed', got %s", dashboard.status)
	}
}

func TestCancel(t *testing.T) {
	var buf bytes.Buffer
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Test",
		Output:   &buf,
		Compact:  true,
	})

	dashboard.Cancel()

	if dashboard.status != DashboardStatusCancelled {
		t.Errorf("expected status 'cancelled', got %s", dashboard.status)
	}
}

func TestStartStop(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName:    "Test",
		RefreshRate: 100 * time.Millisecond,
	})

	dashboard.Start()

	if !dashboard.running {
		t.Error("expected dashboard to be running")
	}

	// Let it run briefly
	time.Sleep(150 * time.Millisecond)

	dashboard.Stop()

	if dashboard.running {
		t.Error("expected dashboard to be stopped")
	}
}

func TestUpdateStepProgress(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})
	dashboard.AddStep("step1", "First", "")

	dashboard.UpdateStepProgress("step1", 0.75)

	for _, step := range dashboard.steps {
		if step.ID == "step1" {
			if step.Progress != 0.75 {
				t.Errorf("expected progress 0.75, got %f", step.Progress)
			}
			return
		}
	}
	t.Error("step not found")
}

func TestDashboardStatusTypes(t *testing.T) {
	statuses := []DashboardStatus{
		DashboardStatusRunning,
		DashboardStatusPaused,
		DashboardStatusCompleted,
		DashboardStatusFailed,
		DashboardStatusCancelled,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("status should not be empty")
		}
	}
}

func TestStepStatusTypes(t *testing.T) {
	statuses := []StepStatus{
		StepPending,
		StepRunning,
		StepCompleted,
		StepFailed,
		StepSkipped,
		StepRetrying,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("step status should not be empty")
		}
	}
}

func TestProgressStepFields(t *testing.T) {
	step := &ProgressStep{
		ID:          "test-step",
		Name:        "Test Step",
		Description: "A test step",
		Status:      StepRunning,
		Progress:    0.5,
		Error:       "",
		Metadata:    map[string]any{"key": "value"},
	}

	if step.ID != "test-step" {
		t.Errorf("expected ID 'test-step', got %s", step.ID)
	}
	if step.Metadata["key"] != "value" {
		t.Error("expected metadata to contain key")
	}
}

func TestDashboardStatsFields(t *testing.T) {
	stats := DashboardStats{
		TaskName:       "Test",
		Duration:       time.Minute,
		Status:         DashboardStatusRunning,
		Progress:       0.5,
		TotalOps:       10,
		CompletedOps:   5,
		FailedOps:      1,
		Retries:        2,
		TokensUsed:     1000,
		Cost:           5.0,
		StepsCompleted: 5,
		StepsTotal:     10,
	}

	if stats.TaskName != "Test" {
		t.Errorf("expected task name 'Test', got %s", stats.TaskName)
	}
	if stats.Progress != 0.5 {
		t.Errorf("expected progress 0.5, got %f", stats.Progress)
	}
}

func TestMultipleSteps(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.AddStep("s1", "Step 1", "")
	dashboard.AddStep("s2", "Step 2", "")
	dashboard.AddStep("s3", "Step 3", "")

	dashboard.StartStep("s1")
	dashboard.CompleteStep("s1")
	dashboard.StartStep("s2")
	dashboard.FailStep("s2", "Failed")
	dashboard.SkipStep("s3")

	stats := dashboard.GetStats()

	if stats.TotalOps != 3 {
		t.Errorf("expected 3 total ops, got %d", stats.TotalOps)
	}
	if stats.CompletedOps != 1 {
		t.Errorf("expected 1 completed op, got %d", stats.CompletedOps)
	}
	if stats.FailedOps != 1 {
		t.Errorf("expected 1 failed op, got %d", stats.FailedOps)
	}
}

func TestProgressBarCalculation(t *testing.T) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Test"})

	dashboard.AddStep("s1", "", "")
	dashboard.AddStep("s2", "", "")
	dashboard.AddStep("s3", "", "")
	dashboard.AddStep("s4", "", "")

	dashboard.StartStep("s1")
	dashboard.CompleteStep("s1")
	dashboard.StartStep("s2")
	dashboard.CompleteStep("s2")

	// Progress should be 2/4 = 50%
	stats := dashboard.GetStats()
	if stats.StepsCompleted != 2 {
		t.Errorf("expected 2 steps completed, got %d", stats.StepsCompleted)
	}
}

// Benchmark tests
func BenchmarkRender(b *testing.B) {
	var buf bytes.Buffer
	dashboard := NewProgressDashboard(DashboardConfig{
		TaskName: "Benchmark",
		Output:   &buf,
		Compact:  true,
	})

	dashboard.AddStep("s1", "Step 1", "")
	dashboard.AddStep("s2", "Step 2", "")
	dashboard.CompleteStep("s1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		dashboard.Render()
	}
}

func BenchmarkAddStep(b *testing.B) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Benchmark"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dashboard.AddStep(string(rune(i)), "Step", "")
	}
}

func BenchmarkGetStats(b *testing.B) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Benchmark"})
	dashboard.AddStep("s1", "Step 1", "")
	dashboard.AddStep("s2", "Step 2", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dashboard.GetStats()
	}
}

func BenchmarkExportJSON(b *testing.B) {
	dashboard := NewProgressDashboard(DashboardConfig{TaskName: "Benchmark"})
	dashboard.AddStep("s1", "Step 1", "")
	dashboard.AddStep("s2", "Step 2", "")
	dashboard.CompleteStep("s1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dashboard.ExportJSON()
	}
}
