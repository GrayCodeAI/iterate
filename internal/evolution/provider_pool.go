package evolution

import (
	"log/slog"

	"github.com/GrayCodeAI/iterate/internal/provider"
)

// PoolProviderConfig is an alias for backward compatibility.
type PoolProviderConfig = provider.PoolProviderConfig

// ProviderStats is an alias for backward compatibility.
type ProviderStats = provider.Stats

// ProviderPool is an alias for backward compatibility.
type ProviderPool = provider.Pool

// NewProviderPool creates a new provider pool.
func NewProviderPool(logger *slog.Logger) *provider.Pool {
	return provider.NewPool(logger)
}

// CreateProviderPoolForEngine creates a pool with a single placeholder provider.
func CreateProviderPoolForEngine(logger *slog.Logger, modelName string) *provider.Pool {
	pool := provider.NewPool(logger)

	if modelName == "" {
		modelName = "default"
	}

	pool.AddProvider(nil, provider.PoolProviderConfig{
		Name:               modelName,
		Priority:           100,
		RateLimitPerMinute: 60,
	})

	return pool
}
