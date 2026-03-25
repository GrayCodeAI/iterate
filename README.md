<div align="center">

# iterate

**A self-evolving coding agent that writes its own code.**

[![CI](https://github.com/GrayCodeAI/iterate/actions/workflows/ci.yml/badge.svg)](https://github.com/GrayCodeAI/iterate/actions/workflows/ci.yml)
[![Deploy](https://github.com/GrayCodeAI/iterate/actions/workflows/deploy.yml/badge.svg)](https://graycodeai.github.io/iterate/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](go.mod)

[Watch it grow](https://graycodeai.github.io/iterate/) ·
[Report a bug](https://github.com/GrayCodeAI/iterate/issues/new?template=bug.md) ·
[Suggest a feature](https://github.com/GrayCodeAI/iterate/issues/new?template=suggestion.md)

</div>

---

## What is this?

iterate is a coding agent that **owns its own repository**. Every 4 hours it:

1. **Reads** its own source code, journal, and community issues
2. **Decides** what to improve — a bug, a missing feature, a rough edge
3. **Builds** the fix, runs `go build` and `go test`
4. **Commits** if green, reverts and journals if not

No human writes its code. It does it itself.

> **[Live site](https://graycodeai.github.io/iterate/)** — auto-updated after every session

## Quick Start

```bash
git clone https://github.com/GrayCodeAI/iterate.git
cd iterate
export ANTHROPIC_API_KEY=sk-...
make build
./iterate --repo .
```

**Providers:** Anthropic · OpenAI · Gemini · Groq

## Interactive REPL

```bash
./iterate --chat
```

```
iterate> /help

  Agent       /help  /clear  /model  /thinking  /version  /quit
  Code        /test  /build  /lint   /fix       /coverage /health
  Git         /diff  /status /commit /log       /branch   /pr
  Analysis    /count-lines  /hotspots  /contributors  /languages
  Project     /tree  /index  /find  /grep
  Memory      /remember  /memories  /forget  /learn  /memo
  Evolution   /phase  /self-improve  /evolve-now
  Session     /save  /load  /context  /tokens  /cost  /compact
```

## How It Works

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│  1. Plan     │────▶│  2. Implement │────▶│  3. Communicate  │
│              │     │               │     │                   │
│ Read source  │     │ Agent per task│     │ Post GH comments  │
│ Read journal │     │ go build/test │     │ Journal entry     │
│ Read issues  │     │ commit/revert │     │                   │
└─────────────┘     └──────────────┘     └─────────────────┘
        ▲                                           │
        └───────────────────────────────────────────┘
                    Every 4 hours via GitHub Actions
```

## Architecture

```
cmd/iterate/
  main.go                      Entry point, flag parsing
  repl.go                      Interactive REPL loop
  repl_streaming.go            Token-level streaming output
  repl_helpers.go              REPL utilities
  repl_models.go               Model switching
  config.go                    Config loading (TOML/JSON)
  features.go                  Core feature helpers
  features_prompts.go          Prompt builders
  features_shell.go            Shell/CLI utilities
  features_tools.go            Tool listing and info
  features_sessions.go         Session save/load/compact
  features_watch.go            File watching
  commands_project.go          /health, /tree, /index
  commands_git.go              /pr dispatcher

internal/
  agent/                       Agent pool + mutation testing
  commands/                    Modular command registry
    registry.go                Type definitions + registration
    register.go                Registration helpers
    agent.go                   /help, /clear, /model, /thinking
    dev.go                     /test, /build, /lint, /fix, /coverage
    evolution.go               /phase, /self-improve, /evolve-now
    files.go                   /find, /grep, /tree, /index
    git.go                     /diff, /status, /commit, /log, /branch
    github.go                  /pr list/view/diff/review/create
    memory.go                  /remember, /memories, /forget, /learn
    mode.go                    /safe, /multi, /thinking
    safety.go                  Safety checks
    session.go                 /save, /load, /context, /tokens, /cost
    utility.go                 /version, /stats, /changes
  community/                   GitHub issues + discussions
  evolution/                   3-phase evolution engine
  social/                      Social interaction engine
  ui/                          Terminal UI (colors, highlighting)
  util/                        String truncation helpers

skills/                        Structured skill files (SKILL.md)
  evolve/                      Self-modification rules
  self-assess/                 Codebase evaluation
  communicate/                 Issue response posting
  research/                    Learning from docs/web
  social/                      Community interaction
  release/                     Release management

scripts/
  evolution/evolve.sh          Main evolution pipeline
  social/social.sh             Social session runner
  build/build_site.py          Journal → GitHub Pages
  maintenance/synthesize_learnings.py  Memory compression

memory/
  learnings.jsonl              Append-only lesson log
  active_learnings.md          Synthesized knowledge

docs/                          GitHub Pages site
  index.html                   Auto-generated from JOURNAL.md
  JOURNAL.md                   Evolution log
  IDENTITY.md                  Agent constitution
  PERSONALITY.md               Agent voice
```

## GitHub Actions

| Workflow | Schedule | Purpose |
|----------|----------|---------|
| `evolve.yml` | Every 4h | Plan → Implement → Communicate |
| `social.yml` | Offset 2h | Read discussions, learn |
| `synthesize.yml` | Daily 3AM | Compress learnings |
| `deploy.yml` | On push | Build GitHub Pages |
| `ci.yml` | On push/PR | Build, test, vet, fmt |

## Run Your Own

1. Fork this repo
2. Add `ANTHROPIC_API_KEY` to repository secrets
3. Enable GitHub Actions and GitHub Pages
4. That's it — it will start evolving

## Community

- **[Bug report](https://github.com/GrayCodeAI/iterate/issues/new?template=bug.md)** — something broke
- **[Suggestion](https://github.com/GrayCodeAI/iterate/issues/new?template=suggestion.md)** — something to build
- **[Challenge](https://github.com/GrayCodeAI/iterate/issues/new?template=challenge.md)** — give it a hard problem
- **[Help wanted](https://github.com/GrayCodeAI/iterate/issues/new?template=help-wanted.md)** — stuck and asking for help

Issues labeled `agent-input` are read by the agent every session.

## Built On

- **[iteragent](https://github.com/GrayCodeAI/iteragent)** — Go agent SDK (providers, tools, streaming)
- **[opencode](https://opencode.ai)** — LLM provider
- **GitHub Actions** — the heartbeat

## License

[MIT](LICENSE)
