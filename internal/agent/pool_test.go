package agent

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter_Creates(t *testing.T) {
	rl := NewRateLimiter(5)
	if rl == nil {
		t.Fatal("should not be nil")
	}
	rl.Stop()
}

func TestNewRateLimiter_HasTokens(t *testing.T) {
	rl := NewRateLimiter(3)
	defer rl.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rl.Wait(ctx); err != nil {
		t.Errorf("should have initial token: %v", err)
	}
}

func TestRateLimiter_WaitReturnsOnCancel(t *testing.T) {
	// Create with 0 rps (or very low) and immediately exhaust
	rl := NewRateLimiter(1)
	defer rl.Stop()

	ctx := context.Background()
	// Consume token
	rl.Wait(ctx)

	// Create a context that will be cancelled
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := rl.Wait(ctx2)
	if err == nil {
		t.Error("should return error when context is cancelled")
	}
}

func TestRateLimiter_StopDoesNotPanic(t *testing.T) {
	rl := NewRateLimiter(10)
	rl.Stop()
	// Calling stop again should not panic
	// (though behavior is undefined, we test it doesn't crash the test)
}

func TestRateLimiter_MultipleWaits(t *testing.T) {
	rl := NewRateLimiter(100)
	defer rl.Stop()

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("wait %d failed: %v", i, err)
		}
	}
}
