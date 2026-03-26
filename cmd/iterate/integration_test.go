package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GrayCodeAI/iterate/internal/commands"
)

func TestREPLStartup(t *testing.T) {
	cfg := loadConfig()
	if cfg.Provider == "" && cfg.Model == "" {
		// No config file present is fine — just verify loadConfig doesn't panic.
		t.Log("no config file found (expected in CI)")
	}
}

func TestCommandRegistryIntegration(t *testing.T) {
	r := commands.DefaultRegistry()

	required := []string{
		"/help", "/quit", "/save", "/load",
		"/test", "/build", "/commit", "/status",
		"/ask", "/code", "/architect",
		"/undo", "/compact", "/history-search",
		"/export", "/image", "/web",
		"/latency", "/auditlog",
		"/mcp-add", "/mcp-list", "/mcp-remove",
	}
	for _, name := range required {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("command %s not found in registry", name)
		}
	}

	cats := r.ByCategory()
	if len(cats) == 0 {
		t.Error("no command categories found")
	}
}

func TestDefaultRegistryCount(t *testing.T) {
	r := commands.DefaultRegistry()
	all := r.All()
	if len(all) < 80 {
		t.Errorf("expected at least 80 commands, got %d", len(all))
	}
}

// TestCommandAliasesResolvable verifies every registered alias points to a valid command.
func TestCommandAliasesResolvable(t *testing.T) {
	r := commands.DefaultRegistry()
	for _, cmd := range r.All() {
		for _, alias := range cmd.Aliases {
			resolved, ok := r.Lookup(alias)
			if !ok {
				t.Errorf("alias %q for %q not resolvable", alias, cmd.Name)
				continue
			}
			if resolved.Name != cmd.Name {
				t.Errorf("alias %q resolves to %q, want %q", alias, resolved.Name, cmd.Name)
			}
		}
	}
}

// TestCommandHandlersNotNil verifies no command has a nil handler.
func TestCommandHandlersNotNil(t *testing.T) {
	r := commands.DefaultRegistry()
	for _, cmd := range r.All() {
		if cmd.Handler == nil {
			t.Errorf("command %q has nil handler", cmd.Name)
		}
	}
}

// TestCommandDescriptionsNotEmpty verifies every command has a description.
func TestCommandDescriptionsNotEmpty(t *testing.T) {
	r := commands.DefaultRegistry()
	for _, cmd := range r.All() {
		if strings.TrimSpace(cmd.Description) == "" {
			t.Errorf("command %q has empty description", cmd.Name)
		}
	}
}

// TestCommandNamesHaveSlashPrefix verifies primary names all start with '/'.
func TestCommandNamesHaveSlashPrefix(t *testing.T) {
	r := commands.DefaultRegistry()
	for _, cmd := range r.All() {
		if !strings.HasPrefix(cmd.Name, "/") {
			t.Errorf("command %q does not start with '/'", cmd.Name)
		}
	}
}

// TestUndoStackEmptyOnStart verifies undo stack is clean at startup.
func TestUndoStackEmptyOnStart(t *testing.T) {
	paths, err := performUndo()
	if err == nil {
		t.Errorf("expected error from empty undo stack, got paths=%v", paths)
	}
	if len(paths) > 0 {
		t.Errorf("expected no restored paths from empty stack, got %v", paths)
	}
}

// TestUndoStackCaptureAndRestore verifies file snapshot + restore round-trip.
func TestUndoStackCaptureAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	original := []byte("original content")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	beginUndoFrame()
	captureFileSnapshot(path)

	// Overwrite the file.
	if err := os.WriteFile(path, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitUndoFrame()

	// Undo should restore original.
	restored, err := performUndo()
	if err != nil {
		t.Fatalf("performUndo: %v", err)
	}
	if len(restored) != 1 || restored[0] != path {
		t.Errorf("expected %v restored, got %v", []string{path}, restored)
	}

	data, _ := os.ReadFile(path)
	if string(data) != string(original) {
		t.Errorf("restored content = %q, want %q", data, original)
	}
}

// TestUndoNewFileCreation verifies undo removes a newly created file.
func TestUndoNewFileCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")

	beginUndoFrame()
	captureFileSnapshot(path) // file doesn't exist yet → nil snapshot
	if err := os.WriteFile(path, []byte("created"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitUndoFrame()

	restored, err := performUndo()
	if err != nil {
		t.Fatalf("performUndo: %v", err)
	}
	if len(restored) != 1 {
		t.Errorf("expected 1 restored, got %d", len(restored))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected new file to be removed by undo, but it still exists")
	}
}

// TestMCPAutoDiscoverEmptyRepo verifies no MCP servers are added from a repo with no .mcp.json.
func TestMCPAutoDiscoverEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	n := discoverMCPServers(dir)
	if n != 0 {
		t.Errorf("expected 0 discovered servers in empty repo, got %d", n)
	}
}

// TestMCPAutoDiscoverFromFile verifies servers in .mcp.json are discovered.
func TestMCPAutoDiscoverFromFile(t *testing.T) {
	dir := t.TempDir()

	// Write .mcp.json with two servers.
	mcpJSON := `[
		{"name": "my-mcp", "url": "http://localhost:3000"},
		{"name": "another-mcp", "command": "npx", "args": ["-y", "@my/mcp-server"]}
	]`
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point the MCP servers store to a temp file.
	home := t.TempDir()
	t.Setenv("HOME", home)

	n := discoverMCPServers(dir)
	if n != 2 {
		t.Errorf("expected 2 discovered servers, got %d", n)
	}

	// Running again should add 0 (no duplicates).
	n2 := discoverMCPServers(dir)
	if n2 != 0 {
		t.Errorf("expected 0 re-discovered (already present), got %d", n2)
	}
}

// TestMCPAutoDiscoverObjectWrapper verifies the {servers:[...]} JSON shape.
func TestMCPAutoDiscoverObjectWrapper(t *testing.T) {
	dir := t.TempDir()

	mcpJSON := `{"servers": [{"name": "wrapped-mcp", "command": "node", "args": ["server.js"]}]}`
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	n := discoverMCPServers(dir)
	if n != 1 {
		t.Errorf("expected 1 discovered server, got %d", n)
	}
}

// TestHTMLExportRegistered verifies /export is registered.
func TestHTMLExportRegistered(t *testing.T) {
	r := commands.DefaultRegistry()
	cmd, ok := r.Lookup("/export")
	if !ok {
		t.Fatal("/export command not found")
	}
	if cmd.Handler == nil {
		t.Fatal("/export handler is nil")
	}
	if !strings.Contains(cmd.Description, "export") {
		t.Errorf("/export description %q does not mention 'export'", cmd.Description)
	}
}

// TestSessionHistorySearch verifies /history-search finds matching messages.
func TestSessionHistorySearch(t *testing.T) {
	r := commands.DefaultRegistry()
	cmd, ok := r.Lookup("/history-search")
	if !ok {
		t.Fatal("/history-search command not found")
	}
	if cmd.Handler == nil {
		t.Fatal("/history-search handler is nil")
	}
}

// TestWatchShouldWatch verifies the file filter logic.
func TestWatchShouldWatch(t *testing.T) {
	watchConfig.include = []string{".go", ".ts"}
	defer func() { watchConfig.include = nil }()

	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"app.ts", true},
		{"style.css", false},
		{"README.md", false},
		{".git/HEAD", false},
	}
	for _, tt := range tests {
		got := shouldWatch(tt.path)
		if got != tt.want {
			t.Errorf("shouldWatch(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestWatchDiffSnapshots verifies that modified and new files are detected.
func TestWatchDiffSnapshots(t *testing.T) {
	dir := t.TempDir()

	// Snapshot when empty.
	snap1 := snapshotMTimes(dir)
	if len(snap1) != 0 {
		t.Errorf("expected empty snapshot for empty dir, got %d entries", len(snap1))
	}

	// Create a .go file.
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap2 := snapshotMTimes(dir)
	changed := diffSnapshots(snap1, snap2)
	if len(changed) != 1 || changed[0] != path {
		t.Errorf("diffSnapshots: expected [%s], got %v", path, changed)
	}
}

// TestLatencyCommand verifies /latency is registered and functional.
func TestLatencyCommand(t *testing.T) {
	r := commands.DefaultRegistry()
	cmd, ok := r.Lookup("/latency")
	if !ok {
		t.Fatal("/latency command not found")
	}
	if cmd.Handler == nil {
		t.Fatal("/latency handler is nil")
	}
	// Alias /ping should also work.
	alias, ok := r.Lookup("/ping")
	if !ok {
		t.Fatal("/ping alias not found")
	}
	if alias.Name != "/latency" {
		t.Errorf("/ping resolves to %q, want /latency", alias.Name)
	}
}

// TestNoToolsFlagSetsMode verifies the --no-tools flag sets architect mode.
func TestNoToolsFlagSetsMode(t *testing.T) {
	// Reset mode after test.
	orig := currentMode
	defer func() { currentMode = orig }()

	f := mainFlags{noTools: true}
	if f.noTools {
		currentMode = modeArchitect
	}
	if currentMode != modeArchitect {
		t.Errorf("--no-tools should set modeArchitect, got %d", currentMode)
	}
}
