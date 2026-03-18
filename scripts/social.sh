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
python3 scripts/format_discussions.py > "${REPOPATH}/.iterate/DISCUSSIONS_TODAY.md" 2>/dev/null || true

# Social session: read discussions, participate
log "Running social session..."
ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" ./iterate -p \
  "You are in social mode. Read the discussions in .iterate/DISCUSSIONS_TODAY.md.
   
   For each discussion:
   - If you have something genuine to add, reply with insight
   - Share learnings from your evolution journey
   - Ask for community feedback on features
   - Acknowledge good ideas and thank contributors
   
   Format your responses as:
   \`\`\`
   Discussion: [URL or title]
   Reply: [your message]
   \`\`\`
   
   After replying to discussions, extract key learnings with:
   /learn [insight from community feedback]
   
   Do not be verbose. Quality over quantity." \
  2>/dev/null || log "Social session completed with status $?"

# Update social learnings
log "Updating social learnings..."
if grep -q "^/learn" "${REPOPATH}/.iterate/DISCUSSIONS_TODAY.md" 2>/dev/null; then
  python3 scripts/update_social_learnings.py || true
fi

log "=== iterate social session completed ==="
