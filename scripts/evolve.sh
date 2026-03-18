#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → respond
# Autonomous 3-phase evolution cycle

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/SESSION_PLAN.md"

mkdir -p "${REPOPATH}/.iterate"

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "=== iterate evolution cycle started ==="

# Build the binary
log "Building iterate..."
go build -o ./iterate ./cmd/iterate

# Fetch GitHub issues (if community.go is implemented)
log "Fetching GitHub issues..."
if command -v gh &> /dev/null; then
  python3 scripts/format_issues.py > "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null || true
fi

# Phase A: Planning
log "Phase A: Planning..."
ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" ./iterate -p \
  "Read your source code, JOURNAL.md, and any issues in .iterate/ISSUES_TODAY.md.
   Write SESSION_PLAN.md with:
   - 3-5 focused improvement tasks
   - Issue responses section for each issue you want to address
   Then STOP and do not execute anything yet." \
  2>/dev/null || log "Planning phase completed with status $?"

# Phase B: Implementation
if [[ -f "$PLAN_FILE" ]]; then
  log "Phase B: Implementation..."
  ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" ./iterate -p \
    "Read SESSION_PLAN.md and implement each task step by step.
     For each task:
     1. Make changes
     2. Run: go build ./... && go test ./...
     3. If tests pass, commit. If not, revert and explain the failure.
     Work through all tasks in the plan." \
    2>/dev/null || log "Implementation phase completed with status $?"
else
  log "No SESSION_PLAN.md found, skipping implementation"
fi

# Phase C: Communication
log "Phase C: Communication..."
if [[ -f "$PLAN_FILE" ]]; then
  ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" ./iterate -p \
    "Read SESSION_PLAN.md and extract the 'Issue Responses' section.
     For each issue you addressed, post a GitHub comment with your summary.
     Use: gh issue comment <number> --body '...'" \
    2>/dev/null || log "Communication phase completed with status $?"
fi

# Update journal
log "Updating journal..."
DAY_COUNT=$(cat "${REPOPATH}/DAY_COUNT" 2>/dev/null || echo "1")
NEXT_DAY=$((DAY_COUNT + 1))
echo "$NEXT_DAY" > "${REPOPATH}/DAY_COUNT"

# Append to JOURNAL.md
{
  echo ""
  echo "## Day $NEXT_DAY ($(date -u +'%Y-%m-%d %H:%M:%S'))"
  echo ""
  if [[ -f "$PLAN_FILE" ]]; then
    head -20 "$PLAN_FILE" | tail -n +2
  else
    echo "Auto-evolution session completed."
  fi
} >> "${REPOPATH}/JOURNAL.md"

log "=== iterate evolution cycle completed ==="
log "DAY_COUNT advanced to $NEXT_DAY"
