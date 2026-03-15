package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	repoPath := "."
	if len(os.Args) > 1 {
		repoPath = os.Args[1]
	}

	outDir := filepath.Join(repoPath, "docs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	commits, err := getCommits(repoPath)
	if err != nil {
		log.Fatal("get commits:", err)
	}

	dayCount := readFile(filepath.Join(repoPath, "DAY_COUNT"), "0")
	journal := readFile(filepath.Join(repoPath, "JOURNAL.md"), "")
	identity := readFile(filepath.Join(repoPath, "IDENTITY.md"), "")

	page := buildSite(commits, dayCount, journal, identity)

	outPath := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
		log.Fatal("write site:", err)
	}

	fmt.Printf("Site built: %s (%d commits)\n", outPath, len(commits))
}

type Commit struct {
	Hash    string
	Date    string
	Message string
	Author  string
}

func getCommits(repoPath string) ([]Commit, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%H|%ai|%s|%an", "--no-merges")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0][:8],
			Date:    formatDate(parts[1]),
			Message: parts[2],
			Author:  parts[3],
		})
	}
	return commits, nil
}

func formatDate(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05 -0700", s)
	if err != nil {
		return s
	}
	return t.Format("Jan 2, 2006")
}

func readFile(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return strings.TrimSpace(string(data))
}

func buildSite(commits []Commit, dayCount, journal, identity string) string {
	day, _ := strconv.Atoi(strings.TrimSpace(dayCount))

	// Count committed vs reverted from journal
	committed := strings.Count(journal, "SUCCESS")
	reverted := strings.Count(journal, "FAILED")

	commitRows := ""
	for _, c := range commits {
		isBot := c.Author == "iterate[bot]"
		rowClass := ""
		if isBot {
			rowClass = ` class="bot-commit"`
		}
		commitRows += fmt.Sprintf(`<tr%s>
			<td class="hash">%s</td>
			<td>%s</td>
			<td>%s</td>
			<td>%s</td>
		</tr>`, rowClass,
			html.EscapeString(c.Hash),
			html.EscapeString(c.Date),
			html.EscapeString(c.Message),
			html.EscapeString(c.Author),
		)
	}

	journalHTML := ""
	entries := strings.Split(journal, "---")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" || entry == "#Journal" {
			continue
		}
		status := "neutral"
		if strings.Contains(entry, "SUCCESS") {
			status = "success"
		} else if strings.Contains(entry, "FAILED") {
			status = "failed"
		}
		journalHTML += fmt.Sprintf(`<div class="journal-entry %s">%s</div>`,
			status, html.EscapeString(entry))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>iterate — self-evolving agent</title>
<style>
  :root { --bg:#0f0f0f; --surface:#161616; --border:#262626; --text:#e0e0e0; --muted:#666; --green:#22c55e; --red:#ef4444; --purple:#a78bfa; }
  * { box-sizing:border-box; margin:0; padding:0; }
  body { background:var(--bg); color:var(--text); font-family:system-ui,sans-serif; line-height:1.6; }
  a { color:var(--purple); text-decoration:none; }
  header { padding:3rem 2rem 2rem; border-bottom:1px solid var(--border); max-width:900px; margin:0 auto; }
  header h1 { font-size:2rem; font-weight:700; letter-spacing:-0.03em; }
  header p { color:var(--muted); margin-top:0.5rem; font-size:1rem; }
  .stats { display:flex; gap:2rem; margin-top:1.5rem; }
  .stat { }
  .stat .val { font-size:1.75rem; font-weight:700; color:#fff; }
  .stat .lbl { font-size:0.75rem; color:var(--muted); text-transform:uppercase; letter-spacing:0.06em; }
  main { max-width:900px; margin:0 auto; padding:2rem; }
  section { margin-bottom:3rem; }
  h2 { font-size:1rem; font-weight:600; color:var(--muted); text-transform:uppercase; letter-spacing:0.08em; margin-bottom:1rem; border-bottom:1px solid var(--border); padding-bottom:0.5rem; }
  table { width:100%%; border-collapse:collapse; font-size:0.85rem; }
  th { text-align:left; padding:0.5rem 0.75rem; color:var(--muted); font-size:0.75rem; font-weight:500; border-bottom:1px solid var(--border); }
  td { padding:0.5rem 0.75rem; border-bottom:1px solid #1a1a1a; }
  .hash { font-family:monospace; color:var(--purple); font-size:0.8rem; }
  .bot-commit td { background:#0d0d1a; }
  .journal-entry { background:var(--surface); border:1px solid var(--border); border-radius:8px; padding:1rem; margin-bottom:1rem; font-size:0.85rem; white-space:pre-wrap; }
  .journal-entry.success { border-left:3px solid var(--green); }
  .journal-entry.failed  { border-left:3px solid var(--red); }
  .identity { background:var(--surface); border:1px solid var(--border); border-radius:8px; padding:1.25rem; font-size:0.85rem; white-space:pre-wrap; }
  footer { text-align:center; padding:2rem; color:var(--muted); font-size:0.8rem; border-top:1px solid var(--border); }
</style>
</head>
<body>
<header>
  <h1>iterate</h1>
  <p>A self-evolving Go coding agent. One session per day. Watch it grow.</p>
  <div class="stats">
    <div class="stat"><div class="val">%d</div><div class="lbl">Days old</div></div>
    <div class="stat"><div class="val">%d</div><div class="lbl">Commits</div></div>
    <div class="stat"><div class="val" style="color:var(--green)">%d</div><div class="lbl">Committed</div></div>
    <div class="stat"><div class="val" style="color:var(--red)">%d</div><div class="lbl">Reverted</div></div>
  </div>
</header>
<main>
  <section>
    <h2>Commit history</h2>
    <table>
      <thead><tr><th>Hash</th><th>Date</th><th>Message</th><th>Author</th></tr></thead>
      <tbody>%s</tbody>
    </table>
  </section>
  <section>
    <h2>Session journal</h2>
    %s
  </section>
  <section>
    <h2>Identity (immutable)</h2>
    <div class="identity">%s</div>
  </section>
</main>
<footer>Built by iterate[bot] · <a href="https://github.com/yourusername/iterate">GitHub</a></footer>
</body>
</html>`,
		day, len(commits), committed, reverted,
		commitRows, journalHTML,
		html.EscapeString(identity),
	)
}
