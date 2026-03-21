#!/usr/bin/env python3
"""Fetch GitHub discussions for iterate social session."""

import subprocess
import json
import sys
from datetime import datetime

def fetch_discussions():
    """Fetch discussions using gh CLI GraphQL."""
    query = """
    query {
      repository(owner: "GrayCodeAI", name: "iterate") {
        discussions(first: 20, orderBy: {field: UPDATED_AT, direction: DESC}) {
          edges {
            node {
              id
              title
              url
              body
              author {
                login
              }
              createdAt
              comments(first: 5) {
                edges {
                  node {
                    author {
                      login
                    }
                    body
                  }
                }
              }
            }
          }
        }
      }
    }
    """
    
    try:
        result = subprocess.run(
            ['gh', 'api', 'graphql', '-f', f'query={query}'],
            capture_output=True,
            text=True,
            timeout=30
        )
        
        if result.returncode == 0 and result.stdout:
            data = json.loads(result.stdout)
            return data.get('data', {}).get('repository', {}).get('discussions', {}).get('edges', [])
        return []
    except Exception as e:
        print(f"Error fetching discussions: {e}", file=sys.stderr)
        return []

def main():
    """Generate DISCUSSIONS_TODAY.md from GitHub discussions."""
    discussions = fetch_discussions()
    
    if not discussions:
        print("# Discussions Today\n\nNo recent discussions found.", file=sys.stdout)
        return
    
    output = ["# Discussions Today\n\n"]
    output.append(f"*Fetched at: {datetime.utcnow().isoformat()}*\n\n")
    
    for edge in discussions:
        node = edge.get('node', {})
        output.append(f"## {node.get('title', 'Untitled')}\n\n")
        output.append(f"**Author**: {node.get('author', {}).get('login', 'unknown')}\n")
        output.append(f"**URL**: {node.get('url', '#')}\n\n")
        output.append(f"**Body**: {node.get('body', '')}\n\n")
        
        # Add comments
        comments = node.get('comments', {}).get('edges', [])
        if comments:
            output.append("**Comments**:\n\n")
            for comment_edge in comments:
                comment = comment_edge.get('node', {})
                output.append(f"- **{comment.get('author', {}).get('login', 'unknown')}**: ")
                output.append(f"{comment.get('body', '')}\n\n")
        
        output.append("---\n\n")
    
    print(''.join(output), file=sys.stdout)

if __name__ == '__main__':
    main()
