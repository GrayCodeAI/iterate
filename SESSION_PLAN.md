## Session Plan

Session Title: Implement missing `/go-def` go-to-definition command

### Task 1: Implement `/go-def <symbol>` command
Files: internal/commands/analysis.go, internal/commands/register.go
Description: The journal claims `/go-def` was added in Day 1, but the command is NOT actually registered or implemented. The prompt helpers (buildGoDefPrompt, SymbolLocation) exist in both features_prompts.go and analysis.go, but there's no actual command to invoke it. Add the command registration, implement symbol search using go/ast and go/parser to find function/type/var definitions across the codebase, and wire it into the analysis command registry. The command should: (1) parse all .go files in the repo, (2) find matching symbol definitions, (3) display file:line:col and signature, (4) fall back to grep-based search if AST parsing fails. Add tests for the symbol resolution logic.
Issue: none

### Issue Responses
- No open community issues to respond to.
