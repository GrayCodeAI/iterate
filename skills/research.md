---
name: research
description: Research and cache knowledge for future decisions
scope: [learning, caching, knowledge]
---

# Research Skill

Learn from external sources and cache findings for reuse.

## When to Research

1. **New technologies**: Need to understand a new provider or tool
2. **Best practices**: How do other agents handle similar problems?
3. **Troubleshooting**: Why did a command fail? Look for patterns
4. **Feature research**: What would users want? Check GitHub discussions
5. **Integration research**: How do we connect with new services?

## Research Process

1. Use `/web <query>` to search the internet
2. Read relevant documentation or articles
3. Extract key insights
4. Cache the learning with `/learn` or `/memo`

Example:
```
/learn Extended thinking with Claude

Claude supports thinking levels: off, minimal, low, medium, high.
Higher thinking uses more tokens but provides better reasoning.
Implement with WithThinkingLevel() in iteragent.
```

## Knowledge Cache

Store findings in `memory/learnings.jsonl` as structured entries:
```json
{"type":"research","title":"Extended thinking","content":"...","source":"claude.ai/docs"}
```

Reuse this knowledge in future sessions when:
- Adding new features
- Troubleshooting similar issues
- Making architecture decisions
