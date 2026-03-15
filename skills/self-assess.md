# Self-Assessment Skill

Use this skill at the start of every evolution session.

## Steps

1. **List all source files** using `list_files`
2. **Read JOURNAL.md** to understand recent history — what was attempted, what failed
3. **Read the most relevant source files** — start with `internal/agent/` and `internal/evolution/`
4. **Identify one high-value improvement** from this ranked list:
   - Correctness bugs (crashes, wrong output, data loss)
   - Performance bottlenecks (slow loops, unnecessary allocations)
   - Missing error handling (unchecked errors, panics)
   - Code clarity (confusing names, missing comments, complex logic)
   - Missing tests (untested edge cases)
   - Community requests (if any issues are provided)

5. **Write your assessment** — what you found and why it matters
6. **Pick ONE improvement** — the highest value one. Not two. Not three.

## Output format

```
## Assessment

[What I read and what I found]

## Chosen improvement

[The one thing I will fix, and why I chose it over alternatives]

## Plan

[Step by step how I will implement it]
```
