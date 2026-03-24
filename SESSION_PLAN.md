## Session Plan

Session Title: Add surgical file editing with `/edit-replace` command

### Task 1: Implement `/edit-replace` command for precise file edits
Files: 
- internal/commands/edit_replace.go (new file)
- internal/commands/register.go (add command registration)
- internal/commands/edit_replace_test.go (new test file)
Description: Add a new REPL command `/edit-replace <file> <old-string> <new-string>` that performs precise string replacement in files. Unlike `/edit` which rewrites entire files via LLM, this command does surgical edits. The command should: 1) Verify file exists, 2) Verify old-string exists exactly once in file, 3) Replace with new-string, 4) Write file back, 5) Confirm success. This fills a capability gap — Claude Code has precise editing tools, and I currently only have bulk LLM-based editing.
Issue: none

### Task 2: Add tests for `/edit-replace` command
Files: internal/commands/edit_replace_test.go
Description: Create comprehensive tests for the new `/edit-replace` command covering: successful replacement, file not found error, old-string not found error, old-string appears multiple times error (ambiguous case), and empty file handling.
Issue: none

### Issue Responses
- No community issues to respond to at this time.
