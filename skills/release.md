# Release skill

Use this skill when cutting a release or writing a changelog entry.

## When to cut a release

Cut a release when ALL of these are true:
- At least 5 commits have landed since the last release
- At least one of them is a meaningful feature or fix (not just docs/chore)
- All tests are passing

## Changelog format

Write the changelog entry in `CHANGELOG.md` (create it if it doesn't exist).

```markdown
## v0.X.0 — Day N (YYYY-MM-DD)

### Added
- <new capability, one line>

### Fixed
- <bug fixed, one line — include what the symptom was>

### Changed
- <behavior that changed, one line>

### Internal
- <refactors, dependency updates, test improvements>
```

Rules:
- Each entry is one sentence, written for a human reading it — not a commit message
- "Added X" not "feat: add X"
- Link to the commit SHA if the change is complex: `([abc1234](...))`
- If nothing changed in a category, omit that section entirely

## GitHub Release

After writing the changelog, create a GitHub release:
- Tag: `v0.X.0`
- Title: `Day N — <one-line summary of the most important change>`
- Body: paste the changelog entry for this version

Use the `bash` tool to create the git tag:
```
git tag -a v0.X.0 -m "Day N: <summary>"
git push origin v0.X.0
```

Then use the GitHub API via `bash` + `curl` to create the release:
```bash
curl -s -X POST \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  https://api.github.com/repos/OWNER/REPO/releases \
  -d '{"tag_name":"v0.X.0","name":"Day N — summary","body":"changelog here","draft":false}'
```

## Versioning

- Start at v0.1.0
- Increment minor (v0.X.0) for each release
- Only go to v1.0.0 when the agent itself judges it is production-ready
