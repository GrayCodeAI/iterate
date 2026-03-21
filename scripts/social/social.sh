#!/bin/bash
set -e

# iterate social session: read discussions, reply, learn
# Runs every 4 hours, offset from evolution loop

REPOPATH="."
LOG_FILE="${REPOPATH}/.iterate/social.log"

mkdir -p "${REPOPATH}/.iterate"

log() {
  echo "[$(date -u +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "=== iterate social session started ==="

# Build the binary
go build -o ./iterate ./cmd/iterate

# Fetch GitHub discussions
log "Fetching GitHub discussions..."
python3 scripts/build/format_discussions.py > "${REPOPATH}/.iterate/DISCUSSIONS_TODAY.md" 2>/dev/null || true

# Social session: read discussions, participate, extract learnings
log "Running social session..."
./iterate --social --gh-owner GrayCodeAI --gh-repo iterate \
  2>/dev/null || log "Social session completed with status $?"

# Synthesize social learnings into active context
log "Synthesizing social learnings..."
python3 scripts/social/update_social_learnings.py || true

log "=== iterate social session completed ==="
