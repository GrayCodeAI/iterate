## Session Plan

Session Title: Add /edit command for natural language file editing

### Task 1: Implement /edit command
Files: 
- internal/commands/edit.go (new file)
- internal/commands/register.go (register the new command)
- internal/commands/edit_test.go (new file)
Description: Create `/edit <file> <instruction>` command that reads a file, uses AI to apply natural language instruction, and writes the result. Must validate file exists, show diff preview before applying, handle errors gracefully, and include comprehensive tests.
Issue: none

### Task 2: Add edit prompt builder
Files:
- internal/prompts/features_prompts.go (extend)
Description: Add `buildEditPrompt(fileContent, instruction string) string` helper that constructs a prompt for the AI to perform file edits. Include the full file content, the instruction, and constraints (preserve formatting, minimal changes, return complete file).
Issue: none

### Issue Responses
No community issues found in repository.
