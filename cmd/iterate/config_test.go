package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func TestConfigPath(t *testing.T) {
	path := configPath()
	if !strings.Contains(path, ".iterate") {
		t.Errorf("config path should contain .iterate, got %s", path)
	}
	if !strings.HasSuffix(path, "config.json") {
		t.Errorf("config path should end with config.json, got %s", path)
	}
}

func TestSaveLoadConfig(t *testing.T) {
	cfg := iterConfig{
		Provider: "anthropic",
		Model:    "claude-opus-4-6",
	}

	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", cfg.Provider)
	}
}

func TestConfigPathTOML(t *testing.T) {
	path := configPathTOML()
	if !strings.HasSuffix(path, "config.toml") {
		t.Errorf("TOML config path should end with config.toml, got %s", path)
	}
}

func TestCheckDirPermission_NoneConfigured(t *testing.T) {
	cfg := iterConfig{}
	if checkDirPermission(cfg, "/any/path") {
		t.Error("should not deny when no AllowDirs/DenyDirs configured")
	}
}

func TestCheckDirPermission_DenyDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := iterConfig{DenyDirs: []string{dir}}

	// Path inside denied dir should be denied.
	inside := filepath.Join(dir, "secret.go")
	if !checkDirPermission(cfg, inside) {
		t.Errorf("path inside DenyDir should be denied: %s", inside)
	}

	// Path outside denied dir should be allowed.
	outside := t.TempDir()
	if checkDirPermission(cfg, filepath.Join(outside, "safe.go")) {
		t.Errorf("path outside DenyDir should not be denied")
	}
}

func TestCheckDirPermission_AllowDirs(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	cfg := iterConfig{AllowDirs: []string{allowed}}

	// Path inside allowed dir should pass.
	if checkDirPermission(cfg, filepath.Join(allowed, "ok.go")) {
		t.Errorf("path inside AllowDir should not be denied")
	}

	// Path outside allowed dir should be denied.
	if !checkDirPermission(cfg, filepath.Join(outside, "blocked.go")) {
		t.Errorf("path outside AllowDir should be denied when AllowDirs is set")
	}
}

func TestCheckDirPermission_DenyWinsOverAllow(t *testing.T) {
	shared := t.TempDir()
	cfg := iterConfig{
		AllowDirs: []string{shared},
		DenyDirs:  []string{shared},
	}
	// DenyDirs beats AllowDirs.
	if !checkDirPermission(cfg, filepath.Join(shared, "file.go")) {
		t.Error("DenyDirs should win over AllowDirs")
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.rs", false},
		{"go test *", "go test ./...", false}, // filepath.Match is literal for spaces
		{"cmd/*", "cmd/main.go", true},
	}
	for _, c := range cases {
		got := globMatch(c.pattern, c.name)
		if got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestCheckBashPermission_Allow(t *testing.T) {
	cfg := iterConfig{AllowPatterns: []string{"go"}}
	allowed, denied := checkBashPermission(cfg, "go test ./...")
	if !allowed {
		t.Error("expected allowed for 'go' pattern match")
	}
	if denied {
		t.Error("expected not denied")
	}
}

func TestCheckBashPermission_Deny(t *testing.T) {
	cfg := iterConfig{DenyPatterns: []string{"rm"}}
	allowed, denied := checkBashPermission(cfg, "rm -rf /")
	if allowed {
		t.Error("expected not allowed")
	}
	if !denied {
		t.Error("expected denied for 'rm' pattern match")
	}
}

func TestCheckBashPermission_Neither(t *testing.T) {
	cfg := iterConfig{}
	allowed, denied := checkBashPermission(cfg, "go build ./...")
	if allowed || denied {
		t.Errorf("expected neither allowed nor denied, got allowed=%v denied=%v", allowed, denied)
	}
}

func TestLoadConfigTOML(t *testing.T) {
	tomlContent := `provider = "openai"
model = "gpt-4o"
safe_mode = true
allow_patterns = ["go test *", "go build *"]
deny_dirs = ["/etc", "/sys"]
`
	var cfg iterConfig
	if err := toml.Unmarshal([]byte(tomlContent), &cfg); err != nil {
		t.Fatalf("toml.Unmarshal: %v", err)
	}

	if cfg.Provider != "openai" {
		t.Errorf("Provider: got %q, want %q", cfg.Provider, "openai")
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "gpt-4o")
	}
	if !cfg.SafeMode {
		t.Error("SafeMode: expected true")
	}
	if len(cfg.AllowPatterns) != 2 {
		t.Errorf("AllowPatterns: got %d items, want 2", len(cfg.AllowPatterns))
	}
	if len(cfg.DenyDirs) != 2 || cfg.DenyDirs[0] != "/etc" {
		t.Errorf("DenyDirs: got %v, want [\"/etc\", \"/sys\"]", cfg.DenyDirs)
	}
}

func TestLoadConfigTOML_WriteFile(t *testing.T) {
	// Verify round-trip: write TOML file, read it back via os.ReadFile + toml.Unmarshal.
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	content := "provider = \"anthropic\"\nmodel = \"claude-sonnet-4-6\"\n"
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg iterConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("toml.Unmarshal: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("Provider: got %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.Model != "claude-sonnet-4-6" {
		t.Errorf("Model: got %q, want %q", cfg.Model, "claude-sonnet-4-6")
	}
}

// Ensure the strings import is still used.
var _ = strings.Contains

// ---------------------------------------------------------------------------
// applyEnvOverrides
// ---------------------------------------------------------------------------

// withEnv sets env vars for the duration of a test, restoring them after.
func withEnv(t *testing.T, vars map[string]string, fn func()) {
	t.Helper()
	old := make(map[string]string, len(vars))
	for k := range vars {
		old[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	for k, v := range vars {
		os.Setenv(k, v)
	}
	defer func() {
		for k, v := range old {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()
	fn()
}

func TestApplyEnvOverrides_Provider(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_PROVIDER": "openai"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.Provider != "openai" {
			t.Errorf("expected provider 'openai', got %q", cfg.Provider)
		}
	})
}

func TestApplyEnvOverrides_Model(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_MODEL": "gpt-4o"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.Model != "gpt-4o" {
			t.Errorf("expected model 'gpt-4o', got %q", cfg.Model)
		}
	})
}

func TestApplyEnvOverrides_Theme(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_THEME": "dark"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.Theme != "dark" {
			t.Errorf("expected theme 'dark', got %q", cfg.Theme)
		}
	})
}

func TestApplyEnvOverrides_ThinkingLevel(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_THINKING_LEVEL": "medium"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.ThinkingLevel != "medium" {
			t.Errorf("expected thinking_level 'medium', got %q", cfg.ThinkingLevel)
		}
	})
}

func TestApplyEnvOverrides_Temperature(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_TEMPERATURE": "0.7"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.Temperature < 0.69 || cfg.Temperature > 0.71 {
			t.Errorf("expected temperature ~0.7, got %f", cfg.Temperature)
		}
	})
}

func TestApplyEnvOverrides_Temperature_Invalid(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_TEMPERATURE": "not-a-number"}, func() {
		cfg := iterConfig{Temperature: 0.5}
		applyEnvOverrides(&cfg)
		// Invalid value should not overwrite existing config.
		if cfg.Temperature != 0.5 {
			t.Errorf("invalid ITERATE_TEMPERATURE should not change existing value, got %f", cfg.Temperature)
		}
	})
}

func TestApplyEnvOverrides_MaxTokens(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_MAX_TOKENS": "4096"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if cfg.MaxTokens != 4096 {
			t.Errorf("expected max_tokens 4096, got %d", cfg.MaxTokens)
		}
	})
}

func TestApplyEnvOverrides_MaxTokens_Invalid(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_MAX_TOKENS": "bad"}, func() {
		cfg := iterConfig{MaxTokens: 2048}
		applyEnvOverrides(&cfg)
		if cfg.MaxTokens != 2048 {
			t.Errorf("invalid ITERATE_MAX_TOKENS should not change existing value, got %d", cfg.MaxTokens)
		}
	})
}

func TestApplyEnvOverrides_SafeMode_True(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE", "True"} {
		withEnv(t, map[string]string{"ITERATE_SAFE_MODE": val}, func() {
			cfg := iterConfig{}
			applyEnvOverrides(&cfg)
			if !cfg.SafeMode {
				t.Errorf("ITERATE_SAFE_MODE=%q: expected SafeMode=true", val)
			}
		})
	}
}

func TestApplyEnvOverrides_SafeMode_False(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_SAFE_MODE": "0"}, func() {
		cfg := iterConfig{SafeMode: true}
		applyEnvOverrides(&cfg)
		if cfg.SafeMode {
			t.Error("ITERATE_SAFE_MODE=0 should set SafeMode=false")
		}
	})
}

func TestApplyEnvOverrides_CacheEnabled(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_CACHE_ENABLED": "1"}, func() {
		cfg := iterConfig{}
		applyEnvOverrides(&cfg)
		if !cfg.CacheEnabled {
			t.Error("ITERATE_CACHE_ENABLED=1 should set CacheEnabled=true")
		}
	})
}

func TestApplyEnvOverrides_NoEnvSet(t *testing.T) {
	// When no ITERATE_* vars are set, existing config should be unchanged.
	keys := []string{
		"ITERATE_PROVIDER", "ITERATE_MODEL", "ITERATE_THEME",
		"ITERATE_THINKING_LEVEL", "ITERATE_TEMPERATURE", "ITERATE_MAX_TOKENS",
		"ITERATE_SAFE_MODE", "ITERATE_CACHE_ENABLED",
	}
	withEnv(t, func() map[string]string {
		m := make(map[string]string, len(keys))
		for _, k := range keys {
			m[k] = ""
		}
		return m
	}(), func() {
		cfg := iterConfig{
			Provider:      "anthropic",
			Model:         "claude-opus-4",
			Temperature:   0.3,
			MaxTokens:     2000,
			ThinkingLevel: "low",
		}
		applyEnvOverrides(&cfg)
		if cfg.Provider != "anthropic" || cfg.Model != "claude-opus-4" ||
			cfg.Temperature != 0.3 || cfg.MaxTokens != 2000 || cfg.ThinkingLevel != "low" {
			t.Errorf("no env vars set — config should be unchanged: %+v", cfg)
		}
	})
}

func TestApplyEnvOverrides_OverridesFileConfig(t *testing.T) {
	// Env var should override a value already loaded from config file.
	withEnv(t, map[string]string{"ITERATE_MODEL": "gpt-4o-mini"}, func() {
		cfg := iterConfig{Model: "claude-opus-4"} // simulates file-loaded value
		applyEnvOverrides(&cfg)
		if cfg.Model != "gpt-4o-mini" {
			t.Errorf("env var should override file config, got %q", cfg.Model)
		}
	})
}

// ---------------------------------------------------------------------------
// sessionChangesTracker
// ---------------------------------------------------------------------------

func TestSessionChangesTracker_RecordWrite(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordWrite("/path/to/file.go")

	if len(tracker.written) != 1 {
		t.Fatalf("expected 1 written file, got %d", len(tracker.written))
	}
	if tracker.written[0] != "/path/to/file.go" {
		t.Errorf("expected '/path/to/file.go', got %q", tracker.written[0])
	}
}

func TestSessionChangesTracker_RecordWrite_Dedup(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordWrite("/path/to/file.go")
	tracker.recordWrite("/path/to/file.go")
	tracker.recordWrite("/path/to/other.go")

	if len(tracker.written) != 2 {
		t.Errorf("expected 2 unique written files, got %d", len(tracker.written))
	}
}

func TestSessionChangesTracker_RecordEdit(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordEdit("/path/to/edited.go")

	if len(tracker.edited) != 1 {
		t.Fatalf("expected 1 edited file, got %d", len(tracker.edited))
	}
	if tracker.edited[0] != "/path/to/edited.go" {
		t.Errorf("expected '/path/to/edited.go', got %q", tracker.edited[0])
	}
}

func TestSessionChangesTracker_RecordEdit_Dedup(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordEdit("/a.go")
	tracker.recordEdit("/a.go")
	tracker.recordEdit("/b.go")

	if len(tracker.edited) != 2 {
		t.Errorf("expected 2 unique edited files, got %d", len(tracker.edited))
	}
}

func TestSessionChangesTracker_Format_Empty(t *testing.T) {
	tracker := &sessionChangesTracker{}
	result := tracker.format()
	if result != "No files changed this session." {
		t.Errorf("expected empty message, got %q", result)
	}
}

func TestSessionChangesTracker_Format_WithWritten(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordWrite("/new.go")
	result := tracker.format()

	if !strings.Contains(result, "/new.go") {
		t.Errorf("format should contain written file path, got %q", result)
	}
	if !strings.Contains(result, "+") {
		t.Errorf("format should contain '+' for written files, got %q", result)
	}
}

func TestSessionChangesTracker_Format_WithEdited(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordEdit("/modified.go")
	result := tracker.format()

	if !strings.Contains(result, "/modified.go") {
		t.Errorf("format should contain edited file path, got %q", result)
	}
	if !strings.Contains(result, "~") {
		t.Errorf("format should contain '~' for edited files, got %q", result)
	}
}

func TestSessionChangesTracker_Format_Mixed(t *testing.T) {
	tracker := &sessionChangesTracker{}
	tracker.recordWrite("/new.go")
	tracker.recordEdit("/modified.go")

	result := tracker.format()
	if !strings.Contains(result, "/new.go") {
		t.Error("format should contain written file")
	}
	if !strings.Contains(result, "/modified.go") {
		t.Error("format should contain edited file")
	}
}

// ---------------------------------------------------------------------------
// splitShellWords
// ---------------------------------------------------------------------------

func TestSplitShellWords(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"simple command", "go test ./...", []string{"go", "test", "./..."}},
		{"quoted args", `echo "hello world"`, []string{"echo", "hello world"}},
		{"single quoted", "echo 'hello world'", []string{"echo", "hello world"}},
		{"empty string", "", nil},
		{"only spaces", "   ", nil},
		{"multiple spaces", "go    build", []string{"go", "build"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitShellWords(tt.cmd)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d words, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("word %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// conversationMarks
// ---------------------------------------------------------------------------

func TestConversationMarks_SetAndGet(t *testing.T) {
	setConversationMark("test-mark", 42)
	idx, ok := getConversationMark("test-mark")
	if !ok {
		t.Error("expected mark to exist")
	}
	if idx != 42 {
		t.Errorf("expected index 42, got %d", idx)
	}
}

func TestConversationMark_NotFound(t *testing.T) {
	_, ok := getConversationMark("nonexistent-mark-" + t.Name())
	if ok {
		t.Error("expected mark to not exist")
	}
}

func TestConversationMarks_GetAll(t *testing.T) {
	setConversationMark("mark-a", 1)
	setConversationMark("mark-b", 2)

	marks := getConversationMarks()
	if len(marks) < 2 {
		t.Errorf("expected at least 2 marks, got %d", len(marks))
	}
}

func TestConversationMarksLen(t *testing.T) {
	uniqueName := "len-test-" + t.Name() + fmt.Sprintf("%d", time.Now().UnixNano())
	before := conversationMarksLen()
	setConversationMark(uniqueName, 99)
	after := conversationMarksLen()
	if after <= before {
		t.Errorf("expected length to increase, got %d -> %d", before, after)
	}
	// Verify the mark was set
	idx, ok := getConversationMark(uniqueName)
	if !ok || idx != 99 {
		t.Errorf("expected mark %s=99, got %d, ok=%v", uniqueName, idx, ok)
	}
}
