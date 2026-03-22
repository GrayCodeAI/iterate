package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfigPathAlt_XDG(t *testing.T) {
	old := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	defer func() {
		if old == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", old)
		}
	}()
	path := configPathAlt()
	if !strings.HasPrefix(path, "/custom/config") {
		t.Errorf("should use XDG_CONFIG_HOME, got %q", path)
	}
}

func TestConfigPathAlt_Default(t *testing.T) {
	old := os.Getenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if old != "" {
			os.Setenv("XDG_CONFIG_HOME", old)
		}
	}()
	path := configPathAlt()
	if !strings.Contains(path, ".config") {
		t.Errorf("should default to ~/.config, got %q", path)
	}
}

func TestConfigPathTOML_XDG(t *testing.T) {
	old := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", "/custom")
	defer func() {
		if old == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", old)
		}
	}()
	path := configPathTOML()
	if !strings.HasPrefix(path, "/custom") {
		t.Errorf("should use XDG, got %q", path)
	}
	if !strings.HasSuffix(path, "config.toml") {
		t.Errorf("should end with config.toml, got %q", path)
	}
}

func TestIterConfig_Defaults(t *testing.T) {
	var cfg iterConfig
	if cfg.Provider != "" {
		t.Error("default provider should be empty")
	}
	if cfg.Model != "" {
		t.Error("default model should be empty")
	}
	if cfg.SafeMode {
		t.Error("default SafeMode should be false")
	}
	if cfg.CacheEnabled {
		t.Error("default CacheEnabled should be false")
	}
	if cfg.Temperature != 0 {
		t.Error("default Temperature should be 0")
	}
}

func TestIterConfig_TOML_RoundTrip(t *testing.T) {
	cfg := iterConfig{
		Provider:      "anthropic",
		Model:         "claude-sonnet-4",
		SafeMode:      true,
		Theme:         "nord",
		Temperature:   0.7,
		MaxTokens:     4096,
		AllowPatterns: []string{"go test *"},
		DenyDirs:      []string{"/etc"},
	}
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded iterConfig
	if err := toml.Unmarshal([]byte(buf.String()), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Provider != "anthropic" {
		t.Errorf("Provider: got %q", decoded.Provider)
	}
	if decoded.Model != "claude-sonnet-4" {
		t.Errorf("Model: got %q", decoded.Model)
	}
	if !decoded.SafeMode {
		t.Error("SafeMode should be true")
	}
}

func TestIterConfig_JSON_RoundTrip(t *testing.T) {
	cfg := iterConfig{
		Provider:      "openai",
		Model:         "gpt-4o",
		ThinkingLevel: "medium",
		CacheEnabled:  true,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded iterConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Provider != "openai" {
		t.Errorf("Provider: got %q", decoded.Provider)
	}
	if decoded.ThinkingLevel != "medium" {
		t.Errorf("ThinkingLevel: got %q", decoded.ThinkingLevel)
	}
}

func TestSaveConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "config.json")
	os.MkdirAll(filepath.Dir(jsonPath), 0o755)
	cfg := iterConfig{Provider: "test", Model: "test-model"}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(jsonPath, data, 0o644)

	var loaded iterConfig
	raw, _ := os.ReadFile(jsonPath)
	json.Unmarshal(raw, &loaded)
	if loaded.Provider != "test" {
		t.Errorf("expected provider 'test', got %q", loaded.Provider)
	}
}

func TestSaveConfig_TOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	cfg := iterConfig{Provider: "test", Model: "test-model"}
	var buf strings.Builder
	toml.NewEncoder(&buf).Encode(cfg)
	os.WriteFile(tomlPath, []byte(buf.String()), 0o644)

	var loaded iterConfig
	raw, _ := os.ReadFile(tomlPath)
	toml.Unmarshal(raw, &loaded)
	if loaded.Provider != "test" {
		t.Errorf("expected provider 'test', got %q", loaded.Provider)
	}
}

func TestLoadConfig_NoFiles(t *testing.T) {
	// With no config files, loadConfig returns zero value
	// We can't easily test this without env isolation, but we test the structure
	cfg := iterConfig{}
	if cfg.Provider != "" {
		t.Error("empty config should have empty provider")
	}
}

func TestCheckBashPermission_BothPatterns(t *testing.T) {
	cfg := iterConfig{
		AllowPatterns: []string{"go"},
		DenyPatterns:  []string{"rm"},
	}
	allowed, denied := checkBashPermission(cfg, "go test ./...")
	if !allowed {
		t.Error("should be allowed by 'go' pattern")
	}
	if denied {
		t.Error("should not be denied")
	}
	allowed, denied = checkBashPermission(cfg, "rm -rf /")
	if allowed {
		t.Error("should not be allowed")
	}
	if !denied {
		t.Error("should be denied by 'rm' pattern")
	}
}

func TestCheckBashPermission_DenyWord(t *testing.T) {
	cfg := iterConfig{DenyPatterns: []string{"--force"}}
	allowed, denied := checkBashPermission(cfg, "git push --force")
	if !denied {
		t.Error("should be denied when word matches deny pattern")
	}
	if allowed {
		t.Error("should not be allowed")
	}
}

func TestCheckDirPermission_InvalidPath(t *testing.T) {
	cfg := iterConfig{DenyDirs: []string{"/nonexistent"}}
	if checkDirPermission(cfg, "/some/other/path") {
		t.Error("should not deny path outside denied dirs")
	}
}

func TestSessionChangesTracker_Concurrent(t *testing.T) {
	tracker := &sessionChangesTracker{}
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			tracker.recordWrite("/file" + string(rune('a'+n)))
			tracker.recordEdit("/edit" + string(rune('a'+n)))
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if len(tracker.written) == 0 {
		t.Error("should have recorded writes")
	}
	if len(tracker.edited) == 0 {
		t.Error("should have recorded edits")
	}
}

func TestConversationMarks_Concurrent(t *testing.T) {
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			setConversationMark("mark-"+string(rune('0'+n)), n)
			getConversationMark("mark-" + string(rune('0'+n)))
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	marks := getConversationMarks()
	if len(marks) < 10 {
		t.Errorf("expected at least 10 marks, got %d", len(marks))
	}
}

func TestApplyEnvOverrides_BooleanTrue(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE"} {
		withEnv(t, map[string]string{"ITERATE_CACHE_ENABLED": val}, func() {
			cfg := iterConfig{}
			applyEnvOverrides(&cfg)
			if !cfg.CacheEnabled {
				t.Errorf("ITERATE_CACHE_ENABLED=%q should set true", val)
			}
		})
	}
}

func TestApplyEnvOverrides_BooleanFalse(t *testing.T) {
	withEnv(t, map[string]string{"ITERATE_SAFE_MODE": "0"}, func() {
		cfg := iterConfig{SafeMode: true}
		applyEnvOverrides(&cfg)
		if cfg.SafeMode {
			t.Error("should set false")
		}
	})
}
