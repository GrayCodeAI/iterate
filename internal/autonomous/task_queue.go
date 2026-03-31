// Package autonomous - Task 8: Task Queue for parallel autonomous operations
package autonomous

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskPriority represents the priority level of a task.
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns the string representation of TaskPriority.
func (p TaskPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// TaskStatus represents the current status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusRetrying  TaskStatus = "retrying"
)

// QueuedTask represents a task in the queue.
type QueuedTask struct {
	mu           sync.RWMutex
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Priority     TaskPriority   `json:"priority"`
	Status       TaskStatus     `json:"status"`
	Dependencies []string       `json:"dependencies,omitempty"`
	CreatedAt    int64          `json:"created_at"`
	StartedAt    int64          `json:"started_at,omitempty"`
	CompletedAt  int64          `json:"completed_at,omitempty"`
	Result       *Result        `json:"result,omitempty"`
	Error        string         `json:"error,omitempty"`
	RetryCount   int            `json:"retry_count"`
	MaxRetries   int            `json:"max_retries"`
	Timeout      time.Duration  `json:"timeout"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// GetStatus returns the current task status with proper locking.
func (t *QueuedTask) GetStatus() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// SetStatus sets the task status with proper locking.
func (t *QueuedTask) SetStatus(status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

// TaskExecutor is a function that executes a task.
type TaskExecutor func(ctx context.Context, task *QueuedTask) (*Result, error)

// TaskQueue manages parallel execution of autonomous tasks.
type TaskQueue struct {
	mu          sync.RWMutex
	tasks       map[string]*QueuedTask
	pending     []*QueuedTask
	running     map[string]*QueuedTask
	completed   []*QueuedTask
	executor    TaskExecutor
	maxParallel int
	timeout     time.Duration
	maxRetries  int
	stopChan    chan struct{}
	wg          sync.WaitGroup
	taskCounter int64
}

// TaskQueueConfig holds configuration for the task queue.
type TaskQueueConfig struct {
	MaxParallel int           // Maximum parallel tasks (default: 4)
	Timeout     time.Duration // Default task timeout (default: 5min)
	MaxRetries  int           // Max retries per task (default: 2)
}

// DefaultTaskQueueConfig returns sensible defaults.
func DefaultTaskQueueConfig() TaskQueueConfig {
	return TaskQueueConfig{
		MaxParallel: 4,
		Timeout:     5 * time.Minute,
		MaxRetries:  2,
	}
}

// NewTaskQueue creates a new task queue.
func NewTaskQueue(executor TaskExecutor, config TaskQueueConfig) *TaskQueue {
	if config.MaxParallel <= 0 {
		config.MaxParallel = 4
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = 2
	}

	return &TaskQueue{
		tasks:       make(map[string]*QueuedTask),
		pending:     make([]*QueuedTask, 0),
		running:     make(map[string]*QueuedTask),
		completed:   make([]*QueuedTask, 0),
		executor:    executor,
		maxParallel: config.MaxParallel,
		timeout:     config.Timeout,
		maxRetries:  config.MaxRetries,
		stopChan:    make(chan struct{}),
	}
}

// AddTask adds a new task to the queue.
func (tq *TaskQueue) AddTask(name string, description string, priority TaskPriority, dependencies []string) *QueuedTask {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	tq.taskCounter++
	task := &QueuedTask{
		ID:           fmt.Sprintf("task_%d", tq.taskCounter),
		Name:         name,
		Description:  description,
		Priority:     priority,
		Status:       TaskStatusPending,
		Dependencies: dependencies,
		CreatedAt:    time.Now().Unix(),
		MaxRetries:   tq.maxRetries,
		Timeout:      tq.timeout,
		Metadata:     make(map[string]any),
	}

	tq.tasks[task.ID] = task
	tq.pending = append(tq.pending, task)

	// Sort by priority
	tq.sortPending()

	return task
}

// sortPending sorts pending tasks by priority (highest first).
func (tq *TaskQueue) sortPending() {
	// Simple bubble sort - adequate for small queues
	for i := 0; i < len(tq.pending)-1; i++ {
		for j := i + 1; j < len(tq.pending); j++ {
			if tq.pending[j].Priority > tq.pending[i].Priority {
				tq.pending[i], tq.pending[j] = tq.pending[j], tq.pending[i]
			}
		}
	}
}

// Start begins processing tasks.
func (tq *TaskQueue) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tq.stopChan:
			return
		default:
			// Check if there's anything to do
			tq.mu.RLock()
			hasPending := len(tq.pending) > 0
			hasRunning := len(tq.running) > 0
			tq.mu.RUnlock()

			if !hasPending && !hasRunning {
				// Nothing to do, brief sleep then check again
				time.Sleep(50 * time.Millisecond)
				continue
			}

			tq.processNext(ctx)
		}
	}
}

// processNext processes the next available task.
func (tq *TaskQueue) processNext(ctx context.Context) bool {
	tq.mu.Lock()

	// Check if we can run more tasks
	if len(tq.running) >= tq.maxParallel {
		tq.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return false
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		tq.mu.Unlock()
		return false
	default:
	}

	// Find next task with satisfied dependencies
	var nextTask *QueuedTask
	var nextIndex int

	for i, task := range tq.pending {
		if tq.dependenciesSatisfiedLocked(task) {
			nextTask = task
			nextIndex = i
			break
		}
	}

	if nextTask == nil {
		tq.mu.Unlock()
		if len(tq.pending) > 0 {
			time.Sleep(50 * time.Millisecond)
		}
		return false
	}

	// Move from pending to running
	tq.pending = append(tq.pending[:nextIndex], tq.pending[nextIndex+1:]...)
	nextTask.Status = TaskStatusRunning
	nextTask.StartedAt = time.Now().Unix()
	tq.running[nextTask.ID] = nextTask

	tq.mu.Unlock()

	// Execute task in goroutine
	tq.wg.Add(1)
	go tq.executeTask(ctx, nextTask)
	return true
}

// dependenciesSatisfiedLocked checks if all dependencies are completed (must hold lock).
func (tq *TaskQueue) dependenciesSatisfiedLocked(task *QueuedTask) bool {
	for _, depID := range task.Dependencies {
		depTask, exists := tq.tasks[depID]
		if !exists {
			// Dependency doesn't exist
			return false
		}
		if depTask.Status != TaskStatusCompleted {
			return false
		}
	}
	return true
}

// executeTask executes a single task.
func (tq *TaskQueue) executeTask(ctx context.Context, task *QueuedTask) {
	defer tq.wg.Done()

	// Create timeout context
	taskCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	// Execute the task
	result, err := tq.executor(taskCtx, task)

	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Update task status
	delete(tq.running, task.ID)
	task.CompletedAt = time.Now().Unix()

	if err != nil {
		task.Error = err.Error()
		task.RetryCount++

		if task.RetryCount <= task.MaxRetries {
			task.SetStatus(TaskStatusRetrying)
			tq.pending = append(tq.pending, task)
			tq.sortPending()
		} else {
			task.SetStatus(TaskStatusFailed)
			tq.completed = append(tq.completed, task)
		}
	} else {
		task.SetStatus(TaskStatusCompleted)
		task.Result = result
		tq.completed = append(tq.completed, task)
	}
}

// Stop stops the queue and waits for running tasks.
func (tq *TaskQueue) Stop() {
	close(tq.stopChan)
	tq.wg.Wait()
}

// GetTask returns a task by ID.
func (tq *TaskQueue) GetTask(id string) *QueuedTask {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return tq.tasks[id]
}

// GetPending returns all pending tasks.
func (tq *TaskQueue) GetPending() []*QueuedTask {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	pending := make([]*QueuedTask, len(tq.pending))
	copy(pending, tq.pending)
	return pending
}

// GetRunning returns all running tasks.
func (tq *TaskQueue) GetRunning() []*QueuedTask {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	running := make([]*QueuedTask, 0, len(tq.running))
	for _, task := range tq.running {
		running = append(running, task)
	}
	return running
}

// GetCompleted returns all completed tasks.
func (tq *TaskQueue) GetCompleted() []*QueuedTask {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	completed := make([]*QueuedTask, len(tq.completed))
	copy(completed, tq.completed)
	return completed
}

// GetStats returns queue statistics.
func (tq *TaskQueue) GetStats() QueueStats {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	var failed, success int
	for _, task := range tq.completed {
		if task.GetStatus() == TaskStatusFailed {
			failed++
		} else {
			success++
		}
	}

	return QueueStats{
		Total:     len(tq.tasks),
		Pending:   len(tq.pending),
		Running:   len(tq.running),
		Completed: len(tq.completed),
		Success:   success,
		Failed:    failed,
	}
}

// QueueStats holds queue statistics.
type QueueStats struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
}

// Cancel cancels a pending task.
func (tq *TaskQueue) Cancel(taskID string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == TaskStatusRunning {
		return fmt.Errorf("cannot cancel running task")
	}

	task.Status = TaskStatusCancelled

	// Remove from pending if there
	for i, t := range tq.pending {
		if t.ID == taskID {
			tq.pending = append(tq.pending[:i], tq.pending[i+1:]...)
			break
		}
	}

	return nil
}

// CancelAll cancels all pending tasks.
func (tq *TaskQueue) CancelAll() {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	for _, task := range tq.pending {
		task.Status = TaskStatusCancelled
	}
	tq.pending = make([]*QueuedTask, 0)
}

// SetTaskMetadata sets metadata on a task.
func (tq *TaskQueue) SetTaskMetadata(taskID string, key string, value any) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Metadata[key] = value
	return nil
}

// WaitForCompletion waits for all tasks to complete.
func (tq *TaskQueue) WaitForCompletion(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stats := tq.GetStats()
			if stats.Pending == 0 && stats.Running == 0 {
				return nil
			}
		}
	}
}

// WaitForTask waits for a specific task to complete.
func (tq *TaskQueue) WaitForTask(ctx context.Context, taskID string) (*QueuedTask, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task := tq.GetTask(taskID)
			if task == nil {
				return nil, fmt.Errorf("task %s not found", taskID)
			}

			switch task.GetStatus() {
			case TaskStatusCompleted:
				return task, nil
			case TaskStatusFailed:
				return task, fmt.Errorf("task failed: %s", task.Error)
			case TaskStatusCancelled:
				return task, fmt.Errorf("task was cancelled")
			}
		}
	}
}
