package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

// printPrompt prints just the input glyph — clean, no decorations.
func printPrompt() {
	switch currentMode {
	case modeAsk:
		fmt.Printf("%s[ask] ❯%s ", colorCyan, colorReset)
	case modeArchitect:
		fmt.Printf("%s[arch] ❯%s ", colorPurple, colorReset)
	default:
		fmt.Printf("%s❯%s ", colorLime, colorReset)
	}
}

// gitStatus returns real-time staged and unstaged file counts.
// Uses git diff directly to avoid stale index mtime false-positives.
func gitStatus() (staged, unstaged int) {
	if replRepoPath == "" {
		return 0, 0
	}
	// Count staged files (index vs HEAD)
	out, err := exec.Command("git", "-C", replRepoPath, "diff", "--cached", "--name-only").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				staged++
			}
		}
	}
	// Count unstaged files (working tree vs index)
	out, err = exec.Command("git", "-C", replRepoPath, "diff", "--name-only").Output()
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

// printStatusLine prints the one-line status shown after each agent response.
// elapsed · model · tokens · ctx% (only >10%) · git dirty
func printStatusLine(elapsed time.Duration) {
	model := os.Getenv("ITERATE_MODEL")
	if model == "" {
		model = os.Getenv("ITERATE_PROVIDER")
	}
	total := sessionInputTokens + sessionOutputTokens

	fmt.Printf("%s●%s %s%s%s",
		colorCyan, colorReset,
		colorCyan, formatElapsed(elapsed), colorReset)

	if model != "" {
		fmt.Printf("%s · %s%s", colorDim, model, colorReset)
	}

	if total >= 1000 {
		fmt.Printf("%s · %s%.1fk tok%s", colorDim, colorPurple, float64(total)/1000, colorReset)
	} else if total > 0 {
		fmt.Printf("%s · %s%d tok%s", colorDim, colorPurple, total, colorReset)
	}

	const windowTokens = 200_000
	pct := 0
	if total > 0 {
		pct = total * 100 / windowTokens
		if pct > 100 {
			pct = 100
		}
	}
	ctxColor := colorBlue
	if pct > 75 {
		ctxColor = colorYellow
	}
	if pct > 90 {
		ctxColor = colorRed
	}
	fmt.Printf("%s · %sctx %.1f%%%s", colorDim, ctxColor, float64(total)*100/float64(windowTokens), colorReset)

	if staged, unstaged := gitStatus(); staged+unstaged > 0 {
		if staged > 0 && unstaged > 0 {
			fmt.Printf("%s · %s+%d staged%s %s±%d unstaged%s", colorDim, colorGreen, staged, colorReset, colorYellow, unstaged, colorReset)
		} else if staged > 0 {
			fmt.Printf("%s · %s+%d staged%s", colorDim, colorGreen, staged, colorReset)
		} else {
			fmt.Printf("%s · %s±%d unstaged%s", colorDim, colorYellow, unstaged, colorReset)
		}
	}

	if safeMode {
		fmt.Printf("%s · %s🔒 safe%s", colorDim, colorCyan, colorReset)
	}

	fmt.Println()
}

// inputHistory holds commands for up/down arrow navigation.
var inputHistory []string
var historyFile string

func initHistory() {
	home, _ := os.UserHomeDir()
	historyFile = filepath.Join(home, ".iterate", "history")
	f, err := os.Open(historyFile)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			inputHistory = append(inputHistory, line)
		}
	}
}

func appendHistory(line string) {
	if line == "" {
		return
	}
	// Avoid duplicate of last entry
	if len(inputHistory) > 0 && inputHistory[len(inputHistory)-1] == line {
		return
	}
	inputHistory = append(inputHistory, line)
	// Persist
	if historyFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(historyFile), 0o755)
	f, err := os.OpenFile(historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
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

// tabCompleteWithArgs completes both slash commands and their arguments.
func tabCompleteWithArgs(partial string) string {
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
			return completeFilePath(partial)
		}
		return partial
	}
	return tabComplete(partial)
}

// tabComplete returns the longest unique completion for a partial slash command.
func tabComplete(partial string) string {
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

// completeFilePath returns file path completion suggestions.
func completeFilePath(partial string) string {
	// Extract the file path part (last space-separated token)
	parts := strings.Fields(partial)
	if len(parts) == 0 {
		return partial
	}

	pathPart := parts[len(parts)-1]

	// Handle paths
	dir := filepath.Dir(pathPart)
	base := filepath.Base(pathPart)

	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return partial
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

	if len(matches) == 0 {
		return partial
	}

	if len(matches) == 1 {
		// Replace the path part with the completion
		result := strings.Join(parts[:len(parts)-1], " ")
		if result != "" {
			result += " "
		}
		result += matches[0]
		return result
	}

	// Find common prefix
	prefix := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}

	// Return with prefix of path part
	result := strings.Join(parts[:len(parts)-1], " ")
	if result != "" {
		result += " "
	}
	result += prefix
	return result
}

// selectItem shows an arrow-key navigable list and returns the selected item.
// Returns ("", false) if cancelled.
func selectItem(title string, items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// fallback: just return first item
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

		fmt.Printf("%s%s%s\r\n", colorYellow+colorBold, title, colorReset)

		for i := offset; i < offset+height; i++ {
			if i == cursor {
				fmt.Printf(" %s›%s %s%s%s\r\n", colorLime+colorBold, colorReset, colorBold, items[i], colorReset)
			} else {
				fmt.Printf("   %s%s%s\r\n", colorDim, items[i], colorReset)
			}
		}

		if len(items) > maxVisible {
			showing := offset + height
			fmt.Printf(" %s↑↓ scroll · %d/%d%s\r\n", colorDim, showing, len(items), colorReset)
		}
	}

	drawMenu(true)

	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return "", false
		}

		switch {
		case buf[0] == '\r' || buf[0] == '\n':
			lines := height + 1
			if len(items) > maxVisible {
				lines++
			}
			fmt.Printf("\033[%dA\033[J", lines)
			fmt.Printf(" %s›%s %s\r\n\r\n", colorLime+colorBold, colorReset, items[cursor])
			return items[cursor], true

		case buf[0] == 3 || (buf[0] == 27 && n == 1): // Ctrl+C or bare ESC
			lines := height + 1
			if len(items) > maxVisible {
				lines++
			}
			fmt.Printf("\033[%dA\033[J", lines)
			return "", false

		case n >= 3 && buf[0] == 27 && buf[1] == '[':
			switch buf[2] {
			case 'A': // up
				if cursor > 0 {
					cursor--
					if cursor < offset {
						offset = cursor
					}
				}
			case 'B': // down
				if cursor < len(items)-1 {
					cursor++
					if cursor >= offset+height {
						offset = cursor - height + 1
					}
				}
			}
			drawMenu(false)
		}
	}
}

// readInput reads user input in raw mode.
// Enter submits. Shift+Enter adds a newline. Up/Down arrow navigates history.
// Returns (text, true) or ("", false) on Ctrl+C/EOF.
func readInput() (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// fallback to simple line read
		printPrompt()
		var line string
		fmt.Scanln(&line)
		return strings.TrimSpace(line), true
	}
	defer term.Restore(fd, oldState)

	printPrompt()

	var buf []byte
	b := make([]byte, 4)
	// histIdx: index into history for up/down; len(inputHistory) = "current line"
	histIdx := len(inputHistory)
	savedBuf := []byte(nil) // saved draft when navigating history

	replacePrompt := func(newText string) {
		// Erase current content and reprint
		for range buf {
			fmt.Print("\b \b")
		}
		buf = []byte(newText)
		fmt.Print(newText)
	}

	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			return "", false
		}

		switch {
		case b[0] == '\r' || b[0] == '\n':
			// Enter — submit (or continue if line ends with \)
			fmt.Print("\r\n")
			result := string(buf)
			if strings.HasSuffix(strings.TrimRight(result, " "), "\\") {
				// Backslash continuation: strip trailing \ and prompt for more
				result = strings.TrimRight(result, " ")
				result = result[:len(result)-1] + "\n"
				buf = []byte(result)
				fmt.Printf("%s  ...%s ", colorDim, colorReset)
				continue
			}
			result = strings.TrimSpace(result)
			appendHistory(result)
			return result, true

		case b[0] == 3: // Ctrl+C
			fmt.Print("\r\n")
			return "", false

		case b[0] == 18: // Ctrl+R — fuzzy history search
			term.Restore(fd, oldState)
			chosen := fuzzyHistorySearch()
			term.MakeRaw(fd) //nolint
			if chosen != "" {
				replacePrompt(chosen)
				histIdx = len(inputHistory)
			} else {
				// Reprint prompt and current buf
				fmt.Printf("\r")
				printPrompt()
				fmt.Printf("%s", string(buf))
			}

		case b[0] == 4: // Ctrl+D EOF
			fmt.Print("\r\n")
			return "", false

		case n >= 2 && b[0] == 13 && b[1] == 10: // Shift+Enter on some terminals
			buf = append(buf, '\n')
			fmt.Print("\r\n")

		case b[0] == 27 && n >= 3 && b[1] == '[':
			switch b[2] {
			case 'A': // Up arrow — go back in history
				if histIdx == len(inputHistory) {
					savedBuf = append([]byte(nil), buf...) // save current draft
				}
				if histIdx > 0 {
					histIdx--
					replacePrompt(inputHistory[histIdx])
				}
			case 'B': // Down arrow — go forward in history
				if histIdx < len(inputHistory) {
					histIdx++
					if histIdx == len(inputHistory) {
						replacePrompt(string(savedBuf))
					} else {
						replacePrompt(inputHistory[histIdx])
					}
				}
			}
		case b[0] == 27 && n == 1:
			// bare ESC — ignore

		case b[0] == '\t': // Tab — autocomplete slash commands, arguments, or file paths
			current := string(buf)
			var completion string
			if strings.HasPrefix(current, "/") {
				completion = tabCompleteWithArgs(current)
			} else {
				completion = completeFilePath(current)
			}
			if completion != "" && completion != current {
				// Erase current input and print completion
				for range buf {
					fmt.Print("\b \b")
				}
				buf = []byte(completion)
				fmt.Printf("%s", completion)
			}
		case b[0] == 127 || b[0] == 8: // backspace
			if len(buf) > 0 {
				// Handle multi-byte UTF-8 backspace
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}

		default:
			if b[0] >= 32 {
				buf = append(buf, b[:n]...)
				fmt.Printf("%s", string(b[:n]))
			}
		}
	}
}

// promptLine shows a prompt and reads a line of input using raw mode char-by-char.
func promptLine(prompt string) (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", false
	}
	defer term.Restore(fd, oldState)

	fmt.Printf("%s%s%s ", colorDim, prompt, colorReset)

	var buf []byte
	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			return "", false
		}
		switch b[0] {
		case '\r', '\n':
			fmt.Print("\r\n")
			return string(buf), true
		case 3: // Ctrl+C
			fmt.Print("\r\n")
			return "", false
		case 127, 8: // backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}
		case 27: // ESC — read and discard the rest of the sequence
			os.Stdin.Read(make([]byte, 2))
		default:
			if b[0] >= 32 {
				buf = append(buf, b[0])
				fmt.Printf("%s", string(b[0]))
			}
		}
	}
}

// fuzzyHistorySearch shows an interactive filtered history picker (Ctrl+R style).
// Returns the chosen entry or "" if cancelled.
func fuzzyHistorySearch() string {
	if len(inputHistory) == 0 {
		return ""
	}
	// Deduplicate and reverse (most recent first)
	seen := map[string]bool{}
	var unique []string
	for i := len(inputHistory) - 1; i >= 0; i-- {
		if !seen[inputHistory[i]] {
			seen[inputHistory[i]] = true
			unique = append(unique, inputHistory[i])
		}
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, oldState)

	query := []byte{}
	cursor := 0

	filtered := func() []string {
		if len(query) == 0 {
			return unique
		}
		q := strings.ToLower(string(query))
		var out []string
		for _, h := range unique {
			if strings.Contains(strings.ToLower(h), q) {
				out = append(out, h)
			}
		}
		return out
	}

	draw := func(first bool) {
		items := filtered()
		visible := maxVisible
		if len(items) < visible {
			visible = len(items)
		}
		if cursor >= len(items) {
			cursor = 0
		}

		if !first {
			lines := visible + 1
			if len(items) > maxVisible {
				lines++
			}
			fmt.Printf("\033[%dA\033[J", lines)
		}
		fmt.Printf("%s ctrl+r %s %s%s%s\r\n",
			colorDim, colorReset, colorYellow, string(query), colorReset)
		for i := 0; i < visible; i++ {
			if i == cursor {
				fmt.Printf(" %s›%s %s%s%s\r\n", colorLime+colorBold, colorReset, colorBold, items[i], colorReset)
			} else {
				fmt.Printf("   %s%s%s\r\n", colorDim, items[i], colorReset)
			}
		}
		if len(items) > maxVisible {
			fmt.Printf(" %s%d/%d%s\r\n", colorDim, visible, len(items), colorReset)
		}
	}

	draw(true)
	b := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			return ""
		}
		items := filtered()
		switch {
		case b[0] == '\r' || b[0] == '\n':
			// Clear menu
			lines := len(items)
			if len(items) > maxVisible {
				lines = maxVisible + 1
			}
			fmt.Printf("\033[%dA\033[J", lines+1)
			if cursor < len(items) {
				return items[cursor]
			}
			return ""
		case b[0] == 3 || b[0] == 18: // Ctrl+C or Ctrl+R again — cancel
			lines := len(items)
			if len(items) > maxVisible {
				lines = maxVisible + 1
			}
			fmt.Printf("\033[%dA\033[J", lines+1)
			return ""
		case b[0] == 27 && n >= 3 && b[1] == '[':
			switch b[2] {
			case 'A': // up
				if cursor > 0 {
					cursor--
				}
			case 'B': // down
				if cursor < len(items)-1 {
					cursor++
				}
			}
			draw(false)
		case b[0] == 127 || b[0] == 8: // backspace
			if len(query) > 0 {
				query = query[:len(query)-1]
				cursor = 0
				draw(false)
			}
		default:
			if b[0] >= 32 {
				query = append(query, b[:n]...)
				cursor = 0
				draw(false)
			}
		}
	}
}
