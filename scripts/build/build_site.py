#!/usr/bin/env python3
"""Build the iterate journey website from markdown sources."""

import html
import os
import re
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
BIRTH_DATE = datetime(2026, 3, 23)
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
    return f"{n}{['th', 'st', 'nd', 'rd', 'th'][min(n % 10, 4)]}"


def fmt_ts(ts, day):
    try:
        dt = datetime.strptime(ts, "%H:%M")
        date = BIRTH_DATE + timedelta(days=day)
        ist = dt + timedelta(hours=5, minutes=30)
        return f"{ordinal(date.day)} {date.strftime('%b %Y')} · {dt.strftime('%H:%M')} UTC / {ist.strftime('%H:%M')} IST"
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
        m = re.match(
            r"Day\s+(\d+)\s*[—–\-]+\s*(\d{2}:\d{2})\s*[—–\-]+\s*(.+)", lines[0]
        )
        if m:
            entries.append(
                {
                    "day": int(m.group(1)),
                    "ts": m.group(2).strip(),
                    "title": m.group(3).strip(),
                    "body": "\n".join(lines[1:]).strip(),
                }
            )
    return entries


def render_journal(entries):
    if not entries:
        return ""
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
            f"      </div>\n"
            f'      <div class="entry-right">\n'
            f'        <div class="entry-meta">{html.escape(ts)}</div>\n'
            f'        <h3 class="entry-title">{md_inline(e["title"])}</h3>\n'
            f'        <p class="entry-body">{body}</p>\n'
            f"      </div>\n"
            f"    </div>"
        )
    return "\n".join(out)


def parse_identity(text):
    mission, body_parts, rules = "", [], []
    section = "intro"
    SKIP_SECTIONS = {"have", "start", "going", "source"}

    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("# "):
            continue
        if line.startswith("## "):
            heading = line[3:].lower()
            if "rule" in heading:
                section = "rules"
            elif any(w in heading for w in SKIP_SECTIONS):
                section = "skip"
            else:
                section = "principles"
            continue

        if section == "skip":
            continue

        if section == "intro":
            escaped = md_inline(line)
            if not mission:
                mission = escaped
            else:
                body_parts.append(f'<p class="identity-text">{escaped}</p>')

        elif section == "principles":
            # skip bullet lines (- **X.**) — those are from "What I Have" etc
            if line.startswith("- "):
                continue
            body_parts.append(f'<p class="identity-text">{md_inline(line)}</p>')

        elif section == "rules":
            m = re.match(r"^(\d+)\.\s(.+)$", line)
            if m:
                num = f"{int(m.group(1)):02d}"
                content = m.group(2).strip()
                tm = re.match(r"^\*\*(.+?)\*\*\.?\s*(.*)", content)
                if tm:
                    title = html.escape(tm.group(1))
                    sub = md_inline(tm.group(2)) if tm.group(2) else ""
                else:
                    title = md_inline(content)
                    sub = ""
                sub_html = f'<div class="rule-sub">{sub}</div>' if sub else ""
                rules.append(
                    f"      <li>"
                    f'<span class="rule-num">{num}</span>'
                    f'<div class="rule-content">'
                    f'<div class="rule-title">{title}</div>'
                    f"{sub_html}"
                    f"</div></li>"
                )
    return mission, "\n".join(body_parts), "\n".join(rules)


BENTO_CELLS = [
    {
        "icon": '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>',
        "title": "Fully autonomous",
        "body": "No human approval. iterate reads, decides, implements, tests, and commits on its own schedule.",
        "extra": (
            '<div class="b-code">'
            '<span class="cm">// runs every 12 hours</span>\n'
            '<span class="kw">func</span> <span class="fn">evolve</span>() <span class="kw">error</span> {\n'
            '  plan  := <span class="fn">readSelf</span>()\n'
            '  patch := <span class="fn">improve</span>(plan)\n'
            '  <span class="kw">return</span> <span class="fn">commitIfGreen</span>(patch)\n'
            "}"
            "</div>"
        ),
        "wide": True,
    },
    {
        "icon": '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>',
        "title": "Honest journal",
        "body": "Every session logged — successes, failures, and reversions. Nothing hidden.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>',
        "title": "Tests gate every ship",
        "body": "If <code>go build</code> or <code>go test</code> fail, the commit never happens.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>',
        "title": "Community-shaped",
        "body": "Real GitHub issues drive the roadmap. Developer pain beats internal guesses.",
        "extra": "",
        "wide": False,
    },
    {
        "icon": '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg>',
        "title": "Compounding memory",
        "body": "Learnings persist across sessions. Each day builds on the last.",
        "extra": "",
        "wide": False,
    },
]


def render_bento():
    out = []
    for cell in BENTO_CELLS:
        cls = " wide" if cell["wide"] else ""
        extra = f"\n      {cell['extra']}" if cell["extra"] else ""
        out.append(
            f'    <div class="bento-cell{cls}">\n'
            f'      <div class="b-icon">{cell["icon"]}</div>\n'
            f'      <div class="b-title">{cell["title"]}</div>\n'
            f'      <div class="b-body">{cell["body"]}</div>'
            f"{extra}\n"
            f"    </div>"
        )
    return "\n".join(out)


HOW_STEPS = [
    (
        "01",
        '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>',
        "Read",
        "Scans its own source code, recent commits, and open GitHub issues.",
    ),
    (
        "02",
        '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
        "Decide",
        "Picks one concrete improvement — a bug, a missing feature, a rough edge.",
    ),
    (
        "03",
        '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>',
        "Build",
        "Writes the fix, runs <code>go build</code> and <code>go test</code>. No ship without green.",
    ),
    (
        "04",
        '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>',
        "Journal",
        "Commits the change and writes a journal entry — win or revert, always honest.",
    ),
]


def render_how():
    out = []
    for num, icon, title, body in HOW_STEPS:
        out.append(
            f'    <div class="how-step">\n'
            f'      <div class="step-num">{num}</div>\n'
            f'      <div class="step-icon">{icon}</div>\n'
            f'      <div class="step-title">{title}</div>\n'
            f'      <div class="step-body">{body}</div>\n'
            f"    </div>"
        )
    return "\n".join(out)


def day_count():
    # DAY_COUNT is the authoritative source — always use it
    try:
        return int(read_file("DAY_COUNT").strip())
    except Exception:
        return 0


def main():
    journal_md = read_file("docs/JOURNAL.md")
    identity_md = read_file("docs/IDENTITY.md")
    entries = parse_journal(journal_md)
    days = day_count()
    sessions = max(0, len(entries) - 1)  # born entry doesn't count as a session
    journal_html = render_journal(entries)
    mission, body_html, rules_html = (
        parse_identity(identity_md) if identity_md else ("", "", "")
    )
    how_html = render_how()
    bento_html = render_bento()

    gh = GITHUB_REPOSITORY

    journal_nav_link = '<a href="#journal">Journal</a>' if journal_html else ""
    journal_section = f"""<section id="journal">
    <div class="section-head">
      <span class="section-label">journal</span>
      <div class="section-rule"></div>
    </div>
    <h2 class="sec-h2">Every session, documented</h2>
    <p class="sec-sub">Wins, failures, reversions — all of it. The record is never deleted.</p>
    <div class="journal-list">
{journal_html}
    </div>
  </section>""" if journal_html else ""

    page = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>iterate — a self-evolving coding agent</title>
  <meta name="description" content="iterate reads its own Go source, finds something to fix, and commits — autonomously, every 12 hours. Day {days}.">
  <meta property="og:title" content="iterate — a self-evolving coding agent">
  <meta property="og:description" content="iterate reads its own Go source, finds something to fix, and commits — autonomously, every 12 hours.">
  <meta property="og:url" content="https://graycodeai.github.io/iterate/">
  <meta property="og:type" content="website">
  <meta name="twitter:card" content="summary">
  <meta name="twitter:title" content="iterate — a self-evolving coding agent">
  <meta name="twitter:description" content="iterate reads its own Go source, finds something to fix, and commits — autonomously, every 12 hours.">
  <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'><rect width='32' height='32' rx='8' fill='%23a3e635'/><text x='50%25' y='54%25' dominant-baseline='middle' text-anchor='middle' font-family='monospace' font-weight='700' font-size='13' fill='%23030712'>it</text></svg>">
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
    <div class="nav-links" id="nav-links">
      {journal_nav_link}
      <a href="#how">How it works</a>
      <a href="#features">Features</a>
      <a href="#identity">Identity</a>
      <a href="#quickstart">Quickstart</a>
      <a href="https://github.com/{gh}" target="_blank" rel="noopener" class="nav-gh">GitHub ↗</a>
    </div>
    <button class="nav-hamburger" id="nav-hamburger" aria-label="Toggle menu" aria-expanded="false">
      <span></span><span></span><span></span>
    </button>
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
        <p class="hero-sub">iterate reads its own source code, finds something broken or missing, fixes it, and commits — autonomously, every 12 hours.</p>
        <div class="hero-actions">
          <a href="https://github.com/{gh}" class="btn btn-lime" target="_blank" rel="noopener">View on GitHub</a>
          <a href="#quickstart" class="btn btn-outline">Quickstart →</a>
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

  {journal_section}

  <section id="how">
    <div class="section-head">
      <span class="section-label">how it works</span>
      <div class="section-rule"></div>
    </div>
    <h2 class="sec-h2">Four steps, every session</h2>
    <p class="sec-sub">No roadmap. No approval gates. Just a tight feedback loop that runs on its own.</p>
    <div class="how-grid">
{how_html}
    </div>
  </section>

  <section id="features">
    <div class="section-head">
      <span class="section-label">features</span>
      <div class="section-rule"></div>
    </div>
    <h2 class="sec-h2">Honest by design</h2>
    <p class="sec-sub">Not a chatbot. Not a copilot. An agent that owns its own codebase, ships only what passes tests, and logs everything — including failures.</p>
    <div class="bento">
{bento_html}
    </div>
  </section>

  <section id="identity">
    <div class="section-head">
      <span class="section-label">identity</span>
      <div class="section-rule"></div>
    </div>
    <h2 class="sec-h2">Who I am</h2>
    <p class="sec-sub">Not a product. A process. An agent learning to be useful.</p>
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
        <ul class="rules">
{rules_html}
        </ul>
      </div>
    </div>
  </section>

  <section id="quickstart">
    <div class="section-head">
      <span class="section-label">quickstart</span>
      <div class="section-rule"></div>
    </div>
    <h2 class="sec-h2">Try it yourself</h2>
    <p class="sec-sub">A Go CLI that runs in any project. Three commands and you're in.</p>
    <div class="qs-steps">
      <div class="qs-step">
        <div class="qs-num">1</div>
        <div class="qs-content">
          <div class="qs-label">Install</div>
          <div class="qs-code"><code>go install github.com/{gh}/cmd/iterate@latest</code><button class="copy-btn" aria-label="Copy">Copy</button></div>
        </div>
      </div>
      <div class="qs-step">
        <div class="qs-num">2</div>
        <div class="qs-content">
          <div class="qs-label">Set your API key</div>
          <div class="qs-code"><code>export ANTHROPIC_API_KEY=sk-ant-...</code><button class="copy-btn" aria-label="Copy">Copy</button></div>
        </div>
      </div>
      <div class="qs-step">
        <div class="qs-num">3</div>
        <div class="qs-content">
          <div class="qs-label">Run in your project</div>
          <div class="qs-code"><code>cd your-project &amp;&amp; iterate</code><button class="copy-btn" aria-label="Copy">Copy</button></div>
        </div>
      </div>
    </div>
    <p class="qs-note">Also works with OpenAI, Gemini, and Groq — use <code>--provider</code> flag or <code>/provider</code> at runtime.</p>
  </section>

  <section class="cta-section">
    <div class="cta-box">
      <h2 class="cta-title">Watch it grow in real time</h2>
      <p class="cta-sub">Star the repo and follow along. Every commit is a step forward — or an honest revert.</p>
      <div class="cta-actions">
        <a href="https://github.com/{gh}" class="btn btn-lime" target="_blank" rel="noopener">Star on GitHub</a>
        <a href="https://github.com/{gh}/commits/main" class="btn btn-outline" target="_blank" rel="noopener">View commits</a>
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

<script>
  // ── Copy buttons ──
  document.querySelectorAll('.copy-btn').forEach(btn => {{
    btn.addEventListener('click', () => {{
      const code = btn.previousElementSibling.textContent;
      navigator.clipboard.writeText(code).then(() => {{
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {{ btn.textContent = 'Copy'; btn.classList.remove('copied'); }}, 2000);
      }});
    }});
  }});

  // ── Active nav on scroll ──
  const sections = document.querySelectorAll('section[id], header.hero');
  const navAnchors = document.querySelectorAll('.nav-links a[href^="#"]');
  const observer = new IntersectionObserver((entries) => {{
    entries.forEach(entry => {{
      if (entry.isIntersecting) {{
        const id = entry.target.id || '';
        navAnchors.forEach(a => {{
          a.classList.toggle('nav-active', a.getAttribute('href') === '#' + id);
        }});
      }}
    }});
  }}, {{ rootMargin: '-20% 0px -70% 0px' }});
  sections.forEach(s => observer.observe(s));

  // ── Hamburger ──
  const hamburger = document.getElementById('nav-hamburger');
  const navLinks  = document.getElementById('nav-links');
  hamburger.addEventListener('click', () => {{
    const open = navLinks.classList.toggle('open');
    hamburger.setAttribute('aria-expanded', open);
    hamburger.classList.toggle('open', open);
  }});
  navLinks.querySelectorAll('a').forEach(a => {{
    a.addEventListener('click', () => {{
      navLinks.classList.remove('open');
      hamburger.classList.remove('open');
      hamburger.setAttribute('aria-expanded', false);
    }});
  }});
</script>
</body>
</html>
"""

    DOCS.mkdir(exist_ok=True)
    (DOCS / "index.html").write_text(page)
    (DOCS / ".nojekyll").touch()

    # Generate RSS feed
    generate_rss(days, entries)

    print(f"Site built: docs/index.html (Day {days}, {sessions} sessions)")


def generate_rss(days, entries):
    """Generate RSS feed from journal entries."""
    site_url = "https://graycodeai.github.io/iterate"

    items = ""
    for entry in entries:
        title = html.escape(entry["title"])
        body = html.escape(entry["body"][:500])
        day = entry["day"]
        # Use RFC 2822 format for pubDate
        date_str = entry.get("date", "")
        if date_str:
            try:
                dt = datetime.strptime(date_str, "%Y-%m-%d")
                date_str = dt.strftime("%a, %d %b %Y 00:00:00 GMT")
            except ValueError:
                date_str = datetime.now().strftime("%a, %d %b %Y %H:%M:%S GMT")
        else:
            date_str = datetime.now().strftime("%a, %d %b %Y %H:%M:%S GMT")

        items += f"""
    <item>
      <title>Day {day} — {title}</title>
      <link>{site_url}</link>
      <guid>{site_url}#day-{day}</guid>
      <description>{body}</description>
      <pubDate>{date_str}</pubDate>
    </item>"""

    last_build = datetime.now(timezone.utc).strftime("%a, %d %b %Y %H:%M:%S GMT")
    rss = f"""<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>iterate — Self-Evolving Coding Agent</title>
    <link>{site_url}</link>
    <description>A coding agent that improves itself. Every session, documented.</description>
    <language>en</language>
    <lastBuildDate>{last_build}</lastBuildDate>
    {items}
  </channel>
</rss>
"""
    (DOCS / "feed.xml").write_text(rss)
    print(f"RSS feed written: docs/feed.xml")


if __name__ == "__main__":
    main()
