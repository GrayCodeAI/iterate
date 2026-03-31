#!/bin/bash
set -uo pipefail

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

  local color="5814783"
  local title="Evolution Day $DAY"

  case "$status" in
    "started")
      color="3447003"
      title="Evolution Started - Day $DAY"
      ;;
    "success")
      color="3066993"
      title="Evolution Complete - Day $DAY"
      ;;
    "failure")
      color="15158332"
      title="Evolution Failed - Day $DAY"
      ;;
    "retry")
      color="16776960"
      title="Evolution Retrying - Day $DAY"
      ;;
  esac

  local pr_url=""
  if [[ -n "${PR_NUMBER:-}" ]]; then
    pr_url="https://github.com/$GITHUB_REPO/pull/$PR_NUMBER"
  fi

  local payload
  payload=$(python3 -c "
import json,sys
print(json.dumps({'embeds':[{'title':sys.argv[1],'description':sys.argv[2],'color':int(sys.argv[3]),'timestamp':sys.argv[4],'footer':{'text':'iterate-evolve[bot]'}}]}),
  '$title','$message','$color','$(date -u +'%Y-%m-%dT%H:%M:%SZ')')" 2>/dev/null) || return 0

  curl -s -H "Content-Type: application/json" -d "$payload" "$DISCORD_WEBHOOK_URL" >/dev/null 2>&1 || true
}

# ── Concurrent run lock ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    LOCK_LINE=$(cat "$PID_FILE" 2>/dev/null) || true
    OLD_PID=$(echo "$LOCK_LINE" | awk '{print $1}')
    OLD_START=$(echo "$LOCK_LINE" | awk '{print $2}')
    if [[ -n "$OLD_PID" ]] && kill -0 "$OLD_PID" 2>/dev/null; then
      # Verify PID start time matches lock file to avoid PID recycling
      if [[ -n "$OLD_START" ]]; then
        ACTUAL_START=$(ps -o lstart= -p "$OLD_PID" 2>/dev/null | tr -s ' ' '_' || echo "")
        if [[ "$ACTUAL_START" != "$OLD_START" ]]; then
          log "WARNING: PID $OLD_PID recycled — stale lock, removing"
          rm -f "$PID_FILE"
          echo "$$ $(ps -o lstart= -p $$ 2>/dev/null | tr -s ' ' '_')" > "$PID_FILE"
          return
        fi
      fi
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
  echo "$$ $(ps -o lstart= -p $$ 2>/dev/null | tr -s ' ' '_')" > "$PID_FILE"
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

# ── API Key & Model Rotation Setup ──
API_KEYS=()
[[ -n "${OPENCODE_API_KEY:-}" ]] && API_KEYS+=("$OPENCODE_API_KEY")
[[ -n "${OPENCODE_API_KEY_2:-}" ]] && API_KEYS+=("$OPENCODE_API_KEY_2")
CURRENT_KEY_INDEX=0

MODELS=()
[[ -n "${ITERATE_MODEL:-}" ]] && MODELS+=("$ITERATE_MODEL")
[[ -n "${ITERATE_MODEL_2:-}" ]] && MODELS+=("$ITERATE_MODEL_2")
CURRENT_MODEL_INDEX=0

rotate_api_key() {
  local next_index=$((CURRENT_KEY_INDEX + 1))
  if [[ $next_index -lt ${#API_KEYS[@]} ]] && [[ -n "${API_KEYS[$next_index]}" ]]; then
    CURRENT_KEY_INDEX=$next_index
    export OPENCODE_API_KEY="${API_KEYS[$CURRENT_KEY_INDEX]}"
    log "Rotated to API key #$((CURRENT_KEY_INDEX + 1))"
    return 0
  fi
  return 1
}

rotate_model() {
  local next_index=$((CURRENT_MODEL_INDEX + 1))
  if [[ $next_index -lt ${#MODELS[@]} ]]; then
    CURRENT_MODEL_INDEX=$next_index
    export ITERATE_MODEL="${MODELS[$CURRENT_MODEL_INDEX]}"
    log "Rotated to model: $ITERATE_MODEL"
    return 0
  fi
  return 1
}

# ── Guards ──
if [[ -z "${OPENCODE_API_KEY:-}" ]]; then
  log "ERROR: OPENCODE_API_KEY not set"
  exit 1
fi

log "Provider: ${ITERATE_PROVIDER:-opencode}, Models: ${MODELS[*]:-glm-5}"
log "API keys: ${#API_KEYS[@]} configured"

# ── Run phase with retry + key/model rotation ──
run_with_rotation() {
  local phase="$1"
  local max_retries=4
  local attempt=1

  while [[ $attempt -le $max_retries ]]; do
    local cmd_args=(./iterate --phase "$phase" --gh-owner "${GH_OWNER:-GrayCodeAI}" --gh-repo "${GH_REPO:-iterate}")
    [[ -n "$ITERATE_MODEL" ]] && cmd_args+=(--model "$ITERATE_MODEL")
    [[ -n "$ITERATE_PROVIDER" ]] && cmd_args+=(--provider "$ITERATE_PROVIDER")

    log "Running phase $phase (attempt $attempt/$max_retries, model: ${ITERATE_MODEL:-default})..."

    local phase_output
    phase_output=$("${cmd_args[@]}" 2>&1)
    local phase_exit=$?

    # Always log phase output
    if [[ -n "$phase_output" ]]; then
      echo "$phase_output" >> "$LOG_FILE"
    fi

    if [[ $phase_exit -eq 0 ]]; then
      log "Phase $phase completed successfully"
      return 0
    fi

    log "Phase $phase failed (attempt $attempt/$max_retries)"

    # Check if it's a rate limit / quota error
    if echo "$phase_output" | grep -qi "rate.*limit\|quota.*exceeded\|429\|SubscriptionUsageLimit"; then
      if [[ $attempt -lt $max_retries ]]; then
        if rotate_api_key; then
          log "Rate limited — rotated key, retrying..."
        elif rotate_model; then
          log "Rate limited — rotated model, retrying..."
        else
          log "Rate limited — all keys and models exhausted, waiting 30s..."
          sleep 30
        fi
      fi
    else
      # Non-rate-limit failure — rotate model and retry
      if [[ $attempt -lt $max_retries ]]; then
        if rotate_model; then
          log "Rotated model, retrying..."
        else
          log "Retrying phase $phase in 10s..."
          sleep 10
        fi
      fi
    fi
    ((attempt++))
  done

  log "Phase $phase failed after $max_retries attempts"
  return 1
}

# ── Validate GitHub authentication ──
if command -v gh &>/dev/null; then
  GH_AUTH_STATUS=$(gh auth status 2>&1 || true)
  if echo "$GH_AUTH_STATUS" | grep -q "not logged in\|no credentials"; then
    log "ERROR: gh CLI not authenticated"
    exit 1
  fi
  log "GitHub authenticated: $(echo "$GH_AUTH_STATUS" | head -1)"
fi

# ── Calculate day from BIRTH_DATE ──
BIRTH_DATE=$(cat "${REPOPATH}/BIRTH_DATE" 2>/dev/null || echo "2026-03-31")
SESSION_TIME=$(date -u +'%H:%M')

# Read existing DAY_COUNT or calculate from birth date
if [[ -f "${REPOPATH}/DAY_COUNT" ]]; then
  DAY=$(cat "${REPOPATH}/DAY_COUNT" 2>/dev/null || echo "0")
  if ! [[ "$DAY" =~ ^[0-9]+$ ]]; then
    DAY=0
  fi
else
  if date -d "$BIRTH_DATE" +%s &>/dev/null 2>&1; then
    DAY=$(( ($(date -u +%s) - $(date -d "$BIRTH_DATE" +%s)) / 86400 ))
  elif date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s &>/dev/null 2>&1; then
    DAY=$(( ($(date -u +%s) - $(date -j -f "%Y-%m-%d" "$BIRTH_DATE" +%s)) / 86400 ))
  else
    DAY=0
  fi
  echo "$DAY" > "${REPOPATH}/DAY_COUNT"
fi
log "Day $DAY ($SESSION_TIME UTC)"

# ── Pre-flight checks ──
preflight_checks() {
  if ! command -v go &>/dev/null; then
    log "ERROR: go not found"
    return 1
  fi
  log "Go version: $(go version)"
  return 0
}

if ! preflight_checks; then
  send_discord_notification "failure" "Pre-flight checks failed"
  exit 1
fi

# ── Send Started Notification ──
send_discord_notification "started" "Evolution Day $DAY starting"

# ── Check CI status ──
GITHUB_REPO="${GITHUB_REPOSITORY:-GrayCodeAI/iterate}"
GH_OWNER="${GITHUB_REPOSITORY%%/*}"
GH_REPO="${GITHUB_REPOSITORY#*/}"
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
if [[ -f "$PR_STATE_FILE" ]]; then
  log "Detected existing pr_state.json — resuming from Phase 4 (Review)"
else
  # ── Clean stale plan ──
  rm -f "$PLAN_FILE"

  # ── Phase 1: Planning ──
  log "Phase 1: Planning..."
  if ! run_with_rotation "plan"; then
    log "WARNING: plan phase exited with error — checking for fallback"
  fi

  # Fallback plan if agent didn't create one
  if [[ ! -f "$PLAN_FILE" ]]; then
    log "Agent did not create SESSION_PLAN.md — writing fallback"
    if [[ "$HAS_ISSUES" == "true" ]]; then
      cat > "$PLAN_FILE" <<EOF
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
      cat > "$PLAN_FILE" <<EOF
## Session Plan

Session Title: Day $DAY evolution — code quality and reliability

### Task 1: Fix error handling gaps
Files: cmd/iterate/, internal/
Description: Find functions that ignore errors (using _ or not checking return values). Add proper error handling with descriptive messages. Write a test that validates the error path.

### Task 2: Add missing tests
Files: internal/
Description: Find exported functions without corresponding tests. Write at least one test per function covering the happy path and one edge case.

### Task 3: Clean up code smells
Files: cmd/iterate/, internal/
Description: Look for: defer in loops, unused variables/imports, hardcoded values that should be constants, missing context propagation. Fix one issue with a test.

### Task 4: Improve documentation
Files: cmd/iterate/, internal/
Description: Add or improve Go doc comments on exported functions that are missing them. This is a lower-priority task.

Criteria: Each task must modify at least one .go source file. Tests are encouraged but not mandatory for small fixes.
EOF
    fi
  fi

  # ── Phase 2: Implementation ──
  log "Phase 2: Implementation..."
  if ! run_with_rotation "implement"; then
    log "WARNING: implement phase exited with error — continuing"
  fi

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
  if ! run_with_rotation "pr"; then
    log "WARNING: PR phase exited with error — continuing"
  fi
fi

BRANCH="evolution/day-${DAY}"
PR_NUMBER=$(gh pr list --repo "$GITHUB_REPO" --head "$BRANCH" --json number --jq '.[0].number' 2>/dev/null || echo "")

PIPELINE_OK=true

# ── Phase 4: Review with Auto-Retry ──
log "Phase 4: Review..."

REVIEW_RETRIES=0
MAX_REVIEW_RETRIES=2
REVIEW_PASSED=false

while [[ $REVIEW_RETRIES -lt $MAX_REVIEW_RETRIES ]] && [[ "$REVIEW_PASSED" == "false" ]]; do
  if run_with_rotation "review"; then
    REVIEW_PASSED=true
    log "Review passed (attempt $((REVIEW_RETRIES + 1)))"
  else
    REVIEW_RETRIES=$((REVIEW_RETRIES + 1))
    log "WARNING: Review failed (attempt $REVIEW_RETRIES/$MAX_REVIEW_RETRIES)"

    if [[ $REVIEW_RETRIES -lt $MAX_REVIEW_RETRIES ]]; then
      log "Auto-retrying review..."
      sleep 10
    fi
  fi
done

if [[ "$REVIEW_PASSED" == "false" ]]; then
  log "WARNING: Review did not pass after $MAX_REVIEW_RETRIES attempts — will still attempt merge"
  send_discord_notification "retry" "Review did not pass after $MAX_REVIEW_RETRIES attempts, attempting merge anyway"
fi

# ── Phase 5: Merge ──
log "Phase 5: Merge..."
if ! run_with_rotation "merge"; then
  log "WARNING: merge phase failed — PR may still be open"
  PIPELINE_OK=false
fi

# ── Phase 6: Communication (always attempt, even if earlier phases failed) ──
log "Phase 6: Communication..."
if ! run_with_rotation "communicate"; then
  log "WARNING: communicate phase exited with error"
fi

# ── Post-merge: write journal and DAY_COUNT directly to main ──
# In CI we're on a detached HEAD after merge, so we must checkout main first
log "Switching to main for journal and DAY_COUNT updates..."
git checkout main 2>/dev/null || git checkout -B main 2>/dev/null || true
git pull origin main 2>/dev/null || true

# Write journal entry
if grep -q "## Day ${DAY}" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  log "Journal entry already exists for Day $DAY"
else
  log "Writing journal entry for Day $DAY"
  SESSION_TIME_NOW=$(date -u +'%H:%M')
  cat >> "${REPOPATH}/docs/JOURNAL.md" <<JEOF

## Day ${DAY} — ${SESSION_TIME_NOW} — Evolution session completed

Evolution session completed. Pipeline status: $([ "$PIPELINE_OK" == "true" ] && echo "success" || echo "partial")
JEOF
fi
git add docs/JOURNAL.md 2>/dev/null || true
git diff --cached --quiet || git commit -m "journal: Day $DAY session entry" 2>/dev/null || true

# Increment DAY_COUNT (only once, on main)
DAY_COUNT_FILE="${REPOPATH}/DAY_COUNT"
CURRENT_DAY=0
if [[ -f "$DAY_COUNT_FILE" ]]; then
  CURRENT_DAY=$(cat "$DAY_COUNT_FILE" 2>/dev/null || echo "0")
  if ! [[ "$CURRENT_DAY" =~ ^[0-9]+$ ]]; then
    CURRENT_DAY=0
  fi
fi
NEXT_DAY=$((CURRENT_DAY + 1))
echo "$NEXT_DAY" > "$DAY_COUNT_FILE"
log "Day count updated: $CURRENT_DAY → $NEXT_DAY"

git add "$DAY_COUNT_FILE" 2>/dev/null || true
git diff --cached --quiet || git commit -m "chore: increment DAY_COUNT to $NEXT_DAY" 2>/dev/null || true

# Push everything to main
git push origin main 2>/dev/null || log "WARNING: failed to push to main"

# ── Increment DAY_COUNT ──
DAY_COUNT_FILE="${REPOPATH}/DAY_COUNT"
CURRENT_DAY=0
if [[ -f "$DAY_COUNT_FILE" ]]; then
  CURRENT_DAY=$(cat "$DAY_COUNT_FILE" 2>/dev/null || echo "0")
  if ! [[ "$CURRENT_DAY" =~ ^[0-9]+$ ]]; then
    CURRENT_DAY=0
  fi
fi
NEXT_DAY=$((CURRENT_DAY + 1))
echo "$NEXT_DAY" > "$DAY_COUNT_FILE"
log "Day count updated: $CURRENT_DAY → $NEXT_DAY"

git add "$DAY_COUNT_FILE" 2>/dev/null || true
git diff --cached --quiet || git commit -m "chore: increment DAY_COUNT to $NEXT_DAY" 2>/dev/null || true

# Push journal and DAY_COUNT to main
git push origin main 2>/dev/null || log "WARNING: failed to push journal/DAY_COUNT to main"

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

# ── Summary ──
log "=== evolution cycle finished ==="
log "Day: $DAY"
log "Branch: $BRANCH"
log "PR: #${PR_NUMBER:-none}"
log "Duration: ${SESSION_DURATION}s"
log "Pipeline: $([ "$PIPELINE_OK" == "true" ] && echo "OK" || echo "partial")"

# ── Cleanup SESSION_PLAN.md ──
if [[ -f "$PLAN_FILE" ]]; then
  log "Cleaning up SESSION_PLAN.md..."
  rm -f "$PLAN_FILE"
  git add "$PLAN_FILE" 2>/dev/null || true
  git diff --cached --quiet || git commit -m "chore: cleanup SESSION_PLAN.md after evolution" 2>/dev/null || true
  git push origin "$BRANCH" 2>/dev/null || true
fi

# ── Discord Success Notification ──
JOURNAL_ENTRY=$(grep -A3 "## Day ${DAY}" docs/JOURNAL.md 2>/dev/null | head -4 || echo "No journal entry")
SUCCESS_MSG="Evolution completed successfully!

**Changes:**
• PR: #${PR_NUMBER:-none}
• Duration: ${SESSION_DURATION}s
• Review retries: $REVIEW_RETRIES
• Day: $NEXT_DAY

**Journal:**
$(echo "$JOURNAL_ENTRY" | head -2 | tr '\n' ' ')"

send_discord_notification "success" "$SUCCESS_MSG"
