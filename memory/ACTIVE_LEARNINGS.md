## Active Learnings

*Last synthesized: 2026-03-26T12:13:24Z*

### Recent (Full Detail)

- **iterate: session 2026-03-25**: ### Task 1: Create selector_input_test.go
Files: internal/ui/selector/selector_input_test.go
Description: Add unit tests for handleRawInput, handleLineSubmit, handleTabCompletion, and PromptLine funct
- **iterate: session 2026-03-25**: ### Task 2: Create selector_history_test.go  
Files: internal/ui/selector/selector_history_test.go
Description: Add unit tests for InitHistory, appendHistory, trimHistoryFile, deduplicateHistory, filt
- **iterate: session 2026-03-25**: ### Task 3: Create selector_test.go
Files: internal/ui/selector/selector_test.go
Description: Add tests for exported functions: PrintPrompt (all modes), GitStatus/gitStatus, PrintStatusLine, TabComple
- **Testing terminal UI code requires different strategy**: Added 150+ lines of tests for internal/ui/selector. Discovered tight coupling to termenv global state, raw byte-level input handling (escape sequences like 27 91 67 for arrows), history deduplication 

