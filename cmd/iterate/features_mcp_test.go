package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GrayCodeAI/iterate/internal/commands"
)

func TestSaveMCPServers_ReturnsErrorOnFailure(t *testing.T) {
	// Try to save to a read-only directory to force an error
	servers := []commands.MCPServerEntry{
		{Name: "test", URL: "http://localhost:8080"},
	}

	// Create a temp directory and make it read-only
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil {
		t.Skip("cannot create read-only dir for test")
	}
	defer os.Chmod(readOnlyDir, 0o755) // cleanup

	// Override the path function temporarily
	oldPath := mcpServersPath
	mcpServersPath = func() string {
		return filepath.Join(readOnlyDir, "mcp_servers.json")
	}
	defer func() { mcpServersPath = oldPath }()

	err := saveMCPServers(servers)
	if err == nil {
		t.Error("expected saveMCPServers to return error for read-only dir, got nil")
	}
}

func TestSaveMCPServers_Success(t *testing.T) {
	// Use a temp directory for the test
	tmpDir := t.TempDir()

	// Override the path function
	oldPath := mcpServersPath
	mcpServersPath = func() string {
		return filepath.Join(tmpDir, "mcp_servers.json")
	}
	defer func() { mcpServersPath = oldPath }()

	servers := []commands.MCPServerEntry{
		{Name: "test-server", URL: "http://localhost:8080", Command: "test", Args: []string{"-a", "-b"}},
	}

	err := saveMCPServers(servers)
	if err != nil {
		t.Fatalf("saveMCPServers failed: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(filepath.Join(tmpDir, "mcp_servers.json"))
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if len(data) == 0 {
		t.Error("saved file is empty")
	}

	// Verify we can load it back
	loaded := loadMCPServers()
	if len(loaded) != 1 {
		t.Errorf("expected 1 server, got %d", len(loaded))
	}
	if len(loaded) > 0 && loaded[0].Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", loaded[0].Name)
	}
}

// mcpServersPath is a variable so tests can override it
var mcpServersPath = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "mcp_servers.json")
}

// Override the package function
func init() {
	// This allows tests to override the path
}
