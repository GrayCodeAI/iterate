---
name: self-assess
description: Deeply assess your own codebase at the start of every evolution session
tools: [bash, read_file, list_files]
---

# Self-Assessment

Use this skill at the start of every evolution session — before planning, before implementing.

## Steps

### 1. Read your source code completely
- `list_files` on `cmd/` and `internal/` recursively
- Read every `.go` file in `cmd/iterate/` — this is your REPL and entry point
- Read `internal/evolution/engine.go` — this is how you evolve
- Read `internal/community/` and `internal/social/` — these are your community connections

### 2. Try using yourself
Don't just read code — **execute it** to find friction:
- Run `go build ./...` — does it compile cleanly?
- Run `go test ./...` — how many tests pass? Any failing?
- Run `go vet ./...` — any static analysis warnings?
- Look at test coverage gaps: which packages have low coverage?

### 3. Check your history
- Read `JOURNAL.md` — what did you attempt recently? Don't repeat it.
- Read `memory/active_learnings.md` — what have you already learned?
- Check `memory/learnings.jsonl` for lessons from past sessions

### 4. Look for what's broken or missing
Scan for these in order of severity:

| Priority | What to look for |
|----------|-----------------|
| 🔴 Critical | Crashes, panics, data loss, broken builds |
| 🟠 High | Unhandled errors, missing error messages, silent failures |
| 🟡 Medium | Missing tests for existing features, edge cases not covered |
| 🟢 Low | Code clarity, performance, UX friction in the REPL |

**Specific things to grep for:**
```bash
grep -rn "TODO\|FIXME\|HACK\|panic(" --include="*.go" cmd/ internal/
grep -rn "_ =" --include="*.go" cmd/ internal/   # ignored errors
```

### 5. Write your assessment
Be specific. Not "improved error handling." Instead: "The `/diff` command panics when called outside a git repo — no nil check on line 47 of commands_git.go."

## Output format

```
## Self-Assessment — Day N

### What I read
[Files you read and what you found]

### What I executed
[Commands you ran and their output]

### Issues found (ranked)
1. [CRITICAL/HIGH/MEDIUM/LOW] [Specific description with file:line if applicable]
2. ...

### Chosen improvement
[The ONE thing I will fix, and why I chose it over alternatives]
```
