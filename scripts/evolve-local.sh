#!/usr/bin/env bash
# scripts/evolve-local.sh — Run a full evolution cycle locally.
#
# Usage:
#   OPENCODE_API_KEY=sk-... ./scripts/evolve-local.sh
#   GEMINI_API_KEY=... ITERATE_PROVIDER=gemini ./scripts/evolve-local.sh
#   ./scripts/evolve-local.sh --phase plan   # run only the plan phase
#
# Runs the same 3-phase pipeline as the GitHub Actions evolve workflow,
# but locally. Useful for testing or manual evolution sessions.

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO"

PHASE="${1:-}"

# Build first.
echo "▶ Building..."
go build ./...
echo "✓ Build OK"

# Run tests.
echo "▶ Running tests..."
go test ./... && echo "✓ Tests OK"

# Fetch issues if gh is available and we know the repo.
ISSUES=""
if command -v gh &>/dev/null; then
    GH_REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo "")
    if [ -n "$GH_REPO" ]; then
        GH_OWNER="${GH_REPO%%/*}"
        GH_REPO_NAME="${GH_REPO##*/}"
        echo "▶ Fetching issues for $GH_REPO..."
        ISSUES_FLAGS="--gh-owner=$GH_OWNER --gh-repo=$GH_REPO_NAME"
    fi
fi

echo "▶ Starting evolution..."

if [ -n "$PHASE" ]; then
    go run ./cmd/iterate/... --phase "$PHASE" ${ISSUES_FLAGS:-}
else
    # Full 3-phase cycle.
    echo "  Phase A: Planning..."
    go run ./cmd/iterate/... --phase plan ${ISSUES_FLAGS:-}

    echo "  Phase B: Implementation..."
    go run ./cmd/iterate/... --phase implement ${ISSUES_FLAGS:-}

    echo "  Phase C: Communication..."
    go run ./cmd/iterate/... --phase communicate ${ISSUES_FLAGS:-}
fi

echo "✓ Evolution cycle complete"
