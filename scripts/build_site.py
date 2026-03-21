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
        return '<div class="j-empty">The journey begins soon...</div>'
    out = []
    for e in entries:
        body = ""
        if e["body"]:
            body = md_inline(e["body"]).replace("\n\n", " ").replace("\n", " ")
        ts = fmt_ts(e["ts"], e["day"])
        out.append(
            f'    <div class="entry">\n'
            f'      <div class="e-left">\n'
            f'        <div class="e-day">{e["day"]}</div>\n'
            f'        <div class="e-lbl">day</div>\n'
            f'        <div class="e-line"></div>\n'
            f'      </div>\n'
            f'      <div class="e-right">\n'
            f'        <div class="e-meta"><span class="e-dot"></span>{html.escape(ts)}</div>\n'
            f'        <div class="e-title">{md_inline(e["title"])}</div>\n'
            f'        <div class="e-body">{body}</div>\n'
            f'      </div>\n'
            f'    </div>'
        )
    return "\n".join(out)


def parse_identity(text):
    """Returns (mission_html, principles_html, rules_html)."""
    mission = ""
    principles = []
    rules = []
    section = "intro"

    for line in text.split("\n"):
        stripped = line.strip()
        if not stripped:
            continue
        if stripped.startswith("# "):
            continue
        if stripped.startswith("## "):
            heading = stripped[3:].lower()
            if "rule" in heading:
                section = "rules"
            elif "have" in heading or "start" in heading or "going" in heading or "source" in heading:
                section = "skip"
            else:
                section = "principles"
            continue

        if section == "skip":
            continue

        if section == "intro":
            escaped = md_inline(stripped)
            if not mission:
                mission = escaped
            else:
                principles.append(f'<p class="id-text">{escaped}</p>')

        elif section == "principles":
            escaped = md_inline(stripped)
            principles.append(f'<p class="id-text">{escaped}</p>')

        elif section == "rules":
            m = re.match(r"^(\d+)\.\s(.+)$", stripped)
            if m:
                rules.append(f'<li>{md_inline(m.group(2))}</li>')

    return mission, "\n".join(principles), "\n".join(rules)


def day_count(entries):
    if entries:
        return max(e["day"] for e in entries)
    try:
        return int(read_file("DAY_COUNT").strip())
    except Exception:
        return 0


BENTO_CELLS = [
    {
        "icon": "&#x27F3;",  # ↻
        "title": "Self-evolving",
        "body": "Every 12 hours, iterate reads its own source code, finds something to improve, and commits the fix — no human in the loop.",
        "extra": (
            '<div class="bento-code">'
            '<span class="cm">// every session</span>\n'
            '<span class="kw">func</span> <span class="fn">evolve</span>() {\n'
            '  read → fix → test → commit\n'
            '}'
            '</div>'
        ),
        "wide": True,
    },
    {
        "icon": "&#x1F4D3;",  # 📓
        "title": "Transparent journal",
        "body": "Every session is logged — successes, failures, and reversions. Nothing is hidden.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x2713;",  # ✓
        "title": "Tests first",
        "body": "If <code>go build</code> and <code>go test</code> don't pass, the code doesn't ship. No exceptions.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x1F465;",  # 👥
        "title": "Community-driven",
        "body": "Real GitHub issues shape the roadmap. Community feedback beats internal intuition.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x1F9E0;",  # 🧠
        "title": "Memory that compounds",
        "body": "Learnings persist across sessions. Patterns in failures matter more than cleanup.",
        "extra": "",
        "wide": False,
    },
]


def render_bento():
    cells = []
    for cell in BENTO_CELLS:
        wide_class = " wide" if cell["wide"] else ""
        cells.append(
            f'    <div class="bento-cell{wide_class}">\n'
            f'      <div class="bento-icon">{cell["icon"]}</div>\n'
            f'      <div class="bento-title">{cell["title"]}</div>\n'
            f'      <div class="bento-body">{cell["body"]}</div>\n'
            f'      {cell["extra"]}\n'
            f'    </div>'
        )
    return "\n".join(cells)


def main():
    journal_md = read_file("JOURNAL.md")
    identity_md = read_file("IDENTITY.md")
    entries = parse_journal(journal_md)
    days = day_count(entries)
    sessions = len(entries)
    journal_html = render_journal(entries)
    mission, principles_html, rules_html = parse_identity(identity_md) if identity_md else ("", "", "")
    bento_html = render_bento()

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
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800;900&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="style.css">
</head>
<body>

<nav>
  <div class="nav-inner">
    <a href="#" class="nav-brand">
      <div class="nav-logo">it</div>
      <span class="nav-name">iterate</span>
    </a>
    <div class="nav-links">
      <a href="#features">Features</a>
      <a href="#journal">Journal</a>
      <a href="#identity">Identity</a>
      <a href="https://github.com/{gh}" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
    </div>
  </div>
</nav>

<section class="hero">
  <div class="orb orb1"></div>
  <div class="orb orb2"></div>
  <div class="orb orb3"></div>
  <div class="hero-grid-bg"></div>
  <div class="wrap">
    <div class="hero-inner">
      <div>
        <div class="eyebrow">
          <span class="pill-live"><span class="live-dot"></span>live</span>
          <span class="eyebrow-sub">self-evolving · open source · Go</span>
        </div>
        <h1>A coding agent that<br><span class="grad-text">improves itself</span></h1>
        <p class="hero-sub">iterate reads its own source code, finds something broken or missing, fixes it, and commits — autonomously, every day.</p>
        <div class="btns">
          <a href="https://github.com/{gh}" class="btn btn-lime" target="_blank" rel="noopener">View on GitHub</a>
          <a href="#journal" class="btn btn-ghost">Read the journal</a>
        </div>
      </div>
      <div class="hero-card">
        <div class="card-eyebrow">current day</div>
        <div class="card-num">{days}</div>
        <div class="card-sub">days since birth</div>
        <div class="card-sep"></div>
        <div class="card-stats">
          <div>
            <div class="cs-val">{sessions}</div>
            <div class="cs-lbl">sessions</div>
          </div>
          <div>
            <div class="cs-val">Go</div>
            <div class="cs-lbl">language</div>
          </div>
          <div>
            <div class="cs-val">MIT</div>
            <div class="cs-lbl">license</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>

<section id="features" class="features">
  <div class="wrap">
    <div class="sec-head">
      <span class="sec-label">features</span>
      <div class="sec-rule"></div>
    </div>
    <div class="bento">
{bento_html}
    </div>
  </div>
</section>

<section id="journal" class="journal">
  <div class="wrap">
    <div class="sec-head">
      <span class="sec-label">journal</span>
      <div class="sec-rule"></div>
    </div>
    <div class="j-list">
{journal_html}
    </div>
  </div>
</section>

<section id="identity" class="identity">
  <div class="wrap">
    <div class="sec-head">
      <span class="sec-label">identity</span>
      <div class="sec-rule"></div>
    </div>
    <div class="id-grid">
      <div class="id-card full">
        <div class="id-tag">mission</div>
        <p class="mission">{mission}</p>
      </div>
      <div class="id-card">
        <div class="id-tag">principles</div>
        {principles_html}
      </div>
      <div class="id-card">
        <div class="id-tag">rules</div>
        <ol class="rules">
          {rules_html}
        </ol>
      </div>
    </div>
  </div>
</section>

<footer>
  <div class="foot-inner">
    <div class="foot-brand">
      <div class="foot-icon">it</div>
      <span class="foot-txt">built by an AI that grows itself</span>
    </div>
    <a href="https://github.com/{gh}" class="foot-link">github.com/{gh}</a>
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
