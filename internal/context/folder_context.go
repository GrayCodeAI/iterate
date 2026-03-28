// Package context provides folder-level context capabilities.
// Task 46: @folder support for directory-level context

package context

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FolderContextConfig holds configuration for folder context.
type FolderContextConfig struct {
	MaxFiles           int           `json:"max_files"`            // Max files per folder
	MaxDepth           int           `json:"max_depth"`            // Max recursion depth
	MaxTotalSize       int64         `json:"max_total_size"`       // Max total size in bytes
	ExcludePatterns    []string      `json:"exclude_patterns"`     // Patterns to exclude
	IncludeHidden      bool          `json:"include_hidden"`       // Include hidden files
	PriorityExtensions []string      `json:"priority_extensions"`  // Extensions to prioritize
	SummaryMode        string        `json:"summary_mode"`         // "full", "structure", "stats"
	CacheTTL           time.Duration `json:"cache_ttl"`            // Cache time-to-live
}

// DefaultFolderContextConfig returns default configuration.
func DefaultFolderContextConfig() *FolderContextConfig {
	return &FolderContextConfig{
		MaxFiles:           50,
		MaxDepth:           3,
		MaxTotalSize:       500 * 1024, // 500KB total
		ExcludePatterns:    []string{".git", "node_modules", "vendor", "__pycache__", "*.lock"},
		IncludeHidden:      false,
		PriorityExtensions: []string{".go", ".ts", ".tsx", ".py", ".rs", ".java"},
		SummaryMode:        "full",
		CacheTTL:           5 * time.Minute,
	}
}

// FolderInfo represents information about a folder.
type FolderInfo struct {
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	FileCount      int               `json:"file_count"`
	DirCount       int               `json:"dir_count"`
	TotalSize      int64             `json:"total_size"`
	Files          []*FileSummary    `json:"files,omitempty"`
	Subdirs        []string          `json:"subdirs,omitempty"`
	Extensions     map[string]int    `json:"extensions"`
	ModTime        time.Time         `json:"mod_time"`
	HasReadme      bool              `json:"has_readme"`
	ReadmePath     string            `json:"readme_path,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// FileSummary represents a summary of a file.
type FileSummary struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Ext         string    `json:"ext"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsDir       bool      `json:"is_dir"`
	IsHidden    bool      `json:"is_hidden"`
	TokenEst    int       `json:"token_est"`
	Priority    int       `json:"priority"` // Higher = more important
}

// FolderContextResult represents the result of folder context gathering.
type FolderContextResult struct {
	Folder       *FolderInfo   `json:"folder"`
	Depth        int           `json:"depth"`
	TotalFiles   int           `json:"total_files"`
	TotalSize    int64         `json:"total_size"`
	GatherTime   time.Duration `json:"gather_time"`
	Truncated    bool          `json:"truncated"` // True if limits were hit
}

// FolderContextManager manages folder-level context gathering.
type FolderContextManager struct {
	config     *FolderContextConfig
	logger     *slog.Logger
	mu         sync.RWMutex
	
	// Cache
	folderCache map[string]*FolderContextResult
	cacheExpiry time.Time
	
	// Token estimator
	tokenEstimator func(string) int
}

// NewFolderContextManager creates a new folder context manager.
func NewFolderContextManager(config *FolderContextConfig, logger *slog.Logger) *FolderContextManager {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultFolderContextConfig()
	}
	
	return &FolderContextManager{
		config:        config,
		logger:        logger.With("component", "folder_context"),
		folderCache:   make(map[string]*FolderContextResult),
		tokenEstimator: EstimateTokens,
	}
}

// GatherFolder gathers context for a folder.
func (fcm *FolderContextManager) GatherFolder(ctx context.Context, folderPath string, depth int) (*FolderContextResult, error) {
	start := time.Now()
	
	fcm.mu.RLock()
	// Check cache
	if result, ok := fcm.folderCache[folderPath]; ok && time.Now().Before(fcm.cacheExpiry) {
		fcm.mu.RUnlock()
		return result, nil
	}
	fcm.mu.RUnlock()
	
	result := &FolderContextResult{
		Depth: depth,
	}
	
	// Validate folder exists
	info, err := os.Stat(folderPath)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %s", folderPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", folderPath)
	}
	
	folderInfo := &FolderInfo{
		Path:       folderPath,
		Name:       filepath.Base(folderPath),
		Extensions: make(map[string]int),
		ModTime:    info.ModTime(),
		Files:      make([]*FileSummary, 0),
		Subdirs:    make([]string, 0),
		Metadata:   make(map[string]string),
	}
	
	// Gather files
	var totalSize int64
	var totalFiles int
	truncated := false
	
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err != nil {
			return nil // Skip errors
		}
		
		// Check depth
		relPath, _ := filepath.Rel(folderPath, path)
		currentDepth := len(strings.Split(relPath, string(os.PathSeparator)))
		if currentDepth > fcm.config.MaxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Check exclusions
		if fcm.shouldExclude(path, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Check limits
		if totalFiles >= fcm.config.MaxFiles {
			truncated = true
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		if totalSize > fcm.config.MaxTotalSize {
			truncated = true
			return nil
		}
		
		// Process file/directory
		if info.IsDir() {
			if path != folderPath {
				folderInfo.Subdirs = append(folderInfo.Subdirs, path)
				folderInfo.DirCount++
			}
		} else {
			summary := &FileSummary{
				Path:     path,
				Name:     info.Name(),
				Ext:      filepath.Ext(info.Name()),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				IsDir:    false,
				IsHidden: strings.HasPrefix(info.Name(), "."),
			}
			
			// Calculate priority
			summary.Priority = fcm.calculatePriority(summary)
			
			// Estimate tokens
			content, err := os.ReadFile(path)
			if err == nil && fcm.tokenEstimator != nil {
				summary.TokenEst = fcm.tokenEstimator(string(content))
			}
			
			folderInfo.Files = append(folderInfo.Files, summary)
			folderInfo.FileCount++
			totalFiles++
			totalSize += info.Size()
			
			// Track extensions
			if summary.Ext != "" {
				folderInfo.Extensions[summary.Ext]++
			}
			
			// Check for README
			name := strings.ToLower(info.Name())
			if name == "readme.md" || name == "readme.txt" || name == "readme" {
				folderInfo.HasReadme = true
				folderInfo.ReadmePath = path
			}
		}
		
		return nil
	})
	
	if err != nil && err != context.Canceled {
		fcm.logger.Debug("Error walking folder", "path", folderPath, "error", err)
	}
	
	// Sort files by priority
	sort.Slice(folderInfo.Files, func(i, j int) bool {
		return folderInfo.Files[i].Priority > folderInfo.Files[j].Priority
	})
	
	folderInfo.TotalSize = totalSize
	result.Folder = folderInfo
	result.TotalFiles = totalFiles
	result.TotalSize = totalSize
	result.GatherTime = time.Since(start)
	result.Truncated = truncated
	
	// Cache result
	fcm.mu.Lock()
	fcm.folderCache[folderPath] = result
	fcm.cacheExpiry = time.Now().Add(fcm.config.CacheTTL)
	fcm.mu.Unlock()
	
	return result, nil
}

// shouldExclude checks if a path should be excluded.
func (fcm *FolderContextManager) shouldExclude(path string, info os.FileInfo) bool {
	name := info.Name()
	
	// Check hidden files
	if !fcm.config.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}
	
	// Check exclusion patterns
	for _, pattern := range fcm.config.ExcludePatterns {
		// Exact match
		if name == pattern {
			return true
		}
		// Glob match
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
		// Directory match
		if info.IsDir() && strings.Contains(pattern, "/") {
			if strings.Contains(path, pattern) {
				return true
			}
		}
	}
	
	return false
}

// calculatePriority calculates the priority of a file.
func (fcm *FolderContextManager) calculatePriority(summary *FileSummary) int {
	priority := 0
	
	// Check priority extensions
	for i, ext := range fcm.config.PriorityExtensions {
		if summary.Ext == ext {
			priority = 100 - i // Higher priority for earlier extensions
			break
		}
	}
	
	// Boost for README
	name := strings.ToLower(summary.Name)
	if name == "readme.md" || name == "readme.txt" {
		priority += 50
	}
	
	// Boost for common config files
	if summary.Name == "go.mod" || summary.Name == "package.json" || 
	   summary.Name == "Cargo.toml" || summary.Name == "requirements.txt" {
		priority += 40
	}
	
	// Boost for main files
	if summary.Name == "main.go" || summary.Name == "index.ts" || 
	   summary.Name == "index.js" || summary.Name == "__init__.py" {
		priority += 30
	}
	
	// Penalty for test files
	if strings.HasSuffix(summary.Name, "_test.go") || 
	   strings.Contains(summary.Name, ".test.") ||
	   strings.HasSuffix(summary.Name, "_test.py") {
		priority -= 10
	}
	
	// Penalty for hidden files
	if summary.IsHidden {
		priority -= 20
	}
	
	return priority
}

// GatherMultipleFolders gathers context for multiple folders.
func (fcm *FolderContextManager) GatherMultipleFolders(ctx context.Context, folderPaths []string, depth int) ([]*FolderContextResult, error) {
	results := make([]*FolderContextResult, 0, len(folderPaths))
	
	for _, path := range folderPaths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		result, err := fcm.GatherFolder(ctx, path, depth)
		if err != nil {
			fcm.logger.Debug("Error gathering folder", "path", path, "error", err)
			continue
		}
		results = append(results, result)
	}
	
	return results, nil
}

// GetFolderStructure returns a tree structure of the folder.
func (fcm *FolderContextManager) GetFolderStructure(ctx context.Context, folderPath string, maxDepth int) (string, error) {
	var sb strings.Builder
	
	err := fcm.buildTree(ctx, folderPath, "", 0, maxDepth, &sb)
	if err != nil {
		return "", err
	}
	
	return sb.String(), nil
}

// buildTree recursively builds a tree representation.
func (fcm *FolderContextManager) buildTree(ctx context.Context, path string, prefix string, depth int, maxDepth int, sb *strings.Builder) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	if depth > maxDepth {
		return nil
	}
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	
	// Sort entries
	sort.Slice(entries, func(i, j int) bool {
		// Dirs first
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	
	for i, entry := range entries {
		// Skip hidden and excluded
		info, _ := entry.Info()
		if info != nil && fcm.shouldExclude(filepath.Join(path, entry.Name()), info) {
			continue
		}
		
		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		
		sb.WriteString(prefix + connector + entry.Name() + "\n")
		
		if entry.IsDir() {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			fcm.buildTree(ctx, filepath.Join(path, entry.Name()), newPrefix, depth+1, maxDepth, sb)
		}
	}
	
	return nil
}

// ClearCache clears the folder cache.
func (fcm *FolderContextManager) ClearCache() {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.folderCache = make(map[string]*FolderContextResult)
}

// GetStats returns statistics about the manager.
func (fcm *FolderContextManager) GetStats() map[string]interface{} {
	fcm.mu.RLock()
	defer fcm.mu.RUnlock()
	
	return map[string]interface{}{
		"cached_folders": len(fcm.folderCache),
		"max_files":      fcm.config.MaxFiles,
		"max_depth":      fcm.config.MaxDepth,
		"max_total_size": fcm.config.MaxTotalSize,
	}
}

// UpdateConfig updates the manager configuration.
func (fcm *FolderContextManager) UpdateConfig(config *FolderContextConfig) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.config = config
}

// ResolveFolderPath resolves a folder path from a mention.
func (fcm *FolderContextManager) ResolveFolderPath(mention string) (string, bool) {
	// Remove @folder prefix if present
	mention = strings.TrimPrefix(mention, "@folder")
	mention = strings.TrimSpace(mention)
	
	// Check if it's an absolute path
	if filepath.IsAbs(mention) {
		if info, err := os.Stat(mention); err == nil && info.IsDir() {
			return mention, true
		}
		return "", false
	}
	
	// Try relative path
	if info, err := os.Stat(mention); err == nil && info.IsDir() {
		return mention, true
	}
	
	return "", false
}

// ToMarkdown generates a markdown representation of the folder context result.
func (r *FolderContextResult) ToMarkdown() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# Folder: %s\n\n", r.Folder.Name))
	sb.WriteString(fmt.Sprintf("**Path:** `%s`\n\n", r.Folder.Path))
	sb.WriteString(fmt.Sprintf("- **Files:** %d\n", r.Folder.FileCount))
	sb.WriteString(fmt.Sprintf("- **Subdirs:** %d\n", r.Folder.DirCount))
	sb.WriteString(fmt.Sprintf("- **Total Size:** %s\n", formatSize(r.Folder.TotalSize)))
	sb.WriteString(fmt.Sprintf("- **Gather Time:** %v\n\n", r.GatherTime))
	
	if r.Truncated {
		sb.WriteString("⚠️ *Results truncated due to size limits*\n\n")
	}
	
	if r.Folder.HasReadme {
		sb.WriteString(fmt.Sprintf("📄 **README:** `%s`\n\n", r.Folder.ReadmePath))
	}
	
	// Extensions
	if len(r.Folder.Extensions) > 0 {
		sb.WriteString("### Extensions\n\n")
		for ext, count := range r.Folder.Extensions {
			sb.WriteString(fmt.Sprintf("- `%s`: %d files\n", ext, count))
		}
		sb.WriteString("\n")
	}
	
	// Top files by priority
	if len(r.Folder.Files) > 0 {
		sb.WriteString("### Top Files\n\n")
		maxShow := 10
		if len(r.Folder.Files) < maxShow {
			maxShow = len(r.Folder.Files)
		}
		for i := 0; i < maxShow; i++ {
			f := r.Folder.Files[i]
			sb.WriteString(fmt.Sprintf("- `%s` (%s, priority: %d)\n", f.Name, formatSize(f.Size), f.Priority))
		}
	}
	
	return sb.String()
}

// formatSize formats a size in bytes to human readable.
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
