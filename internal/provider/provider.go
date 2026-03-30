// Package provider manages LLM provider lifecycle: creation, configuration,
// health checks, and multi-provider pooling with rate-limit awareness.
//
// It sits between the external iteragent library and the rest of the codebase,
// providing a single import point for all provider-related functionality.
package provider

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// Config holds the resolved provider configuration.
type Config struct {
	Provider string
	Model    string
	APIKey   string
}

// ResolveConfig merges flag values with persisted config.
// Flags take precedence: only defaults are overridden by persisted values.
func ResolveConfig(flagProvider, flagModel, flagAPIKey string, cfgProvider, cfgModel, cfgAPIKey, cfgOllamaBaseURL string) Config {
	provider := flagProvider
	model := flagModel
	apiKey := flagAPIKey

	if provider == "gemini" && cfgProvider != "" && cfgProvider != "gemini" {
		provider = cfgProvider
	}
	if model == "" && cfgModel != "" {
		model = cfgModel
	}
	if apiKey == "" && cfgAPIKey != "" {
		apiKey = cfgAPIKey
	}
	if cfgOllamaBaseURL != "" && os.Getenv("OLLAMA_BASE_URL") == "" {
		os.Setenv("OLLAMA_BASE_URL", cfgOllamaBaseURL)
	}
	if provider == "openrouter" && os.Getenv("OPENAI_BASE_URL") == "" {
		os.Setenv("OPENAI_BASE_URL", "https://openrouter.ai/api/v1")
		if apiKey == "" {
			apiKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if apiKey != "" {
			os.Setenv("OPENAI_API_KEY", apiKey)
		}
	}
	if model != "" {
		os.Setenv("ITERATE_MODEL", model)
	}

	return Config{Provider: provider, Model: model, APIKey: apiKey}
}

// ResolveThinkingLevel returns the effective thinking level, falling back to
// the persisted config value when the flag is still at its default "off".
func ResolveThinkingLevel(flagThinking, cfgThinkingLevel string) string {
	if flagThinking == "off" && cfgThinkingLevel != "" {
		return cfgThinkingLevel
	}
	return flagThinking
}

// New creates an LLM provider from the given name and API key.
func New(providerName, apiKey string) (iteragent.Provider, error) {
	return iteragent.NewProvider(providerName, apiKey)
}

// ContextWindow returns the context window size for a provider.
func ContextWindow(p iteragent.Provider) int {
	return iteragent.ProviderContextWindow(p)
}

// HealthCheck sends a minimal prompt to verify provider connectivity.
// Returns nil on success, or an error describing the failure.
func HealthCheck(ctx context.Context, p iteragent.Provider, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ag := iteragent.New(p, nil, logger)
	events := ag.Prompt(ctx, "ping")
	for e := range events {
		if iteragent.EventType(e.Type) == iteragent.EventError {
			return fmt.Errorf("%s", e.Content)
		}
		if iteragent.EventType(e.Type) == iteragent.EventTokenUpdate {
			ag.Finish()
			return nil
		}
	}
	return fmt.Errorf("provider returned no events")
}

// RunHealthCheckInBackground runs a health check in a goroutine and calls
// onFail with the error if the check fails. Skips Ollama by default.
func RunHealthCheckInBackground(p iteragent.Provider, logger *slog.Logger, onFail func(error)) {
	if os.Getenv("ITERATE_SKIP_HEALTH_CHECK") == "1" {
		return
	}
	if strings.EqualFold(p.Name(), "ollama") {
		return
	}
	go func() {
		if err := HealthCheck(context.Background(), p, logger); err != nil {
			logger.Warn("provider health check failed", "error", err, "hint", AuthErrorHint(err.Error()))
			if onFail != nil {
				onFail(err)
			}
		}
	}()
}

// AuthErrorHint returns a human-readable fix suggestion for common auth errors.
func AuthErrorHint(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "invalid_api_key"):
		return "Check your API key is correct and not expired."
	case strings.Contains(lower, "incorrect_api_key"):
		return "The API key is incorrect. Verify the key in your config."
	case strings.Contains(lower, "unauthorized"):
		return "Authentication failed. Check your API key and permissions."
	case strings.Contains(lower, "forbidden"):
		return "Access denied. Check your subscription and permissions."
	case strings.Contains(lower, "quota"):
		return "API quota exceeded. Check your usage limits."
	case strings.Contains(lower, "rate limit"):
		return "Rate limited. Wait a moment and try again."
	case strings.Contains(lower, "connection refused"):
		return "Cannot connect to the API. Check your network and base URL."
	case strings.Contains(lower, "timeout"):
		return "Request timed out. Check your network connection."
	case strings.Contains(lower, "model_not_found"):
		return "Model not found. Check the model name is correct."
	}
	return ""
}

// IsRateLimitError checks if an error indicates a rate limit.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	indicators := []string{
		"rate limit",
		"429",
		"quota exceeded",
		"too many requests",
		"ratelimited",
		"subscriptionusagelimiterror",
	}
	for _, indicator := range indicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}
	return false
}
