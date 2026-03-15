package provider

import (
	"context"
	"fmt"
	"os"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider is the unified LLM interface.
// Switch providers via ITERATE_PROVIDER env var.
type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
	Name() string
}

// New returns the provider selected by ITERATE_PROVIDER.
// Supported values: ollama, openai, anthropic, groq (default: ollama)
func New() (Provider, error) {
	name := os.Getenv("ITERATE_PROVIDER")
	if name == "" {
		name = "ollama"
	}

	switch name {
	case "ollama":
		return NewOpenAICompat(OpenAICompatConfig{
			BaseURL: getEnvOr("OLLAMA_BASE_URL", "http://100.102.194.103:11434/v1"),
			Model:   getEnvOr("ITERATE_MODEL", "qwen3-coder:30b"),
			APIKey:  "ollama", // Ollama doesn't require a real key
		}), nil

	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
		}
		return NewOpenAICompat(OpenAICompatConfig{
			BaseURL: "https://api.openai.com/v1",
			Model:   getEnvOr("ITERATE_MODEL", "gpt-4o"),
			APIKey:  key,
		}), nil

	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for anthropic provider")
		}
		return NewAnthropic(AnthropicConfig{
			Model:  getEnvOr("ITERATE_MODEL", "claude-sonnet-4-6"),
			APIKey: key,
		}), nil

	case "groq":
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GROQ_API_KEY is required for groq provider")
		}
		return NewOpenAICompat(OpenAICompatConfig{
			BaseURL: "https://api.groq.com/openai/v1",
			Model:   getEnvOr("ITERATE_MODEL", "llama-3.3-70b-versatile"),
			APIKey:  key,
		}), nil

	default:
		// Any OpenAI-compatible endpoint via custom base URL
		baseURL := os.Getenv("ITERATE_BASE_URL")
		if baseURL == "" {
			return nil, fmt.Errorf("unknown provider %q — set ITERATE_BASE_URL for custom endpoints", name)
		}
		return NewOpenAICompat(OpenAICompatConfig{
			BaseURL: baseURL,
			Model:   getEnvOr("ITERATE_MODEL", "default"),
			APIKey:  getEnvOr("ITERATE_API_KEY", "none"),
		}), nil
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
