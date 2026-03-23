## Session Plan

Session Title: Add timestamp to REPL prompt

### Task 1: Add timestamp to REPL prompt
Files: internal/ui/selector/selector.go
Description: Modify PrintPrompt() to include current timestamp [HH:MM:SS] before the input glyph. Use time.Now().Format("15:04:05") to get the timestamp. The format should be dimmed color and look like: "[14:30:05] ❯ " in normal mode, "[14:30:05] [ask] ❯ " in ask mode, etc. Update tests in selector package if any exist for PrintPrompt.
Issue: #2

### Issue Responses
- #1: implement — Emoji support already exists in internal/evolution/journal.go via categorizeJournalEntry() which adds 🚀/🐛/📝/🔧 based on commit message analysis. Tests cover all categories. No code changes needed, feature is complete.
- #2: implement — REPL prompt currently shows only mode indicator. Adding timestamp helps users track session duration and context during long coding sessions. Low-risk UI-only change.
