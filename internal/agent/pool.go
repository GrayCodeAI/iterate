package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// Pool manages a pool of agents with rate limiting and concurrency control.
// Safe for spawning 100+ concurrent agents with controlled API rate.
type Pool struct {
	mu          sync.Mutex
	provider    iteragent.Provider
	tools       []iteragent.Tool
	agents      []*iteragent.Agent
	available   chan *iteragent.Agent
	rateLimiter *RateLimiter
	logger      *slog.Logger
	maxAgents   int
}

// RateLimiter controls API call frequency using token bucket algorithm.
type RateLimiter struct {
	tokens   chan struct{}
	refill   time.Duration
	stopChan chan struct{}
	stopOnce sync.Once
}

// NewRateLimiter creates a rate limiter with the given requests per second.
func NewRateLimiter(rps int) *RateLimiter {
	rl := &RateLimiter{
		tokens:   make(chan struct{}, rps*2), // burst capacity
		refill:   time.Second / time.Duration(rps),
		stopChan: make(chan struct{}),
	}
	// Fill initial tokens
	for i := 0; i < rps; i++ {
		rl.tokens <- struct{}{}
	}
	// Refill tokens periodically
	go func() {
		ticker := time.NewTicker(rl.refill)
		defer ticker.Stop()
		for {
			select {
			case <-rl.stopChan:
				return
			case <-ticker.C:
				select {
				case rl.tokens <- struct{}{}:
				default:
				}
			}
		}
	}()
	return rl
}

// Wait blocks until a token is available or context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.tokens:
		return nil
	}
}

// Stop stops the rate limiter goroutine. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stopChan)
	})
}

// NewPool creates an agent pool with rate limiting.
// maxAgents: maximum concurrent agents (e.g., 10)
// rps: API requests per second limit (e.g., 5)
func NewPool(provider iteragent.Provider, tools []iteragent.Tool, logger *slog.Logger, maxAgents, rps int) *Pool {
	pool := &Pool{
		provider:    provider,
		tools:       tools,
		available:   make(chan *iteragent.Agent, maxAgents),
		rateLimiter: NewRateLimiter(rps),
		logger:      logger,
		maxAgents:   maxAgents,
	}
	return pool
}

// Acquire gets an agent from the pool, waiting for rate limit if needed.
func (p *Pool) Acquire(ctx context.Context) (*iteragent.Agent, error) {
	// Wait for rate limit token
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	p.mu.Lock()
	// Check if we have an available agent
	select {
	case agent := <-p.available:
		p.mu.Unlock()
		return agent, nil
	default:
		// Create new agent if under limit
		if len(p.agents) < p.maxAgents {
			agent := iteragent.New(p.provider, p.tools, p.logger)
			p.agents = append(p.agents, agent)
			p.mu.Unlock()
			return agent, nil
		}
		p.mu.Unlock()
	}

	// Wait for an agent to become available
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case agent := <-p.available:
		return agent, nil
	}
}

// Release returns an agent to the pool for reuse.
func (p *Pool) Release(agent *iteragent.Agent) {
	// Clear agent messages for reuse
	agent.Reset()
	select {
	case p.available <- agent:
	default:
		// Pool full, close the agent
		_ = agent.Close() // best-effort cleanup
	}
}

// Spawn executes a task with a pooled agent.
func (p *Pool) Spawn(ctx context.Context, task string, handler func(*iteragent.Agent) error) error {
	agent, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer p.Release(agent)
	return handler(agent)
}

// SpawnAll executes multiple tasks concurrently with rate limiting.
// Returns errors for each task (nil if successful).
func (p *Pool) SpawnAll(ctx context.Context, tasks []string, handler func(*iteragent.Agent, string) error) []error {
	errs := make([]error, len(tasks))
	var errMu sync.Mutex
	var wg sync.WaitGroup

	for i, task := range tasks {
		i, task := i, task // capture loop variables
		wg.Add(1)          // must be before goroutine launch to avoid Wait() returning early
		go func() {
			defer wg.Done()
			err := p.Spawn(ctx, task, func(agent *iteragent.Agent) error {
				return handler(agent, task)
			})
			errMu.Lock()
			errs[i] = err
			errMu.Unlock()
		}()
	}

	wg.Wait()
	return errs
}

// Close releases all pool resources.
func (p *Pool) Close() {
	p.rateLimiter.Stop()
	p.mu.Lock()
	for _, agent := range p.agents {
		_ = agent.Close() // best-effort cleanup
	}
	p.agents = nil
	p.mu.Unlock()
}
