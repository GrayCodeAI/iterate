#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → communicate
# Autonomous evolution cycle with PR-based workflow
# Runs every 4h via GitHub Actions.

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/docs/SESSION_PLAN.md"
PR_STATE_FILE="${REPOPATH}/.iterate/pr_state.json"
PID_FILE="${REPOPATH}/.iterate/evolve.pid"
LOCK_TIMEOUT=3600  # 1 hour max lock

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# ── Concurrent run lock (prevent overlapping evolutions) ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$OLD_PID" ]]; then
      if kill -0 "$OLD_PID" 2>/dev/null; then
        LOCK_AGE=$(( $(date +%s) - $(stat -f %m "$PID_FILE" 2>/dev/null || stat -c %Y "$PID_FILE" 2>/dev/null || echo 0) ))
        if [[ $LOCK_AGE -lt $LOCK_TIMEOUT ]]; then
          log "ERROR: Another evolution is running (PID $OLD_PID, age ${LOCK_AGE}s)"
          exit 1
        else
          log "WARNING: Stale lock found (age ${LOCK_AGE}s), removing"
          rm -f "$PID_FILE"
        fi
      else
        log "Removing stale lock file (process $OLD_PID not running)"
        rm -f "$PID_FILE"
      fi
    fi
  fi
  echo $$ > "$PID_FILE"
  log "Acquired lock (PID $$)"
}

release_lock() {
  rm -f "$PID_FILE"
  log "Released lock"
}

trap release_lock EXIT

mkdir -p "${REPOPATH}/.iterate"
acquire_lock

log "=== iterate evolution cycle started ==="

# ── Guard: require API key ──
if [[ -z "${OPENCODE_API_KEY:-}" ]]; then
  log "ERROR: OPENCODE_API_KEY is not set. Add it as a GitHub Actions secret."
  exit 1
fi

# Calculate current day from birth date
if [[ -f "${REPOPATH}/BIRTH_DATE" ]]; then
  BIRTH_DATE=$(cat "${REPOPATH}/BIRTH_DATE")
else
  BIRTH_DATE="2026-03-22"
fi
SESSION_TIME=$(date -u +'%H:%M')
if date -j &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
else
  DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
fi

# Write DAY_COUNT so the Go engine can read the correct day
echo "$DAY" > "${REPOPATH}/DAY_COUNT"
log "Day $DAY"

# Check if last CI run failed and write status for planning agent
GITHUB_REPO="${GITHUB_REPOSITORY:-GrayCodeAI/iterate}"
if command -v gh &>/dev/null; then
  LAST_CI=$(gh run list --repo "$GITHUB_REPO" --workflow ci.yml --limit 1 --json conclusion --jq '.[0].conclusion' 2>/dev/null || echo "")
  if [[ "$LAST_CI" == "failure" ]]; then
    echo "🔴 PREVIOUS CI FAILED. Fix broken tests FIRST before any new work." > "${REPOPATH}/.iterate/ci_status.txt"
    log "WARNING: last CI run failed"
  else
    rm -f "${REPOPATH}/.iterate/ci_status.txt"
  fi
fi

# Strip placeholder journal entries for today so agent writes a real one
if grep -q "^## Day $DAY" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  python3 -c "
import re, sys
day = sys.argv[1]
journal_path = 'docs/JOURNAL.md'
with open(journal_path, 'r') as f:
    content = f.read()
pattern = r'^## Day ' + day + r'[^\n]*Auto-evolution[^\n]*\n\nEvolution session completed\.\n\n'
cleaned = re.sub(pattern, '', content, flags=re.MULTILINE)
if cleaned != content:
    with open(journal_path, 'w') as f:
        f.write(cleaned)
    print('[evolve.sh] Removed placeholder Day %s entry' % day)
" "$DAY"
fi

# Clean up stale PR state
rm -f "$PR_STATE_FILE"

# Build the binary
log "Building iterate..."
go build -o ./iterate ./cmd/iterate

# Fetch GitHub issues
log "Fetching GitHub issues..."
if command -v gh &>/dev/null; then
  python3 scripts/build/format_issues.py > "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null || true
fi

# Clean up stale session plan
rm -f "$PLAN_FILE"

# Check if we have issues to address
HAS_ISSUES=false
if [[ -f "${REPOPATH}/.iterate/ISSUES_TODAY.md" ]] && grep -q "Issue #" "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null; then
  HAS_ISSUES=true
  log "Issues detected, requiring proper plan..."
fi

# Phase A: Planning
log "Phase A: Planning..."
./iterate --phase plan --gh-owner GrayCodeAI --gh-repo iterate \
  2>>"$LOG_FILE" || log "Planning phase exited with status $?"

if [[ ! -f "$PLAN_FILE" ]]; then
  log "WARNING: SESSION_PLAN.md not created — writing fallback plan"
  if [[ "$HAS_ISSUES" == "true" ]]; then
    cat > "$PLAN_FILE" <<'PLAN'
## Session Plan

Session Title: Address community issues

### Task 1: Review and address community feedback
Files: cmd/iterate/, internal/evolution/, .iterate/ISSUES_TODAY.md
Description: Read the community issues from .iterate/ISSUES_TODAY.md and implement useful features or fixes.
Issue: multiple

### Issue Responses
- TBD

PLAN
  else
    cat > "$PLAN_FILE" <<'PLAN'
## Session Plan

Session Title: General self-improvement

### Task 1: Self-assessment and improvement
Files: cmd/iterate/, internal/evolution/
Description: Read the source code, find one thing to improve (a bug, missing test, or UX gap), implement it, test it, and commit it.
Issue: none

### Issue Responses
PLAN
  fi
fi

# Verify plan addresses issues if we have them
if [[ "$HAS_ISSUES" == "true" ]] && ! grep -q "implement\|wontfix\|partial" "$PLAN_FILE" 2>/dev/null; then
  log "ERROR: Plan does not address issues — retrying plan phase..."
  rm -f "$PLAN_FILE"
  ./iterate --phase plan --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Planning phase retry exited with status $?"
fi

# Phase B: Implementation
if [[ -f "$PLAN_FILE" ]]; then
  log "Phase B: Implementation (waiting 60s for rate limit reset)..."
  sleep 60
  ./iterate --phase implement --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Implementation phase exited with status $?"
else
  log "Skipping implementation (no SESSION_PLAN.md)"
fi

# Phase C: Communication
log "Phase C: Communication..."
if [[ -f "$PLAN_FILE" ]]; then
  sleep 30
  ./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Communication phase exited with status $?"
fi

if ! grep -q "^## Day $DAY" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  log "WARNING: No journal entry written for Day $DAY"
fi

# Rebuild GitHub Pages site
if command -v python3 &>/dev/null && [[ -f "${REPOPATH}/scripts/build/build_site.py" ]]; then
  log "Rebuilding GitHub Pages site..."
  python3 "${REPOPATH}/scripts/build/build_site.py" 2>>"$LOG_FILE" || log "build_site.py failed"
fi

rm -f "$PR_STATE_FILE"

log "=== iterate evolution cycle completed ==="
log "Day $DAY ($SESSION_TIME)"
