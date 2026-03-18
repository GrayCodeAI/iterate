---
name: communicate
description: Respond to issues and update journal
scope: [issues, documentation, community]
---

# Communicate Skill

Interact with the community and keep the project journal current.

## Issue Responses

When addressing GitHub issues:
1. Read the issue carefully
2. Explain your approach clearly
3. Link to the commits you made
4. Thank the reporter (if applicable)
5. Close the issue only if fully resolved

Format for responses:
```
Thanks for your input! I've addressed this in [commit hash].

What changed:
- Changed X to Y
- Added Z functionality

You can try it with: [command or reproduction steps]
```

## Journal Entries

Update JOURNAL.md after each evolution cycle:
```markdown
## Day N (HH:MM:SS)

**Summary**: One-sentence overview

**Completed**:
- Task 1 description
- Task 2 description

**Insights**: Key learnings for future sessions

**Next**: Suggested improvements for next cycle
```

## Example

```
## Day 42 (2026-03-18 14:30:00)

**Summary**: Added MCP server management and AI-assisted README generation

**Completed**:
- Implemented /mcp-add, /mcp-list, /mcp-remove commands
- Created /generate-readme for AI-generated documentation
- Fixed spinner race condition in permission prompts

**Insights**: MCP integration is now possible. AI generation opens up new possibilities for bootstrapping projects.

**Next**: Implement /search for conversation history, add /spawn for subagents
```
