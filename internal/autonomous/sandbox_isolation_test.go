// Package autonomous - Task 24: Tests for file system isolation
package autonomous

import (
	"testing"
)

func TestDefaultIsolationConfig(t *testing.T) {
	config := DefaultIsolationConfig()
	if len(config.DeniedPatterns) == 0 {
		t.Error("expected default denied patterns")
	}
	if !config.EnableAudit {
		t.Error("expected audit to be enabled by default")
	}
}

func TestNewFileSystemIsolation(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	if fsi == nil {
		t.Fatal("expected FileSystemIsolation to be created")
	}
	if fsi.workspaceRoot != "/workspace" {
		t.Errorf("expected workspaceRoot '/workspace', got %q", fsi.workspaceRoot)
	}
}

func TestValidatePath_Allowed(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	// Path within workspace should be allowed
	err := fsi.ValidatePath("/workspace/src/main.go", "read")
	if err != nil {
		t.Errorf("expected path to be allowed, got error: %v", err)
	}
}

func TestValidatePath_PathTraversal(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	// Path with .. should be blocked
	err := fsi.ValidatePath("/workspace/../etc/passwd", "read")
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestValidatePath_DeniedPattern(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	// .env files should be blocked by default
	err := fsi.ValidatePath("/workspace/.env", "read")
	if err == nil {
		t.Error("expected .env to be blocked by denied pattern")
	}
	
	// .pem files should be blocked
	err = fsi.ValidatePath("/workspace/cert.pem", "read")
	if err == nil {
		t.Error("expected .pem to be blocked by denied pattern")
	}
}

func TestValidatePath_ReadOnly(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	config := DefaultIsolationConfig()
	config.ReadOnlyPaths = []string{"/workspace/readonly"}
	fsi := NewFileSystemIsolation(sandbox, "/workspace", config)
	
	// Read should be allowed
	err := fsi.ValidatePath("/workspace/readonly/file.txt", "read")
	if err != nil {
		t.Errorf("expected read to be allowed on read-only path, got: %v", err)
	}
	
	// Write should be blocked
	err = fsi.ValidatePath("/workspace/readonly/file.txt", "write")
	if err == nil {
		t.Error("expected write to be blocked on read-only path")
	}
}

func TestValidatePath_OutsideAllowed(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	// Path outside workspace should be blocked
	err := fsi.ValidatePath("/etc/passwd", "read")
	if err == nil {
		t.Error("expected path outside workspace to be blocked")
	}
}

func TestAddAllowedPath(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	fsi.AddAllowedPath("/external")
	
	// Now /external should be allowed
	err := fsi.ValidatePath("/external/file.txt", "read")
	if err != nil {
		t.Errorf("expected /external/file.txt to be allowed after AddAllowedPath, got: %v", err)
	}
}

func TestAddDeniedPattern(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	fsi.AddDeniedPattern("*.secret")
	
	// Now .secret files should be blocked
	err := fsi.ValidatePath("/workspace/config.secret", "read")
	if err == nil {
		t.Error("expected .secret to be blocked after AddDeniedPattern")
	}
}

func TestGetAuditLog(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	// Trigger a validation failure
	_ = fsi.ValidatePath("/workspace/.env", "read")
	
	log := fsi.GetAuditLog()
	// Note: ValidatePath itself doesn't log, only operations do
	// This test verifies the log retrieval works
	if log == nil {
		t.Error("expected audit log to be non-nil")
	}
}

func TestClearAuditLog(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	fsi.ClearAuditLog()
	
	log := fsi.GetAuditLog()
	if len(log) != 0 {
		t.Error("expected audit log to be empty after ClearAuditLog")
	}
}

func TestToContainerPath(t *testing.T) {
	sandbox := NewSandbox(DefaultSandboxConfig())
	fsi := NewFileSystemIsolation(sandbox, "/workspace", DefaultIsolationConfig())
	
	containerPath := fsi.toContainerPath("/workspace/src/main.go")
	expected := "/workspace/src/main.go"
	if containerPath != expected {
		t.Errorf("expected %q, got %q", expected, containerPath)
	}
}

func TestTask24FileSystemIsolation(t *testing.T) {
	// Comprehensive test for Task 24
	sandbox := NewSandbox(DefaultSandboxConfig())
	config := IsolationConfig{
		AllowedPaths:   []string{"/workspace"},
		ReadOnlyPaths:  []string{"/workspace/docs"},
		DeniedPatterns: []string{"*.secret", "*.key"},
		EnableAudit:    true,
	}
	fsi := NewFileSystemIsolation(sandbox, "/workspace", config)
	
	// Verify configuration applied
	if len(fsi.deniedPatterns) < 2 {
		t.Error("expected denied patterns to be set")
	}
	if !fsi.enableAudit {
		t.Error("expected audit to be enabled")
	}
}
