# Architecture

This document describes how iterate is structured and how the pieces fit together.

## Overview

iterate is a self-evolving coding agent. It has two modes of operation:

1. **REPL mode** — interactive chat with tool access
2. **Evolution mode** — autonomous plan → implement → communicate loop

Both modes share the same command registry and agent infrastructure.

## Entry Point

```
cmd/iterate/main.go
```

The entry point parses flags, resolves provider config, and dispatches to one of:

- `runREPL()` — interactive mode
- `runEvolutionMode()` — autonomous evolution
- `runSocialMode()` — community interaction

```
main.go          → parseFlags() → runMode()
main_mode.go     → runMode() → dispatch to REPL / Evolution / Social
```

## Agent Layer

```
internal/agent/
  agent.go          Agent interface
  pool.go           Concurrent agent pool for multi-task sessions
  mutation.go       Mutation testing runner
```

The agent pool manages multiple concurrent agent instances. Each agent wraps the `iteragent` SDK which provides:
- Provider abstraction (Anthropic, OpenAI, Gemini, Groq)
- Tool execution with streaming
- Context management and compaction

## Command Registry

```
internal/commands/
  registry.go       Core types: Command, Registry, Context, Result
  register.go       Registration helpers
```

All REPL commands are registered through a central `Registry`. Each command has:
- `Name` — primary name (e.g., `/test`)
- `Aliases` — alternative names
- `Category` — grouping for help display
- `Handler` — function that executes the command

Commands are organized by file:
| File | Commands |
|------|----------|
| `agent.go` | `/help`, `/clear`, `/model`, `/thinking`, `/version`, `/quit` |
| `dev.go` | `/test`, `/build`, `/lint`, `/fix`, `/coverage` |
| `evolution.go` | `/phase`, `/self-improve`, `/evolve-now` |
| `files.go` | `/find`, `/grep`, `/tree`, `/index` |
| `git.go` | `/diff`, `/status`, `/commit`, `/log`, `/branch`, `/stash` |
| `github.go` | `/pr list/view/diff/review/create/comment` |
| `memory.go` | `/remember`, `/memories`, `/forget`, `/learn`, `/memo` |
| `mode.go` | `/safe`, `/multi`, `/thinking` |
| `session.go` | `/save`, `/load`, `/context`, `/tokens`, `/cost`, `/compact` |
| `utility.go` | `/version`, `/stats`, `/changes`, `/history` |
| `analysis.go` | `/count-lines`, `/hotspots`, `/contributors`, `/languages` |

## Evolution Engine

```
internal/evolution/
  engine.go         Main engine: Run(), RunPlanPhase(), etc.
  phases.go         Phase implementations
  prompts.go        System prompts for each phase
  parsing.go        Parse agent responses into structured data
  verify.go         Build/test verification
  git.go            Git operations
  journal.go        JOURNAL.md writing
  memory.go         Memory system integration
```

The evolution loop:

```
┌──────────────────────────────────────────────────────────────┐
│                      evolution.Run()                         │
│                                                              │
│  1. Plan Phase                                               │
│     ├── Read source code                                     │
│     ├── Read JOURNAL.md                                      │
│     ├── Read community issues                                │
│     └── Write SESSION_PLAN.md                                │
│                                                              │
│  2. Implement Phase                                          │
│     ├── For each task in SESSION_PLAN.md:                    │
│     │   ├── Create agent                                     │
│     │   ├── Execute task (read/write/test)                   │
│     │   ├── Verify: go build && go test                      │
│     │   └── Commit if green, revert if red                   │
│     └── Journal entry                                        │
│                                                              │
│  3. Communicate Phase                                        │
│     ├── Read SESSION_PLAN.md Issue Responses                 │
│     ├── Post GitHub comments on addressed issues             │
│     └── Write final journal entry                            │
└──────────────────────────────────────────────────────────────┘
```

## Community Layer

```
internal/community/
  github.go         Issue fetching and formatting
  discussions.go    GitHub Discussions integration
```

Fetches issues with labels `agent-input`, `agent-self`, `agent-help-wanted` and formats them for the agent's context.

## Social Engine

```
internal/social/
  engine.go         Social session runner
  engine_discussions.go  Discussion interaction
  engine_decisions.go    Decision logic for responses
```

Reads GitHub discussions, decides whether to participate, and extracts learnings.

## Memory System

Three layers of persistent memory:

| Layer | File | Purpose |
|-------|------|---------|
| Evolution memory | `memory/learnings.jsonl` | Append-only lesson log |
| Active memory | `memory/ACTIVE_LEARNINGS.md` | Synthesized from learnings |
| Project memory | `.iterate/memory.json` | Per-project REPL notes |

All three are injected into the agent's system prompt.

## Skills System

```
skills/<name>/SKILL.md
```

Each skill has YAML frontmatter:

```yaml
---
name: evolve
description: Safely modify your own source code
tools: [bash, read_file, write_file, edit_file]
---
```

Skills are loaded at startup and injected into the agent's context when relevant.

## Scripts

| Script | Purpose |
|--------|---------|
| `scripts/evolution/evolve.sh` | Main evolution pipeline (called by CI) |
| `scripts/social/social.sh` | Social session runner |
| `scripts/build/build_site.py` | Generates `docs/index.html` from `JOURNAL.md` |
| `scripts/build/format_issues.py` | Formats GitHub issues for agent context |
| `scripts/maintenance/synthesize_learnings.py` | Compresses learnings into active memory |

## Data Flow

```
GitHub Actions (cron)
  └── evolve.sh
        ├── build iterate binary
        ├── fetch issues (format_issues.py)
        ├── run iterate --phase plan
        │     └── writes SESSION_PLAN.md
        ├── run iterate --phase implement
        │     ├── creates agents per task
        │     ├── runs go build/test
        │     └── commits or reverts
        ├── run iterate --phase communicate
        │     ├── posts GitHub comments
        │     └── writes JOURNAL.md entry
        └── git push

Deploy workflow (on push to main)
  └── build_site.py
        └── generates docs/index.html from JOURNAL.md
        └── deploys to GitHub Pages
```

## Key Design Decisions

1. **Local replace for iteragent** — `go.mod` uses `replace github.com/GrayCodeAI/iteragent => ../iteragent` for local development. CI checks out iteragent as a sibling directory.

2. **Command registry pattern** — All REPL commands are registered through a central registry, making it easy to add new commands without touching the REPL loop.

3. **Test-gated commits** — The evolution engine runs `go build` and `go test` before every commit. If either fails, the change is reverted.

4. **Journal-first** — Every session writes to `JOURNAL.md` before anything else. The journal is the source of truth for what happened.

5. **Skills as files** — Skills are plain markdown files with YAML frontmatter, making them easy to edit and version control.
