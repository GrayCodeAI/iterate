package agent

import (
	"context"
	"log/slog"
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
	_, _ = p.Acquire(ctx)

	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Acquire(ctx2)
	if err == nil {
		t.Error("expected timeout when pool is full")
	}
}

func TestPool_AcquireRespectsContext(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 1, 1)
	defer p.Close()

	ctx := context.Background()
	p.Acquire(ctx)

	ctx2, cancel := context.WithCancel(context.Background())
	cancel()

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

func TestPool_SpawnAll_ReturnsCorrectLength(t *testing.T) {
	p := NewPool(nil, nil, slog.Default(), 5, 100)
	defer p.Close()

	ctx := context.Background()
	tasks := []string{"task1", "task2", "task3"}

	errs := p.SpawnAll(ctx, tasks, func(a *iteragent.Agent, task string) error {
		return nil
	})

	if len(errs) != 3 {
		t.Errorf("expected 3 error results, got %d", len(errs))
	}
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
	rl := NewRateLimiter(10)
	defer rl.Stop()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		rl.Wait(ctx)
	}

	time.Sleep(150 * time.Millisecond)

	ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rl.Wait(ctx2); err != nil {
		t.Errorf("should have refilled token: %v", err)
	}
}
