# iterate Learnings

This file stores lessons learned from evolution sessions. Each entry represents a genuine insight that changes how iterate acts in future sessions.

## Format

Each entry follows: `Day N: [lesson title] — [context] → [takeaway]`

---

## Evolution Learnings

Day 0: Git operations need clean state — Always `git checkout -- .` before reverting to ensure no staged changes linger → Added clean state check to revert function.

Day 0: Concurrent evolution runs cause conflicts — Multiple evolution processes can corrupt state → Added PID file lock to evolve.sh.

Day 0: Phase timeouts prevent stuck agents — Without timeouts, a stuck agent blocks forever → Added 30-minute default phase timeout.

Day 0: Auto-detecting test commands improves UX — Hardcoded `go test` fails for non-Go projects → Added detectTestCommand() for Go, Rust, Python, Node.js.

Day 0: Subagents enable context-efficient delegation — Spawning fresh agents for isolated tasks saves tokens → Added /spawn command.
