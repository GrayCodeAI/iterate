package main

import (
	"fmt"
	"strings"
)

// ModelPricing holds per-million-token prices for a model.
type ModelPricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// knownPricing maps model name substrings to pricing (USD per million tokens).
var knownPricing = map[string]ModelPricing{
	// ── Anthropic Claude ──────────────────────────────────────────────────────
	"claude-opus-4-5":   {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4":     {15.0, 75.0, 18.75, 1.50},
	"claude-sonnet-4-5": {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4":   {3.0, 15.0, 3.75, 0.30},
	"claude-haiku-4":    {0.80, 4.0, 1.0, 0.08},
	"claude-haiku-3-5":  {0.80, 4.0, 1.0, 0.08},
	"claude-haiku-3":    {0.25, 1.25, 0.30, 0.03},
	"claude-3-opus":     {15.0, 75.0, 18.75, 1.50},
	"claude-3-sonnet":   {3.0, 15.0, 3.75, 0.30},
	"claude-3-haiku":    {0.25, 1.25, 0.30, 0.03},

	// ── OpenAI ────────────────────────────────────────────────────────────────
	"gpt-4o-mini":  {0.15, 0.60, 0, 0},
	"gpt-4o":       {2.50, 10.0, 0, 0},
	"gpt-4-turbo":  {10.0, 30.0, 0, 0},
	"gpt-4.1-mini": {0.40, 1.60, 0, 0},
	"gpt-4.1-nano": {0.10, 0.40, 0, 0},
	"gpt-4.1":      {2.00, 8.0, 0, 0},
	"o1-mini":      {3.0, 12.0, 0, 0},
	"o1":           {15.0, 60.0, 0, 0},
	"o3-mini":      {1.10, 4.40, 0, 0},
	"o3":           {10.0, 40.0, 0, 0},
	"o4-mini":      {1.10, 4.40, 0, 0},

	// ── Google Gemini ─────────────────────────────────────────────────────────
	"gemini-2.5-pro":   {1.25, 10.0, 0, 0},
	"gemini-2.5-flash": {0.075, 0.30, 0, 0},
	"gemini-2.0-flash": {0.10, 0.40, 0, 0},
	"gemini-2.0-pro":   {1.25, 5.0, 0, 0},
	"gemini-1.5-pro":   {1.25, 5.0, 0, 0},
	"gemini-1.5-flash": {0.075, 0.30, 0, 0},

	// ── Groq ──────────────────────────────────────────────────────────────────
	"llama-3.3-70b":    {0.59, 0.79, 0, 0},
	"llama-3.1-8b":     {0.05, 0.08, 0, 0},
	"llama-4-scout":    {0.11, 0.34, 0, 0},
	"llama-4-maverick": {0.50, 0.77, 0, 0},
	"mixtral-8x7b":     {0.24, 0.24, 0, 0},
	"gemma2-9b":        {0.20, 0.20, 0, 0},

	// ── xAI Grok ──────────────────────────────────────────────────────────────
	"grok-3-mini": {0.30, 0.50, 0, 0},
	"grok-3":      {3.0, 15.0, 0, 0},
	"grok-2":      {2.0, 10.0, 0, 0},
	"grok-beta":   {5.0, 15.0, 0, 0},

	// ── Mistral ───────────────────────────────────────────────────────────────
	"mistral-large": {3.0, 9.0, 0, 0},
	"mistral-small": {0.20, 0.60, 0, 0},
	"codestral":     {0.20, 0.60, 0, 0},
	"devstral":      {0.10, 0.30, 0, 0},

	// ── DeepSeek ──────────────────────────────────────────────────────────────
	"deepseek-r1":    {0.55, 2.19, 0, 0},
	"deepseek-v3":    {0.07, 1.10, 0, 0},
	"deepseek-chat":  {0.07, 1.10, 0, 0},
	"deepseek-coder": {0.07, 1.10, 0, 0},

	// ── Google Vertex AI ──────────────────────────────────────────────────────
	// Vertex prices match Gemini API prices but are billed via GCP.
	"vertex/gemini-2.5-pro":   {1.25, 10.0, 0, 0},
	"vertex/gemini-2.5-flash": {0.075, 0.30, 0, 0},
	"vertex/gemini-1.5-pro":   {1.25, 5.0, 0, 0},
	"vertex/gemini-1.5-flash": {0.075, 0.30, 0, 0},

	// ── Azure OpenAI ──────────────────────────────────────────────────────────
	// Azure typically matches OpenAI list prices; exact rates vary by agreement.
	"azure/gpt-4o":      {2.50, 10.0, 0, 0},
	"azure/gpt-4o-mini": {0.15, 0.60, 0, 0},
	"azure/gpt-4":       {10.0, 30.0, 0, 0},

	// ── AWS Bedrock ───────────────────────────────────────────────────────────
	// On-demand inference prices.
	"bedrock/claude-opus-4":      {15.0, 75.0, 0, 0},
	"bedrock/claude-sonnet-4":    {3.0, 15.0, 0, 0},
	"bedrock/claude-haiku-3-5":   {0.80, 4.0, 0, 0},
	"bedrock/claude-haiku-3":     {0.25, 1.25, 0, 0},
	"bedrock/titan-text-lite":    {0.30, 0.40, 0, 0},
	"bedrock/titan-text-express": {0.80, 1.60, 0, 0},
	"bedrock/llama3-70b":         {2.65, 3.50, 0, 0},
	"bedrock/mistral-large":      {4.0, 12.0, 0, 0},

	// ── Nvidia NIM ────────────────────────────────────────────────────────────
	"nim/llama-3.1-405b": {5.0, 16.0, 0, 0},
	"nim/llama-3.1-70b":  {0.97, 0.97, 0, 0},
	"nim/llama-3.1-8b":   {0.20, 0.20, 0, 0},
	"nim/mistral-7b":     {0.15, 0.15, 0, 0},
	"nim/mixtral-8x22b":  {1.58, 1.58, 0, 0},
}

// lookupPricing returns the best pricing match for a model name (substring match, longest wins).
func lookupPricing(model string) (ModelPricing, bool) {
	modelLower := strings.ToLower(model)
	if p, ok := knownPricing[modelLower]; ok {
		return p, true
	}
	best := ""
	var bestP ModelPricing
	for k, v := range knownPricing {
		if strings.Contains(modelLower, strings.ToLower(k)) && len(k) > len(best) {
			best = k
			bestP = v
		}
	}
	if best != "" {
		return bestP, true
	}
	return ModelPricing{}, false
}

// formatCostTable returns a detailed cost breakdown string.
func formatCostTable(inputTokens, outputTokens, cacheWrite, cacheRead int, model string) string {
	p, ok := lookupPricing(model)
	if !ok {
		return fmt.Sprintf(
			"  Model:   %s\n  Input:   ~%d tokens\n  Output:  ~%d tokens\n  (no pricing data for this model)\n",
			model, inputTokens, outputTokens)
	}

	inputCost := float64(inputTokens) / 1e6 * p.InputPerMTok
	outputCost := float64(outputTokens) / 1e6 * p.OutputPerMTok
	cacheWriteCost := float64(cacheWrite) / 1e6 * p.CacheWritePerMTok
	cacheReadCost := float64(cacheRead) / 1e6 * p.CacheReadPerMTok
	total := inputCost + outputCost + cacheWriteCost + cacheReadCost

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Model:   %s\n", model))
	sb.WriteString(fmt.Sprintf("  Input:   %7d tokens  ($%.3f/MTok) → $%.5f\n",
		inputTokens, p.InputPerMTok, inputCost))
	sb.WriteString(fmt.Sprintf("  Output:  %7d tokens  ($%.3f/MTok) → $%.5f\n",
		outputTokens, p.OutputPerMTok, outputCost))
	if cacheWrite > 0 {
		sb.WriteString(fmt.Sprintf("  Cache W: %7d tokens  ($%.3f/MTok) → $%.5f\n",
			cacheWrite, p.CacheWritePerMTok, cacheWriteCost))
	}
	if cacheRead > 0 {
		sb.WriteString(fmt.Sprintf("  Cache R: %7d tokens  ($%.4f/MTok) → $%.5f\n",
			cacheRead, p.CacheReadPerMTok, cacheReadCost))
	}
	sb.WriteString("  ─────────────────────────────────────────────────\n")
	if total < 0.001 {
		sb.WriteString(fmt.Sprintf("  Total:   $%.6f\n", total))
	} else {
		sb.WriteString(fmt.Sprintf("  Total:   $%.4f\n", total))
	}
	return sb.String()
}
