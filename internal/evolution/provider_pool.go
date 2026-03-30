package evolution

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

type ProviderConfig struct {
	Name               string
	Priority           int
	RateLimitPerMinute int
}

type ProviderPool struct {
	providers []*ProviderWithStats
	current   int
	logger    *slog.Logger
	mu        sync.RWMutex
}

type ProviderWithStats struct {
	iteragent.Provider
	Config ProviderConfig
	Stats  ProviderStats
}

type ProviderStats struct {
	TotalCalls   int
	SuccessCalls int
	FailedCalls  int
	RateLimited  int
	LastCall     time.Time
	LastError    string
}

func NewProviderPool(logger *slog.Logger) *ProviderPool {
	return &ProviderPool{
		logger: logger,
	}
}

func (p *ProviderPool) AddProvider(provider iteragent.Provider, config ProviderConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.providers = append(p.providers, &ProviderWithStats{
		Provider: provider,
		Config:   config,
		Stats:    ProviderStats{},
	})
}

func (p *ProviderPool) GetProvider() iteragent.Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.providers) == 0 {
		return nil
	}

	var bestProvider *ProviderWithStats
	var bestScore int

	for i, provider := range p.providers {
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

	return p.providers[p.current%len(p.providers)].Provider
}

func (p *ProviderPool) isRateLimited(provider *ProviderWithStats) bool {
	if provider.Config.RateLimitPerMinute == 0 {
		return false
	}

	sinceLastCall := time.Since(provider.Stats.LastCall)
	if sinceLastCall < time.Minute {
		return provider.Stats.TotalCalls >= provider.Config.RateLimitPerMinute
	}

	return false
}

func (p *ProviderPool) RecordSuccess(providerName string) {
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

func (p *ProviderPool) RecordFailure(providerName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, provider := range p.providers {
		if provider.Config.Name == providerName {
			provider.Stats.TotalCalls++
			provider.Stats.FailedCalls++
			provider.Stats.LastError = err.Error()
			provider.Stats.LastCall = time.Now()

			if isRateLimitError(err) {
				provider.Stats.RateLimited++
			}
			break
		}
	}
}

func (p *ProviderPool) GetStats() map[string]ProviderStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]ProviderStats)
	for _, provider := range p.providers {
		stats[provider.Config.Name] = provider.Stats
	}

	return stats
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	rateLimitIndicators := []string{
		"rate limit",
		"429",
		"quota exceeded",
		"too many requests",
		"ratelimited",
		"subscriptionusagelimiterror",
	}

	for _, indicator := range rateLimitIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}

	return false
}

func CreateProviderPoolForEngine(logger *slog.Logger, modelName string) *ProviderPool {
	pool := NewProviderPool(logger)

	if modelName == "" {
		modelName = "default"
	}

	pool.AddProvider(nil, ProviderConfig{
		Name:               modelName,
		Priority:           100,
		RateLimitPerMinute: 60,
	})

	return pool
}
