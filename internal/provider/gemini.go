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

// GeminiConfig configures the Gemini provider.
type GeminiConfig struct {
	Model  string
	APIKey string
}

type geminiProvider struct {
	cfg    GeminiConfig
	client *http.Client
}

// NewGemini returns a Gemini provider.
func NewGemini(cfg GeminiConfig) Provider {
	return &geminiProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *geminiProvider) Name() string {
	return fmt.Sprintf("gemini(%s)", p.cfg.Model)
}

type geminiContent struct {
	Role  string `json:"role"`
	Parts []struct {
		Text string `json:"text"`
	} `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *geminiProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	var systemContent string
	var contents []geminiContent

	for _, m := range messages {
		if m.Role == "system" {
			systemContent = m.Content
		} else {
			role := "user"
			if m.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role: role,
				Parts: []struct {
					Text string `json:"text"`
				}{{Text: m.Content}},
			})
		}
	}

	reqBody := geminiRequest{
		Contents: contents,
	}

	if systemContent != "" {
		reqBody.SystemInstruction = &geminiContent{
			Role: "system",
			Parts: []struct {
				Text string `json:"text"`
			}{{Text: systemContent}},
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Use generative language API v1beta
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.cfg.Model, p.cfg.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result geminiResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w, raw: %s", err, string(raw))
	}
	if result.Error != nil {
		return "", fmt.Errorf("gemini error: %s", result.Error.Message)
	}
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	texts := make([]string, 0)
	for _, part := range result.Candidates[0].Content.Parts {
		texts = append(texts, part.Text)
	}
	return strings.Join(texts, ""), nil
}
