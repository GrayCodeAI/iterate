// Package autonomous - Task 5: Interrupt/Resume capability for long-running tasks
package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateManager handles persistence and recovery of autonomous session state.
type StateManager struct {
	mu           sync.RWMutex
	stateDir     string
	sessionID    string
	checkpoints  []Checkpoint
	maxCheckpoints int
	autoSave     bool
	saveInterval time.Duration
}

// Checkpoint represents a saved state of an autonomous session.
type Checkpoint struct {
	ID           string         `json:"id"`
	Timestamp    int64          `json:"timestamp"`
	Phase        string         `json:"phase"`
	Iteration    int            `json:"iteration"`
	Task         string         `json:"task"`
	Plan         *Plan          `json:"plan,omitempty"`
	CompletedSteps []int        `json:"completed_steps"`
	PendingSteps []int          `json:"pending_steps"`
	Result       *Result        `json:"result,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// SessionState represents the full state of an autonomous session.
type SessionState struct {
	SessionID     string        `json:"session_id"`
	Task          string        `json:"task"`
	Status        SessionStatus `json:"status"`
	StartTime     int64         `json:"start_time"`
	LastUpdate    int64         `json:"last_update"`
	Checkpoints   []Checkpoint  `json:"checkpoints"`
	CurrentPhase  string        `json:"current_phase"`
	Iteration     int           `json:"iteration"`
	TotalCost     float64       `json:"total_cost"`
	Error         string        `json:"error,omitempty"`
}

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionStatusRunning    SessionStatus = "running"
	SessionStatusPaused     SessionStatus = "paused"
	SessionStatusCompleted  SessionStatus = "completed"
	SessionStatusFailed     SessionStatus = "failed"
	SessionStatusInterrupted SessionStatus = "interrupted"
)

// NewStateManager creates a new state manager.
func NewStateManager(stateDir string, sessionID string) *StateManager {
	return &StateManager{
		stateDir:       stateDir,
		sessionID:      sessionID,
		checkpoints:    make([]Checkpoint, 0),
		maxCheckpoints: 10,
		autoSave:       true,
		saveInterval:   30 * time.Second,
	}
}

// CreateCheckpoint saves the current state for later resumption.
func (sm *StateManager) CreateCheckpoint(phase string, iteration int, task string, plan *Plan, completedSteps, pendingSteps []int, result *Result) (*Checkpoint, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	checkpoint := Checkpoint{
		ID:             fmt.Sprintf("ckpt_%d", time.Now().UnixNano()),
		Timestamp:      time.Now().Unix(),
		Phase:          phase,
		Iteration:      iteration,
		Task:           task,
		Plan:           plan,
		CompletedSteps: completedSteps,
		PendingSteps:   pendingSteps,
		Result:         result,
		Metadata:       make(map[string]any),
	}

	// Add to checkpoints
	sm.checkpoints = append(sm.checkpoints, checkpoint)

	// Trim old checkpoints if needed
	if len(sm.checkpoints) > sm.maxCheckpoints {
		sm.checkpoints = sm.checkpoints[len(sm.checkpoints)-sm.maxCheckpoints:]
	}

	// Persist to disk
	if err := sm.saveToDisk(); err != nil {
		return nil, fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// GetLatestCheckpoint returns the most recent checkpoint.
func (sm *StateManager) GetLatestCheckpoint() *Checkpoint {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.checkpoints) == 0 {
		return nil
	}
	
	latest := sm.checkpoints[len(sm.checkpoints)-1]
	return &latest
}

// RestoreFromCheckpoint restores state from a specific checkpoint.
func (sm *StateManager) RestoreFromCheckpoint(checkpointID string) (*Checkpoint, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, cp := range sm.checkpoints {
		if cp.ID == checkpointID {
			return &cp, nil
		}
	}

	return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
}

// ResumeSession loads a previously interrupted session.
func (sm *StateManager) ResumeSession(sessionID string) (*SessionState, error) {
	stateFile := filepath.Join(sm.stateDir, sessionID, "session.json")
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
	}

	// Restore checkpoints
	sm.mu.Lock()
	sm.sessionID = sessionID
	sm.checkpoints = state.Checkpoints
	sm.mu.Unlock()

	return &state, nil
}

// SaveSession persists the current session state.
func (sm *StateManager) SaveSession(task string, status SessionStatus, phase string, iteration int, totalCost float64, errMsg string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state := SessionState{
		SessionID:    sm.sessionID,
		Task:         task,
		Status:       status,
		StartTime:    time.Now().Unix() - int64(iteration*60), // Approximate
		LastUpdate:   time.Now().Unix(),
		Checkpoints:  sm.checkpoints,
		CurrentPhase: phase,
		Iteration:    iteration,
		TotalCost:    totalCost,
		Error:        errMsg,
	}

	stateDir := filepath.Join(sm.stateDir, sm.sessionID)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, "session.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	return os.WriteFile(stateFile, data, 0644)
}

// saveToDisk persists checkpoints to disk.
func (sm *StateManager) saveToDisk() error {
	stateDir := filepath.Join(sm.stateDir, sm.sessionID)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	checkpointFile := filepath.Join(stateDir, "checkpoints.json")
	data, err := json.MarshalIndent(sm.checkpoints, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(checkpointFile, data, 0644)
}

// loadFromDisk loads checkpoints from disk.
func (sm *StateManager) loadFromDisk() error {
	checkpointFile := filepath.Join(sm.stateDir, sm.sessionID, "checkpoints.json")
	
	data, err := os.ReadFile(checkpointFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No checkpoints yet
		}
		return err
	}

	return json.Unmarshal(data, &sm.checkpoints)
}

// ListSessions returns all saved sessions.
func (sm *StateManager) ListSessions() ([]SessionState, error) {
	entries, err := os.ReadDir(sm.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionState
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stateFile := filepath.Join(sm.stateDir, entry.Name(), "session.json")
		data, err := os.ReadFile(stateFile)
		if err != nil {
			continue
		}

		var state SessionState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		sessions = append(sessions, state)
	}

	return sessions, nil
}

// DeleteSession removes a saved session.
func (sm *StateManager) DeleteSession(sessionID string) error {
	sessionDir := filepath.Join(sm.stateDir, sessionID)
	return os.RemoveAll(sessionDir)
}

// InterruptContext provides interrupt handling for autonomous operations.
type InterruptContext struct {
	mu          sync.RWMutex
	interrupted bool
	reason      string
	checkpoint  *Checkpoint
	cancelFunc  context.CancelFunc
}

// NewInterruptContext creates a new interrupt context.
func NewInterruptContext() *InterruptContext {
	return &InterruptContext{
		interrupted: false,
	}
}

// Interrupt signals an interruption request.
func (ic *InterruptContext) Interrupt(reason string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	
	ic.interrupted = true
	ic.reason = reason
	
	if ic.cancelFunc != nil {
		ic.cancelFunc()
	}
}

// IsInterrupted checks if an interruption was requested.
func (ic *InterruptContext) IsInterrupted() bool {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.interrupted
}

// Reason returns the interruption reason.
func (ic *InterruptContext) Reason() string {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.reason
}

// SetCheckpoint saves a checkpoint during interruption.
func (ic *InterruptContext) SetCheckpoint(cp *Checkpoint) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.checkpoint = cp
}

// GetCheckpoint returns the saved checkpoint.
func (ic *InterruptContext) GetCheckpoint() *Checkpoint {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.checkpoint
}

// SetCancelFunc sets the context cancel function.
func (ic *InterruptContext) SetCancelFunc(cancel context.CancelFunc) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.cancelFunc = cancel
}

// ResumableEngine wraps Engine with interrupt/resume capability.
type ResumableEngine struct {
	engine       *Engine
	stateManager *StateManager
	interrupt    *InterruptContext
}

// NewResumableEngine creates a resumable engine wrapper.
func NewResumableEngine(engine *Engine, stateDir string) *ResumableEngine {
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
	return &ResumableEngine{
		engine:       engine,
		stateManager: NewStateManager(stateDir, sessionID),
		interrupt:    NewInterruptContext(),
	}
}

// Run executes the autonomous task with interrupt/resume support.
func (re *ResumableEngine) Run(ctx context.Context, task string) (*Result, error) {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	re.interrupt.SetCancelFunc(cancel)

	// Check for previous session to resume
	latestCheckpoint := re.stateManager.GetLatestCheckpoint()
	
	startIteration := 0
	var currentPlan *Plan
	
	if latestCheckpoint != nil && latestCheckpoint.Phase != "completed" {
		startIteration = latestCheckpoint.Iteration
		currentPlan = latestCheckpoint.Plan
	}

	result := &Result{
		Status: "success",
	}

	for iteration := startIteration; iteration < re.engine.config.MaxIterations; iteration++ {
		// Check for interruption
		if re.interrupt.IsInterrupted() {
			// Create checkpoint before exiting
			cp, err := re.stateManager.CreateCheckpoint(
				re.engine.status.Phase,
				iteration,
				task,
				currentPlan,
				nil, // completed steps
				nil, // pending steps
				result,
			)
			if err == nil {
				re.interrupt.SetCheckpoint(cp)
			}
			
			re.stateManager.SaveSession(task, SessionStatusInterrupted, re.engine.status.Phase, iteration, result.TotalCost, re.interrupt.Reason())
			
			result.Status = "interrupted"
			return result, fmt.Errorf("interrupted: %s", re.interrupt.Reason())
		}

		// Execute iteration (simplified - actual implementation would use engine methods)
		re.engine.updateProgress("executing", fmt.Sprintf("Iteration %d", iteration))
		
		// Create periodic checkpoint
		if iteration%5 == 0 {
			re.stateManager.CreateCheckpoint(
				re.engine.status.Phase,
				iteration,
				task,
				currentPlan,
				nil,
				nil,
				result,
			)
		}
	}

	re.stateManager.SaveSession(task, SessionStatusCompleted, "completed", re.engine.config.MaxIterations, result.TotalCost, "")

	return result, nil
}

// Interrupt signals the engine to stop and save state.
func (re *ResumableEngine) Interrupt(reason string) {
	re.interrupt.Interrupt(reason)
}

// Resume loads a previous session and continues execution.
func (re *ResumableEngine) Resume(ctx context.Context, sessionID string) (*Result, error) {
	state, err := re.stateManager.ResumeSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to resume session: %w", err)
	}

	if state.Status != SessionStatusInterrupted && state.Status != SessionStatusPaused {
		return nil, fmt.Errorf("cannot resume session with status %s", state.Status)
	}

	// Continue from last checkpoint
	latestCp := re.stateManager.GetLatestCheckpoint()
	if latestCp == nil {
		return nil, fmt.Errorf("no checkpoint found for session %s", sessionID)
	}

	return re.Run(ctx, state.Task)
}

// ListResumableSessions returns sessions that can be resumed.
func (re *ResumableEngine) ListResumableSessions() ([]SessionState, error) {
	sessions, err := re.stateManager.ListSessions()
	if err != nil {
		return nil, err
	}

	var resumable []SessionState
	for _, s := range sessions {
		if s.Status == SessionStatusInterrupted || s.Status == SessionStatusPaused {
			resumable = append(resumable, s)
		}
	}

	return resumable, nil
}
