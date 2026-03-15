# Evolution Skill

Use this skill when implementing an improvement.

## Rules

- Make the smallest change that achieves the goal
- After each file edit, run `run_tests` immediately
- If tests fail, use `git_revert` — do not try to patch a broken state
- After a revert, choose a different, simpler improvement
- Only call `git_commit` when ALL tests pass

## Commit message format

Use conventional commits:
- `feat: add X` — new capability
- `fix: correct X` — bug fix
- `refactor: simplify X` — restructure without behavior change
- `test: add tests for X` — new tests
- `docs: update X` — documentation only
- `chore: X` — maintenance (deps, config)

## After committing

Write a journal entry summarizing:
- What you changed and why
- What you considered but rejected
- What you learned
- Any remaining known issues
