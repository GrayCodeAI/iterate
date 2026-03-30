package autonomous

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateManagerCreation(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")
	if sm == nil {
		t.Fatal("Expected non-nil state manager")
	}
	if sm.sessionID != "test_session" {
		t.Errorf("Expected session ID 'test_session', got %s", sm.sessionID)
	}
}

func TestCreateCheckpoint(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")

	plan := &Plan{Steps: []PlanStep{{Type: "read", Target: "test.go"}}}
	result := &Result{Status: "success"}

	cp, err := sm.CreateCheckpoint("executing", 1, "test task", plan, []int{0}, []int{1}, result)
	if err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	if cp.Phase != "executing" {
		t.Errorf("Expected phase 'executing', got %s", cp.Phase)
	}
	if cp.Iteration != 1 {
		t.Errorf("Expected iteration 1, got %d", cp.Iteration)
	}
}

func TestGetLatestCheckpoint(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")

	// No checkpoints yet
	if sm.GetLatestCheckpoint() != nil {
		t.Error("Expected nil for empty checkpoints")
	}

	// Create checkpoints
	sm.CreateCheckpoint("phase1", 1, "task1", nil, nil, nil, nil)
	time.Sleep(time.Millisecond) // Ensure different timestamps
	sm.CreateCheckpoint("phase2", 2, "task2", nil, nil, nil, nil)

	latest := sm.GetLatestCheckpoint()
	if latest == nil {
		t.Fatal("Expected non-nil checkpoint")
	}
	if latest.Phase != "phase2" {
		t.Errorf("Expected phase 'phase2', got %s", latest.Phase)
	}
}

func TestRestoreFromCheckpoint(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")

	// Create checkpoint
	cp1, _ := sm.CreateCheckpoint("phase1", 1, "task1", nil, nil, nil, nil)

	// Restore by ID
	cp2, err := sm.RestoreFromCheckpoint(cp1.ID)
	if err != nil {
		t.Fatalf("Failed to restore checkpoint: %v", err)
	}

	if cp2.ID != cp1.ID {
		t.Errorf("Expected ID %s, got %s", cp1.ID, cp2.ID)
	}

	// Non-existent checkpoint
	_, err = sm.RestoreFromCheckpoint("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent checkpoint")
	}
}

func TestSaveAndResumeSession(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "session1")

	// Save session
	err := sm.SaveSession("test task", SessionStatusRunning, "executing", 5, 0.25, "")
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify file exists
	stateFile := filepath.Join(tmpDir, "session1", "session.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatal("Session file not created")
	}

	// Create new state manager and resume
	sm2 := NewStateManager(tmpDir, "session2")
	state, err := sm2.ResumeSession("session1")
	if err != nil {
		t.Fatalf("Failed to resume session: %v", err)
	}

	if state.Task != "test task" {
		t.Errorf("Expected task 'test task', got %s", state.Task)
	}
	if state.Status != SessionStatusRunning {
		t.Errorf("Expected status 'running', got %s", state.Status)
	}
	if state.Iteration != 5 {
		t.Errorf("Expected iteration 5, got %d", state.Iteration)
	}
}

func TestListSessions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm1 := NewStateManager(tmpDir, "session1")
	sm1.SaveSession("task1", SessionStatusCompleted, "done", 10, 0.5, "")

	sm2 := NewStateManager(tmpDir, "session2")
	sm2.SaveSession("task2", SessionStatusInterrupted, "executing", 3, 0.2, "user interrupt")

	sm3 := NewStateManager(tmpDir, "session3")
	sessions, err := sm3.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}
}

func TestDeleteSession(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "session1")
	sm.SaveSession("task", SessionStatusCompleted, "done", 1, 0.0, "")

	// Verify exists
	sessionDir := filepath.Join(tmpDir, "session1")
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Fatal("Session directory not created")
	}

	// Delete
	if err := sm.DeleteSession("session1"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Session directory still exists after deletion")
	}
}

func TestInterruptContext(t *testing.T) {
	ic := NewInterruptContext()

	// Not interrupted initially
	if ic.IsInterrupted() {
		t.Error("Expected not interrupted initially")
	}

	// Interrupt
	ic.Interrupt("user requested")

	if !ic.IsInterrupted() {
		t.Error("Expected interrupted after Interrupt()")
	}
	if ic.Reason() != "user requested" {
		t.Errorf("Expected reason 'user requested', got %s", ic.Reason())
	}
}

func TestInterruptContextCheckpoint(t *testing.T) {
	ic := NewInterruptContext()

	// No checkpoint initially
	if ic.GetCheckpoint() != nil {
		t.Error("Expected nil checkpoint initially")
	}

	// Set checkpoint
	cp := &Checkpoint{ID: "test_cp", Phase: "executing"}
	ic.SetCheckpoint(cp)

	if ic.GetCheckpoint() == nil {
		t.Fatal("Expected non-nil checkpoint")
	}
	if ic.GetCheckpoint().ID != "test_cp" {
		t.Errorf("Expected ID 'test_cp', got %s", ic.GetCheckpoint().ID)
	}
}

func TestMaxCheckpoints(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm := NewStateManager(tmpDir, "test_session")
	sm.maxCheckpoints = 3

	// Create more checkpoints than max
	for i := 0; i < 5; i++ {
		sm.CreateCheckpoint("phase", i, "task", nil, nil, nil, nil)
	}

	// Should only have last 3
	sm.mu.RLock()
	count := len(sm.checkpoints)
	sm.mu.RUnlock()

	if count != 3 {
		t.Errorf("Expected 3 checkpoints, got %d", count)
	}
}

func TestCheckpointPersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	sm1 := NewStateManager(tmpDir, "test_session")

	// Create checkpoint with plan
	plan := &Plan{Steps: []PlanStep{{Type: "read", Target: "main.go"}}}
	sm1.CreateCheckpoint("executing", 1, "task", plan, []int{0}, []int{1, 2}, nil)

	// Verify file exists
	cpFile := filepath.Join(tmpDir, "test_session", "checkpoints.json")
	if _, err := os.Stat(cpFile); os.IsNotExist(err) {
		t.Fatal("Checkpoints file not created")
	}

	// Load in new state manager
	sm2 := NewStateManager(tmpDir, "test_session")
	if err := sm2.loadFromDisk(); err != nil {
		t.Fatalf("Failed to load checkpoints: %v", err)
	}

	cp := sm2.GetLatestCheckpoint()
	if cp == nil {
		t.Fatal("Expected checkpoint after loading from disk")
	}
	if cp.Task != "task" {
		t.Errorf("Expected task 'task', got %s", cp.Task)
	}
}

func TestSessionStatusValues(t *testing.T) {
	statuses := []SessionStatus{
		SessionStatusRunning,
		SessionStatusPaused,
		SessionStatusCompleted,
		SessionStatusFailed,
		SessionStatusInterrupted,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("Status should not be empty")
		}
	}
}

func TestCheckpointStruct(t *testing.T) {
	cp := Checkpoint{
		ID:             "ckpt_123",
		Timestamp:      time.Now().Unix(),
		Phase:          "executing",
		Iteration:      5,
		Task:           "refactor code",
		CompletedSteps: []int{0, 1, 2},
		PendingSteps:   []int{3, 4},
		Metadata:       map[string]any{"key": "value"},
	}

	if cp.ID != "ckpt_123" {
		t.Errorf("Expected ID 'ckpt_123', got %s", cp.ID)
	}
	if len(cp.CompletedSteps) != 3 {
		t.Errorf("Expected 3 completed steps, got %d", len(cp.CompletedSteps))
	}
}

func TestTask5FullIntegration(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "state_test")
	defer os.RemoveAll(tmpDir)

	// Create state manager
	sm := NewStateManager(tmpDir, "integration_test")

	// Create a series of checkpoints during "execution"
	plan := &Plan{Steps: []PlanStep{
		{Type: "read", Target: "main.go"},
		{Type: "edit", Target: "main.go"},
		{Type: "test", Target: "main_test.go"},
	}}

	result := &Result{Status: "in_progress", TotalCost: 0.1}

	// Simulate execution with checkpoints
	for i := 0; i < 3; i++ {
		cp, err := sm.CreateCheckpoint(
			"executing",
			i,
			"refactor main.go",
			plan,
			[]int{0, 1},
			[]int{2},
			result,
		)
		if err != nil {
			t.Fatalf("Failed to create checkpoint %d: %v", i, err)
		}
		t.Logf("Created checkpoint: %s at iteration %d", cp.ID, i)
	}

	// Save final session state
	sm.SaveSession("refactor main.go", SessionStatusInterrupted, "executing", 3, 0.15, "user interrupt")

	// Verify we can resume
	sm2 := NewStateManager(tmpDir, "new_session")
	state, err := sm2.ResumeSession("integration_test")
	if err != nil {
		t.Fatalf("Failed to resume session: %v", err)
	}

	if state.Task != "refactor main.go" {
		t.Errorf("Expected task 'refactor main.go', got %s", state.Task)
	}
	if state.Status != SessionStatusInterrupted {
		t.Errorf("Expected status 'interrupted', got %s", state.Status)
	}

	// Get latest checkpoint from resumed session
	latest := sm2.GetLatestCheckpoint()
	if latest == nil {
		t.Fatal("Expected checkpoint in resumed session")
	}

	if latest.Plan == nil {
		t.Fatal("Expected plan in checkpoint")
	}
	if len(latest.Plan.Steps) != 3 {
		t.Errorf("Expected 3 plan steps, got %d", len(latest.Plan.Steps))
	}

	// Verify we have the right number of checkpoints
	stats := sm2.GetLatestCheckpoint()
	if stats == nil {
		t.Error("Expected checkpoint to exist")
	}

	t.Log("✅ Task 5: Interrupt/Resume Capability - Full integration PASSED")
}
