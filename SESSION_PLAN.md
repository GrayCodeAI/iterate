## Session Plan

Session Title: Implement missing /go-def code intelligence command

### Task 1: Implement /go-def command for Go code navigation
Files:
- internal/commands/analysis.go (add actual implementation)
- internal/commands/register.go (ensure command is registered - already done)
- internal/commands/analysis_test.go (add comprehensive tests)
- internal/commands/files.go (add /go-def command registration)

Description:
The /go-def command was documented in Day 1's journal as implemented, but only helper functions exist. Add the actual implementation:

1. Add findSymbolDefinitions(repoPath, symbol string) function that:
   - Walks all .go files in the repo (excluding vendor/, .git/, node_modules/)
   - Uses go/parser.ParseFile with parser.AllErrors to be resilient to parse errors
   - Uses go/ast.Inspect to find FuncDecl, TypeSpec, ValueSpec nodes
   - Matches symbol name against declared identifiers
   - Extracts file path, line number (from token.Position), and signature
   - Handles functions (with params/results), types, methods (check receiver), variables, constants

2. Add cmdGoDef command handler in files.go that:
   - Takes symbol name as argument
   - Calls findSymbolDefinitions
   - Displays results with file:line format
   - If exactly one match, automatically injects context into agent conversation
   - If multiple matches, lists them with numbers and lets user pick
   - Shows "not found" message with suggestions if no matches

3. Add tests in analysis_test.go that verify:
   - Finding a function definition
   - Finding a type definition
   - Finding a method definition (with receiver matching)
   - Finding a variable declaration
   - Graceful handling of parse errors (still finds valid symbols)
   - Multiple matches returns all locations

Issue: none

### Task 2: Fix safety.go TODOs for config persistence
Files:
- internal/commands/safety.go
- cmd/iterate/config.go

Description:
The safety.go file has TODO comments about persisting denied tools to config file. Implement actual persistence:

1. Add functions to config.go: LoadDeniedTools(), SaveDeniedTools([]string)
2. In safety.go, call these functions in denyTool, allowTool
3. Ensure denied tools survive REPL restart

Issue: none

### Issue Responses
No open community issues to respond to today.
