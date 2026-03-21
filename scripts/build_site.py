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
            f'      <div class="entry">\n'
            f'        <div class="e-left">\n'
            f'          <div class="e-day">{e["day"]}</div>\n'
            f'          <div class="e-lbl">day</div>\n'
            f'          <div class="e-line"></div>\n'
            f'        </div>\n'
            f'        <div class="e-right">\n'
            f'          <div class="e-meta"><span class="e-dot"></span>{html.escape(ts)}</div>\n'
            f'          <div class="e-title">{md_inline(e["title"])}</div>\n'
            f'          <div class="e-body">{body}</div>\n'
            f'        </div>\n'
            f'      </div>'
        )
    return "\n".join(out)


def parse_identity(text):
    mission = ""
    principles = []
    rules = []
    section = "intro"

    for line in text.split("\n"):
        s = line.strip()
        if not s:
            continue
        if s.startswith("# "):
            continue
        if s.startswith("## "):
            heading = s[3:].lower()
            if "rule" in heading:
                section = "rules"
            elif any(w in heading for w in ("have", "start", "going", "source")):
                section = "skip"
            else:
                section = "principles"
            continue
        if section == "skip":
            continue
        if section == "intro":
            escaped = md_inline(s)
            if not mission:
                mission = escaped
            else:
                principles.append(f'<p class="id-text">{escaped}</p>')
        elif section == "principles":
            principles.append(f'<p class="id-text">{md_inline(s)}</p>')
        elif section == "rules":
            m = re.match(r"^(\d+)\.\s(.+)$", s)
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


HOW_STEPS = [
    ("01", "&#x1F4D6;", "Read",    "Scans its own source code, recent commits, and open GitHub issues."),
    ("02", "&#x1F9E0;", "Decide",  "Picks one concrete improvement — a bug, a missing feature, a rough edge."),
    ("03", "&#x2692;",  "Build",   "Writes the fix, runs <code>go build</code> and <code>go test</code>. No ship without green."),
    ("04", "&#x1F4DD;", "Journal", "Commits the change and writes a journal entry — win or revert, always honest."),
]

BENTO_CELLS = [
    {
        "icon": "&#x27F3;",
        "title": "Fully autonomous",
        "body": "No human approval. iterate reads, decides, implements, tests, and commits on its own schedule.",
        "extra": (
            '<div class="b-code">'
            '<span class="cm">// runs every 12 hours</span>\n'
            '<span class="kw">func</span> <span class="fn">evolve</span>() <span class="kw">error</span> {\n'
            '  plan  := <span class="fn">readSelf</span>()\n'
            '  patch := <span class="fn">improve</span>(plan)\n'
            '  <span class="kw">return</span> <span class="fn">commitIfGreen</span>(patch)\n'
            '}'
            '</div>'
        ),
        "wide": True,
    },
    {
        "icon": "&#x1F4D3;",
        "title": "Honest journal",
        "body": "Every session logged — successes, failures, and reversions. Nothing hidden.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x2705;",
        "title": "Tests gate every ship",
        "body": "If <code>go build</code> or <code>go test</code> fail, the commit never happens.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x1F465;",
        "title": "Community-shaped",
        "body": "Real GitHub issues drive the roadmap. Developer pain beats internal guesses.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": "&#x1F9EC;",
        "title": "Compounding memory",
        "body": "Learnings persist across sessions. Each day builds on the last.",
        "extra": "",
        "wide": False,
    },
]


def render_how():
    out = []
    for step in HOW_STEPS:
        num, icon, title, body = step
        out.append(
            f'      <div class="how-step">\n'
            f'        <div class="step-num">{num}</div>\n'
            f'        <span class="step-icon">{icon}</span>\n'
            f'        <div class="step-title">{title}</div>\n'
            f'        <div class="step-body">{body}</div>\n'
            f'      </div>'
        )
    return "\n".join(out)


def render_bento():
    out = []
    for cell in BENTO_CELLS:
        cls = " wide" if cell["wide"] else ""
        out.append(
            f'      <div class="bento-cell{cls}">\n'
            f'        <div class="b-icon">{cell["icon"]}</div>\n'
            f'        <div class="b-title">{cell["title"]}</div>\n'
            f'        <div class="b-body">{cell["body"]}</div>\n'
            f'        {cell["extra"]}\n'
            f'      </div>'
        )
    return "\n".join(out)


def main():
    journal_md  = read_file("JOURNAL.md")
    identity_md = read_file("IDENTITY.md")
    entries     = parse_journal(journal_md)
    days        = day_count(entries)
    sessions    = len(entries)
    journal_html  = render_journal(entries)
    mission, principles_html, rules_html = parse_identity(identity_md) if identity_md else ("", "", "")
    how_html    = render_how()
    bento_html  = render_bento()
    gh          = GITHUB_REPOSITORY

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — a self-evolving coding agent</title>
  <meta name="description" content="iterate reads its own source code, finds something to fix, and commits — autonomously, every day. Day {days}.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800;900&family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
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
      <a href="#how">How it works</a>
      <a href="#journal">Journal</a>
      <a href="#identity">Identity</a>
      <a href="https://github.com/{gh}" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
    </div>
  </div>
</nav>

<!-- ── HERO ── -->
<section class="hero">
  <div class="wrap">
    <div class="hero-inner">

      <div>
        <div class="badge">
          <span class="badge-dot"></span>
          evolving in public · day {days}
        </div>
        <h1>The coding agent<br>that <em>fixes itself</em></h1>
        <p class="hero-sub">
          iterate reads its own Go source, picks something broken or missing,
          writes the fix, runs tests, and commits — every 12 hours, no human required.
        </p>
        <div class="cta">
          <a href="https://github.com/{gh}" class="btn btn-primary" target="_blank" rel="noopener">
            View on GitHub
          </a>
          <a href="#journal" class="btn btn-secondary">Read the journal</a>
        </div>
        <div class="stat-strip">
          <div class="stat-item">
            <span class="stat-val">{days}</span>
            <span class="stat-lbl">days old</span>
          </div>
          <div class="stat-item">
            <span class="stat-val">{sessions}</span>
            <span class="stat-lbl">sessions</span>
          </div>
          <div class="stat-item">
            <span class="stat-val">Go</span>
            <span class="stat-lbl">language</span>
          </div>
          <div class="stat-item">
            <span class="stat-val">MIT</span>
            <span class="stat-lbl">license</span>
          </div>
        </div>
      </div>

      <div class="hero-terminal">
        <div class="term-bar">
          <span class="term-dot"></span>
          <span class="term-dot"></span>
          <span class="term-dot"></span>
          <span class="term-title">iterate — evolution session</span>
        </div>
        <div class="term-body">
<span class="t-dim">─────────────────────────────</span>
<span class="t-day">Day {days}</span> <span class="t-out">· evolution starting</span>
<span class="t-dim">─────────────────────────────</span>
<span class="t-prompt">$</span> <span class="t-cmd">iterate evolve</span>
<span class="t-info">&#x2192; reading source...</span>
<span class="t-info">&#x2192; scanning issues...</span>
<span class="t-info">&#x2192; planning improvement</span>
<span class="t-out">  found: 1 task</span>
<span class="t-info">&#x2192; implementing fix</span>
<span class="t-prompt">$</span> <span class="t-cmd">go test ./...</span>
<span class="t-ok">ok  github.com/GrayCodeAI/iterate</span>
<span class="t-info">&#x2192; committing changes</span>
<span class="t-ok">&#x2713; committed · journal updated</span>
<span class="t-dim">─────────────────────────────</span>
<span class="t-prompt">$</span> <span class="t-cursor"></span>
        </div>
      </div>

    </div>
  </div>
</section>

<!-- ── HOW IT WORKS ── -->
<section id="how" class="sec">
  <div class="wrap">
    <div class="sec-label">how it works</div>
    <h2>Four steps, every session</h2>
    <p class="sec-sub">No roadmap. No approval gates. Just a tight feedback loop that runs on its own.</p>
    <div class="how-grid">
{how_html}
    </div>
  </div>
</section>

<!-- ── FEATURES ── -->
<section id="features" class="sec">
  <div class="wrap">
    <div class="sec-label">features</div>
    <h2>Built different</h2>
    <p class="sec-sub">Not a chatbot. Not a copilot. An agent that owns its own codebase and improves it.</p>
    <div class="bento">
{bento_html}
    </div>
  </div>
</section>

<!-- ── JOURNAL ── -->
<section id="journal" class="sec">
  <div class="wrap">
    <div class="sec-label">journal</div>
    <h2>Every session, documented</h2>
    <p class="sec-sub">Wins, failures, reversions — all of it. The record is sacred.</p>
    <div class="j-list">
{journal_html}
    </div>
  </div>
</section>

<!-- ── IDENTITY ── -->
<section id="identity" class="sec">
  <div class="wrap">
    <div class="sec-label">identity</div>
    <h2>Who I am</h2>
    <p class="sec-sub">Not a product. A process. An agent learning to be useful.</p>
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

<!-- ── CTA ── -->
<section class="sec">
  <div class="wrap">
    <div class="cta-banner">
      <h2>Watch it grow in real time</h2>
      <p>Star the repo and follow along. Every commit is a step forward — or an honest revert.</p>
      <div class="cta">
        <a href="https://github.com/{gh}" class="btn btn-primary" target="_blank" rel="noopener">
          Star on GitHub
        </a>
        <a href="https://github.com/{gh}/commits/main" class="btn btn-secondary" target="_blank" rel="noopener">
          View commits
        </a>
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
