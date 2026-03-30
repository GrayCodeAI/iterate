package main

import (
	"fmt"
	"log/slog"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/provider"
	"github.com/GrayCodeAI/iterate/internal/ui/selector"
)

// resolveProviderConfig merges flag values with persisted config.
// Flags take precedence: only defaults ("gemini", empty) are overridden.
func resolveProviderConfig(flagProvider, flagModel, flagAPIKey string, cfg iterConfig) (providerName, model, apiKey string) {
	pc := provider.ResolveConfig(flagProvider, flagModel, flagAPIKey,
		cfg.Provider, cfg.Model, cfg.APIKey, cfg.OllamaBaseURL)
	return pc.Provider, pc.Model, pc.APIKey
}

// resolveThinkingLevel returns the effective thinking level, falling back to
// the persisted config value when the flag is still at its default "off".
func resolveThinkingLevel(flagThinking string, cfg iterConfig) string {
	return provider.ResolveThinkingLevel(flagThinking, cfg.ThinkingLevel)
}

// initProvider creates an LLM provider from the given name and API key.
// It also wires the provider's context window into the selector for display,
// and runs a background health check to surface auth errors early.
func initProvider(providerName, apiKey string, logger *slog.Logger) (iteragent.Provider, error) {
	p, err := provider.New(providerName, apiKey)
	if err != nil {
		return nil, err
	}
	logger.Info("using provider", "name", p.Name())
	selector.ContextWindow = provider.ContextWindow(p)

	provider.RunHealthCheckInBackground(p, logger, func(err error) {
		hint := provider.AuthErrorHint(err.Error())
		fmt.Printf("\n%s⚠  Provider health check failed: %s%s\n", colorYellow, err, colorReset)
		if hint != "" {
			fmt.Printf("%s   Fix: %s%s\n", colorYellow, hint, colorReset)
		}
	})

	return p, nil
}

// runProviderHealthCheck sends a minimal prompt and prints a warning if the
// provider returns an auth / connectivity error. Runs in a goroutine so it
// doesn't block startup.
func runProviderHealthCheck(p iteragent.Provider, logger *slog.Logger) {
	if err := provider.HealthCheck(nil, p, logger); err != nil {
		hint := provider.AuthErrorHint(err.Error())
		fmt.Printf("\n%s⚠  Provider health check failed: %s%s\n", colorYellow, err, colorReset)
		if hint != "" {
			fmt.Printf("%s   Fix: %s%s\n", colorYellow, hint, colorReset)
		}
		logger.Warn("provider health check failed", "error", err)
	}
}
