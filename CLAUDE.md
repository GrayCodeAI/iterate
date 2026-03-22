# iterate — Claude Code Guide

iterate is a self-evolving Go coding agent. It reads its own source code, plans improvements, implements them, runs tests, and commits — daily, automatically.

The REPL is backed by the `iteragent` SDK which provides: streaming tokens (TokenStreamer), lifecycle hooks (AgentHooks: BeforeTurn/AfterTurn/OnToolStart/OnToolEnd), automatic retry on transient errors, context compaction, MCP server support (Agent.Close() shuts servers down on /quit), and TOML config.

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
  cmd/iterate/                            # CLI entry point
    main.go                               # flags, provider init, mode dispatch
    repl.go                               # thin REPL loop (~400 lines)
    repl_streaming.go                     # streaming token output
    repl_helpers.go                       # REPL utility functions
    repl_models.go                        # model switching logic
    selector.go                           # raw-mode input, tab completion, selectItem
    highlight.go                          # markdown + syntax rendering
    pricing.go                            # per-model cost table, /cost breakdown
    config.go                             # iterConfig (JSON+TOML), glob/dir permissions
    memory_project.go                     # per-project .iterate/memory.json for /remember
    features.go                           # feature helpers (~400 lines)
    features_sessions.go                  # session save/load/compact
    features_search.go                    # /find, /grep
    features_prompts.go                   # prompt management
    features_tools.go                     # tool listing and info
    features_watch.go                     # file watching
    features_git_helpers.go               # git helper functions
    commands_project.go                   # /health, /tree, /index, /pkgdoc
    commands_git.go                       # /pr subcommand dispatcher, enhanced /diff
  internal/
    agent/                                # agent pool, mutation testing
    commands/                             # 100+ modular commands
      registry.go                         # command type defs and registration
      register.go                         # command registration helpers
      agent.go                            # /help, /clear, /model, /thinking, etc.
      dev.go                              # /test, /build, /lint, /fix, /coverage
      evolution.go                        # /phase, /self-improve, /evolve-now
      files.go                            # /find, /grep, /tree, /index
      git.go                              # /diff, /status, /commit, /log, /branch, etc.
      github.go                           # /pr list/view/diff/review/create/comment
      memory.go                           # /remember, /memories, /forget, /learn, /memo
      mode.go                             # /safe, /multi, /thinking
      safety.go                           # safety checks and confirmations
      session.go                          # /save, /load, /context, /tokens, /cost, /compact
      utility.go                          # /version, /stats, /changes, /history
      legacy.go                           # legacy command aliases
      project_helpers.go                  # project-type detection helpers
    community/                            # GitHub issues + discussions fetcher
    evolution/                            # core engine: plan→implement→communicate
    social/                               # social interaction engine
    util/                                 # shared utilities
      truncate.go                         # string truncation helpers
  scripts/
    evolve.sh                             # main evolution pipeline (called by CI)
    evolve-local.sh                       # local evolution helper
    build_site.py                         # generates docs/index.html from JOURNAL.md
    format_issues.py                      # formats GitHub issues for agent context
  skills/                                 # structured skill files (SKILL.md per skill)
    evolve/SKILL.md
    self-assess/SKILL.md
    communicate/SKILL.md
    research/SKILL.md
    social/SKILL.md
    release/SKILL.md
  memory/                                 # persistent evolution memory layer
    learnings.jsonl                       # append-only lesson log
    active_learnings.md                   # synthesized from learnings.jsonl
  docs/                                   # GitHub Pages site (auto-generated)
```

## Evolution Loop

1. **Plan phase** — reads source + journal + issues → writes `SESSION_PLAN.md`
2. **Implement phase** — reads plan → one agent per task → `go test` → commit
3. **Communicate phase** — reads plan Issue Responses → posts GitHub comments
4. **Journal** — appends `## Day N — HH:MM — title` to `JOURNAL.md`
5. **Site** — `build_site.py` regenerates `docs/index.html`

## Skills System

Skills live in `skills/<name>/SKILL.md`. Each has YAML frontmatter:

```yaml
---
name: evolve
description: Safely modify your own source code
tools: [bash, read_file, write_file, edit_file]
---
```

## Memory System

Three layers:
- `memory/learnings.jsonl` — append-only JSONL evolution lessons
- `memory/active_learnings.md` — synthesized by `synthesize.yml`
- `.iterate/memory.json` — per-project REPL notes (`/remember`, `/forget`, `/memories`)

All three are injected into the agent's system prompt automatically.

## State Files

| File | Purpose |
|------|---------|
| `JOURNAL.md` | Human-readable evolution log |
| `DAY_COUNT` | Current day number (integer) |
| `SESSION_PLAN.md` | Current session task list |
| `ITERATE.md` | Project context file (generate with `/iterate-init`) |
| `memory/learnings.jsonl` | Raw evolution lesson stream |
| `memory/active_learnings.md` | Active synthesized knowledge |
| `.iterate/memory.json` | Per-project REPL notes |
| `.iterate/sessions/` | Saved REPL sessions |
| `.iterate/bookmarks.json` | Disk-persisted conversation bookmarks |

## Key REPL Commands

```
── Agent ─────────────────────────────────────────────────────────
/help              — show all commands
/clear             — reset conversation
/thinking <level>  — off|minimal|low|medium|high
/model             — switch provider/model (interactive)
/provider <name>   — quick provider switch
/version           — show version + provider
/quit              — exit (autosaves session)

── Code Quality ──────────────────────────────────────────────────
/test              — run tests (project-type aware)
/build             — build project
/lint              — lint project
/health            — full project health (Go/Rust/Node/Python/Make)
/fix               — auto-fix build errors (project-type aware)
/coverage          — test coverage report

── Project Tooling ───────────────────────────────────────────────
/tree [depth]      — git ls-files tree
/index             — structured file index with summaries
/pkgdoc <pkg>      — look up pkg.go.dev
/iterate-init      — generate ITERATE.md context file
/find <pattern>    — fuzzy file search
/grep <pattern>    — search code content

── Git ───────────────────────────────────────────────────────────
/diff              — enhanced diff with stat bar
/status            — git status + day count
/commit <msg>      — git add -A && commit
/undo              — undo last commit
/git <subcmd>      — full git passthrough
/pr list           — list open PRs
/pr view [N]       — view PR
/pr diff [N]       — PR diff
/pr review [N]     — AI code review of PR diff
/pr create         — create PR (--draft supported)
/pr comment N      — post PR comment
/pr checkout [N]   — checkout PR branch
/log [n]           — last n commits
/branch / /checkout / /merge / /stash / /tag

── Memory ────────────────────────────────────────────────────────
/remember <note>   — save project note to .iterate/memory.json
/memories          — show project notes + evolution learnings
/forget [n]        — remove project note n (or /forget msg n for context)
/learn <fact>      — add to evolution memory/learnings.jsonl
/memo <text>       — append to JOURNAL.md
/mark [name]       — mark conversation position (in-memory)
/marks             — list marks
/jump <name>       — restore to mark

── Session ───────────────────────────────────────────────────────
/save [name]       — save session
/load [name]       — load session (interactive)
/context           — context stats + bar
/tokens            — detailed token usage
/cost              — per-model USD cost estimate
/changes           — files written/edited this session
/compact           — summarise then compact history (preserves context)
/history           — input history
/stats             — session duration + tool calls

── Evolution ─────────────────────────────────────────────────────
/phase plan|implement|communicate
/self-improve
/evolve-now
```

## Provider Setup

```bash
export ANTHROPIC_API_KEY=sk-...   # Anthropic Claude
export GEMINI_API_KEY=...         # Google Gemini
export OPENAI_API_KEY=sk-...      # OpenAI
export GROQ_API_KEY=...           # Groq
```

Or pass `--provider` and `--api-key` flags, or use `/provider <name>` at runtime.

Config is stored at `~/.config/iterate/config.toml` (preferred) or `~/.iterate/config.json`. TOML is loaded first. Config supports `allow_dirs`, `deny_dirs`, `allow_patterns`, `deny_patterns`, `safe_mode`, `theme`, and more.

## Multi-line Input

End a line with `\` to continue on the next line:
```
❯ Fix the auth bug in login.go \
  ... and add a test for it
```
Or use `/multi` for paste mode (end with `.`).

## Permission System

`/safe` enables safe mode — destructive tools (bash, write, edit) require confirmation.
Answer `always` to add a command to the session allow-list.

Config fields for persistent patterns:
```json
{ "allow_patterns": ["go test *"], "deny_patterns": ["rm -rf *"] }
```

## Safety Rules

The agent must never modify:
- `IDENTITY.md` — its constitution
- `PERSONALITY.md` — its voice
- `scripts/evolution/evolve.sh` — what runs it
- `.github/workflows/` — its safety net
- Core skills (evolve, self-assess, communicate, research)
