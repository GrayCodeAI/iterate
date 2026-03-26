#!/usr/bin/env python3
"""
Generate docs/dashboard.html from docs/stats.json + memory/coverage_history.jsonl.
Run after generate_stats.py in evolve.sh.
"""
import json
import os
import sys
from datetime import datetime

REPO = sys.argv[1] if len(sys.argv) > 1 else "."
STATS_FILE = os.path.join(REPO, "docs", "stats.json")
COVERAGE_FILE = os.path.join(REPO, "memory", "coverage_history.jsonl")
FAILURES_FILE = os.path.join(REPO, "memory", "failures.jsonl")
JOURNAL_FILE = os.path.join(REPO, "docs", "JOURNAL.md")
OUT_FILE = os.path.join(REPO, "docs", "dashboard.html")


def load_json(path):
    try:
        with open(path) as f:
            return json.load(f)
    except Exception:
        return {}


def load_jsonl(path, limit=30):
    rows = []
    try:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if line:
                    try:
                        rows.append(json.loads(line))
                    except Exception:
                        pass
    except Exception:
        pass
    return rows[-limit:]


def load_journal_days(path, limit=7):
    entries = []
    try:
        with open(path) as f:
            current = []
            for line in f:
                if line.startswith("## Day") and current:
                    entries.append(" ".join(current).strip())
                    current = [line.strip()]
                elif line.startswith("## Day"):
                    current = [line.strip()]
                elif current:
                    current.append(line.strip())
            if current:
                entries.append(" ".join(current).strip())
    except Exception:
        pass
    return entries[:limit]


stats = load_json(STATS_FILE)
coverage_rows = load_jsonl(COVERAGE_FILE)
failure_rows = load_jsonl(FAILURES_FILE, limit=10)
journal_days = load_journal_days(JOURNAL_FILE)

generated_at = stats.get("generated_at", datetime.utcnow().isoformat() + "Z")
total_commits = stats.get("total_commits", "—")
commits_week = stats.get("commits_this_week", "—")
lines_added = stats.get("lines_changed", {}).get("added", 0)
lines_removed = stats.get("lines_changed", {}).get("removed", 0)
test_count = stats.get("test_count", "—")
journal_entries = stats.get("journal_entries", "—")

# Coverage sparkline data
cov_labels = [str(r.get("day", i)) for i, r in enumerate(coverage_rows)]
cov_values = [r.get("coverage", 0) for r in coverage_rows]
cov_js_labels = json.dumps(cov_labels)
cov_js_values = json.dumps(cov_values)

# Failures table rows
failure_html = ""
if failure_rows:
    rows_html = ""
    for r in reversed(failure_rows):
        rows_html += f"<tr><td>Day {r.get('day','?')}</td><td>{r.get('task','?')}</td><td>{r.get('reason','')[:80]}</td></tr>\n"
    failure_html = f"""
<section>
  <h2>Recent Failures</h2>
  <table>
    <thead><tr><th>Day</th><th>Task</th><th>Reason</th></tr></thead>
    <tbody>{rows_html}</tbody>
  </table>
</section>"""

# Journal snippet
journal_html = ""
if journal_days:
    items = "".join(f"<li>{d[:120]}</li>\n" for d in journal_days[:5])
    journal_html = f"<section><h2>Recent Journal</h2><ul>{items}</ul></section>"

html = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>iterate — Evolution Dashboard</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
  <style>
    *{{box-sizing:border-box;margin:0;padding:0}}
    body{{font-family:system-ui,sans-serif;background:#0d0d0d;color:#e0e0e0;padding:2rem}}
    h1{{font-size:1.6rem;margin-bottom:.25rem;color:#a3e635}}
    .sub{{color:#888;font-size:.85rem;margin-bottom:2rem}}
    .grid{{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:1rem;margin-bottom:2rem}}
    .card{{background:#1a1a1a;border:1px solid #2a2a2a;border-radius:8px;padding:1.25rem}}
    .card .value{{font-size:2rem;font-weight:700;color:#a3e635}}
    .card .label{{font-size:.8rem;color:#888;margin-top:.25rem}}
    section{{background:#1a1a1a;border:1px solid #2a2a2a;border-radius:8px;padding:1.25rem;margin-bottom:1.5rem}}
    section h2{{font-size:1rem;color:#a3e635;margin-bottom:1rem}}
    canvas{{max-height:200px}}
    table{{width:100%;border-collapse:collapse;font-size:.85rem}}
    th,td{{text-align:left;padding:.5rem .75rem;border-bottom:1px solid #2a2a2a}}
    th{{color:#888}}
    ul{{list-style:none;font-size:.85rem;line-height:1.8}}
    li::before{{content:"▸ ";color:#a3e635}}
  </style>
</head>
<body>
  <h1>iterate — Evolution Dashboard</h1>
  <p class="sub">Last updated: {generated_at}</p>

  <div class="grid">
    <div class="card"><div class="value">{total_commits}</div><div class="label">Total Commits</div></div>
    <div class="card"><div class="value">{commits_week}</div><div class="label">Commits This Week</div></div>
    <div class="card"><div class="value">+{lines_added:,}</div><div class="label">Lines Added</div></div>
    <div class="card"><div class="value">-{lines_removed:,}</div><div class="label">Lines Removed</div></div>
    <div class="card"><div class="value">{test_count}</div><div class="label">Tests</div></div>
    <div class="card"><div class="value">{journal_entries}</div><div class="label">Journal Days</div></div>
  </div>

  <section>
    <h2>Test Coverage Over Time</h2>
    <canvas id="covChart"></canvas>
  </section>

  {failure_html}
  {journal_html}

  <script>
    new Chart(document.getElementById('covChart'), {{
      type: 'line',
      data: {{
        labels: {cov_js_labels},
        datasets: [{{
          label: 'Coverage %',
          data: {cov_js_values},
          borderColor: '#a3e635',
          backgroundColor: 'rgba(163,230,53,.1)',
          tension: 0.3,
          fill: true,
          pointRadius: 3
        }}]
      }},
      options: {{
        plugins: {{legend: {{display: false}}}},
        scales: {{
          y: {{min: 0, max: 100, grid: {{color:'#2a2a2a'}}, ticks: {{color:'#888'}}}},
          x: {{grid: {{color:'#2a2a2a'}}, ticks: {{color:'#888'}}}}
        }}
      }}
    }});
  </script>
</body>
</html>"""

os.makedirs(os.path.dirname(OUT_FILE), exist_ok=True)
with open(OUT_FILE, "w") as f:
    f.write(html)

print(f"Dashboard written to {OUT_FILE}")
