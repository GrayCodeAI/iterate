#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → respond
# Autonomous evolution cycle with PR-based workflow

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/SESSION_PLAN.md"
PR_STATE_FILE="${REPOPATH}/.iterate/pr_state.json"

mkdir -p "${REPOPATH}/.iterate"

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "=== iterate evolution cycle started ==="

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

# Safety net: if communicate phase completely failed to write a journal,
# write a minimal fallback. The Go engine should always write one now,
# but this catches edge cases (crash, no SESSION_PLAN, etc).
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

# Rebuild GitHub Pages site
if command -v python3 &>/dev/null && [[ -f "${REPOPATH}/scripts/build_site.py" ]]; then
  log "Rebuilding GitHub Pages site..."
  python3 "${REPOPATH}/scripts/build_site.py" 2>>"$LOG_FILE" || log "build_site.py failed"
fi

# Clean up PR state file after successful cycle
rm -f "$PR_STATE_FILE"

log "=== iterate evolution cycle completed ==="
log "Day $DAY ($SESSION_TIME)"
