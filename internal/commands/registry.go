// Package commands provides a modular command registry for the iterate REPL.
// Each command group (session, git, safety, etc.) is in its own file.
package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
	"github.com/GrayCodeAI/iterate/internal/ui"
)

// SessionCallbacks groups session management callbacks.
type SessionCallbacks struct {
	SaveSession   func(name string, msgs []iteragent.Message) error
	LoadSession   func(name string) ([]iteragent.Message, error)
	ListSessions  func() []string
	AddBookmark   func(name string, msgs []iteragent.Message)
	LoadBookmarks func() []Bookmark
	SelectItem    func(title string, items []string) (string, bool)
}

// ConfigCallbacks groups configuration and alias callbacks.
type ConfigCallbacks struct {
	LoadConfig     func() interface{}
	SaveConfig     func(cfg interface{})
	ConfigPath     func() string
	HistoryFile    *string
	LoadAliases    func() map[string]string
	SaveAliases    func(m map[string]string)
	LoadMCPServers func() []MCPServerEntry
	SaveMCPServers func(servers []MCPServerEntry)
}

// REPLCallbacks groups REPL interaction callbacks.
type REPLCallbacks struct {
	StreamAndPrint func(ctx context.Context, a *iteragent.Agent, prompt, repoPath string)
	RunShell       func(repoPath string, name string, args ...string)
	MakeAgent      func() *iteragent.Agent
	ReadMultiLine  func() (string, bool)
	PromptLine     func(prompt string) (string, bool)
	// Undo reverts the last agent file modifications.
	// Returns the list of restored paths and an error (if any).
	Undo func() ([]string, error)
	// BuildRepoMap returns a structural summary of the repository.
	// refresh=true forces a rebuild; false may return a cached result.
	BuildRepoMap func(repoPath string, refresh bool) string
	// InvalidateRepoMap clears the repo map cache.
	InvalidateRepoMap func()
}

// StateAccessors groups thread-safe state access callbacks.
type StateAccessors struct {
	IsDenied             func(name string) bool
	DenyTool             func(name string)
	AllowTool            func(name string)
	GetDeniedList        func() []string
	GetPinnedMessages    func() []iteragent.Message
	SetPinnedMessages    func(msgs []iteragent.Message)
	GetConversationMark  func(name string) (int, bool)
	SetConversationMark  func(name string, idx int)
	GetConversationMarks func() map[string]int
	ConversationMarksLen func() int
}

// TemplateCallbacks groups template management callbacks.
type TemplateCallbacks struct {
	LoadTemplates        func() []PromptTemplate
	AddTemplate          func(name, prompt string)
	FormatSessionChanges func() string
}

// SnapshotCallbacks groups snapshot management callbacks.
type SnapshotCallbacks struct {
	SaveSnapshot  func(name string, msgs []iteragent.Message) error
	ListSnapshots func() []Snapshot
}

// Context holds all state needed by commands.
type Context struct {
	RepoPath    string
	Line        string
	Parts       []string
	Version     string
	Writer      io.Writer
	Logger      *slog.Logger
	Provider    iteragent.Provider
	Agent       *iteragent.Agent
	Thinking    *iteragent.ThinkingLevel
	SafeMode    *bool
	DeniedTools map[string]bool

	// Registry is the command registry — used by /help to generate dynamic output.
	Registry *Registry

	// Session state
	SessionInputTokens  *int
	SessionOutputTokens *int
	SessionCacheRead    *int
	SessionCacheWrite   *int
	InputHistory        *[]string
	StopWatch           func()

	// Agent pool for /swarm
	Pool *agent.Pool

	// Agent state
	LastPrompt   *string
	LastResponse *string
	CurrentMode  *int
	DebugMode    *bool

	// Auto-commit state
	AutoCommitEnabled *bool

	// Runtime config
	RuntimeConfig *RuntimeConfig

	// PersistConfig saves the current live safety state (safe_mode, denied_tools) to the config file.
	// Wired by the REPL; nil-safe — commands should check before calling.
	PersistConfig func()

	// Theme
	Themes     map[string]interface{}
	ApplyTheme func(name string)

	// Notification
	NotifyEnabled *bool

	// Session timing/stats
	SessionStart     *time.Time
	SessionToolCalls *int
	SessionMessages  *int

	// Budget tracking
	BudgetLimit    *float64
	SessionCostUSD *float64

	// Spinner state
	IsSpinnerActive *atomic.Int32

	// Conversation marks
	ConversationMarks *map[string]int

	// Pinned messages
	PinnedMessages *[]iteragent.Message

	// Context window size
	ContextWindow *int

	// Watch callbacks
	StartWatch func(repoPath string)

	// Grouped callbacks
	Session   SessionCallbacks
	Config    ConfigCallbacks
	REPL      REPLCallbacks
	State     StateAccessors
	Templates TemplateCallbacks
	Snapshots SnapshotCallbacks
}

// RuntimeConfig holds runtime settings for the agent.
type RuntimeConfig struct {
	Temperature  *float32
	MaxTokens    *int
	CacheEnabled *bool
}

// PromptTemplate represents a saved prompt template.
type PromptTemplate struct {
	Name    string
	Prompt  string
	Created time.Time
}

// Snapshot represents a saved conversation snapshot.
type Snapshot struct {
	Name      string
	CreatedAt time.Time
	Messages  []iteragent.Message
}

// MCPServerEntry represents an MCP server configuration.
type MCPServerEntry struct {
	Name    string
	URL     string
	Command string
	Args    []string
}

// Bookmark represents a saved conversation state.
type Bookmark struct {
	Name      string
	CreatedAt time.Time
	Messages  []iteragent.Message
}

// Result represents the outcome of a command execution.
type Result struct {
	Done    bool  // true = exit REPL
	Handled bool  // true = command was recognized
	Err     error // non-nil = error occurred
}

// Command represents a single REPL command.
type Command struct {
	Name        string   // primary name (e.g., "/save")
	Aliases     []string // aliases (e.g., "/exit", "/q" for "/quit")
	Description string   // short help text
	Category    string   // grouping for help display
	Handler     func(Context) Result
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = &cmd
	for _, alias := range cmd.Aliases {
		r.commands[alias] = &cmd
	}
}

// Lookup finds a command by name or alias.
func (r *Registry) Lookup(name string) (*Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// All returns all unique commands (by primary name only).
func (r *Registry) All() []*Command {
	seen := make(map[string]bool)
	var result []*Command
	for _, cmd := range r.commands {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true
			result = append(result, cmd)
		}
	}
	return result
}

// ByCategory returns commands grouped by category.
func (r *Registry) ByCategory() map[string][]*Command {
	cats := make(map[string][]*Command)
	for _, cmd := range r.All() {
		cats[cmd.Category] = append(cats[cmd.Category], cmd)
	}
	return cats
}

// Execute runs a command by name with the given context.
func (r *Registry) Execute(name string, ctx Context) Result {
	cmd, ok := r.Lookup(name)
	if !ok {
		return Result{Handled: false}
	}
	return cmd.Handler(ctx)
}

// Color helpers — delegated to internal/ui (reassignable for /theme support)
var (
	ColorReset  = ui.ColorReset
	ColorLime   = ui.ColorLime
	ColorYellow = ui.ColorYellow
	ColorDim    = ui.ColorDim
	ColorBold   = ui.ColorBold
	ColorCyan   = ui.ColorCyan
	ColorRed    = ui.ColorRed
)

// PrintSuccess prints a success message — delegates to ui.PrintSuccess.
var PrintSuccess = ui.PrintSuccess

// PrintError prints an error message — delegates to ui.PrintError.
var PrintError = ui.PrintError

// PrintDim prints a dimmed message — delegates to ui.PrintDim.
var PrintDim = ui.PrintDim

// Stdout is the default writer for commands.
var Stdout io.Writer = os.Stdout

// Write formats to the context writer (or stdout).
func (ctx Context) Write(format string, args ...any) {
	w := ctx.Writer
	if w == nil {
		w = Stdout
	}
	fmt.Fprintf(w, format, args...)
}

// WriteLn writes a line to the context writer.
func (ctx Context) WriteLn(format string, args ...any) {
	ctx.Write(format+"\n", args...)
}

// Arg returns the nth argument (1-indexed, after command name).
func (ctx Context) Arg(n int) string {
	if len(ctx.Parts) > n {
		return ctx.Parts[n]
	}
	return ""
}

// Args returns all arguments after the command name.
func (ctx Context) Args() string {
	if len(ctx.Parts) > 1 {
		return strings.Join(ctx.Parts[1:], " ")
	}
	return ""
}

// HasArg returns true if there are at least n arguments after command.
func (ctx Context) HasArg(n int) bool {
	return len(ctx.Parts) > n
}
