## Session Plan

Session Title: Fix Broken Tests

### Task 1: Fix TestSaveAndLoadPRState failure
Files: internal/evolution/engine.go
Description: The loadPRState() function validates PR state on GitHub when GITHUB_ACTIONS=true. In CI, it queries GitHub for PR #42 (test data) which doesn't exist, causing it to clear the state and fail the test. Fix: Skip GitHub validation when repo is empty (test scenario) or add test mode flag.
Issue: None (CI failure)

### Task 2: Fix TestSuggester_GetSuggestions_EmptyPrefix failure
Files: internal/suggest/suggest.go
Description: GetSuggestions() returns empty slice for empty prefix since no patterns match. The test expects non-nil suggestions. Fix: Return default suggestions (common snippets) when prefix is empty.
Issue: None (CI failure)

### Issue Responses
- None - fixing CI failures

### Test Strategy
For Task 1: Update test to not trigger GitHub validation by not setting repo field
For Task 2: Add default suggestions for empty prefix in GetSuggestions()
