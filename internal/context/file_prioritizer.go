// Package context provides smart file prioritization for context management.
// Task 39: Smart File Prioritization for large repositories

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

// FilePriorityConfig holds configuration for file prioritization.
type FilePriorityConfig struct {
	MaxFiles           int     `json:"max_files"`            // Maximum files to include
	MaxTokens          int     `json:"max_tokens"`           // Token budget
	FocusFileWeight    float64 `json:"focus_file_weight"`    // Weight for focus file
	DirectDepWeight    float64 `json:"direct_dep_weight"`    // Weight for direct dependencies
	TransitiveDepWeight float64 `json:"transitive_dep_weight"` // Weight for transitive dependencies
	TestFileWeight     float64 `json:"test_file_weight"`     // Weight for test files
	RecentChangeWeight float64 `json:"recent_change_weight"` // Weight for recently changed files
	SimilarityWeight   float64 `json:"similarity_weight"`    // Weight for content similarity
}

// DefaultFilePriorityConfig returns default configuration.
func DefaultFilePriorityConfig() *FilePriorityConfig {
	return &FilePriorityConfig{
		MaxFiles:           50,
		MaxTokens:          100000,
		FocusFileWeight:    1.0,
		DirectDepWeight:    0.8,
		TransitiveDepWeight: 0.5,
		TestFileWeight:     0.3,
		RecentChangeWeight: 0.2,
		SimilarityWeight:   0.4,
	}
}

// FilePriority represents a file with its priority score.
type FilePriority struct {
	Path          string    `json:"path"`
	Score         float64   `json:"score"`
	TokenEstimate int       `json:"token_estimate"`
	Reason        string    `json:"reason"`
	Dependencies  []string  `json:"dependencies,omitempty"`
	Dependents    []string  `json:"dependents,omitempty"`
	IsTest        bool      `json:"is_test"`
	LastModified  time.Time `json:"last_modified"`
}

// PrioritizationResult contains the result of file prioritization.
type PrioritizationResult struct {
	Files             []*FilePriority `json:"files"`
	TotalTokens       int             `json:"total_tokens"`
	FilesIncluded     int             `json:"files_included"`
	FilesSkipped      int             `json:"files_skipped"`
	FocusFile         string          `json:"focus_file"`
	PrioritizationTime time.Duration  `json:"prioritization_time"`
}

// FilePrioritizer intelligently prioritizes files for context inclusion.
type FilePrioritizer struct {
	config             *FilePriorityConfig
	dependencyAnalyzer *DependencyAnalyzer
	budgetManager      *BudgetManager
	repoMapGenerator   *RepoMapGenerator
	logger             *slog.Logger
	mu                 sync.RWMutex
}

// NewFilePrioritizer creates a new file prioritizer.
func NewFilePrioritizer(
	config *FilePriorityConfig,
	depAnalyzer *DependencyAnalyzer,
	budgetMgr *BudgetManager,
	repoMapGen *RepoMapGenerator,
	logger *slog.Logger,
) *FilePrioritizer {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultFilePriorityConfig()
	}
	
	return &FilePrioritizer{
		config:             config,
		dependencyAnalyzer: depAnalyzer,
		budgetManager:      budgetMgr,
		repoMapGenerator:   repoMapGen,
		logger:             logger.With("component", "file_prioritizer"),
	}
}

// PrioritizeForFile returns prioritized files for a given focus file.
func (fp *FilePrioritizer) PrioritizeForFile(ctx context.Context, focusFile string, availableFiles []string) (*PrioritizationResult, error) {
	start := time.Now()
	
	fp.mu.RLock()
	defer fp.mu.RUnlock()
	
	// Build file priorities
	priorities := make([]*FilePriority, 0, len(availableFiles))
	
	// Get dependency graph from analyzer if available
	var depGraph *DependencyGraph
	if fp.dependencyAnalyzer != nil {
		depGraph, _ = fp.dependencyAnalyzer.Analyze(ctx, ".")
	}
	
	// Calculate priority for each file
	for _, file := range availableFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		priority := fp.calculateFilePriority(file, focusFile, depGraph)
		priorities = append(priorities, priority)
	}
	
	// Sort by score (highest first)
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Score > priorities[j].Score
	})
	
	// Select files within budget
	result := &PrioritizationResult{
		FocusFile: focusFile,
		Files:     make([]*FilePriority, 0),
	}
	
	totalTokens := 0
	for _, p := range priorities {
		if totalTokens+p.TokenEstimate > fp.config.MaxTokens {
			result.FilesSkipped++
			continue
		}
		
		if len(result.Files) >= fp.config.MaxFiles {
			result.FilesSkipped++
			continue
		}
		
		result.Files = append(result.Files, p)
		totalTokens += p.TokenEstimate
	}
	
	result.TotalTokens = totalTokens
	result.FilesIncluded = len(result.Files)
	result.PrioritizationTime = time.Since(start)
	
	fp.logger.Debug("Prioritized files",
		"focus_file", focusFile,
		"included", result.FilesIncluded,
		"skipped", result.FilesSkipped,
		"tokens", totalTokens,
	)
	
	return result, nil
}

// PrioritizeForQuery returns prioritized files based on a search query.
func (fp *FilePrioritizer) PrioritizeForQuery(ctx context.Context, query string, availableFiles []string) (*PrioritizationResult, error) {
	start := time.Now()
	
	fp.mu.RLock()
	defer fp.mu.RUnlock()
	
	queryTerms := extractTerms(query)
	priorities := make([]*FilePriority, 0, len(availableFiles))
	
	for _, file := range availableFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		priority := fp.calculateQueryPriority(file, queryTerms)
		priorities = append(priorities, priority)
	}
	
	// Sort by score
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Score > priorities[j].Score
	})
	
	// Select files within budget
	result := &PrioritizationResult{
		FocusFile: "",
		Files:     make([]*FilePriority, 0),
	}
	
	totalTokens := 0
	for _, p := range priorities {
		if totalTokens+p.TokenEstimate > fp.config.MaxTokens {
			result.FilesSkipped++
			continue
		}
		
		if len(result.Files) >= fp.config.MaxFiles {
			result.FilesSkipped++
			continue
		}
		
		result.Files = append(result.Files, p)
		totalTokens += p.TokenEstimate
	}
	
	result.TotalTokens = totalTokens
	result.FilesIncluded = len(result.Files)
	result.PrioritizationTime = time.Since(start)
	
	return result, nil
}

// calculateFilePriority calculates the priority score for a file.
func (fp *FilePrioritizer) calculateFilePriority(file, focusFile string, depGraph *DependencyGraph) *FilePriority {
	priority := &FilePriority{
		Path:          file,
		Score:         0,
		TokenEstimate: 1000, // Default estimate
		IsTest:        isTestFile(file),
	}
	
	// Focus file gets highest priority
	if file == focusFile {
		priority.Score += fp.config.FocusFileWeight
		priority.Reason = "focus file"
		return priority
	}
	
	// Calculate dependency-based score
	if depGraph != nil {
		// Check if file is a direct dependency of focus file
		focusDeps := depGraph.GetDependencies(focusFile)
		for _, dep := range focusDeps {
			if dep.ToFile == file {
				priority.Score += fp.config.DirectDepWeight
				priority.Reason = "direct dependency"
				priority.Dependencies = []string{focusFile}
				break
			}
		}
		
		// Check if focus file depends on this file (transitive)
		if priority.Score == 0 {
			if transitiveDeps := fp.findTransitiveDependency(focusFile, file, depGraph, 3); len(transitiveDeps) > 0 {
				priority.Score += fp.config.TransitiveDepWeight
				priority.Reason = "transitive dependency"
				priority.Dependencies = transitiveDeps
			}
		}
		
		// Check if this file is depended upon by focus file (reverse dependency)
		fileDependents := depGraph.GetDependents(file)
		for _, dep := range fileDependents {
			if dep.FromFile == focusFile {
				priority.Score += fp.config.DirectDepWeight * 0.5
				if priority.Reason == "" {
					priority.Reason = "reverse dependency"
				}
				priority.Dependents = []string{focusFile}
				break
			}
		}
	}
	
	// Test file adjustment
	if priority.IsTest {
		// Tests for the focus file get higher priority
		if isTestForFile(file, focusFile) {
			priority.Score += fp.config.TestFileWeight * 2
		} else {
			priority.Score *= (1 - fp.config.TestFileWeight)
		}
	}
	
	// Default score for unrelated files
	if priority.Score == 0 {
		priority.Score = 0.1
		priority.Reason = "unrelated"
	}
	
	return priority
}

// calculateQueryPriority calculates priority based on query terms.
func (fp *FilePrioritizer) calculateQueryPriority(file string, queryTerms []string) *FilePriority {
	priority := &FilePriority{
		Path:          file,
		Score:         0,
		TokenEstimate: 1000,
		IsTest:        isTestFile(file),
	}
	
	// Match file path against query terms
	fileLower := strings.ToLower(file)
	matchedTerms := 0
	for _, term := range queryTerms {
		if strings.Contains(fileLower, strings.ToLower(term)) {
			matchedTerms++
		}
	}
	
	if matchedTerms > 0 {
		priority.Score = float64(matchedTerms) / float64(len(queryTerms))
		priority.Reason = "query match"
	}
	
	// Penalize test files for general queries
	if priority.IsTest {
		priority.Score *= 0.5
	}
	
	if priority.Score == 0 {
		priority.Score = 0.05
		priority.Reason = "unrelated"
	}
	
	return priority
}

// findTransitiveDependency finds the dependency path between two files.
func (fp *FilePrioritizer) findTransitiveDependency(from, to string, graph *DependencyGraph, maxDepth int) []string {
	if maxDepth <= 0 {
		return nil
	}
	
	visited := make(map[string]bool)
	path := []string{}
	
	if fp.dfsFindPath(from, to, graph, visited, &path, maxDepth) {
		return path
	}
	
	return nil
}

// dfsFindPath performs DFS to find dependency path.
func (fp *FilePrioritizer) dfsFindPath(current, target string, graph *DependencyGraph, visited map[string]bool, path *[]string, maxDepth int) bool {
	if current == target {
		return true
	}
	
	if maxDepth <= 0 || visited[current] {
		return false
	}
	
	visited[current] = true
	*path = append(*path, current)
	
	deps := graph.GetDependencies(current)
	for _, dep := range deps {
		if fp.dfsFindPath(dep.ToFile, target, graph, visited, path, maxDepth-1) {
			return true
		}
	}
	
	*path = (*path)[:len(*path)-1]
	return false
}

// UpdateConfig updates the prioritizer configuration.
func (fp *FilePrioritizer) UpdateConfig(config *FilePriorityConfig) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.config = config
}

// GetConfig returns the current configuration.
func (fp *FilePrioritizer) GetConfig() *FilePriorityConfig {
	fp.mu.RLock()
	defer fp.mu.RUnlock()
	return fp.config
}

// Helper functions

// IsTestFilePublic is a public wrapper for isTestFile (for testing).
func IsTestFilePublic(path string) bool {
	return isTestFile(path)
}

// IsTestForFilePublic is a public wrapper for isTestForFile (for testing).
func IsTestForFilePublic(testFile, focusFile string) bool {
	return isTestForFile(testFile, focusFile)
}

// ExtractTermsPublic is a public wrapper for extractTerms (for testing).
func ExtractTermsPublic(query string) []string {
	return extractTerms(query)
}

// isTestFile determines if a file is a test file.
func isTestFile(path string) bool {
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	
	// Common test file patterns
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "Test.java") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, "_test_")
}

// isTestForFile checks if a test file corresponds to the focus file.
func isTestForFile(testFile, focusFile string) bool {
	// Extract base name without extension
	focusBase := focusFile
	if idx := strings.LastIndex(focusFile, "/"); idx >= 0 {
		focusBase = focusFile[idx+1:]
	}
	if idx := strings.LastIndex(focusBase, "."); idx > 0 {
		focusBase = focusBase[:idx]
	}
	
	testBase := testFile
	if idx := strings.LastIndex(testFile, "/"); idx >= 0 {
		testBase = testFile[idx+1:]
	}
	
	// Check common test file naming patterns
	return strings.HasPrefix(testBase, focusBase+"_test") ||
		strings.HasPrefix(testBase, focusBase+".test") ||
		strings.HasPrefix(testBase, focusBase+".spec") ||
		strings.Contains(testBase, focusBase+"_test_")
}

// extractTerms extracts significant terms from a query.
func extractTerms(query string) []string {
	// Simple term extraction - split on common delimiters
	query = strings.ToLower(query)
	
	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"need": true, "dare": true, "ought": true, "used": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true,
		"and": true, "or": true, "but": true, "if": true, "then": true,
		"else": true, "when": true, "where": true, "why": true, "how": true,
		"all": true, "each": true, "every": true, "both": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "nor": true, "not": true, "only": true, "own": true,
		"same": true, "so": true, "than": true, "too": true, "very": true,
	}
	
	words := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '.' || r == ',' ||
			r == ';' || r == ':' || r == '!' || r == '?' || r == '(' || r == ')'
	})
	
	terms := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" && len(word) > 1 && !stopWords[word] {
			terms = append(terms, word)
		}
	}
	
	return terms
}

// ToMarkdown generates a markdown representation of prioritization result.
func (r *PrioritizationResult) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString("# File Prioritization Result\n\n")
	sb.WriteString("| # | File | Score | Tokens | Reason |\n")
	sb.WriteString("|---|------|-------|--------|--------|\n")
	
	for i, f := range r.Files {
		sb.WriteString(fmt.Sprintf("| %d | %s | %.2f | %d | %s |\n",
			i+1, f.Path, f.Score, f.TokenEstimate, f.Reason))
	}
	
	sb.WriteString(fmt.Sprintf("\n**Summary:** %d files included, %d skipped, %d tokens\n",
		r.FilesIncluded, r.FilesSkipped, r.TotalTokens))
	sb.WriteString(fmt.Sprintf("**Focus:** %s\n", r.FocusFile))
	sb.WriteString(fmt.Sprintf("**Time:** %v\n", r.PrioritizationTime))
	
	return sb.String()
}
