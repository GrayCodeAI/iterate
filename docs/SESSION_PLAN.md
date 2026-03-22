## Session Plan

Session Title: Add REPL timestamp + persist safety config

### Task 1: Add timestamp to REPL prompt prefix
Files: cmd/iterate/repl.go, internal/ui/selector/selector_input.go
Description: Display current time like [14:30:05] before the REPL prompt to help users track session duration. Modify the prompt display in selector_input.go to prepend a dimmed timestamp when reading input.
Issue: #2

### Task 2: Persist safety state to config on /safe, /trust, /allow, /deny
Files: internal/commands/safety.go, internal/commands/registry.go
Description: The four safety commands (/safe, /trust, /allow, /deny) change in-memory state but don't persist. Wire up ConfigCallbacks.SaveConfig in buildCommandContext and call it from each handler after state changes. Resolves four TODO comments in safety.go.
Issue: none

### Issue Responses
- #1: wontfix — emoji categorization already implemented in internal/evolution/journal.go via `categorizeJournalEntry()` which adds 🚀, 🐛, 📝, 🔧 based on content analysis
- #2: implement — adding timestamp to REPL prompt prefix for better session time tracking
