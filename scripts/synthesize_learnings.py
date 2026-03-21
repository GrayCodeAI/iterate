#!/usr/bin/env python3
"""Synthesize learnings from JSONL archive with time-weighted compression."""

import json
import os
from datetime import datetime, timedelta

REPO_PATH = '.'
LEARNINGS_FILE = f'{REPO_PATH}/memory/learnings.jsonl'
ACTIVE_FILE = f'{REPO_PATH}/memory/ACTIVE_LEARNINGS.md'

def weight_by_age(days_old):
    """Time-weighted compression factor (recent=100%, old=summarized)."""
    if days_old <= 1:
        return 1.0  # Full detail
    elif days_old <= 7:
        return 0.7  # 70% of detail
    elif days_old <= 30:
        return 0.3  # 30% of detail
    else:
        return 0.1  # Minimal detail

def load_learnings():
    """Load all learnings from JSONL."""
    if not os.path.exists(LEARNINGS_FILE):
        return []
    
    learnings = []
    try:
        with open(LEARNINGS_FILE, 'r') as f:
            for line in f:
                if line.strip():
                    learnings.append(json.loads(line))
    except Exception as e:
        print(f"Error loading learnings: {e}")
    
    return learnings

def synthesize_learnings(learnings):
    """Synthesize learnings with time-weighted compression."""
    if not learnings:
        return "## Active Learnings\n\nNo learnings yet. Start with `/learn` or `/memo`.\n"
    
    now = datetime.fromisoformat(datetime.utcnow().isoformat())
    
    # Group by recency
    recent = []
    medium = []
    old = []
    
    for learning in learnings:
        try:
            created = datetime.fromisoformat(learning.get('timestamp', ''))
            days_old = (now - created).days
            
            weight = weight_by_age(days_old)
            
            if days_old <= 1:
                recent.append((learning, weight))
            elif days_old <= 30:
                medium.append((learning, weight))
            else:
                old.append((learning, weight))
        except:
            recent.append((learning, 1.0))
    
    output = ['## Active Learnings\n']
    output.append(f'*Last synthesized: {datetime.utcnow().isoformat()}*\n\n')
    
    if recent:
        output.append('### Recent (Full Detail)\n\n')
        for learning, _ in recent[:5]:  # Top 5 recent
            output.append(f"- **{learning.get('title', 'Untitled')}**: {learning.get('content', '')}\n")
        output.append('\n')
    
    if medium:
        output.append('### Active Lessons (Condensed)\n\n')
        for learning, weight in medium[:10]:
            content = learning.get('content', '')
            if weight < 1.0:
                content = content[:100] + '...' if len(content) > 100 else content
            output.append(f"- {learning.get('title', 'Lesson')}: {content}\n")
        output.append('\n')
    
    if old:
        output.append('### Archived Insights\n\n')
        themes = {}
        for learning, _ in old:
            theme = learning.get('theme', 'General')
            if theme not in themes:
                themes[theme] = []
            themes[theme].append(learning.get('title', 'Lesson'))
        
        for theme, titles in themes.items():
            output.append(f"- **{theme}**: {', '.join(titles[:3])}\n")
        output.append('\n')
    
    return ''.join(output)

def main():
    """Synthesize and write active_learnings.md."""
    learnings = load_learnings()
    synthesis = synthesize_learnings(learnings)
    
    os.makedirs(os.path.dirname(ACTIVE_FILE), exist_ok=True)
    
    with open(ACTIVE_FILE, 'w') as f:
        f.write(synthesis)
    
    print(f"Synthesized {len(learnings)} learnings into {ACTIVE_FILE}")

if __name__ == '__main__':
    main()
