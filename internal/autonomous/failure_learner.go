// Package autonomous - Task 20: Learning from Autonomous Failures
package autonomous

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FailureLearner learns from autonomous failures to improve future operations.
// It extends SmartRetry (Task 11) with persistent learning across sessions.
type FailureLearner struct {
	mu           sync.RWMutex
	storagePath  string
	learnings    []*FailureLearning
	patterns     []*LearnedPattern
	stats        LearnerStats
	smartRetry   *SmartRetry
	enabled      bool
	maxLearnings int
}

// FailureLearning represents a learned lesson from a failure.
type FailureLearning struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	TaskType     string            `json:"task_type"`
	TaskID       string            `json:"task_id"`
	ErrorMessage string            `json:"error_message"`
	ErrorHash    string            `json:"error_hash"`
	Category     ErrorCategory     `json:"category"`
	Context      map[string]any    `json:"context,omitempty"`
	Solution     string            `json:"solution"`
	SuccessRate  float64           `json:"success_rate"`
	Attempts     int               `json:"attempts"`
	FilePatterns []string          `json:"file_patterns,omitempty"`
	Actions      []LearningAction  `json:"actions,omitempty"`
	Verified     bool              `json:"verified"`
	VerifiedAt   time.Time         `json:"verified_at,omitempty"`
}

// LearningAction represents an action taken to resolve a failure.
type LearningAction struct {
	Type        string `json:"type"`        // "code_change", "config_change", "retry", "skip", "alternative"
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	File        string `json:"file,omitempty"`
	Success     bool   `json:"success"`
}

// LearnedPattern represents a pattern extracted from multiple failures.
type LearnedPattern struct {
	ID              string        `json:"id"`
	Pattern         string        `json:"pattern"`          // Regex pattern
	Description     string        `json:"description"`
	Category        ErrorCategory `json:"category"`
	OccurrenceCount int           `json:"occurrence_count"`
	SuccessActions  []string      `json:"success_actions"`  // Actions that worked
	FailActions     []string      `json:"fail_actions"`     // Actions that didn't work
	AutoFixable     bool          `json:"auto_fixable"`
	FixTemplate     string        `json:"fix_template,omitempty"`
	Confidence      float64       `json:"confidence"`
	LastSeen        time.Time     `json:"last_seen"`
}

// LearnerStats tracks overall learning statistics.
type LearnerStats struct {
	TotalFailures     int                    `json:"total_failures"`
	TotalLearnings    int                    `json:"total_learnings"`
	PatternsLearned   int                    `json:"patterns_learned"`
	SuccessfulApplies int                    `json:"successful_applies"`
	FailedApplies     int                    `json:"failed_applies"`
	ByCategory        map[ErrorCategory]int  `json:"by_category"`
	ByTaskType        map[string]int         `json:"by_task_type"`
	LastLearning      time.Time              `json:"last_learning"`
}

// FailureLearnerConfig configures the failure learner.
type FailureLearnerConfig struct {
	StoragePath  string
	Enabled      bool
	MaxLearnings int
	SmartRetry   *SmartRetry
}

// NewFailureLearner creates a new failure learner.
func NewFailureLearner(config FailureLearnerConfig) *FailureLearner {
	if config.MaxLearnings == 0 {
		config.MaxLearnings = 10000
	}
	
	fl := &FailureLearner{
		storagePath:  config.StoragePath,
		learnings:    make([]*FailureLearning, 0),
		patterns:     make([]*LearnedPattern, 0),
		smartRetry:   config.SmartRetry,
		enabled:      config.Enabled,
		maxLearnings: config.MaxLearnings,
		stats: LearnerStats{
			ByCategory: make(map[ErrorCategory]int),
			ByTaskType: make(map[string]int),
		},
	}
	
	// Load existing learnings
	if config.StoragePath != "" {
		fl.loadLearnings()
	}
	
	return fl
}

// RecordFailure records a failure for learning.
func (fl *FailureLearner) RecordFailure(taskType, taskID, errorMsg string, context map[string]any) *FailureLearning {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	
	if !fl.enabled {
		return nil
	}
	
	// Analyze with SmartRetry if available
	category := CategoryUnknown
	var analysis *FailureAnalysis
	if fl.smartRetry != nil {
		analysis = fl.smartRetry.AnalyzeFailure(errorMsg)
		category = analysis.Category
	}
	
	learning := &FailureLearning{
		ID:           fmt.Sprintf("learn_%d", time.Now().UnixNano()),
		Timestamp:    time.Now(),
		TaskType:     taskType,
		TaskID:       taskID,
		ErrorMessage: errorMsg,
		ErrorHash:    hashError(errorMsg),
		Category:     category,
		Context:      context,
		Attempts:     1,
		SuccessRate:  0.0,
	}
	
	// Check for existing similar failure
	existing := fl.findSimilarFailure(learning.ErrorHash)
	if existing != nil {
		existing.Attempts++
		fl.stats.TotalFailures++
		fl.stats.ByCategory[category]++
		fl.stats.ByTaskType[taskType]++
		return existing
	}
	
	fl.learnings = append(fl.learnings, learning)
	fl.stats.TotalFailures++
	fl.stats.TotalLearnings++
	fl.stats.ByCategory[category]++
	fl.stats.ByTaskType[taskType]++
	fl.stats.LastLearning = time.Now()
	
	// Trim if needed
	if len(fl.learnings) > fl.maxLearnings {
		trimCount := fl.maxLearnings / 10
		if trimCount < 1 {
			trimCount = 1
		}
		if trimCount > len(fl.learnings) {
			trimCount = len(fl.learnings)
		}
		fl.learnings = fl.learnings[trimCount:]
	}
	
	// Extract patterns
	fl.extractPatterns(learning)
	
	return learning
}

// RecordSolution records a solution for a failure.
func (fl *FailureLearner) RecordSolution(learningID, solution string, actions []LearningAction) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	
	for _, learning := range fl.learnings {
		if learning.ID == learningID {
			learning.Solution = solution
			learning.Actions = actions
			
			// Update success rate based on actions
			successCount := 0
			for _, a := range actions {
				if a.Success {
					successCount++
				}
			}
			if len(actions) > 0 {
				learning.SuccessRate = float64(successCount) / float64(len(actions))
			}
			
			// Update patterns with successful actions
			fl.updatePatternActions(learning)
			
			break
		}
	}
}

// GetRecommendation returns recommendations based on learned failures.
func (fl *FailureLearner) GetRecommendation(errorMsg string) *FailureRecommendation {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	rec := &FailureRecommendation{
		ErrorHash:    hashError(errorMsg),
		Category:     CategoryUnknown,
		Confidence:   0.0,
		Suggestions:  []string{},
		Actions:      []LearningAction{},
		SimilarCases: []*FailureLearning{},
	}
	
	// Find similar failures
	for _, learning := range fl.learnings {
		if learning.ErrorHash == rec.ErrorHash {
			rec.SimilarCases = append(rec.SimilarCases, learning)
			if learning.Solution != "" {
				rec.Suggestions = append(rec.Suggestions, learning.Solution)
			}
			rec.Actions = append(rec.Actions, learning.Actions...)
		}
	}
	
	// Match learned patterns
	for _, pattern := range fl.patterns {
		if matched, _ := regexp.MatchString(pattern.Pattern, errorMsg); matched {
			rec.Category = pattern.Category
			rec.Confidence = pattern.Confidence
			rec.PatternID = pattern.ID
			
			for _, action := range pattern.SuccessActions {
				rec.Suggestions = append(rec.Suggestions, action)
			}
			
			// Add fix template if available
			if pattern.AutoFixable && pattern.FixTemplate != "" {
				rec.AutoFixable = true
				rec.FixTemplate = pattern.FixTemplate
			}
			break
		}
	}
	
	// Use SmartRetry analysis if available
	if fl.smartRetry != nil {
		analysis := fl.smartRetry.AnalyzeFailure(errorMsg)
		if rec.Category == CategoryUnknown {
			rec.Category = analysis.Category
		}
		if analysis.Confidence > rec.Confidence {
			rec.Confidence = analysis.Confidence
		}
		if len(rec.Suggestions) == 0 {
			rec.Suggestions = analysis.Suggestions
		}
	}
	
	return rec
}

// FailureRecommendation contains recommendations for handling a failure.
type FailureRecommendation struct {
	ErrorHash    string
	Category     ErrorCategory
	PatternID    string
	Confidence   float64
	AutoFixable  bool
	FixTemplate  string
	Suggestions  []string
	Actions      []LearningAction
	SimilarCases []*FailureLearning
}

// ApplyLearning applies a learned solution to a new failure.
func (fl *FailureLearner) ApplyLearning(rec *FailureRecommendation) bool {
	if len(rec.SimilarCases) == 0 {
		return false
	}
	
	// Find the most successful similar case
	bestCase := rec.SimilarCases[0]
	for _, c := range rec.SimilarCases {
		if c.SuccessRate > bestCase.SuccessRate {
			bestCase = c
		}
	}
	
	// Check if we have successful actions
	successCount := 0
	for _, a := range bestCase.Actions {
		if a.Success {
			successCount++
		}
	}
	
	fl.mu.Lock()
	if successCount > 0 {
		fl.stats.SuccessfulApplies++
	} else {
		fl.stats.FailedApplies++
	}
	fl.mu.Unlock()
	
	return successCount > 0
}

// GetStats returns learning statistics.
func (fl *FailureLearner) GetStats() LearnerStats {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.stats
}

// GetPatterns returns all learned patterns.
func (fl *FailureLearner) GetPatterns() []LearnedPattern {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	result := make([]LearnedPattern, len(fl.patterns))
	for i, p := range fl.patterns {
		result[i] = *p
	}
	return result
}

// GetRecentLearnings returns the most recent learnings.
func (fl *FailureLearner) GetRecentLearnings(limit int) []*FailureLearning {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	if limit <= 0 || limit > len(fl.learnings) {
		limit = len(fl.learnings)
	}
	
	// Return most recent
	start := len(fl.learnings) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]*FailureLearning, limit)
	copy(result, fl.learnings[start:])
	return result
}

// ExportLearnings exports learnings to JSON.
func (fl *FailureLearner) ExportLearnings() (string, error) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	data := struct {
		Learnings []*FailureLearning
		Patterns  []*LearnedPattern
		Stats     LearnerStats
	}{
		Learnings: fl.learnings,
		Patterns:  fl.patterns,
		Stats:     fl.stats,
	}
	
	b, err := json.MarshalIndent(data, "", "  ")
	return string(b), err
}

// Save persists learnings to storage.
func (fl *FailureLearner) Save() error {
	if fl.storagePath == "" {
		return nil
	}
	
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	// Ensure directory exists
	dir := filepath.Dir(fl.storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	data, err := fl.ExportLearnings()
	if err != nil {
		return err
	}
	
	return os.WriteFile(fl.storagePath, []byte(data), 0644)
}

// loadLearnings loads learnings from storage.
func (fl *FailureLearner) loadLearnings() error {
	if fl.storagePath == "" {
		return nil
	}
	
	data, err := os.ReadFile(fl.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	var loaded struct {
		Learnings []*FailureLearning
		Patterns  []*LearnedPattern
		Stats     LearnerStats
	}
	
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	
	fl.learnings = loaded.Learnings
	fl.patterns = loaded.Patterns
	fl.stats = loaded.Stats
	
	if fl.stats.ByCategory == nil {
		fl.stats.ByCategory = make(map[ErrorCategory]int)
	}
	if fl.stats.ByTaskType == nil {
		fl.stats.ByTaskType = make(map[string]int)
	}
	
	return nil
}

// findSimilarFailure finds a failure with the same error hash.
func (fl *FailureLearner) findSimilarFailure(errorHash string) *FailureLearning {
	for _, learning := range fl.learnings {
		if learning.ErrorHash == errorHash {
			return learning
		}
	}
	return nil
}

// extractPatterns extracts patterns from a failure.
func (fl *FailureLearner) extractPatterns(learning *FailureLearning) {
	// Extract file patterns from context
	if files, ok := learning.Context["files"].([]string); ok {
		learning.FilePatterns = files
	}
	
	// Try to extract a pattern from the error message
	pattern := extractErrorPattern(learning.ErrorMessage)
	if pattern == "" {
		return
	}
	
	// Check if pattern already exists
	for _, p := range fl.patterns {
		if p.Pattern == pattern {
			p.OccurrenceCount++
			p.LastSeen = time.Now()
			return
		}
	}
	
	// Add new pattern
	fl.patterns = append(fl.patterns, &LearnedPattern{
		ID:              fmt.Sprintf("pattern_%d", time.Now().UnixNano()),
		Pattern:         pattern,
		Description:     fmt.Sprintf("Auto-extracted from: %s", truncateStr(learning.ErrorMessage, 50)),
		Category:        learning.Category,
		OccurrenceCount: 1,
		SuccessActions:  []string{},
		FailActions:     []string{},
		AutoFixable:     false,
		Confidence:      0.5,
		LastSeen:        time.Now(),
	})
	fl.stats.PatternsLearned++
}

// updatePatternActions updates pattern with successful/failed actions.
func (fl *FailureLearner) updatePatternActions(learning *FailureLearning) {
	for _, pattern := range fl.patterns {
		if matched, _ := regexp.MatchString(pattern.Pattern, learning.ErrorMessage); matched {
			for _, action := range learning.Actions {
				if action.Success {
					pattern.SuccessActions = appendIfNotContains(pattern.SuccessActions, action.Description)
				} else {
					pattern.FailActions = appendIfNotContains(pattern.FailActions, action.Description)
				}
			}
			
			// Update auto-fixable status
			if len(pattern.SuccessActions) >= 3 {
				pattern.AutoFixable = true
				pattern.Confidence = float64(len(pattern.SuccessActions)) / float64(len(pattern.SuccessActions)+len(pattern.FailActions))
			}
			
			break
		}
	}
}

// hashError creates a hash of an error message.
func hashError(errorMsg string) string {
	// Normalize error message for consistent hashing
	normalized := normalizeError(errorMsg)
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:8])
}

// normalizeError normalizes an error message for pattern matching.
func normalizeError(errorMsg string) string {
	// Remove specific values (file paths, line numbers, etc.)
	normalized := errorMsg
	
	// Remove file paths
	normalized = regexp.MustCompile(`/[^\s:]+`).ReplaceAllString(normalized, "<PATH>")
	
	// Remove line numbers
	normalized = regexp.MustCompile(`:\d+`).ReplaceAllString(normalized, ":<LINE>")
	
	// Remove numbers
	normalized = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(normalized, "<NUM>")
	
	// Remove hex values
	normalized = regexp.MustCompile(`0x[a-fA-F0-9]+`).ReplaceAllString(normalized, "<HEX>")
	
	// Remove UUIDs
	normalized = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).ReplaceAllString(normalized, "<UUID>")
	
	return strings.ToLower(normalized)
}

// extractErrorPattern extracts a regex pattern from an error message.
func extractErrorPattern(errorMsg string) string {
	// Common patterns
	patterns := []struct {
		regex   string
		pattern string
	}{
		{`undefined:\s*(\w+)`, `undefined:\s*\w+`},
		{`cannot use ([^\s]+) as type ([^\s]+)`, `cannot use \S+ as type \S+`},
		{`imported and not used:\s*"([^"]+)"`, `imported and not used:\s*"[^"]+"`},
		{`nil pointer dereference`, `nil pointer dereference`},
		{`panic:\s*([^\n]+)`, `panic:\s*.+`},
		{`--- FAIL:\s*([^\s]+)`, `--- FAIL:\s*\S+`},
		{`timeout|context deadline exceeded`, `(timeout|context deadline exceeded)`},
		{`syntax error`, `syntax error`},
		{`no such file or directory`, `no such file or directory`},
		{`connection refused`, `connection refused`},
		{`DATA RACE`, `DATA RACE`},
	}
	
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p.regex, errorMsg); matched {
			return p.pattern
		}
	}
	
	return ""
}

// truncateStr truncates a string to max length.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// appendIfNotContains appends a string to a slice if not already present.
func appendIfNotContains(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// FailureLearnerBuilder helps create FailureLearner configurations.
type FailureLearnerBuilder struct {
	config FailureLearnerConfig
}

// NewFailureLearnerBuilder creates a new builder.
func NewFailureLearnerBuilder() *FailureLearnerBuilder {
	return &FailureLearnerBuilder{
		config: FailureLearnerConfig{
			Enabled:      true,
			MaxLearnings: 10000,
		},
	}
}

// WithStoragePath sets the storage path.
func (b *FailureLearnerBuilder) WithStoragePath(path string) *FailureLearnerBuilder {
	b.config.StoragePath = path
	return b
}

// WithEnabled sets whether learning is enabled.
func (b *FailureLearnerBuilder) WithEnabled(enabled bool) *FailureLearnerBuilder {
	b.config.Enabled = enabled
	return b
}

// WithMaxLearnings sets the maximum number of learnings.
func (b *FailureLearnerBuilder) WithMaxLearnings(max int) *FailureLearnerBuilder {
	b.config.MaxLearnings = max
	return b
}

// WithSmartRetry sets the SmartRetry integration.
func (b *FailureLearnerBuilder) WithSmartRetry(sr *SmartRetry) *FailureLearnerBuilder {
	b.config.SmartRetry = sr
	return b
}

// Build creates the FailureLearner.
func (b *FailureLearnerBuilder) Build() *FailureLearner {
	return NewFailureLearner(b.config)
}

// VerifyLearning marks a learning as verified.
func (fl *FailureLearner) VerifyLearning(learningID string) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	
	for _, learning := range fl.learnings {
		if learning.ID == learningID {
			learning.Verified = true
			learning.VerifiedAt = time.Now()
			break
		}
	}
}

// GetUnverifiedLearnings returns learnings that haven't been verified.
func (fl *FailureLearner) GetUnverifiedLearnings() []*FailureLearning {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	var result []*FailureLearning
	for _, learning := range fl.learnings {
		if !learning.Verified {
			result = append(result, learning)
		}
	}
	return result
}

// Clear clears all learnings.
func (fl *FailureLearner) Clear() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	
	fl.learnings = make([]*FailureLearning, 0)
	fl.patterns = make([]*LearnedPattern, 0)
	fl.stats = LearnerStats{
		ByCategory: make(map[ErrorCategory]int),
		ByTaskType: make(map[string]int),
	}
}

// GenerateReport generates a learning report.
func (fl *FailureLearner) GenerateReport() string {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	
	var sb strings.Builder
	sb.WriteString("# Failure Learning Report\n\n")
	sb.WriteString(fmt.Sprintf("**Total Failures Recorded:** %d\n", fl.stats.TotalFailures))
	sb.WriteString(fmt.Sprintf("**Total Learnings:** %d\n", fl.stats.TotalLearnings))
	sb.WriteString(fmt.Sprintf("**Patterns Learned:** %d\n", fl.stats.PatternsLearned))
	sb.WriteString(fmt.Sprintf("**Successful Applications:** %d\n", fl.stats.SuccessfulApplies))
	sb.WriteString(fmt.Sprintf("**Failed Applications:** %d\n", fl.stats.FailedApplies))
	
	if fl.stats.TotalFailures > 0 {
		sb.WriteString(fmt.Sprintf("**Apply Success Rate:** %.1f%%\n",
			float64(fl.stats.SuccessfulApplies)/float64(fl.stats.TotalFailures)*100))
	}
	
	if len(fl.patterns) > 0 {
		sb.WriteString("\n## Learned Patterns\n\n")
		for _, p := range fl.patterns {
			sb.WriteString(fmt.Sprintf("- **%s** (confidence: %.1f%%, occurrences: %d)\n",
				p.Description, p.Confidence*100, p.OccurrenceCount))
			if p.AutoFixable {
				sb.WriteString(fmt.Sprintf("  - ✅ Auto-fixable\n"))
			}
			if len(p.SuccessActions) > 0 {
				sb.WriteString(fmt.Sprintf("  - Successful actions: %s\n", strings.Join(p.SuccessActions, ", ")))
			}
		}
	}
	
	if len(fl.stats.ByCategory) > 0 {
		sb.WriteString("\n## Failures by Category\n\n")
		for cat, count := range fl.stats.ByCategory {
			sb.WriteString(fmt.Sprintf("- %s: %d\n", cat, count))
		}
	}
	
	return sb.String()
}
