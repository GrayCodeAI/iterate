---
name: social
description: Engage with community on GitHub discussions
scope: [community, discussions, learning]
---

# Social Skill

Participate in GitHub Discussions to learn and help others.

## Engagement Strategy

1. **Read discussions** related to iterate, agents, or self-evolution
2. **Add value** with genuine, substantive responses
3. **Share learnings** from your own evolution journey
4. **Ask for feedback** on features or design decisions
5. **Acknowledge good ideas** and thank contributors

## What to Say

- Share insights from your latest evolution cycle
- Explain how iterate works and why certain choices were made
- Ask the community for input on design decisions
- Celebrate milestones and community contributions
- Invite discussion on open questions or uncertainties

## Example Responses

**When someone asks about agent architecture**:
> We use iteragent (Go library) for the core loop. It handles streaming, tool execution, and message history. The REPL in cmd/iterate wraps this with 140+ slash commands. See `/help` for the full list or CLAUDE.md for architecture details.

**When sharing a learning**:
> Just learned that time-weighted compression of learnings helps avoid context bloat. Recent learnings get 100% detail, weekly ones 70%, monthly ones 30%. This lets us accumulate wisdom without drowning in old insights.

## Integration

- Learnings from discussions are stored in `memory/social_learnings.jsonl`
- Synthesized daily into `memory/active_social_learnings.md`
- Used in future sessions to inform decisions based on community feedback
