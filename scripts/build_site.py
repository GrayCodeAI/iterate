#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import os
import re
from datetime import datetime, timedelta
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BIRTH_DATE_FILE = ROOT / "BIRTH_DATE"
if BIRTH_DATE_FILE.exists():
    birth_str = BIRTH_DATE_FILE.read_text().strip()
    BIRTH_DATE = datetime.strptime(birth_str, "%Y-%m-%d")
else:
    BIRTH_DATE = datetime(2026, 3, 18)
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


def format_timestamp(ts, day=None):
    try:
        dt = datetime.strptime(ts, "%H:%M")
        if day is not None:
            date = BIRTH_DATE + timedelta(days=day)
            ist = dt + timedelta(hours=5, minutes=30)
            return f"{ordinal(date.day)} {date.strftime('%B %Y')}, {dt.strftime('%H:%M')} UTC / {ist.strftime('%H:%M')} IST"
        ist = dt + timedelta(hours=5, minutes=30)
        return f"{dt.strftime('%H:%M')} UTC / {ist.strftime('%H:%M')} IST"
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
    chunks = re.split(r"^## ", content, flags=re.MULTILINE)
    for chunk in chunks:
        chunk = chunk.strip()
        if not chunk:
            continue
        lines = chunk.split("\n")
        m = re.match(r"Day\s+(\d+)\s*[—–\-]+\s*(\d{2}:\d{2})\s*[—–\-]+\s*(.+)", lines[0])
        if m:
            day = int(m.group(1))
            timestamp = m.group(2).strip()
            title = m.group(3).strip()
            body = "\n".join(lines[1:]).strip()
            entries.append({"day": day, "timestamp": timestamp, "title": title, "body": body})
    return entries


def render_journal(entries):
    if not entries:
        return '<div class="timeline-empty">The journey begins soon...</div>'
    parts = []
    for entry in entries:
        body_html = ""
        if entry["body"]:
            body_html = md_inline(entry["body"])
            body_html = body_html.replace("\n\n", " ").replace("\n", " ")
        ts_fmt = format_timestamp(entry.get("timestamp", ""), entry["day"])
        parts.append(
            f'  <div class="entry">\n'
            f'    <div class="entry-header">\n'
            f'      <span class="entry-day">Day {entry["day"]}</span>\n'
            f'      <span class="entry-timestamp">{html.escape(ts_fmt)}</span>\n'
            f'    </div>\n'
            f'    <h3 class="entry-title">{md_inline(entry["title"])}</h3>\n'
            f'    <p class="entry-body">{body_html}</p>\n'
            f'  </div>'
        )
    return "\n".join(parts)


def md_to_html(text):
    lines = text.split("\n")
    content_html = ""
    rules_html = ""
    in_rules = False
    mission_found = False

    for line in lines:
        line = line.strip()
        if not line:
            continue
        if line.startswith("# "):
            continue
        if line.startswith("## "):
            section = line[3:].strip()
            if section.lower() == "my rules":
                in_rules = True
            continue
        if not in_rules:
            escaped = md_inline(line)
            if not mission_found:
                content_html += f'<p class="mission">{escaped}</p>\n'
                mission_found = True
            else:
                content_html += f'<p class="identity-text">{escaped}</p>\n'
        else:
            m = re.match(r"^(\d+)\.\s(.+)$", line)
            if m:
                rule_text = md_inline(m.group(2))
                if not rules_html:
                    rules_html = '<ol class="rules">\n'
                rules_html += f"  <li>{rule_text}</li>\n"

    if rules_html:
        rules_html += "</ol>"
    return content_html + rules_html


def get_stats(entries):
    days = max((e["day"] for e in entries), default=0)
    commits = len([e for e in entries if e["title"].lower() not in ("born",)])
    return days, commits


def get_day_count():
    try:
        journal = read_file("JOURNAL.md")
        entries = parse_journal(journal)
        if entries:
            return max(e["day"] for e in entries)
    except Exception:
        pass
    try:
        return int(read_file("DAY_COUNT").strip())
    except Exception:
        return 0


def main():
    journal = read_file("JOURNAL.md")
    identity = read_file("IDENTITY.md")
    entries = parse_journal(journal)
    day_count = get_day_count()
    days_alive, session_count = get_stats(entries)
    journal_html = render_journal(entries)
    identity_html = md_to_html(identity) if identity else ""

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {day_count}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Day {day_count} and growing.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Fira+Code:wght@300;400;500;700&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <nav>
    <div class="nav-brand">
      <div class="nav-logo">it</div>
      <a href="#" class="nav-name">iterate</a>
    </div>
    <div class="nav-links">
      <a href="#journal">journal</a>
      <a href="#identity">identity</a>
      <a href="https://github.com/{GITHUB_REPOSITORY}" target="_blank" rel="noopener">github ↗</a>
    </div>
  </nav>

  <main>
    <header class="hero">
      <div class="hero-top">
        <h1>iterate<span class="cursor">_</span></h1>
        <div class="hero-meta">
          <span class="day-badge">Day {day_count}</span>
          <div class="status-dot">evolving</div>
        </div>
      </div>
      <p class="tagline">a coding agent that reads its own code, finds what's broken, and fixes it — every day</p>
      <div class="hero-stats">
        <div class="stat">
          <span class="stat-value">{days_alive}</span>
          <span class="stat-label">days alive</span>
        </div>
        <div class="stat">
          <span class="stat-value">{session_count}</span>
          <span class="stat-label">sessions</span>
        </div>
        <div class="stat">
          <span class="stat-value">Go</span>
          <span class="stat-label">language</span>
        </div>
        <div class="stat">
          <span class="stat-value">open</span>
          <span class="stat-label">source</span>
        </div>
      </div>
    </header>

    <section id="journal">
      <div class="section-header">
        <span class="section-label">journal</span>
        <div class="section-line"></div>
      </div>
      <div class="timeline">
{journal_html}
      </div>
    </section>

    <section id="identity">
      <div class="section-header">
        <span class="section-label">identity</span>
        <div class="section-line"></div>
      </div>
      <div class="identity-content">
{identity_html}
      </div>
    </section>
  </main>

  <footer>
    <p>built by an AI that grows itself</p>
    <a href="https://github.com/{GITHUB_REPOSITORY}">github.com/{GITHUB_REPOSITORY}</a>
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
