package provider

import (
	"log/slog"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// Config holds configuration for a provider in the pool.
type PoolProviderConfig struct {
	Name               string
	Priority           int
	RateLimitPerMinute int
}

// Stats tracks usage and error metrics for a provider.
type Stats struct {
	TotalCalls   int
	SuccessCalls int
	FailedCalls  int
	RateLimited  int
	LastCall     time.Time
	LastError    string
}

// Pool manages multiple LLM providers with rate-limit awareness and automatic failover.
type Pool struct {
	providers []*providerWithStats
	current   int
	logger    *slog.Logger
	mu        sync.RWMutex
}

type providerWithStats struct {
	iteragent.Provider
	Config PoolProviderConfig
	Stats  Stats
}

// NewPool creates an empty provider pool.
func NewPool(logger *slog.Logger) *Pool {
	return &Pool{
		logger: logger,
	}
}

// AddProvider registers a provider with the pool.
func (p *Pool) AddProvider(provider iteragent.Provider, config PoolProviderConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.providers = append(p.providers, &providerWithStats{
		Provider: provider,
		Config:   config,
		Stats:    Stats{},
	})
}

// GetProvider returns the best available provider based on priority and rate limits.
// Returns nil if no providers are available.
func (p *Pool) GetProvider() iteragent.Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.providers) == 0 {
		return nil
	}

	var bestProvider *providerWithStats
	var bestScore int

	for i, provider := range p.providers {
		if provider.Provider == nil {
			continue
		}
		if p.isRateLimited(provider) {
			continue
		}

		score := provider.Config.Priority * 1000

		if time.Since(provider.Stats.LastCall) < time.Minute {
			score -= provider.Stats.TotalCalls * 10
		}

		if score > bestScore {
			bestScore = score
			bestProvider = p.providers[i]
		}
	}

	if bestProvider != nil {
		return bestProvider.Provider
	}

	// Fallback: return first non-nil provider
	for _, provider := range p.providers {
		if provider.Provider != nil {
			return provider.Provider
		}
	}
	return nil
}

// RecordSuccess records a successful API call for a provider.
func (p *Pool) RecordSuccess(providerName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, provider := range p.providers {
		if provider.Config.Name == providerName {
			provider.Stats.TotalCalls++
			provider.Stats.SuccessCalls++
			provider.Stats.LastCall = time.Now()
			break
		}
	}
}

// RecordFailure records a failed API call for a provider.
func (p *Pool) RecordFailure(providerName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, provider := range p.providers {
		if provider.Config.Name == providerName {
			provider.Stats.TotalCalls++
			provider.Stats.FailedCalls++
			provider.Stats.LastError = err.Error()
			provider.Stats.LastCall = time.Now()

			if IsRateLimitError(err) {
				provider.Stats.RateLimited++
			}
			break
		}
	}
}

// GetStats returns usage statistics for all providers.
func (p *Pool) GetStats() map[string]Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]Stats)
	for _, provider := range p.providers {
		stats[provider.Config.Name] = provider.Stats
	}

	return stats
}

func (p *Pool) isRateLimited(provider *providerWithStats) bool {
	if provider.Config.RateLimitPerMinute == 0 {
		return false
	}

	sinceLastCall := time.Since(provider.Stats.LastCall)
	if sinceLastCall < time.Minute {
		return provider.Stats.TotalCalls >= provider.Config.RateLimitPerMinute
	}

	return false
}
