# iterate V3 - Complete Implementation Plan
## Based on Research of Top 20 AI Coding Agents

---

## PART 1: TOP 20 AI CODING AGENTS - COMPREHENSIVE LIST

### Category A: CLI/TUI Terminal Agents

| # | Agent | Stars | Developer | Key Feature | Price |
|---|-------|-------|-----------|------------|-------|
| 1 | **Claude Code** | 98K ⭐ | Anthropic | Best autonomous agent, MCP support | $20/mo |
| 2 | **Gemini CLI** | 99K ⭐ | Google | 1M token context, free | Free |
| 3 | **OpenCode** | 103K ⭐ | OpenCode AI | Provider-agnostic, LSP support | Free (BYOK) |
| 4 | **Aider** | 42K ⭐ | Community | Git-native, unified diffs | Free (BYOK) |
| 5 | **Goose** | 33K ⭐ | Block | MCP extensions, recipes | Free |
| 6 | **Codex CLI** | 45K ⭐ | OpenAI | Rust-based, permission levels | $20/mo |
| 7 | **Amp** | - | Sourcegraph | Deep reasoning, Oracle agent | Free tier |
| 8 | **Warp AI** | - | Warp Inc | Terminal replacement, Oz agents | $20/mo |
| 9 | **NovaKit CLI** | - | NovaKit | Gemini-powered terminal | Free |
| 10 | **AI Magicx CLI** | - | AI Magicx | Multi-provider | Free |

### Category B: IDE-Based Agents

| # | Agent | Rating | Developer | Key Feature | Price |
|---|-------|--------|-----------|------------|-------|
| 11 | **Cursor** | 96/100 | Anysphere | Best IDE integration | $20/mo |
| 12 | **Windsurf** | 91/100 | Codeium | Cascade agent | Free tier |
| 13 | **GitHub Copilot** | - | Microsoft | IDE integration | $10/mo |
| 14 | **Cline** | 59K ⭐ | Cline | Autonomous IDE coding | Free |
| 15 | **Kiro** | - | Kiro | AI-first IDE | Free |

### Category C: Autonomous/Cloud Agents

| # | Agent | SWE-bench | Developer | Key Feature | Price |
|---|-------|-----------|-----------|------------|-------|
| 16 | **Devin** | 13.86% | Cognition | Fully autonomous, sandboxed | $20/mo |
| 17 | **Claude Devin** | - | Anthropic | Claude-powered Devin | $20/mo |
| 18 | **DeepWiki** | - | Cognition | Codebase documentation | Free |
| 19 | **Devin Search** | - | Cognition | Q&A about codebases | Free |

### Category D: Specialized Agents

| # | Agent | Focus | Developer | Key Feature | Price |
|---|-------|-------|-----------|------------|-------|
| 20 | **CodeRabbit** | Code review | CodeRabbit | PR review automation | Free tier |

---

## PART 2: KEY TECHNICAL INSIGHTS FROM RESEARCH

### Aider (42K stars) - The Git-Native Pioneer
- **Unified diff format** - 3X better than custom SEARCH/REPLACE
- **Flexible patching** - 9X improvement with error recovery
- **Model-agnostic** - Works with any LLM
- **Auto git commits** - Descriptive commit messages
- **Lesson**: Use familiar formats (git diffs), be flexible with errors

### Claude Code (98K stars) - The Autonomous Leader
- **MCP (Model Context Protocol)** - Standardized tool integration
- **Repo mapping** - Maps entire codebase before changes
- **Context compaction** - Handles long sessions without crashing
- **Multi-step execution** - Plans, executes, verifies
- **Subagents** - Specialized agents for different tasks
- **Lesson**: Build robust scaffolding, use MCP for extensibility

### OpenCode (103K stars) - The Provider-Agnostic
- **Client/server architecture** - Bun runtime + Go TUI
- **AI SDK** - Provider-agnostic LLM access
- **Plan/Build agents** - Separate planning from execution
- **LSP integration** - Real-time diagnostics
- **Snapshot/restore** - Git-based state management
- **Lesson**: Separate concerns, build for flexibility

### Gemini CLI (99K stars) - The Free Powerhouse
- **1M token context** - Entire codebase in one shot
- **Free tier** - 60 req/min, 1000 req/day
- **ReAct loop** - Reason + Act pattern
- **Lesson**: Leverage context window, minimize cost

### Devin (Cognition) - The Autonomous Pioneer
- **Sandboxed execution** - Isolated VM per session
- **Interactive planning** - Human reviews before execution
- **Desktop use** - Can interact with GUI apps
- **DeepWiki** - Auto-generates documentation
- **Lesson**: Safety through sandboxing, human-in-loop for safety

### Goose (33K stars) - The Extensible
- **MCP-first** - Everything via MCP
- **Recipes** - Reusable agent patterns
- **Headless mode** - CI/CD integration
- **Sandboxed execution** - Secure by default
- **Lesson**: Build extension points, think headless

### Codex CLI (OpenAI) - The Lightweight
- **Rust-based** - Fast, small binary
- **3-tier permissions** - Read-only, Auto, Full
- **ChatGPT integration** - Already paid for by users
- **Lesson**: Fast execution, trust levels

### Amp (Sourcegraph) - The Deep Reasoning
- **Deep mode** - Extended reasoning for complex problems
- **Oracle agent** - Codebase analysis
- **Librarian agent** - Documentation Q&A
- **Sub-agents** - Team of specialized agents
- **Lesson**: Multi-agent architecture

---

## PART 3: COMMON SUCCESS PATTERNS

### 1. Edit Format Matters (Aider Research)
| Format | Success Rate | Notes |
|--------|-------------|-------|
| Custom JSON | 20% | LLMs unfamiliar |
| SEARCH/REPLACE | 20% | Our current issue |
| **Unified Diff** | **61%** | **3X better** - familiar to LLMs |
| Tool Calling (JSON) | Varies | Model-dependent |

**Key insight**: Use formats LLMs have seen millions of times in training (git diffs)

### 2. Error Recovery is Critical (Aider Research)
- Without flexible patching: 9X more failures
- Strategies that work:
  1. Normalize whitespace
  2. Try fuzzy matching
  3. Split into smaller hunks
  4. Expand context window

### 3. Context Management
- Repo mapping (Claude Code)
- Incremental context
- Session summarization
- Snapshot/restore (OpenCode)

### 4. Safety Mechanisms
- Sandboxed execution (Devin, Goose)
- Permission levels (Codex)
- Pre-commit verification
- Post-change testing

### 5. Multi-Agent Architecture
- Plan vs Build separation (OpenCode)
- Sub-agents for specialized tasks (Amp)
- Task agents (Claude Code)
- Review agents (CodeRabbit)

---

## PART 4: ITERATE V3 IMPLEMENTATION PLAN

### Phase 1: Core Architecture (Week 1)

#### 1.1 Unified Diff Format ✅ ALREADY IMPLEMENTED
- Replace SEARCH/REPLACE with unified diffs
- Implement 3 fallback strategies
- Update prompts

#### 1.2 Multi-Agent Architecture
**Implementation:**
```go
// Plan Agent - analyzes but doesn't edit
type PlanAgent struct {
    Name string
    Tools []string // read, grep, glob only
    Prompt string
}

// Build Agent - can edit files
type BuildAgent struct {
    Name string
    Tools []string // all tools
    Prompt string
}

// Review Agent - verifies changes
type ReviewAgent struct {
    Name string
    Tools []string
    Prompt string
}
```

#### 1.3 Flexible Error Recovery
**Implementation:**
```go
func ApplyDiffWithRetry(content, diff string) (string, error) {
    // Strategy 1: Direct patch
    // Strategy 2: Normalize whitespace  
    // Strategy 3: Fuzzy matching
    // Strategy 4: Split into smaller hunks
    // Strategy 5: Expand context window
}
```

### Phase 2: Context & Safety (Week 2)

#### 2.1 Repo Mapping
**Implementation:**
```go
type RepoMap struct {
    Files map[string]FileInfo
    Functions map[string][]Function
    Imports map[string][]string
}

func BuildRepoMap(root string) *RepoMap {
    // Use AST parsing
    // Extract functions, classes, imports
    // Build dependency graph
}
```

#### 2.2 Sandboxed Execution
**Implementation:**
```go
type SandboxConfig struct {
    AllowedCmds []string
    BlockedPatterns []string
    Timeout time.Duration
    MemoryLimit int64
}

func (e *Engine) ExecuteInSandbox(cmd string) error {
    // Whitelist allowed commands
    // Block destructive patterns
    // Set timeout
    // Monitor resources
}
```

#### 2.3 Snapshot/Restore
**Implementation:**
```go
func (e *Engine) CreateSnapshot() (string, error) {
    // Git add + write-tree
    return hash, nil
}

func (e *Engine) RestoreSnapshot(hash string) error {
    // Git read-tree + checkout-index
}
```

### Phase 3: Provider & Tools (Week 2-3)

#### 3.1 Multi-Provider Support
**Implementation:**
```go
type ProviderPool struct {
    providers map[string]Provider
    current Provider
    rateLimitCount int
}

func (p *ProviderPool) CallWithFallback(req Request) error {
    for _, provider := range p.providers {
        if resp, err := provider.Call(req); err == nil {
            return resp
        }
        if isRateLimit(err) {
            continue // Try next
        }
        return err // Real error
    }
    return ErrAllProvidersFailed
}
```

#### 3.2 MCP Integration
**Implementation:**
```go
type MCPClient struct {
    serverPath string
    transport string // stdio, http
}

func (m *MCPClient) ListTools() ([]Tool, error)
func (m *MCPClient) CallTool(name string, args map[string]interface{}) (string, error)
```

#### 3.3 Tool Permission Levels
**Implementation:**
```go
const (
    PermissionReadOnly  Permission = "read"   // glob, grep, read
    PermissionAuto      Permission = "auto"   // can edit, needs approval
    PermissionFull      Permission = "full"   // can do anything
)
```

### Phase 4: Verification & Testing (Week 3)

#### 4.1 Pre-Change Verification
**Implementation:**
```go
func PreChangeCheck(changes []string) error {
    // 1. Check protected files
    // 2. Check for destructive commands
    // 3. Check test coverage impact
}
```

#### 4.2 Post-Change Verification
**Implementation:**
```go
func PostChangeCheck() error {
    // 1. Run go build
    // 2. Run go vet
    // 3. Run go test
    // 4. Check for regressions
}
```

#### 4.3 TDD Enforcement
**Implementation:**
```go
func EnforceTDD(task Task) error {
    // 1. Require test file in diff
    // 2. Run test - should FAIL initially
    // 3. Make code change
    // 4. Run test - should PASS
}
```

### Phase 5: Learning & Adaptation (Week 3-4)

#### 5.1 Failure Pattern Memory
**Implementation:**
```go
type FailurePattern struct {
    ErrorType string
    Model string
    Frequency int
    FixSuggestion string
}

func (e *Engine) LearnFromFailure(err error) {
    pattern := categorizeError(err)
    saveToJSONL("memory/failures.jsonl", pattern)
}
```

#### 5.2 Model-Specific Prompts
**Implementation:**
```go
func GetModelSpecificPrompt(model string) string {
    switch {
    case strings.Contains(model, "claude"):
        return claudePrompt // Use JSON tool calls
    case strings.Contains(model, "gpt"):
        return gptPrompt // Use unified diffs
    case strings.Contains(model, "gemini"):
        return geminiPrompt // Use step-by-step
    }
}
```

---

## PART 5: DETAILED FILE CHANGES

### Files to Create:

| File | Purpose |
|------|---------|
| `internal/evolution/agents.go` | Multi-agent architecture |
| `internal/evolution/repo_map.go` | Repository mapping |
| `internal/evolution/sandbox.go` | Sandboxed execution |
| `internal/evolution/snapshot.go` | Git-based snapshots |
| `internal/evolution/provider_pool.go` | Multi-provider fallback |
| `internal/evolution/mcp.go` | MCP client support |
| `internal/evolution/permissions.go` | Tool permission levels |
| `internal/evolution/tdd.go` | TDD enforcement |

### Files to Modify:

| File | Changes |
|------|---------|
| `internal/evolution/prompts_aider.go` | Already updated with unified diffs |
| `internal/evolution/phases.go` | Already updated for unified diffs |
| `internal/evolution/engine.go` | Add agent routing |
| `scripts/evolution/evolve.sh` | Already has API rotation |

---

## PART 6: SUCCESS METRICS

### Current vs Target

| Metric | Current | Target |
|--------|---------|--------|
| Code changes/evolution | ~3 | 10+ |
| Test inclusion | 50% | 100% |
| Build pass rate | 70% | 95% |
| Unified diff success | 0% | 70%+ |
| Rate limit recovery | Manual | Auto |
| Context relevance | 40% | 80% |

### Key KPIs to Track

1. **Task Success Rate** - % of tasks completed successfully
2. **Test Coverage** - Coverage maintained/increased
3. **Error Recovery** - % of errors recovered with retries
4. **Code Quality** - Lint/vet pass rate
5. **API Cost** - Cost per successful task

---

## PART 7: IMPLEMENTATION PRIORITY

### P0 - Must Fix This Week:
1. ✅ Unified diff format (done)
2. ⏳ Flexible diff application (in progress)
3. ⏳ Multi-agent architecture (plan/build separation)
4. ⏳ Better error messages

### P1 - Should Fix Next Week:
1. Repo mapping for better context
2. Sandboxed command execution
3. Multi-provider fallback
4. Pre/post verification gates

### P2 - Week 3:
1. MCP integration
2. Failure pattern learning
3. Model-specific prompts
4. Snapshot/restore

---

## KEY TAKEAWAYS FROM TOP 20 AGENTS

### What Works:
1. **Familiar formats** - Unified diffs > custom formats (3X better)
2. **Flexible error handling** - 9X improvement with retries
3. **Plan/Build separation** - OpenCode, Claude Code
4. **Sandboxing** - Devin, Goose for safety
5. **MCP** - Standardized tool integration
6. **Git-native** - Aider's commit workflow
7. **Context management** - Claude Code compaction

### What Doesn't Work:
1. **Custom edit formats** - LLMs don't follow
2. **Single agent** - Need specialized agents
3. **No error recovery** - Fail fast = fail often
4. **Unbounded execution** - Need sandbox + timeout
5. **Single provider** - Rate limits happen

### iterate's Competitive Advantage:
1. **Autonomous** - Self-evolving (unique)
2. **Go-native** - Built in Go, for Go projects
3. **Evolution** - Can modify itself (unique)
4. **GitHub-native** - Built-in CI/CD integration

---

*Research compiled from: Aider docs, Claude Code docs, OpenCode source, Gemini CLI docs, Devin technical deep-dives, Goose architecture, Codex CLI, Amp, and 15+ comparison articles.*
