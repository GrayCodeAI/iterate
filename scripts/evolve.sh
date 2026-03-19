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

# Load iterate's identity context
source "$(dirname "$0")/iterate_context.sh" 2>/dev/null || true

# Check if last CI run failed and write status for planning agent
if command -v gh &>/dev/null; then
  LAST_CI=$(gh run list --repo GrayCodeAI/iterate --workflow test.yml --limit 1 --json conclusion --jq '.[0].conclusion' 2>/dev/null || echo "")
  if [[ "$LAST_CI" == "failure" ]]; then
    echo "🔴 PREVIOUS CI FAILED. Fix broken tests FIRST before any new work." > "${REPOPATH}/.iterate/ci_status.txt"
    log "WARNING: last CI run failed"
  else
    rm -f "${REPOPATH}/.iterate/ci_status.txt"
  fi
fi

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
  2>>"$LOG_FILE" || log "Planning phase exited with status $?"

if [[ ! -f "$PLAN_FILE" ]]; then
  log "ERROR: SESSION_PLAN.md not created — check $LOG_FILE for details"
fi

# Phase B: Implementation
if [[ -f "$PLAN_FILE" ]]; then
  log "Phase B: Implementation..."
  ./iterate --phase implement \
    2>>"$LOG_FILE" || log "Implementation phase exited with status $?"
else
  log "Skipping implementation (no SESSION_PLAN.md)"
fi

# Phase C: Communication
log "Phase C: Communication..."
if [[ -f "$PLAN_FILE" ]]; then
  ./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Communication phase exited with status $?"
fi

# Update DAY_COUNT from birth date
BIRTH_DATE="2026-03-18"
SESSION_TIME=$(date -u +'%H:%M')
if date -j &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
else
  DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
fi
echo "$DAY" > "${REPOPATH}/DAY_COUNT"

# Journal is written by the agent in the communicate phase.
# If agent skipped it (e.g. no SESSION_PLAN), write a minimal fallback.
if ! grep -q "^## Day $DAY" "${REPOPATH}/JOURNAL.md" 2>/dev/null; then
  log "Agent did not write journal — writing fallback entry"
  TMPJ=$(mktemp)
  {
    echo "# iterate Evolution Journal"
    echo ""
    echo "## Day $DAY — $SESSION_TIME — Auto-evolution"
    echo ""
    echo "Evolution session completed."
    echo ""
    grep -n "^## Day" "${REPOPATH}/JOURNAL.md" | head -1 | cut -d: -f1 | xargs -I{} tail -n +{} "${REPOPATH}/JOURNAL.md" 2>/dev/null || tail -n +2 "${REPOPATH}/JOURNAL.md"
  } > "$TMPJ"
  mv "$TMPJ" "${REPOPATH}/JOURNAL.md"
fi

log "=== iterate evolution cycle completed ==="
log "Day $DAY ($SESSION_TIME)"
