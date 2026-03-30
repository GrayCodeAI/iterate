package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type FileInfo struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Language   string   `json:"language"`
	Size       int64    `json:"size"`
	Functions  []string `json:"functions"`
	Classes    []string `json:"classes"`
	Interfaces []string `json:"interfaces"`
	Imports    []string `json:"imports"`
	Exports    []string `json:"exports"`
}

type RepoMap struct {
	RootPath   string               `json:"root_path"`
	Files      map[string]*FileInfo `json:"files"`
	Functions  map[string][]string  `json:"functions"`
	Classes    map[string][]string  `json:"classes"`
	Imports    map[string][]string  `json:"imports"`
	DepGraph   map[string][]string  `json:"dependency_graph"`
	LastUpdate string               `json:"last_update"`
	mu         sync.RWMutex
}

var LanguageExtensions = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".jsx":   "javascript",
	".java":  "java",
	".c":     "c",
	".cpp":   "cpp",
	".h":     "c",
	".hpp":   "cpp",
	".rs":    "rust",
	".rb":    "ruby",
	".php":   "php",
	".cs":    "csharp",
	".swift": "swift",
	".kt":    "kotlin",
	".scala": "scala",
	".html":  "html",
	".css":   "css",
	".scss":  "scss",
	".sql":   "sql",
	".sh":    "bash",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".md":    "markdown",
}

func NewRepoMap(rootPath string) *RepoMap {
	return &RepoMap{
		RootPath:  rootPath,
		Files:     make(map[string]*FileInfo),
		Functions: make(map[string][]string),
		Classes:   make(map[string][]string),
		Imports:   make(map[string][]string),
		DepGraph:  make(map[string][]string),
	}
}

func (rm *RepoMap) Build() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	err := filepath.Walk(rm.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		lang, ok := LanguageExtensions[ext]
		if !ok {
			return nil
		}

		relPath, err := filepath.Rel(rm.RootPath, path)
		if err != nil {
			return nil
		}

		fileInfo := &FileInfo{
			Path:     relPath,
			Name:     info.Name(),
			Language: lang,
			Size:     info.Size(),
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		switch lang {
		case "go":
			rm.analyzeGoFile(fileInfo, string(content))
		case "javascript", "typescript":
			rm.analyzeJSFile(fileInfo, string(content))
		case "python":
			rm.analyzePythonFile(fileInfo, string(content))
		}

		rm.Files[relPath] = fileInfo
		rm.DepGraph[relPath] = fileInfo.Imports

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to build repo map: %w", err)
	}

	return nil
}

func (rm *RepoMap) analyzeGoFile(info *FileInfo, content string) {
	funcRegex := regexp.MustCompile(`^func\s+(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	interfaceRegex := regexp.MustCompile(`^type\s+(\w+)\s+interface`)
	importRegex := regexp.MustCompile(`^import\s+\(([^)]+)\)"|^import\s+"([^"]+)"`)
	exportRegex := regexp.MustCompile(`^func\s+\([a-z]+\s+\*?\w+\)\s+(\w+)\s*\(|^func\s+(\w+)\s*\(`)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Functions = append(info.Functions, matches[1])
		}
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Classes = append(info.Classes, matches[1])
		}
		if matches := interfaceRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Interfaces = append(info.Interfaces, matches[1])
		}
		if matches := exportRegex.FindStringSubmatch(line); len(matches) > 1 {
			for _, m := range matches[1:] {
				if m != "" {
					info.Exports = append(info.Exports, m)
				}
			}
		}
	}

	for _, match := range importRegex.FindAllStringSubmatch(content, -1) {
		for _, imp := range match[1:] {
			if imp != "" {
				info.Imports = append(info.Imports, cleanImport(imp))
			}
		}
	}
}

func (rm *RepoMap) analyzeJSFile(info *FileInfo, content string) {
	funcRegex := regexp.MustCompile(`(?:function\s+(\w+)|const\s+(\w+)\s*=\s*(?:async\s*)?\(|(\w+)\s*:\s*(?:async\s*)?\([^)]*\)\s*=>)`)
	classRegex := regexp.MustCompile(`class\s+(\w+)`)
	importRegex := regexp.MustCompile(`import\s+.*?from\s+['"]([^'"]+)['"]`)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			for _, m := range matches[1:] {
				if m != "" {
					info.Functions = append(info.Functions, m)
				}
			}
		}
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Classes = append(info.Classes, matches[1])
		}
	}

	for _, match := range importRegex.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 {
			info.Imports = append(info.Imports, match[1])
		}
	}
}

func (rm *RepoMap) analyzePythonFile(info *FileInfo, content string) {
	funcRegex := regexp.MustCompile(`^def\s+(\w+)\s*\(`)
	classRegex := regexp.MustCompile(`^class\s+(\w+)`)
	importRegex := regexp.MustCompile(`^(?:from\s+(\S+)\s+import|import\s+(\S+))`)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Functions = append(info.Functions, matches[1])
		}
		if matches := classRegex.FindStringSubmatch(line); len(matches) > 1 {
			info.Classes = append(info.Classes, matches[1])
		}
	}

	for _, match := range importRegex.FindAllStringSubmatch(content, -1) {
		for _, m := range match[1:] {
			if m != "" && !strings.HasPrefix(m, ".") {
				info.Imports = append(info.Imports, m)
			}
		}
	}
}

func cleanImport(imp string) string {
	imp = strings.Trim(imp, `"`)
	if idx := strings.Index(imp, " "); idx > 0 {
		imp = imp[:idx]
	}
	return imp
}

func shouldSkipDir(name string) bool {
	skipDirs := []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor", "dist", "build",
		".next", ".nuxt", ".svelte-kit",
		"__pycache__", ".pytest_cache",
		"venv", ".venv", "env",
		".idea", ".vscode",
	}
	for _, s := range skipDirs {
		if name == s {
			return true
		}
	}
	return false
}

func (rm *RepoMap) ToJSON() (string, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	data, err := json.MarshalIndent(rm, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (rm *RepoMap) ToPrompt() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## Repository Map\n\n")

	for path, info := range rm.Files {
		if info.Language != "go" {
			continue
		}

		sb.WriteString(fmt.Sprintf("- %s (%s, %d bytes)\n", path, info.Language, info.Size))

		if len(info.Functions) > 0 {
			sb.WriteString(fmt.Sprintf("  - functions: %s\n", strings.Join(info.Functions[:min(5, len(info.Functions))], ", ")))
		}
		if len(info.Classes) > 0 {
			sb.WriteString(fmt.Sprintf("  - classes: %s\n", strings.Join(info.Classes, ", ")))
		}
		if len(info.Imports) > 0 {
			sb.WriteString(fmt.Sprintf("  - imports: %s\n", strings.Join(info.Imports[:min(3, len(info.Imports))], ", ")))
		}
	}

	return sb.String()
}

func (rm *RepoMap) FindFilesByKeyword(keyword string) []*FileInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var results []*FileInfo
	keyword = strings.ToLower(keyword)

	for _, info := range rm.Files {
		if strings.Contains(strings.ToLower(info.Path), keyword) {
			results = append(results, info)
			continue
		}
		for _, fn := range info.Functions {
			if strings.Contains(strings.ToLower(fn), keyword) {
				results = append(results, info)
				break
			}
		}
	}

	return results
}

func (rm *RepoMap) FindRelatedFiles(filePath string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	related := make(map[string]bool)
	related[filePath] = true

	info, ok := rm.Files[filePath]
	if !ok {
		return nil
	}

	for _, imp := range info.Imports {
		for path := range rm.Files {
			if strings.Contains(path, imp) || strings.Contains(imp, path) {
				related[path] = true
			}
		}
	}

	for imp := range rm.DepGraph {
		for _, fi := range rm.Files {
			for _, fimp := range fi.Imports {
				if fimp == imp {
					related[fi.Path] = true
				}
			}
		}
	}

	result := make([]string, 0, len(related))
	for path := range related {
		result = append(result, path)
	}

	return result
}

func (rm *RepoMap) Save(path string) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	data, err := json.MarshalIndent(rm, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadRepoMap(path string) (*RepoMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rm := &RepoMap{}
	if err := json.Unmarshal(data, rm); err != nil {
		return nil, err
	}

	return rm, nil
}

func (e *Engine) BuildRepoMap() (*RepoMap, error) {
	rm := NewRepoMap(e.repoPath)
	if err := rm.Build(); err != nil {
		return nil, err
	}
	return rm, nil
}

func (e *Engine) GetRepoMapForPrompt() (string, error) {
	rm, err := e.BuildRepoMap()
	if err != nil {
		return "", err
	}
	return rm.ToPrompt(), nil
}
