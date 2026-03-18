---
name: release
description: Prepare releases and manage version bumps
scope: [versioning, releases, changelog]
---

# Release Skill

Manage releases and version bumps.

## Version Management

Iterate follows semantic versioning: MAJOR.MINOR.PATCH

When to bump:
- **MAJOR**: Breaking changes to CLI or REPL
- **MINOR**: New features (new slash commands)
- **PATCH**: Bug fixes and improvements

## Release Checklist

1. [ ] Run all tests: `go test ./...`
2. [ ] Build successfully: `go build ./cmd/iterate`
3. [ ] Update go.mod if dependencies changed
4. [ ] Write changelog entry in CHANGELOG.md
5. [ ] Tag release: `git tag v0.X.Y`
6. [ ] Push: `git push && git push --tags`
7. [ ] GitHub releases: Create release notes from CHANGELOG

## Example Release Entry

```markdown
## v0.3.0 (2026-03-20)

### Features
- Added `/mcp-add`, `/mcp-list`, `/mcp-remove` for MCP server management
- Implemented `/generate-readme` for AI-assisted documentation
- Added `/diagram` for ASCII architecture diagrams

### Improvements
- Fixed spinner race condition in permission system
- Enhanced `/snapshot` and `/snapshots` commands
- Better error messages for missing dependencies

### Fixes
- Fixed tab completion for slash commands starting with `/`
- Corrected context bar percentage calculation

### Dependencies
- Updated iteragent to v0.2.0
```
