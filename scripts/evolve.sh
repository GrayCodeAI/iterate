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
./iterate --phase plan --gh-owner GrayCodeAI --gh-repo iterate \
  2>/dev/null || log "Planning phase completed with status $?"

# Phase B: Implementation
if [[ -f "$PLAN_FILE" ]]; then
  log "Phase B: Implementation..."
  ./iterate --phase implement \
    2>/dev/null || log "Implementation phase completed with status $?"
else
  log "No SESSION_PLAN.md found, skipping implementation"
fi

# Phase C: Communication
log "Phase C: Communication..."
if [[ -f "$PLAN_FILE" ]]; then
  ./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate \
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
