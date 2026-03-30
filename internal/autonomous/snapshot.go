// Package autonomous - Task 30: Snapshot capability before destructive changes
package autonomous

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SnapshotStatus represents the status of a snapshot.
type SnapshotStatus string

const (
	// SnapshotStatusCreating - Snapshot is being created
	SnapshotStatusCreating SnapshotStatus = "creating"

	// SnapshotStatusComplete - Snapshot is complete
	SnapshotStatusComplete SnapshotStatus = "complete"

	// SnapshotStatusRestoring - Snapshot is being restored
	SnapshotStatusRestoring SnapshotStatus = "restoring"

	// SnapshotStatusRestored - Snapshot has been restored
	SnapshotStatusRestored SnapshotStatus = "restored"

	// SnapshotStatusFailed - Snapshot operation failed
	SnapshotStatusFailed SnapshotStatus = "failed"

	// SnapshotStatusExpired - Snapshot has expired
	SnapshotStatusExpired SnapshotStatus = "expired"
)

// SnapshotType represents the type of snapshot.
type SnapshotType string

const (
	// SnapshotTypeFile - Single file snapshot
	SnapshotTypeFile SnapshotType = "file"

	// SnapshotTypeDirectory - Directory snapshot
	SnapshotTypeDirectory SnapshotType = "directory"

	// SnapshotTypeProject - Full project snapshot
	SnapshotTypeProject SnapshotType = "project"

	// SnapshotTypeSelective - Selective files/directories
	SnapshotTypeSelective SnapshotType = "selective"
)

// SnapshotMetadata contains metadata about a snapshot.
type SnapshotMetadata struct {
	// ID is the unique snapshot identifier
	ID string `json:"id"`

	// Name is a human-readable name
	Name string `json:"name,omitempty"`

	// Type is the snapshot type
	Type SnapshotType `json:"type"`

	// Status is the current status
	Status SnapshotStatus `json:"status"`

	// CreatedAt is when the snapshot was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the snapshot expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Size is the total size in bytes
	Size int64 `json:"size"`

	// FileCount is the number of files included
	FileCount int `json:"file_count"`

	// Files is the list of files in the snapshot
	Files []SnapshotFile `json:"files,omitempty"`

	// BasePath is the original base path
	BasePath string `json:"base_path"`

	// StoragePath is where the snapshot is stored
	StoragePath string `json:"storage_path"`

	// Tags are user-defined tags
	Tags []string `json:"tags,omitempty"`

	// Reason is why the snapshot was created
	Reason string `json:"reason,omitempty"`

	// ParentSnapshot is the parent snapshot ID (for incremental)
	ParentSnapshot string `json:"parent_snapshot,omitempty"`

	// Checksum for snapshot integrity
	Checksum string `json:"checksum"`

	// RestoredAt is when it was restored
	RestoredAt *time.Time `json:"restored_at,omitempty"`

	// RestoreCount is how many times it was restored
	RestoreCount int `json:"restore_count"`
}

// SnapshotFile represents a file in a snapshot.
type SnapshotFile struct {
	// OriginalPath is the original file path
	OriginalPath string `json:"original_path"`

	// SnapshotPath is the path in the snapshot
	SnapshotPath string `json:"snapshot_path"`

	// Size is the file size
	Size int64 `json:"size"`

	// Mode is the file permissions
	Mode uint32 `json:"mode"`

	// ModTime is the modification time
	ModTime time.Time `json:"mod_time"`

	// IsDir indicates if it's a directory
	IsDir bool `json:"is_dir"`

	// Checksum is the file checksum
	Checksum string `json:"checksum"`
}

// SnapshotConfig configures snapshot behavior.
type SnapshotConfig struct {
	// Enabled turns snapshot functionality on/off
	Enabled bool `json:"enabled"`

	// StoragePath is where snapshots are stored
	StoragePath string `json:"storage_path"`

	// MaxSnapshots is the maximum snapshots to keep
	MaxSnapshots int `json:"max_snapshots"`

	// DefaultTTL is the default time-to-live
	DefaultTTL time.Duration `json:"default_ttl"`

	// AutoSnapshot enables automatic snapshots before destructive ops
	AutoSnapshot bool `json:"auto_snapshot"`

	// DestructivePatterns are patterns that trigger auto-snapshot
	DestructivePatterns []string `json:"destructive_patterns,omitempty"`

	// ExcludePatterns are patterns to exclude from snapshots
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// IncludeHidden includes hidden files
	IncludeHidden bool `json:"include_hidden"`

	// Compress enables compression
	Compress bool `json:"compress"`
}

// DefaultSnapshotConfig returns the default snapshot configuration.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		Enabled:         true,
		StoragePath:     ".iterate/snapshots",
		MaxSnapshots:    50,
		DefaultTTL:      24 * time.Hour,
		AutoSnapshot:    true,
		IncludeHidden:   false,
		Compress:        false,
		ExcludePatterns: []string{".git", "node_modules", "vendor", "*.log", "*.tmp"},
	}
}

// SnapshotStats tracks snapshot statistics.
type SnapshotStats struct {
	TotalSnapshots   int            `json:"total_snapshots"`
	TotalSize        int64          `json:"total_size"`
	TotalFiles       int            `json:"total_files"`
	ActiveSnapshots  int            `json:"active_snapshots"`
	ExpiredSnapshots int            `json:"expired_snapshots"`
	RestoredCount    int            `json:"restored_count"`
	FailedCount      int            `json:"failed_count"`
	ByType           map[string]int `json:"by_type"`
	OldestSnapshot   *time.Time     `json:"oldest_snapshot,omitempty"`
	NewestSnapshot   *time.Time     `json:"newest_snapshot,omitempty"`
}

// SnapshotManager manages file/directory snapshots.
type SnapshotManager struct {
	mu sync.RWMutex

	// config is the snapshot configuration
	config SnapshotConfig

	// snapshots stores snapshot metadata
	snapshots map[string]*SnapshotMetadata

	// ordered stores snapshot IDs in creation order
	ordered []string

	// stats tracks statistics
	stats SnapshotStats

	// auditLogger is the optional audit logger
	auditLogger *AuditLogger

	// timeNow is a function to get current time (for testing)
	timeNow func() time.Time
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(config SnapshotConfig) *SnapshotManager {
	sm := &SnapshotManager{
		config:    config,
		snapshots: make(map[string]*SnapshotMetadata),
		ordered:   make([]string, 0),
		stats: SnapshotStats{
			ByType: make(map[string]int),
		},
		timeNow: time.Now,
	}

	// Create storage directory
	if config.Enabled && config.StoragePath != "" {
		os.MkdirAll(config.StoragePath, 0755)
	}

	return sm
}

// SetAuditLogger sets the audit logger.
func (sm *SnapshotManager) SetAuditLogger(logger *AuditLogger) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.auditLogger = logger
}

// CreateSnapshot creates a new snapshot.
func (sm *SnapshotManager) CreateSnapshot(name string, snapshotType SnapshotType, paths []string, opts ...SnapshotOption) (*SnapshotMetadata, error) {
	if !sm.config.Enabled {
		return nil, fmt.Errorf("snapshots are disabled")
	}

	now := sm.timeNow()
	metadata := &SnapshotMetadata{
		ID:        generateSnapshotID(),
		Name:      name,
		Type:      snapshotType,
		Status:    SnapshotStatusCreating,
		CreatedAt: now,
		Files:     make([]SnapshotFile, 0),
	}

	// Apply options
	for _, opt := range opts {
		opt(metadata)
	}

	// Set expiry if TTL configured
	if metadata.ExpiresAt == nil && sm.config.DefaultTTL > 0 {
		expires := now.Add(sm.config.DefaultTTL)
		metadata.ExpiresAt = &expires
	}

	// Determine base path
	if len(paths) > 0 {
		metadata.BasePath = filepath.Dir(paths[0])
		if snapshotType == SnapshotTypeFile && len(paths) == 1 {
			metadata.BasePath = filepath.Dir(paths[0])
		}
	}

	// Create snapshot storage directory
	snapshotDir := filepath.Join(sm.config.StoragePath, metadata.ID)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	metadata.StoragePath = snapshotDir

	// Copy files to snapshot
	for _, path := range paths {
		if err := sm.addToSnapshot(metadata, path); err != nil {
			// Cleanup on error
			os.RemoveAll(snapshotDir)
			metadata.Status = SnapshotStatusFailed
			return nil, fmt.Errorf("failed to add %s to snapshot: %w", path, err)
		}
	}

	// Calculate total checksum
	metadata.Checksum = sm.calculateSnapshotChecksum(metadata)
	metadata.Status = SnapshotStatusComplete

	// Store metadata
	sm.mu.Lock()
	sm.snapshots[metadata.ID] = metadata
	sm.ordered = append(sm.ordered, metadata.ID)
	sm.updateStats(metadata, true)
	sm.enforceMaxSnapshots()
	sm.mu.Unlock()

	// Log to audit
	if sm.auditLogger != nil {
		sm.auditLogger.LogSnapshot("create", metadata.ID, nil)
	}

	return metadata, nil
}

// addToSnapshot adds a file or directory to the snapshot.
func (sm *SnapshotManager) addToSnapshot(metadata *SnapshotMetadata, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Check exclusion patterns
	if sm.shouldExclude(path) {
		return nil
	}

	if info.IsDir() {
		return sm.addDirectoryToSnapshot(metadata, path)
	}

	return sm.addFileToSnapshot(metadata, path, info)
}

// addFileToSnapshot adds a single file to the snapshot.
func (sm *SnapshotManager) addFileToSnapshot(metadata *SnapshotMetadata, path string, info os.FileInfo) error {
	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create destination path
	relPath, err := filepath.Rel(metadata.BasePath, path)
	if err != nil {
		relPath = filepath.Base(path)
	}

	destPath := filepath.Join(metadata.StoragePath, "files", relPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Copy file
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Calculate checksum while copying
	hash := sha256.New()
	multiWriter := io.MultiWriter(dst, hash)

	if _, err := io.Copy(multiWriter, src); err != nil {
		return err
	}

	// Preserve permissions
	if err := os.Chmod(destPath, info.Mode()); err != nil {
		return err
	}

	// Record file info
	snapshotFile := SnapshotFile{
		OriginalPath: path,
		SnapshotPath: destPath,
		Size:         info.Size(),
		Mode:         uint32(info.Mode()),
		ModTime:      info.ModTime(),
		IsDir:        false,
		Checksum:     fmt.Sprintf("%x", hash.Sum(nil)),
	}

	sm.mu.Lock()
	metadata.Files = append(metadata.Files, snapshotFile)
	metadata.Size += info.Size()
	metadata.FileCount++
	sm.mu.Unlock()

	return nil
}

// addDirectoryToSnapshot adds a directory to the snapshot.
func (sm *SnapshotManager) addDirectoryToSnapshot(metadata *SnapshotMetadata, dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded paths
		if sm.shouldExclude(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files if configured
		if !sm.config.IncludeHidden && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			// Record directory but don't copy
			relPath, _ := filepath.Rel(metadata.BasePath, path)
			snapshotFile := SnapshotFile{
				OriginalPath: path,
				SnapshotPath: filepath.Join(metadata.StoragePath, "files", relPath),
				Mode:         uint32(info.Mode()),
				ModTime:      info.ModTime(),
				IsDir:        true,
			}
			sm.mu.Lock()
			metadata.Files = append(metadata.Files, snapshotFile)
			sm.mu.Unlock()
			return nil
		}

		return sm.addFileToSnapshot(metadata, path, info)
	})
}

// shouldExclude checks if a path should be excluded from snapshot.
func (sm *SnapshotManager) shouldExclude(path string) bool {
	base := filepath.Base(path)

	for _, pattern := range sm.config.ExcludePatterns {
		// Simple pattern matching
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if strings.Contains(path, "/"+pattern+"/") {
			return true
		}
	}

	return false
}

// RestoreSnapshot restores a snapshot.
func (sm *SnapshotManager) RestoreSnapshot(snapshotID string, opts ...RestoreOption) error {
	sm.mu.RLock()
	metadata, exists := sm.snapshots[snapshotID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	if metadata.Status != SnapshotStatusComplete && metadata.Status != SnapshotStatusRestored {
		return fmt.Errorf("snapshot %s is not in a restorable state: %s", snapshotID, metadata.Status)
	}

	// Apply restore options
	restoreConfig := RestoreConfig{
		Overwrite:   true,
		CreateDirs:  true,
		VerifyAfter: true,
	}
	for _, opt := range opts {
		opt(&restoreConfig)
	}

	// Update status
	sm.mu.Lock()
	metadata.Status = SnapshotStatusRestoring
	sm.mu.Unlock()

	// Restore files
	for _, file := range metadata.Files {
		if file.IsDir {
			// Create directory
			if restoreConfig.CreateDirs {
				if err := os.MkdirAll(file.OriginalPath, os.FileMode(file.Mode)); err != nil {
					sm.markFailed(snapshotID)
					return fmt.Errorf("failed to create directory %s: %w", file.OriginalPath, err)
				}
			}
			continue
		}

		// Check if destination exists
		if _, err := os.Stat(file.OriginalPath); err == nil {
			if !restoreConfig.Overwrite {
				continue
			}
		}

		// Ensure parent directory exists
		if restoreConfig.CreateDirs {
			os.MkdirAll(filepath.Dir(file.OriginalPath), 0755)
		}

		// Copy file from snapshot
		if err := sm.restoreFile(file); err != nil {
			sm.markFailed(snapshotID)
			return fmt.Errorf("failed to restore %s: %w", file.OriginalPath, err)
		}
	}

	// Update metadata
	now := sm.timeNow()
	sm.mu.Lock()
	metadata.Status = SnapshotStatusRestored
	metadata.RestoredAt = &now
	metadata.RestoreCount++
	sm.stats.RestoredCount++
	sm.mu.Unlock()

	// Verify restore if requested
	if restoreConfig.VerifyAfter {
		verifyResult, verifyErr := sm.VerifyRestore(snapshotID)
		if verifyErr != nil {
			return fmt.Errorf("restore completed but verification failed: %w", verifyErr)
		}
		if !verifyResult.Success {
			return fmt.Errorf("restore verification failed: %d files missing, %d checksum mismatches",
				len(verifyResult.MissingFiles), len(verifyResult.ChecksumMismatches))
		}
	}

	// Log to audit
	if sm.auditLogger != nil {
		sm.auditLogger.LogSnapshot("restore", snapshotID, nil)
	}

	return nil
}

// restoreFile restores a single file from snapshot.
func (sm *SnapshotManager) restoreFile(file SnapshotFile) error {
	src, err := os.Open(file.SnapshotPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(file.OriginalPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Restore permissions
	return os.Chmod(file.OriginalPath, os.FileMode(file.Mode))
}

// DeleteSnapshot deletes a snapshot.
func (sm *SnapshotManager) DeleteSnapshot(snapshotID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	metadata, exists := sm.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Remove snapshot directory
	if metadata.StoragePath != "" {
		if err := os.RemoveAll(metadata.StoragePath); err != nil {
			return fmt.Errorf("failed to remove snapshot directory: %w", err)
		}
	}

	// Update stats
	sm.stats.TotalSize -= metadata.Size
	sm.stats.TotalFiles -= metadata.FileCount
	sm.stats.TotalSnapshots--
	sm.stats.ByType[string(metadata.Type)]--

	// Remove from maps
	delete(sm.snapshots, snapshotID)
	for i, id := range sm.ordered {
		if id == snapshotID {
			sm.ordered = append(sm.ordered[:i], sm.ordered[i+1:]...)
			break
		}
	}

	// Log to audit
	if sm.auditLogger != nil {
		sm.auditLogger.LogSnapshot("delete", snapshotID, nil)
	}

	return nil
}

// GetSnapshot retrieves snapshot metadata.
func (sm *SnapshotManager) GetSnapshot(snapshotID string) (*SnapshotMetadata, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	metadata, exists := sm.snapshots[snapshotID]
	return metadata, exists
}

// ListSnapshots lists all snapshots.
func (sm *SnapshotManager) ListSnapshots() []*SnapshotMetadata {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SnapshotMetadata, 0, len(sm.ordered))
	for _, id := range sm.ordered {
		if metadata, exists := sm.snapshots[id]; exists {
			result = append(result, metadata)
		}
	}

	return result
}

// GetSnapshotsByType lists snapshots of a specific type.
func (sm *SnapshotManager) GetSnapshotsByType(snapshotType SnapshotType) []*SnapshotMetadata {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SnapshotMetadata, 0)
	for _, id := range sm.ordered {
		if metadata, exists := sm.snapshots[id]; exists && metadata.Type == snapshotType {
			result = append(result, metadata)
		}
	}

	return result
}

// GetStats returns snapshot statistics.
func (sm *SnapshotManager) GetStats() SnapshotStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.stats
}

// ExpireSnapshots removes expired snapshots.
func (sm *SnapshotManager) ExpireSnapshots() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := sm.timeNow()
	expired := 0

	for _, id := range sm.ordered {
		metadata := sm.snapshots[id]
		if metadata != nil && metadata.ExpiresAt != nil && now.After(*metadata.ExpiresAt) {
			metadata.Status = SnapshotStatusExpired
			os.RemoveAll(metadata.StoragePath)
			delete(sm.snapshots, id)
			sm.stats.ExpiredSnapshots++
			expired++
		}
	}

	// Rebuild ordered list
	if expired > 0 {
		newOrdered := make([]string, 0)
		for _, id := range sm.ordered {
			if _, exists := sm.snapshots[id]; exists {
				newOrdered = append(newOrdered, id)
			}
		}
		sm.ordered = newOrdered
	}

	return expired
}

// AutoSnapshot creates a snapshot if the operation is destructive.
func (sm *SnapshotManager) AutoSnapshot(operation, path string, paths []string) (*SnapshotMetadata, error) {
	if !sm.config.AutoSnapshot {
		return nil, nil
	}

	// Check if operation is destructive
	isDestructive := sm.isDestructiveOperation(operation, path)
	if !isDestructive {
		return nil, nil
	}

	// Determine snapshot type
	snapshotType := SnapshotTypeFile
	if len(paths) > 1 {
		snapshotType = SnapshotTypeSelective
	} else if len(paths) == 1 {
		if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
			snapshotType = SnapshotTypeDirectory
		}
	}

	name := fmt.Sprintf("auto-%s-%s", operation, filepath.Base(path))

	return sm.CreateSnapshot(name, snapshotType, paths, WithReason("auto-snapshot before "+operation))
}

// isDestructiveOperation checks if an operation is destructive.
func (sm *SnapshotManager) isDestructiveOperation(operation, path string) bool {
	destructiveOps := []string{"delete", "remove", "overwrite", "truncate", "rm", "unlink"}

	for _, op := range destructiveOps {
		if strings.Contains(strings.ToLower(operation), op) {
			return true
		}
	}

	for _, pattern := range sm.config.DestructivePatterns {
		if matched, _ := filepath.Match(pattern, operation); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// Helper methods

func (sm *SnapshotManager) markFailed(snapshotID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if metadata, exists := sm.snapshots[snapshotID]; exists {
		metadata.Status = SnapshotStatusFailed
		sm.stats.FailedCount++
	}
}

func (sm *SnapshotManager) updateStats(metadata *SnapshotMetadata, added bool) {
	if added {
		sm.stats.TotalSnapshots++
		sm.stats.TotalSize += metadata.Size
		sm.stats.TotalFiles += metadata.FileCount
		sm.stats.ActiveSnapshots++
		sm.stats.ByType[string(metadata.Type)]++

		if sm.stats.OldestSnapshot == nil || metadata.CreatedAt.Before(*sm.stats.OldestSnapshot) {
			sm.stats.OldestSnapshot = &metadata.CreatedAt
		}
		if sm.stats.NewestSnapshot == nil || metadata.CreatedAt.After(*sm.stats.NewestSnapshot) {
			sm.stats.NewestSnapshot = &metadata.CreatedAt
		}
	}
}

func (sm *SnapshotManager) enforceMaxSnapshots() {
	for len(sm.snapshots) > sm.config.MaxSnapshots {
		// Remove oldest snapshot
		oldestID := sm.ordered[0]
		if metadata, exists := sm.snapshots[oldestID]; exists {
			os.RemoveAll(metadata.StoragePath)
			sm.stats.TotalSize -= metadata.Size
			sm.stats.TotalFiles -= metadata.FileCount
			sm.stats.TotalSnapshots--
			sm.stats.ByType[string(metadata.Type)]--
			delete(sm.snapshots, oldestID)
		}
		sm.ordered = sm.ordered[1:]
	}
}

func (sm *SnapshotManager) calculateSnapshotChecksum(metadata *SnapshotMetadata) string {
	data := fmt.Sprintf("%s|%s|%d|%d", metadata.ID, metadata.CreatedAt, metadata.Size, metadata.FileCount)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

// Option types

// SnapshotOption is a functional option for creating snapshots.
type SnapshotOption func(*SnapshotMetadata)

// WithExpiry sets the expiry time.
func WithExpiry(expiresAt time.Time) SnapshotOption {
	return func(m *SnapshotMetadata) { m.ExpiresAt = &expiresAt }
}

// WithTags sets the tags.
func WithTags(tags ...string) SnapshotOption {
	return func(m *SnapshotMetadata) { m.Tags = tags }
}

// WithReason sets the reason.
func WithReason(reason string) SnapshotOption {
	return func(m *SnapshotMetadata) { m.Reason = reason }
}

// WithParent sets the parent snapshot.
func WithParent(parentID string) SnapshotOption {
	return func(m *SnapshotMetadata) { m.ParentSnapshot = parentID }
}

// RestoreOption is a functional option for restoring snapshots.
type RestoreOption func(*RestoreConfig)

// RestoreConfig configures restore behavior.
type RestoreConfig struct {
	Overwrite   bool
	CreateDirs  bool
	VerifyAfter bool
	DryRun      bool
}

// VerifyResult contains the result of a restore verification.
type VerifyResult struct {
	// Success indicates if verification passed
	Success bool `json:"success"`

	// VerifiedFiles is the number of files verified
	VerifiedFiles int `json:"verified_files"`

	// FailedFiles is the number of files that failed verification
	FailedFiles int `json:"failed_files"`

	// MissingFiles are files that were not found after restore
	MissingFiles []string `json:"missing_files,omitempty"`

	// ChecksumMismatches are files with checksum mismatches
	ChecksumMismatches []ChecksumMismatch `json:"checksum_mismatches,omitempty"`

	// PermissionErrors are files with permission mismatches
	PermissionErrors []PermissionError `json:"permission_errors,omitempty"`

	// ExtraFiles are files found that weren't in the snapshot
	ExtraFiles []string `json:"extra_files,omitempty"`

	// Duration is how long verification took
	Duration time.Duration `json:"duration"`
}

// ChecksumMismatch represents a checksum verification failure.
type ChecksumMismatch struct {
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash"`
	ActualHash   string `json:"actual_hash"`
}

// PermissionError represents a permission verification failure.
type PermissionError struct {
	Path         string `json:"path"`
	ExpectedMode uint32 `json:"expected_mode"`
	ActualMode   uint32 `json:"actual_mode"`
}

// WithOverwrite sets whether to overwrite existing files.
func WithOverwrite(overwrite bool) RestoreOption {
	return func(c *RestoreConfig) { c.Overwrite = overwrite }
}

// WithDryRun sets dry run mode.
func WithDryRun(dryRun bool) RestoreOption {
	return func(c *RestoreConfig) { c.DryRun = dryRun }
}

// Helper functions

func generateSnapshotID() string {
	return fmt.Sprintf("snap-%d", time.Now().UnixNano())
}

// ExportMetadata exports snapshot metadata as JSON.
func (sm *SnapshotManager) ExportMetadata(snapshotID string) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	metadata, exists := sm.snapshots[snapshotID]
	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	return json.MarshalIndent(metadata, "", "  ")
}

// VerifyRestore verifies that a snapshot restore was successful.
// It checks that all files exist, have correct checksums, and proper permissions.
func (sm *SnapshotManager) VerifyRestore(snapshotID string) (*VerifyResult, error) {
	sm.mu.RLock()
	metadata, exists := sm.snapshots[snapshotID]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", snapshotID)
	}

	start := sm.timeNow()
	result := &VerifyResult{
		Success:            true,
		MissingFiles:       make([]string, 0),
		ChecksumMismatches: make([]ChecksumMismatch, 0),
		PermissionErrors:   make([]PermissionError, 0),
		ExtraFiles:         make([]string, 0),
	}

	// Verify each file in the snapshot
	for _, file := range metadata.Files {
		if file.IsDir {
			// Verify directory exists
			if _, err := os.Stat(file.OriginalPath); os.IsNotExist(err) {
				result.MissingFiles = append(result.MissingFiles, file.OriginalPath)
				result.FailedFiles++
				continue
			}
			result.VerifiedFiles++
			continue
		}

		// Check file exists
		info, err := os.Stat(file.OriginalPath)
		if os.IsNotExist(err) {
			result.MissingFiles = append(result.MissingFiles, file.OriginalPath)
			result.FailedFiles++
			continue
		}
		if err != nil {
			result.FailedFiles++
			continue
		}

		// Verify checksum
		actualHash, err := sm.calculateFileChecksum(file.OriginalPath)
		if err != nil {
			result.FailedFiles++
			continue
		}

		if actualHash != file.Checksum {
			result.ChecksumMismatches = append(result.ChecksumMismatches, ChecksumMismatch{
				Path:         file.OriginalPath,
				ExpectedHash: file.Checksum,
				ActualHash:   actualHash,
			})
			result.FailedFiles++
			continue
		}

		// Verify permissions
		actualMode := uint32(info.Mode().Perm())
		expectedMode := file.Mode & 0777 // Extract permission bits only
		if actualMode != expectedMode {
			result.PermissionErrors = append(result.PermissionErrors, PermissionError{
				Path:         file.OriginalPath,
				ExpectedMode: expectedMode,
				ActualMode:   actualMode,
			})
			// Permission mismatch is a warning, not a failure
		}

		result.VerifiedFiles++
	}

	// Determine overall success
	result.Success = len(result.MissingFiles) == 0 && len(result.ChecksumMismatches) == 0
	result.Duration = sm.timeNow().Sub(start)

	// Log verification result
	if sm.auditLogger != nil {
		var verifyErr error
		if !result.Success {
			verifyErr = fmt.Errorf("verification failed: %d missing, %d checksum mismatches",
				len(result.MissingFiles), len(result.ChecksumMismatches))
		}
		sm.auditLogger.LogSnapshot("verify", snapshotID, verifyErr)
	}

	return result, nil
}

// VerifyAndRepair verifies a restore and attempts to repair any issues.
func (sm *SnapshotManager) VerifyAndRepair(snapshotID string) (*VerifyResult, error) {
	result, err := sm.VerifyRestore(snapshotID)
	if err != nil {
		return nil, err
	}

	if result.Success {
		return result, nil
	}

	// Attempt to repair missing/mismatched files
	sm.mu.RLock()
	metadata, _ := sm.snapshots[snapshotID]
	sm.mu.RUnlock()

	if metadata == nil {
		return result, nil
	}

	// Build map of files to repair
	repairMap := make(map[string]SnapshotFile)
	for _, file := range metadata.Files {
		repairMap[file.OriginalPath] = file
	}

	// Repair checksum mismatches
	for _, mismatch := range result.ChecksumMismatches {
		file, ok := repairMap[mismatch.Path]
		if !ok {
			continue
		}

		if err := sm.restoreFile(file); err == nil {
			// Re-check checksum after repair
			if actualHash, err := sm.calculateFileChecksum(file.OriginalPath); err == nil {
				if actualHash == file.Checksum {
					// Successfully repaired
					result.ChecksumMismatches = sm.removeChecksumMismatch(result.ChecksumMismatches, mismatch.Path)
					result.FailedFiles--
					result.VerifiedFiles++
				}
			}
		}
	}

	// Repair missing files
	for _, missingPath := range result.MissingFiles {
		file, ok := repairMap[missingPath]
		if !ok {
			continue
		}

		// Create parent directory if needed
		os.MkdirAll(filepath.Dir(file.OriginalPath), 0755)

		if err := sm.restoreFile(file); err == nil {
			// Verify the file now exists with correct checksum
			if actualHash, err := sm.calculateFileChecksum(file.OriginalPath); err == nil {
				if actualHash == file.Checksum {
					// Successfully restored missing file
					result.MissingFiles = sm.removeMissingFile(result.MissingFiles, missingPath)
					result.FailedFiles--
					result.VerifiedFiles++
				}
			}
		}
	}

	// Update success status
	result.Success = len(result.MissingFiles) == 0 && len(result.ChecksumMismatches) == 0

	return result, nil
}

// calculateFileChecksum calculates the SHA256 checksum of a file.
func (sm *SnapshotManager) calculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// removeChecksumMismatch removes a checksum mismatch from the list.
func (sm *SnapshotManager) removeChecksumMismatch(mismatches []ChecksumMismatch, path string) []ChecksumMismatch {
	for i, m := range mismatches {
		if m.Path == path {
			return append(mismatches[:i], mismatches[i+1:]...)
		}
	}
	return mismatches
}

// removeMissingFile removes a path from the missing files list.
func (sm *SnapshotManager) removeMissingFile(missing []string, path string) []string {
	for i, p := range missing {
		if p == path {
			return append(missing[:i], missing[i+1:]...)
		}
	}
	return missing
}

// VerifySnapshotIntegrity verifies the integrity of a snapshot's stored data.
func (sm *SnapshotManager) VerifySnapshotIntegrity(snapshotID string) error {
	sm.mu.RLock()
	metadata, exists := sm.snapshots[snapshotID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Verify all snapshot files exist in storage
	for _, file := range metadata.Files {
		if file.IsDir {
			continue
		}

		if _, err := os.Stat(file.SnapshotPath); os.IsNotExist(err) {
			return fmt.Errorf("snapshot file missing: %s", file.SnapshotPath)
		}

		// Verify stored file checksum
		storedHash, err := sm.calculateFileChecksum(file.SnapshotPath)
		if err != nil {
			return fmt.Errorf("failed to verify stored file %s: %w", file.SnapshotPath, err)
		}

		if storedHash != file.Checksum {
			return fmt.Errorf("checksum mismatch for stored file %s", file.SnapshotPath)
		}
	}

	return nil
}
