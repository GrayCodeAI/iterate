# iterate vs Claude Code — Gap Analysis

How iterate compares to Claude Code today. Updated by the agent as it evolves.

## Feature Matrix

### Core Agent Loop

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Multi-turn conversation | ✅ | ✅ |
| Tool use (bash, read, write, edit) | ✅ | ✅ |
| Streaming output | ✅ | ✅ |
| Context compaction (3-tier) | ✅ | ✅ |
| Extended thinking | ✅ | ✅ |
| Self-evolution (modifies own code) | ✅ | ❌ |
| 3-phase evolution loop | ✅ | ❌ |

### CLI & UX

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Interactive REPL | ✅ | ✅ |
| Slash commands | ✅ (14) | ✅ (42+) |
| Tab completion | ❌ | ✅ |
| Readline history | ❌ | ✅ |
| Syntax highlighting | ❌ | ✅ |
| Diff view | ❌ | ✅ |
| Cost tracking | ❌ | ✅ |

### Context Management

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Session save/load (JSON) | ✅ | ✅ |
| Context compaction | ✅ | ✅ |
| Memory (learnings.jsonl) | ✅ | 🟡 project memory |
| Memory synthesis | ✅ | ❌ |
| Conversation bookmarks | ❌ | ✅ |

### Provider & Model

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Anthropic | ✅ | ✅ |
| OpenAI | ✅ | ❌ |
| Gemini | ✅ | ❌ |
| Groq | ✅ | ❌ |
| Mid-session model switch | ✅ (`/model`) | ✅ |
| Prompt caching | ✅ | ✅ |

### Tools

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| bash / shell | ✅ | ✅ |
| read_file | ✅ | ✅ |
| write_file | ✅ | ✅ |
| edit_file | ✅ | ✅ |
| search (grep) | ✅ | ✅ |
| list_files | ✅ | ✅ |
| git operations | ✅ | ✅ |
| MCP (stdio + HTTP) | ✅ | ✅ |
| OpenAPI adapter | ✅ | ❌ |
| Web fetch | ✅ | ✅ |
| Concurrent tool execution | ❌ | ✅ |

### Project Understanding

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Reads own source code | ✅ | ✅ |
| CLAUDE.md support | ✅ | ✅ |
| Skills system | ✅ | 🟡 slash commands |
| Codebase indexing | ❌ | ✅ |
| Fuzzy file search | ❌ | ✅ |
| PR workflow | ❌ | ✅ |

### Permission System

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Tool approval prompts | ❌ | ✅ |
| Allow/deny glob patterns | ❌ | ✅ |
| Directory restrictions | ❌ | ✅ |
| Audit log | ❌ | ✅ |

### Community

| Feature | iterate | Claude Code |
|---------|---------|-------------|
| Reads GitHub issues | ✅ | ❌ |
| Posts issue comments | ✅ | ❌ |
| Social learning | ✅ | ❌ |
| Journey website | ✅ | ❌ |
| Issue voting system | ❌ | ❌ |

---

## Priority Queue

Things iterate should build next (highest value / lowest effort first):

1. **Tab completion in REPL** — quality of life
2. **`/diff` command** — show pending changes
3. **`/cost` command** — token usage per session
4. **Concurrent tool execution** — big perf win
5. **Permission prompts** — safety and trust
6. **`/pr` workflow** — create/review PRs
7. **Codebase indexing** — faster navigation
8. **Readline history** — persist across sessions

---

## What Humans Still Need to Do

Things iterate cannot do without human intervention:

- **Secrets** — add/rotate API keys, tokens
- **Infrastructure** — DNS, hosting, billing
- **Repository settings** — branch protection, Actions secrets
- **Publishing** — releasing to registries, app stores
- **Ambiguous decisions** — when two valid approaches conflict

---

## Recently Completed

- ✅ 3-phase evolution (plan → implement → communicate)
- ✅ Streaming event architecture
- ✅ Interactive REPL with 14 slash commands
- ✅ MCP stdio + HTTP transports
- ✅ OpenAPI adapter with OperationFilter
- ✅ Prompt caching
- ✅ Skills progressive disclosure
- ✅ Context compaction (3-tier)
- ✅ Memory system (JSONL + synthesis)
- ✅ GitHub issues integration
- ✅ Social learning engine
- ✅ Journey website (GitHub Pages)
- ✅ CI/CD workflows
- ✅ Removed SQLite (simplified, no CGo)
