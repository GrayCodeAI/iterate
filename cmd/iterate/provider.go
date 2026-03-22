package main

import (
	"log/slog"
	"os"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// resolveProviderConfig merges flag values with persisted config.
// Flags take precedence: only defaults ("gemini", empty) are overridden.
func resolveProviderConfig(flagProvider, flagModel string, cfg iterConfig) (provider, model string) {
	provider = flagProvider
	model = flagModel

	if provider == "gemini" && cfg.Provider != "" && cfg.Provider != "gemini" {
		provider = cfg.Provider
	}
	if model == "" && cfg.Model != "" {
		model = cfg.Model
	}
	if cfg.OllamaBaseURL != "" && os.Getenv("OLLAMA_BASE_URL") == "" {
		os.Setenv("OLLAMA_BASE_URL", cfg.OllamaBaseURL)
	}
	if model != "" {
		os.Setenv("ITERATE_MODEL", model)
	}

	return provider, model
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
func initProvider(providerName, apiKey string, logger *slog.Logger) (iteragent.Provider, error) {
	p, err := iteragent.NewProvider(providerName, apiKey)
	if err != nil {
		return nil, err
	}
	logger.Info("using provider", "name", p.Name())
	return p, nil
}
