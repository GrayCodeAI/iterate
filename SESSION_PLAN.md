## Session Plan

Session Title: Add Go-to-Definition Code Intelligence

### Task 1: Implement /go-def command for Go code navigation
Files: 
- internal/commands/analysis.go (extend existing analysis commands)
- internal/commands/register.go (ensure command is registered)
- internal/commands/analysis_test.go (add tests)
- cmd/iterate/features_prompts.go (add prompt builder)

Description: 
Add a `/go-def <symbol>` command that finds where a Go symbol (function, type, variable, method) is defined in the codebase. Use Go's standard library `go/ast`, `go/parser`, and `go/token` packages to parse source files and build a minimal symbol index. The command should:
1. Accept a symbol name as argument
2. Search all .go files in the repo for definitions of that symbol
3. Return the file path, line number, and signature of the definition
4. Handle functions, types, methods, and variables
5. Fall back gracefully if the repo has parse errors

Add comprehensive tests that verify the command can find:
- Function definitions
- Type definitions  
- Method definitions on types
- Variable declarations

This closes the biggest capability gap with Claude Code: semantic code understanding.

Issue: none

### Task 2: Add code intelligence prompt helpers
Files:
- cmd/iterate/features_prompts.go
- internal/commands/analysis.go

Description:
Add helper functions that build intelligent prompts for code analysis:
1. `buildGoDefPrompt(symbol, location)` - builds a prompt asking the LLM to explain a found definition
2. Extend the `/ask` mode to include code intelligence context

These helpers enable deeper code understanding workflows.

Issue: none

### Issue Responses
No community issues to respond to today.
