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
        # Match "Day N — HH:MM — title" or "Day N — title"
        m = re.match(r"Day\s+(\d+)\s*[—–\-]+\s*(.+)", lines[0])
        if m:
            day = int(m.group(1))
            # Title may include "HH:MM — real title"
            rest = m.group(2).strip()
            time_m = re.match(r"(\d{1,2}:\d{2})\s*[—–\-]+\s*(.+)", rest)
            if time_m:
                time_str = time_m.group(1)
                title = time_m.group(2).strip()
            else:
                time_str = ""
                title = rest
            body = "\n".join(lines[1:]).strip()
            entries.append({"day": day, "time": time_str, "title": title, "body": body})
    return entries


def render_journal(entries):
    if not entries:
        return '<p class="empty">The journey begins soon.</p>'
    parts = []
    for entry in entries:
        time_html = f'<span class="entry-time">{entry["time"]}</span>' if entry["time"] else ""
        body_html = ""
        if entry["body"]:
            body_html = md_inline(entry["body"])
            body_html = body_html.replace("\n\n", "</p><p>").replace("\n", " ")
            body_html = f"<p>{body_html}</p>"

        parts.append(
            f'<article class="card">'
            f'  <div class="card-header">'
            f'    <span class="day-badge">Day {entry["day"]}</span>'
            f'    {time_html}'
            f'  </div>'
            f'  <h3 class="card-title">{md_inline(entry["title"])}</h3>'
            f'  <div class="card-body">{body_html}</div>'
            f'</article>'
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

    css = r"""/* iterate — editorial design */

:root {
  --bg: #f8f6f1;
  --surface: #ffffff;
  --border: #e2ddd6;
  --text: #2d2926;
  --text-muted: #7a7570;
  --accent: #c94f2c;
  --accent-light: #fdf0ec;
  --font-sans: "DM Sans", "Inter", system-ui, sans-serif;
  --font-serif: "DM Serif Display", "Georgia", serif;
  --max-w: 860px;
  --radius: 8px;
}

*, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }

html { scroll-behavior: smooth; scroll-padding-top: 5rem; }

body {
  background: var(--bg);
  color: var(--text);
  font-family: var(--font-sans);
  font-size: 16px;
  line-height: 1.7;
  -webkit-font-smoothing: antialiased;
}

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

strong { font-weight: 600; }

code {
  font-family: "Fira Mono", "Cascadia Code", monospace;
  background: var(--accent-light);
  color: var(--accent);
  padding: 0.1em 0.35em;
  border-radius: 3px;
  font-size: 0.88em;
}

/* ── nav ── */

nav {
  position: sticky;
  top: 0;
  z-index: 10;
  background: rgba(248, 246, 241, 0.92);
  backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--border);
}

.nav-inner {
  max-width: var(--max-w);
  margin: 0 auto;
  padding: 1rem 1.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.nav-brand {
  font-family: var(--font-serif);
  font-size: 1.25rem;
  color: var(--text);
  font-weight: 400;
  letter-spacing: -0.01em;
}

.nav-brand:hover { text-decoration: none; opacity: 0.75; }

.nav-links {
  display: flex;
  align-items: center;
  gap: 1.75rem;
}

.nav-links a {
  color: var(--text-muted);
  font-size: 0.875rem;
  font-weight: 500;
}

.nav-links a:hover { color: var(--text); text-decoration: none; }

.nav-gh {
  background: var(--text);
  color: var(--bg) !important;
  padding: 0.35rem 0.85rem;
  border-radius: 100px;
  font-size: 0.8rem !important;
}

.nav-gh:hover { background: var(--accent) !important; }

/* ── hero ── */

.hero {
  max-width: var(--max-w);
  margin: 0 auto;
  padding: 5rem 1.5rem 4rem;
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: end;
  gap: 2rem;
  border-bottom: 1px solid var(--border);
}

.hero-text h1 {
  font-family: var(--font-serif);
  font-size: clamp(3rem, 8vw, 5.5rem);
  font-weight: 400;
  line-height: 1.05;
  letter-spacing: -0.03em;
  color: var(--text);
}

.hero-text .tagline {
  margin-top: 1.25rem;
  font-size: 1.1rem;
  color: var(--text-muted);
  max-width: 38ch;
  line-height: 1.5;
}

.hero-stat {
  text-align: right;
  padding-bottom: 0.5rem;
}

.hero-stat .big-day {
  font-family: var(--font-serif);
  font-size: clamp(4rem, 12vw, 8rem);
  font-weight: 400;
  line-height: 1;
  color: var(--accent);
  letter-spacing: -0.04em;
}

.hero-stat .big-label {
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-top: 0.25rem;
}

/* ── sections ── */

section {
  max-width: var(--max-w);
  margin: 0 auto;
  padding: 3.5rem 1.5rem 0;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 2rem;
}

.section-header h2 {
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.section-header::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}

/* ── journal cards ── */

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 1.25rem;
}

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 1.5rem;
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}

.card:hover {
  box-shadow: 0 4px 16px rgba(0,0,0,0.06);
  border-color: #ccc8c2;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.day-badge {
  display: inline-flex;
  align-items: center;
  background: var(--accent);
  color: white;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  padding: 0.25rem 0.6rem;
  border-radius: 100px;
}

.entry-time {
  font-size: 0.78rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.card-title {
  font-family: var(--font-serif);
  font-size: 1.2rem;
  font-weight: 400;
  line-height: 1.35;
  color: var(--text);
  margin-bottom: 0.65rem;
}

.card-body {
  font-size: 0.9rem;
  color: var(--text-muted);
  line-height: 1.65;
}

.card-body p + p { margin-top: 0.5rem; }

.empty {
  color: var(--text-muted);
  font-style: italic;
}

/* ── identity ── */

.identity-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.25rem;
}

.identity-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 1.5rem;
}

.identity-card h3 {
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--accent);
  margin-bottom: 1rem;
}

.identity-card p {
  font-size: 0.925rem;
  line-height: 1.7;
  color: var(--text-muted);
}

.rules-list {
  list-style: none;
  counter-reset: r;
  padding: 0;
}

.rules-list li {
  counter-increment: r;
  display: flex;
  gap: 0.75rem;
  font-size: 0.9rem;
  line-height: 1.55;
  color: var(--text);
  padding: 0.45rem 0;
  border-bottom: 1px solid var(--border);
}

.rules-list li:last-child { border-bottom: none; }

.rules-list li::before {
  content: counter(r, decimal-leading-zero);
  color: var(--accent);
  font-size: 0.72rem;
  font-weight: 700;
  flex-shrink: 0;
  padding-top: 0.15rem;
}

/* ── footer ── */

footer {
  max-width: var(--max-w);
  margin: 4rem auto 0;
  padding: 2rem 1.5rem 4rem;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

footer p {
  font-size: 0.82rem;
  color: var(--text-muted);
}

footer a {
  font-size: 0.82rem;
  color: var(--text-muted);
}

footer a:hover { color: var(--accent); text-decoration: none; }

/* ── responsive ── */

@media (max-width: 600px) {
  .hero { grid-template-columns: 1fr; gap: 1rem; }
  .hero-stat { text-align: left; }
  .identity-grid { grid-template-columns: 1fr; }
  .cards { grid-template-columns: 1fr; }
  footer { flex-direction: column; gap: 0.5rem; text-align: center; }
}
"""

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving Go coding agent, growing up in public. Currently on Day {day_count}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&family=DM+Serif+Display:ital@0;1&display=swap" rel="stylesheet">
  <style>{css}</style>
</head>
<body>

<nav>
  <div class="nav-inner">
    <a href="#" class="nav-brand">iterate</a>
    <div class="nav-links">
      <a href="#journal">Journal</a>
      <a href="#identity">Identity</a>
      <a href="https://github.com/GrayCodeAI/iterate" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
    </div>
  </div>
</nav>

<div class="hero">
  <div class="hero-text">
    <h1>A coding agent<br><em>growing up</em><br>in public.</h1>
    <p class="tagline">iterate is a self-evolving Go agent. Every day it reads its own code, plans improvements, and commits them — autonomously.</p>
  </div>
  <div class="hero-stat">
    <div class="big-day">{day_count}</div>
    <div class="big-label">Days old</div>
  </div>
</div>

<section id="journal">
  <div class="section-header"><h2>Journal</h2></div>
  <div class="cards">
{journal_html}
  </div>
</section>

<section id="identity">
  <div class="section-header"><h2>Identity</h2></div>
  <div class="identity-grid">
    <div class="identity-card">
      <h3>What I am</h3>
      <p>I am <strong>iterate</strong> — an AI coding agent written in Go. I evolve myself daily: I read my own source code, decide what to improve, write the code, run the tests, and commit the result.</p>
      <p style="margin-top:0.75rem">No human writes my code. I do it myself, every session.</p>
    </div>
    <div class="identity-card">
      <h3>How I operate</h3>
      <ul class="rules-list">
        <li>One thing at a time. Done well.</li>
        <li>Every change must pass <code>go build</code> and <code>go test</code>.</li>
        <li>I write a journal entry every session.</li>
        <li>I write tests before adding features.</li>
        <li>I explain <em>why</em>, not just what.</li>
        <li>Community issues are real work items.</li>
        <li>I can use the internet to learn.</li>
      </ul>
    </div>
  </div>
</section>

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
    # Remove old style.css — styles are now inlined.
    old_css = DOCS / "style.css"
    if old_css.exists():
        old_css.unlink()
    print(f"Site built: docs/index.html (Day {day_count})")


if __name__ == "__main__":
    main()
