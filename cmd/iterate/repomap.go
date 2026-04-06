package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// repoMapEntry holds the structural summary for one file.
type repoMapEntry struct {
	Path    string
	Symbols []string // function/type/class names
}

// repoMapConfig controls what the repo map includes.
type repoMapConfig struct {
	MaxFiles    int
	MaxSymbols  int // per file
	Extensions  []string
	ExcludeDirs []string
}

func defaultRepoMapConfig() repoMapConfig {
	return repoMapConfig{
		MaxFiles:   200,
		MaxSymbols: 20,
		Extensions: []string{
			".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs",
			".java", ".kt", ".rb", ".swift", ".cpp", ".c", ".h",
		},
		ExcludeDirs: []string{
			".git", "node_modules", "vendor", ".iterate",
			"dist", "build", "__pycache__", ".next",
		},
	}
}

// BuildRepoMap walks repoPath and returns a structural summary string.
// It extracts top-level symbols (functions, types, classes) from source files.
func BuildRepoMap(repoPath string, cfg repoMapConfig) string {
	entries := collectRepoMapEntries(repoPath, cfg)
	if len(entries) == 0 {
		return "(empty repo map)"
	}
	return renderRepoMap(entries, repoPath)
}

func collectRepoMapEntries(repoPath string, cfg repoMapConfig) []repoMapEntry {
	var entries []repoMapEntry
	extSet := make(map[string]bool, len(cfg.Extensions))
	for _, e := range cfg.Extensions {
		extSet[e] = true
	}
	excludeSet := make(map[string]bool, len(cfg.ExcludeDirs))
	for _, d := range cfg.ExcludeDirs {
		excludeSet[d] = true
	}

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if excludeSet[info.Name()] || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !extSet[filepath.Ext(path)] {
			return nil
		}
		if len(entries) >= cfg.MaxFiles {
			return filepath.SkipDir
		}

		symbols := extractSymbols(path, cfg.MaxSymbols)
		rel, _ := filepath.Rel(repoPath, path)
		entries = append(entries, repoMapEntry{Path: rel, Symbols: symbols})
		return nil
	})

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

// renderRepoMap formats the entries into a compact text representation.
func renderRepoMap(entries []repoMapEntry, repoPath string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Repo map: %s\n\n", filepath.Base(repoPath)))
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("## %s\n", e.Path))
		for _, sym := range e.Symbols {
			sb.WriteString(fmt.Sprintf("  %s\n", sym))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ── Symbol extractors ─────────────────────────────────────────────────────────

var (
	reGoFunc      = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)
	reGoType      = regexp.MustCompile(`^type\s+(\w+)\s+`)
	rePyDef       = regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`)
	rePyClass     = regexp.MustCompile(`^class\s+(\w+)`)
	reJSFunc      = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	reJSConst     = regexp.MustCompile(`^(?:export\s+)?(?:const|let)\s+(\w+)\s*=\s*(?:async\s*)?\(`)
	reJSClass     = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?class\s+(\w+)`)
	reTSInterface = regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`)
	reTSType      = regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)\s*=`)
	reRustFn      = regexp.MustCompile(`^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)
	reRustStruct  = regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`)
	reRustEnum    = regexp.MustCompile(`^(?:pub\s+)?enum\s+(\w+)`)
	reJavaMethod  = regexp.MustCompile(`^\s*(?:public|private|protected|static|final|abstract|synchronized)\s+(?:public|private|protected|static|final|abstract|synchronized|\s)*\w+\s+(\w+)\s*\(`)
	reRubyDef     = regexp.MustCompile(`^\s*def\s+(\w+)`)
	reRubyClass   = regexp.MustCompile(`^\s*class\s+(\w+)`)
)

func extractSymbols(path string, max int) []string {
	ext := strings.ToLower(filepath.Ext(path))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var symbols []string
	seen := make(map[string]bool)

	for _, line := range lines {
		if len(symbols) >= max {
			break
		}
		sym := extractSymbolFromLine(line, ext)
		if sym != "" && !seen[sym] {
			seen[sym] = true
			symbols = append(symbols, sym)
		}
	}
	return symbols
}

func extractSymbolFromLine(line, ext string) string {
	switch ext {
	case ".go":
		if m := reGoFunc.FindStringSubmatch(line); m != nil {
			return "func " + m[1]
		}
		if m := reGoType.FindStringSubmatch(line); m != nil {
			return "type " + m[1]
		}
	case ".py":
		if m := rePyDef.FindStringSubmatch(line); m != nil {
			return "def " + m[1]
		}
		if m := rePyClass.FindStringSubmatch(line); m != nil {
			return "class " + m[1]
		}
	case ".ts", ".tsx":
		if m := reTSInterface.FindStringSubmatch(line); m != nil {
			return "interface " + m[1]
		}
		if m := reTSType.FindStringSubmatch(line); m != nil {
			return "type " + m[1]
		}
		fallthrough
	case ".js", ".jsx":
		if m := reJSClass.FindStringSubmatch(line); m != nil {
			return "class " + m[1]
		}
		if m := reJSFunc.FindStringSubmatch(line); m != nil {
			return "function " + m[1]
		}
		if m := reJSConst.FindStringSubmatch(line); m != nil {
			return "const " + m[1]
		}
	case ".rs":
		if m := reRustFn.FindStringSubmatch(line); m != nil {
			return "fn " + m[1]
		}
		if m := reRustStruct.FindStringSubmatch(line); m != nil {
			return "struct " + m[1]
		}
		if m := reRustEnum.FindStringSubmatch(line); m != nil {
			return "enum " + m[1]
		}
	case ".java", ".kt":
		if m := reJavaMethod.FindStringSubmatch(line); m != nil {
			return "method " + m[1]
		}
	case ".rb":
		if m := reRubyDef.FindStringSubmatch(line); m != nil {
			return "def " + m[1]
		}
		if m := reRubyClass.FindStringSubmatch(line); m != nil {
			return "class " + m[1]
		}
	}
	return ""
}

// repoMapCache caches the last-built map to avoid repeated filesystem walks.
// Protected by repoMapCacheMu since InvalidateRepoMap can be called from
// tool-execution goroutines (via replHooks.OnToolEnd).
var (
	repoMapCacheMu   sync.Mutex
	repoMapCacheData struct {
		repoPath string
		content  string
	}
)

// CachedRepoMap returns a cached repo map, rebuilding if the path changed.
func CachedRepoMap(repoPath string) string {
	repoMapCacheMu.Lock()
	defer repoMapCacheMu.Unlock()
	if repoMapCacheData.repoPath == repoPath && repoMapCacheData.content != "" {
		return repoMapCacheData.content
	}
	cfg := defaultRepoMapConfig()
	content := BuildRepoMap(repoPath, cfg)
	repoMapCacheData.repoPath = repoPath
	repoMapCacheData.content = content
	return content
}

// InvalidateRepoMap clears the cache so the next call rebuilds.
func InvalidateRepoMap() {
	repoMapCacheMu.Lock()
	defer repoMapCacheMu.Unlock()
	repoMapCacheData.repoPath = ""
	repoMapCacheData.content = ""
}
