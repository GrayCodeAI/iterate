## Session Plan

Session Title: Persist safety config + fix merge conflict

### Task 1: Resolve DAY_COUNT merge conflict
Files: DAY_COUNT
Description: Fix the merge conflict markers in DAY_COUNT. The local value is 3, upstream is 0. Keep 3 since it reflects the actual evolution sessions that have run.
Issue: none

### Task 2: Wire /safe, /trust, /allow, /deny to persist config
Files: internal/commands/safety.go, cmd/iterate/config.go
Description: The safety commands (`/safe`, `/trust`, `/allow`, `/deny`) all have TODO comments saying "persist to config file". The `iterConfig` struct already has `SafeMode` and `DeniedTools` fields, and `saveConfig()` already handles TOML/JSON persistence. The fix: pass a `SaveConfig` callback through the command Context so each handler can persist after modifying state. Read the current config, update the relevant field, call saveConfig. This makes these commands survive REPL restarts.
Issue: none

### Issue Responses
- No community issues to address this session
