## Session Plan

Session Title: Add timestamp prefix to REPL prompt

### Task 1: Add timestamp to REPL prompt
Files: internal/ui/selector/selector.go
Description: Modify PrintPrompt() to include a timestamp prefix [HH:MM:SS] before the prompt glyph. Use the current time formatted as 15:04:05. Add a new color variable for timestamps (colorTime, yellowish dim). The timestamp should appear before the mode indicator in all modes (normal, ask, architect). Format: "[14:30:05] ❯ " or "[14:30:05] [ask] ❯ " etc.
Issue: #2

### Task 2: Add tests for timestamp in prompt
Files: internal/ui/selector/selector_test.go (new file)
Description: Create a new test file that verifies PrintPrompt() outputs contain a timestamp in the expected format. Test that the timestamp regex matches [HH:MM:SS] pattern. Create a helper function to capture the prompt output and verify the timestamp is present.
Issue: none

### Task 3: Update documentation for new prompt feature
Files: docs/JOURNAL.md
Description: Document that the REPL now shows timestamps in the prompt to help track session duration.
Issue: none

### Issue Responses
- #1: wontfix — Emoji support is already implemented in internal/evolution/journal.go via categorizeJournalEntry() which adds 🚀/🐛/📝/🔧 based on commit type. The journal system already has emoji categorization.
- #2: implement — The REPL prompt currently shows just "❯" with optional mode prefixes. Adding a timestamp prefix [HH:MM:SS] will help users track how long they've been working in a session, which is especially useful for long-running evolution sessions.
