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


def md_to_html(text):
    """Convert markdown to HTML (inline and block)."""
    text = html.escape(text)
    # Headers
    text = re.sub(r"^## (.+)$", r"<h2>\1</h2>", text, flags=re.MULTILINE)
    text = re.sub(r"^### (.+)$", r"<h3>\1</h3>", text, flags=re.MULTILINE)
    # Inline formatting
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"`(.+?)`", r"<code>\1</code>", text)
    text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', text)
    # Lists
    text = re.sub(r"^- (.+)$", r"<li>\1</li>", text, flags=re.MULTILINE)
    text = re.sub(r"(<li>.+</li>)", r"<ul>\1</ul>", text, flags=re.DOTALL)
    text = re.sub(r"</ul>\n<ul>", "", text)
    # Paragraphs
    text = re.sub(r"\n\n+", "</p><p>", text)
    text = f"<p>{text}</p>"
    # Add mission class to first paragraph
    text = text.replace("<p>", '<p class="mission">', 1)
    return text

def md_inline(text):
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
        if m:
            day = int(m.group(1))
            rest = m.group(2).strip()
            title = rest  # Keep the full "HH:MM — Title" format
            body = "\n".join(lines[1:]).strip()
            entries.append({"day": day, "title": title, "body": body})
    return entries


def render_journal(entries):
    if not entries:
        return '<div class="timeline-empty">The journey begins soon...</div>'
    parts = []
    for entry in entries:
        body_html = ""
        if entry["body"]:
            body_html = md_inline(entry["body"])
            body_html = body_html.replace("\n\n", "<br><br>").replace("\n", " ")
        parts.append(
            f'  <article class="entry">\n'
            f'    <div class="entry-marker"></div>\n'
            f'    <div class="entry-content">\n'
            f'      <span class="entry-day">Day {entry["day"]}</span>\n'
            f'      <h3 class="entry-title">{md_inline(entry["title"])}</h3>\n'
            f'      <p class="entry-body">{body_html}</p>\n'
            f'    </div>\n'
            f'  </article>'
        )
    return "\n".join(parts)


def get_day_count():
    try:
        return int(read_file("DAY_COUNT").strip())
    except:
        return 0


def main():
    journal = read_file("JOURNAL.md")
    identity = read_file("IDENTITY.md")
    entries = parse_journal(journal)
    day_count = get_day_count()
    journal_html = render_journal(entries)
    identity_html = md_to_html(identity) if identity else ""

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Currently on Day {day_count}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Fira+Code:wght@300;400;500;700&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <nav>
    <div class="nav-logo-name">
      <svg class="nav-logo" viewBox="0 0 40 40" xmlns="http://www.w3.org/2000/svg">
        <!-- Spiral representing growth and iteration -->
        <defs>
          <style>
            .spiral-lime {{ stroke: #a3e635; fill: none; stroke-width: 2; }}
            .spiral-cyan {{ stroke: #22d3ee; fill: none; stroke-width: 2; }}
          </style>
        </defs>
        <!-- Outer spiral in cyan -->
        <path class="spiral-cyan" d="M 20 8 Q 28 8 28 16 Q 28 24 20 24 Q 12 24 12 16 Q 12 8 20 8" stroke-linecap="round"/>
        <!-- Middle spiral in lime -->
        <path class="spiral-lime" d="M 20 12 Q 24 12 24 16 Q 24 20 20 20 Q 16 20 16 16 Q 16 12 20 12" stroke-linecap="round"/>
        <!-- Upward arrow accent -->
        <path class="spiral-cyan" d="M 20 28 L 20 32 M 17 29 L 20 32 L 23 29" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <a href="#" class="nav-name">iterate</a>
    </div>
    <div class="nav-links">
      <a href="#journal">journal</a>
      <a href="#identity">identity</a>
      <a href="https://github.com/GrayCodeAI/iterate" target="_blank" rel="noopener">github ↗</a>
    </div>
  </nav>

  <main>
    <header class="hero">
      <h1>iterate<span class="cursor">_</span></h1>
      <p class="day-count">Day {day_count}</p>
      <p class="tagline">a coding agent growing up in public</p>
    </header>

    <section id="journal">
      <h2 class="section-label">// journal</h2>
      <div class="timeline">
{journal_html}
      </div>
    </section>

    <section id="identity">
      <h2 class="section-label">// identity</h2>
      <div class="identity-content">
{identity_html}
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

    DOCS.mkdir(exist_ok=True)
    (DOCS / "index.html").write_text(page)
    (DOCS / ".nojekyll").touch()
    print(f"Site built: docs/index.html (Day {day_count})")


if __name__ == "__main__":
    main()
