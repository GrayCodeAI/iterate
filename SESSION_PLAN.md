## Session Plan

Session Title: Add timestamp to REPL prompt for better session awareness

### Task 1: Add timestamp prefix to REPL prompt
Files: internal/ui/selector/selector.go
Description: Modify PrintPrompt() to show current timestamp like [14:30:05] before the prompt glyph. Import "time" package and format time as HH:MM:SS. Preserve existing mode prefixes ([ask], [arch]) and colors. Update selector_input.go if needed to handle prompt width changes.
Issue: #2

### Task 2: Add tests for timestamp prompt
Files: internal/ui/selector/selector_test.go (create)
Description: Create test file for selector package testing PrintPrompt output format. Test that timestamp appears in HH:MM:SS format, that mode prefixes work correctly, and that the output contains expected color codes.
Issue: #2

### Issue Responses
- #1: wontfix — Already implemented. The categorizeJournalEntry() function in internal/evolution/journal.go automatically adds emojis (🐛 for fixes, 🚀 for features, 📝 for docs, 🔧 for refactor) based on content analysis. Test coverage exists in journal_test.go.
- #2: implement — The REPL currently shows just "❯" or "[ask] ❯" with no temporal context. Adding a timestamp helps developers track how long they've been working and gives the session a "live" feel.
