// Package autonomous - Task 24: File system isolation for sandbox mode
package autonomous

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileSystemIsolation provides controlled file system access through sandbox.
type FileSystemIsolation struct {
	mu sync.RWMutex

	// allowedPaths are directories the sandbox can access
	allowedPaths map[string]bool

	// readOnlyPaths are directories that can only be read
	readOnlyPaths map[string]bool

	// deniedPatterns are file patterns that are blocked
	deniedPatterns []string

	// sandbox is the active sandbox instance
	sandbox *Sandbox

	// workspaceRoot is the primary workspace mount point
	workspaceRoot string

	// enableAudit logs all file operations
	enableAudit bool

	// auditLog stores operation history
	auditLog []FileOperation
}

// FileOperation records a file system operation.
type FileOperation struct {
	Op        string `json:"op"`
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

// IsolationConfig configures file system isolation.
type IsolationConfig struct {
	AllowedPaths   []string `json:"allowed_paths"`
	ReadOnlyPaths  []string `json:"read_only_paths"`
	DeniedPatterns []string `json:"denied_patterns"`
	EnableAudit    bool     `json:"enable_audit"`
}

// DefaultIsolationConfig returns a secure default configuration.
func DefaultIsolationConfig() IsolationConfig {
	return IsolationConfig{
		AllowedPaths:  []string{},
		ReadOnlyPaths: []string{},
		DeniedPatterns: []string{
			".env",
			".git/config",
			"*.pem",
			"*.key",
			"id_rsa*",
			"credentials.json",
			"secrets.json",
		},
		EnableAudit: true,
	}
}

// NewFileSystemIsolation creates a new file system isolation manager.
func NewFileSystemIsolation(sandbox *Sandbox, workspaceRoot string, config IsolationConfig) *FileSystemIsolation {
	fsi := &FileSystemIsolation{
		sandbox:        sandbox,
		workspaceRoot:  filepath.Clean(workspaceRoot),
		allowedPaths:   make(map[string]bool),
		readOnlyPaths:  make(map[string]bool),
		deniedPatterns: config.DeniedPatterns,
		enableAudit:    config.EnableAudit,
		auditLog:       make([]FileOperation, 0),
	}

	// Normalize and store allowed paths
	for _, p := range config.AllowedPaths {
		fsi.allowedPaths[filepath.Clean(p)] = true
	}

	// Always allow workspace root
	fsi.allowedPaths[fsi.workspaceRoot] = true

	// Normalize and store read-only paths
	for _, p := range config.ReadOnlyPaths {
		fsi.readOnlyPaths[filepath.Clean(p)] = true
	}

	return fsi
}

// ValidatePath checks if a path is allowed for the given operation.
func (fsi *FileSystemIsolation) ValidatePath(path string, op string) error {
	fsi.mu.RLock()
	defer fsi.mu.RUnlock()

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Check denied patterns
	for _, pattern := range fsi.deniedPatterns {
		matched, err := filepath.Match(pattern, filepath.Base(cleanPath))
		if err == nil && matched {
			return fmt.Errorf("path matches denied pattern '%s': %s", pattern, path)
		}
		// Also check full path patterns
		matched, err = filepath.Match(pattern, cleanPath)
		if err == nil && matched {
			return fmt.Errorf("path matches denied pattern '%s': %s", pattern, path)
		}
	}

	// Check if path is within allowed paths
	allowed := false
	for allowedPath := range fsi.allowedPaths {
		if strings.HasPrefix(cleanPath, allowedPath) || cleanPath == allowedPath {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("path outside allowed directories: %s", path)
	}

	// Check read-only restrictions for write operations
	if op == "write" || op == "delete" {
		for roPath := range fsi.readOnlyPaths {
			if strings.HasPrefix(cleanPath, roPath) || cleanPath == roPath {
				return fmt.Errorf("path is read-only: %s", path)
			}
		}
	}

	return nil
}

// ReadFile reads a file through the sandbox.
func (fsi *FileSystemIsolation) ReadFile(ctx context.Context, path string) ([]byte, error) {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "read"); err != nil {
		fsi.logOp("read", path, false, err.Error())
		return nil, err
	}

	// Execute cat command in sandbox
	result := fsi.sandbox.Execute(ctx, "cat", containerPath)
	if !result.Success {
		err := fmt.Errorf("read failed: %s", result.Error)
		fsi.logOp("read", path, false, err.Error())
		return nil, err
	}

	fsi.logOp("read", path, true, "")
	return []byte(result.Output), nil
}

// WriteFile writes a file through the sandbox.
func (fsi *FileSystemIsolation) WriteFile(ctx context.Context, path string, content []byte) error {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "write"); err != nil {
		fsi.logOp("write", path, false, err.Error())
		return err
	}

	// Use printf to write content (handles special chars)
	// For large files, we should use CopyTo instead
	if len(content) > 10000 {
		// Write to temp file and copy
		tmpFile := filepath.Join(os.TempDir(), filepath.Base(path))
		if err := os.WriteFile(tmpFile, content, 0644); err != nil {
			fsi.logOp("write", path, false, err.Error())
			return err
		}
		defer os.Remove(tmpFile)

		if err := fsi.sandbox.CopyTo(ctx, tmpFile, containerPath); err != nil {
			fsi.logOp("write", path, false, err.Error())
			return err
		}
	} else {
		// Use echo for small files
		result := fsi.sandbox.Execute(ctx, "sh", "-c", fmt.Sprintf("printf '%%s' '%s' > %s",
			escapeForShell(string(content)), containerPath))
		if !result.Success {
			err := fmt.Errorf("write failed: %s", result.Error)
			fsi.logOp("write", path, false, err.Error())
			return err
		}
	}

	fsi.logOp("write", path, true, "")
	return nil
}

// DeleteFile deletes a file through the sandbox.
func (fsi *FileSystemIsolation) DeleteFile(ctx context.Context, path string) error {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "delete"); err != nil {
		fsi.logOp("delete", path, false, err.Error())
		return err
	}

	result := fsi.sandbox.Execute(ctx, "rm", "-f", containerPath)
	if !result.Success {
		err := fmt.Errorf("delete failed: %s", result.Error)
		fsi.logOp("delete", path, false, err.Error())
		return err
	}

	fsi.logOp("delete", path, true, "")
	return nil
}

// ListDir lists directory contents through the sandbox.
func (fsi *FileSystemIsolation) ListDir(ctx context.Context, path string) ([]string, error) {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "list"); err != nil {
		fsi.logOp("list", path, false, err.Error())
		return nil, err
	}

	result := fsi.sandbox.Execute(ctx, "ls", "-1", containerPath)
	if !result.Success {
		err := fmt.Errorf("list failed: %s", result.Error)
		fsi.logOp("list", path, false, err.Error())
		return nil, err
	}

	// Parse output
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	var entries []string
	for _, line := range lines {
		if line != "" {
			entries = append(entries, line)
		}
	}

	fsi.logOp("list", path, true, "")
	return entries, nil
}

// MkDir creates a directory through the sandbox.
func (fsi *FileSystemIsolation) MkDir(ctx context.Context, path string) error {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "mkdir"); err != nil {
		fsi.logOp("mkdir", path, false, err.Error())
		return err
	}

	result := fsi.sandbox.Execute(ctx, "mkdir", "-p", containerPath)
	if !result.Success {
		err := fmt.Errorf("mkdir failed: %s", result.Error)
		fsi.logOp("mkdir", path, false, err.Error())
		return err
	}

	fsi.logOp("mkdir", path, true, "")
	return nil
}

// Stat returns file information through the sandbox.
func (fsi *FileSystemIsolation) Stat(ctx context.Context, path string) (map[string]interface{}, error) {
	containerPath := fsi.toContainerPath(path)

	if err := fsi.ValidatePath(path, "stat"); err != nil {
		return nil, err
	}

	result := fsi.sandbox.Execute(ctx, "stat", "-c", "%s,%a,%F", containerPath)
	if !result.Success {
		return nil, fmt.Errorf("stat failed: %s", result.Error)
	}

	// Parse stat output
	parts := strings.Split(strings.TrimSpace(result.Output), ",")
	if len(parts) < 3 {
		return nil, errors.New("invalid stat output")
	}

	return map[string]interface{}{
		"size": parts[0],
		"mode": parts[1],
		"type": parts[2],
		"path": path,
	}, nil
}

// AddAllowedPath adds a path to the allowed list.
func (fsi *FileSystemIsolation) AddAllowedPath(path string) {
	fsi.mu.Lock()
	defer fsi.mu.Unlock()
	fsi.allowedPaths[filepath.Clean(path)] = true
}

// AddReadOnlyPath adds a path to the read-only list.
func (fsi *FileSystemIsolation) AddReadOnlyPath(path string) {
	fsi.mu.Lock()
	defer fsi.mu.Unlock()
	fsi.readOnlyPaths[filepath.Clean(path)] = true
}

// AddDeniedPattern adds a pattern to the denied list.
func (fsi *FileSystemIsolation) AddDeniedPattern(pattern string) {
	fsi.mu.Lock()
	defer fsi.mu.Unlock()
	fsi.deniedPatterns = append(fsi.deniedPatterns, pattern)
}

// GetAuditLog returns the audit log.
func (fsi *FileSystemIsolation) GetAuditLog() []FileOperation {
	fsi.mu.RLock()
	defer fsi.mu.RUnlock()
	return append([]FileOperation{}, fsi.auditLog...)
}

// ClearAuditLog clears the audit log.
func (fsi *FileSystemIsolation) ClearAuditLog() {
	fsi.mu.Lock()
	defer fsi.mu.Unlock()
	fsi.auditLog = make([]FileOperation, 0)
}

// toContainerPath converts a host path to a container path.
func (fsi *FileSystemIsolation) toContainerPath(hostPath string) string {
	rel, err := filepath.Rel(fsi.workspaceRoot, filepath.Clean(hostPath))
	if err != nil {
		return hostPath // fallback
	}
	return filepath.Join(fsi.sandbox.workDir, rel)
}

// logOp logs an operation for auditing.
func (fsi *FileSystemIsolation) logOp(op, path string, success bool, errMsg string) {
	if !fsi.enableAudit {
		return
	}

	fsi.mu.Lock()
	defer fsi.mu.Unlock()

	fsi.auditLog = append(fsi.auditLog, FileOperation{
		Op:        op,
		Path:      path,
		Success:   success,
		Timestamp: currentTime(),
		Error:     errMsg,
	})
}

// escapeForShell escapes a string for shell command.
func escapeForShell(s string) string {
	// Simple escaping - replace single quotes with '\'' and wrap in quotes
	return strings.ReplaceAll(s, "'", "'\\''")
}

// currentTime is a variable for testing.
var currentTime = func() int64 {
	return time.Now().Unix()
}
