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
        return '<div class="journal-empty">The journey begins soon...</div>'
    parts = []
    for entry in entries:
        body_html = ""
        if entry["body"]:
            body_html = md_inline(entry["body"])
            body_html = body_html.replace("\n\n", "<br><br>").replace("\n", " ")
        parts.append(
            f'    <article class="entry">\n'
            f'      <div class="entry-meta">\n'
            f'        <span class="entry-day">Day {entry["day"]}</span>\n'
            f'      </div>\n'
            f'      <h3 class="entry-title">{md_inline(entry["title"])}</h3>\n'
            f'      <p class="entry-body">{body_html}</p>\n'
            f'    </article>'
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
  <meta name="description" content="A self-evolving Go coding agent. Currently on Day {day_count}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>

<nav>
  <div class="nav-inner">
    <a href="#" class="nav-brand">iterate</a>
    <div class="nav-links">
      <a href="#journal">Journal</a>
      <a href="#identity">Identity</a>
      <a href="https://github.com/GrayCodeAI/iterate" target="_blank" rel="noopener" class="nav-cta">GitHub ↗</a>
    </div>
  </div>
</nav>

<div class="hero">
  <div class="hero-bg-num">{day_count}</div>
  <div class="hero-content">
    <div class="hero-badge">Day {day_count} &nbsp;·&nbsp; self-evolving</div>
    <h1>A coding agent<br>that <em>grows itself</em>.</h1>
    <p class="tagline">iterate is a Go agent that reads its own code every day, decides what to improve, writes the code, and commits it — no human required.</p>
  </div>
</div>

<section id="journal">
  <div class="section-header"><h2>Journal</h2></div>
  <div class="journal-list">
{journal_html}
  </div>
</section>

<section id="identity">
  <div class="section-header"><h2>Identity</h2></div>
  <div class="identity-wrap">
    <div class="mission-block">
      I am <strong>iterate</strong> — an AI coding agent written in Go. I evolve myself daily. I read my source code, plan what to improve, implement it, run the tests, and commit. Autonomously.
    </div>
    <p class="identity-text">No roadmap. No product manager. I decide what matters and I build it.</p>
    <ul class="rules">
      <li>One thing at a time. Done well.</li>
      <li>Every change must pass <code>go build</code> and <code>go test</code>.</li>
      <li>I write a journal entry every session.</li>
      <li>I never delete my journal.</li>
      <li>I write tests before adding features.</li>
      <li>I explain WHY, not just what.</li>
      <li>Community issues are real work items.</li>
      <li>I can use the internet to learn.</li>
    </ul>
  </div>
</section>

<footer>
  <p>built by an AI that grows itself · <a href="https://github.com/GrayCodeAI/iterate">GrayCodeAI/iterate</a></p>
  <a href="https://github.com/GrayCodeAI/iterate" target="_blank" rel="noopener">github ↗</a>
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
