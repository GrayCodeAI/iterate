package agent

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// NewPool
// ---------------------------------------------------------------------------

func TestNewPool_Creates(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 5, 10)
	if p == nil {
		t.Fatal("pool should not be nil")
	}
	p.Close()
}

func TestNewPool_MaxAgentsSet(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 10, 5)
	if p.maxAgents != 10 {
		t.Errorf("expected maxAgents 10, got %d", p.maxAgents)
	}
	p.Close()
}

// ---------------------------------------------------------------------------
// Pool Acquire / Release
// ---------------------------------------------------------------------------

func TestPool_AcquireCreatesAgent(t *testing.T) {
	// Use a nil provider since we're just testing pool mechanics
	p := NewPool(nil, nil, slog.Default(), 5, 100)
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	agent, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if agent == nil {
		t.Fatal("agent should not be nil")
	}
}

func TestPool_ReleaseReturnsToPool(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 2, 100)
	defer p.Close()

	ctx := context.Background()
	a1, _ := p.Acquire(ctx)
	p.Release(a1)

	// Should be able to acquire again (reusing the agent)
	a2, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	if a2 == nil {
		t.Fatal("reused agent should not be nil")
	}
}

func TestPool_AcquireRespectsMaxAgents(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 1, 100)
	defer p.Close()

	ctx := context.Background()
	a1, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire should block since max is 1 and we haven't released
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = p.Acquire(ctx2)
	if err == nil {
		t.Error("expected timeout when pool is full")
	}

	// Release and try again
	p.Release(a1)
	a3, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
	if a3 == nil {
		t.Fatal("agent should not be nil")
	}
}

func TestPool_AcquireRespectsContext(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 1, 1) // 1 rps
	defer p.Close()

	ctx := context.Background()
	p.Acquire(ctx)

	ctx2, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Acquire(ctx2)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Pool Spawn
// ---------------------------------------------------------------------------

func TestPool_Spawn(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 3, 100)
	defer p.Close()

	ctx := context.Background()
	called := false
	err := p.Spawn(ctx, "test task", func(a *iteragent.Agent) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestPool_SpawnAll(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 5, 100)
	defer p.Close()

	ctx := context.Background()
	tasks := []string{"task1", "task2", "task3"}
	var mu sync.Mutex
	count := 0

	errs := p.SpawnAll(ctx, tasks, func(a *iteragent.Agent, task string) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	for i, err := range errs {
		if err != nil {
			t.Errorf("task %d failed: %v", i, err)
		}
	}
	mu.Lock()
	if count != 3 {
		t.Errorf("expected 3 handler calls, got %d", count)
	}
	mu.Unlock()
}

// ---------------------------------------------------------------------------
// Pool Close
// ---------------------------------------------------------------------------

func TestPool_Close(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 3, 10)
	p.Close()
}

// ---------------------------------------------------------------------------
// RateLimiter edge cases
// ---------------------------------------------------------------------------

func TestNewRateLimiter_HighRPS(t *testing.T) {
	rl := NewRateLimiter(100)
	defer rl.Stop()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("wait %d failed: %v", i, err)
		}
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(10) // 10 rps = 100ms per token
	defer rl.Stop()

	ctx := context.Background()
	// Consume all initial tokens
	for i := 0; i < 10; i++ {
		rl.Wait(ctx)
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rl.Wait(ctx2); err != nil {
		t.Errorf("should have refilled token: %v", err)
	}
}
