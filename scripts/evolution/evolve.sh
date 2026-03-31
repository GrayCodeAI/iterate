#!/bin/bash
# iterate evolution pipeline: plan → implement → pr → review → merge → communicate
# Autonomous evolution cycle — 6-phase self-evolving pipeline.
# Runs every 12h via GitHub Actions.

# Don't abort on errors — we handle failures per-phase
set +e

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
  payload=$(jq -n \
    --arg title "$title" \
    --arg desc "$message" \
    --arg status "$status" \
    --arg pr "$pr_url" \
    --argjson color "$color" \
    --arg ts "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    '{embeds:[{title:$title,description:$desc,color:$color,timestamp:$ts,footer:{text:"iterate-evolve[bot]"}}]}' 2>/dev/null)

  if [[ -n "$payload" ]]; then
    curl -sf -H "Content-Type: application/json" -d "$payload" "$DISCORD_WEBHOOK_URL" >/dev/null 2>&1 || true
  fi
}

# ── Concurrent run lock ──
acquire_lock() {
  if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$OLD_PID" ]] && kill -0 "$OLD_PID" 2>/dev/null; then
      LOCK_AGE=$(( $(date +%s) - $(stat -f %m "$PID_FILE" 2>/dev/null || stat -c %Y "$PID_FILE" 2>/dev/null || echo 0) ))
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

# ── Ensure directories exist ──
mkdir -p "${REPOPATH}/.iterate" || { echo "ERROR: failed to create .iterate dir"; exit 1; }
mkdir -p "${REPOPATH}/memory" || { echo "ERROR: failed to create memory dir"; exit 1; }
mkdir -p "${REPOPATH}/docs" || { echo "ERROR: failed to create docs dir"; exit 1; }

# Initialize log
log "=== iterate evolution cycle started ==="

acquire_lock

# ── API Key Setup ──
PROVIDER="${ITERATE_PROVIDER:-openrouter}"

# Wire API key for iteragent's custom provider path (uses ITERATE_API_KEY)
if [[ -n "${OPENROUTER_API_KEY:-}" ]] && [[ -z "${ITERATE_API_KEY:-}" ]]; then
  export ITERATE_API_KEY="$OPENROUTER_API_KEY"
fi

# Skip health checks to conserve API calls on rate-limited free tiers
export ITERATE_SKIP_HEALTH_CHECK=1

# Skip health checks to conserve API calls on rate-limited free tiers
export ITERATE_SKIP_HEALTH_CHECK=1

# Wire API key for iteragent's custom provider path (uses ITERATE_API_KEY)
if [[ -n "${OPENROUTER_API_KEY:-}" ]] && [[ -z "${ITERATE_API_KEY:-}" ]]; then
  export ITERATE_API_KEY="$OPENROUTER_API_KEY"
fi

# Wire API key for iteragent's custom provider path (uses ITERATE_API_KEY)
if [[ -n "${OPENROUTER_API_KEY:-}" ]] && [[ -z "${ITERATE_API_KEY:-}" ]]; then
  export ITERATE_API_KEY="$OPENROUTER_API_KEY"
fi

# Check for rate limit in last command and rotate if needed
check_rate_limit() {
  if echo "$1" | grep -qi "rate.*limit\|quota.*exceeded\|429"; then
    log "Rate limit detected"
    return 0
  fi
  return 1
}

# ── Pre-flight checks ──
preflight_checks() {
  local ok=true

  # Check Go toolchain
  if ! command -v go &>/dev/null; then
    log "ERROR: go not found in PATH"
    ok=false
  else
    log "Go version: $(go version 2>/dev/null | head -1)"
  fi

  # Check git
  if ! command -v git &>/dev/null; then
    log "ERROR: git not found in PATH"
    ok=false
  fi

  # Check gh CLI (needed for PR operations)
  if ! command -v gh &>/dev/null; then
    log "WARNING: gh CLI not found — PR operations will fail"
  else
    if gh auth status &>/dev/null 2>&1; then
      log "GitHub authenticated: $(gh auth status 2>&1 | head -1)"
    else
      log "WARNING: gh CLI present but not authenticated"
    fi
  fi

  # Check python3 (needed for stats/coverage scripts)
  if ! command -v python3 &>/dev/null; then
    log "WARNING: python3 not found — coverage/stats will be skipped"
  fi

  # Check jq (needed for Discord notifications)
  if ! command -v jq &>/dev/null; then
    log "WARNING: jq not found — Discord notifications disabled"
  fi

  # Verify repo is clean enough to work
  if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    log "ERROR: not in a git repository"
    ok=false
  fi

  if [[ "$ok" == "false" ]]; then
    log "Pre-flight checks failed — aborting"
    return 1
  fi
  return 0
}

# ── Guards ──
if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  log "ERROR: OPENROUTER_API_KEY not set"
  exit 1
fi

log "Provider: ${PROVIDER:-openrouter}, Model: ${ITERATE_MODEL:-default}"

# ── Request Rate Limiter ──
# OpenRouter free tier: 20 requests/minute. We cap at 15 to leave headroom.
# Each phase makes ~2 API calls (main + internal retries).
# We spread phases across 30s intervals to stay under limit.
REQUEST_COUNT=0
REQUEST_WINDOW_START=$(date +%s)
MAX_REQUESTS_PER_WINDOW=15
WINDOW_DURATION=60
ESTIMATED_CALLS_PER_PHASE=2

check_rate_limit() {
  local now=$(date +%s)
  local elapsed=$(( now - REQUEST_WINDOW_START ))

  # Reset window if minute has passed
  if [[ $elapsed -ge $WINDOW_DURATION ]]; then
    REQUEST_COUNT=0
    REQUEST_WINDOW_START=$now
    return 0
  fi

  # Approaching limit — wait for window to reset
  if [[ $REQUEST_COUNT -ge $MAX_REQUESTS_PER_WINDOW ]]; then
    local wait=$(( WINDOW_DURATION - elapsed ))
    if [[ $wait -gt 0 ]]; then
      log "Rate limit approaching ($REQUEST_COUNT/$MAX_REQUESTS_PER_WINDOW) — waiting ${wait}s for window reset..."
      sleep "$wait"
      REQUEST_COUNT=0
      REQUEST_WINDOW_START=$(date +%s)
    fi
  fi
}

# sleep_between_phases ensures we don't exceed 15 API calls per minute
# by spacing out phase invocations based on estimated call count.
sleep_between_phases() {
  local now=$(date +%s)
  local elapsed=$(( now - REQUEST_WINDOW_START ))
  local projected=$(( REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE ))

  if [[ $projected -gt $MAX_REQUESTS_PER_WINDOW ]]; then
    local wait=$(( WINDOW_DURATION - elapsed ))
    if [[ $wait -gt 0 ]]; then
      log "Spreading requests: waiting ${wait}s to stay under ${MAX_REQUESTS_PER_WINDOW} req/min..."
      sleep "$wait"
      REQUEST_COUNT=0
      REQUEST_WINDOW_START=$(date +%s)
    fi
  fi
}

run_with_rotation() {
  local phase="$1"
  local max_retries=3
  local attempt=1

  local model_arg=""
  if [[ -n "$ITERATE_MODEL" ]]; then
    model_arg="--model $ITERATE_MODEL"
  fi

  local provider_arg=""
  if [[ -n "$ITERATE_PROVIDER" ]]; then
    provider_arg="--provider $ITERATE_PROVIDER"
  fi

  while [[ $attempt -le $max_retries ]]; do
    # Check rate limit before each attempt
    check_rate_limit

    log "Running phase $phase (attempt $attempt/$max_retries)..."
    log "Using model: ${ITERATE_MODEL:-default}, provider: ${ITERATE_PROVIDER:-openrouter}"

    local phase_output
    phase_output=$(./iterate --phase "$phase" --gh-owner GrayCodeAI --gh-repo iterate $model_arg $provider_arg 2>&1)
    local phase_exit=$?

    # Count this as a request (each phase call makes multiple internal API calls)
    REQUEST_COUNT=$((REQUEST_COUNT + 1))

    # Always log phase output for debugging
    if [[ -n "$phase_output" ]]; then
      echo "$phase_output" >> "$LOG_FILE"
    fi

    if [[ $phase_exit -eq 0 ]]; then
      log "Phase $phase completed successfully"
      return 0
    fi

    log "Phase $phase failed (attempt $attempt/$max_retries)"

    # Check if it's a rate limit error — wait 60s to clear per-minute window
    if echo "$phase_output" | grep -qi "rate.*limit\|quota.*exceeded\|429"; then
      if [[ $attempt -lt $max_retries ]]; then
        log "Rate limited — waiting 60s before retry..."
        sleep 60
        REQUEST_COUNT=0
        REQUEST_WINDOW_START=$(date +%s)
      fi
      ((attempt++))
      continue
    fi

    # Non-rate-limit failure — still retry
    if [[ $attempt -lt $max_retries ]]; then
      log "Retrying phase $phase in 10s..."
      sleep 10
    fi
    ((attempt++))
  done

  log "Phase $phase failed after $max_retries attempts"
  return 1
}

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

# ── Pre-flight checks ──
if ! preflight_checks; then
  send_discord_notification "failure" "Pre-flight checks failed"
  exit 1
fi

# ── Send Started Notification ──
send_discord_notification "started" "Evolution Day $DAY starting"

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
  if ! run_with_rotation "plan"; then
    log "WARNING: plan phase exited with error — checking for fallback"
  fi
  REQUEST_COUNT=$((REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE))

  # Fallback plan if agent didn't create one
  if [[ ! -f "$PLAN_FILE" ]]; then
    log "Agent did not create SESSION_PLAN.md — writing fallback"
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

  # ── Phase 2: Implementation ──
  sleep_between_phases
  log "Phase 2: Implementation..."
  if ! run_with_rotation "implement"; then
    log "WARNING: implement phase exited with error — continuing"
  fi
  REQUEST_COUNT=$((REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE))
  REQUEST_COUNT=$((REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE))

  # Fallback plan if agent didn't create one
  if [[ ! -f "$PLAN_FILE" ]]; then
    log "Agent did not create SESSION_PLAN.md — writing fallback"
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

  # ── Phase 2: Implementation ──
  sleep_between_phases
  log "Phase 2: Implementation..."
  if ! run_with_rotation "implement"; then
    log "WARNING: implement phase exited with error — continuing"
  fi
  REQUEST_COUNT=$((REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE))

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
  sleep_between_phases
  log "Phase 3: Pull Request..."
  if ! run_with_rotation "pr"; then
    log "WARNING: PR phase exited with error — continuing"
  fi
  REQUEST_COUNT=$((REQUEST_COUNT + ESTIMATED_CALLS_PER_PHASE))
fi

BRANCH="evolution/day-${DAY}"
PR_NUMBER=$(gh pr list --repo "$GITHUB_REPO" --head "$BRANCH" --json number --jq '.[0].number' 2>/dev/null || echo "")

PIPELINE_OK=true

# ── Phase 4: Review with Auto-Retry ──
sleep_between_phases
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
sleep_between_phases
log "Phase 5: Merge..."
if ! run_with_rotation "merge"; then
  log "WARNING: merge phase failed — PR may still be open"
  PIPELINE_OK=false
fi

# ── Phase 6: Communication (always attempt, even if earlier phases failed) ──
sleep_between_phases
log "Phase 6: Communication..."
if ! run_with_rotation "communicate"; then
  log "WARNING: communicate phase exited with error"
fi

# ── Phase 6: Communication (always attempt, even if earlier phases failed) ──
sleep_between_phases
log "Phase 6: Communication..."
if ! run_with_rotation "communicate"; then
  log "WARNING: communicate phase exited with error"
fi

# ── Verify journal was written ──
if grep -q "## Day ${DAY}" "${REPOPATH}/docs/JOURNAL.md" 2>/dev/null; then
  log "Journal entry written for Day $DAY"
else
  log "WARNING: No journal entry found for Day $DAY — writing fallback"
  SESSION_TIME_NOW=$(date -u +'%H:%M')
  cat >> "${REPOPATH}/docs/JOURNAL.md" <<JEOF

## Day ${DAY} — ${SESSION_TIME_NOW} — Evolution session completed

Evolution session completed. Pipeline status: $([ "$PIPELINE_OK" == "true" ] && echo "success" || echo "partial")
JEOF
  git add docs/JOURNAL.md 2>/dev/null || true
  git diff --cached --quiet || git commit -m "journal: Day $DAY fallback entry" 2>/dev/null || true
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
log "=== evolution cycle finished ==="
log "Day: $DAY"
log "Branch: $BRANCH"
log "PR: #${PR_NUMBER:-none}"
log "Duration: ${SESSION_DURATION}s"
log "Pipeline: $([ "$PIPELINE_OK" == "true" ] && echo "OK" || echo "PARTIAL")"

# ── Cleanup SESSION_PLAN.md ──
if [[ -f "$PLAN_FILE" ]]; then
  log "Cleaning up SESSION_PLAN.md..."
  rm -f "$PLAN_FILE"
fi

# ── Discord Notification ──
JOURNAL_ENTRY=$(grep -A3 "^## Day ${DAY}" docs/JOURNAL.md 2>/dev/null | head -4 || echo "No journal entry")
if [[ "$PIPELINE_OK" == "true" ]]; then
  SUCCESS_MSG="Evolution completed successfully!

**Changes:**
• PR: #${PR_NUMBER:-none}
• Duration: ${SESSION_DURATION}s
• Review retries: $REVIEW_RETRIES

**Journal:**
$(echo "$JOURNAL_ENTRY" | head -2 | tr '\n' ' ')"
  send_discord_notification "success" "$SUCCESS_MSG"
else
  FAILURE_MSG="Evolution completed with issues.

**Changes:**
• PR: #${PR_NUMBER:-none}
• Duration: ${SESSION_DURATION}s
• Review retries: $REVIEW_RETRIES
• Pipeline: PARTIAL (some phases failed)

**Journal:**
$(echo "$JOURNAL_ENTRY" | head -2 | tr '\n' ' ')"
  send_discord_notification "failure" "$FAILURE_MSG"
fi
