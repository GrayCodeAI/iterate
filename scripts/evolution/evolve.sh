#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → communicate
# Autonomous evolution cycle — commits directly to main.
# Runs every 12h via GitHub Actions.

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/SESSION_PLAN.md"
PID_FILE="${REPOPATH}/.iterate/evolve.pid"
LOCK_TIMEOUT=3600

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# ── Concurrent run lock ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$OLD_PID" ]] && kill -0 "$OLD_PID" 2>/dev/null; then
      LOCK_AGE=$(( $(date +%s) - $(stat -c %Y "$PID_FILE" 2>/dev/null || echo 0) ))
      if [[ $LOCK_AGE -lt $LOCK_TIMEOUT ]]; then
        log "ERROR: Another evolution running (PID $OLD_PID, age ${LOCK_AGE}s)"
        exit 1
      fi
    fi
    rm -f "$PID_FILE"
  fi
  echo $$ > "$PID_FILE"
}

release_lock() {
  rm -f "$PID_FILE"
}

trap release_lock EXIT

mkdir -p "${REPOPATH}/.iterate"
acquire_lock

log "=== iterate evolution cycle started ==="

# ── Guards ──
if [[ -z "${OPENCODE_API_KEY:-}" ]]; then
  log "ERROR: OPENCODE_API_KEY not set"
  exit 1
fi

# ── Calculate day from BIRTH_DATE ──
BIRTH_DATE=$(cat "${REPOPATH}/BIRTH_DATE" 2>/dev/null || echo "2026-03-25")
SESSION_TIME=$(date -u +'%H:%M')
if date -d "$BIRTH_DATE" +%s &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
elif date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
else
  DAY=0
fi
echo "$DAY" > "${REPOPATH}/DAY_COUNT"
log "Day $DAY ($SESSION_TIME UTC)"

# ── Check CI status ──
GITHUB_REPO="${GITHUB_REPOSITORY:-GrayCodeAI/iterate}"
if command -v gh &>/dev/null; then
  LAST_CI=$(gh run list --repo "$GITHUB_REPO" --workflow ci.yml --limit 1 --json conclusion --jq '.[0].conclusion' 2>/dev/null || echo "")
  if [[ "$LAST_CI" == "failure" ]]; then
    echo "PREVIOUS CI FAILED. Fix broken tests FIRST." > "${REPOPATH}/.iterate/ci_status.txt"
    log "WARNING: last CI run failed"
  else
    rm -f "${REPOPATH}/.iterate/ci_status.txt"
  fi
fi

# ── Build ──
log "Building iterate..."
go build -o ./iterate ./cmd/iterate

# ── Fetch issues ──
log "Fetching GitHub issues..."
rm -f "${REPOPATH}/.iterate/ISSUES_TODAY.md"
if command -v gh &>/dev/null; then
  python3 scripts/build/format_issues.py > "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null || true
fi

HAS_ISSUES=false
if [[ -f "${REPOPATH}/.iterate/ISSUES_TODAY.md" ]] && grep -q "Issue #" "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null; then
  HAS_ISSUES=true
fi

# ── Clean stale plan ──
rm -f "$PLAN_FILE"

# ── Phase A: Planning ──
log "Phase A: Planning..."
./iterate --phase plan --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE" || true

# Fallback plan if agent didn't create one
if [[ ! -f "$PLAN_FILE" ]]; then
  log "Agent did not create SESSION_PLAN.md — writing fallback"
  if [[ "$HAS_ISSUES" == "true" ]]; then
    cat > "$PLAN_FILE" <<'EOF'
## Session Plan

Session Title: Address community issues

### Task 1: Review and address community feedback
Files: cmd/iterate/, internal/evolution/, .iterate/ISSUES_TODAY.md
Description: Read the community issues from .iterate/ISSUES_TODAY.md and implement useful features or fixes.
Issue: multiple

### Issue Responses
- TBD
EOF
  else
    cat > "$PLAN_FILE" <<'EOF'
## Session Plan

Session Title: General self-improvement

### Task 1: Self-assessment and improvement
Files: cmd/iterate/, internal/evolution/
Description: Read the source code, find one thing to improve (a bug, missing test, or UX gap), implement it, test it, and commit it.
Issue: none

### Issue Responses
EOF
  fi
fi

# ── Phase B: Implementation ──
log "Phase B: Implementation..."
./iterate --phase implement --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE" || true

# ── Phase C: Communication ──
log "Phase C: Communication..."
./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE" || true

# ── Verify journal was written ──
if grep -q "^## Day $DAY" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  log "Journal entry written for Day $DAY"
else
  log "WARNING: No journal entry found for Day $DAY — writing fallback"
  SESSION_TIME_NOW=$(date -u +'%H:%M')
  # Insert fallback entry after header
  python3 << PYEOF
import sys
header = '# iterate Evolution Journal\n'
day = "$DAY"
time_now = "$SESSION_TIME_NOW"
entry = f"## Day {day} — {time_now} — Evolution session\n\nEvolution session completed.\n"
with open('docs/JOURNAL.md', 'r') as f:
    content = f.read()
if not content.startswith(header):
    content = header + '\n' + content
rest = content[len(header):].lstrip('\n')
with open('docs/JOURNAL.md', 'w') as f:
    f.write(header + '\n' + entry + '\n' + rest)
PYEOF
  git add docs/JOURNAL.md
  git commit -m "journal: Day $DAY fallback entry" 2>/dev/null || true
fi

# ── Track coverage ──
log "Tracking test coverage..."
python3 scripts/build/track_coverage.py . 2>/dev/null || true
git add memory/coverage_history.jsonl 2>/dev/null || true

# ── Generate stats ──
log "Generating stats..."
python3 scripts/build/generate_stats.py . 2>/dev/null || true
git add docs/stats.json memory/weekly_summary.md 2>/dev/null || true

# ── Final commit and push ──
log "Pushing changes..."
if [[ -n $(git status -s) ]]; then
  git add -A
  git commit -m "iterate: Day $DAY evolution session" 2>/dev/null || true
fi
git pull --rebase origin main 2>/dev/null || true
git push origin main 2>/dev/null || log "Push failed"

log "=== evolution cycle completed ==="
