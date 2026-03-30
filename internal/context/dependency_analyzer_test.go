// Package context provides dependency analyzer tests.
// Task 37: Cross-file dependency analysis tests

package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDependencyAnalyzer(t *testing.T) {
	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
	}

	if !analyzer.config.IncludeTests {
		t.Error("expected IncludeTests true by default")
	}
}

func TestDefaultDependencyConfig(t *testing.T) {
	config := DefaultDependencyConfig()

	if !config.IncludeTests {
		t.Error("expected IncludeTests true")
	}
	if config.IncludeVendor {
		t.Error("expected IncludeVendor false by default")
	}
	if config.MaxDepth != 10 {
		t.Errorf("expected MaxDepth 10, got %d", config.MaxDepth)
	}
}

func TestDependencyAnalyzer_GoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmodule\n\ngo 1.21"), 0644)

	// Create imported package
	pkgDir := filepath.Join(tmpDir, "pkg")
	os.Mkdir(pkgDir, 0755)
	pkgFile := filepath.Join(pkgDir, "types.go")
	os.WriteFile(pkgFile, []byte("package pkg\n\ntype MyType struct{}"), 0644)

	// Create main file that imports pkg
	mainFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import "testmodule/pkg"

func main() {
	var x pkg.MyType
	_ = x
}
`
	os.WriteFile(mainFile, []byte(content), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	if graph == nil {
		t.Fatal("expected non-nil graph")
	}

	if graph.TotalFiles == 0 {
		t.Error("expected some files")
	}

	// Check main.go has dependency on pkg
	deps := graph.GetDependencies("main.go")
	if len(deps) == 0 {
		t.Error("expected main.go to have dependencies")
	}
}

func TestDependencyAnalyzer_TypeScriptFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create imported file
	utilFile := filepath.Join(tmpDir, "util.ts")
	os.WriteFile(utilFile, []byte("export function helper() {}"), 0644)

	// Create main file that imports util
	mainFile := filepath.Join(tmpDir, "main.ts")
	content := `import { helper } from './util';

function main() {
	helper();
}
`
	os.WriteFile(mainFile, []byte(content), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Check main.ts has dependency on util.ts
	deps := graph.GetDependencies("main.ts")
	if len(deps) == 0 {
		t.Error("expected main.ts to have dependencies")
	}
}

func TestDependencyAnalyzer_PythonFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create imported module
	utilFile := filepath.Join(tmpDir, "util.py")
	os.WriteFile(utilFile, []byte("def helper():\n    pass"), 0644)

	// Create main file that imports util
	mainFile := filepath.Join(tmpDir, "main.py")
	content := `from util import helper

def main():
    helper()
`
	os.WriteFile(mainFile, []byte(content), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Check main.py has dependency on util.py
	deps := graph.GetDependencies("main.py")
	if len(deps) == 0 {
		t.Error("expected main.py to have dependencies")
	}
}

func TestDependencyAnalyzer_RustFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create module file
	modFile := filepath.Join(tmpDir, "utils.rs")
	os.WriteFile(modFile, []byte("pub fn helper() {}"), 0644)

	// Create main file that uses utils
	mainFile := filepath.Join(tmpDir, "main.rs")
	content := `mod utils;

fn main() {
    utils::helper();
}
`
	os.WriteFile(mainFile, []byte(content), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Check main.rs has dependency on utils.rs
	deps := graph.GetDependencies("main.rs")
	if len(deps) == 0 {
		t.Error("expected main.rs to have dependencies")
	}
}

func TestDependencyAnalyzer_SkipVendor(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmodule\n\ngo 1.21"), 0644)

	// Create vendor directory (should be skipped)
	vendorDir := filepath.Join(tmpDir, "vendor")
	os.Mkdir(vendorDir, 0755)
	vendorFile := filepath.Join(vendorDir, "vendor.go")
	os.WriteFile(vendorFile, []byte("package vendor\nfunc Vendor() {}"), 0644)

	// Create main file
	mainFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainFile, []byte("package main\nfunc Main() {}"), 0644)

	config := DefaultDependencyConfig()
	config.IncludeVendor = false
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Should not include vendor files
	for file := range graph.allFiles {
		if strings.Contains(file, "vendor") {
			t.Errorf("expected vendor files to be excluded, found: %s", file)
		}
	}
}

func TestDependencyAnalyzer_IncludeExcludeTests(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main file
	mainFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainFile, []byte("package main\nfunc Main() {}"), 0644)

	// Create test file
	testFile := filepath.Join(tmpDir, "main_test.go")
	os.WriteFile(testFile, []byte("package main\nfunc TestMain(*testing.T) {}"), 0644)

	t.Run("include_tests", func(t *testing.T) {
		config := DefaultDependencyConfig()
		config.IncludeTests = true
		analyzer := NewDependencyAnalyzer(config, nil)

		ctx := context.Background()
		graph, err := analyzer.Analyze(ctx, tmpDir)
		if err != nil {
			t.Fatalf("failed to analyze: %v", err)
		}

		// Should include test files
		found := false
		for file := range graph.allFiles {
			if strings.HasSuffix(file, "_test.go") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected test files to be included")
		}
	})

	t.Run("exclude_tests", func(t *testing.T) {
		config := DefaultDependencyConfig()
		config.IncludeTests = false
		analyzer := NewDependencyAnalyzer(config, nil)

		ctx := context.Background()
		graph, err := analyzer.Analyze(ctx, tmpDir)
		if err != nil {
			t.Fatalf("failed to analyze: %v", err)
		}

		// Should not include test files
		for file := range graph.allFiles {
			if strings.HasSuffix(file, "_test.go") {
				t.Errorf("expected test files to be excluded, found: %s", file)
			}
		}
	})
}

func TestDependencyAnalyzer_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte("package main\nfunc Main() {}"), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := analyzer.Analyze(ctx, tmpDir)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestDependencyAnalyzer_InvalidPath(t *testing.T) {
	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()

	t.Run("nonexistent", func(t *testing.T) {
		_, err := analyzer.Analyze(ctx, "/nonexistent/path")
		if err == nil {
			t.Error("expected error for nonexistent path")
		}
	})

	t.Run("file_not_dir", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "file.txt")
		os.WriteFile(tmpFile, []byte("test"), 0644)

		_, err := analyzer.Analyze(ctx, tmpFile)
		if err == nil {
			t.Error("expected error for file path")
		}
	})
}

func TestDependencyGraph_GetRelatedFiles(t *testing.T) {
	// Create a manual graph for testing
	graph := &DependencyGraph{
		RootPath:    "/test",
		fileDeps:    make(map[string][]*Dependency),
		reverseDeps: make(map[string][]*Dependency),
		allFiles:    make(map[string]bool),
	}

	// Chain: a.go -> b.go -> c.go (a depends on b, b depends on c)
	graph.allFiles["a.go"] = true
	graph.allFiles["b.go"] = true
	graph.allFiles["c.go"] = true

	dep1 := &Dependency{FromFile: "a.go", ToFile: "b.go", Kind: DependencyImport}
	dep2 := &Dependency{FromFile: "b.go", ToFile: "c.go", Kind: DependencyImport}

	graph.fileDeps["a.go"] = []*Dependency{dep1}
	graph.fileDeps["b.go"] = []*Dependency{dep2}
	graph.reverseDeps["b.go"] = []*Dependency{dep1}
	graph.reverseDeps["c.go"] = []*Dependency{dep2}

	// Get related files for a.go with depth 1
	related := graph.GetRelatedFiles("a.go", 1)
	if len(related) == 0 {
		t.Error("expected related files for a.go")
	}

	// Get related files with depth 2
	related = graph.GetRelatedFiles("a.go", 2)
	if len(related) < 2 {
		t.Errorf("expected at least 2 related files, got %d", len(related))
	}
}

func TestDependencyGraph_CycleDetection(t *testing.T) {
	// Create a cycle: A -> B -> A
	graph := &DependencyGraph{
		RootPath:    "/test",
		fileDeps:    make(map[string][]*Dependency),
		reverseDeps: make(map[string][]*Dependency),
		allFiles:    make(map[string]bool),
	}

	graph.allFiles["a.go"] = true
	graph.allFiles["b.go"] = true

	graph.Dependencies = []Dependency{
		{FromFile: "a.go", ToFile: "b.go", Kind: DependencyImport},
		{FromFile: "b.go", ToFile: "a.go", Kind: DependencyImport},
	}

	graph.fileDeps["a.go"] = []*Dependency{&graph.Dependencies[0]}
	graph.fileDeps["b.go"] = []*Dependency{&graph.Dependencies[1]}
	graph.reverseDeps["b.go"] = []*Dependency{&graph.Dependencies[0]}
	graph.reverseDeps["a.go"] = []*Dependency{&graph.Dependencies[1]}

	analyzer := NewDependencyAnalyzer(nil, nil)
	cycles := analyzer.detectCycles(graph)

	if len(cycles) == 0 {
		t.Error("expected to detect cycle")
	}

	// Manually set cycles for HasCycles test
	graph.Cycles = cycles

	if !graph.HasCycles() {
		t.Error("expected HasCycles to return true")
	}
}

func TestDependencyGraph_TopologicalOrder(t *testing.T) {
	graph := &DependencyGraph{
		RootPath:    "/test",
		fileDeps:    make(map[string][]*Dependency),
		reverseDeps: make(map[string][]*Dependency),
		allFiles:    make(map[string]bool),
	}

	// Chain: a.go depends on b.go, b.go depends on c.go
	// Topological order should be: c.go, b.go, a.go (dependencies first)
	graph.allFiles["a.go"] = true
	graph.allFiles["b.go"] = true
	graph.allFiles["c.go"] = true

	dep1 := &Dependency{FromFile: "a.go", ToFile: "b.go", Kind: DependencyImport}
	dep2 := &Dependency{FromFile: "b.go", ToFile: "c.go", Kind: DependencyImport}

	graph.fileDeps["a.go"] = []*Dependency{dep1}
	graph.fileDeps["b.go"] = []*Dependency{dep2}
	graph.reverseDeps["b.go"] = []*Dependency{dep1}
	graph.reverseDeps["c.go"] = []*Dependency{dep2}

	order := graph.GetTopologicalOrder()

	// In topological order, dependencies come before dependents
	// c.go has no dependencies, so it should come first
	// b.go depends on c.go, so b.go comes after c.go
	// a.go depends on b.go, so a.go comes after b.go
	// Expected order: c.go, b.go, a.go
	cIdx := indexOf(order, "c.go")
	bIdx := indexOf(order, "b.go")
	aIdx := indexOf(order, "a.go")

	if cIdx == -1 || bIdx == -1 || aIdx == -1 {
		t.Fatalf("expected all files in order, got: %v", order)
	}

	if cIdx > bIdx {
		t.Errorf("expected c.go before b.go in topological order (c has no dependencies), got: %v", order)
	}

	if bIdx > aIdx {
		t.Errorf("expected b.go before a.go in topological order (a depends on b), got: %v", order)
	}
}

func TestDependencyGraph_Duration(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte("package main\nfunc Main() {}"), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	if graph.Duration <= 0 {
		t.Error("expected positive duration")
	}

	if graph.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestDependencyGraph_ToMarkdown(t *testing.T) {
	graph := &DependencyGraph{
		RootPath:   "/test",
		TotalFiles: 2,
		TotalDeps:  1,
		Timestamp:  time.Now(),
		allFiles:   map[string]bool{"a.go": true, "b.go": true},
		fileDeps: map[string][]*Dependency{
			"a.go": {{FromFile: "a.go", ToFile: "b.go", Kind: DependencyImport}},
		},
	}

	markdown := graph.ToMarkdown()

	if !strings.Contains(markdown, "# Dependency Graph") {
		t.Error("expected markdown to contain header")
	}
	if !strings.Contains(markdown, "a.go") {
		t.Error("expected markdown to contain a.go")
	}
	if !strings.Contains(markdown, "b.go") {
		t.Error("expected markdown to contain b.go")
	}
}

func TestDependencyKinds(t *testing.T) {
	kinds := []DependencyKind{
		DependencyImport,
		DependencyEmbed,
		DependencyGenerated,
		DependencyTest,
	}

	for _, kind := range kinds {
		if string(kind) == "" {
			t.Errorf("kind should not be empty")
		}
	}
}

func TestDependencyAnalyzer_GoEmbed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create embedded file
	templateFile := filepath.Join(tmpDir, "template.html")
	os.WriteFile(templateFile, []byte("<html></html>"), 0644)

	// Create Go file with embed directive
	goFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import "embed"

//go:embed template.html
var template string

func main() {}
`
	os.WriteFile(goFile, []byte(content), 0644)

	config := DefaultDependencyConfig()
	analyzer := NewDependencyAnalyzer(config, nil)

	ctx := context.Background()
	graph, err := analyzer.Analyze(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Check for embed dependency
	deps := graph.GetDependencies("main.go")
	found := false
	for _, dep := range deps {
		if dep.Kind == DependencyEmbed {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find embed dependency")
	}
}

// Helper function
func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
