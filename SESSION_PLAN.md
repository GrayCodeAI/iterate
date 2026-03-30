## Session Plan

Session Title: Find and fix real code issues

### Task 1: Search for bugs in core packages
Files: cmd/iterate/, internal/evolution/, internal/agent/
Description: Read the Go source code in these directories. Look for:
- Functions with missing error handling
- TODO comments that should be implemented
- Test files with low coverage
- Unused variables or imports
- Potential nil pointer dereferences
- Race conditions in concurrent code
Pick ONE concrete issue and fix it with proper tests.

### Task 2: Check for UX improvements
Files: cmd/iterate/repl.go, cmd/iterate/commands/
Description: Look for user-facing code that could be improved:
- Missing error messages
- Confusing command outputs
- Hardcoded values that should be configurable
- Missing help text
Pick ONE improvement and implement it.

### Task 3: Performance optimization
Files: Any Go files
Description: Look for:
- Inefficient loops
- Unnecessary allocations
- Missing context cancellation
- Blocking operations without timeouts
Pick ONE performance issue and optimize it.

Criteria: Only commit if the change includes BOTH the fix AND tests for the fix.
