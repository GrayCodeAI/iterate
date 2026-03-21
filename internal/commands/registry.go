// Package commands provides a modular command registry for the iterate REPL.
// Each command group (session, git, safety, etc.) is in its own file.
package commands

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/agent"
)

// Context holds all state needed by commands.
type Context struct {
	RepoPath    string
	Line        string
	Parts       []string
	Writer      io.Writer
	Logger      *slog.Logger
	Provider    iteragent.Provider
	Agent       *iteragent.Agent
	Thinking    *iteragent.ThinkingLevel
	SafeMode    *bool
	DeniedTools map[string]bool

	// Session state
	SessionInputTokens  *int
	SessionOutputTokens *int
	SessionCacheRead    *int
	SessionCacheWrite   *int
	InputHistory        *[]string
	StopWatch           func()

	// Agent pool for /swarm
	Pool *agent.Pool

	// Session callbacks
	SaveSession   func(name string, msgs []iteragent.Message) error
	LoadSession   func(name string) ([]iteragent.Message, error)
	ListSessions  func() []string
	AddBookmark   func(name string, msgs []iteragent.Message)
	LoadBookmarks func() []Bookmark
	SelectItem    func(title string, items []string) (string, bool)
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

// Color helpers (reassignable for /theme support)
var (
	ColorReset  = "\033[0m"
	ColorLime   = "\033[38;5;154m"
	ColorYellow = "\033[38;5;220m"
	ColorDim    = "\033[2m"
	ColorBold   = "\033[1m"
	ColorCyan   = "\033[36m"
	ColorRed    = "\033[31m"
)

// Print helpers
func PrintSuccess(format string, args ...any) {
	fmt.Printf("%s✓ %s%s\n", ColorLime, fmt.Sprintf(format, args...), ColorReset)
}

func PrintError(format string, args ...any) {
	fmt.Printf("%serror: %s%s\n", ColorRed, fmt.Sprintf(format, args...), ColorReset)
}

func PrintDim(format string, args ...any) {
	fmt.Printf("%s%s%s\n", ColorDim, fmt.Sprintf(format, args...), ColorReset)
}

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
