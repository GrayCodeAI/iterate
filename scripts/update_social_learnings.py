#!/usr/bin/env python3
"""Update social learnings from discussion interactions."""

import json
import os
from datetime import datetime, timezone

REPO_PATH = '.'
SOCIAL_FILE = f'{REPO_PATH}/memory/social_learnings.jsonl'
ACTIVE_FILE = f'{REPO_PATH}/memory/ACTIVE_SOCIAL_LEARNINGS.md'

def add_social_learning(insight, source="discussion"):
    """Append a social learning to the archive."""
    os.makedirs(os.path.dirname(SOCIAL_FILE), exist_ok=True)
    
    learning = {
        "type": "social",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "source": source,
        "insight": insight
    }
    
    with open(SOCIAL_FILE, 'a') as f:
        f.write(json.dumps(learning) + '\n')

def load_social_learnings():
    """Load all social learnings from JSONL."""
    if not os.path.exists(SOCIAL_FILE):
        return []
    
    learnings = []
    with open(SOCIAL_FILE, 'r') as f:
        for line in f:
            if line.strip():
                try:
                    learnings.append(json.loads(line))
                except json.JSONDecodeError:
                    # Skip corrupt lines
                    pass
    
    return learnings

def synthesize_social(learnings):
    """Synthesize social learnings into active context."""
    if not learnings:
        return "## Active Social Learnings\n\nNo social interactions yet.\n"
    
    output = ['## Active Social Learnings\n\n']
    output.append(f'*Last synthesized: {datetime.now(timezone.utc).isoformat()}*\n\n')
    
    # Group by source
    by_source = {}
    for learning in learnings:
        source = learning.get('source', 'unknown')
        if source not in by_source:
            by_source[source] = []
        by_source[source].append(learning)
    
    # Recent insights first
    for source in sorted(by_source.keys(), key=lambda x: -len(by_source[x])):
        insights = by_source[source]
        output.append(f"### From {source.title()}\n\n")
        for insight in insights[-5:]:  # Latest 5
            output.append(f"- {insight.get('insight', '')}\n")
        output.append('\n')
    
    return ''.join(output)

def main():
    """Update active social learnings."""
    learnings = load_social_learnings()
    synthesis = synthesize_social(learnings)
    
    os.makedirs(os.path.dirname(ACTIVE_FILE), exist_ok=True)
    
    with open(ACTIVE_FILE, 'w') as f:
        f.write(synthesis)
    
    print(f"Synthesized {len(learnings)} social learnings")

if __name__ == '__main__':
    main()
