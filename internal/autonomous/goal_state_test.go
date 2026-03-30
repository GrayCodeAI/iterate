// Package autonomous - Task 10: Goal State tracking tests
package autonomous

import (
	"testing"
)

func TestDefaultGoalTrackerConfig(t *testing.T) {
	config := DefaultGoalTrackerConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
}

func TestNewGoalTracker(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	if gt == nil {
		t.Fatal("Expected non-nil goal tracker")
	}
	if gt.maxRetries != 3 {
		t.Errorf("Expected maxRetries 3, got %d", gt.maxRetries)
	}
}

func TestCreateGoal(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Refactor code", "Refactor authentication module", GoalPriorityHigh)

	if goal == nil {
		t.Fatal("Expected non-nil goal")
	}
	if goal.Name != "Refactor code" {
		t.Errorf("Expected name 'Refactor code', got %s", goal.Name)
	}
	if goal.Status != GoalStatusPending {
		t.Errorf("Expected status pending, got %s", goal.Status)
	}
	if goal.Priority != GoalPriorityHigh {
		t.Errorf("Expected high priority, got %d", goal.Priority)
	}

	// Check it's in the tracker
	retrieved := gt.GetGoal(goal.ID)
	if retrieved == nil {
		t.Error("Goal not found in tracker")
	}
}

func TestSetActiveGoal(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test description", GoalPriorityNormal)

	err := gt.SetActiveGoal(goal.ID)
	if err != nil {
		t.Fatalf("Failed to set active goal: %v", err)
	}

	active := gt.GetActiveGoal()
	if active == nil {
		t.Fatal("Expected active goal")
	}
	if active.ID != goal.ID {
		t.Error("Active goal ID mismatch")
	}
	if active.Status != GoalStatusInProgress {
		t.Errorf("Expected status in_progress, got %s", active.Status)
	}
}

func TestAddSuccessCriterion(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	err := gt.AddSuccessCriterion(goal.ID, "tests pass")
	if err != nil {
		t.Fatalf("Failed to add criterion: %v", err)
	}

	err = gt.AddSuccessCriterion(goal.ID, "build succeeds")
	if err != nil {
		t.Fatalf("Failed to add criterion: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.SuccessCriteria) != 2 {
		t.Errorf("Expected 2 criteria, got %d", len(retrieved.SuccessCriteria))
	}
}

func TestAddFailureCondition(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	err := gt.AddFailureCondition(goal.ID, "compilation error")
	if err != nil {
		t.Fatalf("Failed to add condition: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.FailureConditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(retrieved.FailureConditions))
	}
}

func TestMilestones(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	ms1, err := gt.AddMilestone(goal.ID, "Phase 1", "Complete initial setup")
	if err != nil {
		t.Fatalf("Failed to add milestone: %v", err)
	}

	ms2, err := gt.AddMilestone(goal.ID, "Phase 2", "Complete implementation")
	if err != nil {
		t.Fatalf("Failed to add milestone: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.Milestones) != 2 {
		t.Errorf("Expected 2 milestones, got %d", len(retrieved.Milestones))
	}

	// Complete first milestone
	err = gt.CompleteMilestone(goal.ID, ms1.ID)
	if err != nil {
		t.Fatalf("Failed to complete milestone: %v", err)
	}

	// Check progress updated
	retrieved = gt.GetGoal(goal.ID)
	if retrieved.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", retrieved.Progress)
	}

	// Complete second milestone
	gt.CompleteMilestone(goal.ID, ms2.ID)
	retrieved = gt.GetGoal(goal.ID)
	if retrieved.Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", retrieved.Progress)
	}
}

func TestRecordFileModification(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	gt.RecordFileModification(goal.ID, "main.go")
	gt.RecordFileModification(goal.ID, "config.yaml")
	gt.RecordFileModification(goal.ID, "main.go") // Duplicate

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.FilesModified) != 2 {
		t.Errorf("Expected 2 unique files, got %d", len(retrieved.FilesModified))
	}
}

func TestRecordCommand(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	gt.RecordCommand(goal.ID, "go build")
	gt.RecordCommand(goal.ID, "go test")

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.CommandsRun) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(retrieved.CommandsRun))
	}
}

func TestRecordError(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)
	gt.SetMaxRetries(3)

	gt.RecordError(goal.ID, "first error")
	gt.RecordError(goal.ID, "second error")

	retrieved := gt.GetGoal(goal.ID)
	if len(retrieved.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(retrieved.Errors))
	}
	if retrieved.RetryCount != 2 {
		t.Errorf("Expected retry count 2, got %d", retrieved.RetryCount)
	}

	// Goal should still be pending
	if retrieved.Status != GoalStatusPending {
		t.Errorf("Expected status pending, got %s", retrieved.Status)
	}
}

func TestMaxRetriesFailure(t *testing.T) {
	gt := NewGoalTracker(GoalTrackerConfig{MaxRetries: 2})

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	gt.RecordError(goal.ID, "error 1")
	gt.RecordError(goal.ID, "error 2")

	retrieved := gt.GetGoal(goal.ID)
	if retrieved.Status != GoalStatusFailed {
		t.Errorf("Expected status failed after max retries, got %s", retrieved.Status)
	}
}

// Helper to set max retries on existing tracker
func (gt *GoalTracker) SetMaxRetries(max int) {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	gt.maxRetries = max
}

func TestSetProgress(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	gt.SetProgress(goal.ID, 0.5)
	retrieved := gt.GetGoal(goal.ID)
	if retrieved.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", retrieved.Progress)
	}

	// Test bounds
	gt.SetProgress(goal.ID, -0.5)
	retrieved = gt.GetGoal(goal.ID)
	if retrieved.Progress != 0 {
		t.Errorf("Expected progress clamped to 0, got %f", retrieved.Progress)
	}

	gt.SetProgress(goal.ID, 1.5)
	retrieved = gt.GetGoal(goal.ID)
	if retrieved.Progress != 1 {
		t.Errorf("Expected progress clamped to 1, got %f", retrieved.Progress)
	}
}

func TestCompleteGoal(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)
	gt.SetActiveGoal(goal.ID)

	err := gt.CompleteGoal(goal.ID)
	if err != nil {
		t.Fatalf("Failed to complete goal: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if retrieved.Status != GoalStatusCompleted {
		t.Errorf("Expected status completed, got %s", retrieved.Status)
	}
	if retrieved.Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", retrieved.Progress)
	}
	if retrieved.CompletedAt == 0 {
		t.Error("Expected CompletedAt to be set")
	}

	// Active goal should be cleared
	if gt.GetActiveGoal() != nil {
		t.Error("Expected active goal to be cleared")
	}

	// Should be in history
	history := gt.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(history))
	}
}

func TestFailGoal(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	err := gt.FailGoal(goal.ID, "something went wrong")
	if err != nil {
		t.Fatalf("Failed to fail goal: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if retrieved.Status != GoalStatusFailed {
		t.Errorf("Expected status failed, got %s", retrieved.Status)
	}
	if len(retrieved.Errors) != 1 {
		t.Errorf("Expected 1 error recorded, got %d", len(retrieved.Errors))
	}
}

func TestAbandonGoal(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test goal", "Test", GoalPriorityNormal)

	err := gt.AbandonGoal(goal.ID, "no longer needed")
	if err != nil {
		t.Fatalf("Failed to abandon goal: %v", err)
	}

	retrieved := gt.GetGoal(goal.ID)
	if retrieved.Status != GoalStatusAbandoned {
		t.Errorf("Expected status abandoned, got %s", retrieved.Status)
	}
	if retrieved.Metadata["abandon_reason"] != "no longer needed" {
		t.Error("Expected abandon reason in metadata")
	}
}

func TestSubGoals(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	parent := gt.CreateGoal("Parent goal", "Main goal", GoalPriorityHigh)
	child := gt.CreateGoal("Child goal", "Sub-task", GoalPriorityNormal)

	err := gt.AddSubGoal(parent.ID, child)
	if err != nil {
		t.Fatalf("Failed to add sub-goal: %v", err)
	}

	retrieved := gt.GetGoal(parent.ID)
	if len(retrieved.SubGoals) != 1 {
		t.Errorf("Expected 1 sub-goal, got %d", len(retrieved.SubGoals))
	}

	// Child should also be in goals map
	childRetrieved := gt.GetGoal(child.ID)
	if childRetrieved == nil {
		t.Error("Child goal not found in goals map")
	}
}

func TestGoalTrackerDependencies(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal1 := gt.CreateGoal("Setup", "Setup environment", GoalPriorityHigh)
	goal2 := gt.CreateGoal("Build", "Build project", GoalPriorityNormal)

	err := gt.SetDependency(goal2.ID, goal1.ID)
	if err != nil {
		t.Fatalf("Failed to set dependency: %v", err)
	}

	// goal2 should have goal1 as dependency
	g2 := gt.GetGoal(goal2.ID)
	if len(g2.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(g2.Dependencies))
	}

	// goal1 should have goal2 in blocks
	g1 := gt.GetGoal(goal1.ID)
	if len(g1.Blocks) != 1 {
		t.Errorf("Expected 1 blocker, got %d", len(g1.Blocks))
	}
}

func TestCanStart(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	setup := gt.CreateGoal("Setup", "Setup", GoalPriorityHigh)
	build := gt.CreateGoal("Build", "Build", GoalPriorityNormal)

	gt.SetDependency(build.ID, setup.ID)

	// Build cannot start before setup is complete
	canStart, _ := gt.CanStart(build.ID)
	if canStart {
		t.Error("Build should not be able to start before setup completes")
	}

	// Complete setup
	gt.CompleteGoal(setup.ID)

	// Now build can start
	canStart, _ = gt.CanStart(build.ID)
	if !canStart {
		t.Error("Build should be able to start after setup completes")
	}
}

func TestGoalTrackerGetStats(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	gt.CreateGoal("Goal 1", "Test", GoalPriorityNormal)
	goal2 := gt.CreateGoal("Goal 2", "Test", GoalPriorityNormal)
	goal3 := gt.CreateGoal("Goal 3", "Test", GoalPriorityNormal)

	gt.SetActiveGoal(goal2.ID)
	gt.CompleteGoal(goal3.ID)

	stats := gt.GetStats()
	if stats.Total != 3 {
		t.Errorf("Expected total 3, got %d", stats.Total)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected 1 pending, got %d", stats.Pending)
	}
	if stats.InProgress != 1 {
		t.Errorf("Expected 1 in progress, got %d", stats.InProgress)
	}
	if stats.Completed != 1 {
		t.Errorf("Expected 1 completed, got %d", stats.Completed)
	}
}

func TestSetTargetState(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test", "Test", GoalPriorityNormal)

	gt.SetTargetState(goal.ID, "file_count", 5)
	gt.SetTargetState(goal.ID, "test_coverage", 0.8)

	retrieved := gt.GetGoal(goal.ID)
	if retrieved.TargetState["file_count"] != 5 {
		t.Error("Expected file_count to be 5")
	}
	if retrieved.TargetState["test_coverage"] != 0.8 {
		t.Error("Expected test_coverage to be 0.8")
	}
}

func TestCheckSuccessCriteria(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test", "Test", GoalPriorityNormal)
	gt.AddSuccessCriterion(goal.ID, "tests_pass")
	gt.AddSuccessCriterion(goal.ID, "build_success")

	// Evaluator that returns true for all
	allMet, _ := gt.CheckSuccessCriteria(goal.ID, func(c string) (bool, error) {
		return true, nil
	})
	if !allMet {
		t.Error("Expected all criteria met")
	}

	// Evaluator that returns false for one
	notAllMet, _ := gt.CheckSuccessCriteria(goal.ID, func(c string) (bool, error) {
		return c == "tests_pass", nil
	})
	if notAllMet {
		t.Error("Expected not all criteria met")
	}
}

func TestCheckFailureConditions(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	goal := gt.CreateGoal("Test", "Test", GoalPriorityNormal)
	gt.AddFailureCondition(goal.ID, "compilation_error")
	gt.AddFailureCondition(goal.ID, "test_timeout")

	// No conditions met
	failed, _ := gt.CheckFailureConditions(goal.ID, func(c string) (bool, error) {
		return false, nil
	})
	if failed {
		t.Error("Expected no failure conditions met")
	}

	// One condition met
	failed, _ = gt.CheckFailureConditions(goal.ID, func(c string) (bool, error) {
		return c == "compilation_error", nil
	})
	if !failed {
		t.Error("Expected failure condition met")
	}
}

func TestGetPendingAndInProgress(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	g1 := gt.CreateGoal("Goal 1", "Test", GoalPriorityNormal)
	g2 := gt.CreateGoal("Goal 2", "Test", GoalPriorityNormal)
	gt.CreateGoal("Goal 3", "Test", GoalPriorityNormal)

	gt.SetActiveGoal(g2.ID)
	gt.CompleteGoal(g1.ID)

	pending := gt.GetPendingGoals()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending goal, got %d", len(pending))
	}

	inProgress := gt.GetInProgressGoals()
	if len(inProgress) != 1 {
		t.Errorf("Expected 1 in-progress goal, got %d", len(inProgress))
	}
}

func TestTask10FullIntegration(t *testing.T) {
	gt := NewGoalTracker(DefaultGoalTrackerConfig())

	// Create a complex refactoring goal
	refactor := gt.CreateGoal("Refactor Auth Module", "Refactor authentication to use JWT", GoalPriorityHigh)

	// Define success criteria
	gt.AddSuccessCriterion(refactor.ID, "all tests pass")
	gt.AddSuccessCriterion(refactor.ID, "coverage >= 80%")
	gt.AddSuccessCriterion(refactor.ID, "no breaking changes")
	t.Logf("✓ Added 3 success criteria")

	// Define milestones
	m1, _ := gt.AddMilestone(refactor.ID, "Design", "Design new auth flow")
	m2, _ := gt.AddMilestone(refactor.ID, "Implement", "Implement JWT handler")
	m3, _ := gt.AddMilestone(refactor.ID, "Test", "Write and pass tests")
	t.Logf("✓ Added 3 milestones")

	// Set as active
	gt.SetActiveGoal(refactor.ID)
	t.Logf("✓ Set as active goal")

	// Record progress
	gt.RecordFileModification(refactor.ID, "auth/jwt.go")
	gt.RecordFileModification(refactor.ID, "auth/middleware.go")
	gt.RecordCommand(refactor.ID, "go test ./auth/...")
	t.Logf("✓ Recorded file modifications and commands")

	// Complete milestones
	gt.CompleteMilestone(refactor.ID, m1.ID)
	t.Logf("✓ Completed milestone 1 (progress: %.0f%%)", gt.GetGoal(refactor.ID).Progress*100)

	gt.CompleteMilestone(refactor.ID, m2.ID)
	t.Logf("✓ Completed milestone 2 (progress: %.0f%%)", gt.GetGoal(refactor.ID).Progress*100)

	gt.CompleteMilestone(refactor.ID, m3.ID)
	t.Logf("✓ Completed milestone 3 (progress: %.0f%%)", gt.GetGoal(refactor.ID).Progress*100)

	// Check success criteria
	allMet, _ := gt.CheckSuccessCriteria(refactor.ID, func(c string) (bool, error) {
		return true, nil
	})

	if allMet {
		gt.CompleteGoal(refactor.ID)
		t.Logf("✓ All success criteria met, goal completed")
	}

	// Verify final state
	final := gt.GetGoal(refactor.ID)
	if final.Status != GoalStatusCompleted {
		t.Errorf("Expected completed status, got %s", final.Status)
	}
	if final.Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", final.Progress)
	}
	if len(final.FilesModified) != 2 {
		t.Errorf("Expected 2 files modified, got %d", len(final.FilesModified))
	}

	stats := gt.GetStats()
	t.Logf("Final stats: Total=%d, Completed=%d, Avg Progress=%.0f%%",
		stats.Total, stats.Completed, stats.AverageProgress*100)

	t.Log("✅ Task 10: Goal State Tracking - Full integration PASSED")
}
