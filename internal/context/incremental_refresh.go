// Package context provides incremental context refresh capabilities.
// Task 42: Incremental Context Refresh - only process changed files

package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileChange represents a change to a file.
type FileChange struct {
	Path       string    `json:"path"`
	OldHash    string    `json:"old_hash,omitempty"`
	NewHash    string    `json:"new_hash,omitempty"`
	ChangeType string    `json:"change_type"` // "added", "modified", "deleted"
	OldModTime time.Time `json:"old_mod_time,omitempty"`
	NewModTime time.Time `json:"new_mod_time,omitempty"`
	TokensDiff int       `json:"tokens_diff"` // Token difference
}

// RefreshResult contains the result of an incremental refresh.
type RefreshResult struct {
	Changes          []*FileChange `json:"changes"`
	FilesAdded       int           `json:"files_added"`
	FilesModified    int           `json:"files_modified"`
	FilesDeleted     int           `json:"files_deleted"`
	FilesUnchanged   int           `json:"files_unchanged"`
	TotalTokenDiff   int           `json:"total_token_diff"`
	RefreshTime      time.Duration `json:"refresh_time"`
	FullRefresh      bool          `json:"full_refresh"`
	PreviousSnapshot string        `json:"previous_snapshot"`
	NewSnapshot      string        `json:"new_snapshot"`
}

// FileSnapshot represents a snapshot of file state.
type FileSnapshot struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	ModTime  time.Time `json:"mod_time"`
	Size     int64     `json:"size"`
	TokenEst int       `json:"token_est"`
}

// IncrementalRefreshConfig holds configuration for incremental refresh.
type IncrementalRefreshConfig struct {
	HashAlgorithm   string           `json:"hash_algorithm"`   // "sha256", "modtime", "size"
	MaxCacheAge     time.Duration    `json:"max_cache_age"`    // Max age before full refresh
	IncludeHidden   bool             `json:"include_hidden"`   // Include hidden files
	FollowSymlinks  bool             `json:"follow_symlinks"`  // Follow symbolic links
	MaxFileSize     int64            `json:"max_file_size"`    // Max file size in bytes
	ExcludePatterns []string         `json:"exclude_patterns"` // Files to exclude
	TokenEstimator  func(string) int `json:"-"`                // Token estimation function
}

// DefaultIncrementalRefreshConfig returns default configuration.
func DefaultIncrementalRefreshConfig() *IncrementalRefreshConfig {
	return &IncrementalRefreshConfig{
		HashAlgorithm:   "sha256",
		MaxCacheAge:     24 * time.Hour,
		IncludeHidden:   false,
		FollowSymlinks:  false,
		MaxFileSize:     10 * 1024 * 1024, // 10MB
		ExcludePatterns: []string{".git/*", "node_modules/*", "vendor/*", "*.lock"},
		TokenEstimator:  EstimateTokens,
	}
}

// IncrementalRefresher manages incremental context refreshes.
type IncrementalRefresher struct {
	config *IncrementalRefreshConfig
	logger *slog.Logger
	mu     sync.RWMutex

	// File state cache
	snapshots map[string]*FileSnapshot
	lastFull  time.Time
	cacheDir  string
}

// NewIncrementalRefresher creates a new incremental refresher.
func NewIncrementalRefresher(config *IncrementalRefreshConfig, logger *slog.Logger, cacheDir string) *IncrementalRefresher {
	if logger == nil {
		logger = slog.Default()
	}
	if config == nil {
		config = DefaultIncrementalRefreshConfig()
	}

	return &IncrementalRefresher{
		config:    config,
		logger:    logger.With("component", "incremental_refresher"),
		snapshots: make(map[string]*FileSnapshot),
		cacheDir:  cacheDir,
	}
}

// Refresh performs an incremental refresh of the given files.
func (ir *IncrementalRefresher) Refresh(ctx context.Context, files []string) (*RefreshResult, error) {
	start := time.Now()

	ir.mu.Lock()
	defer ir.mu.Unlock()

	result := &RefreshResult{
		Changes:     make([]*FileChange, 0),
		FullRefresh: false,
	}

	// Check if we need a full refresh
	if time.Since(ir.lastFull) > ir.config.MaxCacheAge {
		result.FullRefresh = true
		ir.lastFull = time.Now()
	}

	// Build new snapshot map
	newSnapshots := make(map[string]*FileSnapshot)

	// Process each file
	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		change := ir.processFile(file, newSnapshots)
		if change != nil {
			result.Changes = append(result.Changes, change)
			switch change.ChangeType {
			case "added":
				result.FilesAdded++
			case "modified":
				result.FilesModified++
			case "deleted":
				result.FilesDeleted++
			}
		} else {
			result.FilesUnchanged++
		}
	}

	// Check for deleted files (in old snapshots but not in new)
	for path, oldSnap := range ir.snapshots {
		if _, exists := newSnapshots[path]; !exists {
			result.Changes = append(result.Changes, &FileChange{
				Path:       path,
				OldHash:    oldSnap.Hash,
				ChangeType: "deleted",
				OldModTime: oldSnap.ModTime,
				TokensDiff: -oldSnap.TokenEst,
			})
			result.FilesDeleted++
		}
	}

	// Update snapshots
	ir.snapshots = newSnapshots

	// Calculate token diff
	for _, change := range result.Changes {
		result.TotalTokenDiff += change.TokensDiff
	}

	result.RefreshTime = time.Since(start)

	ir.logger.Debug("Incremental refresh complete",
		"added", result.FilesAdded,
		"modified", result.FilesModified,
		"deleted", result.FilesDeleted,
		"unchanged", result.FilesUnchanged,
		"time", result.RefreshTime,
	)

	return result, nil
}

// processFile processes a single file and returns the change if any.
func (ir *IncrementalRefresher) processFile(file string, newSnapshots map[string]*FileSnapshot) *FileChange {
	// Check if file should be excluded
	if ir.shouldExclude(file) {
		return nil
	}

	// Get file info
	info, err := os.Stat(file)
	if err != nil {
		// File doesn't exist - might have been deleted
		if os.IsNotExist(err) {
			if oldSnap, exists := ir.snapshots[file]; exists {
				return &FileChange{
					Path:       file,
					OldHash:    oldSnap.Hash,
					ChangeType: "deleted",
					OldModTime: oldSnap.ModTime,
					TokensDiff: -oldSnap.TokenEst,
				}
			}
		}
		return nil
	}

	// Check file size
	if info.Size() > ir.config.MaxFileSize {
		ir.logger.Debug("File too large, skipping", "file", file, "size", info.Size())
		return nil
	}

	// Create new snapshot
	snapshot := &FileSnapshot{
		Path:    file,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}

	// Calculate hash
	hash, err := ir.calculateHash(file)
	if err != nil {
		ir.logger.Debug("Failed to calculate hash", "file", file, "error", err)
		return nil
	}
	snapshot.Hash = hash

	// Estimate tokens
	content, err := os.ReadFile(file)
	if err == nil && ir.config.TokenEstimator != nil {
		snapshot.TokenEst = ir.config.TokenEstimator(string(content))
	} else {
		snapshot.TokenEst = int(info.Size() / 4) // Fallback estimation
	}

	newSnapshots[file] = snapshot

	// Check if file is new or modified
	if oldSnap, exists := ir.snapshots[file]; exists {
		if oldSnap.Hash != snapshot.Hash {
			return &FileChange{
				Path:       file,
				OldHash:    oldSnap.Hash,
				NewHash:    snapshot.Hash,
				ChangeType: "modified",
				OldModTime: oldSnap.ModTime,
				NewModTime: snapshot.ModTime,
				TokensDiff: snapshot.TokenEst - oldSnap.TokenEst,
			}
		}
		return nil // Unchanged
	}

	// New file
	return &FileChange{
		Path:       file,
		NewHash:    snapshot.Hash,
		ChangeType: "added",
		NewModTime: snapshot.ModTime,
		TokensDiff: snapshot.TokenEst,
	}
}

// calculateHash calculates the hash of a file.
func (ir *IncrementalRefresher) calculateHash(file string) (string, error) {
	switch ir.config.HashAlgorithm {
	case "modtime":
		info, err := os.Stat(file)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", info.ModTime().UnixNano()), nil

	case "size":
		info, err := os.Stat(file)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", info.Size()), nil

	default: // "sha256"
		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(content)
		return hex.EncodeToString(hash[:]), nil
	}
}

// shouldExclude checks if a file should be excluded.
func (ir *IncrementalRefresher) shouldExclude(file string) bool {
	for _, pattern := range ir.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, file)
		if err == nil && matched {
			return true
		}
		// Also check if the pattern matches any parent directory
		if strings.Contains(pattern, "/*") {
			dir := strings.TrimSuffix(pattern, "/*")
			if strings.Contains(file, dir+"/") {
				return true
			}
		}
	}

	// Check for hidden files
	if !ir.config.IncludeHidden {
		base := filepath.Base(file)
		if strings.HasPrefix(base, ".") && base != "." {
			return true
		}
	}

	return false
}

// ForceFullRefresh forces a full refresh on the next Refresh call.
func (ir *IncrementalRefresher) ForceFullRefresh() {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.lastFull = time.Time{} // Reset to zero time
}

// GetSnapshot returns the current snapshot for a file.
func (ir *IncrementalRefresher) GetSnapshot(file string) *FileSnapshot {
	ir.mu.RLock()
	defer ir.mu.RUnlock()
	return ir.snapshots[file]
}

// GetAllSnapshots returns all current snapshots.
func (ir *IncrementalRefresher) GetAllSnapshots() map[string]*FileSnapshot {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	result := make(map[string]*FileSnapshot, len(ir.snapshots))
	for k, v := range ir.snapshots {
		result[k] = v
	}
	return result
}

// ClearSnapshots clears all cached snapshots.
func (ir *IncrementalRefresher) ClearSnapshots() {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.snapshots = make(map[string]*FileSnapshot)
}

// GetChangedFiles returns a list of files that would change.
func (ir *IncrementalRefresher) GetChangedFiles(files []string) ([]string, error) {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	changed := make([]string, 0)

	for _, file := range files {
		if ir.shouldExclude(file) {
			continue
		}

		// Check if file exists in snapshots
		oldSnap, exists := ir.snapshots[file]
		if !exists {
			changed = append(changed, file)
			continue
		}

		// Check mod time first (faster)
		info, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				changed = append(changed, file)
			}
			continue
		}

		if info.ModTime().After(oldSnap.ModTime) {
			changed = append(changed, file)
			continue
		}

	}

	return changed, nil
}

// UpdateConfig updates the refresher configuration.
func (ir *IncrementalRefresher) UpdateConfig(config *IncrementalRefreshConfig) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.config = config
}

// GetConfig returns the current configuration.
func (ir *IncrementalRefresher) GetConfig() *IncrementalRefreshConfig {
	ir.mu.RLock()
	defer ir.mu.RUnlock()
	return ir.config
}

// ToMarkdown generates a markdown representation of the refresh result.
func (r *RefreshResult) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Incremental Refresh Result\n\n")
	sb.WriteString(fmt.Sprintf("**Full refresh:** %v | **Time:** %v\n\n", r.FullRefresh, r.RefreshTime))
	sb.WriteString(fmt.Sprintf("- **Added:** %d\n", r.FilesAdded))
	sb.WriteString(fmt.Sprintf("- **Modified:** %d\n", r.FilesModified))
	sb.WriteString(fmt.Sprintf("- **Deleted:** %d\n", r.FilesDeleted))
	sb.WriteString(fmt.Sprintf("- **Unchanged:** %d\n", r.FilesUnchanged))
	sb.WriteString(fmt.Sprintf("- **Token diff:** %+d\n\n", r.TotalTokenDiff))

	if len(r.Changes) > 0 {
		sb.WriteString("## Changes\n\n")
		sb.WriteString("| File | Type | Token Diff |\n")
		sb.WriteString("|------|------|------------|\n")

		// Sort changes by type then path
		sort.Slice(r.Changes, func(i, j int) bool {
			if r.Changes[i].ChangeType != r.Changes[j].ChangeType {
				return r.Changes[i].ChangeType < r.Changes[j].ChangeType
			}
			return r.Changes[i].Path < r.Changes[j].Path
		})

		for _, change := range r.Changes {
			sb.WriteString(fmt.Sprintf("| %s | %s | %+d |\n",
				change.Path, change.ChangeType, change.TokensDiff))
		}
	}

	return sb.String()
}
