## Session Plan

Session Title: Add tests for internal/ui/selector package

### Task 1: Create selector_input_test.go
Files: internal/ui/selector/selector_input_test.go
Description: Add unit tests for handleRawInput, handleLineSubmit, handleTabCompletion, and PromptLine functions. Test key scenarios: Enter submission, Ctrl+C cancellation, backspace handling, tab completion triggering. Mock terminal state where needed.
Issue: none

### Task 2: Create selector_history_test.go  
Files: internal/ui/selector/selector_history_test.go
Description: Add unit tests for InitHistory, appendHistory, trimHistoryFile, deduplicateHistory, filterHistoryEntries. Test scenarios: history loading/saving, duplicate prevention, file trimming at maxHistoryLines limit, fuzzy filtering case-insensitivity.
Issue: none

### Task 3: Create selector_test.go
Files: internal/ui/selector/selector_test.go
Description: Add tests for exported functions: PrintPrompt (all modes), GitStatus/gitStatus, PrintStatusLine, TabComplete, TabCompleteWithArgs, SelectItem, CompleteFilePath. Test tab completion logic for slash commands and file paths.
Issue: none

### Issue Responses
- No open issues tagged with agent-input, agent-help-wanted, or agent-self
- internal/ui/selector package has zero test coverage despite 500+ lines of non-trivial code for terminal UI, input handling, and history management
- Safety config persistence TODOs (4x in safety.go) are feature gaps not bugs — lower priority than testing uncovered code
