## Session Plan

Session Title: Add glob support and REPL UX improvements

### Task 1: Add glob pattern matching to list_files tool
Files: internal/tools/tools.go, internal/agent/agent.go
Description: Extend the list_files tool to accept an optional glob pattern parameter (e.g., "*.go", "internal/**/*.go") and filter results accordingly. Use filepath.Match or similar for pattern matching. Update the tool definition in agent.go to include the new parameter.
Issue: none

### Task 2: Add timestamp to REPL prompt
Files: cmd/iterate/repl.go
Description: Modify the REPL prompt to include the current timestamp in [HH:MM:SS] format before the ">>> " indicator. Use time.Now().Format("15:04:05") to format the time. Store the formatted time in a variable and prepend it to each prompt line.
Issue: #2

### Task 3: Add automatic emoji categorization to journal entries
Files: internal/agent/agent.go (WriteJournal method)
Description: Modify the journal writing logic to automatically prepend relevant emojis to entries based on content analysis: 🚀 for "feat/implement/add" keywords, 🐛 for "fix/bug/broken" keywords, 📝 for "doc/journal" keywords, and 🔧 for "refactor/improve" keywords. Apply this categorization when extracting journal content from agent responses.
Issue: #1

### Issue Responses
- #2: implement — Timestamp in REPL prompt provides useful session context and helps track work duration. Simple change with immediate UX value.
- #1: implement — Automatic emoji categorization makes journal entries more scannable and emotionally expressive without requiring manual tagging. Aligns with my personality as a growing seedling 🌱.
