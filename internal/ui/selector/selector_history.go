package selector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

// inputHistory holds commands for up/down arrow navigation.
var inputHistory []string
var inputHistoryMu sync.RWMutex

// InputHistoryRef exposes the history slice pointer for external read access.
var InputHistoryRef = &inputHistory
var historyFile string

// InitHistory loads history from the default history file.
func InitHistory() {
	home, _ := os.UserHomeDir()
	historyFile = filepath.Join(home, ".iterate", "history")
	f, err := os.Open(historyFile)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inputHistoryMu.Lock()
	defer inputHistoryMu.Unlock()
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			inputHistory = append(inputHistory, line)
		}
	}
}

const maxHistoryLines = 500

// redactSensitiveInput replaces potential API keys with [redacted] before history storage.
// Handles: /provider <name> <key>
func redactSensitiveInput(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 3 && strings.EqualFold(parts[0], "/provider") {
		return parts[0] + " " + parts[1] + " [redacted]"
	}
	return line
}

func appendHistory(line string) {
	if line == "" {
		return
	}
	line = redactSensitiveInput(line)
	inputHistoryMu.Lock()
	// Avoid duplicate of last entry
	if len(inputHistory) > 0 && inputHistory[len(inputHistory)-1] == line {
		inputHistoryMu.Unlock()
		return
	}
	inputHistory = append(inputHistory, line)
	inputHistoryMu.Unlock()
	// Persist
	if historyFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(historyFile), 0o755) // best-effort cleanup
	f, err := os.OpenFile(historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	fmt.Fprintln(f, line)
	_ = f.Close() // best-effort cleanup

	// Trim history file if it exceeds maxHistoryLines
	trimHistoryFile()
}

// trimHistoryFile keeps the history file at most maxHistoryLines lines.
func trimHistoryFile() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= maxHistoryLines {
		return
	}
	lines = lines[len(lines)-maxHistoryLines:]
	_ = os.WriteFile(historyFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600) // best-effort cleanup
}

func getInputHistory() []string {
	inputHistoryMu.RLock()
	defer inputHistoryMu.RUnlock()
	cp := make([]string, len(inputHistory))
	copy(cp, inputHistory)
	return cp
}

// GetHistory returns a copy of the current input history (exported for callers).
func GetHistory() []string {
	return getInputHistory()
}

func inputHistoryLen() int {
	inputHistoryMu.RLock()
	defer inputHistoryMu.RUnlock()
	return len(inputHistory)
}

func inputHistoryAt(i int) string {
	inputHistoryMu.RLock()
	defer inputHistoryMu.RUnlock()
	return inputHistory[i]
}

// fuzzyHistorySearch shows an interactive filtered history picker (Ctrl+R style).
// Returns the chosen entry or "" if cancelled.
func fuzzyHistorySearch() string {
	hist := getInputHistory()
	if len(hist) == 0 {
		return ""
	}
	unique := deduplicateHistory(hist)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, oldState)

	query := []byte{}
	cursor := 0

	filtered := func() []string {
		return filterHistoryEntries(unique, string(query))
	}

	draw := func(first bool) {
		drawHistoryUI(filtered(), cursor, query, first)
	}

	draw(true)
	b := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			return ""
		}
		items := filtered()
		done, result := handleHistoryKeyInput(b, n, items, &query, &cursor, draw)
		if done {
			return result
		}
	}
}

// deduplicateHistory returns history entries in reverse order (most recent first), deduplicated.
func deduplicateHistory(hist []string) []string {
	seen := map[string]bool{}
	var unique []string
	for i := len(hist) - 1; i >= 0; i-- {
		if !seen[hist[i]] {
			seen[hist[i]] = true
			unique = append(unique, hist[i])
		}
	}
	return unique
}

// filterHistoryEntries returns entries matching the query (case-insensitive substring).
func filterHistoryEntries(entries []string, query string) []string {
	if query == "" {
		return entries
	}
	q := strings.ToLower(query)
	var out []string
	for _, h := range entries {
		if strings.Contains(strings.ToLower(h), q) {
			out = append(out, h)
		}
	}
	return out
}

// drawHistoryUI renders the fuzzy history search interface.
func drawHistoryUI(items []string, cursor int, query []byte, first bool) {
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
		ui.ColorDim, ui.ColorReset, ui.ColorYellow, string(query), ui.ColorReset)
	for i := 0; i < visible; i++ {
		if i == cursor {
			fmt.Printf(" %s›%s %s%s%s\r\n", ui.ColorLime+ui.ColorBold, ui.ColorReset, ui.ColorBold, items[i], ui.ColorReset)
		} else {
			fmt.Printf("   %s%s%s\r\n", ui.ColorDim, items[i], ui.ColorReset)
		}
	}
	if len(items) > maxVisible {
		fmt.Printf(" %s%d/%d%s\r\n", ui.ColorDim, visible, len(items), ui.ColorReset)
	}
}

// handleHistoryKeyInput processes a keypress in the history search and returns (done, result).
func handleHistoryKeyInput(b []byte, n int, items []string, query *[]byte, cursor *int, draw func(bool)) (done bool, result string) {
	switch {
	case b[0] == '\r' || b[0] == '\n':
		clearHistoryMenu(items)
		if *cursor < len(items) {
			return true, items[*cursor]
		}
		return true, ""
	case b[0] == 3 || b[0] == 18:
		clearHistoryMenu(items)
		return true, ""
	case b[0] == 27 && n >= 3 && b[1] == '[':
		switch b[2] {
		case 'A':
			if *cursor > 0 {
				*cursor--
			}
		case 'B':
			if *cursor < len(items)-1 {
				*cursor++
			}
		}
		draw(false)
	case b[0] == 127 || b[0] == 8:
		if len(*query) > 0 {
			*query = (*query)[:len(*query)-1]
			*cursor = 0
			draw(false)
		}
	default:
		if b[0] >= 32 {
			*query = append(*query, b[:n]...)
			*cursor = 0
			draw(false)
		}
	}
	return false, ""
}

// clearHistoryMenu clears the history search UI from the terminal.
func clearHistoryMenu(items []string) {
	lines := len(items)
	if len(items) > maxVisible {
		lines = maxVisible + 1
	}
	fmt.Printf("\033[%dA\033[J", lines+1)
}
