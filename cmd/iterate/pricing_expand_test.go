package main

import (
	"math"
	"strings"
	"testing"
)

func TestEstimateCost_KnownModel(t *testing.T) {
	est := estimateCost(1000, 500, 0, 0, "gpt-4o")
	if !est.Found {
		t.Error("should find gpt-4o")
	}
	if est.Model != "gpt-4o" {
		t.Errorf("model should be gpt-4o, got %q", est.Model)
	}
	if est.InputCost <= 0 {
		t.Errorf("input cost should be > 0, got %f", est.InputCost)
	}
	if est.OutputCost <= 0 {
		t.Errorf("output cost should be > 0, got %f", est.OutputCost)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	est := estimateCost(1000, 500, 0, 0, "unknown-model-xyz")
	if est.Found {
		t.Error("should not find unknown model")
	}
	if est.Total != 0 {
		t.Errorf("total should be 0 for unknown model, got %f", est.Total)
	}
}

func TestEstimateCost_ZeroTokens(t *testing.T) {
	est := estimateCost(0, 0, 0, 0, "gpt-4o")
	if !est.Found {
		t.Error("should find gpt-4o")
	}
	if est.Total != 0 {
		t.Errorf("total should be 0 for zero tokens, got %f", est.Total)
	}
}

func TestEstimateCost_WithCacheAnthropic(t *testing.T) {
	est := estimateCost(1000, 500, 200, 300, "claude-opus-4")
	if !est.Found {
		t.Error("should find claude-opus-4")
	}
	if est.CacheWriteCost <= 0 {
		t.Errorf("cache write cost should be > 0 for claude, got %f", est.CacheWriteCost)
	}
	if est.CacheReadCost <= 0 {
		t.Errorf("cache read cost should be > 0 for claude, got %f", est.CacheReadCost)
	}
	if est.Total != est.InputCost+est.OutputCost+est.CacheWriteCost+est.CacheReadCost {
		t.Errorf("total should be sum of all costs")
	}
}

func TestEstimateCost_LargeTokens(t *testing.T) {
	est := estimateCost(1000000, 1000000, 0, 0, "gpt-4o")
	if !est.Found {
		t.Fatal("should find gpt-4o")
	}
	// gpt-4o: $2.50/MTok input, $10/MTok output
	if math.Abs(est.InputCost-2.50) > 0.01 {
		t.Errorf("expected input cost ~$2.50, got $%f", est.InputCost)
	}
	if math.Abs(est.OutputCost-10.00) > 0.01 {
		t.Errorf("expected output cost ~$10.00, got $%f", est.OutputCost)
	}
}

func TestEstimateCost_VariousModels(t *testing.T) {
	models := []string{
		"claude-opus-4", "claude-sonnet-4", "claude-haiku-3-5",
		"gpt-4o", "gpt-4o-mini", "gpt-4.1",
		"gemini-2.5-pro", "gemini-2.5-flash",
		"grok-3", "deepseek-r1",
		"llama-3.3-70b", "mistral-large",
	}
	for _, model := range models {
		est := estimateCost(100, 50, 0, 0, model)
		if !est.Found {
			t.Errorf("model %q should be found", model)
		}
		if est.Total <= 0 {
			t.Errorf("model %q should have cost > 0, got %f", model, est.Total)
		}
	}
}

func TestEstimateCost_ProviderPrefixed(t *testing.T) {
	models := []string{"vertex/gemini-2.5-pro", "azure/gpt-4o", "bedrock/claude-opus-4"}
	for _, model := range models {
		est := estimateCost(100, 50, 0, 0, model)
		if !est.Found {
			t.Errorf("provider-prefixed model %q should be found", model)
		}
	}
}

func TestFormatCostTable_CacheWriteOnly(t *testing.T) {
	result := formatCostTable(100, 50, 100, 0, "claude-opus-4")
	if !strings.Contains(result, "Cache W:") {
		t.Errorf("should show Cache W when cacheWrite > 0, got %q", result)
	}
	if strings.Contains(result, "Cache R:") {
		t.Errorf("should not show Cache R when cacheRead = 0, got %q", result)
	}
}

func TestFormatCostTable_CacheReadOnly(t *testing.T) {
	result := formatCostTable(100, 50, 0, 100, "claude-sonnet-4")
	if strings.Contains(result, "Cache W:") {
		t.Errorf("should not show Cache W when cacheWrite = 0, got %q", result)
	}
	if !strings.Contains(result, "Cache R:") {
		t.Errorf("should show Cache R when cacheRead > 0, got %q", result)
	}
}

func TestFormatCostTable_SmallTotal(t *testing.T) {
	// With very few tokens, total might be < 0.001
	result := formatCostTable(1, 1, 0, 0, "gpt-4o-mini")
	if !strings.Contains(result, "Total:") {
		t.Errorf("should always show Total line, got %q", result)
	}
}

func TestLookupPricing_SubstringVariants(t *testing.T) {
	variants := []string{
		"anthropic/claude-opus-4",
		"claude-haiku-3-5-20241022",
		"gpt-4o-2024-11-20",
		"gemini-2.5-flash-latest",
	}
	for _, v := range variants {
		if _, ok := lookupPricing(v); !ok {
			t.Errorf("should find pricing for %q", v)
		}
	}
}

func TestLookupPricing_CaseInsensitiveVarious(t *testing.T) {
	cases := []string{"GPT-4O", "Claude-Opus-4", "GEMINI-2.5-PRO"}
	for _, c := range cases {
		if _, ok := lookupPricing(c); !ok {
			t.Errorf("should find pricing for case-insensitive %q", c)
		}
	}
}
