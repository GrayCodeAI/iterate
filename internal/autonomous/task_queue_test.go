// Package autonomous - Task 8: Task Queue tests
package autonomous

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTaskPriorityString(t *testing.T) {
	priorities := []TaskPriority{
		PriorityLow,
		PriorityNormal,
		PriorityHigh,
		PriorityCritical,
	}

	expected := []string{"low", "normal", "high", "critical"}

	for i, p := range priorities {
		if p.String() != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], p.String())
		}
	}
}

func TestDefaultTaskQueueConfig(t *testing.T) {
	config := DefaultTaskQueueConfig()

	if config.MaxParallel != 4 {
		t.Errorf("Expected MaxParallel 4, got %d", config.MaxParallel)
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout 5m, got %v", config.Timeout)
	}
	if config.MaxRetries != 2 {
		t.Errorf("Expected MaxRetries 2, got %d", config.MaxRetries)
	}
}

func TestNewTaskQueue(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, DefaultTaskQueueConfig())

	if tq == nil {
		t.Fatal("Expected non-nil task queue")
	}
	if tq.maxParallel != 4 {
		t.Errorf("Expected maxParallel 4, got %d", tq.maxParallel)
	}
}

func TestAddTask(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, DefaultTaskQueueConfig())

	task := tq.AddTask("test_task", "Test task description", PriorityNormal, nil)

	if task == nil {
		t.Fatal("Expected non-nil task")
	}
	if task.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got %s", task.Name)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}

	// Check it's in the queue
	stats := tq.GetStats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 total task, got %d", stats.Total)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected 1 pending task, got %d", stats.Pending)
	}
}

func TestPriorityOrdering(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	// Add tasks in reverse priority order
	tq.AddTask("low", "Low priority", PriorityLow, nil)
	tq.AddTask("critical", "Critical priority", PriorityCritical, nil)
	tq.AddTask("normal", "Normal priority", PriorityNormal, nil)
	tq.AddTask("high", "High priority", PriorityHigh, nil)

	pending := tq.GetPending()

	// First should be critical
	if pending[0].Priority != PriorityCritical {
		t.Errorf("Expected first task to be critical, got %s", pending[0].Priority)
	}
}

func TestTaskQueueDependencies(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		time.Sleep(50 * time.Millisecond)
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 2, Timeout: time.Minute, MaxRetries: 0})

	// Add task1 with no dependencies
	task1 := tq.AddTask("task1", "First task", PriorityNormal, nil)

	// Add task2 that depends on task1
	task2 := tq.AddTask("task2", "Second task", PriorityNormal, []string{task1.ID})

	// Start processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go tq.Start(ctx)

	// Wait for completion
	err := tq.WaitForCompletion(ctx)
	if err != nil {
		t.Fatalf("Failed to wait for completion: %v", err)
	}

	// Verify order
	t1 := tq.GetTask(task1.ID)
	t2 := tq.GetTask(task2.ID)

	if t1.Status != TaskStatusCompleted {
		t.Errorf("Expected task1 completed, got %s", t1.Status)
	}
	if t2.Status != TaskStatusCompleted {
		t.Errorf("Expected task2 completed, got %s", t2.Status)
	}

	// Task1 should have started before task2
	if t1.StartedAt > t2.StartedAt {
		t.Error("Task1 should have started before task2")
	}
}

func TestParallelExecution(t *testing.T) {
	var executionCount atomic.Int32

	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		executionCount.Add(1)
		time.Sleep(100 * time.Millisecond)
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 4, Timeout: time.Minute, MaxRetries: 0})

	// Add 4 independent tasks
	tq.AddTask("task1", "Task 1", PriorityNormal, nil)
	tq.AddTask("task2", "Task 2", PriorityNormal, nil)
	tq.AddTask("task3", "Task 3", PriorityNormal, nil)
	tq.AddTask("task4", "Task 4", PriorityNormal, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	go tq.Start(ctx)

	err := tq.WaitForCompletion(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to wait for completion: %v", err)
	}

	// 4 tasks with 100ms each, parallel should take ~100ms, not 400ms
	if elapsed > 300*time.Millisecond {
		t.Errorf("Parallel execution took too long: %v", elapsed)
	}

	if executionCount.Load() != 4 {
		t.Errorf("Expected 4 executions, got %d", executionCount.Load())
	}
}

func TestTaskFailure(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return nil, errors.New("task failed")
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	task := tq.AddTask("failing_task", "This will fail", PriorityNormal, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go tq.Start(ctx)

	err := tq.WaitForCompletion(ctx)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	completed := tq.GetTask(task.ID)
	if completed.Status != TaskStatusFailed {
		t.Errorf("Expected failed status, got %s", completed.Status)
	}
	if completed.Error == "" {
		t.Error("Expected error message")
	}
}

func TestTaskRetry(t *testing.T) {
	var attempts atomic.Int32

	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			return nil, errors.New("not yet")
		}
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 3})

	task := tq.AddTask("retry_task", "Will retry", PriorityNormal, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go tq.Start(ctx)

	err := tq.WaitForCompletion(ctx)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	completed := tq.GetTask(task.ID)
	if completed.Status != TaskStatusCompleted {
		t.Errorf("Expected completed, got %s", completed.Status)
	}

	// Should have succeeded on 3rd attempt
	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts.Load())
	}
}

func TestTaskTimeout(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &Result{Status: "success"}, nil
		}
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: 100 * time.Millisecond, MaxRetries: 0})

	task := tq.AddTask("timeout_task", "Will timeout", PriorityNormal, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go tq.Start(ctx)

	err := tq.WaitForCompletion(ctx)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	completed := tq.GetTask(task.ID)
	// Context deadline exceeded should cause failure
	if completed.Status != TaskStatusFailed {
		t.Errorf("Expected failed status, got %s", completed.Status)
	}
}

func TestCancelTask(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		time.Sleep(100 * time.Millisecond)
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	task := tq.AddTask("cancel_task", "Will be cancelled", PriorityNormal, nil)

	// Cancel before starting
	err := tq.Cancel(task.ID)
	if err != nil {
		t.Fatalf("Failed to cancel: %v", err)
	}

	completed := tq.GetTask(task.ID)
	if completed.Status != TaskStatusCancelled {
		t.Errorf("Expected cancelled status, got %s", completed.Status)
	}

	// Should not be in pending anymore
	stats := tq.GetStats()
	if stats.Pending != 0 {
		t.Errorf("Expected 0 pending, got %d", stats.Pending)
	}
}

func TestCancelAll(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	tq.AddTask("task1", "Task 1", PriorityNormal, nil)
	tq.AddTask("task2", "Task 2", PriorityNormal, nil)
	tq.AddTask("task3", "Task 3", PriorityNormal, nil)

	tq.CancelAll()

	stats := tq.GetStats()
	if stats.Pending != 0 {
		t.Errorf("Expected 0 pending after cancel all, got %d", stats.Pending)
	}
}

func TestWaitForTask(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		time.Sleep(50 * time.Millisecond)
		return &Result{Status: "success", FinalMessage: "done"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	task := tq.AddTask("wait_task", "Will wait for", PriorityNormal, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go tq.Start(ctx)

	completed, err := tq.WaitForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to wait for task: %v", err)
	}

	if completed.Status != TaskStatusCompleted {
		t.Errorf("Expected completed, got %s", completed.Status)
	}
	if completed.Result == nil {
		t.Fatal("Expected result")
	}
	if completed.Result.FinalMessage != "done" {
		t.Errorf("Expected message 'done', got %s", completed.Result.FinalMessage)
	}
}

func TestSetTaskMetadata(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, DefaultTaskQueueConfig())

	task := tq.AddTask("meta_task", "Test metadata", PriorityNormal, nil)

	err := tq.SetTaskMetadata(task.ID, "custom_key", "custom_value")
	if err != nil {
		t.Fatalf("Failed to set metadata: %v", err)
	}

	retrieved := tq.GetTask(task.ID)
	if retrieved.Metadata["custom_key"] != "custom_value" {
		t.Error("Expected metadata to be set")
	}
}

func TestQueueStats(t *testing.T) {
	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		return &Result{Status: "success"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 1, Timeout: time.Minute, MaxRetries: 0})

	tq.AddTask("task1", "Task 1", PriorityNormal, nil)
	tq.AddTask("task2", "Task 2", PriorityNormal, nil)

	stats := tq.GetStats()
	if stats.Total != 2 {
		t.Errorf("Expected 2 total, got %d", stats.Total)
	}
	if stats.Pending != 2 {
		t.Errorf("Expected 2 pending, got %d", stats.Pending)
	}
	if stats.Running != 0 {
		t.Errorf("Expected 0 running, got %d", stats.Running)
	}
}

func TestTask8FullIntegration(t *testing.T) {
	var taskOrder []string
	var mu sync.Mutex

	executor := func(ctx context.Context, task *QueuedTask) (*Result, error) {
		mu.Lock()
		taskOrder = append(taskOrder, task.Name)
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return &Result{Status: "success", FinalMessage: task.Name + " completed"}, nil
	}

	tq := NewTaskQueue(executor, TaskQueueConfig{MaxParallel: 3, Timeout: time.Minute, MaxRetries: 1})

	// Create a dependency graph
	//    setup
	//    /    \
	// build  lint
	//    \    /
	//    test
	setup := tq.AddTask("setup", "Setup environment", PriorityCritical, nil)
	build := tq.AddTask("build", "Build project", PriorityHigh, []string{setup.ID})
	lint := tq.AddTask("lint", "Lint code", PriorityNormal, []string{setup.ID})
	_ = tq.AddTask("test", "Run tests", PriorityHigh, []string{build.ID, lint.ID})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go tq.Start(ctx)

	err := tq.WaitForCompletion(ctx)
	if err != nil {
		t.Fatalf("Failed to wait for completion: %v", err)
	}

	// Verify setup completed
	setupTask := tq.GetTask(setup.ID)
	if setupTask.Status != TaskStatusCompleted {
		t.Errorf("Expected setup completed, got %s", setupTask.Status)
	}

	stats := tq.GetStats()
	t.Logf("Final stats: Total=%d, Completed=%d, Success=%d, Failed=%d",
		stats.Total, stats.Completed, stats.Success, stats.Failed)

	if stats.Success != 4 {
		t.Errorf("Expected 4 successful tasks, got %d", stats.Success)
	}

	// Verify setup ran first
	if taskOrder[0] != "setup" {
		t.Errorf("Expected setup first, got %s", taskOrder[0])
	}

	// Verify test ran last (depends on build and lint)
	if taskOrder[len(taskOrder)-1] != "test" {
		t.Errorf("Expected test last, got %s", taskOrder[len(taskOrder)-1])
	}

	t.Log("✅ Task 8: Task Queue for Parallel Operations - Full integration PASSED")
}
