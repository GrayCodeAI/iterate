// Package evolution provides autonomous code evolution capabilities
// through iterative improvement cycles with safety checks and
// version control integration.
//
// # Architecture
//
// The evolution system is the core self-improvement engine of iterate.
// It operates in phases: plan, code, review, and merge. Each phase
// uses specialized tools and agents to safely modify the codebase.
//
// # Phase Overview
//
//   - Plan Phase: Analyzes the codebase, identifies bugs/issues,
//     and creates a SESSION_PLAN.md with tasks. Uses read-only tools.
//     See: Engine.RunPlanPhase(), BuildSystemPromptAider()
//
//   - Code Phase: Executes tasks from the plan, generating unified diffs
//     that fix bugs or add features. Supports parallel task execution
//     via git worktrees to avoid conflicts.
//     See: Engine.RunCodePhase(), ApplyUnifiedDiffs()
//
//   - Review Phase: Validates changes through build, test, and vet checks.
//     Rejects changes that break the codebase.
//     See: Engine.RunReviewPhase(), verifyProtected()
//
//   - Merge Phase: Commits and optionally creates pull requests with
//     detailed descriptions of changes made.
//     See: phases_pr.go, buildPRBody()
//
// # Diff Application
//
// The system uses unified diff format (git-style) for code changes,
// which is 3x more effective than custom formats. Three fallback
// strategies ensure robust application:
//
//  1. Strategy 1: Exact match with context lines
//  2. Strategy 2: Normalized whitespace matching
//  3. Strategy 3: Block-level replacement without context
//
// See: prompts_aider.go (ApplyUnifiedDiffs, applyDiffStrategy1-3)
//
// # Safety Mechanisms
//
//   - Sandboxed execution: Commands validated against allowlists and
//     blocked patterns (sandbox.go)
//   - Protected files: Git-related and config files cannot be modified
//     (safety.go)
//   - Protected directories: vendor, node_modules, .git are excluded
//   - Verification gates: go build, go vet, go test run after changes
//     (verify.go)
//   - Git snapshots: Automatic backup before changes (snapshot.go)
//
// # Multi-Agent Architecture
//
// Specialized agent types handle different aspects:
//
//   - Plan Agent: Analyzes codebase, creates plans (read-only tools)
//   - Build Agent: Implements code changes (full tool access)
//   - Review Agent: Validates changes (read + test tools)
//   - Test Agent: Writes and runs tests
//
// See: agents.go for AgentType, AgentConfig
//
// # Provider Management
//
// Supports multiple AI model providers with automatic fallback
// on rate limits or failures.
//
// See: provider_pool.go
//
// # Memory & Learning
//
// The system maintains memory of:
//   - Active learnings (memory/ACTIVE_LEARNINGS.md)
//   - Evolution journal (internal/evolution/journal.go)
//   - Failure patterns (failure_learning.go)
//   - Historical evolution data (memory.go)
//
// # Test-Driven Development
//
// TDD enforcement ensures code changes include tests:
//   - Write failing test → Write code fix → Verify test passes
//
// See: tdd.go
//
// # MCP Integration
//
// Model Context Protocol support for external tool integration.
//
// See: mcp.go
//
// # Repository Mapping
//
// Builds comprehensive maps of files, functions, and imports
// for better context during planning.
//
// See: repo_map.go
package evolution
