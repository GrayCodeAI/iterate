package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type iterConfig struct {
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	OllamaBaseURL string   `json:"ollama_base_url,omitempty"`
	SafeMode      bool     `json:"safe_mode,omitempty"`
	DeniedTools   []string `json:"denied_tools,omitempty"`
	Theme         string   `json:"theme,omitempty"`
	Notify        bool     `json:"notify,omitempty"`
	// Glob-based allow/deny patterns for bash commands.
	AllowPatterns []string `json:"allow_patterns,omitempty"`
	DenyPatterns  []string `json:"deny_patterns,omitempty"`
	// Directory restrictions for file tools.
	AllowDirs []string `json:"allow_dirs,omitempty"`
	DenyDirs  []string `json:"deny_dirs,omitempty"`
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

func loadConfig() iterConfig {
	// Try XDG path first, then legacy ~/.iterate/config.json
	for _, path := range []string{configPathAlt(), configPath()} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg iterConfig
		if json.Unmarshal(data, &cfg) == nil {
			return cfg
		}
	}
	return iterConfig{}
}

func saveConfig(cfg iterConfig) {
	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0o644)
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

