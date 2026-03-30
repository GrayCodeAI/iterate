# iterate V2 Implementation Plan
## Based on Research of Top 20 AI Coding Agents

---

## Executive Summary

Research of top coding agents (Aider, Claude Code, Gemini CLI, Devin, OpenCode) reveals the key differences:

| Agent | Key Innovation | Success Rate |
|-------|---------------|--------------|
| **Aider** | Unified diffs + flexible patching | 88% self-written |
| **Claude Code** | MCP + tool calling | Best instruction following |
| **Gemini CLI** | ReAct loop + 1M context | Free, fast |
| **Devin** | Sandboxed execution + desktop use | 13.86% SWE-bench |
| **OpenCode** | Provider-agnostic | Multi-model support |

**Core Insight:** Aider's unified diff format works 3X better than custom SEARCH/REPLACE because:
1. LLMs are already trained on git diffs
2. No escaping/JSON overhead
3. Flexible error recovery when diffs fail

---

## Problem Analysis

### Current iterate Issues:
1. ❌ **LLM ignores SEARCH/REPLACE format** - Custom format unfamiliar to LLMs
2. ❌ **No error recovery** - Single diff failure = complete failure
3. ❌ **Rate limiting** - Single API key, no rotation at tool level
4. ❌ **No repo mapping** - LLM lacks full codebase context
5. ❌ **Test verification is post-hoc** - Tests should drive development

---

## Implementation Plan

### Phase 1: Fix Core Editing (Week 1)

#### 1.1 Replace SEARCH/REPLACE with Unified Diffs
**Why:** Aider showed 3X improvement using unified diffs over custom formats

**Implementation:**
```go
// Current (bad):
FILE: path/to/file.go
<<<<<<< SEARCH
old code
=======
new code
>>>>>>>

// New (good) - use standard unified diff:
--- a/path/to/file.go
+++ b/path/to/file.go
@@ -1,5 +1,7 @@
 func example() {
-    // old
+    // new
+    // added
     return
 }
```

**Files to modify:**
- `internal/evolution/prompts_aider.go` - Change format
- `internal/evolution/search_replace.go` - Rename to `diff_parser.go`

#### 1.2 Add Flexible Patching (Critical)
**Why:** Aider's 9X improvement with flexible error recovery

**Implementation:**
```go
func ApplyUnifiedDiff(content string, diff string) (string, error) {
    // Strategy 1: Standard patch
    // Strategy 2: Normalize whitespace
    // Strategy 3: Relative indentation matching
    // Strategy 4: Split into smaller hunks
    // Strategy 5: Expand context window
}
```

**Key techniques:**
- If hunk fails → try without line numbers
- If whitespace fails → normalize and retry
- If location fails → try fuzzy matching on content

#### 1.3 Multi-Model Support
**Why:** Different models follow instructions differently

**Implementation:**
```go
// Detect model and use optimal format
func GetEditFormatForModel(model string) string {
    switch {
    case strings.Contains(model, "claude"):
        return "json"  // Claude follows JSON well
    case strings.Contains(model, "gpt"):
        return "unified-diff"  // Aider's approach
    case strings.Contains(model, "gemini"):
        return "unified-diff"
    default:
        return "unified-diff"
    }
}
```

---

### Phase 2: Enhanced Context (Week 1-2)

#### 2.1 Repo Mapping (Like Aider)
**Why:** Aider's repo map dramatically improves LLM context

**Implementation:**
```go
type RepoMap struct {
    Files     []FileNode
    Functions []FunctionNode
    Classes   []ClassNode
}

type FileNode struct {
    Path     string
    Language string
    Size     int
    Functions []string  // function names
    Classes   []string  // class names
}
```

**Usage in prompts:**
```
## Repository Map
- internal/evolution/
  - engine.go (500 lines)
    - func: Run(), Execute(), Verify()
    - type: Engine, Config
  - phases.go (400 lines)
    - func: Plan(), Implement(), Review()
```

#### 2.2 Incremental Context Window
**Why:** Full context exceeds token limits

**Implementation:**
```go
// Send only relevant files based on task
func SelectRelevantFiles(task string, repoMap RepoMap) []string {
    // Use embeddings or keyword matching
    // Prioritize files mentioned in task
    // Add files that import/are imported by those files
}
```

---

### Phase 3: Tool-Level Rate Limiting (Week 2)

#### 3.1 Implement Retry with Backoff
**Why:** Current rotation is script-level, not tool-level

**Implementation:**
```go
func (e *Engine) callProviderWithRetry(ctx context.Context, req Request) (*Response, error) {
    maxRetries := 3
    baseDelay := 2 * time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        resp, err := e.provider.Call(ctx, req)
        
        if isRateLimit(err) {
            delay := baseDelay * math.Pow(2, float64(attempt))
            time.Sleep(delay)
            continue
        }
        
        return resp, err
    }
    return nil, ErrMaxRetriesExceeded
}
```

#### 3.2 Multi-Provider Fallback
**Why:** Swap providers when one fails

**Implementation:**
```go
type ProviderPool struct {
    providers []Provider
    current   int
}

func (p *ProviderPool) Call(ctx context.Context, req Request) (*Response, error) {
    for i := range p.providers {
        idx := (p.current + i) % len(p.providers)
        resp, err := p.providers[idx].Call(ctx, req)
        
        if !isProviderError(err) {
            p.current = idx
            return resp, err
        }
    }
    return nil, ErrAllProvidersFailed
}
```

---

### Phase 4: Test-First Workflow (Week 2)

#### 4.1 TDD Enforcement
**Why:** Tests should drive development, not be added after

**Implementation:**
```go
// In prompts, require test FIRST:
## Task: Fix bug in findTodos

### Step 1: Write Test (REQUIRED)
Create a test that FAILS with current code.
The test should demonstrate the bug.

### Step 2: Run Test
Verify it fails: `go test -v -run TestFindTodos`

### Step 3: Fix Code
Now fix the bug to make test pass.

### Step 4: Verify
Run: `go test -v -run TestFindTodos`
Must pass.
```

#### 4.2 Test Coverage Gates
**Why:** Prevent partial fixes

**Implementation:**
```go
func verifyTestDriven(ctx context.Context, changes []string) error {
    // 1. Verify test file was created/modified BEFORE code file
    testFiles := filterTestFiles(changes)
    codeFiles := filterNonTestFiles(changes)
    
    if len(testFiles) == 0 {
        return ErrNoTestChanges
    }
    
    // 2. Run tests - should pass
    if !runTests().Success {
        return ErrTestsFailed
    }
    
    // 3. Verify coverage didn't decrease
    if getCoverage() < getPreviousCoverage() {
        return ErrCoverageDecreased
    }
    
    return nil
}
```

---

### Phase 5: Safety & Verification (Week 2-3)

#### 5.1 Sandboxed Execution (Like Devin)
**Why:** Prevent destructive commands

**Implementation:**
```go
type SandboxConfig struct {
    AllowedCommands []string
    DisallowedPatterns []string
    Timeout time.Duration
}

var SafeConfig = SandboxConfig{
    AllowedCommands: []string{
        "go test", "go build", "go vet", "go fmt",
        "git status", "git diff", "git add",
        "ls", "cat", "grep", "find",
    },
    DisallowedPatterns: []string{
        "rm -rf", "curl | sh", "wget | sh",
        "git push --force", "dd of=/dev/",
    },
}

func (e *Engine) validateCommand(cmd string) error {
    for _, pattern := range SafeConfig.DisallowedPatterns {
        if strings.Contains(cmd, pattern) {
            return fmt.Errorf("command blocked: %s", pattern)
        }
    }
    return nil
}
```

#### 5.2 Pre-Merge Verification
**Why:** Catch issues before merging

**Implementation:**
```go
func preMergeVerification(ctx context.Context, prNumber int) error {
    // 1. Run full test suite
    if !runTests(ctx).Success {
        return ErrTestsFailed
    }
    
    // 2. Run linting
    if !runLinters(ctx).Success {
        return ErrLintingFailed
    }
    
    // 3. Verify no protected files changed
    if hasProtectedChanges(ctx) {
        return ErrProtectedFilesModified
    }
    
    // 4. Verify code coverage maintained
    if coverageDecreased(ctx) {
        return ErrCoverageDecreased
    }
    
    return nil
}
```

---

### Phase 6: Learning & Adaptation (Week 3)

#### 6.1 Failure Pattern Memory
**Why:** Don't repeat mistakes

**Implementation:**
```go
type FailurePattern struct {
    ErrorType     string
    Model         string
    FixSuggestion string
}

func (e *Engine) learnFromFailure(err error, model string) {
    pattern := FailurePattern{
        ErrorType: categorizeError(err),
        Model:     model,
        FixSuggestion: generateFixSuggestion(err),
    }
    
    // Save to memory
    saveToJSONL("memory/failure_patterns.jsonl", pattern)
    
    // Update next prompt with learned patterns
    e.addLearnedPatternsToPrompt()
}
```

#### 6.2 Model-Specific Tuning
**Why:** Different models need different prompts

**Implementation:**
```go
func GetSystemPromptForModel(model string) string {
    basePrompt := loadBasePrompt()
    
    switch {
    case strings.Contains(model, "claude"):
        // Claude follows instructions well, use concise format
        return basePrompt + "\n\nUse JSON for file edits."
        
    case strings.Contains(model, "gpt-4"):
        // GPT-4 works best with unified diffs
        return basePrompt + "\n\nUse unified diff format for edits."
        
    case strings.Contains(model, "gemini"):
        // Gemini benefits from step-by-step
        return basePrompt + "\n\nThink step by step."
        
    default:
        return basePrompt
    }
}
```

---

## Detailed File Changes

### Files to Create:
1. `internal/evolution/diff_parser.go` - Unified diff parsing
2. `internal/evolution/flexible_patch.go` - Error recovery
3. `internal/evolution/repo_map.go` - Repository mapping
4. `internal/evolution/provider_pool.go` - Multi-provider support
5. `internal/evolution/sandbox.go` - Command sandboxing
6. `scripts/evolution/test_first_prompt.go` - TDD prompts

### Files to Modify:
1. `internal/evolution/prompts_aider.go` - Use unified diffs
2. `internal/evolution/phases.go` - Add verification gates
3. `scripts/evolution/evolve.sh` - Already has rotation

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Code changes per evolution | ~3 | 5+ |
| Test inclusion rate | 50% | 100% |
| Build/test pass rate | 70% | 95% |
| SEARCH/REPLACE success | 0% | 80%+ |
| Rate limit recovery | Manual | Auto |

---

## Priority Order

### Must Fix (This Week):
1. ✅ Unified diff format (replaces SEARCH/REPLACE)
2. ✅ Flexible patching (error recovery)
3. ✅ Repo mapping (better context)

### Should Fix (Next Week):
4. Multi-provider fallback
5. Test-first prompts
6. Pre-merge verification

### Nice to Have (Week 3):
7. Sandboxed execution
8. Failure pattern learning
9. Model-specific tuning

---

## Key Takeaways from Research

1. **Use familiar formats** - Unified diffs are 3X better than custom formats
2. **Be flexible** - 9X improvement with error recovery
3. **Provide context** - Repo mapping improves LLM understanding
4. **Multi-provider** - Rate limits happen, have backups
5. **Test-first** - TDD prevents bugs, not post-hoc testing
6. **Verify everything** - Pre-merge gates catch failures

---

*Based on research of Aider (42K stars), Claude Code, Gemini CLI (99K stars), Devin, and OpenCode. Key insight: Format familiarity matters more than format sophistication.*
