// Package context provides test coverage context capabilities.
// Task 44: Test Coverage Context - include related tests

package context

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// TestCoverageConfig holds configuration for test coverage context.
type TestCoverageConfig struct {
	TestFileSuffix     string   `json:"test_file_suffix"`     // e.g., "_test.go", ".test.ts"
	IncludeCoverage    bool     `json:"include_coverage"`     // Include coverage metrics
	MaxTestFiles       int      `json:"max_test_files"`       // Max test files to include
	MaxTestSize        int      `json:"max_test_size"`        // Max size per test file in bytes
	PrioritizeRelated  bool     `json:"prioritize_related"`   // Prioritize tests for source files
	TestRunTimeout     time.Duration `json:"test_run_timeout"` // Timeout for test runs
}

// DefaultTestCoverageConfig returns default configuration.
func DefaultTestCoverageConfig() *TestCoverageConfig {
	return &TestCoverageConfig{
		TestFileSuffix:    "_test.go",
		IncludeCoverage:   true,
		MaxTestFiles:      10,
		MaxTestSize:       100 * 1024, // 100KB
		PrioritizeRelated: true,
		TestRunTimeout:    30 * time.Second,
	}
}

// TestFileInfo represents information about a test file.
type TestFileInfo struct {
	Path           string            `json:"path"`
	TestFunctions  []string          `json:"test_functions"`
	Benchmarks     []string          `json:"benchmarks"`
	SourceFile     string            `json:"source_file,omitempty"`
	CoveragePct    float64           `json:"coverage_pct,omitempty"`
	LastRun        time.Time         `json:"last_run,omitempty"`
	LastResult     string            `json:"last_result,omitempty"` // "pass", "fail", "skip"
	RelatedSources []string          `json:"related_sources,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TestCoverageResult represents the result of test coverage analysis.
type TestCoverageResult struct {
	SourceFile       string          `json:"source_file"`
	TestFiles        []*TestFileInfo `json:"test_files"`
	TotalTests       int             `json:"total_tests"`
	TotalBenchmarks  int             `json:"total_benchmarks"`
	CoveragePct      float64         `json:"coverage_pct"`
	HasTests         bool            `json:"has_tests"`
	AnalysisTime     time.Duration   `json:"analysis_time"`
}

// TestCoverageManager manages test coverage context.
type TestCoverageManager struct {
	config     *TestCoverageConfig
	logger     *slog.Logger
	mu         sync.RWMutex
	
	// Cache
	testCache     map[string]*TestFileInfo
	coverageCache map[string]float64
	lastAnalysis  time.Time
}

// NewTestCoverageManager creates a new test coverage manager.
func NewTestCoverageManager(config *TestCoverageConfig, logger *slog.Logger) *TestCoverageManager {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultTestCoverageConfig()
	}
	
	return &TestCoverageManager{
		config:        config,
		logger:        logger.With("component", "test_coverage"),
		testCache:     make(map[string]*TestFileInfo),
		coverageCache: make(map[string]float64),
	}
}

// FindTestForSource finds test files for a source file.
func (tcm *TestCoverageManager) FindTestForSource(sourceFile string) (*TestCoverageResult, error) {
	start := time.Now()
	
	tcm.mu.RLock()
	defer tcm.mu.RUnlock()
	
	result := &TestCoverageResult{
		SourceFile: sourceFile,
		TestFiles:  make([]*TestFileInfo, 0),
		HasTests:   false,
	}
	
	// Determine test file path
	testFile := tcm.getTestFilePath(sourceFile)
	if testFile == "" {
		result.AnalysisTime = time.Since(start)
		return result, nil
	}
	
	// Check if test file exists
	if info, err := os.Stat(testFile); err == nil {
		if info.Size() > int64(tcm.config.MaxTestSize) {
			tcm.logger.Debug("Test file too large", "path", testFile, "size", info.Size())
		} else {
			testInfo, err := tcm.parseTestFile(testFile)
			if err == nil {
				testInfo.SourceFile = sourceFile
				result.TestFiles = append(result.TestFiles, testInfo)
				result.TotalTests = len(testInfo.TestFunctions)
				result.TotalBenchmarks = len(testInfo.Benchmarks)
				result.HasTests = true
			}
		}
	}
	
	// Check for additional test files in the same package
	dir := filepath.Dir(sourceFile)
	if additionalTests := tcm.findAdditionalTests(dir, testFile); len(additionalTests) > 0 {
		for _, tf := range additionalTests {
			if len(result.TestFiles) >= tcm.config.MaxTestFiles {
				break
			}
			testInfo, err := tcm.parseTestFile(tf)
			if err == nil {
				result.TestFiles = append(result.TestFiles, testInfo)
				result.TotalTests += len(testInfo.TestFunctions)
				result.TotalBenchmarks += len(testInfo.Benchmarks)
			}
		}
	}
	
	// Get coverage if available
	if tcm.config.IncludeCoverage {
		result.CoveragePct = tcm.coverageCache[sourceFile]
	}
	
	result.AnalysisTime = time.Since(start)
	return result, nil
}

// getTestFilePath returns the expected test file path for a source file.
func (tcm *TestCoverageManager) getTestFilePath(sourceFile string) string {
	ext := filepath.Ext(sourceFile)
	base := strings.TrimSuffix(sourceFile, ext)
	
	// Language-specific test file patterns
	switch ext {
	case ".go":
		return base + "_test" + ext
	case ".ts", ".tsx", ".js", ".jsx":
		return base + ".test" + ext
	case ".py":
		return base + "_test" + ext
	case ".rs":
		return sourceFile // Rust tests are inline
	default:
		return base + "_test" + ext
	}
}

// parseTestFile parses a test file and extracts test functions.
func (tcm *TestCoverageManager) parseTestFile(path string) (*TestFileInfo, error) {
	// Check cache first
	if cached, ok := tcm.testCache[path]; ok {
		return cached, nil
	}
	
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	info := &TestFileInfo{
		Path:      path,
		Metadata:  make(map[string]string),
	}
	
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		tcm.parseGoTests(content, info)
	case ".ts", ".tsx", ".js", ".jsx":
		tcm.parseJSTests(content, info)
	case ".py":
		tcm.parsePythonTests(content, info)
	case ".rs":
		tcm.parseRustTests(content, info)
	}
	
	// Cache result
	tcm.testCache[path] = info
	
	return info, nil
}

// parseGoTests parses Go test file and extracts test functions.
func (tcm *TestCoverageManager) parseGoTests(content []byte, info *TestFileInfo) {
	// Match TestXxx functions
	testRegex := regexp.MustCompile(`func\s+(Test[A-Z][a-zA-Z0-9]*)\s*\(`)
	matches := testRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, string(m[1]))
		}
	}
	
	// Match BenchmarkXxx functions
	benchRegex := regexp.MustCompile(`func\s+(Benchmark[A-Z][a-zA-Z0-9]*)\s*\(`)
	matches = benchRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.Benchmarks = append(info.Benchmarks, string(m[1]))
		}
	}
	
	// Match FuzzXxx functions (Go 1.18+)
	fuzzRegex := regexp.MustCompile(`func\s+(Fuzz[A-Z][a-zA-Z0-9]*)\s*\(`)
	matches = fuzzRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, string(m[1]))
		}
	}
}

// parseJSTests parses JavaScript/TypeScript test files.
func (tcm *TestCoverageManager) parseJSTests(content []byte, info *TestFileInfo) {
	// Match describe/test/it blocks
	// Note: Using string concatenation for backtick since it can't appear in raw string literals
	testRegex := regexp.MustCompile(`(?:describe|test|it)\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]`)
	matches := testRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, string(m[1]))
		}
	}
}

// parsePythonTests parses Python test files.
func (tcm *TestCoverageManager) parsePythonTests(content []byte, info *TestFileInfo) {
	// Match test_ functions and Test classes
	testFuncRegex := regexp.MustCompile(`def\s+(test_[a-zA-Z0-9_]*)\s*\(`)
	matches := testFuncRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, string(m[1]))
		}
	}
	
	// Match test classes
	testClassRegex := regexp.MustCompile(`class\s+(Test[A-Z][a-zA-Z0-9]*)\s*[:\(]`)
	matches = testClassRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, "class:"+string(m[1]))
		}
	}
}

// parseRustTests parses Rust test code.
func (tcm *TestCoverageManager) parseRustTests(content []byte, info *TestFileInfo) {
	// Match #[test] functions
	testRegex := regexp.MustCompile(`#\s*\[test\]\s*(?:#\s*\[.*\]\s*)*\s*fn\s+([a-z_][a-z0-9_]*)\s*\(`)
	matches := testRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.TestFunctions = append(info.TestFunctions, string(m[1]))
		}
	}
	
	// Match #[bench] functions
	benchRegex := regexp.MustCompile(`#\s*\[bench\]\s*fn\s+([a-z_][a-z0-9_]*)\s*\(`)
	matches = benchRegex.FindAllSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			info.Benchmarks = append(info.Benchmarks, string(m[1]))
		}
	}
}

// findAdditionalTests finds additional test files in the same directory.
func (tcm *TestCoverageManager) findAdditionalTests(dir string, excludeFile string) []string {
	var testFiles []string
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return testFiles
	}
	
	ext := tcm.config.TestFileSuffix
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		
		// Check if it's a test file
		if strings.HasSuffix(name, ext) || strings.Contains(name, ".test.") || strings.HasSuffix(name, "_test"+filepath.Ext(name)) {
			fullPath := filepath.Join(dir, name)
			if fullPath != excludeFile {
				testFiles = append(testFiles, fullPath)
			}
		}
	}
	
	// Sort by name for consistent ordering
	sort.Strings(testFiles)
	
	return testFiles
}

// GetTestContext returns test context for multiple source files.
func (tcm *TestCoverageManager) GetTestContext(ctx context.Context, sourceFiles []string) ([]*TestCoverageResult, error) {
	results := make([]*TestCoverageResult, 0, len(sourceFiles))
	
	for _, source := range sourceFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		result, err := tcm.FindTestForSource(source)
		if err != nil {
			tcm.logger.Debug("Error finding tests", "source", source, "error", err)
			continue
		}
		results = append(results, result)
	}
	
	return results, nil
}

// UpdateCoverage updates the coverage cache for a source file.
func (tcm *TestCoverageManager) UpdateCoverage(sourceFile string, coverage float64) {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()
	tcm.coverageCache[sourceFile] = coverage
}

// ClearCache clears the test and coverage cache.
func (tcm *TestCoverageManager) ClearCache() {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()
	tcm.testCache = make(map[string]*TestFileInfo)
	tcm.coverageCache = make(map[string]float64)
}

// GetTestStats returns statistics about cached tests.
func (tcm *TestCoverageManager) GetTestStats() map[string]interface{} {
	tcm.mu.RLock()
	defer tcm.mu.RUnlock()
	
	totalTests := 0
	totalBenchmarks := 0
	
	for _, info := range tcm.testCache {
		totalTests += len(info.TestFunctions)
		totalBenchmarks += len(info.Benchmarks)
	}
	
	return map[string]interface{}{
		"cached_test_files":   len(tcm.testCache),
		"cached_sources":      len(tcm.coverageCache),
		"total_tests":         totalTests,
		"total_benchmarks":    totalBenchmarks,
		"last_analysis":       tcm.lastAnalysis,
	}
}

// ToMarkdown generates a markdown representation of the test coverage result.
func (r *TestCoverageResult) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# Test Coverage: %s\n\n", filepath.Base(r.SourceFile)))
	
	if !r.HasTests {
		sb.WriteString("⚠️ **No tests found for this file.**\n")
		return sb.String()
	}
	
	sb.WriteString(fmt.Sprintf("**Coverage:** %.1f%% | **Tests:** %d | **Benchmarks:** %d | **Analysis time:** %v\n\n",
		r.CoveragePct, r.TotalTests, r.TotalBenchmarks, r.AnalysisTime))
	
	for _, tf := range r.TestFiles {
		sb.WriteString(fmt.Sprintf("## %s\n\n", filepath.Base(tf.Path)))
		
		if len(tf.TestFunctions) > 0 {
			sb.WriteString("### Tests\n\n")
			for _, test := range tf.TestFunctions {
				sb.WriteString(fmt.Sprintf("- `%s`\n", test))
			}
			sb.WriteString("\n")
		}
		
		if len(tf.Benchmarks) > 0 {
			sb.WriteString("### Benchmarks\n\n")
			for _, bench := range tf.Benchmarks {
				sb.WriteString(fmt.Sprintf("- `%s`\n", bench))
			}
			sb.WriteString("\n")
		}
	}
	
	return sb.String()
}

// GetSourceFromTest returns the source file for a test file.
func (tcm *TestCoverageManager) GetSourceFromTest(testFile string) string {
	ext := filepath.Ext(testFile)
	base := strings.TrimSuffix(testFile, ext)
	
	switch ext {
	case ".go":
		// file_test.go -> file.go
		if strings.HasSuffix(base, "_test") {
			return strings.TrimSuffix(base, "_test") + ext
		}
	case ".ts", ".tsx", ".js", ".jsx":
		// file.test.ts -> file.ts
		if idx := strings.Index(base, ".test"); idx > 0 {
			return base[:idx] + ext
		}
	case ".py":
		// file_test.py -> file.py
		if strings.HasSuffix(base, "_test") {
			return strings.TrimSuffix(base, "_test") + ext
		}
	}
	
	return ""
}

// ShouldIncludeTest determines if a test file should be included in context.
func (tcm *TestCoverageManager) ShouldIncludeTest(testFile string, sourceFiles []string) bool {
	source := tcm.GetSourceFromTest(testFile)
	if source == "" {
		return false
	}
	
	for _, sf := range sourceFiles {
		if sf == source {
			return true
		}
	}
	
	return false
}
