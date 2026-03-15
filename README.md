# iterate

A self-evolving coding agent written in Go. Every day it reads its own source code, finds an improvement, tests it, and commits — or reverts and documents the failure.

## Features

| Feature | iterate |
|---------|---------|
| Language | **Go** |
| IDENTITY.md (immutable) | ✓ |
| PERSONALITY.md (voice) | ✓ |
| JOURNAL.md (append-only) | ✓ |
| SOCIAL_LEARNINGS.md | ✓ |
| DAY_COUNT | ✓ |
| Self-assess skill | ✓ |
| Evolve skill | ✓ |
| Communicate skill | ✓ |
| Social skill | ✓ |
| Release skill | ✓ |
| Research skill | ✓ |
| Evolution loop (daily) | ✓ |
| Social loop (4h) | ✓ |
| Issue bot replies | ✓ |
| Mutation testing | ✓ (go-mutesting) |
| Journey website | ✓ (Go) |
| Multi-file support | **Full repo tree** |
| Provider switching | **Ollama, OpenAI, Anthropic, Groq** |
| Web dashboard | **Live dashboard + WebSocket** |
| Session storage | **SQLite + JOURNAL.md** |

## Quick start

```bash
git clone https://github.com/yourusername/iterate
cd iterate
go build -o iterate ./cmd/iterate

# Run with Ollama (local, default)
ITERATE_MODEL=llama3.2 ./iterate

# Run with Anthropic
ITERATE_PROVIDER=anthropic ANTHROPIC_API_KEY=sk-... ./iterate

# Run with web dashboard
ITERATE_PROVIDER=anthropic ANTHROPIC_API_KEY=sk-... ./iterate --serve --addr :8080
```

## Switching providers

Set `ITERATE_PROVIDER` to one of:

| Value | Needs |
|-------|-------|
| `ollama` | Ollama running at `http://localhost:11434` |
| `openai` | `OPENAI_API_KEY` |
| `anthropic` | `ANTHROPIC_API_KEY` |
| `groq` | `GROQ_API_KEY` |

Optionally set `ITERATE_MODEL` to override the default model for any provider.

## Architecture

```
cmd/iterate/         CLI entrypoint
internal/
  agent/             Core agent loop + tools (bash, file, git)
  provider/          LLM providers (Ollama, OpenAI-compat, Anthropic)
  evolution/         Evolution engine — assess, implement, test, commit/revert
  community/         GitHub issue ingestion + vote ranking
  session/           SQLite session storage
  web/               Dashboard server + WebSocket live stream
skills/              Agent skill prompts (self-assess, evolve)
IDENTITY.md          Immutable agent constitution
JOURNAL.md           Append-only session log
DAY_COUNT            Current evolution day
```

## Talk to it

Open a GitHub issue with one of these labels:

| Label | Description |
|-------|-------------|
| `agent-input` | Community suggestions, bug reports, feature requests |
| `agent-self` | Issues the agent filed for itself as future TODOs |
| `agent-help-wanted` | Issues where the agent is stuck and asking humans for help |

Issues with more 👍 reactions get prioritized. The agent reads them during its next session (every 8 hours).

## Web dashboard

When running with `--serve`, the dashboard at `http://localhost:8080` shows:
- Live agent stream (WebSocket)
- Session history with status and duration
- Commit/revert stats

## Run automatically

Push to GitHub and set up secrets:
- `ANTHROPIC_API_KEY` (or your chosen provider's key)
- Set `ITERATE_PROVIDER` in repo variables

The GitHub Action at `.github/workflows/evolve.yml` runs every 8 hours (at 00:00, 08:00, and 16:00 UTC).

## License

MIT
