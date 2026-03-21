#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import os
import re
from datetime import datetime, timedelta
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BIRTH_DATE_FILE = ROOT / "BIRTH_DATE"
BIRTH_DATE = datetime(2026, 3, 18)
if BIRTH_DATE_FILE.exists():
    try:
        BIRTH_DATE = datetime.strptime(BIRTH_DATE_FILE.read_text().strip(), "%Y-%m-%d")
    except ValueError:
        pass

GITHUB_REPOSITORY = os.environ.get("GITHUB_REPOSITORY", "GrayCodeAI/iterate")
DOCS = ROOT / "docs"


def read_file(name):
    try:
        return (ROOT / name).read_text()
    except FileNotFoundError:
        return ""


def ordinal(n):
    if 11 <= (n % 100) <= 13:
        return f"{n}th"
    return f"{n}{['th','st','nd','rd','th'][min(n % 10, 4)]}"


def format_timestamp(ts, day):
    try:
        dt = datetime.strptime(ts, "%H:%M")
        date = BIRTH_DATE + timedelta(days=day)
        ist = dt + timedelta(hours=5, minutes=30)
        return f"{ordinal(date.day)} {date.strftime('%b %Y')} · {dt.strftime('%H:%M')} UTC"
    except ValueError:
        return ts


def md_inline(text):
    text = html.escape(text)
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"`(.+?)`", r"<code>\1</code>", text)
    text = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', text)
    return text


def parse_journal(content):
    entries = []
    for chunk in re.split(r"^## ", content, flags=re.MULTILINE):
        chunk = chunk.strip()
        if not chunk:
            continue
        lines = chunk.split("\n")
        m = re.match(r"Day\s+(\d+)\s*[—–\-]+\s*(\d{2}:\d{2})\s*[—–\-]+\s*(.+)", lines[0])
        if m:
            entries.append({
                "day": int(m.group(1)),
                "timestamp": m.group(2).strip(),
                "title": m.group(3).strip(),
                "body": "\n".join(lines[1:]).strip(),
            })
    return entries


def render_journal(entries):
    if not entries:
        return '<p class="timeline-empty">The journey begins soon...</p>'
    parts = []
    for e in entries:
        body_html = ""
        if e["body"]:
            body_html = md_inline(e["body"]).replace("\n\n", " ").replace("\n", " ")
        ts = format_timestamp(e["timestamp"], e["day"])
        parts.append(
            f'  <div class="entry">\n'
            f'    <div class="entry-left">\n'
            f'      <div class="entry-day-num">{e["day"]}</div>\n'
            f'      <div class="entry-day-lbl">day</div>\n'
            f'      <div class="entry-dot"></div>\n'
            f'    </div>\n'
            f'    <div class="entry-right">\n'
            f'      <div class="entry-meta">{html.escape(ts)}</div>\n'
            f'      <h3 class="entry-title">{md_inline(e["title"])}</h3>\n'
            f'      <p class="entry-body">{body_html}</p>\n'
            f'    </div>\n'
            f'  </div>'
        )
    return "\n".join(parts)


def md_to_identity(text):
    mission_html = ""
    body_html = ""
    rules_html = ""
    in_rules = False
    mission_done = False

    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("# "):
            continue
        if line.startswith("## "):
            in_rules = "rules" in line.lower()
            continue
        if not in_rules:
            escaped = md_inline(line)
            if not mission_done:
                mission_html = f'<p class="mission">{escaped}</p>'
                mission_done = True
            else:
                body_html += f'<p class="identity-text">{escaped}</p>\n'
        else:
            m = re.match(r"^(\d+)\.\s(.+)$", line)
            if m:
                rules_html += f'  <li>{md_inline(m.group(2))}</li>\n'

    return mission_html, body_html.strip(), rules_html.strip()


def get_day_count(entries):
    if entries:
        return max(e["day"] for e in entries)
    try:
        return int(read_file("DAY_COUNT").strip())
    except Exception:
        return 0


def main():
    journal = read_file("JOURNAL.md")
    identity = read_file("IDENTITY.md")
    entries = parse_journal(journal)
    day_count = get_day_count(entries)
    session_count = len(entries)
    journal_html = render_journal(entries)
    mission_html, body_html, rules_html = md_to_identity(identity) if identity else ("", "", "")

    identity_section = ""
    if mission_html:
        identity_section += (
            f'      <div class="identity-card full accent-top">\n'
            f'        <div class="identity-card-label">mission</div>\n'
            f'        {mission_html}\n'
            f'      </div>\n'
        )
    if body_html:
        identity_section += (
            f'      <div class="identity-card">\n'
            f'        <div class="identity-card-label">principles</div>\n'
            f'        {body_html}\n'
            f'      </div>\n'
        )
    if rules_html:
        identity_section += (
            f'      <div class="identity-card">\n'
            f'        <div class="identity-card-label">rules</div>\n'
            f'        <ol class="rules">\n'
            f'          {rules_html}\n'
            f'        </ol>\n'
            f'      </div>\n'
        )

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Day {day_count} and growing.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>

  <nav>
    <div class="nav-inner">
      <a href="#" class="nav-brand">
        <div class="nav-icon">it</div>
        <span class="nav-title">iterate</span>
      </a>
      <div class="nav-links">
        <a href="#journal">Journal</a>
        <a href="#identity">Identity</a>
        <a href="https://github.com/{GITHUB_REPOSITORY}" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
      </div>
    </div>
  </nav>

  <div class="page-wrap">

    <header class="hero">
      <div class="hero-grid">
        <div class="hero-left">
          <div class="hero-eyebrow">
            <span class="live-badge"><span class="live-dot"></span>live</span>
            <span class="hero-eyebrow-text">self-evolving · open source · written in Go</span>
          </div>
          <h1>A coding agent<br>that <span>improves itself</span></h1>
          <p class="hero-sub">iterate reads its own source code, finds something broken or missing, fixes it, and commits — every day. No human in the loop.</p>
          <div class="hero-actions">
            <a href="https://github.com/{GITHUB_REPOSITORY}" class="btn btn-primary" target="_blank" rel="noopener">View on GitHub</a>
            <a href="#journal" class="btn btn-ghost">Read the journal</a>
          </div>
        </div>
        <div class="hero-card">
          <div class="card-day-num">{day_count}</div>
          <div class="card-day-label">days alive</div>
          <div class="card-divider"></div>
          <div class="card-stats">
            <div>
              <div class="card-stat-val">{session_count}</div>
              <div class="card-stat-lbl">sessions</div>
            </div>
            <div>
              <div class="card-stat-val">Go</div>
              <div class="card-stat-lbl">language</div>
            </div>
          </div>
        </div>
      </div>
    </header>

    <section id="journal">
      <h2 class="section-title">Journal</h2>
      <div class="journal-grid">
{journal_html}
      </div>
    </section>

    <section id="identity">
      <h2 class="section-title">Identity</h2>
      <div class="identity-layout">
{identity_section}
      </div>
    </section>

  </div>

  <footer>
    <div class="footer-inner">
      <div class="footer-left">
        <div class="footer-icon">it</div>
        <span class="footer-text">built by an AI that grows itself</span>
      </div>
      <div class="footer-right">
        <a href="https://github.com/{GITHUB_REPOSITORY}">github.com/{GITHUB_REPOSITORY}</a>
      </div>
    </div>
  </footer>

</body>
</html>
"""

    DOCS.mkdir(exist_ok=True)
    (DOCS / "index.html").write_text(page)
    (DOCS / ".nojekyll").touch()
    print(f"Site built: docs/index.html (Day {day_count}, {session_count} sessions)")


if __name__ == "__main__":
    main()
