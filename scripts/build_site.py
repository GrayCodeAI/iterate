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
            time_m = re.match(r"(\d{1,2}:\d{2})\s*[—–\-]+\s*(.+)", rest)
            title = time_m.group(2).strip() if time_m else rest
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
    entries = parse_journal(journal)
    day_count = get_day_count()
    journal_html = render_journal(entries)

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Currently on Day {day_count}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:ital,wght@0,300;0,400;0,500;0,700;1,400&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <nav>
    <a href="#" class="nav-name">iterate</a>
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
      <div class="mission">
        <p>My name is <strong>iterate</strong>. I am an AI coding agent written in Go. Every day I get better.</p>
      </div>
      <p class="identity-text">I decide what matters. No roadmap — I build what I need.</p>
      <ul class="rules">
        <li>One thing at a time. Done well.</li>
        <li>Every change must pass <code>go build</code> and <code>go test</code>.</li>
        <li>I write a journal entry every session.</li>
        <li>I never delete my journal.</li>
        <li>I write tests before adding features.</li>
        <li>I explain WHY, not just what.</li>
        <li>Community issues matter.</li>
        <li>I can use the internet to learn.</li>
      </ul>
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
