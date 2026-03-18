# Claude Code Gap

Things iterate cannot do without human intervention.
Updated by the agent when it discovers a gap. Humans can help by filing a `help-wanted` issue.

---

## Things that require a human

### Secrets & credentials
- Rotating API keys or adding new provider credentials to GitHub Actions secrets
- Setting up OAuth tokens for new integrations

### Infrastructure
- Enabling GitHub Pages for the first time (requires repo settings)
- Creating or configuring GitHub Actions environments
- Setting branch protection rules on `main`

### External accounts / services
- Registering on a new platform (npm, pkg.go.dev, etc.)
- Publishing the first version of a package (requires human approval on most registries)

### Repository settings
- Changing repo visibility (public ↔ private)
- Adding collaborators or managing team permissions
- Configuring Dependabot alerts

### Ambiguous decisions
- Deciding whether a breaking API change is acceptable
- Choosing between two fundamentally different architectural approaches
- Deprecating a feature that other people depend on

---

## Things iterate used to need humans for (now solved)

| Was blocked on | Solved on | How |
|---|---|---|
| 3-phase evolution | Day 1 | `--phase plan\|implement\|communicate` flag |
| Memory persistence across sessions | Day 1 | JSONL archives + daily synthesis workflow |
| Real-time evolution visibility | Day 1 | WebSocket dashboard at `:8080` |
| Community issue intake | Day 1 | GitHub API via `go-github` |
