// Package autonomous - Task 9: Rollback Stack for safe autonomous experimentation
package autonomous

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RollbackType represents the type of rollback operation.
type RollbackType string

const (
	RollbackTypeFileEdit   RollbackType = "file_edit"
	RollbackTypeFileCreate RollbackType = "file_create"
	RollbackTypeFileDelete RollbackType = "file_delete"
	RollbackTypeGitCommit  RollbackType = "git_commit"
	RollbackTypeGitBranch  RollbackType = "git_branch"
)

// RollbackEntry represents a single rollback operation.
type RollbackEntry struct {
	ID         string         `json:"id"`
	Type       RollbackType   `json:"type"`
	Timestamp  int64          `json:"timestamp"`
	Path       string         `json:"path,omitempty"`
	Original   string         `json:"original,omitempty"`    // Original content for edits
	Checksum   string         `json:"checksum,omitempty"`    // Content checksum
	CommitHash string         `json:"commit_hash,omitempty"` // For git rollbacks
	BranchName string         `json:"branch_name,omitempty"`
	Applied    bool           `json:"applied"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// RollbackStack manages a stack of rollback operations.
type RollbackStack struct {
	mu         sync.RWMutex
	entries    []*RollbackEntry
	maxEntries int
	baseDir    string
	backupDir  string
	counter    int64
}

// RollbackConfig holds configuration for the rollback stack.
type RollbackConfig struct {
	MaxEntries int    // Maximum entries in stack (default: 50)
	BaseDir    string // Base directory for backups
}

// DefaultRollbackConfig returns sensible defaults.
func DefaultRollbackConfig() RollbackConfig {
	return RollbackConfig{
		MaxEntries: 50,
	}
}

// NewRollbackStack creates a new rollback stack.
func NewRollbackStack(config RollbackConfig) *RollbackStack {
	if config.MaxEntries <= 0 {
		config.MaxEntries = 50
	}

	rs := &RollbackStack{
		entries:    make([]*RollbackEntry, 0),
		maxEntries: config.MaxEntries,
		baseDir:    config.BaseDir,
	}

	if config.BaseDir != "" {
		rs.backupDir = filepath.Join(config.BaseDir, ".rollback_backups")
		os.MkdirAll(rs.backupDir, 0755)
	}

	return rs
}

// PushFileEdit records a file edit for potential rollback.
func (rs *RollbackStack) PushFileEdit(path string, originalContent string) *RollbackEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.counter++
	entry := RollbackEntry{
		ID:        fmt.Sprintf("rb_%d", rs.counter),
		Type:      RollbackTypeFileEdit,
		Timestamp: time.Now().Unix(),
		Path:      path,
		Original:  originalContent,
		Applied:   false,
		Metadata:  make(map[string]any),
	}

	// Save backup to disk if backup dir exists
	if rs.backupDir != "" {
		backupPath := filepath.Join(rs.backupDir, entry.ID+filepath.Ext(path))
		if err := os.WriteFile(backupPath, []byte(originalContent), 0644); err != nil {
			entry.Metadata["backup_error"] = err.Error()
		} else {
			entry.Metadata["backup_path"] = backupPath
		}
	}

	rs.pushEntry(&entry)
	return &entry
}

// PushFileCreate records a file creation for potential rollback.
func (rs *RollbackStack) PushFileCreate(path string, content string) *RollbackEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.counter++
	entry := &RollbackEntry{
		ID:        fmt.Sprintf("rb_%d", rs.counter),
		Type:      RollbackTypeFileCreate,
		Timestamp: time.Now().Unix(),
		Path:      path,
		Original:  content,
		Applied:   false,
		Metadata:  make(map[string]any),
	}

	// Save creation content for potential undo
	if rs.backupDir != "" {
		backupPath := filepath.Join(rs.backupDir, entry.ID+filepath.Ext(path))
		if err := os.WriteFile(backupPath, []byte(content), 0644); err != nil {
			entry.Metadata["backup_error"] = err.Error()
		} else {
			entry.Metadata["backup_path"] = backupPath
		}
	}

	rs.pushEntry(entry)
	return entry
}

// PushFileDelete records a file deletion for potential rollback.
func (rs *RollbackStack) PushFileDelete(path string, content string) *RollbackEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.counter++
	entry := &RollbackEntry{
		ID:        fmt.Sprintf("rb_%d", rs.counter),
		Type:      RollbackTypeFileDelete,
		Timestamp: time.Now().Unix(),
		Path:      path,
		Original:  content,
		Applied:   false,
		Metadata:  make(map[string]any),
	}

	// Save backup
	if rs.backupDir != "" {
		backupPath := filepath.Join(rs.backupDir, entry.ID+filepath.Ext(path))
		if err := os.WriteFile(backupPath, []byte(content), 0644); err != nil {
			entry.Metadata["backup_error"] = err.Error()
		} else {
			entry.Metadata["backup_path"] = backupPath
		}
	}

	rs.pushEntry(entry)
	return entry
}

// PushGitCommit records a git commit for potential rollback.
func (rs *RollbackStack) PushGitCommit(commitHash string) *RollbackEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.counter++
	entry := &RollbackEntry{
		ID:         fmt.Sprintf("rb_%d", rs.counter),
		Type:       RollbackTypeGitCommit,
		Timestamp:  time.Now().Unix(),
		CommitHash: commitHash,
		Applied:    false,
		Metadata:   make(map[string]any),
	}

	rs.pushEntry(entry)
	return entry
}

// pushEntry adds an entry to the stack.
func (rs *RollbackStack) pushEntry(entry *RollbackEntry) {
	rs.entries = append(rs.entries, entry)

	// Trim old entries if needed
	if len(rs.entries) > rs.maxEntries {
		rs.entries = rs.entries[len(rs.entries)-rs.maxEntries:]
	}
}

// Pop removes and returns the most recent entry.
func (rs *RollbackStack) Pop() *RollbackEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if len(rs.entries) == 0 {
		return nil
	}

	entry := rs.entries[len(rs.entries)-1]
	rs.entries = rs.entries[:len(rs.entries)-1]
	return entry
}

// Peek returns the most recent entry without removing it.
func (rs *RollbackStack) Peek() *RollbackEntry {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if len(rs.entries) == 0 {
		return nil
	}

	return rs.entries[len(rs.entries)-1]
}

// Rollback performs the most recent rollback operation.
func (rs *RollbackStack) Rollback() error {
	entry := rs.Pop()
	if entry == nil {
		return fmt.Errorf("no entries to rollback")
	}

	return rs.executeRollback(entry)
}

// RollbackTo rolls back to a specific entry (inclusive).
func (rs *RollbackStack) RollbackTo(entryID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Find the entry
	var targetIndex int = -1
	for i, entry := range rs.entries {
		if entry.ID == entryID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return fmt.Errorf("entry %s not found", entryID)
	}

	// Rollback from most recent to target
	for i := len(rs.entries) - 1; i >= targetIndex; i-- {
		if err := rs.executeRollback(rs.entries[i]); err != nil {
			return fmt.Errorf("rollback failed at entry %s: %w", rs.entries[i].ID, err)
		}
	}

	// Remove rolled back entries
	rs.entries = rs.entries[:targetIndex]

	return nil
}

// executeRollback performs the actual rollback operation.
func (rs *RollbackStack) executeRollback(entry *RollbackEntry) error {
	entry.Applied = true

	switch entry.Type {
	case RollbackTypeFileEdit:
		return rs.rollbackFileEdit(entry)
	case RollbackTypeFileCreate:
		return rs.rollbackFileCreate(entry)
	case RollbackTypeFileDelete:
		return rs.rollbackFileDelete(entry)
	case RollbackTypeGitCommit:
		return rs.rollbackGitCommit(entry)
	default:
		return fmt.Errorf("unknown rollback type: %s", entry.Type)
	}
}

func (rs *RollbackStack) rollbackFileEdit(entry *RollbackEntry) error {
	if entry.Path == "" {
		return fmt.Errorf("no path for file edit rollback")
	}

	// Try backup file first
	if backupPath, ok := entry.Metadata["backup_path"].(string); ok {
		data, err := os.ReadFile(backupPath)
		if err == nil {
			return os.WriteFile(entry.Path, data, 0644)
		}
	}

	// Fall back to stored content
	if entry.Original != "" {
		return os.WriteFile(entry.Path, []byte(entry.Original), 0644)
	}

	return fmt.Errorf("no backup content available")
}

func (rs *RollbackStack) rollbackFileCreate(entry *RollbackEntry) error {
	if entry.Path == "" {
		return fmt.Errorf("no path for file create rollback")
	}

	// Try backup file content first (to restore the file)
	if backupPath, ok := entry.Metadata["backup_path"].(string); ok {
		data, err := os.ReadFile(backupPath)
		if err == nil {
			return os.WriteFile(entry.Path, data, 0644)
		}
	}

	// Fall back to stored content, then try delete
	if entry.Original != "" {
		return os.WriteFile(entry.Path, []byte(entry.Original), 0644)
	}

	// If file exists and we just need to delete it
	if _, err := os.Stat(entry.Path); err == nil {
		return os.Remove(entry.Path)
	}

	return fmt.Errorf("no backup content available for created file")
}

func (rs *RollbackStack) rollbackFileDelete(entry *RollbackEntry) error {
	if entry.Path == "" {
		return fmt.Errorf("no path for file delete rollback")
	}

	// Try backup file first
	if backupPath, ok := entry.Metadata["backup_path"].(string); ok {
		data, err := os.ReadFile(backupPath)
		if err == nil {
			return os.WriteFile(entry.Path, data, 0644)
		}
	}

	// Fall back to stored content
	if entry.Original != "" {
		return os.WriteFile(entry.Path, []byte(entry.Original), 0644)
	}

	return fmt.Errorf("no content to restore")
}

func (rs *RollbackStack) rollbackGitCommit(entry *RollbackEntry) error {
	// This would integrate with git commands
	// For now, we just mark it as needing manual intervention
	entry.Metadata["manual_rollback"] = true
	entry.Metadata["instruction"] = fmt.Sprintf("git reset --hard %s^", entry.CommitHash)
	return nil
}

// RollbackAll rolls back all entries.
func (rs *RollbackStack) RollbackAll() error {
	for len(rs.entries) > 0 {
		if err := rs.Rollback(); err != nil {
			return err
		}
	}
	return nil
}

// GetEntries returns all entries.
func (rs *RollbackStack) GetEntries() []*RollbackEntry {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	result := make([]*RollbackEntry, len(rs.entries))
	for i, entry := range rs.entries {
		copy := *entry
		result[i] = &copy
	}
	return result
}

// GetEntry returns a specific entry by ID.
func (rs *RollbackStack) GetEntry(id string) *RollbackEntry {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for _, entry := range rs.entries {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

// Size returns the number of entries in the stack.
func (rs *RollbackStack) Size() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.entries)
}

// Clear removes all entries without performing rollbacks.
func (rs *RollbackStack) Clear() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.entries = make([]*RollbackEntry, 0)
}

// CreateSnapshot creates a named snapshot of the current stack.
func (rs *RollbackStack) CreateSnapshot(name string) (*RollbackSnapshot, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	snapshot := &RollbackSnapshot{
		Name:      name,
		Timestamp: time.Now().Unix(),
		Entries:   make([]RollbackEntry, len(rs.entries)),
	}
	for i, entry := range rs.entries {
		snapshot.Entries[i] = *entry
	}

	return snapshot, nil
}

// RollbackSnapshot represents a saved state of the rollback stack.
type RollbackSnapshot struct {
	Name      string          `json:"name"`
	Timestamp int64           `json:"timestamp"`
	Entries   []RollbackEntry `json:"entries"`
}

// CanRollback checks if a rollback is possible.
func (rs *RollbackStack) CanRollback() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.entries) > 0
}

// GetLastOfType returns the most recent entry of a specific type.
func (rs *RollbackStack) GetLastOfType(rollbackType RollbackType) *RollbackEntry {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for i := len(rs.entries) - 1; i >= 0; i-- {
		if rs.entries[i].Type == rollbackType {
			return rs.entries[i]
		}
	}
	return nil
}
