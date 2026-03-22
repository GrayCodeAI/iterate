# Design Document: Code Reorganization

## Overview

The `iterate` codebase has grown organically to 50 files in `cmd/iterate/` and several `internal/` packages with blurred boundaries. This reorganization applies Go project layout best practices: each package has a single coherent responsibility, dependencies flow inward (UI → business logic → data), and the binary entry point is minimal.

The migration is purely structural — no behavior changes, no API changes, no flag renames. Every existing test must pass before and after.

### Goals

- `cmd/iterate/` shrinks to ≤ 5 files (entry point + flag parsing + mode dispatch)
- All reusable logic moves to `internal/` packages with clear names
- GitHub client construction is consolidated in one place
- Terminal UI concerns are grouped under `internal/ui`
- `internal/agent` is either given a clear purpose or inlined

### Non-Goals

- Changing any CLI flag, REPL command, or evolution phase name
- Changing any public API exposed by `internal/` packages to callers outside this repo
- Rewriting logic — only moving and re-packaging

---

## Architecture

### Dependency Flow

```
cmd/iterate/          ← entry point only
    ↓
internal/repl/        ← REPL loop, streaming, agent wiring
    ↓
internal/commands/    ← slash-command registry (existing, unchanged)
    ↓
internal/ui/          ← highlight, selector, colors (no upward imports)
internal/config/      ← config load/save (no upward imports)
internal/session/     ← session/bookmark persistence
internal/pricing/     ← token cost calculation
internal/provider/    ← provider init helpers
internal/memory/      ← project memory + evolution learnings
internal/github/      ← unified GitHub client + issues + discussions
    ↓
internal/evolution/   ← evolution engine (existing, unchanged)
internal/social/      ← social engine (imports internal/github)
internal/agentpool/   ← pool + mutation tools (renamed from internal/agent)
internal/util/        ← shared utilities (existing, unchanged)
```

### Package Dependency Rules

1. `internal/ui` MUST NOT import any other `internal/` package
2. `internal/config` MUST NOT import `internal/repl` or `internal/commands`
3. `internal/github` MUST NOT import `internal/social` or `internal/community`
4. `internal/repl` MAY import `internal/ui`, `internal/config`, `internal/session`, `internal/commands`, `internal/pricing`, `internal/provider`, `internal/memory`, `internal/agentpool`
5. `cmd/iterate` imports only `internal/repl`, `internal/config`, `internal/provider`, `internal/evolution`, `internal/social`, `internal/github`

### Migration Sequence

The migration proceeds package-by-package, with `go build ./...` verified after each move:

1. `internal/ui` (no dependencies on other internal packages — safe first)
2. `internal/config` (depends only on stdlib + BurntSushi/toml)
3. `internal/pricing` (pure computation, no dependencies)
4. `internal/provider` (depends on iteragent + internal/config)
5. `internal/session` (depends on iteragent)
6. `internal/memory` (depends on stdlib only)
7. `internal/github` (consolidates community + social GitHub client)
8. `internal/agentpool` (rename from internal/agent, retain pool + mutation)
9. `internal/repl` (depends on all of the above + internal/commands)
10. `cmd/iterate` cleanup (remove moved files, update imports)

---

## Components and Interfaces

### `cmd/iterate` (after reorganization)

Target: ≤ 5 files.

| File | Responsibility |
|------|---------------|
| `main.go` | `main()`, `incrementDayCount()`, `saveSessionToFile()`, `loadSessionFromFile()` |
| `main_flags.go` | `parseFlags()`, `setupLogging()` — unchanged |
| `main_mode.go` | `runMode()`, `runSocialMode()`, `runEvolutionMode()`, `fetchCommunityIssues()`, etc. — unchanged logic, updated imports |

Files removed from `cmd/iterate/` (moved to `internal/`):
`repl.go`, `repl_helpers.go`, `repl_streaming.go`, `repl_models.go`, `state.go`,
`selector.go`, `selector_input.go`, `selector_history.go`,
`highlight.go`, `pricing.go`, `config.go`, `provider.go`,
`features_sessions.go`, `memory_project.go`,
`features.go`, `features_git_helpers.go`, `features_prompts.go`,
`features_search.go`, `features_shell.go`, `features_tools.go`, `features_watch.go`,
`commands_git.go`, `commands_project.go`

---

### `internal/repl`

The REPL loop, agent construction, streaming output, and command dispatch wiring.

```go
// Package repl implements the interactive REPL for iterate.
package repl

// Run starts an interactive REPL session.
func Run(ctx context.Context, p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger)

// MakeAgent constructs a configured iteragent.Agent for the given mode.
func MakeAgent(p iteragent.Provider, repoPath string, thinking iteragent.ThinkingLevel, logger *slog.Logger) *iteragent.Agent

// StreamAndPrint sends a prompt to the agent and streams the response to stdout.
func StreamAndPrint(ctx context.Context, a *iteragent.Agent, prompt, repoPath string)
```

Internal files:
- `repl.go` — main loop, `Run()`, `handleCommand()`, `buildCommandContext()`
- `repl_helpers.go` — `printHeader()`, `printSessionSummary()`, `printStatusLine()`
- `repl_streaming.go` — `StreamAndPrint()`, event loop
- `repl_models.go` — `selectModel()`, model list
- `state.go` — `sessionState`, `replConfig`, package-level vars

---

### `internal/ui`

All terminal presentation concerns. No imports from other `internal/` packages.

```go
// Package ui provides terminal rendering helpers for iterate.
package ui

// Sub-packages:
//   internal/ui/highlight  — syntax highlighting
//   internal/ui/selector   — fuzzy-search / arrow-key selector

// highlight package
func RenderResponse(text string)
func HighlightCode(line, lang string) string

// selector package
func SelectItem(title string, items []string) (string, bool)
func TabComplete(partial string) string
func ReadInput(historyFile string) (string, bool)
```

Color constants and print helpers currently in `internal/commands/registry.go` move here:

```go
// Package ui
var (
    ColorReset  = "\033[0m"
    ColorLime   = "\033[38;5;154m"
    ColorYellow = "\033[38;5;220m"
    ColorDim    = "\033[2m"
    ColorBold   = "\033[1m"
    ColorCyan   = "\033[36m"
    ColorRed    = "\033[31m"
)

func PrintSuccess(format string, args ...any)
func PrintError(format string, args ...any)
func PrintDim(format string, args ...any)
```

`internal/commands/registry.go` will import `internal/ui` for these symbols.

---

### `internal/config`

Config load/save, env overrides, glob permission helpers.

```go
// Package config handles iterate configuration loading and persistence.
package config

type Config struct { /* same fields as iterConfig */ }

func Load() Config
func Save(cfg Config)
func Path() string
func ApplyEnvOverrides(cfg *Config)
func CheckBashPermission(cfg Config, cmd string) (allowed, denied bool)
func CheckDirPermission(cfg Config, filePath string) (denied bool)
```

---

### `internal/pricing`

Pure token-cost calculation. No I/O, no dependencies beyond stdlib.

```go
// Package pricing calculates LLM token costs.
package pricing

type ModelPricing struct { /* unchanged */ }
type CostEstimate struct { /* unchanged */ }

func LookupPricing(model string) (ModelPricing, bool)
func EstimateCost(inputTokens, outputTokens, cacheWrite, cacheRead int, model string) CostEstimate
func FormatCostTable(inputTokens, outputTokens, cacheWrite, cacheRead int, model string) string
```

---

### `internal/provider`

Provider initialization and resolution helpers.

```go
// Package provider handles LLM provider initialization for iterate.
package provider

func ResolveConfig(flagProvider, flagModel string, cfg config.Config) (provider, model string)
func ResolveThinkingLevel(flagThinking string, cfg config.Config) string
func Init(providerName, apiKey string, logger *slog.Logger) (iteragent.Provider, error)
```

---

### `internal/session`

Session and bookmark persistence (currently `features_sessions.go`).

```go
// Package session manages REPL conversation persistence.
package session

func Save(name string, messages []iteragent.Message) error
func Load(name string) ([]iteragent.Message, error)
func List() []string
func Dir() string

type Bookmark struct { /* unchanged */ }
func LoadBookmarks() []Bookmark
func SaveBookmarks(bms []Bookmark)
func AddBookmark(name string, messages []iteragent.Message)

func InitAuditLog()
func LogAudit(toolName string, args map[string]interface{}, result string)
```

---

### `internal/memory`

Per-project memory (`.iterate/memory.json`) and evolution learnings helpers.

```go
// Package memory manages project-scoped and evolution memory for iterate.
package memory

type ProjectEntry struct { /* unchanged */ }
type ProjectMemory struct { /* unchanged */ }

func LoadProject(repoPath string) ProjectMemory
func SaveProject(repoPath string, m ProjectMemory) error
func AddProjectNote(repoPath, note string) error
func RemoveProjectEntry(repoPath string, idx int) (ProjectEntry, bool)
func FormatForPrompt(m ProjectMemory) string
func PrintProjectMemory(repoPath string)
```

---

### `internal/github`

Unified GitHub client. Consolidates `internal/community/github.go` and the client construction in `internal/social/engine.go`.

```go
// Package github provides a shared GitHub API client for iterate.
package github

// ErrNoToken is returned when GITHUB_TOKEN is not set.
var ErrNoToken = errors.New("GITHUB_TOKEN not set")

// NewClient creates an authenticated go-github client.
// Returns ErrNoToken if GITHUB_TOKEN is not set.
func NewClient(ctx context.Context) (*ghclient.Client, error)

// Sub-packages:
//   internal/github/issues      — issue fetching + formatting
//   internal/github/discussions — discussion fetching + reply posting
```

`internal/github/issues`:
```go
func FetchIssues(ctx context.Context, owner, repo string, issueTypes []IssueType, limit int) (map[IssueType][]Issue, error)
func FormatIssuesByType(issues map[IssueType][]Issue) string
func PostReply(ctx context.Context, owner, repo string, issueNumber int, body string) error
```

`internal/github/discussions`:
```go
func FetchDiscussions(ctx context.Context, token, owner, repo string) ([]Discussion, error)
func PostDiscussionReply(ctx context.Context, token, discussionID, body string) error
func CreateDiscussion(ctx context.Context, token, owner, repo, title, body string) error
```

`internal/social/engine.go` will call `github.NewClient()` instead of `community.NewGitHubClient()`.

---

### `internal/agentpool` (renamed from `internal/agent`)

Retains `pool.go` and `mutation.go`. The thin re-export wrapper `agent.go` is removed; call sites use `iteragent` types directly.

```go
// Package agentpool manages concurrent iteragent instances with rate limiting.
package agentpool

type Pool struct { /* unchanged from agent.Pool */ }
type RateLimiter struct { /* unchanged */ }

func NewPool(provider iteragent.Provider, tools []iteragent.Tool, logger *slog.Logger, maxAgents, rps int) *Pool
func MutationTestTool(repoPath string) iteragent.Tool
```

Type aliases (`Event`, `Message`, `Tool`, `Provider`) in `internal/agent/agent.go` are removed. All call sites use `iteragent.Event`, `iteragent.Message`, etc. directly.

---

## Data Models

### Config (`internal/config`)

```go
type Config struct {
    Provider      string   `json:"provider"       toml:"provider"`
    Model         string   `json:"model"          toml:"model"`
    OllamaBaseURL string   `json:"ollama_base_url,omitempty" toml:"ollama_base_url"`
    SafeMode      bool     `json:"safe_mode,omitempty"      toml:"safe_mode"`
    DeniedTools   []string `json:"denied_tools,omitempty"   toml:"denied_tools"`
    Theme         string   `json:"theme,omitempty"          toml:"theme"`
    Notify        bool     `json:"notify,omitempty"         toml:"notify"`
    Temperature   float64  `json:"temperature,omitempty"    toml:"temperature"`
    MaxTokens     int      `json:"max_tokens,omitempty"     toml:"max_tokens"`
    ThinkingLevel string   `json:"thinking_level,omitempty" toml:"thinking_level"`
    CacheEnabled  bool     `json:"cache_enabled,omitempty"  toml:"cache_enabled"`
    AllowPatterns []string `json:"allow_patterns,omitempty" toml:"allow_patterns"`
    DenyPatterns  []string `json:"deny_patterns,omitempty"  toml:"deny_patterns"`
    AllowDirs     []string `json:"allow_dirs,omitempty"     toml:"allow_dirs"`
    DenyDirs      []string `json:"deny_dirs,omitempty"      toml:"deny_dirs"`
}
```

Identical to the current `iterConfig` struct — only the package and type name change.

### Session (`internal/session`)

```go
type Bookmark struct {
    Name      string              `json:"name"`
    CreatedAt time.Time           `json:"created_at"`
    Messages  []iteragent.Message `json:"messages"`
}
```

On-disk format unchanged: `~/.iterate/sessions/<name>.json` and `~/.iterate/bookmarks.json`.

### Project Memory (`internal/memory`)

```go
type ProjectEntry struct {
    Note      string `json:"note"`
    CreatedAt string `json:"created_at"`
}

type ProjectMemory struct {
    Entries []ProjectEntry `json:"entries"`
}
```

On-disk format unchanged: `<repo>/.iterate/memory.json`.

### GitHub Issue (`internal/github/issues`)

```go
type IssueType string

const (
    IssueTypeInput      IssueType = "agent-input"
    IssueTypeSelf       IssueType = "agent-self"
    IssueTypeHelpWanted IssueType = "agent-help-wanted"
)

type Issue struct {
    Number   int
    Title    string
    Body     string
    NetVotes int
    URL      string
    Type     IssueType
}
```

### GitHub Discussion (`internal/github/discussions`)

```go
type Discussion struct {
    ID       string
    Number   int
    Title    string
    Body     string
    URL      string
    Comments []Comment
}

type Comment struct {
    ID     string
    Author string
    Body   string
}
```

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

This reorganization is structural, so most properties are static (file system, import graph, build output) rather than runtime behavioral. The key correctness guarantee is: the reorganized codebase is observationally equivalent to the original from the perspective of users, tests, and the build system.

### Property 1: Entry Point Is Minimal

*For any* valid state of the reorganized repository, the number of non-test `.go` files in `cmd/iterate/` SHALL be ≤ 5.

**Validates: Requirements 2.1**

---

### Property 2: Build Integrity

*For any* package move performed during reorganization, running `go build ./...` from the repository root SHALL succeed with zero errors.

**Validates: Requirements 2.10, 7.1, 7.5**

---

### Property 3: Test Suite Preservation

*For any* test that existed before the reorganization, that test SHALL still exist and pass after the reorganization. The set of passing tests is non-decreasing.

**Validates: Requirements 6.1, 6.2, 6.3**

---

### Property 4: UI Package Has No Upward Imports

*For any* `.go` file within `internal/ui` or its sub-packages, the file's import list SHALL NOT contain `internal/repl`, `internal/evolution`, or `internal/commands`.

**Validates: Requirements 5.2**

---

### Property 5: Single GitHub Client Constructor

*For any* `.go` file in the repository that constructs a `*github.Client`, that construction SHALL go through `internal/github.NewClient()` and not through a locally-defined OAuth2 flow.

**Validates: Requirements 3.1, 3.2, 3.5**

---

### Property 6: Missing Token Returns Sentinel Error

*For any* call to `internal/github.NewClient()` made when `GITHUB_TOKEN` is not set in the environment, the function SHALL return `(nil, ErrNoToken)` rather than `(nil, nil)`.

**Validates: Requirements 3.6**

---

### Property 7: Every Package Has a Doc Comment

*For any* Go package introduced or modified by this reorganization, the package's primary `.go` file SHALL contain a `// Package <name> ...` doc comment.

**Validates: Requirements 8.2**

---

### Property 8: CLI Interface Is Unchanged

*For any* build of the `iterate` binary produced after reorganization, running `./iterate --help` SHALL produce output byte-for-byte identical to the output produced by the pre-reorganization binary.

**Validates: Requirements 9.1, 9.2, 9.4**

---

### Property 9: New Package Layout Is Structurally Correct

*For any* valid completion of the reorganization, the following directories SHALL exist and contain the indicated logic:
- `internal/repl/` — REPL loop, streaming, agent construction
- `internal/ui/` — color constants, print helpers, highlight, selector
- `internal/config/` — config load/save, env overrides, permission helpers
- `internal/pricing/` — token cost tables and calculation
- `internal/provider/` — provider init and resolution
- `internal/session/` — session and bookmark persistence
- `internal/memory/` — project memory helpers
- `internal/github/` — unified GitHub client, issues, discussions
- `internal/agentpool/` — agent pool and mutation test tool

AND the following SHALL NOT exist (or SHALL be empty of logic):
- `internal/agent/agent.go` type-alias-only file
- `internal/community/` (merged into `internal/github/`)

**Validates: Requirements 2.2–2.9, 3.1, 3.3, 3.4, 4.2, 4.4, 5.1, 5.3**

---

## Error Handling

### Import Path Errors

During migration, `go build ./...` will fail if an import path is not updated. The migration sequence (one package at a time, build verified after each) ensures errors are caught immediately and localized to the most recent move.

Strategy: after moving each package, run `grep -r "old/import/path" --include="*.go" .` to find any remaining references before proceeding.

### Test Compilation Errors

When a test file is moved, its `package` declaration and imports must be updated. If a test uses unexported symbols from the original package, those symbols must either be exported or the test must be co-located with the source.

Strategy: run `go test ./...` after each package move. Any compilation failure is a signal to check package declarations and unexported symbol access.

### Circular Import Detection

The new dependency graph must be acyclic. The most likely cycle risk is `internal/repl` ↔ `internal/commands` (commands already imports from `internal/agent`; repl will import commands).

Strategy: `go build ./...` catches cycles at compile time. The dependency rules in the Architecture section are designed to prevent cycles.

### `internal/agent` Type Alias Removal

Removing the type aliases (`Event`, `Message`, `Tool`, `Provider`) from `internal/agent/agent.go` requires updating all call sites to use `iteragent.Event`, etc. directly. Since these are type aliases (not new types), the change is mechanical and the compiler will flag any missed sites.

### `internal/community` Removal

`internal/community` is currently imported by `cmd/iterate/main_mode.go` (for `community.IssueType`, `community.FetchIssues`, `community.FormatIssuesByType`). After moving these to `internal/github/issues`, `main_mode.go` must be updated to import `internal/github/issues` instead.

---

## Testing Strategy

### Dual Testing Approach

Both unit tests and property-based tests are used. Unit tests verify specific examples and edge cases; property tests verify universal invariants across all valid states.

### Unit Tests

Unit tests focus on:
- `internal/github.NewClient()` with and without `GITHUB_TOKEN` set (Property 6)
- `internal/pricing.EstimateCost()` with known model names and token counts
- `internal/config.CheckBashPermission()` with allow/deny patterns
- `internal/config.CheckDirPermission()` with allow/deny dirs
- `internal/session.Save()` / `Load()` round-trip
- `internal/memory.FormatForPrompt()` with empty and non-empty entries

Existing tests are preserved as-is (moved alongside their source files).

### Property-Based Tests

Property-based testing library: **`pgregory.net/rapid`** (already idiomatic for Go; no new dependencies needed beyond what the project already uses).

Each property test runs a minimum of **100 iterations**.

Tag format: `// Feature: code-reorganization, Property N: <property_text>`

#### Property 1 Test: Entry Point File Count

```go
// Feature: code-reorganization, Property 1: cmd/iterate/ contains ≤ 5 non-test .go files
func TestEntryPointIsMinimal(t *testing.T) {
    entries, _ := os.ReadDir("../../cmd/iterate")
    count := 0
    for _, e := range entries {
        if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
            count++
        }
    }
    if count > 5 {
        t.Errorf("cmd/iterate has %d non-test .go files, want ≤ 5", count)
    }
}
```

#### Property 2 Test: Build Integrity

Verified by CI: `go build ./...` in the repository root. Not a Go unit test — it is a Makefile target and CI step.

#### Property 3 Test: Test Suite Preservation

Verified by CI: `go test ./...` before and after. The test count (from `go test -v ./... | grep -c "^--- PASS"`) must be non-decreasing.

#### Property 4 Test: UI Package Import Constraints

```go
// Feature: code-reorganization, Property 4: internal/ui has no upward imports
// Implemented as a go/packages analysis test in internal/ui/ui_imports_test.go
func TestUIPackageHasNoUpwardImports(t *testing.T) {
    // Use golang.org/x/tools/go/packages to load internal/ui/...
    // Assert no import path contains "internal/repl", "internal/evolution", "internal/commands"
}
```

#### Property 5 Test: Single GitHub Client Constructor

```go
// Feature: code-reorganization, Property 5: all GitHub client construction via internal/github.NewClient
// Implemented as a grep-based test: search all .go files for oauth2.NewClient or github.NewClient
// outside of internal/github/ — assert zero matches
func TestSingleGitHubClientConstructor(t *testing.T) {
    // Walk all .go files, grep for "oauth2.NewClient" or "github.NewClient("
    // Assert no matches outside internal/github/
}
```

#### Property 6 Test: Missing Token Returns Sentinel Error

```go
// Feature: code-reorganization, Property 6: NewClient returns ErrNoToken when token absent
func TestNewClientNoToken(t *testing.T) {
    t.Setenv("GITHUB_TOKEN", "")
    client, err := github.NewClient(context.Background())
    if !errors.Is(err, github.ErrNoToken) {
        t.Errorf("expected ErrNoToken, got err=%v client=%v", err, client)
    }
}
```

#### Property 7 Test: Package Doc Comments

```go
// Feature: code-reorganization, Property 7: every new/modified package has a doc comment
// Implemented as a go/ast analysis test
func TestPackageDocComments(t *testing.T) {
    // For each package in internal/{repl,ui,config,pricing,provider,session,memory,github,agentpool}
    // Parse the primary .go file and assert fset.File(pkg.Syntax[0]).Name() has a doc comment
}
```

#### Property 8 Test: CLI Interface Unchanged

```go
// Feature: code-reorganization, Property 8: --help output is identical before and after
// Captured as a golden file test: golden/help_output.txt
func TestHelpOutputUnchanged(t *testing.T) {
    out, _ := exec.Command("./iterate", "--help").CombinedOutput()
    golden, _ := os.ReadFile("testdata/help_output.golden")
    if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(golden)) {
        t.Errorf("--help output changed:\ngot:  %s\nwant: %s", out, golden)
    }
}
```

#### Property 9 Test: Structural Layout

```go
// Feature: code-reorganization, Property 9: new package layout is structurally correct
func TestNewPackageLayout(t *testing.T) {
    required := []string{
        "internal/repl",
        "internal/ui",
        "internal/config",
        "internal/pricing",
        "internal/provider",
        "internal/session",
        "internal/memory",
        "internal/github",
        "internal/agentpool",
    }
    for _, pkg := range required {
        if _, err := os.Stat(pkg); os.IsNotExist(err) {
            t.Errorf("required package directory %s does not exist", pkg)
        }
    }
    // Assert internal/agent/agent.go (type-alias-only file) no longer exists
    if _, err := os.Stat("internal/agent/agent.go"); err == nil {
        t.Error("internal/agent/agent.go should have been removed")
    }
}
```

### Testing Balance

Unit tests handle specific examples (token cost for a known model, config parsing, session round-trip). Property tests handle structural invariants that must hold across the entire codebase. The existing test suite (moved alongside source files) provides behavioral regression coverage.
