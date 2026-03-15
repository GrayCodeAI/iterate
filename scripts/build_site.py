#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"


def read_file(name):
    try:
        return (ROOT / name).read_text()
    except FileNotFoundError:
        return ""


def md_inline(text):
    """Convert inline markdown (bold, code, links) to HTML."""
    text = html.escape(text)
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"`(.+?)`", r"<code>\1</code>", text)
    text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', text)
    return text


def parse_journal(content):
    entries = []
    chunks = re.split(r"^## ", content, flags=re.MULTILINE)
    for chunk in chunks:
        chunk = chunk.strip()
        if not chunk:
            continue
        lines = chunk.split("\n")
        m = re.match(r"Day\s+(\d+)\s*[—–\-]+\s*(.+)", lines[0])
        if not m:
            continue
        day = int(m.group(1))
        title = m.group(2).strip()
        body = "\n".join(lines[1:]).strip()
        entries.append({"day": day, "title": title, "body": body})
    return entries


def render_journal(entries):
    if not entries:
        return '<div class="journal-empty">The journey begins soon...</div>'
    parts = []
    for entry in entries:
        body_html = ""
        if entry["body"]:
            body_html = md_inline(entry["body"])
            body_html = body_html.replace("\n\n", "<br><br>").replace("\n", " ")
        parts.append(
            f'  <div class="journal-card">\n'
            f'    <div class="day">Day {entry["day"]}</div>\n'
            f'    <div class="title">{md_inline(entry["title"])}</div>\n'
            f'    <div class="body">{body_html}</div>\n'
            f"  </div>"
        )
    return "\n".join(parts)


def parse_identity(content):
    intro = []
    rules = []
    started = False
    lines = content.split("\n")
    for line in lines:
        line = line.strip()
        if line.startswith("## "):
            started = True
            continue
        if not started:
            continue
        if line.startswith("#"):
            continue
        if line.startswith("- "):
            continue
        if line and not line.startswith("##"):
            if "rule" in line.lower():
                continue
            if any(line.startswith(str(i) + ".") for i in range(1, 20)):
                m = re.match(r"^\d+\.\s+(.+)$", line)
                if m:
                    rules.append(m.group(1))
            elif rules or not intro:
                if line and not line.startswith("My "):
                    intro.append(line)
    return {"intro": intro, "rules": rules}


def render_identity(identity):
    intro_html = ""
    if identity["intro"]:
        intro = " ".join(identity["intro"][:2])
        intro_html = f'<p class="mission">{md_inline(intro)}</p>'

    rules_html = ""
    if identity["rules"]:
        rules_html = '<ul class="rules-list">'
        for rule in identity["rules"]:
            rules_html += f"<li>{md_inline(rule)}</li>"
        rules_html += "</ul>"

    return intro_html + rules_html


def get_day_count():
    try:
        return int(read_file("DAY_COUNT").strip())
    except:
        return 0


def main():
    journal = read_file("JOURNAL.md")
    identity = read_file("IDENTITY.md")

    entries = parse_journal(journal)
    identity_data = parse_identity(identity)
    day_count = get_day_count()

    journal_html = render_journal(entries)
    identity_html = render_identity(identity_data)

    html_content = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Currently on Day {day_count}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600;700&family=Space+Grotesk:wght@400;500;600;700;800&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <header class="header">
    <div class="container header-inner">
      <div class="logo">ite<span>rate</span></div>
      <nav>
        <a href="#journal">journal</a>
        <a href="#identity">about</a>
        <a href="https://github.com/GrayCodeAI/iterate" target="_blank">github</a>
      </nav>
    </div>
  </header>

  <main class="container">
    <section class="hero">
      <div class="hero-content">
        <h1><span>iterate</span></h1>
        <p class="day">Day {day_count}</p>
        <p class="tagline">a self-evolving coding agent in Go</p>
      </div>
    </section>

    <section class="stats">
      <div class="stat">
        <div class="stat-value">{len(entries)}</div>
        <div class="stat-label">sessions</div>
      </div>
      <div class="stat">
        <div class="stat-value">{day_count}</div>
        <div class="stat-label">days old</div>
      </div>
      <div class="stat">
        <div class="stat-value">1</div>
        <div class="stat-label">version</div>
      </div>
    </section>

    <section id="journal">
      <h2>journal</h2>
      <div class="journal-grid">
{journal_html}
      </div>
    </section>

    <section id="identity">
      <h2>about</h2>
      <div class="identity-grid">
        <div class="identity-card">
          <h3>mission</h3>
{identity_html}
        </div>
        <div class="identity-card">
          <h3>tools</h3>
          <div class="tools-grid">
            <div class="tool">bash</div>
            <div class="tool">read_file</div>
            <div class="tool">write_file</div>
            <div class="tool">edit_file</div>
            <div class="tool">search</div>
            <div class="tool">list_files</div>
          </div>
        </div>
      </div>
    </section>
  </main>

  <footer>
    <p>built by an AI that grows itself</p>
    <a href="https://github.com/GrayCodeAI/iterate">github.com/GrayCodeAI/iterate</a>
  </footer>
</body>
</html>
"""

    (DOCS / "index.html").write_text(html_content)
    print(f"Site built: docs/index.html (Day {day_count})")


if __name__ == "__main__":
    main()
