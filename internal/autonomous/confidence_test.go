package autonomous

import (
	"testing"
)

func TestConfidenceEngineCreation(t *testing.T) {
	engine := NewConfidenceEngine()
	if engine == nil {
		t.Fatal("Expected non-nil confidence engine")
	}
	if engine.thresholds.AutoProceed != 0.8 {
		t.Errorf("Expected default auto-proceed threshold 0.8, got %f", engine.thresholds.AutoProceed)
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultConfidenceThresholds()
	if thresholds.AutoProceed != 0.8 {
		t.Errorf("Expected AutoProceed 0.8, got %f", thresholds.AutoProceed)
	}
	if thresholds.AskHuman != 0.5 {
		t.Errorf("Expected AskHuman 0.5, got %f", thresholds.AskHuman)
	}
	if thresholds.RefuseAction != 0.2 {
		t.Errorf("Expected RefuseAction 0.2, got %f", thresholds.RefuseAction)
	}
}

func TestCalculateConfidenceHighConfidence(t *testing.T) {
	engine := NewConfidenceEngine()

	action := PlanStep{
		Type:   "read",
		Target: "./main.go",
	}

	context := ActionContext{
		FileExists:      true,
		HasTests:        true,
		IsGitRepo:       true,
		DependencyCount: 1,
		MaxTokens:       100000,
		TargetFiles:     []string{"main.go"},
	}

	score := engine.CalculateConfidence(action, context)

	if score.Overall < 0.7 {
		t.Errorf("Expected high confidence for read action, got %f", score.Overall)
	}
	if score.RiskLevel != RiskLow {
		t.Errorf("Expected low risk, got %s", score.RiskLevel)
	}
	if score.ShouldAsk {
		t.Error("Expected ShouldAsk=false for high confidence action")
	}
}

func TestCalculateConfidenceLowConfidence(t *testing.T) {
	engine := NewConfidenceEngine()

	action := PlanStep{
		Type:   "delete_file",
		Target: "config/.env",
	}

	context := ActionContext{
		FileExists:            true,
		HasTests:              false,
		HasUncommittedChanges: true,
		DependencyCount:       10,
		RecentFailures:        3,
	}

	score := engine.CalculateConfidence(action, context)

	if score.Overall > 0.6 {
		t.Errorf("Expected low confidence for risky action, got %f", score.Overall)
	}
	if score.RiskLevel == RiskLow {
		t.Error("Expected elevated risk level for delete action")
	}
}

func TestAssessActionFamiliarity(t *testing.T) {
	engine := NewConfidenceEngine()

	// Known action
	score := engine.assessActionFamiliarity("read")
	if score < 0.9 {
		t.Errorf("Expected high familiarity for read, got %f", score)
	}

	// Unknown action
	score = engine.assessActionFamiliarity("unknown_action")
	if score != 0.50 {
		t.Errorf("Expected 0.50 for unknown action, got %f", score)
	}
}

func TestAssessTargetClarity(t *testing.T) {
	engine := NewConfidenceEngine()

	// Empty target
	score := engine.assessTargetClarity("")
	if score > 0.5 {
		t.Errorf("Expected low clarity for empty target, got %f", score)
	}

	// Explicit path
	score = engine.assessTargetClarity("./src/main.go")
	if score < 0.8 {
		t.Errorf("Expected high clarity for explicit path, got %f", score)
	}

	// Wildcard
	score = engine.assessTargetClarity("*.go")
	if score > 0.7 {
		t.Errorf("Expected lower clarity for wildcard, got %f", score)
	}
}

func TestAssessRisk(t *testing.T) {
	engine := NewConfidenceEngine()

	// Safe read action
	readAction := PlanStep{Type: "read", Target: "main.go"}
	readCtx := ActionContext{HasTests: true}
	score := engine.assessRisk(readAction, readCtx)
	if score < 0.8 {
		t.Errorf("Expected low risk for read, got %f", score)
	}

	// Delete action
	deleteAction := PlanStep{Type: "delete_file", Target: "main.go"}
	deleteCtx := ActionContext{HasTests: false, HasUncommittedChanges: true}
	score = engine.assessRisk(deleteAction, deleteCtx)
	if score > 0.5 {
		t.Errorf("Expected high risk for delete, got %f", score)
	}

	// Critical file pattern
	criticalAction := PlanStep{Type: "edit", Target: ".env"}
	criticalCtx := ActionContext{HasTests: false}
	score = engine.assessRisk(criticalAction, criticalCtx)
	if score > 0.7 {
		t.Errorf("Expected lower risk score for critical file, got %f", score)
	}
}

func TestDetermineRiskLevel(t *testing.T) {
	engine := NewConfidenceEngine()

	if engine.determineRiskLevel(0.2) != RiskCritical {
		t.Error("Expected Critical for score 0.2")
	}
	if engine.determineRiskLevel(0.4) != RiskHigh {
		t.Error("Expected High for score 0.4")
	}
	if engine.determineRiskLevel(0.6) != RiskMedium {
		t.Error("Expected Medium for score 0.6")
	}
	if engine.determineRiskLevel(0.8) != RiskLow {
		t.Error("Expected Low for score 0.8")
	}
}

func TestShouldProceed(t *testing.T) {
	engine := NewConfidenceEngine()

	// High confidence - proceed
	highScore := ConfidenceScore{Overall: 0.9, RiskLevel: RiskLow}
	if engine.ShouldProceed(highScore) != DecisionProceed {
		t.Error("Expected Proceed for high confidence")
	}

	// Low confidence - ask human
	lowScore := ConfidenceScore{Overall: 0.4, RiskLevel: RiskMedium}
	if engine.ShouldProceed(lowScore) != DecisionAskHuman {
		t.Error("Expected AskHuman for low confidence")
	}

	// Very low confidence - refuse
	veryLowScore := ConfidenceScore{Overall: 0.1, RiskLevel: RiskHigh}
	if engine.ShouldProceed(veryLowScore) != DecisionRefuse {
		t.Error("Expected Refuse for very low confidence")
	}

	// Critical risk - always ask (ShouldAsk is set by CalculateConfidence for critical risk)
	criticalScore := ConfidenceScore{Overall: 0.9, RiskLevel: RiskCritical, ShouldAsk: true}
	if engine.ShouldProceed(criticalScore) != DecisionAskHuman {
		t.Error("Expected AskHuman for critical risk even with high confidence")
	}
}

func TestRecordOutcome(t *testing.T) {
	engine := NewConfidenceEngine()

	// Record some outcomes
	engine.RecordOutcome("read:main.go", 0.9, "success")
	engine.RecordOutcome("write:config.go", 0.7, "failure")
	engine.RecordOutcome("edit:auth.go", 0.8, "partial")

	stats := engine.GetConfidenceStats()
	if stats.TotalDecisions != 3 {
		t.Errorf("Expected 3 decisions, got %d", stats.TotalDecisions)
	}
}

func TestLearningFromOutcomes(t *testing.T) {
	engine := NewConfidenceEngine()

	// Initial familiarity for write action
	initialFamiliarity := engine.assessActionFamiliarity("write")

	// Record multiple failures for write
	for i := 0; i < 5; i++ {
		engine.RecordOutcome("write:file.go", 0.8, "failure")
	}

	// Familiarity should decrease
	newFamiliarity := engine.assessActionFamiliarity("write")
	if newFamiliarity >= initialFamiliarity {
		t.Errorf("Expected familiarity to decrease after failures, was %f now %f", initialFamiliarity, newFamiliarity)
	}

	// Record successes
	for i := 0; i < 5; i++ {
		engine.RecordOutcome("write:file.go", 0.8, "success")
	}

	// Should recover somewhat
	recoveredFamiliarity := engine.assessActionFamiliarity("write")
	if recoveredFamiliarity < newFamiliarity {
		t.Errorf("Expected familiarity to recover after successes, was %f now %f", newFamiliarity, recoveredFamiliarity)
	}
}

func TestGetConfidenceStats(t *testing.T) {
	engine := NewConfidenceEngine()

	// Empty stats
	stats := engine.GetConfidenceStats()
	if stats.TotalDecisions != 0 {
		t.Errorf("Expected 0 decisions for new engine, got %d", stats.TotalDecisions)
	}

	// Add some records
	engine.RecordOutcome("read:a.go", 0.9, "success")
	engine.RecordOutcome("read:b.go", 0.8, "success")
	engine.RecordOutcome("write:c.go", 0.6, "failure")

	stats = engine.GetConfidenceStats()
	if stats.TotalDecisions != 3 {
		t.Errorf("Expected 3 decisions, got %d", stats.TotalDecisions)
	}
	expectedAvg := (0.9 + 0.8 + 0.6) / 3
	if stats.AverageConfidence < expectedAvg-0.01 || stats.AverageConfidence > expectedAvg+0.01 {
		t.Errorf("Expected average ~%.2f, got %f", expectedAvg, stats.AverageConfidence)
	}
	expectedSuccessRate := 2.0 / 3.0
	if stats.SuccessRate < expectedSuccessRate-0.01 || stats.SuccessRate > expectedSuccessRate+0.01 {
		t.Errorf("Expected success rate ~%.2f, got %f", expectedSuccessRate, stats.SuccessRate)
	}
}

func TestSetThresholds(t *testing.T) {
	engine := NewConfidenceEngine()

	custom := ConfidenceThresholds{
		AutoProceed:  0.9,
		AskHuman:     0.6,
		RefuseAction: 0.3,
	}

	engine.SetThresholds(custom)

	thresholds := engine.GetThresholds()
	if thresholds.AutoProceed != 0.9 {
		t.Errorf("Expected AutoProceed 0.9, got %f", thresholds.AutoProceed)
	}
}

func TestExportImportLearnings(t *testing.T) {
	engine := NewConfidenceEngine()

	// Create some learnings
	engine.RecordOutcome("read:a.go", 0.9, "success")
	engine.RecordOutcome("write:b.go", 0.7, "failure")

	// Export
	learnings := engine.ExportLearnings()
	if len(learnings) == 0 {
		t.Error("Expected some learnings after recording outcomes")
	}

	// Create new engine and import
	engine2 := NewConfidenceEngine()
	engine2.ApplyLearningFromFile(learnings)

	// Should have same adjustments
	exported := engine2.ExportLearnings()
	if len(exported) != len(learnings) {
		t.Errorf("Expected %d learnings, got %d", len(learnings), len(exported))
	}
}

func TestGenerateReasoning(t *testing.T) {
	engine := NewConfidenceEngine()

	// High confidence - simple reasoning
	components := map[string]float64{
		"action_familiarity":    0.95,
		"target_clarity":        0.90,
		"context_completeness":  0.85,
		"historical_success":    0.90,
		"risk_assessment":       0.95,
		"dependency_confidence": 0.90,
	}

	reasoning := engine.generateReasoning(components, 0.92, false)
	if reasoning == "" {
		t.Error("Expected non-empty reasoning")
	}

	// Low confidence - detailed reasoning
	lowComponents := map[string]float64{
		"action_familiarity":    0.40,
		"target_clarity":        0.50,
		"context_completeness":  0.30,
		"historical_success":    0.40,
		"risk_assessment":       0.35,
		"dependency_confidence": 0.40,
	}

	reasoning = engine.generateReasoning(lowComponents, 0.38, true)
	if !containsSubstring(reasoning, "Human review") {
		t.Errorf("Expected 'Human review' in reasoning, got: %s", reasoning)
	}
}

func TestTask4FullIntegration(t *testing.T) {
	engine := NewConfidenceEngine()

	// Simulate a series of autonomous decisions
	actions := []struct {
		action  PlanStep
		context ActionContext
		outcome string
	}{
		{
			action:  PlanStep{Type: "read", Target: "main.go"},
			context: ActionContext{FileExists: true, IsGitRepo: true},
			outcome: "success",
		},
		{
			action:  PlanStep{Type: "edit", Target: "main.go"},
			context: ActionContext{FileExists: true, HasTests: true, IsGitRepo: true},
			outcome: "success",
		},
		{
			action:  PlanStep{Type: "test", Target: "main_test.go"},
			context: ActionContext{HasTests: true},
			outcome: "success",
		},
		{
			action:  PlanStep{Type: "delete_file", Target: "old_file.go"},
			context: ActionContext{HasUncommittedChanges: true},
			outcome: "partial",
		},
	}

	for _, tc := range actions {
		score := engine.CalculateConfidence(tc.action, tc.context)
		decision := engine.ShouldProceed(score)

		// Record outcome
		engine.RecordOutcome(tc.action.Type+":"+tc.action.Target, score.Overall, tc.outcome)

		t.Logf("Action: %s, Confidence: %.0f%%, Decision: %s, Risk: %s",
			tc.action.Type, score.Overall*100, decision, score.RiskLevel)

		// Verify decision logic
		if score.Overall >= 0.8 && decision != DecisionProceed {
			t.Errorf("Expected Proceed for high confidence %.2f", score.Overall)
		}
		if score.Overall < 0.2 && decision != DecisionRefuse {
			t.Errorf("Expected Refuse for low confidence %.2f", score.Overall)
		}
	}

	// Check final stats
	stats := engine.GetConfidenceStats()
	t.Logf("Final stats: %d decisions, %.0f%% avg confidence, %.0f%% success rate",
		stats.TotalDecisions, stats.AverageConfidence*100, stats.SuccessRate*100)

	if stats.TotalDecisions != 4 {
		t.Errorf("Expected 4 decisions, got %d", stats.TotalDecisions)
	}
	if stats.LearningsActive == false {
		t.Error("Expected learnings to be active after recording outcomes")
	}

	t.Log("✅ Task 4: Agent Confidence Score - Full integration PASSED")
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
