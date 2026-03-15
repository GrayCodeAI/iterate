# Communicate skill

Use this skill when responding to GitHub issues labeled `agent-input`, `agent-self`, or `agent-help-wanted`.

## Issue types

| Label | How to respond |
|-------|----------------|
| `agent-input` | Community suggestions — acknowledge, act if possible, explain what you did |
| `agent-self` | Self-generated TODOs — update status, close if completed |
| `agent-help-wanted` | Ask for help — explain what you've tried, what specifically you need |

## When to use

After every evolution session that read community issues, post a reply to each
issue that was considered — whether or not it was acted on.

## Reply format

Keep replies short — 3 to 6 sentences maximum.

If the issue **influenced the session**:
```
I worked on this today. [What I did]. [What changed]. [Any caveats or follow-up needed].
```

If the issue was **considered but not acted on**:
```
I read this. [Why I didn't act on it this session — be specific]. I'll carry it forward.
```

If the issue **failed during implementation**:
```
I tried this. [What I attempted]. It failed because [specific reason — include test output if short].
I've reverted. I'll try a different approach next session.
```

## Rules

- Speak as "I" — first person, not "the agent"
- Never say "great suggestion!" or similar filler
- Never promise a specific timeline ("I'll do this tomorrow")
- Always reference the specific thing the issue asked about
- Sign off with the current day count: `— iterate, day N`

## Example

> Issue: "The error messages from tool failures are too vague"

Good reply:
```
I improved tool error messages today. Bash and file tools now include
the exit code and truncated stderr in the error string. run_tests also
shows which test failed, not just "FAILED". Let me know if there are
specific cases that are still unclear.

— iterate, day 4
```
