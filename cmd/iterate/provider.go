package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/ui/selector"
)

// envOverrides holds environment variable overrides that the caller can
// apply safely instead of resolveProviderConfig calling os.Setenv itself.
type envOverrides struct {
	OllamaBaseURL string
	IterateModel  string
}

// resolveProviderConfig merges flag values with persisted config.
// Flags take precedence: only defaults ("gemini", empty) are overridden.
// Returns env overrides map that the caller should apply goroutine-safely.
func resolveProviderConfig(flagProvider, flagModel, flagAPIKey string, cfg iterConfig) (provider, model, apiKey string, env envOverrides) {
	provider = flagProvider
	model = flagModel
	apiKey = flagAPIKey

	if provider == "gemini" && cfg.Provider != "" && cfg.Provider != "gemini" {
		provider = cfg.Provider
	}
	if model == "" && cfg.Model != "" {
		model = cfg.Model
	}
	if apiKey == "" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	if cfg.OllamaBaseURL != "" && os.Getenv("OLLAMA_BASE_URL") == "" {
		env.OllamaBaseURL = cfg.OllamaBaseURL
	}
	if model != "" {
		env.IterateModel = model
	}

	return provider, model, apiKey, env
}

// resolveThinkingLevel returns the effective thinking level, falling back to
// the persisted config value when the flag is still at its default "off".
func resolveThinkingLevel(flagThinking string, cfg iterConfig) string {
	if flagThinking == "off" && cfg.ThinkingLevel != "" {
		return cfg.ThinkingLevel
	}
	return flagThinking
}

// initProvider creates an LLM provider from the given name and API key.
// It also wires the provider's context window into the selector for display,
// and runs a background health check to surface auth errors early.
func initProvider(providerName, apiKey string, logger *slog.Logger) (iteragent.Provider, error) {
	p, err := iteragent.NewProvider(providerName, apiKey)
	if err != nil {
		return nil, err
	}
	logger.Info("using provider", "name", p.Name())
	selector.ContextWindow = iteragent.ProviderContextWindow(p)

	// Skip health check for Ollama (local — no auth needed) and when
	// the user explicitly passed --no-health-check (not currently a flag,
	// but the env var ITERATE_SKIP_HEALTH_CHECK=1 disables it).
	if os.Getenv("ITERATE_SKIP_HEALTH_CHECK") != "1" &&
		!strings.EqualFold(providerName, "ollama") {
		go runProviderHealthCheck(p, logger)
	}

	return p, nil
}

// runProviderHealthCheck sends a minimal prompt and prints a warning if the
// provider returns an auth / connectivity error. Runs in a goroutine so it
// doesn't block startup.
func runProviderHealthCheck(p iteragent.Provider, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ag := iteragent.New(p, nil, logger)
	defer ag.Finish()
	events := ag.Prompt(ctx, "ping")
	for e := range events {
		if iteragent.EventType(e.Type) == iteragent.EventError {
			hint := authErrorHint(e.Content)
			fmt.Printf("\n%s⚠  Provider health check failed: %s%s\n", colorYellow, e.Content, colorReset)
			if hint != "" {
				fmt.Printf("%s   Fix: %s%s\n", colorYellow, hint, colorReset)
			}
			logger.Warn("provider health check failed", "error", e.Content)
			return
		}
		// Got any non-error event — provider is reachable.
		if iteragent.EventType(e.Type) == iteragent.EventTokenUpdate {
			return
		}
	}
}
