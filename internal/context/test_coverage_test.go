// Package context provides test coverage context capabilities.
package context

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultTestCoverageConfig(t *testing.T) {
	config := DefaultTestCoverageConfig()
	
	if config.TestFileSuffix != "_test.go" {
		t.Errorf("expected _test.go, got %s", config.TestFileSuffix)
	}
	if !config.IncludeCoverage {
		t.Error("expected IncludeCoverage to be true")
	}
	if config.MaxTestFiles != 10 {
		t.Errorf("expected 10, got %d", config.MaxTestFiles)
	}
	if config.MaxTestSize != 100*1024 {
		t.Errorf("expected 100KB, got %d", config.MaxTestSize)
	}
	if config.TestRunTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", config.TestRunTimeout)
	}
}

func TestNewTestCoverageManager(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	if tcm == nil {
		t.Fatal("expected non-nil manager")
	}
	if tcm.testCache == nil {
		t.Error("expected testCache to be initialized")
	}
	if tcm.coverageCache == nil {
		t.Error("expected coverageCache to be initialized")
	}
}

func TestTestCoverageManager_GetTestFilePath(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	tests := []struct {
		source   string
		expected string
	}{
		{"handler.go", "handler_test.go"},
		{"service.ts", "service.test.ts"},
		{"utils.py", "utils_test.py"},
		{"main.rs", "main.rs"}, // Rust tests are inline
	}
	
	for _, tc := range tests {
		result := tcm.getTestFilePath(tc.source)
		if result != tc.expected {
			t.Errorf("getTestFilePath(%s) = %s, expected %s", tc.source, result, tc.expected)
		}
	}
}

func TestTestCoverageManager_FindTestForSource_NoTest(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create source file without test
	sourceFile := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(sourceFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	result, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.HasTests {
		t.Error("expected HasTests to be false")
	}
	if len(result.TestFiles) != 0 {
		t.Errorf("expected no test files, got %d", len(result.TestFiles))
	}
}

func TestTestCoverageManager_FindTestForSource_WithTest(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create source file
	sourceFile := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(sourceFile, []byte("package main\n\nfunc Handle() {}"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	
	// Create test file
	testFile := filepath.Join(tmpDir, "handler_test.go")
	testContent := `package main

import "testing"

func TestHandle(t *testing.T) {
	Handle()
}

func BenchmarkHandle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Handle()
	}
}

func FuzzHandle(f *testing.F) {
	f.Fuzz(func(t *testing.T, s string) {
		Handle()
	})
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	result, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !result.HasTests {
		t.Error("expected HasTests to be true")
	}
	if len(result.TestFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d", len(result.TestFiles))
	}
	
	tf := result.TestFiles[0]
	if len(tf.TestFunctions) != 2 {
		t.Errorf("expected 2 test functions (TestHandle, FuzzHandle), got %d: %v", len(tf.TestFunctions), tf.TestFunctions)
	}
	if len(tf.Benchmarks) != 1 {
		t.Errorf("expected 1 benchmark, got %d: %v", len(tf.Benchmarks), tf.Benchmarks)
	}
	
	// Check test function names
	foundTest := false
	foundFuzz := false
	for _, fn := range tf.TestFunctions {
		if fn == "TestHandle" {
			foundTest = true
		}
		if fn == "FuzzHandle" {
			foundFuzz = true
		}
	}
	if !foundTest {
		t.Error("expected to find TestHandle")
	}
	if !foundFuzz {
		t.Error("expected to find FuzzHandle")
	}
}

func TestTestCoverageManager_ParseJSTests(t *testing.T) {
	tmpDir := t.TempDir()
	
	testFile := filepath.Join(tmpDir, "utils.test.ts")
	testContent := `describe('Utils', () => {
  test('should format date', () => {});
  it('should parse JSON', () => {});
});
test('standalone test', () => {});`
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	info, err := tcm.parseTestFile(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(info.TestFunctions) < 3 {
		t.Errorf("expected at least 3 test functions, got %d: %v", len(info.TestFunctions), info.TestFunctions)
	}
}

func TestTestCoverageManager_ParsePythonTests(t *testing.T) {
	tmpDir := t.TempDir()
	
	testFile := filepath.Join(tmpDir, "test_utils.py")
	testContent := `import unittest

class TestUtils(unittest.TestCase):
    def test_format(self):
        pass
    
    def test_parse(self):
        pass

def test_standalone():
    pass
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	info, err := tcm.parseTestFile(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should find test_format, test_parse, test_standalone, and class:TestUtils
	if len(info.TestFunctions) < 3 {
		t.Errorf("expected at least 3 test functions, got %d: %v", len(info.TestFunctions), info.TestFunctions)
	}
}

func TestTestCoverageManager_ParseRustTests(t *testing.T) {
	tmpDir := t.TempDir()
	
	testFile := filepath.Join(tmpDir, "main.rs")
	testContent := `#[test]
fn test_add() {
    assert_eq!(1 + 1, 2);
}

#[test]
#[should_panic]
fn test_panic() {
    panic!("expected");
}

#[bench]
fn bench_add(b: &mut test::Bencher) {
    b.iter(|| 1 + 1);
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	info, err := tcm.parseTestFile(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(info.TestFunctions) != 2 {
		t.Errorf("expected 2 test functions, got %d: %v", len(info.TestFunctions), info.TestFunctions)
	}
	if len(info.Benchmarks) != 1 {
		t.Errorf("expected 1 benchmark, got %d: %v", len(info.Benchmarks), info.Benchmarks)
	}
}

func TestTestCoverageManager_FindAdditionalTests(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple test files
	testFiles := []string{"handler_test.go", "utils_test.go", "other_test.go"}
	for _, tf := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, tf), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	additional := tcm.findAdditionalTests(tmpDir, filepath.Join(tmpDir, "handler_test.go"))
	
	if len(additional) != 2 {
		t.Errorf("expected 2 additional test files, got %d: %v", len(additional), additional)
	}
}

func TestTestCoverageManager_GetTestContext(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create source and test files
	source1 := filepath.Join(tmpDir, "handler.go")
	test1 := filepath.Join(tmpDir, "handler_test.go")
	source2 := filepath.Join(tmpDir, "utils.go")
	test2 := filepath.Join(tmpDir, "utils_test.go")
	
	if err := os.WriteFile(source1, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source1: %v", err)
	}
	if err := os.WriteFile(test1, []byte("package main\n\nfunc TestHandler(t *testing.T) {}"), 0644); err != nil {
		t.Fatalf("failed to create test1: %v", err)
	}
	if err := os.WriteFile(source2, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source2: %v", err)
	}
	if err := os.WriteFile(test2, []byte("package main\n\nfunc TestUtils(t *testing.T) {}"), 0644); err != nil {
		t.Fatalf("failed to create test2: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	results, err := tcm.GetTestContext(context.Background(), []string{source1, source2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	
	totalTests := 0
	for _, r := range results {
		totalTests += r.TotalTests
	}
	if totalTests < 2 {
		t.Errorf("expected at least 2 total tests, got %d", totalTests)
	}
}

func TestTestCoverageManager_UpdateCoverage(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	tcm.UpdateCoverage("handler.go", 85.5)
	
	coverage := tcm.coverageCache["handler.go"]
	if coverage != 85.5 {
		t.Errorf("expected 85.5, got %f", coverage)
	}
}

func TestTestCoverageManager_ClearCache(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	// Add some cache entries
	tcm.testCache["test.go"] = &TestFileInfo{Path: "test.go"}
	tcm.coverageCache["source.go"] = 75.0
	
	tcm.ClearCache()
	
	if len(tcm.testCache) != 0 {
		t.Error("expected testCache to be empty")
	}
	if len(tcm.coverageCache) != 0 {
		t.Error("expected coverageCache to be empty")
	}
}

func TestTestCoverageManager_GetTestStats(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	// Add some cache entries
	tcm.testCache["test1.go"] = &TestFileInfo{
		Path:          "test1.go",
		TestFunctions: []string{"TestA", "TestB"},
		Benchmarks:    []string{"BenchmarkA"},
	}
	tcm.testCache["test2.go"] = &TestFileInfo{
		Path:          "test2.go",
		TestFunctions: []string{"TestC"},
	}
	tcm.coverageCache["source.go"] = 90.0
	
	stats := tcm.GetTestStats()
	
	if stats["cached_test_files"].(int) != 2 {
		t.Errorf("expected 2 cached test files, got %v", stats["cached_test_files"])
	}
	if stats["total_tests"].(int) != 3 {
		t.Errorf("expected 3 total tests, got %v", stats["total_tests"])
	}
	if stats["total_benchmarks"].(int) != 1 {
		t.Errorf("expected 1 benchmark, got %v", stats["total_benchmarks"])
	}
}

func TestTestCoverageManager_GetSourceFromTest(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	tests := []struct {
		testFile string
		expected string
	}{
		{"handler_test.go", "handler.go"},
		{"service.test.ts", "service.ts"},
		{"utils_test.py", "utils.py"},
		{"unknown.txt", ""},
	}
	
	for _, tc := range tests {
		result := tcm.GetSourceFromTest(tc.testFile)
		if result != tc.expected {
			t.Errorf("GetSourceFromTest(%s) = %s, expected %s", tc.testFile, result, tc.expected)
		}
	}
}

func TestTestCoverageManager_ShouldIncludeTest(t *testing.T) {
	tcm := NewTestCoverageManager(nil, nil)
	
	sourceFiles := []string{"handler.go", "utils.go"}
	
	tests := []struct {
		testFile string
		expected bool
	}{
		{"handler_test.go", true},
		{"utils_test.go", true},
		{"other_test.go", false},
	}
	
	for _, tc := range tests {
		result := tcm.ShouldIncludeTest(tc.testFile, sourceFiles)
		if result != tc.expected {
			t.Errorf("ShouldIncludeTest(%s) = %v, expected %v", tc.testFile, result, tc.expected)
		}
	}
}

func TestTestCoverageManager_MaxTestSize(t *testing.T) {
	tmpDir := t.TempDir()
	
	sourceFile := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(sourceFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	
	// Create large test file
	testFile := filepath.Join(tmpDir, "handler_test.go")
	largeContent := make([]byte, 200*1024) // 200KB
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	config := DefaultTestCoverageConfig()
	config.MaxTestSize = 100 * 1024 // 100KB
	tcm := NewTestCoverageManager(config, nil)
	
	result, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should not include the large test file
	if result.HasTests {
		t.Error("expected HasTests to be false (file too large)")
	}
}

func TestTestCoverageManager_MaxTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	sourceFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(sourceFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	
	// Create multiple test files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('0'+i))+"_test.go")
		content := "package main\n\nfunc Test" + string(rune('A'+i)) + "(t *testing.T) {}"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	
	config := DefaultTestCoverageConfig()
	config.MaxTestFiles = 2
	tcm := NewTestCoverageManager(config, nil)
	
	result, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should only include 2 test files
	if len(result.TestFiles) > config.MaxTestFiles {
		t.Errorf("expected at most %d test files, got %d", config.MaxTestFiles, len(result.TestFiles))
	}
}

func TestTestCoverageManager_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple source files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go")
		if err := os.WriteFile(files[i], []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create file %d: %v", i, err)
		}
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	_, err := tcm.GetTestContext(ctx, files)
	if err != context.Canceled {
		t.Logf("context cancellation result: %v", err)
	}
}

func TestTestCoverageResult_ToMarkdown(t *testing.T) {
	result := &TestCoverageResult{
		SourceFile:      "handler.go",
		HasTests:        true,
		TotalTests:      5,
		TotalBenchmarks: 2,
		CoveragePct:     85.5,
		AnalysisTime:    10 * time.Millisecond,
		TestFiles: []*TestFileInfo{
			{
				Path:          "handler_test.go",
				TestFunctions: []string{"TestHandle", "TestProcess"},
				Benchmarks:    []string{"BenchmarkHandle"},
			},
		},
	}
	
	markdown := result.ToMarkdown()
	
	if markdown == "" {
		t.Error("expected non-empty markdown")
	}
	if !contains(markdown, "handler.go") {
		t.Error("expected source file name in markdown")
	}
	if !contains(markdown, "85.5%") {
		t.Error("expected coverage percentage in markdown")
	}
}

func TestTestCoverageResult_ToMarkdown_NoTests(t *testing.T) {
	result := &TestCoverageResult{
		SourceFile: "handler.go",
		HasTests:   false,
	}
	
	markdown := result.ToMarkdown()
	
	if !contains(markdown, "No tests found") {
		t.Errorf("expected 'No tests found' message in markdown: %s", markdown)
	}
}

func TestTestCoverageManager_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	
	sourceFile := filepath.Join(tmpDir, "handler.go")
	testFile := filepath.Join(tmpDir, "handler_test.go")
	
	if err := os.WriteFile(sourceFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc TestHandler(t *testing.T) {}"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	tcm := NewTestCoverageManager(nil, nil)
	
	// First call - should parse and cache
	result1, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Second call - should use cache
	result2, err := tcm.FindTestForSource(sourceFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result1.TotalTests != result2.TotalTests {
		t.Errorf("cached result differs: %d vs %d", result1.TotalTests, result2.TotalTests)
	}
	
	// Check cache was populated
	if len(tcm.testCache) == 0 {
		t.Error("expected test cache to be populated")
	}
}
