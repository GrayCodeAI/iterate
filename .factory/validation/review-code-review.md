# Deep Code Review — Iterate (github.com/GrayCodeAI/iterate)

## Summary

Reviewed all non-test Go source files in `cmd/iterate/` and `internal/`. Below are confirmed bugs and issues, organized by severity.

---

## CRITICAL

### 1. File Descriptor Leak — `countLines()` 
**File:** `cmd/iterate/features_shell.go` — `countLines()` (≈ lines 78–97)

```go
f, err := os.Open(path)
if err != nil {
    return nil
}
defer f.Close()
```

`defer f.Close()` is called inside a `filepath.WalkDir` callback. Since defer statements only execute when the **enclosing function** returns (not when the loop iteration ends), this never closes files during the walk. A repository with hundreds of files will exhaust file descriptors.

**Fix:** Call `f.Close()` at the end of the callback body (before `return nil`), not via defer.

---

## HIGH

### 2. PushFileEdit Returns Pointer to Stale Copy
**File:** `internal/autonomous/rollback.go` — `PushFileEdit()` (≈ lines 93–113)

```go
func (rs *RollbackStack) PushFileEdit(path string, originalContent string) *RollbackEntry {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    entry := RollbackEntry{...}
    // Save backup to disk — modifies entry.Metadata
    if rs.backupDir != "" {
        ...
        entry.Metadata["backup_path"] = backupPath  // modifies local copy
    }
    rs.pushEntry(entry)    // pushes a COPY of entry to the slice
    return &entry           // returns pointer to local copy (NOT the one in slice)
}
```

The returned `*RollbackEntry` points to a copy that is NOT stored in the entries slice. Any modifications the caller makes through this pointer (e.g., `entry.Metadata["foo"] = "bar"`) are lost because the slice holds a separate copy. The same issue exists in `PushFileCreate()`, `PushFileDelete()`.

**Fix:** Store the entry as a pointer (`[]*RollbackEntry`) in `RollbackStack.entries`, or return a pointer to the element inside the slice instead of the local copy.

### 3. PushFileCreate Does Not Save Backup Content — Rollback Always Fails
**File:** `internal/autonomous/rollback.go` — `PushFileCreate()` (≈ lines 115–128)

```go
func (rs *RollbackStack) PushFileCreate(path string) *RollbackEntry {
    entry := RollbackEntry{
        Type:  RollbackTypeFileCreate,
        Path:  path,
        // NOTE: Original is never set — it defaults to ""
    }
    // NOTE: No backup file is written (unlike PushFileEdit)
    rs.pushEntry(entry)
    return &entry
}
```

`rollbackFileCreate()` calls `os.Remove(entry.Path)` to undo a file creation. However, if the path was never actually created (the caller creates the file AFTER calling `PushFileCreate`), or if the file was already deleted, `os.Remove()` returns an error. But more importantly, the matching `PushFileEdit()` writes a backup to disk; `PushFileCreate()` does not, which is inconsistent with the backup strategy.

**Fix:** Either accept a copy of the created file content and store it (like `PushFileEdit` does), or document that the cleanup strategy for PushFileCreate is best-effort delete.

### 4. Config TOML Fallback Doesn't Actually Fall Back to JSON
**File:** `cmd/iterate/config.go` — `loadConfig()` (≈ lines 87–97)

```go
if data, err := os.ReadFile(configPathTOML()); err == nil {
    if err := toml.Unmarshal(data, &cfg); err != nil {
        slog.Warn("failed to parse TOML config, falling back to JSON", "err", err)
        // No actual fallback — cfg stays empty, code continues
    }
} else {
    // JSON fallback only runs when TOML file doesn't exist (read error)
    for _, path := range []string{configPathAlt(), configPath()} {
        ...
    }
}
```

If the TOML config file exists but is corrupt, the code logs a warning saying "falling back to JSON" but actually **doesn't** fall back — `cfg` remains all zero-values. The `else` branch (JSON loading) is only reachable when the TOML file **cannot be read** (not when parsing fails).

**Fix:** On TOML parse error, execute the JSON fallback logic instead of just warning.

### 5. findTestedFile Loses Directory Path
**File:** `internal/context/related_files.go` — `findTestedFile()` (≈ lines 345–360)

```go
func findTestedFile(testPath string) string {
    base := testPath
    if idx := strings.LastIndex(testPath, "/"); idx >= 0 {
        base = testPath[idx+1:]  // Only takes the filename, drops the directory
    }
    // ... processing on base ...
    return strings.TrimSuffix(base, suffix) + ext  // Returns just "related_files.go", not "internal/context/related_files.go"
}
```

Given input `internal/context/related_files_test.go`, this returns `related_files.go` instead of `internal/context/related_files.go`. The directory path is lost.

**Fix:** Compute the directory once and rejoin with the processed filename:
```go
dir := ""
if idx := strings.LastIndex(testPath, "/"); idx >= 0 {
    dir = testPath[:idx+1]
    base = testPath[idx+1:]
}
// ...
return dir + strings.TrimSuffix(base, suffix) + ext
```

### 6. Safe Mode Strict Approval Always Auto-Approves
**File:** `internal/autonomous/autonomous.go` — `waitForApproval()` (≈ lines 340–346)

```go
func (e *Engine) waitForApproval(step PlanStep) bool {
    if e.config.SafetyMode == SafetyPermissive {
        return true
    }
    // In strict mode, would prompt user - for now, auto-approve
    return true  // <-- Auto-approves even in SafetyStrict mode!
}
```

Despite `SafetyStrict` being documented as "Requires approval for all file changes", this function always returns `true`, effectively bypassing any approval mechanism. The comment acknowledges this is a stub. In a CLI intended for autonomous coding, this means **no file change is actually gated by approval**, regardless of safety level.

**Fix:** Either implement actual user prompting (e.g., via stdin) or document that `waitForApproval` is not yet functional and `SafetyStrict` is not enforced.

---

## MEDIUM

### 7. `PushFileEdit`/`PushFileCreate`/`PushFileDelete` — Missing os.MkdirAll for Backup Dirs
**File:** `internal/autonomous/rollback.go` — `PushFileEdit` (≈ line 104), `PushFileDelete` (≈ line 145)

```go
backupPath := filepath.Join(rs.backupDir, entry.ID+filepath.Ext(path))
if err := os.WriteFile(backupPath, []byte(originalContent), 0644); err != nil {
    // Error is stored in metadata but backup dir may not exist
}
```

The `NewRollbackStack` only calls `os.MkdirAll(rs.backupDir, 0755)` if `config.BaseDir != ""`. If `BaseDir` is empty, `backupDir` is empty and writes to `.go` or similar will fail silently (the error is recorded in metadata but the operation silently loses the backup).

**Fix:** Ensure `backupDir` is created before WriteFile, or skip backup silently with clearer logging.

### 8. `checkBashPermission` — Prefix Match Has False Positives
**File:** `cmd/iterate/config.go` — `checkBashPermission()` (≈ lines 168–178)

```go
if globMatch(p, cmd) || (len(cmd) >= len(p) && globMatch(p, cmd[:len(p)])) {
```

The prefix match `cmd[:len(p)]` can produce surprising results. If the deny pattern is `"rm "` (length 3) and the command is `"rm-rf"` (no space), `cmd[:3]` = `"rm-"` and `globMatch("rm ", "rm-")` would NOT match. But if the user intends to deny `rm `, the prefix check is redundant with the word-splitting logic below. The prefix check adds complexity without clear benefit and could have edge cases.

**Fix:** Remove the prefix match clause since word-splitting handles command prefix matching more correctly.

### 9. `handleSafeModePrompt` — `cfg` is Passed by Value — AllowPatterns Changes Are Lost
**File:** `cmd/iterate/features_tools.go` — `handleSafeModePrompt()` (≈ lines 245–265)

```go
func handleSafeModePrompt(cfg iterConfig, tool iteragent.Tool, ...) (string, bool) {
    if tool.Name == "bash" {
        if allowed, denied := checkBashPermission(cfg, cmd); allowed {
            ...
        }
    }
    ...
    if ans == "always" {
        if tool.Name == "bash" {
            cfg.AllowPatterns = append(cfg.AllowPatterns, cmd)  // Modifies local copy!
        } else {
            allowTool(tool.Name)  // This is fine — modifies global map
        }
    }
```

When user selects "always" for a bash command, the new pattern is appended to the local `cfg` copy which is never written back to disk or the global config. The pattern is forgotten on the next tool call.

**Fix:** Call `saveConfig(cfg)` after appending, or use a pointer to the global config.

### 10. `wrapToolsWithPermissions` — `cfg` Captured Once at Wrap Time
**File:** `cmd/iterate/features_tools.go` — `wrapToolsWithPermissions()` (≈ lines 204–211)

```go
func wrapToolsWithPermissions(tools []iteragent.Tool) []iteragent.Tool {
    cfg := loadConfig()  // Loaded ONCE when wrapping
    ...
    t.Execute = func(...) {
        if denied := checkToolDirPermission(cfg, ...); denied != "" {
```

The config is captured at tool wrap time. If the user changes config at runtime (e.g., `/config` command, or editing the config file), the tools still use the stale `cfg` captured at startup.

**Fix:** Call `loadConfig()` inside the Execute closure, not outside it.

### 11. `NewSandboxBuilder.WithVolumeMount` and `WithEnvVar` — Mutates Shared Config
**File:** `internal/autonomous/sandbox.go` — `WithVolumeMount` (≈ lines 405), `WithEnvVar` (≈ lines 420)

```go
func (b *SandboxBuilder) WithEnvVar(key, value string) *SandboxBuilder {
    if b.config.EnvVars == nil {
        b.config.EnvVars = make(map[string]string) // Only nil-checks, doesn't copy
    }
    b.config.EnvVars[key] = value
    return b
}
```

If someone calls `NewSandboxBuilder()` and then shares the config or builder, mutations affect the original. Not critical but a common builder pattern issue.

**Fix:** Clone the map when creating the builder.

---

## LOW

### 12. `resolveGoImport` Doesn't Check for `.go` Files in Directory Resolution
**File:** `internal/context/dependency_analyzer.go` — `resolveGoImport()` (≈ lines 291–308)

When the import resolves to a directory, it just returns the directory path, but Go packages may have multiple `.go` files. The caller treats this as a single file path, so it won't appear in the file index.

### 13. `analyzeGoFile` Reuses Token FileSet Across Files
**File:** `internal/context/dependency_analyzer.go` — `analyzeGoFile()` and `NewDependencyAnalyzer()`

```go
func NewDependencyAnalyzer(...) *DependencyAnalyzer {
    return &DependencyAnalyzer{
        fset: token.NewFileSet(),  // Single shared FileSet
    }
}
```

The `token.FileSet` is shared across all files analyzed. This means position line numbers from different files may have collisions in the position space. Typically each file should get its own FileSet, or at minimum the caller should be aware that positions refer to the cumulative position space, not per-file. However, `parser.ParseFile` does add files with proper offsets, so this may work correctly — it's non-standard and confusing.

### 14. `extractEmbedDirectives` — Regex-Like Matching Is Fragile
**File:** `internal/context/dependency_analyzer.go` — `extractEmbedDirectives()` (≈ lines 310–340)

The code splits by newlines and does `strings.Contains(line, "//go:embed")`, but Go embed directives can span multiple lines or use `/* */` comments. This would miss or misparse those cases.

### 15. `executeSwarmAgents` Uses `context.Background()` — No Cancellation Support
**File:** `internal/commands/agent.go` — `executeSwarmAgents()` (≈ lines 290–310)

Each agent in the swarm is spawned with `context.Background()`, which means:
- No timeout on individual agents
- If the caller cancels their context, spawned agents keep running
- No way to stop the swarm mid-execution

**Fix:** Accept a context parameter and pass it to agent operations.

### 16. `cmdCherryPick` — No Validation of Git Argument
**File:** `internal/commands/git.go` — `cmdCherryPick()`

```go
ctx.REPL.RunShell(ctx.RepoPath, "git", "cherry-pick", ctx.Arg(1))
```

The commit-hash argument is passed directly without validation. If it starts with `-`, git interprets it as a flag rather than a commit, which could lead to unintended behavior (e.g., `cherry-pick -n` commits without creating a commit).

**Fix:** Validate the argument doesn't start with `-`.

---

## File Summary by Issue

| # | File | Lines | Severity | Issue |
|---|------|-------|----------|-------|
| 1 | `cmd/iterate/features_shell.go` | ~91 | **CRITICAL** | File descriptor leak in countLines |
| 2 | `internal/autonomous/rollback.go` | ~113 | **HIGH** | PushFileEdit returns stale copy pointer |
| 3 | `internal/autonomous/rollback.go` | ~127 | **HIGH** | PushFileCreate doesn't save backup content |
| 4 | `cmd/iterate/config.go` | ~92 | **HIGH** | TOML fallback doesn't actually fall back to JSON |
| 5 | `internal/context/related_files.go` | ~355 | **HIGH** | findTestedFile loses directory path |
| 6 | `internal/autonomous/autonomous.go` | ~345 | **HIGH** | SafetyStrict approval always auto-approves |
| 7 | `internal/autonomous/rollback.go` | ~104 | **MEDIUM** | Missing os.MkdirAll for backup dirs |
| 8 | `cmd/iterate/config.go` | ~174 | **MEDIUM** | Prefix match in checkBashPermission has false positives |
| 9 | `cmd/iterate/features_tools.go` | ~258 | **MEDIUM** | cfg passed by value — AllowPatterns changes are lost |
| 10 | `cmd/iterate/features_tools.go` | ~204 | **MEDIUM** | cfg captured once at wrap time — stale config |
| 11 | `internal/autonomous/sandbox.go` | ~420 | **MEDIUM** | Builder mutates shared config |
| 12 | `internal/context/dependency_analyzer.go` | ~300 | **LOW** | resolveGoImport doesn't handle file-level resolution |
| 13 | `internal/context/dependency_analyzer.go` | ~72 | **LOW** | Shared token.FileSet across files |
| 14 | `internal/context/dependency_analyzer.go` | ~310 | **LOW** | Embedded directive parsing is fragile |
| 15 | `internal/commands/agent.go` | ~290 | **LOW** | Swarm agents use context.Background() |
| 16 | `internal/commands/git.go` | ~189 | **LOW** | Cherry-pick arg not validated |

---

## Notes on What Was Reviewed

- All non-test Go files in `cmd/iterate/` and `internal/` were reviewed.
- Focused on: logic errors, nil pointer risk, resource leaks, incorrect error handling, race conditions, security issues, and dead/unreachable code.
- Did NOT review: test files, coding style issues, naming preferences, or documentation quality.
- The iteragent dependency (external) was not reviewed.
