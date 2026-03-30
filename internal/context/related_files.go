// Package context provides related file suggestions based on dependency analysis.
// Task 40: Related Files auto-suggestion based on imports

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

// RelatedFileReason describes why a file is related.
type RelatedFileReason string

const (
	RelatedReasonDirectImport RelatedFileReason = "direct_import"
	RelatedReasonTransitive   RelatedFileReason = "transitive_dependency"
	RelatedReasonReverseDep   RelatedFileReason = "reverse_dependency"
	RelatedReasonSamePackage  RelatedFileReason = "same_package"
	RelatedReasonTestForFile  RelatedFileReason = "test_for_file"
	RelatedReasonTestedbyFile RelatedFileReason = "tested_by_file"
	RelatedReasonSharedImport RelatedFileReason = "shared_import"
	RelatedReasonSimilarName  RelatedFileReason = "similar_name"
)

// RelatedFile represents a file related to a focus file.
type RelatedFile struct {
	Path          string              `json:"path"`
	Score         float64             `json:"score"`
	Reasons       []RelatedFileReason `json:"reasons"`
	ReasonDetails []string            `json:"reason_details,omitempty"`
	TokenEstimate int                 `json:"token_estimate"`
	IsTest        bool                `json:"is_test"`
	Distance      int                 `json:"distance"` // Dependency distance (1=direct, 2=transitive)
}

// RelatedFilesResult contains the result of a related files query.
type RelatedFilesResult struct {
	FocusFile    string         `json:"focus_file"`
	RelatedFiles []*RelatedFile `json:"related_files"`
	TotalFound   int            `json:"total_found"`
	QueryTime    time.Duration  `json:"query_time"`
	MaxDepth     int            `json:"max_depth"`
}

// RelatedFilesConfig holds configuration for related file suggestions.
type RelatedFilesConfig struct {
	MaxResults      int     `json:"max_results"`
	MaxDepth        int     `json:"max_depth"`
	MinScore        float64 `json:"min_score"`
	IncludeTests    bool    `json:"include_tests"`
	IncludeExternal bool    `json:"include_external"`
	ScoreThreshold  float64 `json:"score_threshold"`
}

// DefaultRelatedFilesConfig returns default configuration.
func DefaultRelatedFilesConfig() *RelatedFilesConfig {
	return &RelatedFilesConfig{
		MaxResults:      20,
		MaxDepth:        3,
		MinScore:        0.1,
		IncludeTests:    true,
		IncludeExternal: false,
		ScoreThreshold:  0.3,
	}
}

// RelatedFilesSuggester suggests related files based on dependency analysis.
type RelatedFilesSuggester struct {
	config             *RelatedFilesConfig
	dependencyAnalyzer *DependencyAnalyzer
	logger             *slog.Logger
	mu                 sync.RWMutex

	// Cache for performance
	cache   map[string]*RelatedFilesResult
	cacheMu sync.RWMutex
}

// NewRelatedFilesSuggester creates a new related files suggester.
func NewRelatedFilesSuggester(
	config *RelatedFilesConfig,
	depAnalyzer *DependencyAnalyzer,
	logger *slog.Logger,
) *RelatedFilesSuggester {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultRelatedFilesConfig()
	}

	return &RelatedFilesSuggester{
		config:             config,
		dependencyAnalyzer: depAnalyzer,
		logger:             logger.With("component", "related_files_suggester"),
		cache:              make(map[string]*RelatedFilesResult),
	}
}

// SuggestRelatedFiles returns files related to the given focus file.
func (rfs *RelatedFilesSuggester) SuggestRelatedFiles(ctx context.Context, focusFile string) (*RelatedFilesResult, error) {
	start := time.Now()

	// Check cache
	rfs.cacheMu.RLock()
	if cached, ok := rfs.cache[focusFile]; ok {
		rfs.cacheMu.RUnlock()
		return cached, nil
	}
	rfs.cacheMu.RUnlock()

	rfs.mu.RLock()
	defer rfs.mu.RUnlock()

	result := &RelatedFilesResult{
		FocusFile:    focusFile,
		RelatedFiles: make([]*RelatedFile, 0),
		MaxDepth:     rfs.config.MaxDepth,
	}

	// Get dependency graph
	var depGraph *DependencyGraph
	if rfs.dependencyAnalyzer != nil {
		var err error
		depGraph, err = rfs.dependencyAnalyzer.Analyze(ctx, ".")
		if err != nil {
			rfs.logger.Debug("Failed to analyze dependencies", "error", err)
		}
	}

	// Build related files map
	relatedMap := make(map[string]*RelatedFile)

	// Add focus file's direct dependencies
	if depGraph != nil {
		rfs.addDirectDependencies(focusFile, depGraph, relatedMap)
		rfs.addReverseDependencies(focusFile, depGraph, relatedMap)
		rfs.addTransitiveDependencies(focusFile, depGraph, relatedMap)
		rfs.addSharedImportFiles(focusFile, depGraph, relatedMap)
	}

	// Add test file relationships
	if rfs.config.IncludeTests {
		rfs.addTestRelationships(focusFile, relatedMap)
	}

	// Convert to slice and sort
	for _, rf := range relatedMap {
		if rf.Score >= rfs.config.MinScore {
			result.RelatedFiles = append(result.RelatedFiles, rf)
		}
	}

	// Sort by score
	sort.Slice(result.RelatedFiles, func(i, j int) bool {
		return result.RelatedFiles[i].Score > result.RelatedFiles[j].Score
	})

	// Limit results
	if len(result.RelatedFiles) > rfs.config.MaxResults {
		result.RelatedFiles = result.RelatedFiles[:rfs.config.MaxResults]
	}

	result.TotalFound = len(relatedMap)
	result.QueryTime = time.Since(start)

	// Cache result
	rfs.cacheMu.Lock()
	rfs.cache[focusFile] = result
	rfs.cacheMu.Unlock()

	rfs.logger.Debug("Found related files",
		"focus_file", focusFile,
		"found", result.TotalFound,
		"returned", len(result.RelatedFiles),
		"time", result.QueryTime,
	)

	return result, nil
}

// addDirectDependencies adds direct dependencies to the related files map.
func (rfs *RelatedFilesSuggester) addDirectDependencies(focusFile string, graph *DependencyGraph, relatedMap map[string]*RelatedFile) {
	deps := graph.GetDependencies(focusFile)
	for _, dep := range deps {
		if dep.ToFile == focusFile {
			continue // Skip self-reference
		}

		rf, exists := relatedMap[dep.ToFile]
		if !exists {
			rf = &RelatedFile{
				Path:          dep.ToFile,
				Score:         0,
				Reasons:       []RelatedFileReason{},
				ReasonDetails: []string{},
				TokenEstimate: 1000,
				IsTest:        isTestFile(dep.ToFile),
				Distance:      1,
			}
		}

		rf.Score += 0.8 // High score for direct imports
		rf.Reasons = append(rf.Reasons, RelatedReasonDirectImport)
		rf.ReasonDetails = append(rf.ReasonDetails,
			fmt.Sprintf("imported via %s", dep.ImportPath))
		relatedMap[dep.ToFile] = rf
	}
}

// addReverseDependencies adds files that depend on the focus file.
func (rfs *RelatedFilesSuggester) addReverseDependencies(focusFile string, graph *DependencyGraph, relatedMap map[string]*RelatedFile) {
	dependents := graph.GetDependents(focusFile)
	for _, dep := range dependents {
		if dep.FromFile == focusFile {
			continue // Skip self-reference
		}

		rf, exists := relatedMap[dep.FromFile]
		if !exists {
			rf = &RelatedFile{
				Path:          dep.FromFile,
				Score:         0,
				Reasons:       []RelatedFileReason{},
				ReasonDetails: []string{},
				TokenEstimate: 1000,
				IsTest:        isTestFile(dep.FromFile),
				Distance:      1,
			}
		}

		rf.Score += 0.6 // Medium-high score for reverse deps
		rf.Reasons = append(rf.Reasons, RelatedReasonReverseDep)
		rf.ReasonDetails = append(rf.ReasonDetails,
			fmt.Sprintf("depends on %s", focusFile))
		relatedMap[dep.FromFile] = rf
	}
}

// addTransitiveDependencies adds transitive dependencies.
func (rfs *RelatedFilesSuggester) addTransitiveDependencies(focusFile string, graph *DependencyGraph, relatedMap map[string]*RelatedFile) {
	// Get related files up to max depth
	related := graph.GetRelatedFiles(focusFile, rfs.config.MaxDepth)

	for i, path := range related {
		if path == focusFile {
			continue // Skip self
		}

		distance := i + 1
		if distance <= 1 {
			continue // Already handled by direct dependencies
		}

		// Score decreases with distance
		score := 0.4 / float64(distance)

		rf, exists := relatedMap[path]
		if !exists {
			rf = &RelatedFile{
				Path:          path,
				Score:         0,
				Reasons:       []RelatedFileReason{},
				ReasonDetails: []string{},
				TokenEstimate: 1000,
				IsTest:        isTestFile(path),
				Distance:      distance,
			}
		}

		rf.Score += score
		rf.Reasons = append(rf.Reasons, RelatedReasonTransitive)
		rf.ReasonDetails = append(rf.ReasonDetails,
			fmt.Sprintf("distance %d dependency", distance))
		relatedMap[path] = rf
	}
}

// addSharedImportFiles adds files that share imports with the focus file.
func (rfs *RelatedFilesSuggester) addSharedImportFiles(focusFile string, graph *DependencyGraph, relatedMap map[string]*RelatedFile) {
	// Get focus file's imports
	focusDeps := graph.GetDependencies(focusFile)
	focusImports := make(map[string]bool)
	for _, dep := range focusDeps {
		if dep.ImportPath != "" {
			focusImports[dep.ImportPath] = true
		}
	}

	// Find files with shared imports
	allFiles := graph.GetAllFiles()
	for _, file := range allFiles {
		if file == focusFile {
			continue
		}

		fileDeps := graph.GetDependencies(file)
		sharedCount := 0
		for _, dep := range fileDeps {
			if focusImports[dep.ImportPath] {
				sharedCount++
			}
		}

		if sharedCount > 0 {
			// Score based on number of shared imports
			score := float64(sharedCount) * 0.1
			if score > 0.5 {
				score = 0.5 // Cap score
			}

			rf, exists := relatedMap[file]
			if !exists {
				rf = &RelatedFile{
					Path:          file,
					Score:         0,
					Reasons:       []RelatedFileReason{},
					ReasonDetails: []string{},
					TokenEstimate: 1000,
					IsTest:        isTestFile(file),
					Distance:      2,
				}
			}

			rf.Score += score
			rf.Reasons = append(rf.Reasons, RelatedReasonSharedImport)
			rf.ReasonDetails = append(rf.ReasonDetails,
				fmt.Sprintf("shares %d imports", sharedCount))
			relatedMap[file] = rf
		}
	}
}

// addTestRelationships adds test file relationships.
func (rfs *RelatedFilesSuggester) addTestRelationships(focusFile string, relatedMap map[string]*RelatedFile) {
	isFocusTest := isTestFile(focusFile)

	if !isFocusTest {
		// Find test files for the focus file
		testFile := findTestFile(focusFile)
		if testFile != "" {
			rf := &RelatedFile{
				Path:          testFile,
				Score:         0.5,
				Reasons:       []RelatedFileReason{RelatedReasonTestForFile},
				ReasonDetails: []string{"test file for " + focusFile},
				TokenEstimate: 1000,
				IsTest:        true,
				Distance:      1,
			}
			relatedMap[testFile] = rf
		}
	} else {
		// Focus is a test file, find the tested file
		testedFile := findTestedFile(focusFile)
		if testedFile != "" {
			rf := &RelatedFile{
				Path:          testedFile,
				Score:         0.7,
				Reasons:       []RelatedFileReason{RelatedReasonTestedbyFile},
				ReasonDetails: []string{"tested by " + focusFile},
				TokenEstimate: 1000,
				IsTest:        false,
				Distance:      1,
			}
			relatedMap[testedFile] = rf
		}
	}
}

// GetAllFiles returns all files in the dependency graph.
func (g *DependencyGraph) GetAllFiles() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	files := make([]string, 0, len(g.allFiles))
	for f := range g.allFiles {
		files = append(files, f)
	}
	return files
}

// findTestFile attempts to find the test file for a given file.
func findTestFile(filePath string) string {
	// Remove extension
	ext := ""
	base := filePath
	if idx := strings.LastIndex(filePath, "."); idx > 0 {
		ext = filePath[idx:]
		base = filePath[:idx]
	}

	// Try common test file patterns
	testPatterns := []string{
		base + "_test" + ext,
		base + ".test" + ext,
		base + ".spec" + ext,
	}

	// Note: This doesn't actually check if files exist
	// In a real implementation, you'd check file existence
	// For now, return the first pattern as a suggestion
	if len(testPatterns) > 0 {
		return testPatterns[0]
	}
	return ""
}

// FindTestFilePublic is a public wrapper for findTestFile (for testing).
func FindTestFilePublic(filePath string) string {
	return findTestFile(filePath)
}

// findTestedFile attempts to find the tested file for a test file.
func findTestedFile(testPath string) string {
	base := testPath
	if idx := strings.LastIndex(testPath, "/"); idx >= 0 {
		base = testPath[idx+1:]
	}

	// Remove test suffixes
	testSuffixes := []string{"_test", ".test", ".spec"}
	ext := ""
	if idx := strings.LastIndex(base, "."); idx > 0 {
		ext = base[idx:]
		base = base[:idx]
	}

	for _, suffix := range testSuffixes {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix) + ext
		}
	}

	return ""
}

// FindTestedFilePublic is a public wrapper for findTestedFile (for testing).
func FindTestedFilePublic(testPath string) string {
	return findTestedFile(testPath)
}

// ClearCache clears the suggestion cache.
func (rfs *RelatedFilesSuggester) ClearCache() {
	rfs.cacheMu.Lock()
	defer rfs.cacheMu.Unlock()
	rfs.cache = make(map[string]*RelatedFilesResult)
}

// UpdateConfig updates the suggester configuration.
func (rfs *RelatedFilesSuggester) UpdateConfig(config *RelatedFilesConfig) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()
	rfs.config = config
	rfs.ClearCache()
}

// GetConfig returns the current configuration.
func (rfs *RelatedFilesSuggester) GetConfig() *RelatedFilesConfig {
	rfs.mu.RLock()
	defer rfs.mu.RUnlock()
	return rfs.config
}

// ToMarkdown generates a markdown representation of the result.
func (r *RelatedFilesResult) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Related Files for %s\n\n", r.FocusFile))
	sb.WriteString(fmt.Sprintf("**Query time:** %v | **Max depth:** %d | **Found:** %d\n\n",
		r.QueryTime, r.MaxDepth, r.TotalFound))

	sb.WriteString("| # | File | Score | Distance | Reasons |\n")
	sb.WriteString("|---|------|-------|----------|--------|\n")

	for i, rf := range r.RelatedFiles {
		reasons := make([]string, len(rf.Reasons))
		for j, r := range rf.Reasons {
			reasons[j] = string(r)
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %.2f | %d | %s |\n",
			i+1, rf.Path, rf.Score, rf.Distance, strings.Join(reasons, ", ")))
	}

	return sb.String()
}
