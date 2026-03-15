#!/usr/bin/env python3
"""Format GitHub issues for the agent prompt."""

import json
import sys


def format_issues(issues_json):
    """Format issues for the agent prompt."""
    try:
        issues = json.load(issues_json)
    except json.JSONDecodeError:
        print("No issues found.")
        return

    if not issues:
        print("No issues found.")
        return

    for issue in issues:
        num = issue.get("number", 0)
        title = issue.get("title", "No title")
        body = issue.get("body", "")[:500] if issue.get("body") else ""
        labels = [l["name"] for l in issue.get("labels", [])]

        # Count reactions
        reactions = issue.get("reactionGroups", [])
        upvotes = 0
        for r in reactions:
            if r.get("content") == "+1":
                upvotes = r.get("totalCount", 0)

        print(f"### Issue #{num}")
        print(f"**Title:** {title}")
        if labels:
            print(f"**Labels:** {', '.join(labels)}")
        if upvotes:
            print(f"**Upvotes:** {upvotes}")
        if body:
            print(f"\n{body}")
        print()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: format_issues.py <issues.json>")
        sys.exit(1)

    with open(sys.argv[1], "r") as f:
        format_issues(f)
