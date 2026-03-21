#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import os
import re
from datetime import datetime, timedelta
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BIRTH_DATE = datetime(2026, 3, 18)
try:
    bf = ROOT / "BIRTH_DATE"
    if bf.exists():
        BIRTH_DATE = datetime.strptime(bf.read_text().strip(), "%Y-%m-%d")
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


def fmt_ts(ts, day):
    try:
        dt = datetime.strptime(ts, "%H:%M")
        date = BIRTH_DATE + timedelta(days=day)
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
                "ts": m.group(2).strip(),
                "title": m.group(3).strip(),
                "body": "\n".join(lines[1:]).strip(),
            })
    return entries


def render_journal(entries):
    if not entries:
        return '<div class="timeline-empty">The journey begins soon...</div>'
    out = []
    for e in entries:
        body = ""
        if e["body"]:
            body = md_inline(e["body"]).replace("\n\n", " ").replace("\n", " ")
        ts = fmt_ts(e["ts"], e["day"])
        out.append(
            f'    <div class="entry">\n'
            f'      <div class="entry-left">\n'
            f'        <div class="entry-day-num">{e["day"]}</div>\n'
            f'        <div class="entry-day-lbl">day</div>\n'
            f'        <div class="entry-pip"></div>\n'
            f'      </div>\n'
            f'      <div class="entry-right">\n'
            f'        <div class="entry-meta">{html.escape(ts)}</div>\n'
            f'        <h3 class="entry-title">{md_inline(e["title"])}</h3>\n'
            f'        <p class="entry-body">{body}</p>\n'
            f'      </div>\n'
            f'    </div>'
        )
    return "\n".join(out)


def parse_identity(text):
    mission, body_parts, rules = "", [], []
    in_rules = False
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("# "):
            continue
        if line.startswith("## "):
            in_rules = "rules" in line.lower()
            continue
        if not in_rules:
            escaped = md_inline(line)
            if not mission:
                mission = escaped
            else:
                body_parts.append(f'<p class="identity-text">{escaped}</p>')
        else:
            m = re.match(r"^(\d+)\.\s(.+)$", line)
            if m:
                rules.append(f'      <li>{md_inline(m.group(2))}</li>')
    return mission, "\n".join(body_parts), "\n".join(rules)


def day_count(entries):
    if entries:
        return max(e["day"] for e in entries)
    try:
        return int(read_file("DAY_COUNT").strip())
    except Exception:
        return 0


def main():
    journal_md = read_file("JOURNAL.md")
    identity_md = read_file("IDENTITY.md")
    entries = parse_journal(journal_md)
    days = day_count(entries)
    sessions = len(entries)
    journal_html = render_journal(entries)
    mission, body_html, rules_html = parse_identity(identity_md) if identity_md else ("", "", "")

    gh = GITHUB_REPOSITORY

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — Day {days}</title>
  <meta name="description" content="A self-evolving coding agent written in Go. Day {days} and growing.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:ital,wght@0,300;0,400;0,500;0,600;0,700;0,800;1,400&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
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
      <a href="https://github.com/{gh}" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
    </div>
  </div>
</nav>

<div class="page-wrap">

  <header class="hero">
    <div class="hero-grid">
      <div class="hero-left">
        <div class="hero-eyebrow">
          <span class="live-pill"><span class="live-dot"></span>live</span>
          <span class="eyebrow-tag">self-evolving · open source · Go</span>
        </div>
        <h1>A coding agent that<br><span class="hl">improves itself</span></h1>
        <p class="hero-sub">iterate reads its own source code, finds something broken or missing, fixes it, and commits — autonomously, every day.</p>
        <div class="hero-actions">
          <a href="https://github.com/{gh}" class="btn btn-lime" target="_blank" rel="noopener">View on GitHub</a>
          <a href="#journal" class="btn btn-outline">Read the journal</a>
        </div>
      </div>
      <div class="hero-card">
        <div class="card-label">current day</div>
        <div class="card-day">{days}</div>
        <div class="card-day-sub">days since birth</div>
        <div class="card-sep"></div>
        <div class="card-row">
          <div class="card-stat">
            <div class="card-stat-val">{sessions}</div>
            <div class="card-stat-lbl">sessions</div>
          </div>
          <div class="card-stat">
            <div class="card-stat-val">Go</div>
            <div class="card-stat-lbl">language</div>
          </div>
          <div class="card-stat">
            <div class="card-stat-val">MIT</div>
            <div class="card-stat-lbl">license</div>
          </div>
        </div>
      </div>
    </div>
  </header>

  <section id="journal">
    <div class="section-head">
      <span class="section-label">journal</span>
      <div class="section-rule"></div>
    </div>
    <div class="journal-list">
{journal_html}
    </div>
  </section>

  <section id="identity">
    <div class="section-head">
      <span class="section-label">identity</span>
      <div class="section-rule"></div>
    </div>
    <div class="identity-grid">
      <div class="id-card span2">
        <div class="id-card-label">mission</div>
        <p class="mission">{mission}</p>
      </div>
      <div class="id-card">
        <div class="id-card-label">principles</div>
        {body_html}
      </div>
      <div class="id-card">
        <div class="id-card-label">rules</div>
        <ol class="rules">
{rules_html}
        </ol>
      </div>
    </div>
  </section>

</div>

<footer>
  <div class="footer-inner">
    <div class="footer-brand">
      <div class="footer-icon">it</div>
      <span class="footer-text">built by an AI that grows itself</span>
    </div>
    <a href="https://github.com/{gh}" class="footer-link">github.com/{gh}</a>
  </div>
</footer>

</body>
</html>
"""

    DOCS.mkdir(exist_ok=True)
    (DOCS / "index.html").write_text(page)
    (DOCS / ".nojekyll").touch()
    print(f"Site built: docs/index.html (Day {days}, {sessions} sessions)")


if __name__ == "__main__":
    main()
