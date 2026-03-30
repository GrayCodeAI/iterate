package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OpenRouterModel represents a model from the OpenRouter API.
type OpenRouterModel struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ContextLength int     `json:"context_length"`
	Modality      string  `json:"modality"`
	InputPrice    float64 `json:"input_price"`
	OutputPrice   float64 `json:"output_price"`
	IsFree        bool    `json:"is_free"`
}

// OpenRouterModelsResponse is the top-level API response.
type OpenRouterModelsResponse struct {
	Data []OpenRouterModelRaw `json:"data"`
}

// OpenRouterModelRaw is the raw model struct from the API.
type OpenRouterModelRaw struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextLength *int   `json:"context_length"`
	Architecture  struct {
		Modality string `json:"modality"`
	} `json:"architecture"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

var (
	openRouterCache     []OpenRouterModel
	openRouterCacheTime time.Time
	openRouterMu        sync.Mutex
)

// FetchOpenRouterModels fetches all models from the OpenRouter API.
// Results are cached for 1 hour to avoid rate limits.
func FetchOpenRouterModels(ctx context.Context) ([]OpenRouterModel, error) {
	openRouterMu.Lock()
	defer openRouterMu.Unlock()

	// Return cached results if fresh enough (1 hour).
	if len(openRouterCache) > 0 && time.Since(openRouterCacheTime) < time.Hour {
		return openRouterCache, nil
	}

	// Try loading from disk cache first.
	if models, err := loadDiskCache(); err == nil && len(models) > 0 {
		openRouterCache = models
		openRouterCacheTime = time.Now()
		return models, nil
	}

	// Fetch from API.
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp OpenRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]OpenRouterModel, 0, len(apiResp.Data))
	for _, raw := range apiResp.Data {
		inputPrice := parseFloat(raw.Pricing.Prompt)
		outputPrice := parseFloat(raw.Pricing.Completion)
		ctxLen := 0
		if raw.ContextLength != nil {
			ctxLen = *raw.ContextLength
		}

		models = append(models, OpenRouterModel{
			ID:            raw.ID,
			Name:          raw.Name,
			Description:   raw.Description,
			ContextLength: ctxLen,
			Modality:      raw.Architecture.Modality,
			InputPrice:    inputPrice,
			OutputPrice:   outputPrice,
			IsFree:        inputPrice == 0 && outputPrice == 0,
		})
	}

	// Sort by ID for consistent ordering.
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	openRouterCache = models
	openRouterCacheTime = time.Now()

	// Save to disk cache.
	_ = saveDiskCache(models)

	return models, nil
}

// FreeOpenRouterModels returns only the free models.
func FreeOpenRouterModels(ctx context.Context) ([]OpenRouterModel, error) {
	all, err := FetchOpenRouterModels(ctx)
	if err != nil {
		return nil, err
	}

	free := make([]OpenRouterModel, 0, len(all))
	for _, m := range all {
		if m.IsFree {
			free = append(free, m)
		}
	}
	return free, nil
}

// OpenRouterModelIDs returns all model IDs (optionally free-only).
func OpenRouterModelIDs(ctx context.Context, freeOnly bool) ([]string, error) {
	all, err := FetchOpenRouterModels(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(all))
	for _, m := range all {
		if freeOnly && !m.IsFree {
			continue
		}
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func loadDiskCache() ([]OpenRouterModel, error) {
	cachePath := diskCachePath()
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	// Check if cache is stale (older than 24 hours).
	info, err := os.Stat(cachePath)
	if err != nil || time.Since(info.ModTime()) > 24*time.Hour {
		return nil, fmt.Errorf("cache stale")
	}

	var models []OpenRouterModel
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}
	return models, nil
}

func saveDiskCache(models []OpenRouterModel) error {
	cachePath := diskCachePath()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(models)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}

func diskCachePath() string {
	// Try XDG cache dir first, fall back to home dir.
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = "/tmp"
		}
		cacheDir = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheDir, "iterate", "openrouter_models.json")
}

// IsOpenRouterModel checks if a model ID is an OpenRouter model.
func IsOpenRouterModel(model string) bool {
	return strings.Contains(model, "openrouter/") ||
		strings.HasSuffix(model, ":free") ||
		strings.HasPrefix(model, "google/gemma") ||
		strings.HasPrefix(model, "meta-llama/") ||
		strings.HasPrefix(model, "qwen/") ||
		strings.HasPrefix(model, "nvidia/") ||
		strings.HasPrefix(model, "minimax/") ||
		strings.HasPrefix(model, "stepfun/") ||
		strings.HasPrefix(model, "z-ai/") ||
		strings.HasPrefix(model, "liquid/") ||
		strings.HasPrefix(model, "arcee-ai/") ||
		strings.HasPrefix(model, "nousresearch/") ||
		strings.HasPrefix(model, "cognitivecomputations/") ||
		strings.HasPrefix(model, "openai/gpt-oss")
}
