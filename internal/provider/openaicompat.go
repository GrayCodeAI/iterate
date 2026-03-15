package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatConfig configures any OpenAI-compatible endpoint.
type OpenAICompatConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type openAICompat struct {
	cfg    OpenAICompatConfig
	client *http.Client
}

// NewOpenAICompat returns a provider for any OpenAI-compatible API.
func NewOpenAICompat(cfg OpenAICompatConfig) Provider {
	return &openAICompat{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *openAICompat) Name() string {
	return fmt.Sprintf("openai-compat(%s @ %s)", p.cfg.Model, p.cfg.BaseURL)
}

type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *openAICompat) Complete(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(openAIRequest{
		Model:       p.cfg.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4096,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Ensure base URL has /v1 suffix for Ollama
	baseURL := p.cfg.BaseURL
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result openAIResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		// Debug: print raw response on error
		return "", fmt.Errorf("unmarshal response: %w, raw: %s", err, string(raw))
	}
	if result.Error != nil {
		return "", fmt.Errorf("provider error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}
