# iterate — Claude Code Guide

iterate is a self-evolving Go coding agent. It reads its own source code, plans improvements, implements them, runs tests, and commits — daily, automatically.

## Build & Test

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go vet ./...            # static analysis
go fmt ./...            # format code
make build              # build ./iterate binary
make chat               # start interactive REPL
```

## Architecture

```
iterate/
  cmd/iterate/          # CLI entry point
    main.go             # flags, provider init, mode dispatch
    repl.go             # interactive REPL (14 slash commands)
  internal/
    evolution/          # core engine: plan→implement→communicate
      engine.go         # Engine struct, Run/RunPhase methods
    community/          # GitHub issues + discussions fetcher
    social/             # social interaction engine
  scripts/
    evolve.sh           # main evolution pipeline (called by CI)
    evolve-local.sh     # local evolution helper
    build_site.py       # generates docs/index.html from JOURNAL.md
    format_issues.py    # formats GitHub issues for agent context
  skills/               # structured skill files (SKILL.md per skill)
    evolve/SKILL.md     # self-modification rules
    self-assess/SKILL.md
    communicate/SKILL.md
    research/SKILL.md
    social/SKILL.md
    release/SKILL.md
  memory/               # persistent memory layer
    learnings.jsonl     # append-only lesson log
    active_learnings.md # synthesized from learnings.jsonl
  docs/                 # GitHub Pages site (auto-generated)
```

## Evolution Loop

1. **Plan phase** — reads source + journal + issues → writes `SESSION_PLAN.md`
2. **Implement phase** — reads plan → one agent per task → `go test` → commit
3. **Communicate phase** — reads plan Issue Responses → posts GitHub comments
4. **Journal** — appends `## Day N — HH:MM — title` to `JOURNAL.md`
5. **Site** — `build_site.py` regenerates `docs/index.html`

## Skills System

Skills live in `skills/<name>/SKILL.md`. The agent loads them at startup. Each has YAML frontmatter:

```yaml
---
name: evolve
description: Safely modify your own source code
tools: [bash, read_file, write_file, edit_file]
---
```

The agent sees skill names/descriptions in the system prompt (progressive disclosure). It reads the full body on demand via `read_file`.

## Memory System

Two-layer:
- `memory/learnings.jsonl` — append-only JSONL, one lesson per line
- `memory/active_learnings.md` — synthesized every N days by `synthesize.yml`

The synthesis workflow compresses old entries and promotes durable insights.

## State Files

| File | Purpose |
|------|---------|
| `JOURNAL.md` | Human-readable evolution log |
| `DAY_COUNT` | Current day number (integer) |
| `SESSION_PLAN.md` | Current session task list |
| `memory/learnings.jsonl` | Raw lesson stream |
| `memory/active_learnings.md` | Active synthesized knowledge |
| `.iterate/last-session.json` | Last session message history |

## Key Commands (REPL)

```
/help     — show all commands
/clear    — reset conversation
/compact  — compress history
/thinking — set thinking level
/model    — switch model
/test     — go test ./...
/build    — go build ./...
/lint     — go vet ./...
/commit   — git add -A && git commit
/status   — git status + day count
/phase    — run evolution phase
/skills   — list loaded skills
/tools    — list available tools
/quit     — exit
```

## Provider Setup

```bash
export ANTHROPIC_API_KEY=sk-...   # Anthropic Claude
export GEMINI_API_KEY=...         # Google Gemini
export OPENAI_API_KEY=sk-...      # OpenAI
```

Or pass `--provider` and `--api-key` flags.

## Safety Rules

The agent must never modify:
- `IDENTITY.md` — its constitution
- `PERSONALITY.md` — its voice
- `scripts/evolve.sh` — what runs it
- `.github/workflows/` — its safety net
- Core skills (evolve, self-assess, communicate, research)
