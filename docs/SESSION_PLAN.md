## Session Plan

Session Title: Wire up modular command stubs to fix broken REPL commands

### Task 1: Wire up files.go command handlers
Files: internal/commands/files.go
Description: Replace stub implementations in internal/commands/files.go with working logic. The commands /add, /find, /grep, /web, /todos, /deps, and /ls are registered in the modular registry but return "not yet wired" instead of doing actual work. Since the modular registry intercepts these commands before the legacy switch in repl.go can handle them, users get broken stubs. Wire each handler to: /grep runs grep with file type filters and prints results, /todos scans .go/.md/.sh/.py/.ts/.js files for TODO/FIXME/HACK and prints results, /find does fuzzy file matching and lets user pick a file to inject into context, /web fetches a URL and injects content into agent context, /add reads a file and injects into agent context, /deps reads go.mod and prints it, /ls lists directory contents. Each handler uses the existing Context fields (RepoPath, Agent, Arg, HasArg, SelectItem). Add necessary imports (os, filepath, bufio, strings, net/http, io, os/exec, bytes, time, iteragent).
Issue: none

### Task 2: Wire up memory.go command handlers
Files: internal/commands/memory.go
Description: Replace stub implementations in internal/commands/memory.go. The commands /memo, /learn, /memories, and /remember print success messages but don't actually write anything. Wire each handler to: /memo appends a memo entry to JOURNAL.md, /learn appends a JSON entry to memory/learnings.jsonl, /memories reads and displays memory/active_learnings.md and .iterate/memory.json, /remember saves a note to .iterate/memory.json. Add necessary imports (os, filepath, encoding/json, time, fmt, strings, iteragent).
Issue: none

### Task 3: Wire up utility.go command handlers
Files: internal/commands/utility.go, internal/commands/registry.go
Description: Replace stub implementations in internal/commands/utility.go. The commands /export, /retry, /copy, /pin, /unpin, and /compact are stubs. Wire each handler to: /export writes conversation to markdown file, /retry retries the last message (needs last prompt tracking), /copy copies last response to clipboard via pbcopy/xclip, /pin appends last agent message to pinned messages list, /unpin clears pinned messages, /compact uses iteragent.CompactMessagesTiered. Add LastPrompt and LastResponse string fields to Context in registry.go so /retry and /copy can access them.
Issue: none

### Issue Responses
