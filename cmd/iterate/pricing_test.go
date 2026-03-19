package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// lookupPricing
// ---------------------------------------------------------------------------

func TestLookupPricing_ExactMatch(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"claude-opus-4", true},
		{"claude-sonnet-4", true},
		{"claude-haiku-3-5", true},
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gemini-2.5-pro", true},
		{"gemini-2.5-flash", true},
	}
	for _, c := range cases {
		_, ok := lookupPricing(c.model)
		if ok != c.want {
			t.Errorf("lookupPricing(%q) found=%v, want found=%v", c.model, ok, c.want)
		}
	}
}

func TestLookupPricing_SubstringMatch(t *testing.T) {
	// Provider prefixes and version suffixes should still match via substring.
	cases := []string{
		"claude-opus-4-20250514",        // version suffix
		"anthropic/claude-sonnet-4",     // provider prefix
		"claude-haiku-3-5-20241022",     // date suffix
		"gpt-4o-2024-11-20",             // OpenAI versioned
	}
	for _, model := range cases {
		_, ok := lookupPricing(model)
		if !ok {
			t.Errorf("lookupPricing(%q): expected substring match, got not-found", model)
		}
	}
}

func TestLookupPricing_LongestWins(t *testing.T) {
	// "gpt-4o-mini" is longer than "gpt-4o" — the more specific entry must win.
	p, ok := lookupPricing("gpt-4o-mini")
	if !ok {
		t.Fatal("gpt-4o-mini not found")
	}
	// gpt-4o-mini input price is 0.15, gpt-4o is 2.50 — must get 0.15.
	if p.InputPerMTok != 0.15 {
		t.Errorf("expected gpt-4o-mini pricing (0.15/MTok input), got %.4f", p.InputPerMTok)
	}
}

func TestLookupPricing_CaseInsensitive(t *testing.T) {
	_, ok := lookupPricing("Claude-Opus-4")
	if !ok {
		t.Error("lookupPricing should be case-insensitive")
	}
}

func TestLookupPricing_Unknown(t *testing.T) {
	_, ok := lookupPricing("totally-unknown-model-xyz")
	if ok {
		t.Error("expected not-found for unknown model")
	}
}

func TestLookupPricing_EmptyString(t *testing.T) {
	_, ok := lookupPricing("")
	if ok {
		t.Error("expected not-found for empty model string")
	}
}

func TestLookupPricing_NewModels(t *testing.T) {
	// Verify models added in round 8 are present.
	newModels := []string{
		"claude-opus-4-5",
		"claude-sonnet-4-5",
		"claude-haiku-4",
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"o4-mini",
		"gemini-2.0-flash",
		"grok-3",
		"deepseek-r1",
		"deepseek-v3",
		"llama-4-scout",
	}
	for _, model := range newModels {
		_, ok := lookupPricing(model)
		if !ok {
			t.Errorf("expected new model %q to be in pricing table", model)
		}
	}
}

func TestLookupPricing_ProviderPrefixed(t *testing.T) {
	// Vertex, Azure, Bedrock, Nvidia entries use provider/model naming.
	prefixed := []string{
		"vertex/gemini-2.5-pro",
		"azure/gpt-4o",
		"bedrock/claude-opus-4",
		"nim/llama-3.1-70b",
	}
	for _, model := range prefixed {
		_, ok := lookupPricing(model)
		if !ok {
			t.Errorf("expected provider-prefixed model %q to be in pricing table", model)
		}
	}
}

// ---------------------------------------------------------------------------
// formatCostTable
// ---------------------------------------------------------------------------

func TestFormatCostTable_UnknownModel(t *testing.T) {
	result := formatCostTable(100, 50, 0, 0, "unknown-model-xyz")
	if !strings.Contains(result, "no pricing data") {
		t.Error("expected 'no pricing data' for unknown model")
	}
	if !strings.Contains(result, "unknown-model-xyz") {
		t.Error("expected model name in output")
	}
}

func TestFormatCostTable_KnownModel_NoCaching(t *testing.T) {
	result := formatCostTable(1000, 500, 0, 0, "gpt-4o")
	if !strings.Contains(result, "gpt-4o") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(result, "Input:") {
		t.Error("expected Input line")
	}
	if !strings.Contains(result, "Output:") {
		t.Error("expected Output line")
	}
	if !strings.Contains(result, "Total:") {
		t.Error("expected Total line")
	}
	// Should NOT show Cache lines when counts are zero.
	if strings.Contains(result, "Cache W:") {
		t.Error("should not show Cache W when cacheWrite=0")
	}
	if strings.Contains(result, "Cache R:") {
		t.Error("should not show Cache R when cacheRead=0")
	}
}

func TestFormatCostTable_WithCaching(t *testing.T) {
	// Anthropic models have cache pricing.
	result := formatCostTable(1000, 500, 200, 300, "claude-opus-4")
	if !strings.Contains(result, "Cache W:") {
		t.Error("expected Cache W line when cacheWrite > 0")
	}
	if !strings.Contains(result, "Cache R:") {
		t.Error("expected Cache R line when cacheRead > 0")
	}
}

func TestFormatCostTable_TotalIsSum(t *testing.T) {
	// With zero tokens the total should be effectively $0.
	result := formatCostTable(0, 0, 0, 0, "gpt-4o")
	if !strings.Contains(result, "Total:") {
		t.Error("expected Total line")
	}
	// The total should contain zeros.
	if !strings.Contains(result, "$0.00") && !strings.Contains(result, "$0.000000") {
		t.Errorf("expected zero total, got: %s", result)
	}
}

func TestFormatCostTable_ContainsSeparator(t *testing.T) {
	result := formatCostTable(100, 50, 0, 0, "claude-sonnet-4")
	if !strings.Contains(result, "─") {
		t.Error("expected separator line in cost table")
	}
}

func TestFormatCostTable_TokenCounts(t *testing.T) {
	result := formatCostTable(12345, 6789, 0, 0, "gpt-4o")
	if !strings.Contains(result, "12345") {
		t.Errorf("expected input token count in output: %s", result)
	}
	if !strings.Contains(result, "6789") {
		t.Errorf("expected output token count in output: %s", result)
	}
}
