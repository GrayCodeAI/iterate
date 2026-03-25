#!/usr/bin/env python3
"""Track test coverage over time.

Runs `go test -cover` and appends the result to memory/coverage_history.jsonl.
"""

import json
import subprocess
import sys
from datetime import datetime, timezone


def get_coverage(repo_path="."):
    """Run go test with coverage and return the percentage."""
    try:
        result = subprocess.run(
            ["go", "test", "-coverprofile=/tmp/iterate-cover.out", "./..."],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=120,
        )

        # Parse coverage from output
        total_statements = 0
        covered_statements = 0

        for line in result.stdout.split("\n"):
            if "coverage:" in line:
                # Format: "ok  	package	0.123s	coverage: 85.0% of statements"
                parts = line.split("coverage:")
                if len(parts) > 1:
                    pct_str = parts[1].strip().split("%")[0]
                    try:
                        pct = float(pct_str)
                        # This is per-package, we'll use the summary
                    except ValueError:
                        pass

        # Get total coverage from the profile
        cover_result = subprocess.run(
            ["go", "tool", "cover", "-func=/tmp/iterate-cover.out"],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=30,
        )

        if cover_result.returncode == 0:
            for line in cover_result.stdout.split("\n"):
                if "total:" in line:
                    # Format: "total:	(statements)	85.0%"
                    parts = line.split("\t")
                    for part in parts:
                        part = part.strip()
                        if part.endswith("%"):
                            try:
                                return float(part.rstrip("%"))
                            except ValueError:
                                pass

        return 0.0
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 0.0


def count_tests(repo_path="."):
    """Count total test functions."""
    try:
        result = subprocess.run(
            ["go", "test", "-list", ".*", "./..."],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=60,
        )
        count = 0
        for line in result.stdout.split("\n"):
            if line.startswith("Test"):
                count += 1
        return count
    except Exception:
        return 0


def main():
    repo_path = sys.argv[1] if len(sys.argv) > 1 else "."

    coverage = get_coverage(repo_path)
    test_count = count_tests(repo_path)

    entry = {
        "date": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "coverage_pct": round(coverage, 1),
        "test_count": test_count,
    }

    print(json.dumps(entry))

    # Append to history
    history_file = f"{repo_path}/memory/coverage_history.jsonl"
    with open(history_file, "a") as f:
        f.write(json.dumps(entry) + "\n")


if __name__ == "__main__":
    main()
