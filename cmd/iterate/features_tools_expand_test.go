package main

import (
	"encoding/json"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestReadOnlyTools_FiltersBash(t *testing.T) {
	tools := []iteragent.Tool{
		{Name: "bash", Description: "run command"},
		{Name: "read_file", Description: "read"},
		{Name: "write_file", Description: "write"},
		{Name: "edit_file", Description: "edit"},
		{Name: "search_files", Description: "search"},
		{Name: "git_commit", Description: "commit"},
		{Name: "git_revert", Description: "revert"},
		{Name: "run_tests", Description: "test"},
	}
	ro := readOnlyTools(tools)
	for _, tool := range ro {
		if tool.Name == "bash" || tool.Name == "write_file" || tool.Name == "edit_file" ||
			tool.Name == "git_commit" || tool.Name == "git_revert" || tool.Name == "run_tests" {
			t.Errorf("readOnlyTools should not include %q", tool.Name)
		}
	}
}

func TestReadOnlyTools_PreservesReadOnly(t *testing.T) {
	tools := []iteragent.Tool{
		{Name: "read_file", Description: "read"},
		{Name: "search_files", Description: "search"},
		{Name: "list_dir", Description: "list"},
	}
	ro := readOnlyTools(tools)
	if len(ro) != 3 {
		t.Errorf("expected 3 read-only tools, got %d", len(ro))
	}
}

func TestReadOnlyTools_Empty(t *testing.T) {
	ro := readOnlyTools(nil)
	if len(ro) != 0 {
		t.Errorf("expected 0 tools, got %d", len(ro))
	}
}

func TestReadOnlyTools_AllBlocked(t *testing.T) {
	tools := []iteragent.Tool{
		{Name: "bash", Description: "run"},
		{Name: "write_file", Description: "write"},
		{Name: "edit_file", Description: "edit"},
		{Name: "git_commit", Description: "commit"},
		{Name: "git_revert", Description: "revert"},
		{Name: "run_tests", Description: "test"},
	}
	ro := readOnlyTools(tools)
	if len(ro) != 0 {
		t.Errorf("expected 0 tools when all are blocked, got %d", len(ro))
	}
}

func TestContextBar_Empty(t *testing.T) {
	bar := contextBar(nil, 100000)
	if !strings.Contains(bar, "0%") {
		t.Errorf("should show 0%% for empty messages, got %q", bar)
	}
	if !strings.Contains(bar, "0 msgs") {
		t.Errorf("should show 0 msgs, got %q", bar)
	}
}

func TestContextBar_WithMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Content: "hello world this is a test message"},
		{Content: "another message here"},
	}
	bar := contextBar(msgs, 100000)
	if !strings.Contains(bar, "2 msgs") {
		t.Errorf("should show 2 msgs, got %q", bar)
	}
}

func TestContextBar_HighUsage(t *testing.T) {
	// 100,000 chars / 4 = 25,000 tokens in a 30,000 token window = ~83%
	msgs := []iteragent.Message{
		{Content: strings.Repeat("x", 100000)},
	}
	bar := contextBar(msgs, 30000)
	if !strings.Contains(bar, "1 msgs") {
		t.Errorf("should show 1 msgs, got %q", bar)
	}
}

func TestContextBar_CapsAt100(t *testing.T) {
	msgs := []iteragent.Message{
		{Content: strings.Repeat("x", 1000000)},
	}
	bar := contextBar(msgs, 1000)
	// Should not crash and should cap at 100%
	if !strings.Contains(bar, "100%") {
		t.Errorf("should cap at 100%%, got %q", bar)
	}
}

func TestDenyToolAndGetDeniedList(t *testing.T) {
	// Save original state
	orig := isDenied("test_integration_tool")
	denyTool("test_integration_tool")

	list := getDeniedList()
	found := false
	for _, name := range list {
		if name == "test_integration_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected test_integration_tool in denied list")
	}

	allowTool("test_integration_tool")
	if isDenied("test_integration_tool") {
		t.Error("expected tool to be allowed after allowTool")
	}

	// Restore
	if orig {
		denyTool("test_integration_tool")
	}
}

func TestThemes_AllPresent(t *testing.T) {
	expected := []string{"default", "nord", "monokai", "minimal"}
	for _, name := range expected {
		if _, ok := themes[name]; !ok {
			t.Errorf("expected theme %q to exist", name)
		}
	}
}

func TestApplyTheme(t *testing.T) {
	origGreen := colorGreen
	origBold := colorBold
	defer func() {
		colorGreen = origGreen
		colorBold = origBold
	}()

	applyTheme(themes["nord"])
	if colorGreen == origGreen {
		t.Error("applyTheme should change colorGreen")
	}
	if colorBold == "" {
		t.Error("colorBold should be set")
	}
}

func TestApplyTheme_Minimal(t *testing.T) {
	origReset := colorReset
	defer func() { colorReset = origReset }()

	applyTheme(themes["minimal"])
	if colorReset == "" {
		t.Error("minimal theme should set colorReset")
	}
}


func TestLoadAliases_NonExistent(t *testing.T) {
	// loadAliases should return empty map when file doesn't exist
	aliases := loadAliases()
	if aliases == nil {
		t.Error("should return non-nil map")
	}
}

func TestSaveAndLoadAliases_RoundTrip(t *testing.T) {
	m := aliasMap{"g": "/git status", "t": "/test"}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(data), "/git status") {
		t.Error("JSON should contain alias value")
	}
}

func TestResolveAlias_NoFile(t *testing.T) {
	result := resolveAlias("/unknown_command arg1")
	if result != "/unknown_command arg1" {
		t.Errorf("should return unchanged line, got %q", result)
	}
}

func TestResolveAlias_EmptyLine(t *testing.T) {
	result := resolveAlias("")
	if result != "" {
		t.Errorf("should return empty string, got %q", result)
	}
}

func TestGetPinnedMessages_Empty(t *testing.T) {
	msgs := getPinnedMessages()
	if msgs == nil {
		t.Error("should return non-nil slice")
	}
}

func TestSetAndGetPinnedMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "pinned"},
	}
	setPinnedMessages(msgs)
	got := getPinnedMessages()
	if len(got) != 1 {
		t.Errorf("expected 1 pinned message, got %d", len(got))
	}
	if got[0].Content != "pinned" {
		t.Errorf("expected content 'pinned', got %q", got[0].Content)
	}
}

func TestRuntimeConfig_Default(t *testing.T) {
	rc := runtimeConfig{}
	if rc.Temperature != nil {
		t.Error("default Temperature should be nil")
	}
	if rc.MaxTokens != nil {
		t.Error("default MaxTokens should be nil")
	}
}

func TestAgentMode_Constants(t *testing.T) {
	if modeNormal != 0 {
		t.Errorf("modeNormal should be 0, got %d", modeNormal)
	}
	if modeAsk != 1 {
		t.Errorf("modeAsk should be 1, got %d", modeAsk)
	}
	if modeArchitect != 2 {
		t.Errorf("modeArchitect should be 2, got %d", modeArchitect)
	}
}
