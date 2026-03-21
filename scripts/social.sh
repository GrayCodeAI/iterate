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
./iterate --social --gh-owner GrayCodeAI --gh-repo iterate \
  2>/dev/null || log "Social session completed with status $?"

# Update social learnings
log "Updating social learnings..."
if grep -q "^/learn" "${REPOPATH}/.iterate/DISCUSSIONS_TODAY.md" 2>/dev/null; then
  python3 scripts/update_social_learnings.py || true
fi

# Also update the top-level SOCIAL_LEARNINGS.md if new learnings exist
if [[ -f "${REPOPATH}/memory/active_social_learnings.md" ]]; then
  # Append recent insights to SOCIAL_LEARNINGS.md (keep under 200 lines)
  if ! grep -q "No social interactions yet" "${REPOPATH}/memory/active_social_learnings.md" 2>/dev/null; then
    log "Updating SOCIAL_LEARNINGS.md..."
    # Extract new insights and append to Recent Interactions section
    python3 -c "
import re
with open('SOCIAL_LEARNINGS.md', 'r') as f:
    content = f.read()
with open('memory/active_social_learnings.md', 'r') as f:
    active = f.read()
# Find recent insights (skip header and timestamp)
insights = []
for line in active.split('\n'):
    if line.startswith('- '):
        insights.append(line)
if insights:
    # Find and update Recent Interactions section
    match = re.search(r'## Recent Interactions\n', content)
    if match:
        # Keep header and guidelines, replace interactions
        header = content[:match.end()]
        new_content = header + '\n'.join(insights[-10:]) + '\n'
        with open('SOCIAL_LEARNINGS.md', 'w') as f:
            f.write(new_content)
        print(f'Added {len(insights[-10:])} insights to SOCIAL_LEARNINGS.md')
" 2>/dev/null || true
  fi
fi

log "=== iterate social session completed ==="
