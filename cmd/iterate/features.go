package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"golang.org/x/term"
)

// ---------------------------------------------------------------------------
// /context — show current context stats
// ---------------------------------------------------------------------------

func contextStats(messages []iteragent.Message) string {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	approxTokens := totalChars / 4
	pct := float64(approxTokens) / float64(contextWindow) * 100 // assume 200k context window
	return fmt.Sprintf("Messages: %d  |  ~%d tokens  |  ~%.0f%% of context window",
		len(messages), approxTokens, pct)
}

// ---------------------------------------------------------------------------
// /export — export conversation to markdown
// ---------------------------------------------------------------------------

func exportConversation(messages []iteragent.Message, path string) error {
	var b strings.Builder
	b.WriteString("# iterate — Conversation Export\n\n")
	b.WriteString(fmt.Sprintf("Exported: %s\n\n---\n\n", time.Now().Format("2006-01-02 15:04:05")))
	for _, m := range messages {
		role := strings.ToUpper(m.Role[:1]) + m.Role[1:]
		b.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", role, m.Content))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ---------------------------------------------------------------------------
// /todos — find TODO/FIXME/HACK comments in the codebase
// ---------------------------------------------------------------------------

func findTodos(repoPath string) []string {
	var results []string
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".md" && ext != ".sh" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		rel, _ := filepath.Rel(repoPath, path)
		lineNum := 0
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			lineNum++
			line := sc.Text()
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") || strings.Contains(upper, "HACK") {
				trimmed := strings.TrimSpace(line)
				results = append(results, fmt.Sprintf("%s:%d  %s", rel, lineNum, trimmed))
			}
		}
		return nil
	})
	return results
}

// ---------------------------------------------------------------------------
// /issues — list GitHub issues via gh cli
// ---------------------------------------------------------------------------

func listGitHubIssues(repoPath string, limit int) (string, error) {
	cmd := exec.Command("gh", "issue", "list",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,state,labels",
		"--template", `{{range .}}#{{.number}} {{.title}} [{{.state}}]{{"\n"}}{{end}}`)
	cmd.Dir = repoPath
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh issue list: %s", strings.TrimSpace(out.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// ---------------------------------------------------------------------------
// lastResponse stores the most recent agent response for /copy and /retry
// ---------------------------------------------------------------------------

var lastResponse string
var lastPrompt string

// ---------------------------------------------------------------------------
// /multi — multi-line paste/input mode
// ---------------------------------------------------------------------------

// readMultiLine collects lines until the user enters a line that is exactly "."
// Returns the collected text (without the terminating ".").
func readMultiLine() (string, bool) {
	fd := int(os.Stdin.Fd())
	// Restore normal mode for multi-line
	oldState, _ := term.MakeRaw(fd)
	term.Restore(fd, oldState)

	fmt.Printf("%s[multi-line mode — enter . on a blank line to send, Ctrl+C to cancel]%s\n", colorDim, colorReset)
	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return strings.Join(lines, "\n"), true
}

// ---------------------------------------------------------------------------
// /search-replace — find and replace text across all Go files
// ---------------------------------------------------------------------------

func searchReplace(repoPath, oldText, newText string) (int, error) {
	count := 0
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".md" && ext != ".sh" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(data), oldText) {
			return nil
		}
		newData := strings.ReplaceAll(string(data), oldText, newText)
		if err := os.WriteFile(path, []byte(newData), 0o644); err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoPath, path)
		fmt.Printf("  %s%s%s\n", colorDim, rel, colorReset)
		count++
		return nil
	})
	return count, err
}

// ---------------------------------------------------------------------------
// Memory helpers — /learn and /memories
// ---------------------------------------------------------------------------

func appendLearning(repoPath, fact string) error {
	path := filepath.Join(repoPath, "memory", "learnings.jsonl")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry := map[string]string{
		"fact":       fact,
		"created_at": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(entry)
	_, err = fmt.Fprintln(f, string(data))
	return err
}

// ---------------------------------------------------------------------------
// /memo — append to JOURNAL.md
// ---------------------------------------------------------------------------

func appendMemo(repoPath, text string) error {
	path := filepath.Join(repoPath, "docs/docs/JOURNAL.md")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## Memo — %s\n\n%s\n", time.Now().Format("2006-01-02 15:04"), text)
	return err
}

// ---------------------------------------------------------------------------
// /compact-hard — very aggressive context compaction
// ---------------------------------------------------------------------------

func compactHard(messages []iteragent.Message, keepLast int) []iteragent.Message {
	if len(messages) <= keepLast {
		return messages
	}
	// Keep system context (first 2) + last keepLast messages
	var out []iteragent.Message
	if len(messages) > 2 {
		out = append(out, messages[:2]...)
	}
	tail := messages[len(messages)-keepLast:]
	out = append(out, tail...)
	return out
}

// ---------------------------------------------------------------------------
// /pin-list
// ---------------------------------------------------------------------------

func formatPinnedMessages(msgs []iteragent.Message) string {
	if len(msgs) == 0 {
		return "No pinned messages."
	}
	var lines []string
	for i, m := range msgs {
		snippet := m.Content
		if len(snippet) > 80 {
			snippet = snippet[:80] + "…"
		}
		lines = append(lines, fmt.Sprintf("  %d  [%s] %s", i+1, m.Role, snippet))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /snapshot — save a named snapshot of the repo state
// ---------------------------------------------------------------------------

func snapshotsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "snapshots")
}

type snapshot struct {
	Name      string              `json:"name"`
	CreatedAt time.Time           `json:"created_at"`
	Branch    string              `json:"branch"`
	Commit    string              `json:"commit"`
	Messages  []iteragent.Message `json:"messages"`
}

func saveSnapshot(repoPath, name string, messages []iteragent.Message) error {
	dir := snapshotsDir()
	_ = os.MkdirAll(dir, 0o755)

	branchOut, _ := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	commitOut, _ := exec.Command("git", "-C", repoPath, "rev-parse", "--short", "HEAD").Output()

	snap := snapshot{
		Name:      name,
		CreatedAt: time.Now(),
		Branch:    strings.TrimSpace(string(branchOut)),
		Commit:    strings.TrimSpace(string(commitOut)),
		Messages:  messages,
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644)
}

func listSnapshots() []snapshot {
	entries, _ := os.ReadDir(snapshotsDir())
	var snaps []snapshot
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			data, err := os.ReadFile(filepath.Join(snapshotsDir(), e.Name()))
			if err != nil {
				continue
			}
			var s snapshot
			if json.Unmarshal(data, &s) == nil {
				snaps = append(snaps, s)
			}
		}
	}
	return snaps
}

// ---------------------------------------------------------------------------
// /pair — pair programming mode system prompt
// ---------------------------------------------------------------------------

const pairModePrompt = `You are in pair programming mode. Act as an experienced pair programmer:
- Think out loud as you work through problems
- Explain what you're about to do before doing it
- Ask clarifying questions when requirements are ambiguous
- Point out potential issues or edge cases you notice
- Suggest alternative approaches when relevant
- Keep the human in the loop on every significant decision`

// ---------------------------------------------------------------------------
// /template system
// ---------------------------------------------------------------------------

type promptTemplate struct {
	Name    string    `json:"name"`
	Prompt  string    `json:"prompt"`
	Created time.Time `json:"created"`
}

func templatesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "templates.json")
}

func loadTemplates() []promptTemplate {
	data, err := os.ReadFile(templatesPath())
	if err != nil {
		return nil
	}
	var ts []promptTemplate
	if err := json.Unmarshal(data, &ts); err != nil {
		slog.Warn("failed to parse templates", "err", err)
	}
	return ts
}

func saveTemplates(ts []promptTemplate) {
	data, _ := json.MarshalIndent(ts, "", "  ")
	if err := os.MkdirAll(filepath.Dir(templatesPath()), 0o755); err != nil {
		slog.Warn("failed to create templates dir", "err", err)
		return
	}
	if err := os.WriteFile(templatesPath(), data, 0o644); err != nil {
		slog.Warn("failed to write templates file", "err", err)
	}
}

func addTemplate(name, prompt string) {
	ts := loadTemplates()
	for i, t := range ts {
		if t.Name == name {
			ts[i] = promptTemplate{Name: name, Prompt: prompt, Created: time.Now()}
			saveTemplates(ts)
			return
		}
	}
	ts = append(ts, promptTemplate{Name: name, Prompt: prompt, Created: time.Now()})
	saveTemplates(ts)
}

// ---------------------------------------------------------------------------
// /init — scaffold a new iterate-enabled project
// ---------------------------------------------------------------------------

func initProject(repoPath, projectName string) []string {
	files := map[string]string{
		"docs/docs/IDENTITY.md":            fmt.Sprintf("# %s\n\nA self-evolving project powered by iterate.\n", projectName),
		"docs/docs/PERSONALITY.md":         "Helpful, concise, and direct.\n",
		"docs/docs/JOURNAL.md":             fmt.Sprintf("# iterate Evolution Journal\n\n## Day 1 — %s\n\nProject initialized.\n", time.Now().Format("2006-01-02")),
		"DAY_COUNT":              "1",
		"memory/learnings.jsonl": "",
		"skills/.keep":           "",
	}
	var created []string
	for path, content := range files {
		full := filepath.Join(repoPath, path)
		if _, err := os.Stat(full); err == nil {
			continue // already exists
		}
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err == nil {
			created = append(created, path)
		}
	}
	return created
}
