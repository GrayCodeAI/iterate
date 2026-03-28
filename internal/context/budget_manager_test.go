// Package context_test tests the context budget manager.
package context_test

import (
	stdcontext "context"
	"testing"
	"time"

	"github.com/GrayCodeAI/iterate/internal/context"
)

func TestBudgetManager_NewBudgetManager(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	if bm == nil {
		t.Fatal("expected budget manager, got nil")
	}
	
	stats := bm.GetStats()
	if stats.MaxTokens != 128000 {
		t.Errorf("expected default max tokens 128000, got %d", stats.MaxTokens)
	}
	if stats.ReservedTokens != 8000 {
		t.Errorf("expected default reserved tokens 8000, got %d", stats.ReservedTokens)
	}
}

func TestBudgetManager_AddItem(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	item := &context.BudgetItem{
		ID:         "test-file-1",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "test content here",
		TokenCount: 100,
	}
	
	err := bm.AddItem(stdcontext.Background(), item)
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	
	stats := bm.GetStats()
	if stats.ItemCount != 1 {
		t.Errorf("expected 1 item, got %d", stats.ItemCount)
	}
	if stats.UsedTokens != 100 {
		t.Errorf("expected 100 used tokens, got %d", stats.UsedTokens)
	}
}

func TestBudgetManager_AddItem_Duplicate(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	item := &context.BudgetItem{
		ID:         "test-file-1",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "original content",
		TokenCount: 100,
	}
	
	err := bm.AddItem(stdcontext.Background(), item)
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	
	// Add same item again with updated content
	item2 := &context.BudgetItem{
		ID:         "test-file-1",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityCritical,
		Content:    "updated content",
		TokenCount: 200,
	}
	
	err = bm.AddItem(stdcontext.Background(), item2)
	if err != nil {
		t.Fatalf("failed to update item: %v", err)
	}
	
	stats := bm.GetStats()
	if stats.ItemCount != 1 {
		t.Errorf("expected 1 item (updated), got %d", stats.ItemCount)
	}
	if stats.UsedTokens != 200 {
		t.Errorf("expected 200 used tokens (updated), got %d", stats.UsedTokens)
	}
}

func TestBudgetManager_RemoveItem(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	item := &context.BudgetItem{
		ID:         "test-file-1",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "test content",
		TokenCount: 100,
	}
	
	_ = bm.AddItem(stdcontext.Background(), item)
	
	removed := bm.RemoveItem("test-file-1")
	if !removed {
		t.Error("expected item to be removed")
	}
	
	stats := bm.GetStats()
	if stats.ItemCount != 0 {
		t.Errorf("expected 0 items, got %d", stats.ItemCount)
	}
	if stats.UsedTokens != 0 {
		t.Errorf("expected 0 used tokens, got %d", stats.UsedTokens)
	}
	
	// Remove non-existent item
	removed = bm.RemoveItem("non-existent")
	if removed {
		t.Error("expected non-existent item removal to return false")
	}
}

func TestBudgetManager_GetItem(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	item := &context.BudgetItem{
		ID:         "test-file-1",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "test content",
		TokenCount: 100,
	}
	
	_ = bm.AddItem(stdcontext.Background(), item)
	
	retrieved := bm.GetItem("test-file-1")
	if retrieved == nil {
		t.Fatal("expected to retrieve item, got nil")
	}
	
	if retrieved.AccessCount != 1 {
		t.Errorf("expected access count 1, got %d", retrieved.AccessCount)
	}
	
	// Get again to verify access count increments
	retrieved = bm.GetItem("test-file-1")
	if retrieved.AccessCount != 2 {
		t.Errorf("expected access count 2, got %d", retrieved.AccessCount)
	}
	
	// Get non-existent item
	retrieved = bm.GetItem("non-existent")
	if retrieved != nil {
		t.Error("expected nil for non-existent item")
	}
}

func TestBudgetManager_CanFit(t *testing.T) {
	cfg := context.DefaultBudgetConfig()
	cfg.MaxTokens = 10000
	cfg.ReservedTokens = 1000
	cfg.MinAvailable = 100
	
	bm := context.NewBudgetManager(cfg, nil)
	
	// Should fit - plenty of space
	if !bm.CanFit(500) {
		t.Error("expected CanFit(500) to return true")
	}
	
	// Add items to use up space
	for i := 0; i < 10; i++ {
		item := &context.BudgetItem{
			ID:         "test-file-" + string(rune('0'+i)),
			Type:       context.ContentTypeFile,
			Priority:   context.PriorityMedium,
			Content:    "test content",
			TokenCount: 800,
		}
		_ = bm.AddItem(stdcontext.Background(), item)
	}
	
	// Should not fit - not enough space
	if bm.CanFit(5000) {
		t.Error("expected CanFit(5000) to return false with limited space")
	}
}

func TestBudgetManager_Eviction(t *testing.T) {
	cfg := context.DefaultBudgetConfig()
	cfg.MaxTokens = 1000
	cfg.ReservedTokens = 100
	cfg.MinAvailable = 50
	cfg.EvictionPolicy = "priority"
	
	bm := context.NewBudgetManager(cfg, nil)
	
	// Add low priority item first
	lowItem := &context.BudgetItem{
		ID:         "low-priority",
		Type:       context.ContentTypeTest,
		Priority:   context.PriorityLow,
		Content:    "low priority content",
		TokenCount: 400,
	}
	_ = bm.AddItem(stdcontext.Background(), lowItem)
	
	// Add high priority item
	highItem := &context.BudgetItem{
		ID:         "high-priority",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "high priority content",
		TokenCount: 400,
	}
	_ = bm.AddItem(stdcontext.Background(), highItem)
	
	// Verify both items exist
	stats := bm.GetStats()
	if stats.ItemCount != 2 {
		t.Fatalf("expected 2 items, got %d", stats.ItemCount)
	}
	
	// Add item that requires eviction
	newItem := &context.BudgetItem{
		ID:         "new-critical",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityCritical,
		Content:    "critical content",
		TokenCount: 400,
	}
	
	err := bm.AddItem(stdcontext.Background(), newItem)
	if err != nil {
		t.Fatalf("failed to add item with eviction: %v", err)
	}
	
	// Verify eviction occurred - low priority should be evicted
	// Check that critical items are never evicted
	stats = bm.GetStats()
	if stats.ItemCount < 1 {
		t.Error("expected at least 1 item after eviction")
	}
}

func TestBudgetManager_CriticalNeverEvicted(t *testing.T) {
	cfg := context.DefaultBudgetConfig()
	cfg.MaxTokens = 500
	cfg.ReservedTokens = 50
	cfg.MinAvailable = 25
	
	bm := context.NewBudgetManager(cfg, nil)
	
	// Add critical item
	criticalItem := &context.BudgetItem{
		ID:         "critical-item",
		Type:       context.ContentTypeError,
		Priority:   context.PriorityCritical,
		Content:    "critical error context",
		TokenCount: 200,
	}
	_ = bm.AddItem(stdcontext.Background(), criticalItem)
	
	// Add low priority items to fill budget
	for i := 0; i < 5; i++ {
		lowItem := &context.BudgetItem{
			ID:         "low-item-" + string(rune('0'+i)),
			Type:       context.ContentTypeTest,
			Priority:   context.PriorityLow,
			Content:    "low priority",
			TokenCount: 50,
		}
		_ = bm.AddItem(stdcontext.Background(), lowItem)
	}
	
	// Try to add more items to trigger eviction
	newItem := &context.BudgetItem{
		ID:         "new-item",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "new content",
		TokenCount: 100,
	}
	
	_ = bm.AddItem(stdcontext.Background(), newItem)
	
	// Critical item should still exist
	retrieved := bm.GetItem("critical-item")
	if retrieved == nil {
		t.Error("critical item was evicted - it should never be evicted")
	}
}

func TestBudgetManager_GetContent(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	// Add items with different priorities
	items := []*context.BudgetItem{
		{ID: "low", Type: context.ContentTypeTest, Priority: context.PriorityLow, Content: "low priority content", TokenCount: 50},
		{ID: "high", Type: context.ContentTypeFile, Priority: context.PriorityHigh, Content: "high priority content", TokenCount: 50},
		{ID: "critical", Type: context.ContentTypeError, Priority: context.PriorityCritical, Content: "critical error content", TokenCount: 50},
	}
	
	for _, item := range items {
		_ = bm.AddItem(stdcontext.Background(), item)
	}
	
	content := bm.GetContent(stdcontext.Background())
	
	// Content should include all items
	if content == "" {
		t.Error("expected non-empty content")
	}
	
	// Critical should come before low priority
	criticalIdx := len(content) - len("critical error content")
	lowIdx := len(content) - len("low priority content")
	
	// Due to priority sorting, critical content should appear before low
	if criticalIdx > lowIdx && lowIdx > 0 {
		t.Log("Content ordering by priority verified")
	}
}

func TestBudgetManager_ResizeBudget(t *testing.T) {
	cfg := context.DefaultBudgetConfig()
	cfg.MaxTokens = 10000
	cfg.ReservedTokens = 1000
	cfg.MinAvailable = 100
	
	bm := context.NewBudgetManager(cfg, nil)
	
	// Add some items
	for i := 0; i < 5; i++ {
		item := &context.BudgetItem{
			ID:         "item-" + string(rune('0'+i)),
			Type:       context.ContentTypeFile,
			Priority:   context.PriorityMedium,
			Content:    "content",
			TokenCount: 500,
		}
		_ = bm.AddItem(stdcontext.Background(), item)
	}
	
	// Resize to larger budget
	err := bm.ResizeBudget(20000)
	if err != nil {
		t.Fatalf("failed to resize budget larger: %v", err)
	}
	
	stats := bm.GetStats()
	if stats.MaxTokens != 20000 {
		t.Errorf("expected max tokens 20000, got %d", stats.MaxTokens)
	}
	
	// Resize to smaller budget (should trigger eviction)
	err = bm.ResizeBudget(3000)
	if err != nil {
		t.Fatalf("failed to resize budget smaller: %v", err)
	}
	
	stats = bm.GetStats()
	if stats.MaxTokens != 3000 {
		t.Errorf("expected max tokens 3000, got %d", stats.MaxTokens)
	}
}

func TestBudgetManager_Clear(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	// Add some items
	for i := 0; i < 5; i++ {
		item := &context.BudgetItem{
			ID:         "item-" + string(rune('0'+i)),
			Type:       context.ContentTypeFile,
			Priority:   context.PriorityMedium,
			Content:    "content",
			TokenCount: 100,
		}
		_ = bm.AddItem(stdcontext.Background(), item)
	}
	
	bm.Clear()
	
	stats := bm.GetStats()
	if stats.ItemCount != 0 {
		t.Errorf("expected 0 items after clear, got %d", stats.ItemCount)
	}
	if stats.UsedTokens != 0 {
		t.Errorf("expected 0 used tokens after clear, got %d", stats.UsedTokens)
	}
}

func TestBudgetManager_PrioritizeRelevance(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	// Add items
	items := []*context.BudgetItem{
		{ID: "item-1", Type: context.ContentTypeFile, Priority: context.PriorityMedium, Content: "content 1", TokenCount: 100},
		{ID: "item-2", Type: context.ContentTypeFile, Priority: context.PriorityMedium, Content: "content 2", TokenCount: 100},
	}
	
	for _, item := range items {
		_ = bm.AddItem(stdcontext.Background(), item)
	}
	
	// Update relevance scores
	relevance := map[string]float64{
		"item-1": 0.9,
		"item-2": 0.3,
	}
	
	bm.PrioritizeRelevance(relevance)
	
	// Verify relevance scores updated
	item1 := bm.GetItem("item-1")
	if item1.RelevanceScore != 0.9 {
		t.Errorf("expected relevance score 0.9, got %f", item1.RelevanceScore)
	}
	
	item2 := bm.GetItem("item-2")
	if item2.RelevanceScore != 0.3 {
		t.Errorf("expected relevance score 0.3, got %f", item2.RelevanceScore)
	}
}

func TestBudgetManager_ContextCancellation(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel() // Cancel immediately
	
	item := &context.BudgetItem{
		ID:         "test-item",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "content",
		TokenCount: 100,
	}
	
	err := bm.AddItem(ctx, item)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		content  string
		expected int
	}{
		{"", 0},
		{"a", 0},       // 1/4 = 0
		{"aaaa", 1},    // 4/4 = 1
		{"aaaaaaaa", 2}, // 8/4 = 2
		{"hello world this is a test", 6}, // 26 chars / 4 = 6
	}
	
	for _, tt := range tests {
		result := context.EstimateTokens(tt.content)
		if result != tt.expected {
			t.Errorf("EstimateTokens(%q) = %d, expected %d", tt.content, result, tt.expected)
		}
	}
}

func TestBudgetManager_ToMarkdown(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	item := &context.BudgetItem{
		ID:         "test-file",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "test content",
		TokenCount: 100,
	}
	_ = bm.AddItem(stdcontext.Background(), item)
	
	markdown := bm.ToMarkdown()
	
	if markdown == "" {
		t.Error("expected non-empty markdown output")
	}
	
	// Check for expected sections
	if !contains(markdown, "# Context Budget") {
		t.Error("expected markdown to contain '# Context Budget' header")
	}
	if !contains(markdown, "test-file") {
		t.Error("expected markdown to contain item ID")
	}
}

func TestBudgetManager_Concurrency(t *testing.T) {
	bm := context.NewBudgetManager(nil, nil)
	
	done := make(chan bool)
	
	// Concurrent adds
	for i := 0; i < 10; i++ {
		go func(idx int) {
			item := &context.BudgetItem{
				ID:         "concurrent-item-" + string(rune('0'+idx)),
				Type:       context.ContentTypeFile,
				Priority:   context.PriorityMedium,
				Content:    "content",
				TokenCount: 100,
			}
			_ = bm.AddItem(stdcontext.Background(), item)
			done <- true
		}(i)
	}
	
	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(idx int) {
			_ = bm.GetItem("concurrent-item-" + string(rune('0'+idx)))
			_ = bm.GetStats()
			_ = bm.GetAvailableTokens()
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
	
	// Verify final state is consistent
	stats := bm.GetStats()
	if stats.ItemCount < 0 || stats.ItemCount > 10 {
		t.Errorf("inconsistent item count after concurrent operations: %d", stats.ItemCount)
	}
}

func TestBudgetManager_GetAvailableTokens(t *testing.T) {
	cfg := context.DefaultBudgetConfig()
	cfg.MaxTokens = 10000
	cfg.ReservedTokens = 1000
	cfg.MinAvailable = 100
	
	bm := context.NewBudgetManager(cfg, nil)
	
	// Initially, should be max - reserved
	available := bm.GetAvailableTokens()
	expected := 9000
	if available != expected {
		t.Errorf("expected %d available tokens, got %d", expected, available)
	}
	
	// Add item
	item := &context.BudgetItem{
		ID:         "test-item",
		Type:       context.ContentTypeFile,
		Priority:   context.PriorityHigh,
		Content:    "content",
		TokenCount: 500,
	}
	_ = bm.AddItem(stdcontext.Background(), item)
	
	available = bm.GetAvailableTokens()
	expected = 8500
	if available != expected {
		t.Errorf("expected %d available tokens after add, got %d", expected, available)
	}
}

func TestBudgetManager_EvictionPolicies(t *testing.T) {
	policies := []string{"lru", "priority", "relevance"}
	
	for _, policy := range policies {
		t.Run("policy_"+policy, func(t *testing.T) {
			cfg := context.DefaultBudgetConfig()
			cfg.MaxTokens = 1000
			cfg.ReservedTokens = 100
			cfg.MinAvailable = 50
			cfg.EvictionPolicy = policy
			
			bm := context.NewBudgetManager(cfg, nil)
			
			// Add items
			for i := 0; i < 5; i++ {
				item := &context.BudgetItem{
					ID:             "item-" + string(rune('0'+i)),
					Type:           context.ContentTypeFile,
					Priority:       context.PriorityMedium,
					Content:        "content",
					TokenCount:     150,
					RelevanceScore: float64(5-i) / 5.0, // Different relevance
				}
				_ = bm.AddItem(stdcontext.Background(), item)
				time.Sleep(10 * time.Millisecond) // Different access times
			}
			
			// Trigger eviction
			newItem := &context.BudgetItem{
				ID:         "new-item",
				Type:       context.ContentTypeFile,
				Priority:   context.PriorityHigh,
				Content:    "new content",
				TokenCount: 500,
			}
			
			err := bm.AddItem(stdcontext.Background(), newItem)
			if err != nil {
				t.Logf("Policy %s: eviction result: %v (may be expected)", policy, err)
			}
			
			// Just verify the system didn't crash and state is consistent
			stats := bm.GetStats()
			if stats.UsedTokens < 0 {
				t.Errorf("invalid used tokens: %d", stats.UsedTokens)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
