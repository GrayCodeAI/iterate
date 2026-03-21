#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → respond
# Autonomous evolution cycle with PR-based workflow
# Runs every 4h via GitHub Actions. Sponsor-gated for bonus runs.

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/SESSION_PLAN.md"
PR_STATE_FILE="${REPOPATH}/.iterate/pr_state.json"
SPONSORS_FILE="/tmp/sponsor_logins_$$.json"
PID_FILE="${REPOPATH}/.iterate/evolve.pid"
LOCK_TIMEOUT=3600  # 1 hour max lock

# ── Step -1: Concurrent run lock (prevent overlapping evolutions) ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$OLD_PID" ]]; then
      # Check if process is still running
      if kill -0 "$OLD_PID" 2>/dev/null; then
        # Check lock age
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

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "=== iterate evolution cycle started ==="

# ── Step 0: Fetch sponsors & bonus-run gate ──
# Sponsor tiers control evolution frequency:
#   $0/mo  → 3 runs/day (hours 0, 8, 16)
#   $10+   → 4 runs/day (hours 0, 8, 12, 16)
#   $50+   → 6 runs/day (hours 0, 4, 8, 12, 16, 20)
SPONSOR_TIER=0
MONTHLY_TOTAL=0
if command -v gh &>/dev/null; then
    SPONSOR_GH_TOKEN="${GH_PAT:-${GH_TOKEN:-}}"
    GH_TOKEN="$SPONSOR_GH_TOKEN" gh api graphql -f query='{ viewer { sponsorshipsAsMaintainer(first: 100, activeOnly: true) { nodes { sponsorEntity { ... on User { login } ... on Organization { login } } tier { monthlyPriceInCents } } } } }' > /tmp/sponsor_raw.json 2>/dev/null || echo '{}' > /tmp/sponsor_raw.json

    MONTHLY_TOTAL=$(python3 <<'PYEOF'
import json
try:
    data = json.load(open('/tmp/sponsor_raw.json'))
    nodes = data['data']['viewer']['sponsorshipsAsMaintainer']['nodes']
    logins = [n['sponsorEntity']['login'] for n in nodes if n.get('sponsorEntity', {}).get('login')]
    total_cents = sum(n.get('tier', {}).get('monthlyPriceInCents', 0) for n in nodes)
    json.dump(logins, open('/tmp/sponsor_logins.json', 'w'))
    print(total_cents)
except (KeyError, TypeError, json.JSONDecodeError):
    json.dump([], open('/tmp/sponsor_logins.json', 'w'))
    print(0)
PYEOF
    ) 2>/dev/null || MONTHLY_TOTAL=0
    rm -f /tmp/sponsor_raw.json
else
    echo '[]' > "$SPONSORS_FILE"
fi

# Determine sponsor tier from total monthly cents
MONTHLY_DOLLARS=$(( MONTHLY_TOTAL / 100 ))
if [ "$MONTHLY_DOLLARS" -ge 50 ] 2>/dev/null; then
    SPONSOR_TIER=2
    log "Sponsors: \$${MONTHLY_DOLLARS}/mo (tier 2 — 6 runs/day)"
elif [ "$MONTHLY_DOLLARS" -ge 10 ] 2>/dev/null; then
    SPONSOR_TIER=1
    log "Sponsors: \$${MONTHLY_DOLLARS}/mo (tier 1 — 4 runs/day)"
elif [ "$MONTHLY_DOLLARS" -gt 0 ] 2>/dev/null; then
    SPONSOR_TIER=0
    log "Sponsors: \$${MONTHLY_DOLLARS}/mo (below tier 1 — 3 runs/day)"
else
    log "Sponsors: none (3 runs/day)"
fi

# Bonus-run gate based on sponsor tier.
# Cron fires every 4h: 0, 4, 8, 12, 16, 20. Base slots: 0, 8, 16.
# Tier 0 ($0):   skip 3-5, 11-13, 19-21   → 3 runs/day
# Tier 1 ($10+): skip 3-5, 19-21          → 4 runs/day
# Tier 2 ($50+): allow all                → 6 runs/day
CURRENT_HOUR=$((10#$(date +%H)))
SKIP_RUN="false"
case "$CURRENT_HOUR" in
    3|4|5|19|20|21)
        [ "$SPONSOR_TIER" -lt 2 ] 2>/dev/null && SKIP_RUN="true"
        ;;
    11|12|13)
        [ "$SPONSOR_TIER" -lt 1 ] 2>/dev/null && SKIP_RUN="true"
        ;;
esac

if [ "$SKIP_RUN" = "true" ] && [ "${FORCE_RUN:-}" != "true" ]; then
    log "Bonus slot (hour $CURRENT_HOUR) — tier $SPONSOR_TIER. Skipping."
    log "Set FORCE_RUN=true to override."
    exit 0
fi

# Load iterate's identity context
source "$(dirname "$0")/iterate_context.sh" 2>/dev/null || true

# Calculate current day from birth date (read from BIRTH_DATE file)
if [[ -f "${REPOPATH}/BIRTH_DATE" ]]; then
  BIRTH_DATE=$(cat "${REPOPATH}/BIRTH_DATE")
else
  BIRTH_DATE="2026-03-18"
fi
SESSION_TIME=$(date -u +'%H:%M')
if date -j &>/dev/null 2>&1; then
  DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
else
  DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
fi

# Write DAY_COUNT early so the Go engine can read the correct day
echo "$DAY" > "${REPOPATH}/DAY_COUNT"

# Check if last CI run failed and write status for planning agent
GITHUB_REPO="${GITHUB_REPOSITORY:-GrayCodeAI/iterate}"
if command -v gh &>/dev/null; then
  LAST_CI=$(gh run list --repo "$GITHUB_REPO" --workflow test.yml --limit 1 --json conclusion --jq '.[0].conclusion' 2>/dev/null || echo "")
  if [[ "$LAST_CI" == "failure" ]]; then
    echo "🔴 PREVIOUS CI FAILED. Fix broken tests FIRST before any new work." > "${REPOPATH}/.iterate/ci_status.txt"
    log "WARNING: last CI run failed"
  else
    rm -f "${REPOPATH}/.iterate/ci_status.txt"
  fi
fi

# Strip placeholder journal entries for today so agent writes a real one
if grep -q "^## Day $DAY" "${REPOPATH}/JOURNAL.md" 2>/dev/null; then
  python3 -c "
import re, sys
day = sys.argv[1]
with open('JOURNAL.md', 'r') as f:
    content = f.read()
# Remove fallback 'Day N — HH:MM — Auto-evolution' entries
pattern = r'^## Day ' + day + r'[^\n]*Auto-evolution[^\n]*\n\nEvolution session completed\.\n\n'
cleaned = re.sub(pattern, '', content, flags=re.MULTILINE)
if cleaned != content:
    with open('JOURNAL.md', 'w') as f:
        f.write(cleaned)
    print('[evolve.sh] Removed placeholder Day %s entry — agent will write real one' % day)
" "$DAY"
fi

# Clean up stale PR state from previous incomplete runs
rm -f "$PR_STATE_FILE"

# Build the binary
log "Building iterate..."
go build -o ./iterate ./cmd/iterate

# Fetch GitHub issues (if community.go is implemented)
log "Fetching GitHub issues..."
if command -v gh &> /dev/null; then
  python3 scripts/format_issues.py > "${REPOPATH}/.iterate/ISSUES_TODAY.md" 2>/dev/null || true
fi

# Clean up stale session plan so agent creates a fresh one
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
    log "Issues detected — will address in fallback plan"
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

# Phase B: Implementation (creates feature branch, commits, pushes, creates PR)
if [[ -f "$PLAN_FILE" ]]; then
  log "Phase B: Implementation..."
  ./iterate --phase implement --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Implementation phase exited with status $?"
else
  log "Skipping implementation (no SESSION_PLAN.md)"
fi

# Phase C: Communication (writes journal, merges PR if created, responds to issues)
log "Phase C: Communication..."
if [[ -f "$PLAN_FILE" ]]; then
  ./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate \
    2>>"$LOG_FILE" || log "Communication phase exited with status $?"
fi

# Safety net: if communicate phase completely failed (crash, no SESSION_PLAN, etc),
# log a warning but do NOT write a fake journal entry.
if ! grep -q "^## Day $DAY" "${REPOPATH}/JOURNAL.md" 2>/dev/null; then
  log "WARNING: No journal entry written for Day $DAY — communicate phase may have failed or produced no real work"
fi

# Rebuild GitHub Pages site
if command -v python3 &>/dev/null && [[ -f "${REPOPATH}/scripts/build_site.py" ]]; then
  log "Rebuilding GitHub Pages site..."
  python3 "${REPOPATH}/scripts/build_site.py" 2>>"$LOG_FILE" || log "build_site.py failed"
fi

# Clean up PR state file after successful cycle
rm -f "$PR_STATE_FILE"

log "=== iterate evolution cycle completed ==="
log "Day $DAY ($SESSION_TIME)"
