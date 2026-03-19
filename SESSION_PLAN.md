## Session Plan

Session Title: Add session cost tracking with /cost command

### Task 1: Implement token cost tracking
Files: cmd/iterate/repl.go, cmd/iterate/pricing.go, cmd/iterate/features.go
Description: Track input/output/cache tokens per session and calculate approximate cost based on provider rates. Add /cost slash command to display usage. Use existing sessionInputTokens/sessionOutputTokens variables in repl.go, add cost calculation functions in pricing.go using known rates for anthropic, openai, gemini, groq. Display in a formatted table showing tokens used and estimated cost.
Issue: none

### Task 2: Remove dead code
Files: cmd/iterate/commands_git.go
Description: Remove the duplicate containsString function (lines 167-173) that's already defined in repl.go. This function is unused and creates maintenance burden.
Issue: none

### Issue Responses
- No open community issues to address.
