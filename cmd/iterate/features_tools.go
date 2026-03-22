package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
)

// ---------------------------------------------------------------------------
// Permission / safe mode
// ---------------------------------------------------------------------------

// spinnerActive is set to 1 while the spinner goroutine is printing.
// Tool wrappers wait for it to reach 0 before showing a prompt.
var spinnerActive atomic.Int32

// deniedTools is the set of tools blocked in safe mode.
var deniedTools = map[string]bool{
	"bash":       true,
	"write_file": true,
	"edit_file":  true,
}

var deniedToolsMu sync.RWMutex

func isDenied(name string) bool {
	deniedToolsMu.RLock()
	defer deniedToolsMu.RUnlock()
	return deniedTools[name]
}

func denyTool(name string) {
	deniedToolsMu.Lock()
	defer deniedToolsMu.Unlock()
	deniedTools[name] = true
}

func allowTool(name string) {
	deniedToolsMu.Lock()
	defer deniedToolsMu.Unlock()
	delete(deniedTools, name)
}

func getDeniedList() []string {
	deniedToolsMu.RLock()
	defer deniedToolsMu.RUnlock()
	out := make([]string, 0, len(deniedTools))
	for t := range deniedTools {
		out = append(out, t)
	}
	return out
}

// agentPool is the shared agent pool for /swarm command.
var agentPool *agent.Pool

// wrapToolsWithPermissions wraps tools that need approval in safe mode
// and adds audit logging to all tools.
func wrapToolsWithPermissions(tools []iteragent.Tool) []iteragent.Tool {
	cfg := loadConfig()
	out := make([]iteragent.Tool, len(tools))
	for i, t := range tools {
		t := t // capture
		origExec := t.Execute
		t.Execute = func(ctx context.Context, args map[string]string) (string, error) {
			auditArgs := make(map[string]interface{}, len(args))
			for k, v := range args {
				auditArgs[k] = v
			}

			// Track file changes for /changes command.
			if t.Name == "write_file" {
				if p, ok := args["path"]; ok {
					sessionChanges.recordWrite(p)
				}
			}
			if t.Name == "edit_file" {
				if p, ok := args["path"]; ok {
					sessionChanges.recordEdit(p)
				}
			}

			// Directory restrictions: check AllowDirs/DenyDirs for file tools.
			if t.Name == "write_file" || t.Name == "edit_file" || t.Name == "read_file" {
				if p, ok := args["path"]; ok {
					if checkDirPermission(cfg, p) {
						msg := fmt.Sprintf("Access denied: %s is outside allowed directories.", p)
						logAudit(t.Name, auditArgs, "DENIED (dir restriction)")
						return msg, nil
					}
				}
			}

			if cfg.SafeMode && isDenied(t.Name) {
				// Glob-based auto-allow/deny for bash commands.
				if t.Name == "bash" {
					cmd := args["command"]
					if allowed, denied := checkBashPermission(cfg, cmd); allowed {
						result, err := origExec(ctx, args)
						logAudit(t.Name, auditArgs, result)
						return result, err
					} else if denied {
						logAudit(t.Name, auditArgs, "DENIED (pattern)")
						return "Command blocked by deny pattern.", nil
					}
				}

				// Wait briefly for spinner to stop.
				for spinnerActive.Load() == 1 {
					time.Sleep(5 * time.Millisecond)
				}
				fmt.Printf("\n%s⚠ Safe mode: allow %s?%s ", colorYellow, t.Name, colorReset)
				answer, ok := promptLine("(y/N/always):")
				if !ok {
					logAudit(t.Name, auditArgs, "DENIED")
					return "Tool execution denied by user (safe mode).", nil
				}
				ans := strings.ToLower(strings.TrimSpace(answer))
				if ans == "always" {
					// Add to allow patterns for this session
					if t.Name == "bash" {
						if cmd, ok := args["command"]; ok {
							cfg.AllowPatterns = append(cfg.AllowPatterns, cmd)
						}
					} else {
						allowTool(t.Name)
					}
				} else if ans != "y" {
					logAudit(t.Name, auditArgs, "DENIED")
					return "Tool execution denied by user (safe mode).", nil
				}
			}

			result, err := origExec(ctx, args)
			logAudit(t.Name, auditArgs, result)
			return result, err
		}
		out[i] = t
	}
	return out
}

// ---------------------------------------------------------------------------
// Agent mode — read-only /ask mode (bash and write tools disabled)
// ---------------------------------------------------------------------------

type agentMode int

const (
	modeNormal    agentMode = iota
	modeAsk                 // read-only: no bash, no write_file, no edit_file
	modeArchitect           // planning only: no tools at all
)

var currentMode agentMode

// readOnlyTools filters out destructive tools for /ask mode.
func readOnlyTools(tools []iteragent.Tool) []iteragent.Tool {
	blocked := map[string]bool{
		"bash": true, "write_file": true, "edit_file": true,
		"git_commit": true, "git_revert": true, "run_tests": true,
	}
	var out []iteragent.Tool
	for _, t := range tools {
		if !blocked[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// /theme — color theme switching
// ---------------------------------------------------------------------------

type theme struct {
	name   string
	lime   string
	yellow string
	cyan   string
	purple string
	dim    string
	bold   string
	red    string
	green  string
	amber  string
	blue   string
	reset  string
}

var themes = map[string]theme{
	"default": {
		name: "default", lime: "\033[38;5;154m", yellow: "\033[38;5;220m",
		cyan: "\033[36m", purple: "\033[38;5;141m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[31m", green: "\033[38;5;114m",
		amber: "\033[38;5;221m", blue: "\033[38;5;75m", reset: "\033[0m",
	},
	"nord": {
		name: "nord", lime: "\033[38;5;109m", yellow: "\033[38;5;222m",
		cyan: "\033[38;5;110m", purple: "\033[38;5;146m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[38;5;174m", green: "\033[38;5;108m",
		amber: "\033[38;5;179m", blue: "\033[38;5;67m", reset: "\033[0m",
	},
	"monokai": {
		name: "monokai", lime: "\033[38;5;148m", yellow: "\033[38;5;227m",
		cyan: "\033[38;5;81m", purple: "\033[38;5;141m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[38;5;197m", green: "\033[38;5;148m",
		amber: "\033[38;5;215m", blue: "\033[38;5;81m", reset: "\033[0m",
	},
	"minimal": {
		name: "minimal", lime: "\033[32m", yellow: "\033[33m",
		cyan: "\033[36m", purple: "\033[35m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[31m", green: "\033[32m",
		amber: "\033[33m", blue: "\033[34m", reset: "\033[0m",
	},
}

func applyTheme(t theme) {
	colorLime = t.lime
	colorYellow = t.yellow
	colorCyan = t.cyan
	colorPurple = t.purple
	colorDim = t.dim
	colorBold = t.bold
	colorRed = t.red
	colorGreen = t.green
	colorAmber = t.amber
	colorBlue = t.blue
	colorReset = t.reset
}

// ---------------------------------------------------------------------------
// /alias — persistent command shortcuts
// ---------------------------------------------------------------------------

type aliasMap map[string]string

func aliasesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "aliases.json")
}

func loadAliases() aliasMap {
	data, err := os.ReadFile(aliasesPath())
	if err != nil {
		return aliasMap{}
	}
	var m aliasMap
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to parse aliases: %v\n", err)
		return aliasMap{}
	}
	if m == nil {
		return aliasMap{}
	}
	return m
}

func saveAliases(m aliasMap) {
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.MkdirAll(filepath.Dir(aliasesPath()), 0o755); err != nil {
		slog.Warn("failed to create aliases dir", "err", err)
		return
	}
	if err := os.WriteFile(aliasesPath(), data, 0o644); err != nil {
		slog.Warn("failed to write aliases file", "err", err)
	}
}

// resolveAlias expands an alias if one exists, otherwise returns line unchanged.
func resolveAlias(line string) string {
	aliases := loadAliases()
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return line
	}
	if expanded, ok := aliases[parts[0]]; ok {
		if len(parts) > 1 {
			return expanded + " " + strings.Join(parts[1:], " ")
		}
		return expanded
	}
	return line
}

// ---------------------------------------------------------------------------
// /mcp — MCP server config management
// ---------------------------------------------------------------------------

type mcpServer struct {
	Name    string   `json:"name"`
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

func mcpConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "mcp.json")
}

func loadMCPServers() []mcpServer {
	data, err := os.ReadFile(mcpConfigPath())
	if err != nil {
		return nil
	}
	var servers []mcpServer
	if err := json.Unmarshal(data, &servers); err != nil {
		slog.Warn("failed to parse mcp servers", "err", err)
	}
	return servers
}

func saveMCPServers(servers []mcpServer) {
	data, _ := json.MarshalIndent(servers, "", "  ")
	if err := os.MkdirAll(filepath.Dir(mcpConfigPath()), 0o755); err != nil {
		slog.Warn("failed to create mcp config dir", "err", err)
		return
	}
	if err := os.WriteFile(mcpConfigPath(), data, 0o644); err != nil {
		slog.Warn("failed to write mcp config file", "err", err)
	}
}

// ---------------------------------------------------------------------------
// Visual context bar
// ---------------------------------------------------------------------------

func contextBar(messages []iteragent.Message, windowSize int) string {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	tokens := totalChars / 4
	pct := float64(tokens) / float64(windowSize) * 100
	if pct > 100 {
		pct = 100
	}
	barWidth := 40
	filled := int(float64(barWidth) * pct / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	color := colorGreen
	if pct > 75 {
		color = colorYellow
	}
	if pct > 90 {
		color = colorRed
	}
	return fmt.Sprintf("%s%s%s %.0f%%  ~%d / %d tokens  %d msgs",
		color, bar, colorReset, pct, tokens, windowSize, len(messages))
}

// ---------------------------------------------------------------------------
// /set — runtime config (temperature, max_tokens)
// ---------------------------------------------------------------------------

type runtimeConfig struct {
	Temperature  *float32
	MaxTokens    *int
	CacheEnabled *bool
}

var rtConfig runtimeConfig

// ---------------------------------------------------------------------------
// /pin — pin messages so they survive compaction
// ---------------------------------------------------------------------------

// pinnedMessages are always prepended when the agent runs after compaction.
var pinnedMessages []iteragent.Message
var pinnedMessagesMu sync.RWMutex

func getPinnedMessages() []iteragent.Message {
	pinnedMessagesMu.RLock()
	defer pinnedMessagesMu.RUnlock()
	dst := make([]iteragent.Message, len(pinnedMessages))
	copy(dst, pinnedMessages)
	return dst
}

func setPinnedMessages(msgs []iteragent.Message) {
	pinnedMessagesMu.Lock()
	defer pinnedMessagesMu.Unlock()
	pinnedMessages = msgs
}
