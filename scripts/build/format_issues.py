#!/usr/bin/env python3
"""Fetch and format GitHub issues for iterate evolution context.

Issues are scored by net votes (thumbs up - thumbs down).
Higher score = higher priority. Negative scores are deprioritized.
"""

import subprocess
import json
import sys
from datetime import datetime


def run_gh_command(cmd):
    """Run a gh CLI command and return JSON output."""
    try:
        result = subprocess.run(
            cmd, shell=True, capture_output=True, text=True, timeout=30
        )
        if result.returncode == 0 and result.stdout:
            return json.loads(result.stdout)
        return None
    except Exception as e:
        print(f"Error running command: {e}", file=sys.stderr)
        return None


def fetch_issue_reactions(issue_number):
    """Fetch reaction counts for an issue."""
    cmd = f"gh api repos/:owner/:repo/issues/{issue_number}/reactions"
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
        if result.returncode == 0 and result.stdout:
            reactions = json.loads(result.stdout)
            # Count reactions by type
            counts = {}
            for r in reactions:
                content = r.get("content", "")
                counts[content] = counts.get(content, 0) + 1
            # Net score: +1 reactions minus -1 reactions
            plus_one = counts.get("+1", 0)
            minus_one = counts.get("-1", 0)
            return plus_one - minus_one
        return 0
    except Exception:
        return 0


def fetch_issues():
    """Fetch open issues from GitHub."""
    cmd = "gh issue list --json number,title,body,labels --limit 30"
    data = run_gh_command(cmd)

    if not data:
        return []

    # Parse and filter by labels
    issues = []
    for item in data:
        labels = [l.get("name", "") for l in item.get("labels", [])]

        # Only process issues with relevant labels
        if not any(l in labels for l in ["agent-input", "agent-help-wanted", "agent-self"]):
            continue

        # Calculate priority based on labels
        label_priority = 0
        if "agent-input" in labels:
            label_priority = 10
        elif "agent-help-wanted" in labels:
            label_priority = 5
        elif "agent-self" in labels:
            label_priority = 1

        # Fetch reaction-based score
        vote_score = fetch_issue_reactions(item["number"])

        issues.append(
            {
                "number": item["number"],
                "title": item["title"],
                "body": item["body"] or "",
                "priority": label_priority,
                "vote_score": vote_score,
                "labels": labels,
            }
        )

    # Sort by: vote score (desc), then label priority (desc), then issue number (desc)
    issues.sort(key=lambda x: (-x["vote_score"], -x["priority"], -x["number"]))

    return issues[:10]  # Top 10 issues


def main():
    """Generate ISSUES_TODAY.md from GitHub issues."""
    issues = fetch_issues()

    if not issues:
        print(
            "# Issues Today\n\nNo open issues labeled with agent-input, agent-help-wanted, or agent-self.",
            file=sys.stdout,
        )
        return

    output = ["# Issues Today\n\n"]
    output.append("Issues sorted by community votes (👍 - 👎). Higher score = higher priority.\n\n")

    for issue in issues:
        vote_str = f"[👍{issue['vote_score']:+d}] " if issue['vote_score'] != 0 else ""
        output.append(f"## Issue #{issue['number']}: {issue['title']}\n")
        output.append(f"**{vote_str}Labels:** {', '.join(issue['labels'])}\n\n")
        
        # Truncate body if too long
        body = issue['body']
        if len(body) > 500:
            body = body[:500] + "\n...[truncated]"
        output.append(f"{body}\n\n")
        output.append("---\n\n")

    print("".join(output), file=sys.stdout)


if __name__ == "__main__":
    main()
