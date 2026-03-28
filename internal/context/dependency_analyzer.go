// Package context provides cross-file dependency analysis.
// Task 37: Cross-file dependency analysis for intelligent context management

package context

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DependencyKind represents the type of dependency relationship.
type DependencyKind string

const (
	DependencyImport    DependencyKind = "import"     // Direct import
	DependencyEmbed     DependencyKind = "embed"      // Go embed directive
	DependencyGenerated DependencyKind = "generated"  // Generated from another file
	DependencyTest      DependencyKind = "test"       // Test file dependency
)

// Dependency represents a single dependency relationship between files.
type Dependency struct {
	FromFile    string         `json:"from_file"`
	ToFile      string         `json:"to_file"`
	Kind        DependencyKind `json:"kind"`
	ImportPath  string         `json:"import_path,omitempty"`
	Line        int            `json:"line,omitempty"`
	Strength    int            `json:"strength"` // 1-10, based on usage frequency
}

// DependencyGraph represents the complete dependency graph of a repository.
type DependencyGraph struct {
	RootPath     string                  `json:"root_path"`
	Dependencies []Dependency            `json:"dependencies"`
	
	// Indexes for fast lookup
	fileDeps     map[string][]*Dependency // file -> dependencies from this file
	reverseDeps  map[string][]*Dependency // file -> dependencies to this file
	allFiles     map[string]bool          // all files in graph
	mu           sync.RWMutex
	
	// Stats
	TotalFiles     int           `json:"total_files"`
	TotalDeps      int           `json:"total_deps"`
	Cycles         [][]string    `json:"cycles,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
	Duration       time.Duration `json:"duration"`
}

// DependencyAnalyzer analyzes cross-file dependencies.
type DependencyAnalyzer struct {
	config    *DependencyConfig
	logger    *slog.Logger
	fset      *token.FileSet
	goModules map[string]string // module path -> directory
}

// DependencyConfig holds configuration for dependency analysis.
type DependencyConfig struct {
	IncludeTests      bool     // Include test file dependencies
	IncludeVendor     bool     // Include vendor dependencies
	IncludeGenerated  bool     // Include generated file dependencies
	MaxDepth          int      // Maximum dependency chain depth to analyze
	ExcludePatterns   []string // Files to exclude from analysis
}

// DefaultDependencyConfig returns default configuration.
func DefaultDependencyConfig() *DependencyConfig {
	return &DependencyConfig{
		IncludeTests:     true,
		IncludeVendor:    false,
		IncludeGenerated: false,
		MaxDepth:         10,
		ExcludePatterns:  []string{"vendor/*", "node_modules/*", ".git/*"},
	}
}

// NewDependencyAnalyzer creates a new dependency analyzer.
func NewDependencyAnalyzer(config *DependencyConfig, logger *slog.Logger) *DependencyAnalyzer {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultDependencyConfig()
	}
	
	return &DependencyAnalyzer{
		config:    config,
		logger:    logger.With("component", "dependency_analyzer"),
		fset:      token.NewFileSet(),
		goModules: make(map[string]string),
	}
}

// Analyze performs dependency analysis on a repository.
func (a *DependencyAnalyzer) Analyze(ctx context.Context, rootPath string) (*DependencyGraph, error) {
	startTime := time.Now()
	
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	a.logger.Info("Analyzing dependencies", "path", rootPath)
	
	// Clean path
	rootPath = filepath.Clean(rootPath)
	
	// Verify path exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}
	
	graph := &DependencyGraph{
		RootPath:    rootPath,
		fileDeps:    make(map[string][]*Dependency),
		reverseDeps: make(map[string][]*Dependency),
		allFiles:    make(map[string]bool),
	}
	
	// Detect Go modules
	a.detectGoModules(rootPath)
	
	// Find all source files
	files, err := a.findSourceFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find source files: %w", err)
	}
	
	a.logger.Info("Found source files", "count", len(files))
	
	// Analyze dependencies for each file
	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		deps, err := a.analyzeFile(rootPath, file)
		if err != nil {
			a.logger.Debug("Failed to analyze file", "path", file, "err", err)
			continue
		}
		
		relPath, _ := filepath.Rel(rootPath, file)
		graph.allFiles[relPath] = true
		
		for _, dep := range deps {
			graph.Dependencies = append(graph.Dependencies, dep)
			graph.fileDeps[dep.FromFile] = append(graph.fileDeps[dep.FromFile], &dep)
			graph.reverseDeps[dep.ToFile] = append(graph.reverseDeps[dep.ToFile], &dep)
			graph.allFiles[dep.ToFile] = true
		}
	}
	
	// Detect cycles
	graph.Cycles = a.detectCycles(graph)
	
	// Calculate stats
	graph.TotalFiles = len(graph.allFiles)
	graph.TotalDeps = len(graph.Dependencies)
	graph.Timestamp = time.Now()
	graph.Duration = time.Since(startTime)
	
	a.logger.Info("Dependency analysis complete",
		"files", graph.TotalFiles,
		"dependencies", graph.TotalDeps,
		"cycles", len(graph.Cycles),
		"duration", graph.Duration,
	)
	
	return graph, nil
}

// detectGoModules detects Go modules in the repository.
func (a *DependencyAnalyzer) detectGoModules(rootPath string) {
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "go.mod" {
			dir := filepath.Dir(path)
			// Parse module path from go.mod
			content, err := os.ReadFile(path)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "module ") {
						modulePath := strings.TrimPrefix(line, "module ")
						modulePath = strings.TrimSpace(modulePath)
						a.goModules[modulePath] = dir
						break
					}
				}
			}
		}
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == "node_modules" || info.Name() == ".git") {
			return filepath.SkipDir
		}
		return nil
	})
}

// findSourceFiles finds all source files to analyze.
func (a *DependencyAnalyzer) findSourceFiles(rootPath string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" ||
				name == "dist" || name == "build" || name == "target" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Check exclusion patterns
		relPath, _ := filepath.Rel(rootPath, path)
		for _, pattern := range a.config.ExcludePatterns {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				return nil
			}
		}
		
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			// Skip test files if configured
			if !a.config.IncludeTests && strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
		case ".ts", ".tsx", ".js", ".jsx", ".py", ".rs":
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// analyzeFile analyzes dependencies for a single file.
func (a *DependencyAnalyzer) analyzeFile(rootPath, filePath string) ([]Dependency, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".go":
		return a.analyzeGoFile(rootPath, filePath)
	case ".ts", ".tsx", ".js", ".jsx":
		return a.analyzeTypeScriptFile(rootPath, filePath)
	case ".py":
		return a.analyzePythonFile(rootPath, filePath)
	case ".rs":
		return a.analyzeRustFile(rootPath, filePath)
	default:
		return nil, nil
	}
}

// analyzeGoFile analyzes a Go file's dependencies.
func (a *DependencyAnalyzer) analyzeGoFile(rootPath, filePath string) ([]Dependency, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	f, err := parser.ParseFile(a.fset, filePath, content, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}
	
	var deps []Dependency
	relFromPath, _ := filepath.Rel(rootPath, filePath)
	
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		
		// Try to resolve import to a file
		resolvedPath := a.resolveGoImport(rootPath, filePath, importPath)
		if resolvedPath != "" {
			relToPath, _ := filepath.Rel(rootPath, resolvedPath)
			
			dep := Dependency{
				FromFile:   relFromPath,
				ToFile:     relToPath,
				Kind:       DependencyImport,
				ImportPath: importPath,
				Line:       a.fset.Position(imp.Pos()).Line,
				Strength:   5, // Base strength
			}
			deps = append(deps, dep)
		}
	}
	
	// Check for embed directives
	depDeps := a.extractEmbedDirectives(rootPath, filePath, content)
	deps = append(deps, depDeps...)
	
	return deps, nil
}

// resolveGoImport resolves a Go import path to a file path.
func (a *DependencyAnalyzer) resolveGoImport(rootPath, fromFile, importPath string) string {
	// Check if it's a local import (relative)
	if strings.HasPrefix(importPath, ".") || strings.HasPrefix(importPath, "./") {
		fromDir := filepath.Dir(fromFile)
		resolved := filepath.Join(fromDir, importPath)
		if info, err := os.Stat(resolved); err == nil && info.IsDir() {
			return resolved
		}
		return ""
	}
	
	// Check if it matches a known module
	for modulePath, moduleDir := range a.goModules {
		if strings.HasPrefix(importPath, modulePath) {
			relativePath := strings.TrimPrefix(importPath, modulePath)
			relativePath = strings.TrimPrefix(relativePath, "/")
			resolved := filepath.Join(moduleDir, relativePath)
			
			// Check if directory exists
			if info, err := os.Stat(resolved); err == nil && info.IsDir() {
				return resolved
			}
		}
	}
	
	return ""
}

// extractEmbedDirectives extracts embed directives from Go source.
func (a *DependencyAnalyzer) extractEmbedDirectives(rootPath, filePath string, content []byte) []Dependency {
	var deps []Dependency
	relFromPath, _ := filepath.Rel(rootPath, filePath)
	
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "//go:embed") {
			// Extract embedded file path
			fields := strings.Fields(line)
			for _, field := range fields {
				if field != "//go:embed" && !strings.HasPrefix(field, "//") {
					// This is a file pattern
					fromDir := filepath.Dir(filePath)
					resolved := filepath.Join(fromDir, field)
					
					if _, err := os.Stat(resolved); err == nil {
						relToPath, _ := filepath.Rel(rootPath, resolved)
						dep := Dependency{
							FromFile: relFromPath,
							ToFile:   relToPath,
							Kind:     DependencyEmbed,
							Line:     i + 1,
							Strength: 8, // Embeds are strong dependencies
						}
						deps = append(deps, dep)
					}
				}
			}
		}
	}
	
	return deps
}

// analyzeTypeScriptFile analyzes TypeScript/JavaScript dependencies.
func (a *DependencyAnalyzer) analyzeTypeScriptFile(rootPath, filePath string) ([]Dependency, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var deps []Dependency
	relFromPath, _ := filepath.Rel(rootPath, filePath)
	fromDir := filepath.Dir(filePath)
	
	lines := strings.Split(string(content), "\n")
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Match import statements
		// import ... from '...'
		// import ... from "..."
		// require('...')
		// import('...')
		
		patterns := []struct {
			prefix string
			quote  byte
		}{
			{"import ", '\''},
			{"import ", '"'},
			{"from ", '\''},
			{"from ", '"'},
			{"require(", '\''},
			{"require(", '"'},
			{"import(", '\''},
			{"import(", '"'},
		}
		
		for _, p := range patterns {
			idx := strings.Index(line, p.prefix)
			if idx == -1 {
				continue
			}
			
			rest := line[idx+len(p.prefix):]
			quoteIdx := strings.IndexByte(rest, p.quote)
			if quoteIdx == -1 {
				continue
			}
			
			rest = rest[quoteIdx+1:]
			endIdx := strings.IndexByte(rest, p.quote)
			if endIdx == -1 {
				continue
			}
			
			importPath := rest[:endIdx]
			
			// Skip external packages (node_modules)
			if !strings.HasPrefix(importPath, ".") && !strings.HasPrefix(importPath, "/") {
				continue
			}
			
			// Resolve relative path
			resolved := filepath.Join(fromDir, importPath)
			
			// Try common extensions
			extensions := []string{"", ".ts", ".tsx", ".js", ".jsx", ".json", "/index.ts", "/index.tsx", "/index.js"}
			for _, ext := range extensions {
				tryPath := resolved + ext
				if info, err := os.Stat(tryPath); err == nil && !info.IsDir() {
					relToPath, _ := filepath.Rel(rootPath, tryPath)
					dep := Dependency{
						FromFile:   relFromPath,
						ToFile:     relToPath,
						Kind:       DependencyImport,
						ImportPath: importPath,
						Line:       i + 1,
						Strength:   5,
					}
					deps = append(deps, dep)
					break
				}
			}
		}
	}
	
	return deps, nil
}

// analyzePythonFile analyzes Python dependencies.
func (a *DependencyAnalyzer) analyzePythonFile(rootPath, filePath string) ([]Dependency, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var deps []Dependency
	relFromPath, _ := filepath.Rel(rootPath, filePath)
	fromDir := filepath.Dir(filePath)
	
	lines := strings.Split(string(content), "\n")
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Match import statements
		// import X
		// from X import Y
		// from X import *
		
		if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
			var moduleName string
			
			if strings.HasPrefix(line, "import ") {
				moduleName = strings.Fields(line)[1]
			} else if strings.HasPrefix(line, "from ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					moduleName = fields[1]
				}
			}
			
			if moduleName == "" {
				continue
			}
			
			// Try to resolve to a file
			// Convert module.submodule to module/submodule.py
			modulePath := strings.ReplaceAll(moduleName, ".", string(filepath.Separator))
			
			// Try as file
			tryPaths := []string{
				filepath.Join(fromDir, modulePath+".py"),
				filepath.Join(fromDir, modulePath, "__init__.py"),
			}
			
			for _, tryPath := range tryPaths {
				if info, err := os.Stat(tryPath); err == nil && !info.IsDir() {
					relToPath, _ := filepath.Rel(rootPath, tryPath)
					dep := Dependency{
						FromFile:   relFromPath,
						ToFile:     relToPath,
						Kind:       DependencyImport,
						ImportPath: moduleName,
						Line:       i + 1,
						Strength:   5,
					}
					deps = append(deps, dep)
					break
				}
			}
		}
	}
	
	return deps, nil
}

// analyzeRustFile analyzes Rust dependencies.
func (a *DependencyAnalyzer) analyzeRustFile(rootPath, filePath string) ([]Dependency, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var deps []Dependency
	relFromPath, _ := filepath.Rel(rootPath, filePath)
	fromDir := filepath.Dir(filePath)
	
	lines := strings.Split(string(content), "\n")
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		// Match mod declarations
		// mod submodule;
		if strings.HasPrefix(line, "mod ") && strings.HasSuffix(line, ";") {
			modName := strings.TrimPrefix(line, "mod ")
			modName = strings.TrimSuffix(modName, ";")
			modName = strings.TrimSpace(modName)
			
			// Try to resolve to a file
			tryPaths := []string{
				filepath.Join(fromDir, modName+".rs"),
				filepath.Join(fromDir, modName, "mod.rs"),
			}
			
			for _, tryPath := range tryPaths {
				if info, err := os.Stat(tryPath); err == nil && !info.IsDir() {
					relToPath, _ := filepath.Rel(rootPath, tryPath)
					dep := Dependency{
						FromFile:   relFromPath,
						ToFile:     relToPath,
						Kind:       DependencyImport,
						ImportPath: modName,
						Line:       i + 1,
						Strength:   5,
					}
					deps = append(deps, dep)
					break
				}
			}
		}
		
		// Match use declarations
		// use crate::module::Item;
		if strings.HasPrefix(line, "use ") {
			// Extract the module path
			usePath := strings.TrimPrefix(line, "use ")
			usePath = strings.TrimSuffix(usePath, ";")
			usePath = strings.TrimSpace(usePath)
			
			// Handle crate:: paths
			if strings.HasPrefix(usePath, "crate::") {
				modulePath := strings.TrimPrefix(usePath, "crate::")
				modulePath = strings.Split(modulePath, "::")[0]
				
				// Try to resolve
				tryPaths := []string{
					filepath.Join(rootPath, modulePath+".rs"),
					filepath.Join(rootPath, modulePath, "mod.rs"),
				}
				
				for _, tryPath := range tryPaths {
					if info, err := os.Stat(tryPath); err == nil && !info.IsDir() {
						relToPath, _ := filepath.Rel(rootPath, tryPath)
						dep := Dependency{
							FromFile:   relFromPath,
							ToFile:     relToPath,
							Kind:       DependencyImport,
							ImportPath: usePath,
							Line:       i + 1,
							Strength:   5,
						}
						deps = append(deps, dep)
						break
					}
				}
			}
		}
	}
	
	return deps, nil
}

// detectCycles detects circular dependencies in the graph.
func (a *DependencyAnalyzer) detectCycles(graph *DependencyGraph) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	var dfs func(path []string, current string) [][]string
	dfs = func(path []string, current string) [][]string {
		var foundCycles [][]string
		
		visited[current] = true
		recStack[current] = true
		
		for _, dep := range graph.fileDeps[current] {
			if !visited[dep.ToFile] {
				foundCycles = append(foundCycles, dfs(append(path, current), dep.ToFile)...)
			} else if recStack[dep.ToFile] {
				// Found cycle
				cycle := append(path, current, dep.ToFile)
				// Trim to just the cycle
				for i, p := range cycle {
					if p == dep.ToFile {
						cycle = cycle[i:]
						break
					}
				}
				foundCycles = append(foundCycles, cycle)
			}
		}
		
		recStack[current] = false
		return foundCycles
	}
	
	for file := range graph.allFiles {
		if !visited[file] {
			cycles = append(cycles, dfs([]string{}, file)...)
		}
	}
	
	return cycles
}

// GetDependencies returns all dependencies from a file.
func (g *DependencyGraph) GetDependencies(file string) []*Dependency {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.fileDeps[file]
}

// GetDependents returns all files that depend on a file.
func (g *DependencyGraph) GetDependents(file string) []*Dependency {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.reverseDeps[file]
}

// GetRelatedFiles returns all files related to a file (both dependencies and dependents).
func (g *DependencyGraph) GetRelatedFiles(file string, maxDepth int) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	related := make(map[string]bool)
	related[file] = true
	
	// BFS to find related files
	queue := []string{file}
	depth := make(map[string]int)
	depth[file] = 0
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		currentDepth := depth[current]
		if currentDepth >= maxDepth {
			continue
		}
		
		// Add dependencies
		for _, dep := range g.fileDeps[current] {
			if !related[dep.ToFile] {
				related[dep.ToFile] = true
				depth[dep.ToFile] = currentDepth + 1
				queue = append(queue, dep.ToFile)
			}
		}
		
		// Add dependents
		for _, dep := range g.reverseDeps[current] {
			if !related[dep.FromFile] {
				related[dep.FromFile] = true
				depth[dep.FromFile] = currentDepth + 1
				queue = append(queue, dep.FromFile)
			}
		}
	}
	
	// Convert to sorted slice
	var result []string
	for f := range related {
		if f != file {
			result = append(result, f)
		}
	}
	sort.Strings(result)
	
	return result
}

// HasCycles returns true if the graph has circular dependencies.
func (g *DependencyGraph) HasCycles() bool {
	return len(g.Cycles) > 0
}

// GetTopologicalOrder returns files in topological order (dependencies first).
func (g *DependencyGraph) GetTopologicalOrder() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Kahn's algorithm for dependency graph
	// We want files with no dependencies (leaf nodes) to come first
	// fileDeps[A] = [B] means A depends on B (A -> B edge)
	// For topological order: B must come before A
	
	// Count how many dependencies each file has (out-degree)
	depCount := make(map[string]int)
	for file := range g.allFiles {
		depCount[file] = len(g.fileDeps[file])
	}
	
	// Find all files with no dependencies
	var queue []string
	for file, count := range depCount {
		if count == 0 {
			queue = append(queue, file)
		}
	}
	sort.Strings(queue) // Deterministic order
	
	var result []string
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// Find files that depend on current (reverse deps)
		var newQueue []string
		for _, dep := range g.reverseDeps[current] {
			depCount[dep.FromFile]--
			if depCount[dep.FromFile] == 0 {
				newQueue = append(newQueue, dep.FromFile)
			}
		}
		sort.Strings(newQueue)
		queue = append(queue, newQueue...)
	}
	
	return result
}

// ToMarkdown generates a markdown representation of the dependency graph.
func (g *DependencyGraph) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString("# Dependency Graph\n\n")
	sb.WriteString(fmt.Sprintf("- **Root**: `%s`\n", g.RootPath))
	sb.WriteString(fmt.Sprintf("- **Files**: %d\n", g.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Dependencies**: %d\n", g.TotalDeps))
	sb.WriteString(fmt.Sprintf("- **Cycles**: %d\n", len(g.Cycles)))
	sb.WriteString(fmt.Sprintf("- **Generated**: %s\n", g.Timestamp.Format(time.RFC3339)))
	sb.WriteString("\n---\n\n")
	
	if len(g.Cycles) > 0 {
		sb.WriteString("## ⚠️ Circular Dependencies\n\n")
		for i, cycle := range g.Cycles {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.Join(cycle, " → ")))
		}
		sb.WriteString("\n---\n\n")
	}
	
	// Group dependencies by file
	sb.WriteString("## Dependencies by File\n\n")
	
	var files []string
	for file := range g.allFiles {
		files = append(files, file)
	}
	sort.Strings(files)
	
	for _, file := range files {
		deps := g.fileDeps[file]
		if len(deps) == 0 {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("### `%s`\n\n", file))
		
		for _, dep := range deps {
			sb.WriteString(fmt.Sprintf("- → `%s` (%s", dep.ToFile, dep.Kind))
			if dep.ImportPath != "" {
				sb.WriteString(fmt.Sprintf(", %s", dep.ImportPath))
			}
			sb.WriteString(")\n")
		}
		sb.WriteString("\n")
	}
	
	return sb.String()
}
