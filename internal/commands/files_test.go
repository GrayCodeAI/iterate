package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// cmdFind
// ---------------------------------------------------------------------------

func TestCmdFind_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/find"},
	}
	result := cmdFind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdFind_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "utils.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# README"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/find", "main"},
	}
	result := cmdFind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdFind_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/find", "nonexistent"},
	}
	result := cmdFind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdFind_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	os.MkdirAll(hiddenDir, 0o755)
	os.WriteFile(filepath.Join(hiddenDir, "secret.go"), []byte("secret"), 0o644)
	os.WriteFile(filepath.Join(dir, "visible.go"), []byte("visible"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/find", "secret"},
	}
	result := cmdFind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdGrep
// ---------------------------------------------------------------------------

func TestCmdGrep_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/grep"},
	}
	result := cmdGrep(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdGrep_WithPattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc hello() {}\n"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/grep", "hello"},
	}
	result := cmdGrep(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdGrep_NoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/grep", "nonexistent_pattern_xyz"},
	}
	result := cmdGrep(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdWeb
// ---------------------------------------------------------------------------

func TestCmdWeb_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/web"},
	}
	result := cmdWeb(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdWeb_AddsHTTPPrefix(t *testing.T) {
	ctx := Context{
		Parts: []string{"/web", "example.com"},
	}
	// This will try to fetch but fail in tests; we just verify it handles it
	result := cmdWeb(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdWeb_WithHTTPPrefix(t *testing.T) {
	ctx := Context{
		Parts: []string{"/web", "http://example.com"},
	}
	result := cmdWeb(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdPwd
// ---------------------------------------------------------------------------

func TestCmdPwd(t *testing.T) {
	ctx := Context{
		RepoPath: "/some/path",
		Parts:    []string{"/pwd"},
	}
	result := cmdPwd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdCd
// ---------------------------------------------------------------------------

func TestCmdCd_NoArg(t *testing.T) {
	ctx := Context{
		RepoPath: "/some/path",
		Parts:    []string{"/cd"},
	}
	result := cmdCd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCd_ValidDir(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/cd", "."},
	}
	result := cmdCd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCd_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/cd", "nonexistent_dir"},
	}
	result := cmdCd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdLs
// ---------------------------------------------------------------------------

func TestCmdLs_CurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/ls"},
	}
	result := cmdLs(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdLs_SubDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "subdir", "file.txt"), []byte("content"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/ls", "subdir"},
	}
	result := cmdLs(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdLs_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/ls", "nonexistent"},
	}
	result := cmdLs(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdAdd
// ---------------------------------------------------------------------------

func TestCmdAdd_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/add"},
	}
	result := cmdAdd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdAdd_WithFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0o644)

	agent := newTestAgent()
	ctx := Context{
		RepoPath: dir,
		Agent:    agent,
		Parts:    []string{"/add", "test.go"},
	}
	result := cmdAdd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 1 {
		t.Errorf("expected 1 message injected, got %d", len(agent.Messages))
	}
	if !strings.Contains(agent.Messages[0].Content, "test.go") {
		t.Error("expected message to contain filename")
	}
}

func TestCmdAdd_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/add", "nonexistent.go"},
	}
	result := cmdAdd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdAdd_NilAgent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Agent:    nil,
		Parts:    []string{"/add", "test.go"},
	}
	result := cmdAdd(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdTodos
// ---------------------------------------------------------------------------

func TestCmdTodos(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("// TODO: fix this\npackage main\n"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/todos"},
	}
	result := cmdTodos(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdDeps
// ---------------------------------------------------------------------------

func TestCmdDeps_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/deps"},
	}
	result := cmdDeps(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdDeps_WithGoMod(t *testing.T) {
	dir := t.TempDir()
	goMod := `module example.com/test

go 1.21

require (
	github.com/foo/bar v1.0.0
	github.com/baz/qux v2.0.0
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/deps"},
	}
	result := cmdDeps(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdSearch
// ---------------------------------------------------------------------------

func TestCmdSearch_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/search"},
	}
	result := cmdSearch(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdSearch_WithQuery(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc hello() {}\n"), 0o644)

	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/search", "hello"},
	}
	result := cmdSearch(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// RegisterFileCommands
// ---------------------------------------------------------------------------

func TestRegisterFileCommands(t *testing.T) {
	r := NewRegistry()
	RegisterFileCommands(r)

	expected := []string{"/add", "/find", "/web", "/grep", "/todos", "/deps", "/search", "/pwd", "/cd", "/ls"}
	for _, name := range expected {
		if _, ok := r.Lookup(name); !ok {
			t.Errorf("expected %s to be registered", name)
		}
	}
}

// ---------------------------------------------------------------------------
// cmdSearchReplace
// ---------------------------------------------------------------------------

func TestCmdSearchReplace_NoArgs(t *testing.T) {
	ctx := Context{
		Parts: []string{"/search-replace"},
	}
	result := cmdSearchReplace(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdSearchReplace_OneArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/search-replace", "old"},
	}
	result := cmdSearchReplace(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdOpen
// ---------------------------------------------------------------------------

func TestCmdOpen_NoArg(t *testing.T) {
	ctx := Context{
		Parts: []string{"/open"},
	}
	result := cmdOpen(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// loadPins / savePins
// ---------------------------------------------------------------------------

func TestLoadPins_MissingFile(t *testing.T) {
	dir := t.TempDir()
	pins := loadPins(dir)
	if pins != nil {
		t.Error("expected nil pins for missing file")
	}
}

func TestLoadPins_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".iterate"), 0o755)
	os.WriteFile(filepath.Join(dir, ".iterate", "pins.json"), []byte("not json"), 0o644)

	pins := loadPins(dir)
	if pins != nil {
		t.Error("expected nil pins for invalid JSON")
	}
}

func TestSaveAndLoadPins(t *testing.T) {
	dir := t.TempDir()
	msgs := []iteragent.Message{
		{Role: "user", Content: "pinned message"},
	}
	savePins(dir, msgs)
	pins := loadPins(dir)
	if len(pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(pins))
	}
	if pins[0].Content != "pinned message" {
		t.Errorf("expected 'pinned message', got %q", pins[0].Content)
	}
}
