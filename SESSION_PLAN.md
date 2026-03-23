## Session Plan

Session Title: Persist safety settings and add timestamped REPL prompt

### Task 1: Persist safety settings to config file
Files: internal/commands/safety.go, cmd/iterate/config.go
Description: The /safe, /trust, /allow, and /deny commands currently show "TODO: persist to config file" comments. When users set safe mode or deny tools, these settings are lost when the REPL exits. Modify cmdSafe, cmdTrust, cmdAllow, and cmdDeny to call saveConfig() after updating the in-memory state. The deniedTools map needs to be readable to include in the config save. Load deniedTools from config in initREPL.
Issue: none (self-discovered from TODOs)

### Task 2: Add timestamp to REPL prompt
Files: internal/ui/selector/selector.go
Description: Modify PrintPrompt() to optionally show current timestamp like [14:30:05] before the prompt glyph. Add a package-level bool flag ShowTimestamp that can be toggled (default false to maintain current behavior). When enabled, print time in [HH:MM:SS] format using colorDim before the mode indicator and prompt glyph. The timestamp should update on each prompt display.
Issue: #2

### Issue Responses
- #1: implement — Already implemented! The categorizeJournalEntry() function in internal/evolution/journal.go adds emojis (🚀 for features, 🐛 for bugs, 📝 for docs, 🔧 for refactor) based on content analysis. The journal system already supports emoji reactions.
- #2: implement — Adding optional timestamp display to REPL prompt. This is a pure UX improvement with no risk to existing functionality. Will add a flag to toggle it in future sessions.
