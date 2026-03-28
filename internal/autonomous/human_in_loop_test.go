package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestNewHumanInLoopManager(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	if m == nil {
		t.Fatal("expected manager, got nil")
	}
	
	if !m.config.Enabled {
		t.Error("expected enabled by default")
	}
	
	if len(m.triggers) == 0 {
		t.Error("expected default triggers to be added")
	}
}

func TestDefaultHumanInLoopConfig(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	
	if !config.Enabled {
		t.Error("expected Enabled to be true")
	}
	
	if config.DefaultTimeout != 5*time.Minute {
		t.Errorf("expected DefaultTimeout 5m, got %v", config.DefaultTimeout)
	}
	
	if config.MaxPendingRequests != 100 {
		t.Errorf("expected MaxPendingRequests 100, got %d", config.MaxPendingRequests)
	}
	
	if config.LowRiskThreshold != 0.9 {
		t.Errorf("expected LowRiskThreshold 0.9, got %f", config.LowRiskThreshold)
	}
}

func TestAddTrigger(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	initialCount := len(m.triggers)
	
	trigger := &HumanInLoopTrigger{
		ID:       "test-trigger",
		Name:     "Test Trigger",
		Enabled:  true,
		Priority: 50,
	}
	
	m.AddTrigger(trigger)
	
	if len(m.triggers) != initialCount+1 {
		t.Errorf("expected %d triggers, got %d", initialCount+1, len(m.triggers))
	}
	
	// Verify it was added
	found := false
	for _, tr := range m.triggers {
		if tr.ID == "test-trigger" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("trigger not found in list")
	}
}

func TestAddTriggerPriorityOrder(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Clear existing triggers
	m.mu.Lock()
	m.triggers = make([]*HumanInLoopTrigger, 0)
	m.mu.Unlock()
	
	// Add triggers in random order
	m.AddTrigger(&HumanInLoopTrigger{ID: "low", Priority: 10})
	m.AddTrigger(&HumanInLoopTrigger{ID: "high", Priority: 100})
	m.AddTrigger(&HumanInLoopTrigger{ID: "mid", Priority: 50})
	
	// Verify priority order (high to low)
	triggers := m.GetTriggers()
	if triggers[0].ID != "high" {
		t.Errorf("expected first trigger to be 'high', got %s", triggers[0].ID)
	}
	if triggers[1].ID != "mid" {
		t.Errorf("expected second trigger to be 'mid', got %s", triggers[1].ID)
	}
	if triggers[2].ID != "low" {
		t.Errorf("expected third trigger to be 'low', got %s", triggers[2].ID)
	}
}

func TestRemoveTrigger(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Add a trigger
	trigger := &HumanInLoopTrigger{
		ID:       "to-remove",
		Name:     "To Remove",
		Enabled:  true,
		Priority: 50,
	}
	m.AddTrigger(trigger)
	
	initialCount := len(m.triggers)
	
	// Remove it
	removed := m.RemoveTrigger("to-remove")
	if !removed {
		t.Error("expected trigger to be removed")
	}
	
	if len(m.triggers) != initialCount-1 {
		t.Errorf("expected %d triggers, got %d", initialCount-1, len(m.triggers))
	}
	
	// Try to remove non-existent
	removed = m.RemoveTrigger("non-existent")
	if removed {
		t.Error("expected false for non-existent trigger")
	}
}

func TestEnableTrigger(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Disable a trigger
	success := m.EnableTrigger("low-confidence", false)
	if !success {
		t.Error("expected to find and disable trigger")
	}
	
	// Verify it's disabled
	triggers := m.GetTriggers()
	for _, tr := range triggers {
		if tr.ID == "low-confidence" {
			if tr.Enabled {
				t.Error("expected trigger to be disabled")
			}
			break
		}
	}
	
	// Re-enable
	m.EnableTrigger("low-confidence", true)
}

func TestShouldTriggerLowConfidence(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Low confidence should trigger
	triggered := m.ShouldTrigger(DecisionTypeUncertain, 0.3, nil)
	if !triggered {
		t.Error("expected trigger for low confidence")
	}
	
	// High confidence should not trigger
	triggered = m.ShouldTrigger(DecisionTypeUncertain, 0.9, nil)
	if triggered {
		t.Error("expected no trigger for high confidence")
	}
}

func TestShouldTriggerDestructive(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Destructive operations should always trigger
	triggered := m.ShouldTrigger(DecisionTypeDestructive, 0.99, nil)
	if !triggered {
		t.Error("expected trigger for destructive operation")
	}
}

func TestShouldTriggerSecurity(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Security operations should always trigger
	triggered := m.ShouldTrigger(DecisionTypeSecurity, 0.95, nil)
	if !triggered {
		t.Error("expected trigger for security operation")
	}
}

func TestShouldTriggerCostThreshold(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Below threshold should not trigger
	triggered := m.ShouldTrigger(DecisionTypeCost, 0.8, map[string]any{"cost": 0.5})
	if triggered {
		t.Error("expected no trigger for below cost threshold")
	}
	
	// Above threshold should trigger
	triggered = m.ShouldTrigger(DecisionTypeCost, 0.8, map[string]any{"cost": 2.0})
	if !triggered {
		t.Error("expected trigger for above cost threshold")
	}
}

func TestShouldTriggerDisabledManager(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.Enabled = false
	m := NewHumanInLoopManager(config)
	
	// Should not trigger when disabled
	triggered := m.ShouldTrigger(DecisionTypeUncertain, 0.1, nil)
	if triggered {
		t.Error("expected no trigger when manager disabled")
	}
}

func TestShouldTriggerWithPatterns(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Add trigger with pattern
	trigger := &HumanInLoopTrigger{
		ID:       "pattern-trigger",
		Enabled:  true,
		Priority: 200,
		Condition: TriggerCondition{
			Type:     DecisionTypeAmbiguous,
			Patterns: []string{"delete", "remove"},
		},
	}
	m.AddTrigger(trigger)
	
	// Should trigger with matching pattern
	triggered := m.ShouldTrigger(DecisionTypeAmbiguous, 0.8, map[string]any{
		"description": "delete all files",
	})
	if !triggered {
		t.Error("expected trigger for matching pattern")
	}
	
	// Should not trigger without matching pattern
	triggered = m.ShouldTrigger(DecisionTypeAmbiguous, 0.8, map[string]any{
		"description": "create new file",
	})
	if triggered {
		t.Error("expected no trigger for non-matching pattern")
	}
}

func TestShouldTriggerWithKeywords(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Add trigger with keywords
	trigger := &HumanInLoopTrigger{
		ID:       "keyword-trigger",
		Enabled:  true,
		Priority: 200,
		Condition: TriggerCondition{
			Type:     DecisionTypeApproval,
			Keywords: []string{"deploy", "release"},
		},
	}
	m.AddTrigger(trigger)
	
	// Should trigger with matching keyword
	triggered := m.ShouldTrigger(DecisionTypeApproval, 0.8, map[string]any{
		"description": "deploy to production",
	})
	if !triggered {
		t.Error("expected trigger for matching keyword")
	}
}

func TestRequestDecisionTimeout(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.DefaultTimeout = 100 * time.Millisecond
	m := NewHumanInLoopManager(config)
	
	ctx := context.Background()
	request := &DecisionRequest{
		Type:        DecisionTypeUncertain,
		Urgency:     UrgencyHigh,
		Title:       "Test Decision",
		Description: "Testing timeout",
		Confidence:  0.3,
	}
	
	start := time.Now()
	_, err := m.RequestDecision(ctx, request)
	elapsed := time.Since(start)
	
	if err == nil {
		t.Error("expected timeout error")
	}
	
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected timeout after ~100ms, got %v", elapsed)
	}
}

func TestRequestDecisionWithResponse(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.DefaultTimeout = 5 * time.Second
	m := NewHumanInLoopManager(config)
	
	ctx := context.Background()
	request := &DecisionRequest{
		Type:        DecisionTypeUncertain,
		Urgency:     UrgencyHigh,
		Title:       "Test Decision",
		Description: "Testing response",
		Confidence:  0.3,
		Options: []DecisionOption{
			{ID: "opt1", Label: "Option 1"},
			{ID: "opt2", Label: "Option 2"},
		},
	}
	
	// Start request in goroutine
	var response *DecisionResponse
	var err error
	done := make(chan struct{})
	
	go func() {
		response, err = m.RequestDecision(ctx, request)
		close(done)
	}()
	
	// Wait for request to be pending
	time.Sleep(50 * time.Millisecond)
	
	// Get the request ID from pending
	pending := m.GetPendingRequests()
	if len(pending) == 0 {
		t.Fatal("expected pending request")
	}
	
	// Respond to it
	respErr := m.RespondToDecision(&DecisionResponse{
		RequestID:   pending[0].ID,
		SelectedID:  "opt1",
		RespondedBy: "test-user",
		Reasoning:   "Test choice",
	})
	
	if respErr != nil {
		t.Errorf("error responding: %v", respErr)
	}
	
	// Wait for request to complete
	<-done
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if response == nil {
		t.Fatal("expected response, got nil")
	}
	
	if response.SelectedID != "opt1" {
		t.Errorf("expected opt1, got %s", response.SelectedID)
	}
}

func TestRespondToDecisionInvalidID(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	err := m.RespondToDecision(&DecisionResponse{
		RequestID:  "non-existent",
		SelectedID: "opt1",
	})
	
	if err == nil {
		t.Error("expected error for non-existent request")
	}
}

func TestGetPendingRequests(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.DefaultTimeout = 5 * time.Second
	config.MaxPendingRequests = 10
	m := NewHumanInLoopManager(config)
	
	// Start multiple requests
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		go func() {
			m.RequestDecision(ctx, &DecisionRequest{
				Type:        DecisionTypeUncertain,
				Title:       "Test",
				Confidence:  0.3,
			})
		}()
	}
	
	// Wait for requests to be pending
	time.Sleep(100 * time.Millisecond)
	
	pending := m.GetPendingRequests()
	if len(pending) != 3 {
		t.Errorf("expected 3 pending requests, got %d", len(pending))
	}
}

func TestGetHistory(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	history := m.GetHistory()
	if history == nil {
		t.Error("expected history slice, got nil")
	}
}

func TestGetTriggers(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	triggers := m.GetTriggers()
	if len(triggers) == 0 {
		t.Error("expected default triggers")
	}
}

func TestHumanInLoopGetStats(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	stats := m.GetStats()
	
	if stats == nil {
		t.Fatal("expected stats map, got nil")
	}
	
	if stats["enabled"] != true {
		t.Error("expected enabled to be true")
	}
	
	if stats["triggers_count"].(int) == 0 {
		t.Error("expected triggers_count > 0")
	}
}

func TestMaxPendingRequests(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.MaxPendingRequests = 2
	config.DefaultTimeout = 5 * time.Second
	m := NewHumanInLoopManager(config)
	
	ctx := context.Background()
	
	// Fill up pending requests
	for i := 0; i < 2; i++ {
		go func() {
			m.RequestDecision(ctx, &DecisionRequest{
				Type:       DecisionTypeUncertain,
				Confidence: 0.3,
			})
		}()
	}
	
	// Wait for them to be pending
	time.Sleep(100 * time.Millisecond)
	
	// Try to add another - should fail
	_, err := m.RequestDecision(ctx, &DecisionRequest{
		Type:       DecisionTypeUncertain,
		Confidence: 0.3,
	})
	
	if err == nil {
		t.Error("expected error for max pending requests")
	}
}

func TestSetConfidenceCheck(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	m := NewHumanInLoopManager(config)
	
	// Set custom check
	m.SetConfidenceCheck(func(confidence float64, context map[string]any) bool {
		return confidence < 0.5
	})
	
	// Custom check should trigger
	triggered := m.ShouldTrigger(DecisionTypeUncertain, 0.3, nil)
	if !triggered {
		t.Error("expected custom check to trigger")
	}
	
	// Above threshold should not
	triggered = m.ShouldTrigger(DecisionTypeUncertain, 0.8, nil)
	if triggered {
		t.Error("expected custom check not to trigger")
	}
}

func TestDecisionTypeConstants(t *testing.T) {
	types := []DecisionType{
		DecisionTypeAmbiguous,
		DecisionTypeDestructive,
		DecisionTypeUncertain,
		DecisionTypeConflict,
		DecisionTypeSecurity,
		DecisionTypeCost,
		DecisionTypeExternal,
		DecisionTypeApproval,
	}
	
	for _, dt := range types {
		if dt == "" {
			t.Error("decision type should not be empty")
		}
	}
}

func TestUrgencyLevelConstants(t *testing.T) {
	levels := []UrgencyLevel{
		UrgencyLow,
		UrgencyMedium,
		UrgencyHigh,
		UrgencyCritical,
	}
	
	for _, ul := range levels {
		if ul == "" {
			t.Error("urgency level should not be empty")
		}
	}
}

func TestDecisionOptionFields(t *testing.T) {
	opt := DecisionOption{
		ID:          "test-opt",
		Label:       "Test Option",
		Description: "A test option",
		Risk:        "low",
		Recommended: true,
		Confidence:  0.9,
		Metadata:    map[string]any{"key": "value"},
	}
	
	if opt.ID != "test-opt" {
		t.Errorf("expected test-opt, got %s", opt.ID)
	}
}

func TestDecisionRequestFields(t *testing.T) {
	now := time.Now()
	req := DecisionRequest{
		ID:          "req-1",
		Type:        DecisionTypeUncertain,
		Urgency:     UrgencyHigh,
		Title:       "Test",
		Description: "Test request",
		Context:     "Some context",
		Options:     []DecisionOption{{ID: "opt1"}},
		Confidence:  0.5,
		CreatedAt:   now,
		SourceStep:  "step-1",
		Metadata:    map[string]any{"key": "value"},
	}
	
	if req.ID != "req-1" {
		t.Errorf("expected req-1, got %s", req.ID)
	}
}

func TestDecisionResponseFields(t *testing.T) {
	resp := DecisionResponse{
		RequestID:   "req-1",
		SelectedID:  "opt1",
		CustomInput: "custom",
		Reasoning:   "Because...",
		RespondedAt: time.Now(),
		RespondedBy: "user1",
	}
	
	if resp.RequestID != "req-1" {
		t.Errorf("expected req-1, got %s", resp.RequestID)
	}
}

func TestTriggerConditionFields(t *testing.T) {
	cond := TriggerCondition{
		Type:          DecisionTypeCost,
		MinConfidence: 0.0,
		MaxConfidence: 1.0,
		Patterns:      []string{"delete"},
		Keywords:      []string{"deploy"},
		CostThreshold: 5.0,
		CustomCheck:   "customFunc",
	}
	
	if cond.Type != DecisionTypeCost {
		t.Errorf("expected cost type, got %s", cond.Type)
	}
}

func TestTriggerActionFields(t *testing.T) {
	action := TriggerAction{
		AutoEscalate:   true,
		Timeout:        30 * time.Second,
		AutoDecision:   "safe-default",
		NotifyChannels: []string{"slack", "email"},
		BlockExecution: true,
	}
	
	if !action.AutoEscalate {
		t.Error("expected AutoEscalate to be true")
	}
}

func TestHumanInLoopTriggerFields(t *testing.T) {
	trigger := HumanInLoopTrigger{
		ID:        "test",
		Name:      "Test Trigger",
		Enabled:   true,
		Priority:  50,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	if trigger.ID != "test" {
		t.Errorf("expected test, got %s", trigger.ID)
	}
}

func TestHILContains(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"test", "test", true},
		{"test", "testing", false},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
	}
	
	for _, tt := range tests {
		result := hilContains(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	config := DefaultHumanInLoopConfig()
	config.DefaultTimeout = 10 * time.Second
	m := NewHumanInLoopManager(config)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel immediately
	cancel()
	
	request := &DecisionRequest{
		Type:       DecisionTypeUncertain,
		Confidence: 0.3,
	}
	
	_, err := m.RequestDecision(ctx, request)
	
	if err == nil {
		t.Error("expected context canceled error")
	}
}
