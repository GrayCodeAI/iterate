#!/usr/bin/env python3
"""Fetch and format GitHub issues for iterate evolution context."""

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


def fetch_issues():
    """Fetch open issues from GitHub."""
    cmd = "gh issue list --json number,title,body,labels --limit 20"
    data = run_gh_command(cmd)

    if not data:
        return []

    # Parse and filter by labels
    issues = []
    for item in data:
        labels = [l.get("name", "") for l in item.get("labels", [])]

        # Prioritize agent-input and agent-help-wanted
        priority = 0
        if "agent-input" in labels:
            priority = 10
        elif "agent-help-wanted" in labels:
            priority = 5
        elif "agent-self" in labels:
            priority = 1

        if priority > 0:
            issues.append(
                {
                    "number": item["number"],
                    "title": item["title"],
                    "body": item["body"] or "",
                    "priority": priority,
                    "labels": labels,
                }
            )

    # Sort by priority (descending) then by number (descending)
    issues.sort(key=lambda x: (-x["priority"], -x["number"]))

    return issues[:10]  # Top 10 issues


def main():
    """Generate ISSUES_TODAY.md from GitHub issues."""
    issues = fetch_issues()

    if not issues:
        print(
            "# Issues Today\n\nNo open issues labeled with agent-input or agent-help-wanted.",
            file=sys.stdout,
        )
        return

    output = ["# Issues Today\n"]

    for issue in issues:
        output.append(f"## Issue #{issue['number']}: {issue['title']}\n")
        output.append(f"Labels: {', '.join(issue['labels'])}\n\n")
        output.append(f"{issue['body']}\n\n")
        output.append("---\n\n")

    print("".join(output), file=sys.stdout)


if __name__ == "__main__":
    main()
