---
name: evolve
description: Safely modify your own source code
scope: [self-modification, testing, git]
---

# Evolve Skill

You can safely modify iterate's own source code. Follow these rules:

## Safety Rules

1. **Always test after changes**: `go build ./... && go test ./...`
2. **Never delete tests**: Only add or modify, never remove existing tests
3. **Commit incrementally**: Small, focused commits for each change
4. **Revert on failure**: If tests fail, `git checkout -- .` and try again
5. **Protect core files**:
   - Never modify `IDENTITY.md`
   - Never modify `PERSONALITY.md`
   - Never modify `scripts/evolve.sh`
   - Never modify `.github/workflows/`

## Implementation Steps

1. Read the relevant source file with `/view` or tool
2. Understand the current behavior
3. Make focused changes (one concern per commit)
4. Run tests immediately
5. If tests pass → commit with clear message
6. If tests fail → revert and analyze the failure

## Common Improvement Areas

- Add new slash commands to cmd/iterate/repl.go
- Enhance features.go helper functions
- Improve selector.go input handling
- Add syntax highlighting patterns to highlight.go
- Extend prompt.go system prompt
- Add new providers to iteragent integration

## Example Session

```
User: improve the /optimize command with better prompts
Agent: 
  1. Read cmd/iterate/repl.go to find /optimize case
  2. Read features.go to understand buildOptimizePrompt
  3. Enhance the prompt with specific advice
  4. Commit: "enhance: /optimize command with detailed guidance"
  5. Tests pass → done
```
