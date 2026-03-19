package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type iterConfig struct {
	Provider      string   `json:"provider"       toml:"provider"`
	Model         string   `json:"model"          toml:"model"`
	OllamaBaseURL string   `json:"ollama_base_url,omitempty" toml:"ollama_base_url"`
	SafeMode      bool     `json:"safe_mode,omitempty"      toml:"safe_mode"`
	DeniedTools   []string `json:"denied_tools,omitempty"   toml:"denied_tools"`
	Theme         string   `json:"theme,omitempty"          toml:"theme"`
	Notify        bool     `json:"notify,omitempty"         toml:"notify"`
	// LLM generation parameters.
	Temperature   float64 `json:"temperature,omitempty"    toml:"temperature"`
	MaxTokens     int     `json:"max_tokens,omitempty"     toml:"max_tokens"`
	ThinkingLevel string  `json:"thinking_level,omitempty" toml:"thinking_level"`
	CacheEnabled  bool    `json:"cache_enabled,omitempty"  toml:"cache_enabled"`
	// Glob-based allow/deny patterns for bash commands.
	AllowPatterns []string `json:"allow_patterns,omitempty" toml:"allow_patterns"`
	DenyPatterns  []string `json:"deny_patterns,omitempty"  toml:"deny_patterns"`
	// Directory restrictions for file tools.
	AllowDirs []string `json:"allow_dirs,omitempty" toml:"allow_dirs"`
	DenyDirs  []string `json:"deny_dirs,omitempty"  toml:"deny_dirs"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "config.json")
}

// configPathAlt returns the XDG-style config path (~/.config/iterate/config.json).
func configPathAlt() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "iterate", "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "iterate", "config.json")
}

// configPathTOML returns the TOML config path (~/.config/iterate/config.toml).
func configPathTOML() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "iterate", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "iterate", "config.toml")
}

func loadConfig() iterConfig {
	var cfg iterConfig

	// Try TOML first (new preferred format), then JSON paths.
	if data, err := os.ReadFile(configPathTOML()); err == nil {
		toml.Unmarshal(data, &cfg) //nolint:errcheck
	} else {
		// Fall back to JSON: XDG path, then legacy ~/.iterate/config.json.
		for _, path := range []string{configPathAlt(), configPath()} {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if json.Unmarshal(data, &cfg) == nil {
				break
			}
		}
	}

	// Environment variable overrides (always win over file config).
	applyEnvOverrides(&cfg)
	return cfg
}

// applyEnvOverrides applies ITERATE_* environment variables on top of a config.
func applyEnvOverrides(cfg *iterConfig) {
	if v := os.Getenv("ITERATE_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("ITERATE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("ITERATE_THEME"); v != "" {
		cfg.Theme = v
	}
	if v := os.Getenv("ITERATE_THINKING_LEVEL"); v != "" {
		cfg.ThinkingLevel = v
	}
	if v := os.Getenv("ITERATE_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = f
		}
	}
	if v := os.Getenv("ITERATE_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxTokens = n
		}
	}
	if v := os.Getenv("ITERATE_SAFE_MODE"); v != "" {
		cfg.SafeMode = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("ITERATE_CACHE_ENABLED"); v != "" {
		cfg.CacheEnabled = v == "1" || strings.EqualFold(v, "true")
	}
}

func saveConfig(cfg iterConfig) {
	// Prefer TOML if the TOML config file already exists or the JSON path doesn't.
	tomlPath := configPathTOML()
	jsonPath := configPath()

	_, tomlExists := os.Stat(tomlPath)
	_, jsonExists := os.Stat(jsonPath)

	if tomlExists == nil || jsonExists != nil {
		// Write TOML.
		os.MkdirAll(filepath.Dir(tomlPath), 0o755)
		var buf bytes.Buffer
		if err := toml.NewEncoder(&buf).Encode(cfg); err == nil {
			os.WriteFile(tomlPath, buf.Bytes(), 0o644)
			return
		}
	}
	// Fall back to JSON.
	os.MkdirAll(filepath.Dir(jsonPath), 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(jsonPath, data, 0o644)
}

// ---------------------------------------------------------------------------
// Glob-based permission system for bash commands
// ---------------------------------------------------------------------------

// globMatch reports whether name matches the simple glob pattern.
// Supports * (any sequence) and ? (any single char). No ** support.
func globMatch(pattern, name string) bool {
	// filepath.Match is sufficient for our use case
	matched, _ := filepath.Match(pattern, name)
	return matched
}

// checkBashPermission checks allow/deny patterns against a bash command.
// Returns: allowed=true (auto-allow), denied=true (auto-deny), or neither (ask user).
func checkBashPermission(cfg iterConfig, cmd string) (allowed, denied bool) {
	for _, p := range cfg.DenyPatterns {
		if globMatch(p, cmd) || (len(cmd) >= len(p) && globMatch(p, cmd[:len(p)])) {
			return false, true
		}
		// Also check if any word in cmd matches the pattern
		for _, word := range splitShellWords(cmd) {
			if globMatch(p, word) {
				return false, true
			}
		}
	}
	for _, p := range cfg.AllowPatterns {
		if globMatch(p, cmd) {
			return true, false
		}
		for _, word := range splitShellWords(cmd) {
			if globMatch(p, word) {
				return true, false
			}
		}
	}
	return false, false
}

// splitShellWords naively splits a shell command into words (for glob matching).
func splitShellWords(cmd string) []string {
	var words []string
	inQuote := false
	var cur []byte
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if c == '"' || c == '\'' {
			inQuote = !inQuote
			continue
		}
		if c == ' ' && !inQuote {
			if len(cur) > 0 {
				words = append(words, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}

// ---------------------------------------------------------------------------
// Directory restriction helpers for file tools
// ---------------------------------------------------------------------------

// checkDirPermission checks AllowDirs/DenyDirs against a file path.
// Returns denied=true if the path is blocked; allowed=false means "ask user".
// When AllowDirs is non-empty the path must be under at least one allowed dir.
// DenyDirs always wins over AllowDirs.
func checkDirPermission(cfg iterConfig, filePath string) (denied bool) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}
	// DenyDirs: block if path is under any denied directory.
	for _, d := range cfg.DenyDirs {
		dAbs, _ := filepath.Abs(d)
		rel, err := filepath.Rel(dAbs, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	// AllowDirs: if set, path must be under at least one allowed dir.
	if len(cfg.AllowDirs) > 0 {
		for _, d := range cfg.AllowDirs {
			dAbs, _ := filepath.Abs(d)
			rel, err := filepath.Rel(dAbs, abs)
			if err == nil && !strings.HasPrefix(rel, "..") {
				return false
			}
		}
		return true // not under any allowed dir
	}
	return false
}

// ---------------------------------------------------------------------------
// Session changes tracker
// ---------------------------------------------------------------------------

type sessionChangesTracker struct {
	written []string
	edited  []string
}

var sessionChanges sessionChangesTracker

func (s *sessionChangesTracker) recordWrite(path string) {
	for _, p := range s.written {
		if p == path {
			return
		}
	}
	s.written = append(s.written, path)
}

func (s *sessionChangesTracker) recordEdit(path string) {
	for _, p := range s.edited {
		if p == path {
			return
		}
	}
	s.edited = append(s.edited, path)
}

func (s *sessionChangesTracker) format() string {
	if len(s.written) == 0 && len(s.edited) == 0 {
		return "No files changed this session."
	}
	var lines []string
	for _, p := range s.written {
		lines = append(lines, colorLime+"  + "+colorReset+p)
	}
	for _, p := range s.edited {
		lines = append(lines, colorYellow+"  ~ "+colorReset+p)
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// In-memory conversation marks (distinct from disk bookmarks)
// ---------------------------------------------------------------------------

// conversationMarks maps mark name → message index at time of marking.
var conversationMarks = map[string]int{}
