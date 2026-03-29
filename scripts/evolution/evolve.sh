#!/bin/bash
set -e

# iterate evolution pipeline: plan → implement → pr → review → merge → communicate
# Autonomous evolution cycle — 6-phase self-evolving pipeline.
# Runs every 12h via GitHub Actions.

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/evolution.log"
PLAN_FILE="${REPOPATH}/SESSION_PLAN.md"
PID_FILE="${REPOPATH}/.iterate/evolve.pid"
LOCK_TIMEOUT=3600

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# ── Discord Notifications ──
send_discord_notification() {
  local status="$1"
  local message="$2"
  
  if [[ -z "${DISCORD_WEBHOOK_URL:-}" ]]; then
    return 0
  fi
  
  local color="5814783"  # Default blue
  local title="Evolution Day $DAY"
  
  case "$status" in
    "started")
      color="3447003"  # Blue
      title="🚀 Evolution Started - Day $DAY"
      ;;
    "success")
      color="3066993"  # Green
      title="✅ Evolution Complete - Day $DAY"
      ;;
    "failure")
      color="15158332" # Red
      title="❌ Evolution Failed - Day $DAY"
      ;;
    "retry")
      color="16776960" # Yellow
      title="🔄 Evolution Retrying - Day $DAY"
      ;;
  esac
  
  local pr_url=""
  if [[ -n "${PR_NUMBER:-}" ]]; then
    pr_url="https://github.com/$GITHUB_REPO/pull/$PR_NUMBER"
  fi
  
  local payload=$(jq -n \
    --arg title "$title" \
    --arg desc "$message" \
    --arg status "$status" \
    --arg pr "$pr_url" \
    --argjson color "$color" \
    --arg ts "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    '{embeds:[{title:$title,description:$desc,color:$color,timestamp:$ts,footer:{text:"iterate-evolve[bot]"}}]}')
  
  curl -s -H "Content-Type: application/json" -d "$payload" "$DISCORD_WEBHOOK_URL" >/dev/null 2>&1 || true
}

# ── Concurrent run lock ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$OLD_PID" ]] && kill -0 "$OLD_PID" 2>/dev/null; then
      LOCK_AGE=$(( $(date +%s) - $(stat -c %Y "$PID_FILE" 2>/dev/null || echo 0) ))
      if [[ $LOCK_AGE -lt $LOCK_TIMEOUT ]]; then
        log "ERROR: Another evolution running (PID $OLD_PID, age ${LOCK_AGE}s) — aborting"
        exit 1
      else
        log "WARNING: Stale lock found (PID $OLD_PID, age ${LOCK_AGE}s) — killing and continuing"
        kill "$OLD_PID" 2>/dev/null || true
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

mkdir -p "${REPOPATH}/.iterate" || { echo "ERROR: failed to create .iterate dir"; exit 1; }
mkdir -p "${REPOPATH}/memory" || { echo "ERROR: failed to create memory dir"; exit 1; }
mkdir -p "${REPOPATH}/docs" || { echo "ERROR: failed to create docs dir"; exit 1; }
acquire_lock

log "=== iterate evolution cycle started ==="

# ── Send Started Notification ──
send_discord_notification "started" "Evolution Day $DAY is starting...\n\n**Plan:**\n• Analyze codebase for bugs/improvements\n• Fix real code issues (not just metrics)\n• Auto-retry if review fails (max 2 retries)\n\nNext scheduled: 12 hours"

# ── Guards ──
if [[ -z "${OPENCODE_API_KEY:-}" ]]; then
  log "ERROR: OPENCODE_API_KEY not set"
  exit 1
fi

# ── Validate GitHub authentication ──
if command -v gh &>/dev/null; then
  GH_AUTH_STATUS=$(gh auth status 2>&1 || true)
  if echo "$GH_AUTH_STATUS" | grep -q "not logged in\|no credentials"; then
    log "ERROR: gh CLI not authenticated"
    exit 1
  fi
  log "GitHub authentication verified"
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

PR_STATE_FILE="${REPOPATH}/.iterate/pr_state.json"

# ── Resume detection: skip completed phases on re-run ──
# If pr_state.json exists, a PR was already created — skip phases 1-3.
if [[ -f "$PR_STATE_FILE" ]]; then
  log "Detected existing pr_state.json — resuming from Phase 4 (Review)"
else
  # ── Clean stale plan ──
  rm -f "$PLAN_FILE"

  # ── Phase 1: Planning ──
  log "Phase 1: Planning..."
  if ! ./iterate --phase plan --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
    log "WARNING: plan phase exited with error — checking for fallback"
  fi

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

Session Title: Find and fix real code issues

### Task 1: Search for bugs in core packages
Files: cmd/iterate/, internal/evolution/, internal/agent/
Description: Read the Go source code in these directories. Look for:
- Functions with missing error handling
- TODO comments that should be implemented
- Test files with low coverage
- Unused variables or imports
- Potential nil pointer dereferences
- Race conditions in concurrent code
Pick ONE concrete issue and fix it with proper tests.

### Task 2: Check for UX improvements
Files: cmd/iterate/repl.go, cmd/iterate/commands/
Description: Look for user-facing code that could be improved:
- Missing error messages
- Confusing command outputs
- Hardcoded values that should be configurable
- Missing help text
Pick ONE improvement and implement it.

### Task 3: Performance optimization
Files: Any Go files
Description: Look for:
- Inefficient loops
- Unnecessary allocations
- Missing context cancellation
- Blocking operations without timeouts
Pick ONE performance issue and optimize it.

Criteria: Only commit if the change includes BOTH the fix AND tests for the fix.
EOF
    fi
  fi

  # ── Phase 2: Implementation ──
  log "Phase 2: Implementation..."
  sleep 5  # Brief pause between phases
  if ! ./iterate --phase implement --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
    log "WARNING: implement phase exited with error — continuing"
  fi

  # Update DAY_COUNT before creating PR
  echo "$DAY" > "${REPOPATH}/DAY_COUNT"
  git add DAY_COUNT 2>/dev/null || true
  git diff --cached --quiet || git commit -m "chore: update DAY_COUNT to day $DAY" 2>/dev/null || true

  # ── Track coverage and generate stats before PR ──
  log "Tracking test coverage..."
  python3 scripts/build/track_coverage.py . 2>/dev/null || true
  git add memory/coverage_history.jsonl 2>/dev/null || true
  git diff --cached --quiet || git commit -m "chore: update coverage history" 2>/dev/null || true

  log "Generating stats..."
  python3 scripts/build/generate_stats.py . 2>/dev/null || true
  git add docs/stats.json memory/weekly_summary.md 2>/dev/null || true
  git diff --cached --quiet || git commit -m "chore: update stats" 2>/dev/null || true

  log "Generating dashboard..."
  python3 scripts/build/generate_dashboard.py . 2>/dev/null || true
  git add docs/dashboard.html 2>/dev/null || true
  git diff --cached --quiet || git commit -m "chore: update metrics dashboard" 2>/dev/null || true

  # ── Phase 3: Pull Request ──
  log "Phase 3: Pull Request..."
  sleep 5
  if ! ./iterate --phase pr --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
    log "WARNING: PR phase exited with error — continuing"
  fi
fi

BRANCH="evolution/day-${DAY}"
PR_NUMBER=$(gh pr list --repo "$GITHUB_REPO" --head "$BRANCH" --json number --jq '.[0].number' 2>/dev/null || echo "")

# ── Phase 4: Review with Auto-Retry ──
log "Phase 4: Review..."
sleep 5

REVIEW_RETRIES=0
MAX_REVIEW_RETRIES=2
REVIEW_PASSED=false

while [[ $REVIEW_RETRIES -lt $MAX_REVIEW_RETRIES ]] && [[ "$REVIEW_PASSED" == "false" ]]; do
  if ./iterate --phase review --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
    REVIEW_PASSED=true
    log "Review passed (attempt $((REVIEW_RETRIES + 1)))"
  else
    REVIEW_RETRIES=$((REVIEW_RETRIES + 1))
    log "WARNING: Review failed (attempt $REVIEW_RETRIES/$MAX_REVIEW_RETRIES)"
    
    if [[ $REVIEW_RETRIES -lt $MAX_REVIEW_RETRIES ]]; then
      log "Auto-retrying with enhanced context..."
      sleep 10
    else
      log "ERROR: Review failed after $MAX_REVIEW_RETRIES attempts — blocking merge"
      send_discord_notification "failure" "Review failed after $MAX_REVIEW_RETRIES attempts"
      exit 1
    fi
  fi
done

# ── Phase 5: Merge ──
log "Phase 5: Merge..."
sleep 5
if ! ./iterate --phase merge --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
  log "ERROR: merge phase failed"
  exit 1
fi

# ── Phase 6: Communication ──
log "Phase 6: Communication..."
sleep 5
if ! ./iterate --phase communicate --gh-owner GrayCodeAI --gh-repo iterate 2>>"$LOG_FILE"; then
  log "WARNING: communicate phase exited with error"
fi

# ── Verify journal was written ──
if grep -qP "^## Day ${DAY}(\s|$|—)" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null || grep -q "^## Day ${DAY} " "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  log "Journal entry written for Day $DAY"
else
  log "WARNING: No journal entry found for Day $DAY — writing fallback"
  SESSION_TIME_NOW=$(date -u +'%H:%M')
  python3 << PYEOF
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

# ── Cleanup stale branches ──
log "Cleaning up old evolution branches..."
gh api repos/"$GITHUB_REPO"/branches --jq '.[].name' 2>/dev/null | grep "^evolution/day-" | while read -r branch; do
  if [[ "$branch" == "$BRANCH" ]]; then
    continue
  fi
  PR_STATE=$(gh pr view --repo "$GITHUB_REPO" --json state --jq '.state' --head "$branch" 2>/dev/null || echo "")
  if [[ "$PR_STATE" == "MERGED" || "$PR_STATE" == "CLOSED" ]]; then
    log "Deleting stale branch: $branch (PR state: $PR_STATE)"
    git push origin --delete "$branch" 2>/dev/null || log "Failed to delete branch: $branch"
  fi
done

# ── Cost estimation ──
SESSION_DURATION=$SECONDS
log "Session duration: ${SESSION_DURATION}s"
log "Estimated cost: ~\$0.00 (depends on API usage)"

# ── Summary ──
log "=== evolution cycle completed ==="
log "Day: $DAY"
log "Branch: $BRANCH"
log "PR: #${PR_NUMBER:-none}"
log "Duration: ${SESSION_DURATION}s"

# ── Cleanup SESSION_PLAN.md ──
if [[ -f "$PLAN_FILE" ]]; then
  log "Cleaning up SESSION_PLAN.md..."
  rm -f "$PLAN_FILE"
  git add "$PLAN_FILE" 2>/dev/null || true
  git commit -m "chore: cleanup SESSION_PLAN.md after evolution" 2>/dev/null || true
  git push origin "$BRANCH" 2>/dev/null || true
fi

# ── Discord Success Notification ──
JOURNAL_ENTRY=$(grep -A3 "^## Day ${DAY} \|^## Day ${DAY}—" docs/JOURNAL.md 2>/dev/null | head -4 || echo "No journal entry")
SUCCESS_MSG=$(echo "Evolution completed successfully!

**Changes:**
• PR: #${PR_NUMBER:-none}
• Duration: ${SESSION_DURATION}s
• Review retries: $REVIEW_RETRIES

**Journal:**
$(echo "$JOURNAL_ENTRY" | head -2 | tr '\n' ' ')")

send_discord_notification "success" "$SUCCESS_MSG"
