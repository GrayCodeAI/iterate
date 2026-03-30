// Package context provides token budget management.
// Task 38: Context Budget manager for smart token allocation

package context

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// BudgetPriority represents the priority level of context content.
type BudgetPriority int

const (
	PriorityCritical BudgetPriority = iota // Must include (e.g., current file, error messages)
	PriorityHigh                           // Important context (e.g., related files, imports)
	PriorityMedium                         // Useful context (e.g., documentation, comments)
	PriorityLow                            // Optional context (e.g., test files, examples)
)

// ContentType represents the type of context content.
type ContentType string

const (
	ContentTypeFile        ContentType = "file"
	ContentTypeSymbol      ContentType = "symbol"
	ContentTypeImport      ContentType = "import"
	ContentTypeDoc         ContentType = "doc"
	ContentTypeError       ContentType = "error"
	ContentTypeTest        ContentType = "test"
	ContentTypeHistory     ContentType = "history"
	ContentTypeSystem      ContentType = "system"
	ContentTypeInstruction ContentType = "instruction"
)

// BudgetItem represents a single item in the context budget.
type BudgetItem struct {
	ID             string         `json:"id"`
	Type           ContentType    `json:"type"`
	Priority       BudgetPriority `json:"priority"`
	Content        string         `json:"content"`
	TokenCount     int            `json:"token_count"`
	Source         string         `json:"source,omitempty"`          // File path or source identifier
	SymbolName     string         `json:"symbol_name,omitempty"`     // For symbol items
	RelevanceScore float64        `json:"relevance_score,omitempty"` // 0.0-1.0 relevance to current task
	AddedAt        time.Time      `json:"added_at"`
	LastAccessed   time.Time      `json:"last_accessed"`
	AccessCount    int            `json:"access_count"`
}

// BudgetStats holds statistics about the context budget.
type BudgetStats struct {
	TotalTokens     int                    `json:"total_tokens"`
	MaxTokens       int                    `json:"max_tokens"`
	ReservedTokens  int                    `json:"reserved_tokens"`
	UsedTokens      int                    `json:"used_tokens"`
	AvailableTokens int                    `json:"available_tokens"`
	ItemCount       int                    `json:"item_count"`
	ByPriority      map[BudgetPriority]int `json:"by_priority"`
	ByType          map[ContentType]int    `json:"by_type"`
	Utilization     float64                `json:"utilization"` // 0.0-1.0
}

// BudgetConfig holds configuration for the budget manager.
type BudgetConfig struct {
	MaxTokens        int                        `json:"max_tokens"`      // Maximum total tokens
	ReservedTokens   int                        `json:"reserved_tokens"` // Reserved for system/response
	MinAvailable     int                        `json:"min_available"`   // Minimum tokens to keep available
	EvictionPolicy   string                     `json:"eviction_policy"` // "lru", "priority", "relevance"
	PriorityWeights  map[BudgetPriority]float64 `json:"priority_weights"`
	ContentTypeBonus map[ContentType]float64    `json:"content_type_bonus"`
}

// DefaultBudgetConfig returns default configuration.
func DefaultBudgetConfig() *BudgetConfig {
	return &BudgetConfig{
		MaxTokens:      128000, // Default context window
		ReservedTokens: 8000,   // Reserved for response
		MinAvailable:   2000,   // Keep at least 2k available
		EvictionPolicy: "priority",
		PriorityWeights: map[BudgetPriority]float64{
			PriorityCritical: 1.0,
			PriorityHigh:     0.8,
			PriorityMedium:   0.6,
			PriorityLow:      0.4,
		},
		ContentTypeBonus: map[ContentType]float64{
			ContentTypeFile:        0.2,
			ContentTypeSymbol:      0.1,
			ContentTypeImport:      0.05,
			ContentTypeDoc:         0.0,
			ContentTypeError:       0.3,
			ContentTypeTest:        -0.1,
			ContentTypeHistory:     0.1,
			ContentTypeSystem:      0.5,
			ContentTypeInstruction: 0.4,
		},
	}
}

// BudgetManager manages the token budget for context allocation.
type BudgetManager struct {
	config *BudgetConfig
	logger *slog.Logger
	items  map[string]*BudgetItem
	order  []string // Ordered list of item IDs (for eviction)
	mu     sync.RWMutex
	stats  BudgetStats
}

// NewBudgetManager creates a new budget manager.
func NewBudgetManager(config *BudgetConfig, logger *slog.Logger) *BudgetManager {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultBudgetConfig()
	}

	bm := &BudgetManager{
		config: config,
		logger: logger.With("component", "budget_manager"),
		items:  make(map[string]*BudgetItem),
		order:  make([]string, 0),
	}

	bm.updateStats()
	return bm
}

// AddItem adds an item to the budget, evicting if necessary.
func (bm *BudgetManager) AddItem(ctx context.Context, item *BudgetItem) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if item.ID == "" {
		return fmt.Errorf("item must have an ID")
	}

	item.AddedAt = time.Now()
	item.LastAccessed = time.Now()

	// Check if item already exists
	if existing, ok := bm.items[item.ID]; ok {
		// Update existing item
		*existing = *item
		bm.updateStatsLocked()
		return nil
	}

	// Check if we need to evict
	available := bm.config.MaxTokens - bm.config.ReservedTokens - bm.calculateUsedTokensLocked()
	if available < item.TokenCount+bm.config.MinAvailable {
		if !bm.evictToFit(ctx, item.TokenCount+bm.config.MinAvailable-available) {
			return fmt.Errorf("cannot fit item: need %d tokens, have %d available",
				item.TokenCount, available)
		}
	}

	// Add the item
	bm.items[item.ID] = item
	bm.order = append(bm.order, item.ID)
	bm.updateStatsLocked()

	bm.logger.Debug("Added item to budget",
		"id", item.ID,
		"type", item.Type,
		"tokens", item.TokenCount,
		"priority", item.Priority,
	)

	return nil
}

// RemoveItem removes an item from the budget.
func (bm *BudgetManager) RemoveItem(id string) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	_, ok := bm.items[id]
	if !ok {
		return false
	}

	delete(bm.items, id)

	// Remove from order
	for i, oid := range bm.order {
		if oid == id {
			bm.order = append(bm.order[:i], bm.order[i+1:]...)
			break
		}
	}

	bm.updateStatsLocked()
	return true
}

// GetItem retrieves an item and updates its access time.
func (bm *BudgetManager) GetItem(id string) *BudgetItem {
	bm.mu.RLock()
	item, ok := bm.items[id]
	bm.mu.RUnlock()

	if !ok {
		return nil
	}

	// Update access stats
	bm.mu.Lock()
	item.LastAccessed = time.Now()
	item.AccessCount++
	bm.mu.Unlock()

	return item
}

// GetContent returns all content that fits within the budget.
func (bm *BudgetManager) GetContent(ctx context.Context) string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Sort items by priority and relevance
	items := make([]*BudgetItem, 0, len(bm.items))
	for _, item := range bm.items {
		items = append(items, item)
	}

	// Sort by score (priority + relevance)
	sort.Slice(items, func(i, j int) bool {
		scoreI := bm.calculateScore(items[i])
		scoreJ := bm.calculateScore(items[j])
		return scoreI > scoreJ
	})

	// Build content within budget
	maxTokens := bm.config.MaxTokens - bm.config.ReservedTokens
	usedTokens := 0
	var builder strings.Builder

	for _, item := range items {
		select {
		case <-ctx.Done():
			break
		default:
		}

		if usedTokens+item.TokenCount > maxTokens {
			// Try to fit partial content
			remaining := maxTokens - usedTokens
			if remaining > 100 { // Only include if meaningful amount
				truncated := truncateContent(item.Content, remaining)
				builder.WriteString(fmt.Sprintf("\n--- %s (%s) [truncated] ---\n", item.ID, item.Type))
				builder.WriteString(truncated)
			}
			break
		}

		builder.WriteString(fmt.Sprintf("\n--- %s (%s) ---\n", item.ID, item.Type))
		builder.WriteString(item.Content)
		usedTokens += item.TokenCount
	}

	return builder.String()
}

// GetStats returns current budget statistics.
func (bm *BudgetManager) GetStats() BudgetStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.stats
}

// GetAvailableTokens returns the number of available tokens.
func (bm *BudgetManager) GetAvailableTokens() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.config.MaxTokens - bm.config.ReservedTokens - bm.calculateUsedTokensLocked()
}

// CanFit returns true if an item of the given size can fit.
func (bm *BudgetManager) CanFit(tokenCount int) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	available := bm.config.MaxTokens - bm.config.ReservedTokens - bm.calculateUsedTokensLocked()
	return available >= tokenCount+bm.config.MinAvailable
}

// ResizeBudget changes the maximum token budget.
func (bm *BudgetManager) ResizeBudget(maxTokens int) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if maxTokens < bm.config.ReservedTokens+bm.config.MinAvailable {
		return fmt.Errorf("max tokens too small: need at least %d",
			bm.config.ReservedTokens+bm.config.MinAvailable)
	}

	bm.config.MaxTokens = maxTokens

	// Evict if necessary
	available := maxTokens - bm.config.ReservedTokens - bm.calculateUsedTokensLocked()
	if available < bm.config.MinAvailable {
		bm.evictToFit(context.Background(), bm.config.MinAvailable-available)
	}

	bm.updateStatsLocked()
	return nil
}

// Clear removes all items from the budget.
func (bm *BudgetManager) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.items = make(map[string]*BudgetItem)
	bm.order = make([]string, 0)
	bm.updateStatsLocked()
}

// PrioritizeRelevance adjusts priorities based on relevance scores.
func (bm *BudgetManager) PrioritizeRelevance(relevantIDs map[string]float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for id, score := range relevantIDs {
		if item, ok := bm.items[id]; ok {
			item.RelevanceScore = score
		}
	}
}

// calculateUsedTokensLocked returns total tokens used (helper, must hold lock).
func (bm *BudgetManager) calculateUsedTokensLocked() int {
	total := 0
	for _, item := range bm.items {
		total += item.TokenCount
	}
	return total
}

// calculateScore calculates the priority score for an item.
func (bm *BudgetManager) calculateScore(item *BudgetItem) float64 {
	weight := bm.config.PriorityWeights[item.Priority]
	bonus := bm.config.ContentTypeBonus[item.Type]

	score := weight + bonus
	if item.RelevanceScore > 0 {
		score += item.RelevanceScore * 0.3 // Relevance bonus
	}

	// Access frequency bonus
	if item.AccessCount > 0 {
		score += float64(item.AccessCount) * 0.01
	}

	return score
}

// evictToFit evicts items to make room for the specified amount.
func (bm *BudgetManager) evictToFit(ctx context.Context, needed int) bool {
	// Sort items by eviction priority (lowest score first)
	items := make([]*BudgetItem, 0, len(bm.items))
	for _, item := range bm.items {
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		// Never evict critical items
		if items[i].Priority == PriorityCritical {
			return false
		}
		if items[j].Priority == PriorityCritical {
			return true
		}

		// Use configured eviction policy
		switch bm.config.EvictionPolicy {
		case "lru":
			return items[i].LastAccessed.Before(items[j].LastAccessed)
		case "relevance":
			return items[i].RelevanceScore < items[j].RelevanceScore
		default: // "priority"
			return bm.calculateScore(items[i]) < bm.calculateScore(items[j])
		}
	})

	freed := 0
	evicted := 0

	for _, item := range items {
		select {
		case <-ctx.Done():
			break
		default:
		}

		// Stop if we've freed enough
		if freed >= needed {
			break
		}

		// Don't evict critical items
		if item.Priority == PriorityCritical {
			continue
		}

		freed += item.TokenCount
		delete(bm.items, item.ID)

		// Remove from order
		for i, oid := range bm.order {
			if oid == item.ID {
				bm.order = append(bm.order[:i], bm.order[i+1:]...)
				break
			}
		}

		evicted++
		bm.logger.Debug("Evicted item from budget",
			"id", item.ID,
			"tokens", item.TokenCount,
			"priority", item.Priority,
		)
	}

	bm.updateStatsLocked()
	return freed >= needed
}

// updateStats updates statistics (must hold lock).
func (bm *BudgetManager) updateStats() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.updateStatsLocked()
}

// updateStatsLocked updates statistics (must hold lock).
func (bm *BudgetManager) updateStatsLocked() {
	bm.stats = BudgetStats{
		MaxTokens:      bm.config.MaxTokens,
		ReservedTokens: bm.config.ReservedTokens,
		ByPriority:     make(map[BudgetPriority]int),
		ByType:         make(map[ContentType]int),
	}

	for _, item := range bm.items {
		bm.stats.UsedTokens += item.TokenCount
		bm.stats.ByPriority[item.Priority] += item.TokenCount
		bm.stats.ByType[item.Type] += item.TokenCount
	}

	bm.stats.TotalTokens = bm.config.MaxTokens
	bm.stats.AvailableTokens = bm.config.MaxTokens - bm.config.ReservedTokens - bm.stats.UsedTokens
	bm.stats.ItemCount = len(bm.items)
	bm.stats.Utilization = float64(bm.stats.UsedTokens) / float64(bm.config.MaxTokens-bm.config.ReservedTokens)
}

// EstimateTokens estimates the token count for a string.
// Uses a simple heuristic: ~4 characters per token on average.
func EstimateTokens(content string) int {
	// Simple estimation: average ~4 chars per token
	// More accurate would require a tokenizer
	return len(content) / 4
}

// truncateContent truncates content to fit within a token limit.
func truncateContent(content string, maxTokens int) string {
	// Estimate character limit
	maxChars := maxTokens * 4

	if len(content) <= maxChars {
		return content
	}

	// Try to truncate at a sentence boundary
	truncated := content[:maxChars]

	// Find last sentence end
	lastPeriod := strings.LastIndex(truncated, ".")
	lastNewline := strings.LastIndex(truncated, "\n")

	endPos := maxChars
	if lastPeriod > maxChars-100 {
		endPos = lastPeriod + 1
	} else if lastNewline > maxChars-50 {
		endPos = lastNewline
	}

	return content[:endPos] + "\n... [truncated]"
}

// ToMarkdown generates a markdown representation of the budget state.
func (bm *BudgetManager) ToMarkdown() string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# Context Budget\n\n")
	sb.WriteString(fmt.Sprintf("- **Max Tokens**: %d\n", bm.stats.MaxTokens))
	sb.WriteString(fmt.Sprintf("- **Used Tokens**: %d\n", bm.stats.UsedTokens))
	sb.WriteString(fmt.Sprintf("- **Available Tokens**: %d\n", bm.stats.AvailableTokens))
	sb.WriteString(fmt.Sprintf("- **Utilization**: %.1f%%\n", bm.stats.Utilization*100))
	sb.WriteString(fmt.Sprintf("- **Items**: %d\n\n", bm.stats.ItemCount))

	// By priority
	sb.WriteString("## By Priority\n\n")
	sb.WriteString("| Priority | Tokens |\n")
	sb.WriteString("|----------|--------|\n")
	for p := PriorityCritical; p <= PriorityLow; p++ {
		sb.WriteString(fmt.Sprintf("| %d | %d |\n", p, bm.stats.ByPriority[p]))
	}

	// Items table
	sb.WriteString("\n## Items\n\n")
	sb.WriteString("| ID | Type | Priority | Tokens | Relevance |\n")
	sb.WriteString("|----|------|----------|--------|----------|\n")

	for _, id := range bm.order {
		item := bm.items[id]
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %.2f |\n",
			item.ID, item.Type, item.Priority, item.TokenCount, item.RelevanceScore))
	}

	return sb.String()
}
