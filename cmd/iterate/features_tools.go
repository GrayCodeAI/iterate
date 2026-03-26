package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
	"github.com/GrayCodeAI/iterate/internal/ui/selector"
)

// ---------------------------------------------------------------------------
// Permission / safe mode
// ---------------------------------------------------------------------------

// spinnerActive is set to 1 while the spinner goroutine is printing.
var spinnerActive atomic.Int32

// spinnerQuiet is closed by the spinner goroutine when it finishes clearing
// the terminal line. Safe-mode prompts wait on this instead of busy-looping.
var spinnerQuiet = make(chan struct{})

func init() {
	// Start closed so the first prompt doesn't block if no spinner ever ran.
	close(spinnerQuiet)
}

// notifySpinnerQuiet replaces spinnerQuiet with a new open channel, then
// closes the old one so any waiters unblock. Called by the spinner when done.
func notifySpinnerQuiet() {
	old := spinnerQuiet
	spinnerQuiet = make(chan struct{})
	select {
	case <-old:
	default:
		close(old)
	}
}

// waitForSpinner blocks until the spinner has stopped and cleared the line,
// or until 500 ms have elapsed (so a stuck spinner never deadlocks a prompt).
func waitForSpinner() {
	select {
	case <-spinnerQuiet:
	case <-time.After(500 * time.Millisecond):
	}
}

// streamingTokenCount is incremented for each token received during streaming.
// The spinner reads this to display tok/s.
var streamingTokenCount atomic.Int64

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

// ---------------------------------------------------------------------------
// Undo stack — snapshots of file contents before agent writes
// ---------------------------------------------------------------------------

// undoSnapshot records the state of a single file before an agent modification.
type undoSnapshot struct {
	Path    string
	Content []byte // nil means the file did not exist (new file created)
}

// undoFrame is one agent turn's worth of file modifications.
type undoFrame []undoSnapshot

var (
	undoStack   []undoFrame
	undoStackMu sync.Mutex
	// currentFrame accumulates snapshots for the in-progress turn.
	currentFrame undoFrame
)

// beginUndoFrame starts a new undo frame for the current agent turn.
func beginUndoFrame() {
	undoStackMu.Lock()
	defer undoStackMu.Unlock()
	currentFrame = undoFrame{}
}

// commitUndoFrame pushes the current frame (if non-empty) onto the stack.
func commitUndoFrame() {
	undoStackMu.Lock()
	defer undoStackMu.Unlock()
	if len(currentFrame) > 0 {
		undoStack = append(undoStack, currentFrame)
		currentFrame = nil
	}
}

// captureFileSnapshot saves the current content of path into the active frame.
// Called just before a write/edit tool overwrites the file.
func captureFileSnapshot(path string) {
	undoStackMu.Lock()
	defer undoStackMu.Unlock()
	// Only capture once per path per frame.
	for _, s := range currentFrame {
		if s.Path == path {
			return
		}
	}
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		currentFrame = append(currentFrame, undoSnapshot{Path: path, Content: nil})
	} else if err == nil {
		currentFrame = append(currentFrame, undoSnapshot{Path: path, Content: content})
	}
}

// performUndo restores the most recent undo frame.
// Returns the list of restored paths, or an error.
func performUndo() ([]string, error) {
	undoStackMu.Lock()
	defer undoStackMu.Unlock()
	if len(undoStack) == 0 {
		return nil, fmt.Errorf("nothing to undo")
	}
	frame := undoStack[len(undoStack)-1]
	undoStack = undoStack[:len(undoStack)-1]

	var restored []string
	var errs []string
	for _, snap := range frame {
		if snap.Content == nil {
			// File was newly created — remove it.
			if err := os.Remove(snap.Path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("remove %s: %v", snap.Path, err))
				continue
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(snap.Path), 0o755); err != nil {
				errs = append(errs, fmt.Sprintf("mkdir %s: %v", snap.Path, err))
				continue
			}
			if err := os.WriteFile(snap.Path, snap.Content, 0o644); err != nil {
				errs = append(errs, fmt.Sprintf("restore %s: %v", snap.Path, err))
				continue
			}
		}
		restored = append(restored, snap.Path)
	}
	if len(errs) > 0 {
		return restored, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return restored, nil
}

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

			trackSessionChanges(t.Name, args)

			// Capture file snapshot before any write/edit so /undo can restore.
			if t.Name == "write_file" || t.Name == "edit_file" || t.Name == "create_file" {
				if p, ok := args["path"]; ok {
					captureFileSnapshot(p)
				}
			}

			if denied := checkToolDirPermission(cfg, t.Name, args); denied != "" {
				logAudit(t.Name, auditArgs, "DENIED (dir restriction)")
				return denied, nil
			}

			if cfg.SafeMode && isDenied(t.Name) {
				if result, handled := handleSafeModePrompt(cfg, t, args, origExec, auditArgs); handled {
					return result, nil
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

func trackSessionChanges(toolName string, args map[string]string) {
	if toolName == "write_file" {
		if p, ok := args["path"]; ok {
			sessionChanges.recordWrite(p)
		}
	}
	if toolName == "edit_file" {
		if p, ok := args["path"]; ok {
			sessionChanges.recordEdit(p)
		}
	}
}

func checkToolDirPermission(cfg iterConfig, toolName string, args map[string]string) string {
	if toolName == "write_file" || toolName == "edit_file" || toolName == "read_file" {
		if p, ok := args["path"]; ok {
			if checkDirPermission(cfg, p) {
				return fmt.Sprintf("Access denied: %s is outside allowed directories.", p)
			}
		}
	}
	return ""
}

func handleSafeModePrompt(cfg iterConfig, tool iteragent.Tool, args map[string]string, origExec func(context.Context, map[string]string) (string, error), auditArgs map[string]interface{}) (string, bool) {
	if tool.Name == "bash" {
		cmd := args["command"]
		if allowed, denied := checkBashPermission(cfg, cmd); allowed {
			result, err := origExec(context.Background(), args)
			logAudit(tool.Name, auditArgs, result)
			if err != nil {
				return err.Error(), true
			}
			return result, true
		} else if denied {
			logAudit(tool.Name, auditArgs, "DENIED (pattern)")
			return "Command blocked by deny pattern.", true
		}
	}

	waitForSpinner()
	fmt.Printf("\n%s⚠ Safe mode: allow %s?%s ", colorYellow, tool.Name, colorReset)
	answer, ok := selector.PromptLine("(y/N/always):")
	if !ok {
		logAudit(tool.Name, auditArgs, "DENIED")
		return "Tool execution denied by user (safe mode).", true
	}
	ans := strings.ToLower(strings.TrimSpace(answer))
	if ans == "always" {
		if tool.Name == "bash" {
			if cmd, ok := args["command"]; ok {
				cfg.AllowPatterns = append(cfg.AllowPatterns, cmd)
			}
		} else {
			allowTool(tool.Name)
		}
	} else if ans != "y" {
		logAudit(tool.Name, auditArgs, "DENIED")
		return "Tool execution denied by user (safe mode).", true
	}
	return "", false
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
	colorMu.Lock()
	defer colorMu.Unlock()
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
