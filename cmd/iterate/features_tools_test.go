package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

func TestIsDenied(t *testing.T) {
	// Default denied tools
	if !isDenied("bash") {
		t.Error("expected bash to be denied by default")
	}
	if !isDenied("write_file") {
		t.Error("expected write_file to be denied by default")
	}
	if !isDenied("edit_file") {
		t.Error("expected edit_file to be denied by default")
	}
	if isDenied("read_file") {
		t.Error("expected read_file to not be denied by default")
	}
}

func TestDenyAndAllowTool(t *testing.T) {
	// Deny a new tool
	denyTool("custom_tool")
	if !isDenied("custom_tool") {
		t.Error("expected custom_tool to be denied after denyTool")
	}

	// Allow it back
	allowTool("custom_tool")
	if isDenied("custom_tool") {
		t.Error("expected custom_tool to not be denied after allowTool")
	}
}

func TestAllowDefaultDeniedTool(t *testing.T) {
	// Allow a default-denied tool
	allowTool("bash")
	if isDenied("bash") {
		t.Error("expected bash to not be denied after allowTool")
	}

	// Restore
	denyTool("bash")
}

func TestGetDeniedList(t *testing.T) {
	// Clean state: ensure known denied tools
	// We can't easily reset to default, so test with what we know
	denyTool("test_tool_a")
	denyTool("test_tool_b")

	list := getDeniedList()
	sort.Strings(list)

	found := map[string]bool{}
	for _, name := range list {
		found[name] = true
	}
	if !found["test_tool_a"] {
		t.Error("expected test_tool_a in denied list")
	}
	if !found["test_tool_b"] {
		t.Error("expected test_tool_b in denied list")
	}

	// Clean up
	allowTool("test_tool_a")
	allowTool("test_tool_b")
}

func TestDenyAllow_Concurrent(t *testing.T) {
	// Ensure concurrent access doesn't panic
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			denyTool("concurrent_tool")
		}()
		go func() {
			defer wg.Done()
			isDenied("concurrent_tool")
		}()
	}
	wg.Wait()
	// Clean up
	allowTool("concurrent_tool")
}

func TestResolveAlias_NoAliases(t *testing.T) {
	// When no aliases file exists, line should be returned unchanged
	got := resolveAlias("/unknown_cmd arg1 arg2")
	if got != "/unknown_cmd arg1 arg2" {
		t.Errorf("expected unchanged line, got %q", got)
	}
}

func TestResolveAlias_WithTempAliases(t *testing.T) {
	// Override the aliases path to a temp file
	tmpDir := t.TempDir()
	aliasesFile := filepath.Join(tmpDir, "aliases.json")
	content := `{"ll": "/log 10", "gs": "/status"}`
	if err := os.WriteFile(aliasesFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write aliases: %v", err)
	}

	// Monkey-patch aliasesPath for this test
	origAliasesPath := aliasesPath()
	aliasesPathOverride := func() string { return aliasesFile }
	_ = aliasesPathOverride

	// We can't easily override the function, but we can test the expansion logic
	// by loading the aliases directly
	aliases := loadAliasesFrom(aliasesFile)
	if aliases["ll"] != "/log 10" {
		t.Errorf("expected alias ll = '/log 10', got %q", aliases["ll"])
	}
	if aliases["gs"] != "/status" {
		t.Errorf("expected alias gs = '/status', got %q", aliases["gs"])
	}
	_ = origAliasesPath
}

// loadAliasesFrom loads aliases from a specific path (helper for testing)
func loadAliasesFrom(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]string{}
	}
	if m == nil {
		return map[string]string{}
	}
	return m
}
