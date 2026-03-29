// Package autonomous - Task 10: Goal State tracking for complex multi-file changes
package autonomous

import (
	"fmt"
	"sync"
	"time"
)

// GoalStatus represents the status of a goal.
type GoalStatus string

const (
	GoalStatusPending    GoalStatus = "pending"
	GoalStatusInProgress GoalStatus = "in_progress"
	GoalStatusCompleted  GoalStatus = "completed"
	GoalStatusFailed     GoalStatus = "failed"
	GoalStatusAbandoned  GoalStatus = "abandoned"
)

// GoalPriority represents the priority of a goal.
type GoalPriority int

const (
	GoalPriorityLow GoalPriority = iota
	GoalPriorityNormal
	GoalPriorityHigh
	GoalPriorityCritical
)

// GoalState represents a desired end state for the autonomous agent.
type GoalState struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Priority    GoalPriority           `json:"priority"`
	Status      GoalStatus             `json:"status"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
	CompletedAt int64                  `json:"completed_at,omitempty"`
	
	// Goal definition
	TargetState  map[string]any `json:"target_state,omitempty"`  // Desired state
	SuccessCriteria []string    `json:"success_criteria"`         // Conditions for success
	FailureConditions []string  `json:"failure_conditions,omitempty"` // Conditions for failure
	
	// Progress tracking
	SubGoals    []*GoalState  `json:"sub_goals,omitempty"`
	Progress    float64       `json:"progress"` // 0.0 - 1.0
	Milestones  []Milestone   `json:"milestones,omitempty"`
	
	// Execution tracking
	FilesModified  []string `json:"files_modified,omitempty"`
	CommandsRun    []string `json:"commands_run,omitempty"`
	Errors         []string `json:"errors,omitempty"`
	RetryCount     int      `json:"retry_count"`
	MaxRetries     int      `json:"max_retries"`
	
	// Dependencies
	Dependencies []string `json:"dependencies,omitempty"` // Goal IDs this depends on
	Blocks       []string `json:"blocks,omitempty"`       // Goal IDs that depend on this
	
	// Metadata
	Tags    []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Milestone represents a checkpoint in goal progress.
type Milestone struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt int64  `json:"completed_at,omitempty"`
}

// GoalTracker manages goal states for autonomous operations.
type GoalTracker struct {
	mu          sync.RWMutex
	goals       map[string]*GoalState
	activeGoal  *GoalState
	maxRetries  int
	history     []*GoalState
}

// GoalTrackerConfig holds configuration for the goal tracker.
type GoalTrackerConfig struct {
	MaxRetries int `json:"max_retries"`
}

// DefaultGoalTrackerConfig returns sensible defaults.
func DefaultGoalTrackerConfig() GoalTrackerConfig {
	return GoalTrackerConfig{
		MaxRetries: 3,
	}
}

// NewGoalTracker creates a new goal tracker.
func NewGoalTracker(config GoalTrackerConfig) *GoalTracker {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	
	return &GoalTracker{
		goals:      make(map[string]*GoalState),
		maxRetries: config.MaxRetries,
		history:    make([]*GoalState, 0),
	}
}

// CreateGoal creates a new goal.
func (gt *GoalTracker) CreateGoal(name string, description string, priority GoalPriority) *GoalState {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	now := time.Now().Unix()
	goal := &GoalState{
		ID:             fmt.Sprintf("goal_%d_%d", time.Now().UnixNano(), len(gt.goals)),
		Name:           name,
		Description:    description,
		Priority:       priority,
		Status:         GoalStatusPending,
		CreatedAt:      now,
		UpdatedAt:       now,
		TargetState:    make(map[string]any),
		SuccessCriteria: make([]string, 0),
		FailureConditions: make([]string, 0),
		SubGoals:       make([]*GoalState, 0),
		Milestones:     make([]Milestone, 0),
		FilesModified:  make([]string, 0),
		CommandsRun:    make([]string, 0),
		Errors:         make([]string, 0),
		MaxRetries:     gt.maxRetries,
		Tags:           make([]string, 0),
		Metadata:       make(map[string]any),
	}
	
	gt.goals[goal.ID] = goal
	return goal
}

// SetActiveGoal sets the current active goal.
func (gt *GoalTracker) SetActiveGoal(goalID string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	gt.activeGoal = goal
	goal.Status = GoalStatusInProgress
	goal.UpdatedAt = time.Now().Unix()
	
	return nil
}

// GetActiveGoal returns the current active goal.
func (gt *GoalTracker) GetActiveGoal() *GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	return gt.activeGoal
}

// AddSuccessCriterion adds a success criterion to a goal.
func (gt *GoalTracker) AddSuccessCriterion(goalID string, criterion string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.SuccessCriteria = append(goal.SuccessCriteria, criterion)
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// AddFailureCondition adds a failure condition to a goal.
func (gt *GoalTracker) AddFailureCondition(goalID string, condition string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.FailureConditions = append(goal.FailureConditions, condition)
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// AddMilestone adds a milestone to a goal.
func (gt *GoalTracker) AddMilestone(goalID string, name string, description string) (*Milestone, error) {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return nil, fmt.Errorf("goal %s not found", goalID)
	}
	
	milestone := Milestone{
		ID:          fmt.Sprintf("ms_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Completed:   false,
	}
	
	goal.Milestones = append(goal.Milestones, milestone)
	goal.UpdatedAt = time.Now().Unix()
	
	return &milestone, nil
}

// CompleteMilestone marks a milestone as completed.
func (gt *GoalTracker) CompleteMilestone(goalID string, milestoneID string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	for i, ms := range goal.Milestones {
		if ms.ID == milestoneID {
			goal.Milestones[i].Completed = true
			goal.Milestones[i].CompletedAt = time.Now().Unix()
			goal.UpdatedAt = time.Now().Unix()
			gt.recalculateProgress(goal)
			return nil
		}
	}
	
	return fmt.Errorf("milestone %s not found", milestoneID)
}

// recalculateProgress updates the progress based on milestones.
func (gt *GoalTracker) recalculateProgress(goal *GoalState) {
	if len(goal.Milestones) == 0 {
		return
	}
	
	completed := 0
	for _, ms := range goal.Milestones {
		if ms.Completed {
			completed++
		}
	}
	
	goal.Progress = float64(completed) / float64(len(goal.Milestones))
}

// RecordFileModification records a file modification for a goal.
func (gt *GoalTracker) RecordFileModification(goalID string, filePath string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	// Check if already recorded
	for _, f := range goal.FilesModified {
		if f == filePath {
			return nil
		}
	}
	
	goal.FilesModified = append(goal.FilesModified, filePath)
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// RecordCommand records a command execution for a goal.
func (gt *GoalTracker) RecordCommand(goalID string, command string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.CommandsRun = append(goal.CommandsRun, command)
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// RecordError records an error for a goal.
func (gt *GoalTracker) RecordError(goalID string, errMsg string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.Errors = append(goal.Errors, errMsg)
	goal.RetryCount++
	goal.UpdatedAt = time.Now().Unix()
	
	// Check if max retries exceeded
	if goal.RetryCount >= goal.MaxRetries {
		goal.Status = GoalStatusFailed
	}
	
	return nil
}

// SetProgress manually sets the progress of a goal.
func (gt *GoalTracker) SetProgress(goalID string, progress float64) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}
	
	goal.Progress = progress
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// CompleteGoal marks a goal as completed.
func (gt *GoalTracker) CompleteGoal(goalID string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.Status = GoalStatusCompleted
	goal.Progress = 1.0
	now := time.Now().Unix()
	goal.CompletedAt = now
	goal.UpdatedAt = now
	
	// Add to history
	gt.history = append(gt.history, goal)
	
	// Clear active goal if this was it
	if gt.activeGoal != nil && gt.activeGoal.ID == goalID {
		gt.activeGoal = nil
	}
	
	return nil
}

// FailGoal marks a goal as failed.
func (gt *GoalTracker) FailGoal(goalID string, reason string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.Status = GoalStatusFailed
	now := time.Now().Unix()
	goal.CompletedAt = now
	goal.UpdatedAt = now
	
	if reason != "" {
		goal.Errors = append(goal.Errors, reason)
	}
	
	// Add to history
	gt.history = append(gt.history, goal)
	
	// Clear active goal if this was it
	if gt.activeGoal != nil && gt.activeGoal.ID == goalID {
		gt.activeGoal = nil
	}
	
	return nil
}

// AbandonGoal marks a goal as abandoned.
func (gt *GoalTracker) AbandonGoal(goalID string, reason string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.Status = GoalStatusAbandoned
	now := time.Now().Unix()
	goal.CompletedAt = now
	goal.UpdatedAt = now
	
	if reason != "" {
		goal.Metadata["abandon_reason"] = reason
	}
	
	// Add to history
	gt.history = append(gt.history, goal)
	
	// Clear active goal if this was it
	if gt.activeGoal != nil && gt.activeGoal.ID == goalID {
		gt.activeGoal = nil
	}
	
	return nil
}

// AddSubGoal adds a sub-goal to a parent goal.
func (gt *GoalTracker) AddSubGoal(parentID string, subGoal *GoalState) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	parent, exists := gt.goals[parentID]
	if !exists {
		return fmt.Errorf("parent goal %s not found", parentID)
	}
	
	parent.SubGoals = append(parent.SubGoals, subGoal)
	parent.UpdatedAt = time.Now().Unix()
	
	// Also add to goals map
	gt.goals[subGoal.ID] = subGoal
	
	return nil
}

// SetDependency sets a dependency between goals.
func (gt *GoalTracker) SetDependency(goalID string, dependsOnGoalID string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	dependsOnGoal, exists := gt.goals[dependsOnGoalID]
	if !exists {
		return fmt.Errorf("dependency goal %s not found", dependsOnGoalID)
	}
	
	// Add dependency
	goal.Dependencies = append(goal.Dependencies, dependsOnGoalID)
	dependsOnGoal.Blocks = append(dependsOnGoal.Blocks, goalID)
	
	goal.UpdatedAt = time.Now().Unix()
	dependsOnGoal.UpdatedAt = time.Now().Unix()
	
	return nil
}

// CanStart checks if a goal can be started (all dependencies completed).
func (gt *GoalTracker) CanStart(goalID string) (bool, error) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return false, fmt.Errorf("goal %s not found", goalID)
	}
	
	for _, depID := range goal.Dependencies {
		depGoal, exists := gt.goals[depID]
		if !exists || depGoal.Status != GoalStatusCompleted {
			return false, nil
		}
	}
	
	return true, nil
}

// GetGoal returns a goal by ID.
func (gt *GoalTracker) GetGoal(goalID string) *GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	return gt.goals[goalID]
}

// GetAllGoals returns all goals.
func (gt *GoalTracker) GetAllGoals() []*GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	goals := make([]*GoalState, 0, len(gt.goals))
	for _, goal := range gt.goals {
		goals = append(goals, goal)
	}
	return goals
}

// GetPendingGoals returns all pending goals.
func (gt *GoalTracker) GetPendingGoals() []*GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	pending := make([]*GoalState, 0)
	for _, goal := range gt.goals {
		if goal.Status == GoalStatusPending {
			pending = append(pending, goal)
		}
	}
	return pending
}

// GetInProgressGoals returns all in-progress goals.
func (gt *GoalTracker) GetInProgressGoals() []*GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	inProgress := make([]*GoalState, 0)
	for _, goal := range gt.goals {
		if goal.Status == GoalStatusInProgress {
			inProgress = append(inProgress, goal)
		}
	}
	return inProgress
}

// GetHistory returns the goal history.
func (gt *GoalTracker) GetHistory() []*GoalState {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	history := make([]*GoalState, len(gt.history))
	copy(history, gt.history)
	return history
}

// GetStats returns goal statistics.
func (gt *GoalTracker) GetStats() GoalStats {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	stats := GoalStats{}
	for _, goal := range gt.goals {
		stats.Total++
		switch goal.Status {
		case GoalStatusPending:
			stats.Pending++
		case GoalStatusInProgress:
			stats.InProgress++
		case GoalStatusCompleted:
			stats.Completed++
		case GoalStatusFailed:
			stats.Failed++
		case GoalStatusAbandoned:
			stats.Abandoned++
		}
		stats.TotalProgress += goal.Progress
	}
	
	if stats.Total > 0 {
		stats.AverageProgress = stats.TotalProgress / float64(stats.Total)
	}
	
	return stats
}

// GoalStats holds statistics about goals.
type GoalStats struct {
	Total           int     `json:"total"`
	Pending         int     `json:"pending"`
	InProgress      int     `json:"in_progress"`
	Completed       int     `json:"completed"`
	Failed          int     `json:"failed"`
	Abandoned       int     `json:"abandoned"`
	TotalProgress   float64 `json:"total_progress"`
	AverageProgress float64 `json:"average_progress"`
}

// SetTargetState sets the target state for a goal.
func (gt *GoalTracker) SetTargetState(goalID string, key string, value any) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return fmt.Errorf("goal %s not found", goalID)
	}
	
	goal.TargetState[key] = value
	goal.UpdatedAt = time.Now().Unix()
	return nil
}

// CheckSuccessCriteria evaluates if all success criteria are met.
func (gt *GoalTracker) CheckSuccessCriteria(goalID string, evaluator func(criterion string) (bool, error)) (bool, error) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return false, fmt.Errorf("goal %s not found", goalID)
	}
	
	for _, criterion := range goal.SuccessCriteria {
		met, err := evaluator(criterion)
		if err != nil {
			return false, err
		}
		if !met {
			return false, nil
		}
	}
	
	return true, nil
}

// CheckFailureConditions evaluates if any failure conditions are met.
func (gt *GoalTracker) CheckFailureConditions(goalID string, evaluator func(condition string) (bool, error)) (bool, error) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	
	goal, exists := gt.goals[goalID]
	if !exists {
		return false, fmt.Errorf("goal %s not found", goalID)
	}
	
	for _, condition := range goal.FailureConditions {
		met, err := evaluator(condition)
		if err != nil {
			return false, err
		}
		if met {
			return true, nil
		}
	}
	
	return false, nil
}
