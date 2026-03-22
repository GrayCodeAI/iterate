# Requirements Document

## Introduction

The `iterate` codebase has grown organically as a self-evolving Go CLI tool. Over time, `cmd/iterate/` has accumulated 50 files mixing REPL logic, UI concerns, session management, config, pricing, and command dispatch. The `internal/` packages are reasonably separated but have some boundary blurring (e.g., `social` duplicating GitHub client logic from `community`, `agent` being a thin re-export wrapper). This reorganization aims to apply Go project layout best practices: clear separation of concerns, discoverable package names, minimal coupling between layers, and a structure that scales as the agent continues to self-evolve.

## Glossary

- **Reorganizer**: The tooling, scripts, or agent process that performs the structural migration
- **Package**: A Go package directory with a single coherent responsibility
- **REPL**: The interactive read-eval-print loop (`--chat` mode)
- **Evolution_Engine**: The 3-phase self-improvement loop (plan → implement → communicate)
- **Command_Registry**: The modular slash-command system used inside the REPL
- **Provider**: An LLM API backend (Anthropic, OpenAI, Gemini, Groq)
- **Session**: A persisted conversation history (messages + metadata)
- **Memory**: The append-only learning store (`learnings.jsonl`, `social_learnings.jsonl`)
- **Selector**: The interactive terminal UI component for fuzzy-search and history navigation
- **Highlight**: The syntax-highlighting subsystem for terminal output
- **Pricing**: The token-cost calculation subsystem
- **Config**: The user-facing configuration file (`~/.iterate/config.json` or repo-local)
- **Skills**: Structured markdown files that guide agent behavior
- **Community**: GitHub Issues/Discussions integration for reading external input
- **Social_Engine**: The loop that reads GitHub Discussions and posts replies

## Requirements

### Requirement 1: Audit Current Package Boundaries

**User Story:** As a developer navigating the codebase, I want a clear audit of what each file does and which logical layer it belongs to, so that I can understand the reorganization rationale before any files are moved.

#### Acceptance Criteria

1. THE Reorganizer SHALL produce a mapping of every file in `cmd/iterate/` to one of the following logical layers: `cli-entry`, `repl-core`, `repl-ui`, `session`, `config`, `provider`, `pricing`, `commands`, `features`
2. THE Reorganizer SHALL identify all files in `internal/` that have overlapping responsibilities with other packages
3. THE Reorganizer SHALL identify all import cycles that would be introduced or resolved by the proposed new structure
4. WHEN two packages share more than one type or function, THE Reorganizer SHALL flag them as candidates for consolidation

---

### Requirement 2: Flatten `cmd/iterate/` into Focused Sub-packages

**User Story:** As a developer, I want `cmd/iterate/` to contain only the CLI entry point and mode dispatch, so that the package is easy to read and the binary's wiring is obvious at a glance.

#### Acceptance Criteria

1. THE Reorganizer SHALL ensure `cmd/iterate/` contains only files directly responsible for `main()`, flag parsing, and top-level mode dispatch (target: ≤ 5 files)
2. THE Reorganizer SHALL move REPL logic (`repl.go`, `repl_helpers.go`, `repl_streaming.go`, `repl_models.go`, `state.go`) into a dedicated `internal/repl` package
3. THE Reorganizer SHALL move selector/input UI components (`selector.go`, `selector_input.go`, `selector_history.go`) into `internal/repl/selector` or `internal/ui/selector`
4. THE Reorganizer SHALL move syntax highlighting (`highlight.go`) into `internal/ui/highlight` or `internal/repl/highlight`
5. THE Reorganizer SHALL move pricing logic (`pricing.go`) into `internal/pricing`
6. THE Reorganizer SHALL move config loading/saving (`config.go`) into `internal/config`
7. THE Reorganizer SHALL move provider initialization (`provider.go`) into `internal/provider` or merge it into `internal/config`
8. THE Reorganizer SHALL move session persistence (`features_sessions.go`) into `internal/session`
9. THE Reorganizer SHALL move memory/project helpers (`memory_project.go`) into `internal/memory` alongside the existing memory files
10. WHEN a file is moved, THE Reorganizer SHALL update all import paths referencing that file's package

---

### Requirement 3: Consolidate GitHub Integration

**User Story:** As a developer, I want all GitHub API interaction to live in one place, so that authentication, rate limiting, and client construction are not duplicated.

#### Acceptance Criteria

1. THE Reorganizer SHALL consolidate `internal/community/github.go` and the GitHub client construction in `internal/social/engine.go` into a single `internal/github` package
2. THE `internal/github` package SHALL expose a single `NewClient(ctx) *github.Client` constructor used by all callers
3. THE Reorganizer SHALL move issue-fetching logic (`internal/community/`) into `internal/github/issues`
4. THE Reorganizer SHALL move discussion-fetching logic (`internal/community/discussions.go`) into `internal/github/discussions`
5. WHEN `internal/social` needs GitHub access, THE Social_Engine SHALL import from `internal/github` rather than constructing its own client
6. IF `GITHUB_TOKEN` is not set, THEN THE `internal/github` package SHALL return a sentinel error rather than a nil client, so callers can handle the absence explicitly

---

### Requirement 4: Clarify `internal/agent` Responsibility

**User Story:** As a developer, I want `internal/agent` to have a clear, non-trivial purpose, so that it is not just a thin re-export wrapper around `iteragent`.

#### Acceptance Criteria

1. THE Reorganizer SHALL evaluate whether `internal/agent/agent.go` (currently only type aliases and a constructor) provides enough value to remain a separate package
2. WHERE `internal/agent` adds no logic beyond re-exporting `iteragent` types, THE Reorganizer SHALL inline those type aliases at their call sites and remove the package
3. THE Reorganizer SHALL retain `internal/agent/pool.go` and `internal/agent/mutation.go` as a package only if they contain logic not present in `iteragent`
4. WHEN `internal/agent` is retained, THE Reorganizer SHALL rename it to `internal/agentpool` to signal its specific purpose (managing concurrent agent instances)

---

### Requirement 5: Establish a `internal/ui` Layer

**User Story:** As a developer, I want all terminal UI concerns (colors, highlighting, selectors, spinners) grouped together, so that they can be tested and themed independently of business logic.

#### Acceptance Criteria

1. THE Reorganizer SHALL create an `internal/ui` package (or sub-packages) containing: syntax highlighting, color/theme helpers, selector/fuzzy-search UI, and spinner state
2. THE Reorganizer SHALL ensure no `internal/ui` sub-package imports from `internal/repl`, `internal/evolution`, or `internal/commands` (dependency must flow inward only)
3. THE Reorganizer SHALL move color constants and print helpers currently in `internal/commands/registry.go` (`ColorReset`, `PrintSuccess`, etc.) into `internal/ui`
4. WHEN a command handler needs to print colored output, THE Command_Registry SHALL import from `internal/ui` rather than defining colors inline

---

### Requirement 6: Preserve All Existing Tests

**User Story:** As a developer, I want every existing test to pass after the reorganization, so that I have confidence no behavior was accidentally changed.

#### Acceptance Criteria

1. THE Reorganizer SHALL run `go test ./...` before and after the reorganization and SHALL produce identical pass/fail results
2. WHEN a test file is moved alongside its source file, THE Reorganizer SHALL update the `package` declaration and import paths in the test file
3. THE Reorganizer SHALL NOT delete or skip any existing test
4. IF any test fails after reorganization, THEN THE Reorganizer SHALL revert the relevant file move and document the conflict before proceeding

---

### Requirement 7: Update Import Paths and Build Integrity

**User Story:** As a developer, I want the project to build cleanly after reorganization with no broken imports, so that the binary can be produced immediately.

#### Acceptance Criteria

1. THE Reorganizer SHALL run `go build ./...` after each package move and SHALL resolve any import errors before moving the next package
2. THE Reorganizer SHALL update `go.mod` if any new internal module boundaries are introduced
3. THE Reorganizer SHALL update the `Makefile` if any build targets reference moved file paths
4. THE Reorganizer SHALL update `README.md`'s Architecture section to reflect the new structure
5. WHEN a file is moved, THE Reorganizer SHALL search all `.go` files for the old import path and replace it with the new one

---

### Requirement 8: Document the New Structure

**User Story:** As a developer (or the agent itself in future sessions), I want a clear reference for what each package does, so that new code is placed in the right location.

#### Acceptance Criteria

1. THE Reorganizer SHALL produce an updated Architecture section in `README.md` listing every package with a one-line description
2. THE Reorganizer SHALL add a `// Package <name> <description>.` doc comment to every package that currently lacks one
3. THE Reorganizer SHALL ensure the `docs/CLAUDE.md` agent context file reflects the new package layout so the agent places new code correctly in future sessions

---

### Requirement 9: Maintain Backward-Compatible CLI Interface

**User Story:** As a user of the `iterate` binary, I want the CLI flags, REPL commands, and behavior to remain identical after reorganization, so that my scripts and workflows are not broken.

#### Acceptance Criteria

1. THE Reorganizer SHALL NOT change any flag name, default value, or behavior defined in `cmd/iterate/main_flags.go`
2. THE Reorganizer SHALL NOT change any REPL slash-command name, alias, or output format
3. THE Reorganizer SHALL NOT change the evolution phase names (`plan`, `implement`, `communicate`)
4. WHEN the binary is rebuilt after reorganization, THE Reorganizer SHALL verify `./iterate --help` produces output identical to the pre-reorganization output
