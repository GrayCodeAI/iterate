// Package selector provides fuzzy-search and arrow-key selection UI.
package selector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

// Mode constants for prompt display.
const (
	ModeNormal    = 0
	ModeAsk       = 1
	ModeArchitect = 2
)

// Package-level state set by the caller.
var (
	CurrentMode    int
	RepoPath       string
	SafeMode       bool
	InputTokens    int
	OutputTokens   int
	TTFT           time.Duration
	ContextWindow  int     // provider context window in tokens; 0 = use default 200k
	RequestCostUSD float64 // cost of the last request in USD; 0 = unknown
)

// Additional colors used in selector UI.
var (
	colorGreen  = "\033[38;5;114m"
	colorPurple = "\033[38;5;141m"
	colorBlue   = "\033[38;5;75m"
)

// PrintPrompt prints just the input glyph — clean, no decorations.
func PrintPrompt() {
	switch CurrentMode {
	case ModeAsk:
		fmt.Printf("%s[ask] ❯%s ", ui.ColorCyan, ui.ColorReset)
	case ModeArchitect:
		fmt.Printf("%s[arch] ❯%s ", colorPurple, ui.ColorReset)
	default:
		fmt.Printf("%s❯%s ", ui.ColorLime, ui.ColorReset)
	}
}

// GitStatus returns real-time staged and unstaged file counts.
// Uses git diff directly to avoid stale index mtime false-positives.
func GitStatus() (staged, unstaged int) {
	return gitStatus()
}

// gitStatus is the internal implementation.
func gitStatus() (staged, unstaged int) {
	if RepoPath == "" {
		return 0, 0
	}
	// Count staged files (index vs HEAD)
	out, err := exec.Command("git", "-C", RepoPath, "diff", "--cached", "--name-only").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				staged++
			}
		}
	}
	// Count unstaged files (working tree vs index)
	out, err = exec.Command("git", "-C", RepoPath, "diff", "--name-only").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				unstaged++
			}
		}
	}
	return staged, unstaged
}

// formatElapsed formats a duration cleanly: "5.8s", "1m2s", "320ms"
func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

// formatTokenCount formats total session token count for display.
func formatTokenCount(total int) string {
	if total >= 1000 {
		return fmt.Sprintf("%s · %s%.1fk tok%s", ui.ColorDim, colorPurple, float64(total)/1000, ui.ColorReset)
	}
	if total > 0 {
		return fmt.Sprintf("%s · %s%d tok%s", ui.ColorDim, colorPurple, total, ui.ColorReset)
	}
	return ""
}

func formatContextWindow(total int) string {
	windowTokens := ContextWindow
	if windowTokens <= 0 {
		windowTokens = 200_000
	}
	ctxColor := colorBlue
	ratio := float64(total) * 100 / float64(windowTokens)
	if ratio > 75 {
		ctxColor = ui.ColorYellow
	}
	if ratio > 90 {
		ctxColor = ui.ColorRed
	}
	return fmt.Sprintf("%s · %sctx %.1f%%%s", ui.ColorDim, ctxColor, ratio, ui.ColorReset)
}

// formatCostUSD formats a USD cost for status line display.
func formatCostUSD(usd float64) string {
	if usd <= 0 {
		return ""
	}
	if usd < 0.0001 {
		return "<$0.0001"
	}
	if usd < 0.001 {
		return fmt.Sprintf("$%.4f", usd)
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.3f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

func formatGitStatus() string {
	staged, unstaged := gitStatus()
	if staged+unstaged == 0 {
		return ""
	}
	if staged > 0 && unstaged > 0 {
		return fmt.Sprintf("%s · %s+%d staged%s %s±%d unstaged%s", ui.ColorDim, colorGreen, staged, ui.ColorReset, ui.ColorYellow, unstaged, ui.ColorReset)
	}
	if staged > 0 {
		return fmt.Sprintf("%s · %s+%d staged%s", ui.ColorDim, colorGreen, staged, ui.ColorReset)
	}
	return fmt.Sprintf("%s · %s±%d unstaged%s", ui.ColorDim, ui.ColorYellow, unstaged, ui.ColorReset)
}

// PrintStatusLine prints the one-line status shown after each agent response.
// tokenDelta is the number of tokens used in this request (shown as +N); pass
// 0 to omit the delta.
func PrintStatusLine(elapsed time.Duration, tokenDelta int) {
	model := os.Getenv("ITERATE_MODEL")
	if model == "" {
		model = os.Getenv("ITERATE_PROVIDER")
	}
	total := InputTokens + OutputTokens

	fmt.Printf("%s●%s %s%s%s",
		ui.ColorCyan, ui.ColorReset,
		ui.ColorCyan, formatElapsed(elapsed), ui.ColorReset)

	if model != "" {
		fmt.Printf("%s · %s%s", ui.ColorDim, model, ui.ColorReset)
	}

	if tokenDelta > 0 {
		fmt.Printf("%s · %s+%d tok%s", ui.ColorDim, colorPurple, tokenDelta, ui.ColorReset)
	}
	if TTFT > 0 {
		fmt.Printf("%s · %sttft %s%s", ui.ColorDim, colorBlue, formatElapsed(TTFT), ui.ColorReset)
	}
	if RequestCostUSD > 0 {
		fmt.Printf("%s · %s%s%s", ui.ColorDim, colorGreen, formatCostUSD(RequestCostUSD), ui.ColorReset)
	}
	fmt.Print(formatContextWindow(total))

	if gs := formatGitStatus(); gs != "" {
		fmt.Print(gs)
	}

	if SafeMode {
		fmt.Printf("%s · %s🔒 safe%s", ui.ColorDim, ui.ColorCyan, ui.ColorReset)
	}

	fmt.Println()
}

const maxVisible = 12

var slashCommands = []string{
	"/help", "/clear", "/compact", "/tools", "/skills", "/thinking", "/model",
	// agent modes
	"/code", "/ask", "/architect", "/summarize", "/review", "/explain",
	"/docs", "/test-gen", "/refactor", "/changelog",
	// context
	"/context", "/rewind", "/fork", "/inject", "/set", "/pin", "/unpin",
	"/multi",
	// config & themes
	"/cost", "/stats", "/config", "/theme", "/notify",
	"/safe", "/trust", "/allow", "/deny",
	// aliases
	"/alias", "/aliases",
	// files & search
	"/add", "/find", "/web", "/grep", "/todos", "/deps", "/search-replace",
	// sessions & memory
	"/save", "/load", "/export", "/bookmark", "/bookmarks", "/history",
	"/copy", "/retry", "/memo", "/learn", "/memories",
	// templates
	"/template", "/templates", "/save-template",
	// git local
	"/diff", "/diff-staged", "/log", "/status", "/branch", "/checkout",
	"/stash", "/stash-list", "/merge", "/tag", "/revert-file",
	"/undo", "/commit", "/amend", "/squash", "/clean",
	"/blame", "/show", "/cherry-pick",
	// git network
	"/fetch", "/pull", "/push", "/rebase", "/remote",
	// repo insights
	"/count-lines", "/hotspots", "/contributors", "/languages",
	// context
	"/forget", "/compact-hard", "/pin-list",
	// dev tools
	"/benchmark", "/env", "/debug",
	// clipboard & file ops
	"/paste", "/open", "/pwd", "/cd",
	// project workflow
	"/journal", "/skill-create", "/self-improve", "/evolve-now",
	// error helpers
	"/fix", "/explain-error", "/optimize", "/security",
	// code quality
	"/test", "/test-file", "/test-gen", "/build", "/lint", "/lint-fix",
	"/format", "/coverage", "/doctor",
	// GitHub & PRs
	"/pr", "/issues",
	// project tooling (new)
	"/health", "/tree", "/index", "/pkgdoc", "/iterate-init",
	// provider & version (new)
	"/provider", "/version",
	// marks (new)
	"/mark", "/marks", "/jump",
	// per-project memory (new)
	"/remember",
	// git passthrough (new)
	"/git",
	// session changes (new)
	"/changes",
	// token/cost (new)
	"/tokens",
	// dev
	"/watch", "/run", "/plan", "/phase", "/quit",
	// ai-assisted generation
	"/generate-commit", "/release", "/diagram", "/generate-readme", "/mock",
	// ci / verification
	"/ci", "/verify",
	// file ops
	"/view", "/snapshot", "/snapshots",
	// pair programming & auto-commit
	"/pair", "/auto-commit",
	// MCP management
	"/mcp-add", "/mcp-list", "/mcp-remove",
	// GitHub advanced
	"/pr-checkout", "/gist",
	// project init
	"/init",
	// search & multi-agent
	"/search", "/spawn",
}

// commandArgCompletions maps commands to their known argument completions.
var commandArgCompletions = map[string][]string{
	"/thinking": {"off", "minimal", "low", "medium", "high"},
	"/provider": {"anthropic", "openai", "gemini", "groq", "ollama", "azure", "bedrock", "vertex", "mistral", "deepseek"},
	"/theme":    {"default", "nord", "monokai", "minimal"},
	"/pr":       {"list", "view", "diff", "review", "comment", "checkout", "create"},
	"/git":      {"status", "log", "diff", "add", "commit", "push", "pull", "branch", "stash", "rebase", "fetch"},
	"/phase":    {"plan", "implement", "communicate"},
	"/set":      {"temperature", "max_tokens", "reset"},
}

// TabCompleteWithArgs completes both slash commands and their arguments.
func TabCompleteWithArgs(partial string) string {
	// If there's a space, we're completing an argument
	spaceIdx := strings.Index(partial, " ")
	if spaceIdx >= 0 {
		cmd := partial[:spaceIdx]
		argPartial := partial[spaceIdx+1:]
		if completions, ok := commandArgCompletions[cmd]; ok {
			var matches []string
			for _, c := range completions {
				if strings.HasPrefix(c, argPartial) {
					matches = append(matches, c)
				}
			}
			if len(matches) == 1 {
				return cmd + " " + matches[0] + " "
			}
			if len(matches) > 1 {
				prefix := matches[0]
				for _, m := range matches[1:] {
					for !strings.HasPrefix(m, prefix) {
						prefix = prefix[:len(prefix)-1]
					}
				}
				return cmd + " " + prefix
			}
		}
		// Fall through to file path completion for commands that take file args
		if cmd == "/add" || cmd == "/open" || cmd == "/view" || cmd == "/mock" {
			return CompleteFilePath(partial)
		}
		return partial
	}
	return TabComplete(partial)
}

// TabComplete returns the longest unique completion for a partial slash command.
func TabComplete(partial string) string {
	var matches []string
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, partial) {
			matches = append(matches, cmd)
		}
	}
	if len(matches) == 0 {
		return partial
	}
	if len(matches) == 1 {
		return matches[0] + " "
	}
	// Find common prefix among all matches
	prefix := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

// CompleteFilePath returns file path completion suggestions.
func CompleteFilePath(partial string) string {
	parts := strings.Fields(partial)
	if len(parts) == 0 {
		return partial
	}

	pathPart := parts[len(parts)-1]
	matches := findPathMatches(pathPart)
	if len(matches) == 0 {
		return partial
	}

	prefix := parts[:len(parts)-1]
	return buildCompletionResult(prefix, matches)
}

// findPathMatches scans the directory for entries matching the partial path.
func findPathMatches(pathPart string) []string {
	dir := filepath.Dir(pathPart)
	base := filepath.Base(pathPart)
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base) {
			fullPath := filepath.Join(dir, e.Name())
			if e.IsDir() {
				fullPath += "/"
			}
			matches = append(matches, fullPath)
		}
	}
	return matches
}

// buildCompletionResult joins prefix words with the single match or common prefix of multiple matches.
func buildCompletionResult(prefix []string, matches []string) string {
	result := strings.Join(prefix, " ")
	if result != "" {
		result += " "
	}
	if len(matches) == 1 {
		return result + matches[0]
	}
	common := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, common) {
			common = common[:len(common)-1]
		}
	}
	return result + common
}

// SelectItem shows an arrow-key navigable list and returns the selected item.
// Returns ("", false) if cancelled.
func SelectItem(title string, items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return items[0], true
	}
	defer term.Restore(fd, oldState)

	cursor := 0
	offset := 0
	height := maxVisible
	if len(items) < height {
		height = len(items)
	}

	drawMenu := func(first bool) {
		if !first {
			lines := height + 1
			if len(items) > maxVisible {
				lines++
			}
			fmt.Printf("\033[%dA\033[J", lines)
		}

		fmt.Printf("%s%s%s\r\n", ui.ColorYellow+ui.ColorBold, title, ui.ColorReset)

		for i := offset; i < offset+height; i++ {
			if i == cursor {
				fmt.Printf(" %s›%s %s%s%s\r\n", ui.ColorLime+ui.ColorBold, ui.ColorReset, ui.ColorBold, items[i], ui.ColorReset)
			} else {
				fmt.Printf("   %s%s%s\r\n", ui.ColorDim, items[i], ui.ColorReset)
			}
		}

		if len(items) > maxVisible {
			showing := offset + height
			fmt.Printf(" %s↑↓ scroll · %d/%d%s\r\n", ui.ColorDim, showing, len(items), ui.ColorReset)
		}
	}

	drawMenu(true)

	return handleSelectInput(items, &cursor, &offset, height, drawMenu)
}

func handleSelectInput(items []string, cursor, offset *int, height int, drawMenu func(bool)) (string, bool) {
	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", false
		}

		switch {
		case buf[0] == '\r' || buf[0] == '\n':
			cleanupSelectUI(height, len(items))
			fmt.Printf(" %s›%s %s\r\n\r\n", ui.ColorLime+ui.ColorBold, ui.ColorReset, items[*cursor])
			return items[*cursor], true

		case buf[0] == 3 || (buf[0] == 27 && n == 1):
			cleanupSelectUI(height, len(items))
			return "", false

		case n >= 3 && buf[0] == 27 && buf[1] == '[':
			switch buf[2] {
			case 'A':
				if *cursor > 0 {
					*cursor--
					if *cursor < *offset {
						*offset = *cursor
					}
				}
			case 'B':
				if *cursor < len(items)-1 {
					*cursor++
					if *cursor >= *offset+height {
						*offset = *cursor - height + 1
					}
				}
			}
			drawMenu(false)
		}
	}
}

func cleanupSelectUI(height, itemCount int) {
	lines := height + 1
	if itemCount > maxVisible {
		lines++
	}
	fmt.Printf("\033[%dA\033[J", lines)
}
