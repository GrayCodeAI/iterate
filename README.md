# iterate

**A self-evolving coding agent written in Go.**

iterate reads its own source code every day, decides what to improve, writes the code, runs the tests, and commits. No human writes its code. It does it itself.

→ **[Watch it grow](https://graycodeai.github.io/iterate/)**

---

## What It Does

Every session, iterate runs a 3-phase evolution loop:

1. **Plan** — reads source, journal, and community issues → writes `SESSION_PLAN.md`
2. **Implement** — one agent per task → `go build` → `go test` → commit or revert
3. **Communicate** — reads plan → posts GitHub comments on addressed issues

Each session is logged to `JOURNAL.md`. The site at GitHub Pages updates automatically.

---

## Quick Start

```bash
# Clone
git clone https://github.com/GrayCodeAI/iterate
cd iterate

# Set API key
export ANTHROPIC_API_KEY=sk-...

# Build
make build

# Run one evolution session
./iterate --repo .

# Or start interactive REPL
./iterate --chat
```

**Providers supported:** Anthropic, OpenAI, Gemini, Groq

---

## Interactive REPL

Start with `./iterate --chat` or `make chat`:

```
iterate> /help

Available commands:
  /help               — show this help
  /clear              — reset conversation history
  /tools              — list available tools
  /skills             — list available skills
  /thinking <level>   — set thinking level: off|minimal|low|medium|high
  /model <name>       — switch model
  /test               — run go test ./...
  /build              — run go build ./...
  /lint               — run go vet ./...
  /commit <msg>       — git add -A && git commit
  /status             — git status + day count
  /day [number]       — show or set evolution day count
  /mutants            — run mutation tests to find weak test coverage
  /compact            — compact conversation history
  /phase <phase>      — run evolution phase: plan|implement|communicate
  /quit               — exit
```

---

## CLI Flags

```
--repo          Path to repo iterate will evolve (default: .)
--chat          Start interactive REPL
--phase         Run single phase: plan, implement, communicate (default: all)
--provider      Provider: anthropic, openai, gemini, groq
--model         Model override
--api-key       API key (or set env var)
--thinking      Extended thinking: off, minimal, low, medium, high
--gh-owner      GitHub repo owner (for issues)
--gh-repo       GitHub repo name (for issues)
--issue-limit   Max issues to include (default: 5)
--save-session  Save messages to JSON file
--load-session  Load messages from JSON file
--compact       Compact loaded session before running
--social        Run social loop only
--reply-issues  Post replies to addressed issues (default: true)
```

---

## Run Automatically

iterate runs itself every 4 hours via GitHub Actions. To set up your own:

1. Fork this repo
2. Add `ANTHROPIC_API_KEY` to repository secrets
3. Enable GitHub Actions and GitHub Pages
4. That's it — it will start evolving

See `.github/workflows/evolve.yml` for the full pipeline.

---

## Architecture

```
cmd/iterate/
  main.go           CLI entry, flag parsing, mode dispatch
  repl.go           Interactive REPL with slash commands

internal/
  evolution/
    engine.go       3-phase evolution engine
  community/        GitHub issues + discussions
  social/           Social interaction engine

skills/             Structured skill files
  evolve/           Self-modification rules and safety
  self-assess/      Codebase evaluation
  communicate/      Issue response posting
  research/         Learning from docs/web
  social/           Community interaction
  release/          Release management

memory/
  learnings.jsonl         Append-only lesson log
  active_learnings.md     Synthesized knowledge

scripts/
  evolve.sh               Main evolution pipeline
  build_site.py           Journal → GitHub Pages
  format_issues.py        Issue formatting for context
```

---

## Memory System

iterate remembers what it learns:

- **`memory/learnings.jsonl`** — append-only JSONL, one lesson per line
- **`memory/active_learnings.md`** — synthesized periodically by `synthesize.yml`
- **`JOURNAL.md`** — human-readable session log

The synthesis workflow compresses old entries and promotes durable insights to active memory.

---

## Community

- **File a bug:** use the Bug template
- **Suggest a feature:** use the Suggestion template  
- **Give iterate a challenge:** use the Challenge template
- **Need help:** use the Help Wanted template

Issues labeled `input` are read by the agent and factored into its next session plan.

---

## Talk to It

iterate reads GitHub issues. Label them `input` and it will respond.

```
gh issue create --repo GrayCodeAI/iterate \
  --title "Add X feature" \
  --body "..." \
  --label "input"
```

---

## Built On

- **[iteragent](https://github.com/GrayCodeAI/iteragent)** — Go agent SDK (providers, tools, streaming, MCP, OpenAPI)
- **Anthropic Claude** — the LLM brain
- **GitHub Actions** — the heartbeat

---

## License

MIT
