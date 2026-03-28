// Package autonomous provides autonomous agent capabilities
package autonomous

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DecisionType represents the type of decision requiring human input
type DecisionType string

const (
	DecisionTypeAmbiguous      DecisionType = "ambiguous"       // Multiple valid options
	DecisionTypeDestructive    DecisionType = "destructive"      // Potentially harmful operation
	DecisionTypeUncertain      DecisionType = "uncertain"        // Low confidence score
	DecisionTypeConflict       DecisionType = "conflict"         // Conflicting information
	DecisionTypeSecurity       DecisionType = "security"         // Security-sensitive operation
	DecisionTypeCost           DecisionType = "cost"             // High cost operation
	DecisionTypeExternal       DecisionType = "external"         // External service interaction
	DecisionTypeApproval       DecisionType = "approval"         // Needs explicit approval
)

// UrgencyLevel represents how quickly a decision is needed
type UrgencyLevel string

const (
	UrgencyLow      UrgencyLevel = "low"      // Can wait hours
	UrgencyMedium   UrgencyLevel = "medium"   // Should be addressed soon
	UrgencyHigh     UrgencyLevel = "high"     // Blocking current operation
	UrgencyCritical UrgencyLevel = "critical" // System stuck without input
)

// DecisionOption represents a possible choice for the human
type DecisionOption struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Risk        string            `json:"risk"`         // Risk level: low/medium/high
	Recommended bool              `json:"recommended"`  // Is this the agent's recommendation?
	Confidence  float64           `json:"confidence"`   // Agent confidence in this option
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// DecisionRequest represents a request for human input
type DecisionRequest struct {
	ID            string            `json:"id"`
	Type          DecisionType      `json:"type"`
	Urgency       UrgencyLevel      `json:"urgency"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Context       string            `json:"context"`        // Additional context
	Options       []DecisionOption  `json:"options"`
	Confidence    float64           `json:"confidence"`     // Agent's overall confidence
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	SourceStep    string            `json:"source_step"`    // Which step triggered this
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// DecisionResponse represents a human's decision
type DecisionResponse struct {
	RequestID    string         `json:"request_id"`
	SelectedID   string         `json:"selected_id"`    // Which option was chosen
	CustomInput  string         `json:"custom_input,omitempty"` // Free-form input if allowed
	Reasoning    string         `json:"reasoning,omitempty"`   // Why this choice
	RespondedAt  time.Time      `json:"responded_at"`
	RespondedBy  string         `json:"responded_by"`   // User identifier
}

// TriggerCondition defines when a human-in-loop trigger should fire
type TriggerCondition struct {
	Type           DecisionType   `json:"type"`
	MinConfidence  float64        `json:"min_confidence"`  // Fire if confidence below this
	MaxConfidence  float64        `json:"max_confidence"`  // Or if confidence above this (for risky ops)
	Patterns       []string       `json:"patterns,omitempty"` // Regex patterns to match
	Keywords       []string       `json:"keywords,omitempty"` // Keywords to trigger on
	CostThreshold  float64        `json:"cost_threshold,omitempty"` // Cost in dollars
	CustomCheck    string         `json:"custom_check,omitempty"` // Named custom check function
}

// TriggerAction defines what happens when a trigger fires
type TriggerAction struct {
	AutoEscalate   bool           `json:"auto_escalate"`   // Automatically escalate urgency
	Timeout        time.Duration  `json:"timeout"`         // Time before auto-decision
	AutoDecision   string         `json:"auto_decision"`   // Default choice on timeout
	NotifyChannels []string       `json:"notify_channels"` // Where to notify (slack, email, etc.)
	BlockExecution bool           `json:"block_execution"` // Block until response
}

// HumanInLoopTrigger represents a configured trigger
type HumanInLoopTrigger struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Condition   TriggerCondition  `json:"condition"`
	Action      TriggerAction     `json:"action"`
	Priority    int               `json:"priority"`    // Higher = checked first
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// HumanInLoopConfig holds configuration for the human-in-loop system
type HumanInLoopConfig struct {
	Enabled             bool              `json:"enabled"`
	DefaultTimeout      time.Duration     `json:"default_timeout"`
	MaxPendingRequests  int               `json:"max_pending_requests"`
	AutoApproveLowRisk  bool              `json:"auto_approve_low_risk"`
	LowRiskThreshold    float64           `json:"low_risk_threshold"`
	NotifyOnRequest     bool              `json:"notify_on_request"`
	NotificationChannels []string         `json:"notification_channels"`
}

// DefaultHumanInLoopConfig returns default configuration
func DefaultHumanInLoopConfig() HumanInLoopConfig {
	return HumanInLoopConfig{
		Enabled:              true,
		DefaultTimeout:       5 * time.Minute,
		MaxPendingRequests:   100,
		AutoApproveLowRisk:   false,
		LowRiskThreshold:     0.9,
		NotifyOnRequest:      true,
		NotificationChannels: []string{"cli"},
	}
}

// HumanInLoopManager manages human-in-loop interactions
type HumanInLoopManager struct {
	mu              sync.RWMutex
	config          HumanInLoopConfig
	triggers        []*HumanInLoopTrigger
	pendingRequests map[string]*DecisionRequest
	responseChan    chan *DecisionResponse
	history         []*DecisionResponse
	confidenceCheck func(confidence float64, context map[string]any) bool
}

// NewHumanInLoopManager creates a new human-in-loop manager
func NewHumanInLoopManager(config HumanInLoopConfig) *HumanInLoopManager {
	m := &HumanInLoopManager{
		config:          config,
		triggers:        make([]*HumanInLoopTrigger, 0),
		pendingRequests: make(map[string]*DecisionRequest),
		responseChan:    make(chan *DecisionResponse, 100),
		history:         make([]*DecisionResponse, 0),
	}
	
	// Add default triggers
	m.addDefaultTriggers()
	
	return m
}

// addDefaultTriggers adds the built-in trigger conditions
func (m *HumanInLoopManager) addDefaultTriggers() {
	// Low confidence trigger
	m.AddTrigger(&HumanInLoopTrigger{
		ID:        "low-confidence",
		Name:      "Low Confidence Decision",
		Enabled:   true,
		Priority:  100,
		Condition: TriggerCondition{
			Type:          DecisionTypeUncertain,
			MinConfidence: 0.0,
			MaxConfidence: 0.6,
		},
		Action: TriggerAction{
			BlockExecution: true,
			Timeout:        m.config.DefaultTimeout,
		},
	})
	
	// Destructive operation trigger
	m.AddTrigger(&HumanInLoopTrigger{
		ID:        "destructive-op",
		Name:      "Destructive Operation Approval",
		Enabled:   true,
		Priority:  90,
		Condition: TriggerCondition{
			Type: DecisionTypeDestructive,
		},
		Action: TriggerAction{
			BlockExecution: true,
			Timeout:        m.config.DefaultTimeout,
		},
	})
	
	// Security-sensitive trigger
	m.AddTrigger(&HumanInLoopTrigger{
		ID:        "security-sensitive",
		Name:      "Security Sensitive Operation",
		Enabled:   true,
		Priority:  95,
		Condition: TriggerCondition{
			Type: DecisionTypeSecurity,
		},
		Action: TriggerAction{
			BlockExecution: true,
			Timeout:        m.config.DefaultTimeout,
		},
	})
	
	// High cost trigger
	m.AddTrigger(&HumanInLoopTrigger{
		ID:        "high-cost",
		Name:      "High Cost Operation",
		Enabled:   true,
		Priority:  80,
		Condition: TriggerCondition{
			Type:          DecisionTypeCost,
			CostThreshold: 1.0, // $1 threshold
		},
		Action: TriggerAction{
			BlockExecution: true,
			Timeout:        m.config.DefaultTimeout,
		},
	})
}

// AddTrigger adds a new trigger to the manager
func (m *HumanInLoopManager) AddTrigger(trigger *HumanInLoopTrigger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	trigger.CreatedAt = time.Now()
	trigger.UpdatedAt = time.Now()
	
	// Insert in priority order
	inserted := false
	for i, t := range m.triggers {
		if trigger.Priority > t.Priority {
			m.triggers = append(m.triggers[:i], append([]*HumanInLoopTrigger{trigger}, m.triggers[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.triggers = append(m.triggers, trigger)
	}
}

// RemoveTrigger removes a trigger by ID
func (m *HumanInLoopManager) RemoveTrigger(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i, t := range m.triggers {
		if t.ID == id {
			m.triggers = append(m.triggers[:i], m.triggers[i+1:]...)
			return true
		}
	}
	return false
}

// SetConfidenceCheck sets a custom confidence check function
func (m *HumanInLoopManager) SetConfidenceCheck(fn func(confidence float64, context map[string]any) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.confidenceCheck = fn
}

// ShouldTrigger checks if a decision should trigger human input
func (m *HumanInLoopManager) ShouldTrigger(decisionType DecisionType, confidence float64, context map[string]any) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.config.Enabled {
		return false
	}
	
	// Check custom confidence function first
	if m.confidenceCheck != nil {
		if m.confidenceCheck(confidence, context) {
			return true
		}
	}
	
	// Check triggers in priority order
	for _, trigger := range m.triggers {
		if !trigger.Enabled {
			continue
		}
		
		cond := trigger.Condition
		
		// Check type match
		if cond.Type != decisionType {
			continue
		}
		
		// Check confidence range (0,0 means any confidence is valid)
		if cond.MinConfidence != 0 || cond.MaxConfidence != 0 {
			if confidence < cond.MinConfidence || confidence > cond.MaxConfidence {
				continue
			}
		}
		
		// Check patterns if specified
		if len(cond.Patterns) > 0 {
			if desc, ok := context["description"].(string); ok {
				matched := false
				for _, pattern := range cond.Patterns {
					// Simple substring match for now
					if hilContains(desc, pattern) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		
		// Check keywords if specified
		if len(cond.Keywords) > 0 {
			if desc, ok := context["description"].(string); ok {
				matched := false
				for _, keyword := range cond.Keywords {
					if hilContains(desc, keyword) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		
		// Check cost threshold
		if cond.CostThreshold > 0 {
			if cost, ok := context["cost"].(float64); ok {
				if cost < cond.CostThreshold {
					continue
				}
			}
		}
		
		// All conditions met
		return true
	}
	
	return false
}

// RequestDecision creates a new decision request and waits for response
func (m *HumanInLoopManager) RequestDecision(ctx context.Context, request *DecisionRequest) (*DecisionResponse, error) {
	m.mu.Lock()
	
	// Check max pending requests
	if len(m.pendingRequests) >= m.config.MaxPendingRequests {
		m.mu.Unlock()
		return nil, fmt.Errorf("max pending requests (%d) reached", m.config.MaxPendingRequests)
	}
	
	// Set request metadata
	request.ID = generateRequestID()
	request.CreatedAt = time.Now()
	
	// Find matching trigger to get timeout
	var timeout time.Duration = m.config.DefaultTimeout
	for _, trigger := range m.triggers {
		if trigger.Enabled && trigger.Condition.Type == request.Type {
			if trigger.Action.Timeout > 0 {
				timeout = trigger.Action.Timeout
			}
			break
		}
	}
	
	// Set expiry
	expiry := time.Now().Add(timeout)
	request.ExpiresAt = &expiry
	
	// Store pending request
	m.pendingRequests[request.ID] = request
	
	m.mu.Unlock()
	
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Wait for response or timeout
	select {
	case response := <-m.responseChan:
		if response.RequestID == request.ID {
			m.completeRequest(request.ID, response)
			return response, nil
		}
		// Not our response, put it back
		go func() {
			m.responseChan <- response
		}()
		return nil, fmt.Errorf("received response for different request")
		
	case <-timeoutCtx.Done():
		m.mu.Lock()
		delete(m.pendingRequests, request.ID)
		m.mu.Unlock()
		
		// Check for auto-decision
		for _, trigger := range m.triggers {
			if trigger.Enabled && trigger.Condition.Type == request.Type && trigger.Action.AutoDecision != "" {
				return &DecisionResponse{
					RequestID:   request.ID,
					SelectedID:  trigger.Action.AutoDecision,
					RespondedAt: time.Now(),
					RespondedBy: "auto-timeout",
					Reasoning:   "Auto-selected due to timeout",
				}, nil
			}
		}
		
		return nil, fmt.Errorf("decision request timed out")
		
	case <-ctx.Done():
		m.mu.Lock()
		delete(m.pendingRequests, request.ID)
		m.mu.Unlock()
		return nil, ctx.Err()
	}
}

// RespondToDecision submits a decision response
func (m *HumanInLoopManager) RespondToDecision(response *DecisionResponse) error {
	m.mu.RLock()
	_, pending := m.pendingRequests[response.RequestID]
	m.mu.RUnlock()
	
	if !pending {
		return fmt.Errorf("no pending request with ID %s", response.RequestID)
	}
	
	response.RespondedAt = time.Now()
	m.responseChan <- response
	return nil
}

// completeRequest removes a pending request and stores the response in history
func (m *HumanInLoopManager) completeRequest(requestID string, response *DecisionResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.pendingRequests, requestID)
	m.history = append(m.history, response)
	
	// Limit history size
	if len(m.history) > 1000 {
		m.history = m.history[len(m.history)-1000:]
	}
}

// GetPendingRequests returns all pending decision requests
func (m *HumanInLoopManager) GetPendingRequests() []*DecisionRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	requests := make([]*DecisionRequest, 0, len(m.pendingRequests))
	for _, req := range m.pendingRequests {
		requests = append(requests, req)
	}
	return requests
}

// GetHistory returns the decision response history
func (m *HumanInLoopManager) GetHistory() []*DecisionResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*DecisionResponse, len(m.history))
	copy(result, m.history)
	return result
}

// GetTriggers returns all configured triggers
func (m *HumanInLoopManager) GetTriggers() []*HumanInLoopTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*HumanInLoopTrigger, len(m.triggers))
	copy(result, m.triggers)
	return result
}

// EnableTrigger enables or disables a trigger by ID
func (m *HumanInLoopManager) EnableTrigger(id string, enabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, t := range m.triggers {
		if t.ID == id {
			t.Enabled = enabled
			t.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// GetStats returns statistics about the human-in-loop system
func (m *HumanInLoopManager) GetStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	totalDecisions := len(m.history)
	byType := make(map[DecisionType]int)
	byUser := make(map[string]int)
	
	for _, resp := range m.history {
		// Count auto-decisions
		if resp.RespondedBy == "auto-timeout" {
			byUser["auto"]++
		} else {
			byUser[resp.RespondedBy]++
		}
	}
	
	return map[string]any{
		"enabled":            m.config.Enabled,
		"pending_requests":   len(m.pendingRequests),
		"total_decisions":    totalDecisions,
		"triggers_count":     len(m.triggers),
		"triggers_enabled":   countEnabledTriggers(m.triggers),
		"decisions_by_user":  byUser,
		"decisions_by_type":  byType,
	}
}

// Helper functions

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

func hilContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && hilContainsSubstring(s, substr)))
}

func hilContainsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func countEnabledTriggers(triggers []*HumanInLoopTrigger) int {
	count := 0
	for _, t := range triggers {
		if t.Enabled {
			count++
		}
	}
	return count
}
