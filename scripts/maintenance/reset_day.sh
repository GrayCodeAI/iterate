#!/usr/bin/env bash
# scripts/maintenance/reset_day.sh — Reset (or set) the DAY_COUNT file.
#
# Usage:
#   ./scripts/maintenance/reset_day.sh        # reset to 0
#   ./scripts/maintenance/reset_day.sh 42     # set to 42

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-0}"

if ! [[ "$TARGET" =~ ^[0-9]+$ ]]; then
    echo "Error: day count must be a non-negative integer, got: $TARGET" >&2
    exit 1
fi

echo "$TARGET" > "$REPO/DAY_COUNT"
echo "DAY_COUNT set to $TARGET"
