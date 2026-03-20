#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import re
from datetime import datetime, timedelta
from pathlib import Path

BIRTH_DATE = datetime(2026, 3, 18)

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"


def read_file(name):
    try:
        return (ROOT / name).read_text()
    except FileNotFoundError:
        return ""


def md_to_html(text):
    """Convert markdown to HTML with identity structure."""
    lines = text.split("\n")
    content_html = ""
    rules_html = ""
    in_rules = False

    mission_found = False

    for i, line in enumerate(lines):
        line = line.strip()

        # Skip empty lines
        if not line:
            continue

        # Detect section headers
        if line.startswith("## "):
            section = line[3:].strip()
            if section.lower() == "my rules":
                in_rules = True
            continue

        if line.startswith("# "):
            # Skip main header
            continue

        # Process content
        if not in_rules:
            # Process mission/identity paragraphs
            escaped = html.escape(line)
            # Inline formatting
            escaped = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", escaped)
            escaped = re.sub(r"`(.+?)`", r"<code>\1</code>", escaped)
            escaped = re.sub(
                r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', escaped
            )

            if not mission_found:
                content_html += f'<p class="mission">{escaped}</p>\n'
                mission_found = True
            else:
                content_html += f'<p class="identity-text">{escaped}</p>\n'
        else:
            # Process rules (numbered list items)
            match = re.match(r"^(\d+)\.\s(.+)$", line)
            if match:
                rule_text = html.escape(match.group(2))
                # Inline formatting
                rule_text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", rule_text)
                rule_text = re.sub(r"`(.+?)`", r"<code>\1</code>", rule_text)
                rule_text = re.sub(
                    r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', rule_text
                )

                if not rules_html:
                    rules_html = '<ol class="rules">\n'
                rules_html += f"    <li>{rule_text}</li>\n"

    if rules_html:
        rules_html += "  </ol>"

    return content_html + rules_html


def ordinal(n):
    if 11 <= (n % 100) <= 13:
        return f"{n}th"
    return f"{n}{['th', 'st', 'nd', 'rd', 'th'][min(n % 10, 4)]}"


def format_timestamp(ts, day=None):
    """Format 'HH:MM' + day number to '19th March 2026, 08:56 UTC / 14:26 IST'."""
    try:
        dt = datetime.strptime(ts, "%H:%M")
        if day is not None:
            date = BIRTH_DATE + timedelta(days=day)
            ist = dt + timedelta(hours=5, minutes=30)
            return f"{ordinal(date.day)} {date.strftime('%B %Y')}, {dt.strftime('%H:%M')} UTC / {ist.strftime('%H:%M')} IST"
        utc_str = dt.strftime("%H:%M")
        ist = dt + timedelta(hours=5, minutes=30)
        return f"{utc_str} UTC / {ist.strftime('%H:%M')} IST"
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
        # Format: "Day N — HH:MM — Title"
        m = re.match(
            r"Day\s+(\d+)\s*[—–\-]+\s*(\d{2}:\d{2})\s*[—–\-]+\s*(.+)", lines[0]
        )
        if m:
            day = int(m.group(1))
            timestamp = m.group(2).strip()
            title = m.group(3).strip()
            body = "\n".join(lines[1:]).strip()
            entries.append(
                {"day": day, "timestamp": timestamp, "title": title, "body": body}
            )
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
        ts = entry.get("timestamp", "")
        ts_fmt = format_timestamp(ts, entry["day"]) if ts else ""
        ts_html = (
            f'      <span class="entry-timestamp">{html.escape(ts_fmt)}</span>\n'
            if ts_fmt
            else ""
        )
        parts.append(
            f'  <article class="entry">\n'
            f'    <div class="entry-marker"></div>\n'
            f'    <div class="entry-content">\n'
            f'      <span class="entry-day">Day {entry["day"]}</span>\n'
            f"{ts_html}"
            f'      <h3 class="entry-title">{md_inline(entry["title"])}</h3>\n'
            f'      <p class="entry-body">{body_html}</p>\n'
            f"    </div>\n"
            f"  </article>"
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
    mission = ""
    if identity:
        lines = identity.strip().split("\n")
        for line in lines:
            line = line.strip()
            if line and not line.startswith("#") and not line.startswith("-"):
                mission = md_inline(line)
                break

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
      <p class="mission">{mission}</p>
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
