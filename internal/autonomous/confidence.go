// Package autonomous - Task 4: Agent Confidence Score for autonomous decision-making
package autonomous

import (
	"fmt"
	"math"
	"strings"
	"sync"
)

// ConfidenceScore represents the agent's confidence in a decision or action.
// Range: 0.0 (no confidence) to 1.0 (absolute confidence)
type ConfidenceScore struct {
	Overall     float64            `json:"overall"`
	Components  map[string]float64 `json:"components"`
	Reasoning   string             `json:"reasoning"`
	ShouldAsk   bool               `json:"should_ask_human"`
	RiskLevel   RiskLevel          `json:"risk_level"`
}

// RiskLevel represents the assessed risk of an action.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// ConfidenceThresholds defines when to ask for human input.
type ConfidenceThresholds struct {
	AutoProceed     float64 // Confidence above this = proceed automatically
	AskHuman        float64 // Confidence below this = ask human
	RefuseAction    float64 // Confidence below this = refuse action
}

// DefaultConfidenceThresholds returns sensible defaults.
func DefaultConfidenceThresholds() ConfidenceThresholds {
	return ConfidenceThresholds{
		AutoProceed:  0.8,
		AskHuman:     0.5,
		RefuseAction: 0.2,
	}
}

// ConfidenceEngine calculates and manages confidence scores.
type ConfidenceEngine struct {
	mu          sync.RWMutex
	thresholds  ConfidenceThresholds
	history     []ConfidenceRecord
	learnings   map[string]float64 // pattern -> confidence adjustment
	maxHistory  int
}

// ConfidenceRecord tracks past confidence decisions for learning.
type ConfidenceRecord struct {
	Action      string
	Confidence  float64
	Outcome     string // "success", "failure", "partial"
	Timestamp   int64
}

// NewConfidenceEngine creates a new confidence engine.
func NewConfidenceEngine() *ConfidenceEngine {
	return &ConfidenceEngine{
		thresholds: DefaultConfidenceThresholds(),
		history:    make([]ConfidenceRecord, 0),
		learnings:  make(map[string]float64),
		maxHistory: 1000,
	}
}

// CalculateConfidence computes confidence score for an action.
func (ce *ConfidenceEngine) CalculateConfidence(action PlanStep, context ActionContext) ConfidenceScore {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	components := make(map[string]float64)

	// Component 1: Action type familiarity (0.0-1.0)
	components["action_familiarity"] = ce.assessActionFamiliarity(action.Type)

	// Component 2: Target clarity (0.0-1.0)
	components["target_clarity"] = ce.assessTargetClarity(action.Target)

	// Component 3: Context completeness (0.0-1.0)
	components["context_completeness"] = ce.assessContextCompleteness(context)

	// Component 4: Historical success rate (0.0-1.0)
	components["historical_success"] = ce.assessHistoricalSuccess(action.Type)

	// Component 5: Risk assessment (0.0-1.0, higher = lower risk)
	components["risk_assessment"] = ce.assessRisk(action, context)

	// Component 6: Dependency confidence (0.0-1.0)
	components["dependency_confidence"] = ce.assessDependencyConfidence(context)

	// Calculate weighted overall score
	overall := ce.calculateWeightedScore(components)

	// Determine risk level
	riskLevel := ce.determineRiskLevel(components["risk_assessment"])

	// Determine if we should ask human
	shouldAsk := overall < ce.thresholds.AskHuman || riskLevel == RiskCritical

	// Generate reasoning
	reasoning := ce.generateReasoning(components, overall, shouldAsk)

	return ConfidenceScore{
		Overall:    overall,
		Components: components,
		Reasoning:  reasoning,
		ShouldAsk:  shouldAsk,
		RiskLevel:  riskLevel,
	}
}

// ActionContext provides context for confidence assessment.
type ActionContext struct {
	FileExists        bool
	HasTests          bool
	IsGitRepo         bool
	HasUncommittedChanges bool
	DependencyCount   int
	RecentFailures    int
	RecentSuccesses   int
	TokensUsed        int
	MaxTokens         int
	PreviousActions   []string
	TargetFiles       []string
}

// assessActionFamiliarity evaluates how familiar the agent is with this action type.
func (ce *ConfidenceEngine) assessActionFamiliarity(actionType string) float64 {
	familiarActions := map[string]float64{
		"read":         0.95,
		"write":        0.85,
		"edit":         0.80,
		"test":         0.85,
		"build":        0.80,
		"git":          0.75,
		"run_command":  0.70,
		"create_file":  0.85,
		"delete_file":  0.60,
		"refactor":     0.65,
	}

	if score, ok := familiarActions[strings.ToLower(actionType)]; ok {
		// Apply learning adjustment
		if adjustment, exists := ce.learnings["action:"+actionType]; exists {
			score = math.Max(0, math.Min(1, score+adjustment))
		}
		return score
	}

	return 0.50 // Unknown action type
}

// assessTargetClarity evaluates how clear the target is.
func (ce *ConfidenceEngine) assessTargetClarity(target string) float64 {
	if target == "" {
		return 0.30 // No target specified
	}

	// Check for specific patterns
	if strings.Contains(target, "*") || strings.Contains(target, "?") {
		return 0.60 // Wildcard - less certain
	}

	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "./") {
		return 0.85 // Explicit path
	}

	if strings.Contains(target, ".") && len(target) > 2 {
		return 0.80 // Likely a file
	}

	return 0.70 // Reasonable target
}

// assessContextCompleteness evaluates how complete the context is.
func (ce *ConfidenceEngine) assessContextCompleteness(ctx ActionContext) float64 {
	score := 0.0

	// File existence known
	if ctx.FileExists || !ctx.FileExists {
		score += 0.20
	}

	// Git state known
	if ctx.IsGitRepo {
		score += 0.15
	}

	// Test state known
	if ctx.HasTests || !ctx.HasTests {
		score += 0.15
	}

	// Recent history available
	if len(ctx.PreviousActions) > 0 {
		score += 0.20
	}

	// Target files identified
	if len(ctx.TargetFiles) > 0 {
		score += 0.15
	}

	// Token budget known
	if ctx.MaxTokens > 0 {
		score += 0.15
	}

	return math.Min(1.0, score)
}

// assessHistoricalSuccess evaluates past success with this action type.
func (ce *ConfidenceEngine) assessHistoricalSuccess(actionType string) float64 {
	if len(ce.history) == 0 {
		return 0.60 // No history, moderate confidence
	}

	var relevant []ConfidenceRecord
	for _, record := range ce.history {
		if strings.Contains(record.Action, actionType) {
			relevant = append(relevant, record)
		}
	}

	if len(relevant) == 0 {
		return 0.60
	}

	successCount := 0
	partialCount := 0
	for _, r := range relevant {
		if r.Outcome == "success" {
			successCount++
		} else if r.Outcome == "partial" {
			partialCount++
		}
	}

	// Weighted success rate
	successRate := float64(successCount)/float64(len(relevant))*0.8 +
		float64(partialCount)/float64(len(relevant))*0.4

	return math.Min(1.0, math.Max(0.0, successRate))
}

// assessRisk evaluates the risk level of the action.
func (ce *ConfidenceEngine) assessRisk(action PlanStep, ctx ActionContext) float64 {
	riskScore := 1.0 // Start with no risk

	actionType := strings.ToLower(action.Type)

	// Deletion is always risky
	if strings.Contains(actionType, "delete") {
		riskScore -= 0.40
	}

	// Writing to files without tests
	if (strings.Contains(actionType, "write") || strings.Contains(actionType, "edit")) && !ctx.HasTests {
		riskScore -= 0.20
	}

	// Uncommitted changes
	if ctx.HasUncommittedChanges {
		riskScore -= 0.15
	}

	// High dependency count
	if ctx.DependencyCount > 5 {
		riskScore -= float64(ctx.DependencyCount-5) * 0.05
	}

	// Recent failures
	if ctx.RecentFailures > 0 {
		riskScore -= float64(ctx.RecentFailures) * 0.10
	}

	// Critical file patterns
	criticalPatterns := []string{"config", "secret", "key", "credential", ".env", "auth"}
	targetLower := strings.ToLower(action.Target)
	for _, pattern := range criticalPatterns {
		if strings.Contains(targetLower, pattern) {
			riskScore -= 0.30
			break
		}
	}

	return math.Max(0.0, math.Min(1.0, riskScore))
}

// assessDependencyConfidence evaluates confidence in dependencies.
func (ce *ConfidenceEngine) assessDependencyConfidence(ctx ActionContext) float64 {
	if ctx.DependencyCount == 0 {
		return 0.90 // No dependencies = high confidence
	}

	// More dependencies = lower confidence
	baseConfidence := 1.0 - (float64(ctx.DependencyCount) * 0.05)
	
	// Recent successes boost confidence
	if ctx.RecentSuccesses > 0 {
		baseConfidence += float64(ctx.RecentSuccesses) * 0.03
	}

	return math.Max(0.30, math.Min(1.0, baseConfidence))
}

// calculateWeightedScore computes the overall confidence score.
func (ce *ConfidenceEngine) calculateWeightedScore(components map[string]float64) float64 {
	// Define weights for each component
	weights := map[string]float64{
		"action_familiarity":    0.15,
		"target_clarity":        0.10,
		"context_completeness":  0.20,
		"historical_success":    0.20,
		"risk_assessment":       0.25,
		"dependency_confidence": 0.10,
	}

	weightedSum := 0.0
	totalWeight := 0.0

	for component, score := range components {
		if weight, ok := weights[component]; ok {
			weightedSum += score * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedSum / totalWeight
}

// determineRiskLevel converts risk score to risk level.
func (ce *ConfidenceEngine) determineRiskLevel(riskScore float64) RiskLevel {
	switch {
	case riskScore < 0.3:
		return RiskCritical
	case riskScore < 0.5:
		return RiskHigh
	case riskScore < 0.7:
		return RiskMedium
	default:
		return RiskLow
	}
}

// generateReasoning creates a human-readable explanation.
func (ce *ConfidenceEngine) generateReasoning(components map[string]float64, overall float64, shouldAsk bool) string {
	var reasons []string

	if components["action_familiarity"] < 0.6 {
		reasons = append(reasons, "unfamiliar action type")
	}
	if components["target_clarity"] < 0.6 {
		reasons = append(reasons, "unclear target")
	}
	if components["context_completeness"] < 0.5 {
		reasons = append(reasons, "incomplete context")
	}
	if components["historical_success"] < 0.5 {
		reasons = append(reasons, "poor historical success")
	}
	if components["risk_assessment"] < 0.5 {
		reasons = append(reasons, "elevated risk")
	}
	if components["dependency_confidence"] < 0.6 {
		reasons = append(reasons, "complex dependencies")
	}

	if len(reasons) == 0 {
		return fmt.Sprintf("High confidence (%.0f%%) - proceeding autonomously", overall*100)
	}

	prefix := ""
	if shouldAsk {
		prefix = "Human review recommended. "
	}

	return prefix + fmt.Sprintf("Confidence: %.0f%%. Concerns: %s", overall*100, strings.Join(reasons, ", "))
}

// RecordOutcome records the outcome of a confidence-based decision for learning.
func (ce *ConfidenceEngine) RecordOutcome(action string, confidence float64, outcome string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	record := ConfidenceRecord{
		Action:     action,
		Confidence: confidence,
		Outcome:    outcome,
		Timestamp:  0, // Would use time.Now().Unix() in production
	}

	ce.history = append(ce.history, record)

	// Trim history if needed
	if len(ce.history) > ce.maxHistory {
		ce.history = ce.history[len(ce.history)-ce.maxHistory:]
	}

	// Update learnings based on outcome
	ce.updateLearnings(record)
}

// updateLearnings adjusts confidence based on outcomes.
func (ce *ConfidenceEngine) updateLearnings(record ConfidenceRecord) {
	// Extract action type
	actionType := ""
	if parts := strings.Split(record.Action, ":"); len(parts) > 0 {
		actionType = parts[0]
	}

	key := "action:" + actionType
	
	adjustment := 0.0
	switch record.Outcome {
	case "success":
		adjustment = 0.02 // Boost confidence
	case "failure":
		adjustment = -0.05 // Reduce confidence
	case "partial":
		adjustment = -0.01 // Slight reduction
	}

	current, exists := ce.learnings[key]
	if !exists {
		current = 0.0
	}

	// Dampen adjustment based on confidence
	if record.Confidence > 0.8 && record.Outcome == "failure" {
		// High confidence failure = bigger adjustment
		adjustment *= 1.5
	}

	ce.learnings[key] = math.Max(-0.3, math.Min(0.3, current+adjustment))
}

// ShouldProceed determines if the agent should proceed with an action.
func (ce *ConfidenceEngine) ShouldProceed(score ConfidenceScore) Decision {
	// Always ask if ShouldAsk is set (e.g., for critical risk)
	if score.ShouldAsk {
		return DecisionAskHuman
	}
	
	switch {
	case score.Overall >= ce.thresholds.AutoProceed:
		return DecisionProceed
	case score.Overall < ce.thresholds.RefuseAction:
		return DecisionRefuse
	case score.Overall < ce.thresholds.AskHuman:
		return DecisionAskHuman
	default:
		return DecisionProceed
	}
}

// Decision represents the confidence-based decision.
type Decision string

const (
	DecisionProceed  Decision = "proceed"
	DecisionAskHuman Decision = "ask_human"
	DecisionRefuse   Decision = "refuse"
)

// GetConfidenceStats returns statistics about confidence decisions.
func (ce *ConfidenceEngine) GetConfidenceStats() ConfidenceStats {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	stats := ConfidenceStats{
		TotalDecisions: len(ce.history),
		LearningCount:  len(ce.learnings),
	}

	if len(ce.history) == 0 {
		return stats
	}

	var sumConfidence float64
	outcomes := make(map[string]int)

	for _, record := range ce.history {
		sumConfidence += record.Confidence
		outcomes[record.Outcome]++
	}

	stats.AverageConfidence = sumConfidence / float64(len(ce.history))
	stats.SuccessRate = float64(outcomes["success"]) / float64(len(ce.history))
	stats.LearningsActive = len(ce.learnings) > 0

	return stats
}

// ConfidenceStats contains statistics about the confidence engine.
type ConfidenceStats struct {
	TotalDecisions     int
	AverageConfidence  float64
	SuccessRate        float64
	LearningCount      int
	LearningsActive    bool
}

// SetThresholds allows customizing confidence thresholds.
func (ce *ConfidenceEngine) SetThresholds(thresholds ConfidenceThresholds) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	
	ce.thresholds = thresholds
}

// GetThresholds returns current confidence thresholds.
func (ce *ConfidenceEngine) GetThresholds() ConfidenceThresholds {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	
	return ce.thresholds
}

// ApplyLearningFromFile applies pre-trained learnings from a file.
func (ce *ConfidenceEngine) ApplyLearningFromFile(learnings map[string]float64) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	for key, adjustment := range learnings {
		ce.learnings[key] = math.Max(-0.3, math.Min(0.3, adjustment))
	}
}

// ExportLearnings returns current learnings for persistence.
func (ce *ConfidenceEngine) ExportLearnings() map[string]float64 {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	result := make(map[string]float64)
	for k, v := range ce.learnings {
		result[k] = v
	}
	return result
}
