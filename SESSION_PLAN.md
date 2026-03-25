## Session Plan

Session Title: Persist safety settings to config file

### Task 1: Add config persistence to safety commands
Files: 
- internal/commands/safety.go (modify cmdSafe, cmdTrust, cmdAllow, cmdDeny)
- internal/commands/safety_test.go (add tests for persistence)

Description:
The `/safe`, `/trust`, `/allow`, and `/deny` commands currently only modify in-memory state. They need to persist changes to the config file so settings survive REPL restarts.

The `/notify` command in `internal/commands/config.go` (lines 206-221) already demonstrates the correct pattern:
1. Check if `ctx.Config.LoadConfig` and `ctx.Config.SaveConfig` are available
2. Load the current config using `ctx.Config.LoadConfig()`
3. Modify the appropriate field
4. Save with `ctx.Config.SaveConfig(cfg)`

For safety commands:
- `cmdSafe`: Set `cfg.SafeMode = true`, save to `DeniedTools` if needed
- `cmdTrust`: Set `cfg.SafeMode = false`
- `cmdAllow`: Remove tool from `cfg.DeniedTools` slice
- `cmdDeny`: Add tool to `cfg.DeniedTools` slice

The config struct in `cmd/iterate/config.go` already has `SafeMode bool` and `DeniedTools []string` fields.

Implementation details:
- Type assert the loaded config to `iterConfig` (or use the concrete type if accessible)
- Handle case where LoadConfig/SaveConfig callbacks are nil (skip persistence, don't error)
- For DeniedTools, avoid duplicates when adding, properly remove when allowing
- Keep existing in-memory updates (they affect immediate behavior)

Testing:
- Add tests that verify Config.LoadConfig/SaveConfig are called with correct values
- Test that persistence is skipped gracefully when callbacks are nil
- Test duplicate prevention in DeniedTools

### Issue Responses

- none: This is a self-identified improvement from codebase TODOs (lines 54, 63, 77, 91 in internal/commands/safety.go)

### Success Criteria

- [ ] `/safe` persists SafeMode=true to config
- [ ] `/trust` persists SafeMode=false to config  
- [ ] `/deny <tool>` persists the denied tool to config
- [ ] `/allow <tool>` removes the tool from denied list in config
- [ ] All existing tests still pass
- [ ] New tests cover persistence behavior
- [ ] The four TODO comments are removed
