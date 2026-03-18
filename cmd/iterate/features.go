package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"golang.org/x/term"
)

// ---------------------------------------------------------------------------
// Audit log
// ---------------------------------------------------------------------------

var auditLogPath string

func initAuditLog() {
	home, _ := os.UserHomeDir()
	auditLogPath = filepath.Join(home, ".iterate", "audit.log")
	_ = os.MkdirAll(filepath.Dir(auditLogPath), 0o755)
}

func logAudit(toolName string, args map[string]interface{}, result string) {
	if auditLogPath == "" {
		return
	}
	f, err := os.OpenFile(auditLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	argBytes, _ := json.Marshal(args)
	snippet := result
	if len(snippet) > 120 {
		snippet = snippet[:120] + "…"
	}
	fmt.Fprintf(f, "[%s] %s %s → %s\n", time.Now().Format("2006-01-02 15:04:05"), toolName, argBytes, snippet)
}

// ---------------------------------------------------------------------------
// Permission / safe mode
// ---------------------------------------------------------------------------

// spinnerActive is set to 1 while the spinner goroutine is printing.
// Tool wrappers wait for it to reach 0 before showing a prompt.
var spinnerActive atomic.Int32

// safeMode controls whether destructive tools require confirmation.
var safeMode bool

// deniedTools is the set of tools blocked in safe mode.
var deniedTools = map[string]bool{
	"bash":       true,
	"write_file": true,
	"edit_file":  true,
}

// wrapToolsWithPermissions wraps tools that need approval in safe mode
// and adds audit logging to all tools.
func wrapToolsWithPermissions(tools []iteragent.Tool) []iteragent.Tool {
	out := make([]iteragent.Tool, len(tools))
	for i, t := range tools {
		t := t // capture
		origExec := t.Execute
		t.Execute = func(ctx context.Context, args map[string]string) (string, error) {
			// Build args for audit log
			auditArgs := make(map[string]interface{}, len(args))
			for k, v := range args {
				auditArgs[k] = v
			}

			if safeMode && deniedTools[t.Name] {
				// Wait briefly for spinner to stop (EventToolExecutionStart was just emitted)
				for spinnerActive.Load() == 1 {
					time.Sleep(5 * time.Millisecond)
				}
				// Show permission prompt
				fmt.Printf("\n%s⚠ Safe mode: allow %s?%s ", colorYellow, t.Name, colorReset)
				answer, ok := promptLine("(y/N):")
				if !ok || strings.ToLower(strings.TrimSpace(answer)) != "y" {
					logAudit(t.Name, auditArgs, "DENIED")
					return "Tool execution denied by user (safe mode).", nil
				}
			}

			result, err := origExec(ctx, args)
			logAudit(t.Name, auditArgs, result)
			return result, err
		}
		out[i] = t
	}
	return out
}

// ---------------------------------------------------------------------------
// Repo index (codebase summary for system prompt)
// ---------------------------------------------------------------------------

// buildRepoIndex returns a compact file-tree string for the repo.
// Limited to 200 files and 4 levels deep to avoid bloating the prompt.
func buildRepoIndex(repoPath string) string {
	var lines []string
	count := 0
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || count >= 200 {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		if rel == "." {
			return nil
		}
		// Skip hidden dirs (except .iterate), vendor, node_modules, build artefacts
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, p := range parts {
			if (strings.HasPrefix(p, ".") && p != ".iterate") ||
				p == "vendor" || p == "node_modules" || p == "dist" || p == "build" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if len(parts) > 4 {
			return nil
		}
		indent := strings.Repeat("  ", len(parts)-1)
		if d.IsDir() {
			lines = append(lines, indent+d.Name()+"/")
		} else {
			lines = append(lines, indent+d.Name())
			count++
		}
		return nil
	})
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Session save / load (exposed to REPL)
// ---------------------------------------------------------------------------

func sessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "sessions")
}

func saveSession(name string, messages []iteragent.Message) error {
	dir := sessionsDir()
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadSession(name string) ([]iteragent.Message, error) {
	path := filepath.Join(sessionsDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var msgs []iteragent.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func listSessions() []string {
	entries, _ := os.ReadDir(sessionsDir())
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// Bookmarks
// ---------------------------------------------------------------------------

type Bookmark struct {
	Name      string              `json:"name"`
	CreatedAt time.Time           `json:"created_at"`
	Messages  []iteragent.Message `json:"messages"`
}

func bookmarksPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "bookmarks.json")
}

func loadBookmarks() []Bookmark {
	data, err := os.ReadFile(bookmarksPath())
	if err != nil {
		return nil
	}
	var bms []Bookmark
	json.Unmarshal(data, &bms)
	return bms
}

func saveBookmarks(bms []Bookmark) {
	data, _ := json.MarshalIndent(bms, "", "  ")
	_ = os.WriteFile(bookmarksPath(), data, 0o644)
}

func addBookmark(name string, messages []iteragent.Message) {
	bms := loadBookmarks()
	// Replace if name exists
	for i, b := range bms {
		if b.Name == name {
			bms[i] = Bookmark{Name: name, CreatedAt: time.Now(), Messages: messages}
			saveBookmarks(bms)
			return
		}
	}
	bms = append(bms, Bookmark{Name: name, CreatedAt: time.Now(), Messages: messages})
	saveBookmarks(bms)
}

// ---------------------------------------------------------------------------
// /find — fuzzy file search
// ---------------------------------------------------------------------------

// findFiles returns files in repoPath whose relative path contains the pattern (case-insensitive).
func findFiles(repoPath, pattern string) []string {
	pattern = strings.ToLower(pattern)
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
		rel, _ := filepath.Rel(repoPath, path)
		if strings.Contains(strings.ToLower(rel), pattern) {
			results = append(results, rel)
		}
		return nil
	})
	return results
}

// ---------------------------------------------------------------------------
// /web — fetch a URL and return readable text
// ---------------------------------------------------------------------------

func fetchURL(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024)) // 256 KB max
	if err != nil {
		return "", err
	}
	text := string(body)
	// Strip HTML tags very simply for readability
	if strings.Contains(resp.Header.Get("Content-Type"), "html") {
		text = stripHTMLTags(text)
	}
	// Trim excessive blank lines
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
			b.WriteRune(' ')
		} else if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// /grep — content search across the repo
// ---------------------------------------------------------------------------

func grepRepo(repoPath, pattern string) (string, error) {
	cmd := exec.Command("grep", "-rn", "--include=*.go", "--include=*.md",
		"--include=*.json", "--include=*.sh", "--include=*.yaml", "--include=*.yml",
		"-m", "5", pattern, repoPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run() // ignore exit code (1 = no matches)
	result := out.String()
	if result == "" {
		return fmt.Sprintf("No matches for %q", pattern), nil
	}
	// Make paths relative
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(result), "\n") {
		lines = append(lines, strings.TrimPrefix(l, repoPath+"/"))
	}
	return strings.Join(lines, "\n"), nil
}

// ---------------------------------------------------------------------------
// /context — show current context stats
// ---------------------------------------------------------------------------

func contextStats(messages []iteragent.Message) string {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	approxTokens := totalChars / 4
	pct := float64(approxTokens) / 200000.0 * 100 // assume 200k context window
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
// /copy — copy text to clipboard (macOS pbcopy, Linux xclip)
// ---------------------------------------------------------------------------

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch {
	case commandExists("pbcopy"):
		cmd = exec.Command("pbcopy")
	case commandExists("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case commandExists("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--input")
	default:
		return fmt.Errorf("no clipboard tool found (pbcopy/xclip/xsel)")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ---------------------------------------------------------------------------
// /watch — watch repo for file changes and auto-run tests
// ---------------------------------------------------------------------------

// watchState tracks the active watch goroutine.
var watchCancel context.CancelFunc
var watchMu sync.Mutex

func startWatch(repoPath string) {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	watchCancel = cancel

	go func() {
		fmt.Printf("%s[watch] monitoring %s for changes…%s\n", colorDim, repoPath, colorReset)
		lastMod := latestModTime(repoPath)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("%s[watch] stopped%s\n", colorDim, colorReset)
				return
			case <-ticker.C:
				cur := latestModTime(repoPath)
				if cur.After(lastMod) {
					lastMod = cur
					fmt.Printf("\n%s[watch] change detected — running tests…%s\n", colorYellow, colorReset)
					cmd := exec.Command("go", "test", "./...")
					cmd.Dir = repoPath
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						fmt.Printf("%s[watch] tests failed%s\n", colorRed, colorReset)
					} else {
						fmt.Printf("%s[watch] ✓ all tests pass%s\n", colorLime, colorReset)
					}
				}
			}
		}
	}()
}

func stopWatch() {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watchCancel != nil {
		watchCancel()
		watchCancel = nil
	}
}

func latestModTime(repoPath string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}

// ---------------------------------------------------------------------------
// /pin — pin messages so they survive compaction
// ---------------------------------------------------------------------------

// pinnedMessages are always prepended when the agent runs after compaction.
var pinnedMessages []iteragent.Message

// ---------------------------------------------------------------------------
// /issues — list GitHub issues via gh cli
// ---------------------------------------------------------------------------

func listGitHubIssues(repoPath string, limit int) (string, error) {
	cmd := exec.Command("gh", "issue", "list",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,state,labels",
		"--template", `{{range .}}#{{.number}} {{.title}} [{{.state}}]{{"\n"}}{{end}}`)
	cmd.Dir = repoPath
	var out bytes.Buffer
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
// /alias — persistent command shortcuts
// ---------------------------------------------------------------------------

type aliasMap map[string]string

func aliasesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "aliases.json")
}

func loadAliases() aliasMap {
	data, err := os.ReadFile(aliasesPath())
	if err != nil {
		return aliasMap{}
	}
	var m aliasMap
	json.Unmarshal(data, &m)
	if m == nil {
		return aliasMap{}
	}
	return m
}

func saveAliases(m aliasMap) {
	data, _ := json.MarshalIndent(m, "", "  ")
	_ = os.MkdirAll(filepath.Dir(aliasesPath()), 0o755)
	_ = os.WriteFile(aliasesPath(), data, 0o644)
}

// resolveAlias expands an alias if one exists, otherwise returns line unchanged.
func resolveAlias(line string) string {
	aliases := loadAliases()
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return line
	}
	if expanded, ok := aliases[parts[0]]; ok {
		if len(parts) > 1 {
			return expanded + " " + strings.Join(parts[1:], " ")
		}
		return expanded
	}
	return line
}

// ---------------------------------------------------------------------------
// /stats — session statistics
// ---------------------------------------------------------------------------

var sessionStart = time.Now()
var sessionToolCalls int
var sessionMessages int

func recordToolCall() { sessionToolCalls++ }
func recordMessage()  { sessionMessages++ }

func sessionStats() string {
	elapsed := time.Since(sessionStart).Round(time.Second)
	return fmt.Sprintf(
		"Duration: %s  |  Messages sent: %d  |  Tool calls: ~%d  |  Output tokens: ~%d",
		elapsed, sessionMessages, sessionToolCalls, sessionTokens)
}

// ---------------------------------------------------------------------------
// /theme — color theme switching
// ---------------------------------------------------------------------------

type theme struct {
	name   string
	lime   string
	yellow string
	cyan   string
	purple string
	dim    string
	bold   string
	red    string
	green  string
	amber  string
	blue   string
	reset  string
}

var themes = map[string]theme{
	"default": {
		name: "default", lime: "\033[38;5;154m", yellow: "\033[38;5;220m",
		cyan: "\033[36m", purple: "\033[38;5;141m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[31m", green: "\033[38;5;114m",
		amber: "\033[38;5;221m", blue: "\033[38;5;75m", reset: "\033[0m",
	},
	"nord": {
		name: "nord", lime: "\033[38;5;109m", yellow: "\033[38;5;222m",
		cyan: "\033[38;5;110m", purple: "\033[38;5;146m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[38;5;174m", green: "\033[38;5;108m",
		amber: "\033[38;5;179m", blue: "\033[38;5;67m", reset: "\033[0m",
	},
	"monokai": {
		name: "monokai", lime: "\033[38;5;148m", yellow: "\033[38;5;227m",
		cyan: "\033[38;5;81m", purple: "\033[38;5;141m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[38;5;197m", green: "\033[38;5;148m",
		amber: "\033[38;5;215m", blue: "\033[38;5;81m", reset: "\033[0m",
	},
	"minimal": {
		name: "minimal", lime: "\033[32m", yellow: "\033[33m",
		cyan: "\033[36m", purple: "\033[35m", dim: "\033[2m",
		bold: "\033[1m", red: "\033[31m", green: "\033[32m",
		amber: "\033[33m", blue: "\033[34m", reset: "\033[0m",
	},
}

func applyTheme(t theme) {
	colorLime = t.lime
	colorYellow = t.yellow
	colorCyan = t.cyan
	colorPurple = t.purple
	colorDim = t.dim
	colorBold = t.bold
	colorRed = t.red
	colorGreen = t.green
	colorAmber = t.amber
	colorBlue = t.blue
	colorReset = t.reset
}

// ---------------------------------------------------------------------------
// /doctor — project health check
// ---------------------------------------------------------------------------

type healthResult struct {
	check  string
	ok     bool
	detail string
}

func runDoctor(repoPath string) []healthResult {
	var results []healthResult
	run := func(check, name string, args ...string) healthResult {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		detail := strings.TrimSpace(string(out))
		if len(detail) > 80 {
			detail = detail[:80] + "…"
		}
		return healthResult{check: check, ok: err == nil, detail: detail}
	}

	results = append(results, run("go build", "go", "build", "./..."))
	results = append(results, run("go vet", "go", "vet", "./..."))
	results = append(results, run("go test", "go", "test", "-count=1", "-timeout=30s", "./..."))

	// Check go.sum is up to date
	modCmd := exec.Command("go", "mod", "verify")
	modCmd.Dir = repoPath
	out, err := modCmd.CombinedOutput()
	results = append(results, healthResult{
		check: "go mod verify", ok: err == nil,
		detail: strings.TrimSpace(string(out)),
	})

	// Git clean?
	statusOut, _ := exec.Command("git", "-C", repoPath, "status", "--short").Output()
	dirty := strings.TrimSpace(string(statusOut)) != ""
	detail := "working tree clean"
	if dirty {
		detail = "uncommitted changes present"
	}
	results = append(results, healthResult{check: "git clean", ok: !dirty, detail: detail})

	return results
}

// ---------------------------------------------------------------------------
// /coverage — test coverage report
// ---------------------------------------------------------------------------

func runCoverage(repoPath string) (string, error) {
	cmd := exec.Command("go", "test", "-coverprofile=/tmp/iterate-cover.out", "./...")
	cmd.Dir = repoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	// Summary
	summary := exec.Command("go", "tool", "cover", "-func=/tmp/iterate-cover.out")
	summary.Dir = repoPath
	sumOut, err := summary.Output()
	if err != nil {
		return out.String(), nil
	}
	return string(sumOut), nil
}

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
	json.Unmarshal(data, &ts)
	return ts
}

func saveTemplates(ts []promptTemplate) {
	data, _ := json.MarshalIndent(ts, "", "  ")
	_ = os.MkdirAll(filepath.Dir(templatesPath()), 0o755)
	_ = os.WriteFile(templatesPath(), data, 0o644)
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
// /notify — ring terminal bell when agent finishes
// ---------------------------------------------------------------------------

var notifyEnabled bool

func maybeNotify() {
	if notifyEnabled {
		fmt.Print("\a") // terminal bell
	}
}

// ---------------------------------------------------------------------------
// /changelog — generate CHANGELOG from git log
// ---------------------------------------------------------------------------

func buildChangelogPrompt(repoPath string, since string) string {
	args := []string{"-C", repoPath, "log", "--oneline"}
	if since != "" {
		args = append(args, since+"..HEAD")
	} else {
		args = append(args, "-50")
	}
	out, _ := exec.Command("git", args...).Output()
	log := strings.TrimSpace(string(out))
	if log == "" {
		return "No commits found."
	}
	return fmt.Sprintf(
		"Generate a CHANGELOG.md entry from these git commits. "+
			"Group into: Added, Changed, Fixed, Removed. Be concise.\n\n```\n%s\n```", log)
}

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
// Agent mode — read-only /ask mode (bash and write tools disabled)
// ---------------------------------------------------------------------------

type agentMode int

const (
	modeNormal    agentMode = iota
	modeAsk                 // read-only: no bash, no write_file, no edit_file
	modeArchitect           // planning only: no tools at all
)

var currentMode agentMode

// readOnlyTools filters out destructive tools for /ask mode.
func readOnlyTools(tools []iteragent.Tool) []iteragent.Tool {
	blocked := map[string]bool{
		"bash": true, "write_file": true, "edit_file": true,
		"git_commit": true, "git_revert": true, "run_tests": true,
	}
	var out []iteragent.Tool
	for _, t := range tools {
		if !blocked[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// /set — runtime config (temperature, max_tokens)
// ---------------------------------------------------------------------------

type runtimeConfig struct {
	Temperature *float32
	MaxTokens   *int
}

var rtConfig runtimeConfig

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

func readActiveLearnings(repoPath string) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "memory", "active_learnings.md"))
	if err != nil {
		// Fall back to last 10 lines of learnings.jsonl
		raw, err2 := os.ReadFile(filepath.Join(repoPath, "memory", "learnings.jsonl"))
		if err2 != nil {
			return ""
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		if len(lines) > 10 {
			lines = lines[len(lines)-10:]
		}
		return strings.Join(lines, "\n")
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// /memo — append to JOURNAL.md
// ---------------------------------------------------------------------------

func appendMemo(repoPath, text string) error {
	path := filepath.Join(repoPath, "JOURNAL.md")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## Memo — %s\n\n%s\n", time.Now().Format("2006-01-02 15:04"), text)
	return err
}

// ---------------------------------------------------------------------------
// git helpers — /log, /stash, /branch, /checkout, /merge, /tag, /revert-file
// ---------------------------------------------------------------------------

func gitLog(repoPath string, n int) string {
	out, err := exec.Command("git", "-C", repoPath, "log",
		fmt.Sprintf("-%d", n),
		"--oneline", "--decorate", "--color").Output()
	if err != nil {
		return fmt.Sprintf("git log error: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func gitStash(repoPath string, pop bool) error {
	args := []string{"-C", repoPath, "stash"}
	if pop {
		args = append(args, "pop")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitBranches(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil
	}
	var branches []string
	for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if b = strings.TrimSpace(b); b != "" {
			branches = append(branches, b)
		}
	}
	return branches
}

func gitCurrentBranch(repoPath string) string {
	out, _ := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	return strings.TrimSpace(string(out))
}

func gitTags(repoPath string) []string {
	out, err := exec.Command("git", "-C", repoPath, "tag", "--sort=-creatordate").Output()
	if err != nil {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// ---------------------------------------------------------------------------
// /review — code review prompt builder
// ---------------------------------------------------------------------------

func buildReviewPrompt(repoPath string) string {
	// Get current diff for context
	out, _ := exec.Command("git", "-C", repoPath, "diff", "HEAD").Output()
	diff := strings.TrimSpace(string(out))
	if diff == "" {
		out, _ = exec.Command("git", "-C", repoPath, "diff").Output()
		diff = strings.TrimSpace(string(out))
	}
	base := "Review the current code changes. Look for: bugs, security issues, performance problems, " +
		"missing error handling, and style violations. Be concise and actionable.\n\n"
	if diff != "" {
		if len(diff) > 6000 {
			diff = diff[:6000] + "\n…[truncated]"
		}
		return base + "```diff\n" + diff + "\n```"
	}
	return base + "No diff found — review the overall codebase structure and quality."
}

// ---------------------------------------------------------------------------
// /summarize — ask the model to summarize the conversation
// ---------------------------------------------------------------------------

func buildSummarizePrompt(messages []iteragent.Message) string {
	if len(messages) == 0 {
		return "The conversation is empty."
	}
	return fmt.Sprintf(
		"Summarize this conversation in 3-5 bullet points. Focus on: what was asked, "+
			"what was implemented, and any decisions made. Be brief.\n\n"+
			"(Conversation has %d messages)", len(messages))
}

// ---------------------------------------------------------------------------
// Visual context bar
// ---------------------------------------------------------------------------

func contextBar(messages []iteragent.Message, windowSize int) string {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	tokens := totalChars / 4
	pct := float64(tokens) / float64(windowSize) * 100
	if pct > 100 {
		pct = 100
	}
	barWidth := 40
	filled := int(float64(barWidth) * pct / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	color := colorGreen
	if pct > 75 {
		color = colorYellow
	}
	if pct > 90 {
		color = colorRed
	}
	return fmt.Sprintf("%s%s%s %.0f%%  ~%d / %d tokens  %d msgs",
		color, bar, colorReset, pct, tokens, windowSize, len(messages))
}

// ---------------------------------------------------------------------------
// /count-lines — lines of code breakdown
// ---------------------------------------------------------------------------

func countLines(repoPath string) map[string]int {
	counts := map[string]int{}
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
		if ext == "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Count(string(data), "\n") + 1
		counts[ext] += lines
		return nil
	})
	return counts
}

// ---------------------------------------------------------------------------
// /hotspots — files changed most in git history
// ---------------------------------------------------------------------------

func gitHotspots(repoPath string, n int) string {
	out, err := exec.Command("git", "-C", repoPath, "log",
		"--pretty=format:", "--name-only", "--diff-filter=M").Output()
	if err != nil {
		return ""
	}
	freq := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			freq[line]++
		}
	}
	type entry struct {
		file  string
		count int
	}
	var entries []entry
	for f, c := range freq {
		entries = append(entries, entry{f, c})
	}
	// sort descending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].count > entries[i].count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	if len(entries) > n {
		entries = entries[:n]
	}
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("  %3d  %s", e.count, e.file))
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /contributors
// ---------------------------------------------------------------------------

func gitContributors(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "shortlog", "-sn", "--no-merges").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// /languages — file extension breakdown
// ---------------------------------------------------------------------------

func languageBreakdown(repoPath string) string {
	counts := countLines(repoPath)
	type entry struct {
		ext   string
		lines int
	}
	var entries []entry
	for ext, n := range counts {
		entries = append(entries, entry{ext, n})
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].lines > entries[i].lines {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	total := 0
	for _, e := range entries {
		total += e.lines
	}
	var lines []string
	for _, e := range entries {
		if e.lines < 10 {
			continue
		}
		pct := float64(e.lines) / float64(total) * 100
		bar := strings.Repeat("█", int(pct/2))
		lines = append(lines, fmt.Sprintf("  %-8s %5d  %s %.0f%%", e.ext, e.lines, bar, pct))
	}
	return fmt.Sprintf("Total: %d lines\n%s", total, strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// /benchmark
// ---------------------------------------------------------------------------

func runBenchmark(repoPath, pkg string) (string, error) {
	args := []string{"test", "-bench=.", "-benchmem", "-run=^$"}
	if pkg != "" {
		args = append(args, pkg)
	} else {
		args = append(args, "./...")
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------------
// /env — show/set environment variables
// ---------------------------------------------------------------------------

func showEnv(filter string) string {
	var lines []string
	for _, e := range os.Environ() {
		if filter == "" || strings.Contains(strings.ToUpper(e), strings.ToUpper(filter)) {
			lines = append(lines, e)
		}
	}
	// Sort
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[j] < lines[i] {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// /paste — paste from clipboard into context
// ---------------------------------------------------------------------------

func pasteFromClipboard() (string, error) {
	var cmd *exec.Cmd
	switch {
	case commandExists("pbpaste"):
		cmd = exec.Command("pbpaste")
	case commandExists("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard", "-out")
	case commandExists("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--output")
	default:
		return "", fmt.Errorf("no clipboard tool found (pbpaste/xclip/xsel)")
	}
	out, err := cmd.Output()
	return string(out), err
}

// ---------------------------------------------------------------------------
// /open — open file in $EDITOR
// ---------------------------------------------------------------------------

func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"nvim", "vim", "nano", "vi"} {
			if commandExists(e) {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found — set $EDITOR")
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /debug — toggle debug logging
// ---------------------------------------------------------------------------

var debugMode bool

// ---------------------------------------------------------------------------
// /squash — squash last N commits into one
// ---------------------------------------------------------------------------

func squashCommits(repoPath string, n int, msg string) error {
	// Soft reset N commits back
	cmd := exec.Command("git", "-C", repoPath, "reset", "--soft", fmt.Sprintf("HEAD~%d", n))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	commitCmd := exec.Command("git", "-C", repoPath, "commit", "-m", msg)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	return commitCmd.Run()
}

// ---------------------------------------------------------------------------
// /journal — view/tail JOURNAL.md
// ---------------------------------------------------------------------------

func viewJournal(repoPath string, lines int) string {
	data, err := os.ReadFile(filepath.Join(repoPath, "JOURNAL.md"))
	if err != nil {
		return "JOURNAL.md not found."
	}
	all := strings.Split(string(data), "\n")
	if lines > 0 && len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

// ---------------------------------------------------------------------------
// /skill-create — scaffold a new skill file
// ---------------------------------------------------------------------------

func scaffoldSkill(repoPath, name, description string) (string, error) {
	dir := filepath.Join(repoPath, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	content := fmt.Sprintf(`---
name: %s
description: %s
tools: [bash, read_file, write_file, edit_file]
---

# %s

## Overview

Describe what this skill does.

## Steps

1. Step one
2. Step two
3. Step three

## Notes

- Add any special considerations here.
`, name, description, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ---------------------------------------------------------------------------
// /amend — amend last commit
// ---------------------------------------------------------------------------

func gitAmend(repoPath, msg string) error {
	args := []string{"-C", repoPath, "commit", "--amend"}
	if msg != "" {
		args = append(args, "-m", msg)
	} else {
		args = append(args, "--no-edit")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /stash-list
// ---------------------------------------------------------------------------

func gitStashList(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "stash", "list").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
// Error helpers for /fix and /explain-error
// ---------------------------------------------------------------------------

func buildFixPrompt(errText string) string {
	return fmt.Sprintf(
		"Fix the following error. Read the relevant files first, then apply the minimal fix. "+
			"Run `go build ./...` to verify.\n\nError:\n```\n%s\n```", errText)
}

func buildExplainErrorPrompt(errText string) string {
	return fmt.Sprintf(
		"Explain this error clearly: what it means, why it happens, and how to fix it.\n\n```\n%s\n```",
		errText)
}

// ---------------------------------------------------------------------------
// /generate-commit — AI-suggested commit message from current diff
// ---------------------------------------------------------------------------

func buildGenerateCommitPrompt(repoPath string) string {
	diff, _ := exec.Command("git", "-C", repoPath, "diff", "--cached").Output()
	if len(strings.TrimSpace(string(diff))) == 0 {
		diff, _ = exec.Command("git", "-C", repoPath, "diff", "HEAD").Output()
	}
	d := strings.TrimSpace(string(diff))
	if d == "" {
		return "No diff found. Stage or make changes first."
	}
	if len(d) > 6000 {
		d = d[:6000] + "\n…[truncated]"
	}
	return fmt.Sprintf(
		"Write a concise git commit message for this diff. "+
			"Format: <type>(<scope>): <summary> — one line, under 72 chars. "+
			"Types: feat/fix/refactor/docs/test/chore. Reply with ONLY the commit message, no explanation.\n\n```diff\n%s\n```", d)
}

// ---------------------------------------------------------------------------
// /release — create a GitHub release
// ---------------------------------------------------------------------------

func buildReleaseNotes(repoPath, from, to string) string {
	args := []string{"-C", repoPath, "log", "--oneline"}
	if from != "" {
		args = append(args, from+".."+to)
	} else {
		args = append(args, "-30")
	}
	out, _ := exec.Command("git", args...).Output()
	log := strings.TrimSpace(string(out))
	if log == "" {
		return "No commits found."
	}
	return fmt.Sprintf(
		"Write release notes for a GitHub release. Group into: ## Features, ## Bug Fixes, ## Other. "+
			"Be user-friendly, not technical. End with installation instructions: `go install github.com/GrayCodeAI/iterate@latest`\n\n```\n%s\n```", log)
}

// ---------------------------------------------------------------------------
// /ci — show GitHub Actions run status
// ---------------------------------------------------------------------------

func getCIStatus(repoPath string) (string, error) {
	out, err := exec.Command("gh", "run", "list", "--limit", "5",
		"--json", "status,conclusion,name,createdAt,url",
		"--template", `{{range .}}{{.status}} {{.conclusion}} {{.name}} {{.createdAt}}{{"\n"}}{{end}}`).Output()
	if err != nil {
		return "", fmt.Errorf("gh run list: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------------------
// /view — syntax-highlighted file viewer
// ---------------------------------------------------------------------------

func viewFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// /verify — run all quality checks
// ---------------------------------------------------------------------------

type verifyResult struct {
	name   string
	ok     bool
	output string
}

func runVerify(repoPath string) []verifyResult {
	type check struct {
		name string
		args []string
	}
	checks := []check{
		{"build", []string{"go", "build", "./..."}},
		{"vet", []string{"go", "vet", "./..."}},
		{"test", []string{"go", "test", "./..."}},
		{"fmt", []string{"gofmt", "-l", "."}},
	}
	var results []verifyResult
	for _, c := range checks {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		detail := strings.TrimSpace(string(out))
		// gofmt: non-empty output means files need formatting
		ok := err == nil
		if c.name == "fmt" {
			ok = detail == ""
			if !ok {
				detail = "needs formatting: " + detail
			} else {
				detail = "all files formatted"
			}
		}
		if len(detail) > 100 {
			detail = detail[:100] + "…"
		}
		results = append(results, verifyResult{c.name, ok, detail})
	}
	return results
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
// /auto-commit — toggle automatic commit after each file write
// ---------------------------------------------------------------------------

var autoCommitEnabled bool

// ---------------------------------------------------------------------------
// /mcp — MCP server config management
// ---------------------------------------------------------------------------

type mcpServer struct {
	Name    string   `json:"name"`
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

func mcpConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".iterate", "mcp.json")
}

func loadMCPServers() []mcpServer {
	data, err := os.ReadFile(mcpConfigPath())
	if err != nil {
		return nil
	}
	var servers []mcpServer
	json.Unmarshal(data, &servers)
	return servers
}

func saveMCPServers(servers []mcpServer) {
	data, _ := json.MarshalIndent(servers, "", "  ")
	_ = os.MkdirAll(filepath.Dir(mcpConfigPath()), 0o755)
	_ = os.WriteFile(mcpConfigPath(), data, 0o644)
}

// ---------------------------------------------------------------------------
// /diagram — ASCII architecture diagram prompt
// ---------------------------------------------------------------------------

func buildDiagramPrompt(repoPath string) string {
	index := buildRepoIndex(repoPath)
	return fmt.Sprintf(
		"Generate an ASCII architecture diagram for this codebase. "+
			"Show: packages, their relationships, data flow, and key interfaces. "+
			"Use ASCII box-drawing characters. Be clear and concise.\n\n"+
			"Repo structure:\n```\n%s\n```", index)
}

// ---------------------------------------------------------------------------
// /generate-readme — AI-generated README
// ---------------------------------------------------------------------------

func buildReadmePrompt(repoPath string) string {
	index := buildRepoIndex(repoPath)
	// Try to get existing README for context
	existing, _ := os.ReadFile(filepath.Join(repoPath, "README.md"))
	base := fmt.Sprintf(
		"Generate a comprehensive README.md for this project. Include: "+
			"title, description, features, installation, usage, configuration, "+
			"and contributing sections. Make it compelling and clear.\n\nRepo:\n```\n%s\n```", index)
	if len(existing) > 0 {
		base += fmt.Sprintf("\n\nExisting README (improve this):\n```\n%s\n```", string(existing))
	}
	return base
}

// ---------------------------------------------------------------------------
// /mock — generate interface mocks prompt
// ---------------------------------------------------------------------------

func buildMockPrompt(filePath string) string {
	return fmt.Sprintf(
		"Read %s and generate Go mock implementations for all interfaces found. "+
			"Use a simple struct-based mock with recorded calls and configurable return values. "+
			"Write mocks to a *_mock_test.go file in the same package.", filePath)
}

// ---------------------------------------------------------------------------
// /pr-checkout — checkout a PR locally
// ---------------------------------------------------------------------------

func prCheckout(repoPath string, prNum string) error {
	cmd := exec.Command("gh", "pr", "checkout", prNum)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// /gist — create a GitHub gist from a file or stdin
// ---------------------------------------------------------------------------

func createGist(content, filename, description string, public bool) (string, error) {
	tmp, err := os.CreateTemp("", "iterate-gist-*."+filepath.Ext(filename))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(content)
	tmp.Close()

	args := []string{"gist", "create", tmp.Name(),
		"--filename", filename,
		"--desc", description}
	if public {
		args = append(args, "--public")
	}
	out, err := exec.Command("gh", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// ---------------------------------------------------------------------------
// /init — scaffold a new iterate-enabled project
// ---------------------------------------------------------------------------

func initProject(repoPath, projectName string) []string {
	files := map[string]string{
		"IDENTITY.md":            fmt.Sprintf("# %s\n\nA self-evolving project powered by iterate.\n", projectName),
		"PERSONALITY.md":         "Helpful, concise, and direct.\n",
		"JOURNAL.md":             fmt.Sprintf("# Journal\n\n## Day 1 — %s\n\nProject initialized.\n", time.Now().Format("2006-01-02")),
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
