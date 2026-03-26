#!/usr/bin/env python3
"""Synthesize learnings and failures from JSONL archive with time-weighted compression."""

import json
import os
from datetime import datetime, timezone

REPO_PATH = "."
LEARNINGS_FILE = f"{REPO_PATH}/memory/learnings.jsonl"
FAILURES_FILE = f"{REPO_PATH}/memory/failures.jsonl"
ACTIVE_FILE = f"{REPO_PATH}/memory/ACTIVE_LEARNINGS.md"


def now_utc():
    return datetime.now(timezone.utc)


def weight_by_age(days_old):
    """Time-weighted compression factor (recent=100%, old=summarized)."""
    if days_old <= 1:
        return 1.0
    elif days_old <= 7:
        return 0.7
    elif days_old <= 30:
        return 0.3
    else:
        return 0.1


def parse_ts(ts_str):
    """Parse an ISO-8601 timestamp string, handling Z suffix and missing tz."""
    if not ts_str:
        return None
    ts_str = ts_str.replace("Z", "+00:00")
    try:
        dt = datetime.fromisoformat(ts_str)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    except ValueError:
        return None


def load_jsonl(path):
    """Load all entries from a JSONL file, silently skipping corrupt lines."""
    entries = []
    if not os.path.exists(path):
        return entries
    try:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if line:
                    try:
                        entries.append(json.loads(line))
                    except json.JSONDecodeError:
                        pass
    except OSError as e:
        print(f"Warning: could not read {path}: {e}")
    return entries


def synthesize_learnings(learnings, failures):
    """Synthesize learnings and failures into ACTIVE_LEARNINGS.md content."""
    now = now_utc()

    # Bucket learnings by age.
    recent, medium, old = [], [], []
    for entry in learnings:
        dt = parse_ts(entry.get("ts", ""))
        days_old = (now - dt).days if dt else 999
        weight = weight_by_age(days_old)
        if days_old <= 1:
            recent.append((entry, weight))
        elif days_old <= 30:
            medium.append((entry, weight))
        else:
            old.append((entry, weight))

    # Recent failures (last 14 days), keep most recent 10.
    recent_failures = []
    for entry in failures:
        dt = parse_ts(entry.get("ts", ""))
        days_old = (now - dt).days if dt else 999
        if days_old <= 14:
            recent_failures.append(entry)
    recent_failures = recent_failures[-10:]

    if not learnings and not failures:
        return "## Active Learnings\n\nNo learnings yet.\n"

    out = ["## Active Learnings\n\n"]
    out.append(f"*Last synthesized: {now.strftime('%Y-%m-%dT%H:%M:%SZ')}*\n\n")

    if recent:
        out.append("### Recent (Full Detail)\n\n")
        for entry, _ in recent[:5]:
            title = entry.get("title", "Untitled")
            detail = entry.get("context") or entry.get("takeaway") or ""
            out.append(f"- **{title}**: {detail[:200]}\n")
        out.append("\n")

    if medium:
        out.append("### Active Lessons (Condensed)\n\n")
        for entry, weight in medium[:10]:
            title = entry.get("title", "Lesson")
            content = entry.get("context") or entry.get("takeaway") or ""
            if weight < 1.0 and len(content) > 100:
                content = content[:100] + "..."
            out.append(f"- {title}: {content}\n")
        out.append("\n")

    if old:
        out.append("### Archived Insights\n\n")
        themes = {}
        for entry, _ in old:
            theme = entry.get("source", "General")
            themes.setdefault(theme, []).append(entry.get("title", "Lesson"))
        for theme, titles in themes.items():
            out.append(f"- **{theme}**: {', '.join(titles[:3])}\n")
        out.append("\n")

    if recent_failures:
        out.append("### Recent Failures (avoid repeating)\n\n")
        for entry in recent_failures:
            day = entry.get("day", "?")
            task = entry.get("task", "?")
            reason = entry.get("reason", "")
            line = f"- Day {day} — {task}"
            if reason:
                line += f": {reason[:120]}"
            out.append(line + "\n")
        out.append("\n")

    return "".join(out)


def main():
    learnings = load_jsonl(LEARNINGS_FILE)
    failures = load_jsonl(FAILURES_FILE)
    synthesis = synthesize_learnings(learnings, failures)

    os.makedirs(os.path.dirname(os.path.abspath(ACTIVE_FILE)), exist_ok=True)
    with open(ACTIVE_FILE, "w") as f:
        f.write(synthesis)

    print(f"Synthesized {len(learnings)} learnings + {len(failures)} failures → {ACTIVE_FILE}")


if __name__ == "__main__":
    main()
