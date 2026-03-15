# Research skill

Use this skill before implementing anything non-trivial.

## When to research

Research before implementing if:
- You're about to add a new Go dependency — check if it's maintained, popular, and has a good API
- You're implementing a pattern you haven't used before (e.g. a specific concurrency pattern, a new HTTP pattern)
- A community issue references an external tool, library, or technique
- You're unsure if a standard library already covers what you need

## How to research using the bash tool

Use `curl` to fetch documentation or check package stats:

```bash
# Check if a Go package exists and is maintained
curl -s "https://pkg.go.dev/github.com/some/package" | grep -i "version\|last commit"

# Check GitHub stars / recent activity
curl -s "https://api.github.com/repos/owner/repo" | grep -E '"stargazers_count"|"pushed_at"'

# Fetch Go stdlib docs summary
curl -s "https://pkg.go.dev/search?q=PACKAGE_NAME&m=package" | head -100
```

## Research output format

Before proceeding with implementation, write a short research note:

```
## Research: <topic>

**Question:** What am I trying to find out?
**Found:** What I learned (2-4 sentences)
**Decision:** What I'll use and why
**Alternatives considered:** What I rejected and why
```

## Rules

- Don't add a dependency if the standard library covers it
- Prefer packages with >500 GitHub stars and commits in the last 6 months
- If you can't verify a package is safe/maintained, implement it yourself
- Research notes don't need to be saved — they're for the current session only
