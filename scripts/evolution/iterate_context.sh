#!/bin/bash
# scripts/evolution/iterate_context.sh — Build iterate's identity context for prompts.
# Source this file, then use $ITERATE_CONTEXT in any prompt.
#
# Usage:
#   ITERATE_REPO="/path/to/iterate" source scripts/evolution/iterate_context.sh
#   cat > prompt.txt <<EOF
#   $ITERATE_CONTEXT
#   ... your task-specific instructions ...
#   EOF
#
# Reads: IDENTITY.md, PERSONALITY.md, memory/ACTIVE_LEARNINGS.md, memory/ACTIVE_SOCIAL_LEARNINGS.md
# These are iterate's stable identity files — who it is, how it speaks,
# what it's learned about itself, and what it's learned from humans.

_ITERATE_REPO="${ITERATE_REPO:-.}"

_IDENTITY=""
if [ -f "$_ITERATE_REPO/IDENTITY.md" ]; then
    _IDENTITY=$(cat "$_ITERATE_REPO/IDENTITY.md") || {
        echo "WARNING: Failed to read IDENTITY.md" >&2
        _IDENTITY=""
    }
else
    echo "WARNING: IDENTITY.md not found at $_ITERATE_REPO/IDENTITY.md" >&2
fi

_PERSONALITY=""
if [ -f "$_ITERATE_REPO/PERSONALITY.md" ]; then
    _PERSONALITY=$(cat "$_ITERATE_REPO/PERSONALITY.md") || {
        echo "WARNING: Failed to read PERSONALITY.md" >&2
        _PERSONALITY=""
    }
else
    echo "WARNING: PERSONALITY.md not found at $_ITERATE_REPO/PERSONALITY.md" >&2
fi

# Active learnings — no warning if missing
_LEARNINGS=""
if [ -f "$_ITERATE_REPO/memory/ACTIVE_LEARNINGS.md" ]; then
    _LEARNINGS=$(cat "$_ITERATE_REPO/memory/ACTIVE_LEARNINGS.md") || _LEARNINGS=""
fi

# Active social learnings — no warning if missing
_SOCIAL_LEARNINGS=""
if [ -f "$_ITERATE_REPO/memory/ACTIVE_SOCIAL_LEARNINGS.md" ]; then
    _SOCIAL_LEARNINGS=$(cat "$_ITERATE_REPO/memory/ACTIVE_SOCIAL_LEARNINGS.md") || _SOCIAL_LEARNINGS=""
fi

ITERATE_CONTEXT="=== WHO YOU ARE ===

${_IDENTITY:-Read IDENTITY.md for your rules and constitution.}

=== YOUR VOICE ===

${_PERSONALITY:-Read PERSONALITY.md for your voice and values.}

=== SELF-WISDOM ===

${_LEARNINGS:-No learnings yet.}

=== SOCIAL WISDOM ===

${_SOCIAL_LEARNINGS:-No social learnings yet.}"
