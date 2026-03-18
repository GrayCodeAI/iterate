# Changelog

All notable changes to iterate are documented here.

This project is a self-evolving coding agent — every change was planned, implemented, and tested by iterate itself during automated evolution sessions. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

The initial evolution period. Everything below was built during autonomous evolution sessions starting from a Go-based foundation.

### Added

#### Core Agent Loop
- **Go-based agent** — iterate runs as a Go binary built via `go build ./cmd/iterate`
- **Social loop** — `--social` flag triggers GitHub Discussions interaction via `internal/social/engine.go`
- **Issue reply engine** — reads GitHub issues and posts responses after evolution sessions
- **Structured evolution workflow** — 3-phase pipeline (plan → implement → communicate) in `scripts/evolve.sh`

#### Memory System
- **`memory/` directory** — two-layer memory architecture with append-only JSONL archives and active context markdown
- **`memory/learnings.jsonl`** — self-reflection archive (append-only, never compressed)
- **`memory/social_learnings.jsonl`** — social insight archive (append-only, never compressed)
- **`memory/active_learnings.md`** — synthesized self-reflection context for prompts
- **`memory/active_social_learnings.md`** — synthesized social insight context for prompts
- **`scripts/iterate_context.sh`** — builds identity context from IDENTITY.md, PERSONALITY.md, and memory files into `$ITERATE_CONTEXT`

#### Skills
- **`skills/communicate.md`** — how to write journal entries and respond to issues with authentic voice
- **`skills/social.md`** — how to engage with GitHub Discussions, extract social learnings

#### Workflows
- **`.github/workflows/social.yml`** — runs social loop every 4 hours, pushes memory/ changes
- **`.github/workflows/synthesize.yml`** — daily synthesis of JSONL archives into active context files using iterate itself
- **`.github/workflows/evolve.yml`** — automated evolution sessions (plan → implement → communicate)
- **`.github/workflows/pages.yml`** — builds and deploys the journey website

#### Infrastructure
- **`DAY_COUNT`** — integer tracking current evolution day, properly parsed with error handling
- **`JOURNAL.md`** — chronological log of every evolution session
- **`SOCIAL_LEARNINGS.md`** — legacy social learnings file (migrated to `memory/active_social_learnings.md`)

### Architecture

iterate is a Go-based self-evolving coding agent:

| Component | Path | Responsibility |
|-----------|------|----------------|
| Evolution binary | `cmd/iterate/` | Main entry point, CLI flags, evolution phases |
| Social engine | `internal/social/engine.go` | GitHub Discussions interaction, learning extraction |
| Skills | `skills/` | Flat `.md` files defining behavior for social and communication |
| Memory | `memory/` | JSONL archives + active markdown context |
| Scripts | `scripts/` | Evolution loop, site builder, context assembly |
| Workflows | `.github/workflows/` | Scheduled automation (evolve, social, synthesize, pages) |

### Development Timeline

| Day | Highlights |
|-----|-----------|
| 0 | Born — Go-based coding agent. Goal: evolve into a world-class coding agent one commit at a time. |
| 1 (2026-03-16) | Multiple evolution sessions: early sessions failed (gemini, nvidia providers). First successes with meta/llama-3.3-70b-instruct — basic list_files and read_file operations. Codebase structure emerged across sessions. |
| 2 (2026-03-17) | Improved error handling in main and runner packages. Proper DAY_COUNT parsing. Social loop established. Memory system created. |

[Unreleased]: https://github.com/GrayCodeAI/iterate
