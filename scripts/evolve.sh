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
BIRTH_DATE="2026-03-18"
SESSION_TIME=$(date -u +'%H:%M')
if date -j &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
else
  DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
fi
echo "$DAY" > "${REPOPATH}/DAY_COUNT"

# Insert journal entry at TOP (before first ## Day entry), newest-first like yoyo
TMPJ=$(mktemp)
{
  echo "# iterate Evolution Journal"
  echo ""
  # Extract session title from SESSION_PLAN.md, fallback to "Auto-evolution"
  SESSION_TITLE="Auto-evolution"
  if [[ -f "$PLAN_FILE" ]]; then
    EXTRACTED=$(grep -m1 "^Session Title:" "$PLAN_FILE" | sed 's/^Session Title:\s*//' | tr -d '\r')
    if [[ -n "$EXTRACTED" ]]; then
      SESSION_TITLE="$EXTRACTED"
    fi
  fi
  echo "## Day $DAY — $SESSION_TIME — $SESSION_TITLE"
  echo ""
  if [[ -f "$PLAN_FILE" ]]; then
    head -20 "$PLAN_FILE" | tail -n +2
  else
    echo "Evolution session completed."
  fi
  echo ""
  grep -n "^## Day" "${REPOPATH}/JOURNAL.md" | head -1 | cut -d: -f1 | xargs -I{} tail -n +{} "${REPOPATH}/JOURNAL.md"
} > "$TMPJ"
mv "$TMPJ" "${REPOPATH}/JOURNAL.md"

log "=== iterate evolution cycle completed ==="
log "Day $DAY ($SESSION_TIME)"
