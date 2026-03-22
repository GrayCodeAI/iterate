# Implementation Plan: Code Reorganization

## Overview

Migrate `cmd/iterate/` from 50 files to ≤ 5 files by extracting focused `internal/` packages one at a time. Each step ends with a passing `go build ./...`. The migration sequence follows the dependency order defined in the design: leaf packages first, `internal/repl` last, then `cmd/iterate` cleanup.

## Tasks

- [x] 1. Capture pre-reorganization baseline
  - Capture `./iterate --help` output to `cmd/iterate/testdata/help_output.golden`
  - Record the passing test count with `go test -v ./... 2>&1 | grep -c "^--- PASS"` and save to a scratch note
  - _Requirements: 6.1, 9.4_

- [-] 2. Create `internal/ui` package
  - [ ] 2.1 Create `internal/ui/ui.go` with color constants and print helpers
    - Move `ColorReset`, `ColorLime`, `ColorYellow`, `ColorDim`, `ColorBold`, `ColorCyan`, `ColorRed` from `internal/commands/registry.go`
    - Move `PrintSuccess`, `PrintError`, `PrintDim` from `internal/commands/registry.go`
    - Add `// Package ui provides terminal rendering helpers for iterate.` doc comment
    - Update `internal/commands/registry.go` to import `internal/ui` for these symbols
    - _Requirements: 5.1, 5.3, 5.4, 8.2_
  - [ ] 2.2 Create `internal/ui/highlight/` sub-package
    - Move `highlight.go` from `cmd/iterate/` to `internal/ui/highlight/highlight.go`
    - Update package declaration to `package highlight`
    - Add `// Package highlight provides syntax highlighting for terminal output.` doc comment
    - Move `highlight_test.go` and `highlight_expand_test.go` alongside, updating package declarations and imports
    - _Requirements: 2.4, 5.1, 6.2_
  - [ ] 2.3 Create `internal/ui/selector/` sub-package
    - Move `selector.go`, `selector_input.go`, `selector_history.go` from `cmd/iterate/` to `internal/ui/selector/`
    - Update package declarations to `package selector`
    - Add `// Package selector provides fuzzy-search and arrow-key selection UI.` doc comment
    - _Requirements: 2.3, 5.1, 8.2_
  - [ ]* 2.4 Write property test for UI import constraints (Property 4)
    - Create `internal/ui/ui_imports_test.go`
    - Use `golang.org/x/tools/go/packages` to load `internal/ui/...`
    - Assert no import path contains `internal/repl`, `internal/evolution`, or `internal/commands`
    - **Property 4: UI Package Has No Upward Imports**
    - **Validates: Requirements 5.2**
  - [ ]* 2.5 Write unit tests for ui color helpers
    - Verify `PrintSuccess`, `PrintError`, `PrintDim` write to stdout/stderr without panicking
    - _Requirements: 5.1_

- [~] 3. Create `internal/config` package
  - [ ] 3.1 Create `internal/config/config.go`
    - Move `iterConfig` struct (renamed to `Config`) and all load/save/path/env-override logic from `cmd/iterate/config.go`
    - Move `CheckBashPermission` and `CheckDirPermission` from `cmd/iterate/config.go`
    - Add `// Package config handles iterate configuration loading and persistence.` doc comment
    - Move `config_test.go` and `config_expand_test.go` alongside, updating package declarations and imports
    - _Requirements: 2.6, 6.2, 8.2_
  - [ ] 3.2 Update all callers of the old config package
    - Search all `.go` files for the old `iterConfig` type and `loadConfig`/`saveConfig` function references
    - Replace with `config.Config`, `config.Load()`, `config.Save()` from `internal/config`
    - _Requirements: 2.10, 7.1, 7.5_
  - [ ]* 3.3 Write unit tests for config permission helpers
    - Test `CheckBashPermission` with allow/deny patterns
    - Test `CheckDirPermission` with allow/deny dirs
    - _Requirements: 2.6_

- [~] 4. Create `internal/pricing` package
  - [ ] 4.1 Create `internal/pricing/pricing.go`
    - Move `ModelPricing`, `CostEstimate`, `LookupPricing`, `EstimateCost`, `FormatCostTable` from `cmd/iterate/pricing.go`
    - Add `// Package pricing calculates LLM token costs.` doc comment
    - _Requirements: 2.5, 8.2_
  - [ ] 4.2 Update all callers of pricing functions
    - Update imports in `cmd/iterate/` files that call pricing functions
    - _Requirements: 2.10, 7.1_
  - [ ]* 4.3 Write unit tests for pricing calculation
    - Test `EstimateCost` with known model names and token counts
    - Test `LookupPricing` for known and unknown model names
    - _Requirements: 2.5_

- [~] 5. Create `internal/provider` package
  - [ ] 5.1 Create `internal/provider/provider.go`
    - Move `ResolveConfig`, `ResolveThinkingLevel`, `Init` from `cmd/iterate/provider.go`
    - Import `internal/config` for the `Config` type
    - Add `// Package provider handles LLM provider initialization for iterate.` doc comment
    - _Requirements: 2.7, 8.2_
  - [ ] 5.2 Update all callers of provider functions
    - Update imports in `cmd/iterate/main_mode.go` and any other callers
    - _Requirements: 2.10, 7.1_

- [~] 6. Create `internal/session` package
  - [ ] 6.1 Create `internal/session/session.go`
    - Move `Save`, `Load`, `List`, `Dir`, `Bookmark`, `LoadBookmarks`, `SaveBookmarks`, `AddBookmark`, `InitAuditLog`, `LogAudit` from `cmd/iterate/features_sessions.go`
    - Add `// Package session manages REPL conversation persistence.` doc comment
    - Move `features_sessions_test.go` alongside, updating package declaration and imports
    - _Requirements: 2.8, 6.2, 8.2_
  - [ ] 6.2 Update all callers of session functions
    - Update imports in `cmd/iterate/` files that call session functions
    - _Requirements: 2.10, 7.1_
  - [ ]* 6.3 Write unit tests for session round-trip
    - Test `Save` / `Load` round-trip with a known message slice
    - Test `LoadBookmarks` / `SaveBookmarks` round-trip
    - _Requirements: 2.8_

- [~] 7. Create `internal/memory` package
  - [ ] 7.1 Create `internal/memory/memory.go`
    - Move `ProjectEntry`, `ProjectMemory`, `LoadProject`, `SaveProject`, `AddProjectNote`, `RemoveProjectEntry`, `FormatForPrompt`, `PrintProjectMemory` from `cmd/iterate/memory_project.go`
    - Add `// Package memory manages project-scoped and evolution memory for iterate.` doc comment
    - _Requirements: 2.9, 8.2_
  - [ ] 7.2 Update all callers of memory functions
    - Update imports in `cmd/iterate/` files that call memory functions
    - _Requirements: 2.10, 7.1_
  - [ ]* 7.3 Write unit tests for memory helpers
    - Test `FormatForPrompt` with empty and non-empty `ProjectMemory`
    - Test `AddProjectNote` / `RemoveProjectEntry` round-trip
    - _Requirements: 2.9_

- [~] 8. Checkpoint — build and tests green
  - Run `go build ./...` and ensure zero errors
  - Run `go test ./...` and ensure no previously-passing tests now fail
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 6.1, 7.1_

- [~] 9. Create `internal/github` package
  - [ ] 9.1 Create `internal/github/github.go` with `NewClient` and `ErrNoToken`
    - Consolidate OAuth2 client construction from `internal/community/github.go` and `internal/social/engine.go`
    - Expose `var ErrNoToken = errors.New("GITHUB_TOKEN not set")`
    - Expose `func NewClient(ctx context.Context) (*ghclient.Client, error)`
    - Add `// Package github provides a shared GitHub API client for iterate.` doc comment
    - _Requirements: 3.1, 3.2, 3.6, 8.2_
  - [ ] 9.2 Create `internal/github/issues/` sub-package
    - Move issue-fetching logic from `internal/community/` into `internal/github/issues/`
    - Expose `FetchIssues`, `FormatIssuesByType`, `PostReply`, `IssueType`, `Issue` types
    - Move relevant tests from `internal/community/` alongside
    - Add `// Package issues fetches and formats GitHub issues for iterate.` doc comment
    - _Requirements: 3.3, 6.2, 8.2_
  - [ ] 9.3 Create `internal/github/discussions/` sub-package
    - Move discussion-fetching logic from `internal/community/discussions.go` into `internal/github/discussions/`
    - Expose `FetchDiscussions`, `PostDiscussionReply`, `CreateDiscussion`, `Discussion`, `Comment` types
    - Move relevant tests alongside
    - Add `// Package discussions fetches and posts GitHub Discussions for iterate.` doc comment
    - _Requirements: 3.4, 6.2, 8.2_
  - [ ] 9.4 Update `internal/social/engine.go` to use `internal/github.NewClient()`
    - Replace the local OAuth2 client construction with a call to `github.NewClient(ctx)`
    - _Requirements: 3.5_
  - [ ] 9.5 Update `cmd/iterate/main_mode.go` to import `internal/github/issues` instead of `internal/community`
    - Replace `community.IssueType`, `community.FetchIssues`, `community.FormatIssuesByType` with the equivalents from `internal/github/issues`
    - _Requirements: 3.3, 7.1_
  - [ ] 9.6 Remove `internal/community/` package
    - Delete remaining files in `internal/community/` after all callers are updated
    - _Requirements: 3.1, 3.3, 3.4_
  - [ ]* 9.7 Write unit test for `NewClient` sentinel error (Property 6)
    - Create `internal/github/github_test.go`
    - Test that `NewClient` with `GITHUB_TOKEN=""` returns `(nil, ErrNoToken)`
    - **Property 6: Missing Token Returns Sentinel Error**
    - **Validates: Requirements 3.6**
  - [ ]* 9.8 Write property test for single GitHub client constructor (Property 5)
    - Create `internal/github/single_client_test.go`
    - Walk all `.go` files outside `internal/github/`, assert no direct `oauth2.NewClient` or inline `github.NewClient` construction
    - **Property 5: Single GitHub Client Constructor**
    - **Validates: Requirements 3.1, 3.2, 3.5**

- [~] 10. Rename `internal/agent` to `internal/agentpool`
  - [ ] 10.1 Create `internal/agentpool/` with `pool.go` and `mutation.go`
    - Copy `internal/agent/pool.go` and `internal/agent/mutation.go` to `internal/agentpool/`
    - Update package declarations to `package agentpool`
    - Add `// Package agentpool manages concurrent iteragent instances with rate limiting.` doc comment
    - Move `pool_test.go`, `pool_extended_test.go`, `mutation_test.go` alongside, updating package declarations
    - _Requirements: 4.3, 4.4, 6.2, 8.2_
  - [ ] 10.2 Remove `internal/agent/agent.go` type-alias file
    - Update all call sites that used `agent.Event`, `agent.Message`, `agent.Tool`, `agent.Provider` to use `iteragent.Event`, `iteragent.Message`, `iteragent.Tool`, `iteragent.Provider` directly
    - _Requirements: 4.2_
  - [ ] 10.3 Update all callers of `internal/agent` to import `internal/agentpool`
    - Search all `.go` files for `internal/agent` imports and replace with `internal/agentpool`
    - Delete `internal/agent/` directory once all callers are updated
    - _Requirements: 4.4, 7.1, 7.5_

- [~] 11. Create `internal/repl` package
  - [ ] 11.1 Create `internal/repl/repl.go` with `Run`, `MakeAgent`, `handleCommand`, `buildCommandContext`
    - Move REPL loop logic from `cmd/iterate/repl.go`
    - Import `internal/ui`, `internal/config`, `internal/session`, `internal/commands`, `internal/pricing`, `internal/provider`, `internal/memory`, `internal/agentpool`
    - Add `// Package repl implements the interactive REPL for iterate.` doc comment
    - _Requirements: 2.2, 8.2_
  - [ ] 11.2 Move `repl_helpers.go` into `internal/repl/`
    - Move `printHeader`, `printSessionSummary`, `printStatusLine` from `cmd/iterate/repl_helpers.go`
    - Update package declaration and imports
    - _Requirements: 2.2_
  - [ ] 11.3 Move `repl_streaming.go` into `internal/repl/`
    - Move `StreamAndPrint` and the event loop from `cmd/iterate/repl_streaming.go`
    - Update package declaration and imports
    - _Requirements: 2.2_
  - [ ] 11.4 Move `repl_models.go` and `state.go` into `internal/repl/`
    - Move `selectModel`, model list, `sessionState`, `replConfig`, and package-level vars
    - Update package declarations and imports
    - _Requirements: 2.2_
  - [ ] 11.5 Move features files into `internal/repl/`
    - Move `features.go`, `features_git_helpers.go`, `features_prompts.go`, `features_search.go`, `features_shell.go`, `features_tools.go`, `features_watch.go` from `cmd/iterate/`
    - Move corresponding test files alongside, updating package declarations and imports
    - _Requirements: 2.2, 6.2_
  - [ ] 11.6 Move commands files into `internal/repl/` or `internal/commands/`
    - Move `commands_git.go` and `commands_project.go` from `cmd/iterate/` to `internal/commands/`
    - Move corresponding test files alongside, updating package declarations and imports
    - _Requirements: 2.2, 6.2_

- [~] 12. Clean up `cmd/iterate/` entry point
  - [ ] 12.1 Update `cmd/iterate/main.go` to import `internal/repl`
    - Replace direct REPL calls with `repl.Run(...)`
    - Remove any imports that are now satisfied by `internal/repl`
    - _Requirements: 2.1, 2.2_
  - [ ] 12.2 Update `cmd/iterate/main_mode.go` imports
    - Replace all moved-package imports with their new `internal/` paths
    - Ensure `runMode`, `runSocialMode`, `runEvolutionMode` logic is unchanged
    - _Requirements: 2.1, 9.1, 9.2, 9.3_
  - [ ] 12.3 Delete all files moved out of `cmd/iterate/`
    - Remove every file that was moved to an `internal/` package
    - Verify `cmd/iterate/` contains exactly: `main.go`, `main_flags.go`, `main_mode.go` (≤ 5 files)
    - _Requirements: 2.1_
  - [ ]* 12.4 Write property test for entry point file count (Property 1)
    - Create `cmd/iterate/entry_point_test.go`
    - Count non-test `.go` files in `cmd/iterate/` and assert count ≤ 5
    - **Property 1: Entry Point Is Minimal**
    - **Validates: Requirements 2.1**

- [~] 13. Checkpoint — full build and test suite
  - Run `go build ./...` and ensure zero errors
  - Run `go test ./...` and ensure the passing test count is ≥ the baseline captured in task 1
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 6.1, 7.1_

- [~] 14. Add package doc comments to all new/modified packages
  - [ ] 14.1 Verify doc comments exist in every new package's primary file
    - Check `internal/repl`, `internal/ui`, `internal/ui/highlight`, `internal/ui/selector`, `internal/config`, `internal/pricing`, `internal/provider`, `internal/session`, `internal/memory`, `internal/github`, `internal/github/issues`, `internal/github/discussions`, `internal/agentpool`
    - Add any missing `// Package <name> <description>.` comments
    - _Requirements: 8.2_
  - [ ]* 14.2 Write property test for package doc comments (Property 7)
    - Create `internal/reorganization_test.go` (or similar)
    - Use `go/ast` to parse each new package's primary file and assert a doc comment is present
    - **Property 7: Every Package Has a Doc Comment**
    - **Validates: Requirements 8.2**

- [~] 15. Update documentation and build files
  - [ ] 15.1 Update `README.md` Architecture section
    - List every package with a one-line description reflecting the new layout
    - _Requirements: 7.4, 8.1_
  - [ ] 15.2 Update `docs/CLAUDE.md` agent context file
    - Reflect the new package layout so the agent places new code correctly in future sessions
    - _Requirements: 8.3_
  - [ ] 15.3 Update `Makefile` if any build targets reference moved file paths
    - Search for hardcoded file paths in `Makefile` and update as needed
    - _Requirements: 7.3_

- [~] 16. Verify CLI interface is unchanged
  - [ ] 16.1 Rebuild the binary and compare `--help` output to the golden file
    - Run `go build -o iterate ./cmd/iterate`
    - Run `./iterate --help` and diff against `cmd/iterate/testdata/help_output.golden`
    - _Requirements: 9.1, 9.2, 9.4_
  - [ ]* 16.2 Write golden file test for CLI interface (Property 8)
    - Create `cmd/iterate/help_output_test.go`
    - Run `./iterate --help` via `exec.Command` and compare bytes to `testdata/help_output.golden`
    - **Property 8: CLI Interface Is Unchanged**
    - **Validates: Requirements 9.1, 9.2, 9.4**

- [~] 17. Write structural layout property test (Property 9)
  - [ ]* 17.1 Create `internal/layout_test.go`
    - Assert all required directories exist: `internal/repl`, `internal/ui`, `internal/config`, `internal/pricing`, `internal/provider`, `internal/session`, `internal/memory`, `internal/github`, `internal/agentpool`
    - Assert `internal/agent/agent.go` no longer exists
    - Assert `internal/community/` no longer exists (or is empty)
    - **Property 9: New Package Layout Is Structurally Correct**
    - **Validates: Requirements 2.2–2.9, 3.1, 3.3, 3.4, 4.2, 4.4, 5.1, 5.3**

- [~] 18. Final checkpoint — all tests pass
  - Run `go build ./...` — zero errors
  - Run `go test ./...` — all tests pass, count ≥ baseline
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 6.1, 6.3, 7.1_

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster migration
- Each task ends with a `go build ./...` verification before moving to the next
- The migration sequence (tasks 2–11) follows the dependency order in the design: leaf packages first, `internal/repl` last
- Property tests validate structural invariants that must hold across the entire codebase
- Existing tests are moved alongside their source files — no tests are deleted
