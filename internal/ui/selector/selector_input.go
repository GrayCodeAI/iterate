package selector

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

// ReadInput reads user input in raw mode.
// Enter submits. Shift+Enter adds a newline. Up/Down arrow navigates history.
// Returns (text, true) or ("", false) on Ctrl+C/EOF.
func ReadInput() (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// fallback to simple line read
		PrintPrompt()
		var line string
		fmt.Scanln(&line)
		return strings.TrimSpace(line), true
	}
	defer term.Restore(fd, oldState)

	PrintPrompt()

	var buf []byte
	b := make([]byte, 4)
	histSnapshot := getInputHistory()
	histIdx := len(histSnapshot)
	savedBuf := []byte(nil)

	replacePrompt := func(newText string) {
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

		done, result, ok := handleRawInput(b, n, &buf, &histSnapshot, &histIdx, &savedBuf, replacePrompt, fd, oldState)
		if done {
			return result, ok
		}
	}
}

// handleRawInput processes a single raw-mode key event and returns (done, text, ok).
func handleRawInput(b []byte, n int, buf *[]byte, histSnapshot *[]string, histIdx *int, savedBuf *[]byte, replacePrompt func(string), fd int, oldState *term.State) (done bool, result string, ok bool) {
	switch {
	case b[0] == '\r' || b[0] == '\n':
		return handleLineSubmit(buf, histSnapshot, histIdx, savedBuf)

	case b[0] == 3:
		// Two-stage Ctrl+C: first press clears the line, second press exits.
		if len(*buf) > 0 {
			// Clear the current line and reprint the prompt.
			for range *buf {
				fmt.Print("\b \b")
			}
			*buf = (*buf)[:0]
			return false, "", false
		}
		fmt.Print("\r\n")
		return true, "", false

	case b[0] == 18:
		return handleFuzzyHistorySearch(buf, histSnapshot, histIdx, replacePrompt, fd, oldState)

	case b[0] == 4:
		fmt.Print("\r\n")
		return true, "", false

	case n >= 2 && b[0] == 13 && b[1] == 10:
		*buf = append(*buf, '\n')
		fmt.Print("\r\n")
		return false, "", false

	case b[0] == 27 && n >= 3 && b[1] == '[':
		handleArrowKeys(b[2], buf, histSnapshot, histIdx, savedBuf, replacePrompt)
		return false, "", false

	case b[0] == 27 && n == 1:
		return false, "", false

	case b[0] == '\t':
		handleTabCompletion(buf)
		return false, "", false

	case b[0] == 127 || b[0] == 8:
		// Backspace: delete one character.
		if len(*buf) > 0 {
			*buf = (*buf)[:len(*buf)-1]
			fmt.Print("\b \b")
		}
		return false, "", false

	case b[0] == 23:
		// Ctrl+W: kill word backward (delete back to last whitespace boundary).
		if len(*buf) > 0 {
			end := len(*buf)
			// Skip trailing spaces.
			i := end - 1
			for i >= 0 && (*buf)[i] == ' ' {
				i--
			}
			// Delete back to the next space.
			for i >= 0 && (*buf)[i] != ' ' {
				i--
			}
			deleted := end - (i + 1)
			for range deleted {
				fmt.Print("\b \b")
			}
			*buf = (*buf)[:i+1]
		}
		return false, "", false

	case b[0] == 21:
		// Ctrl+U: kill to beginning of line.
		for range *buf {
			fmt.Print("\b \b")
		}
		*buf = (*buf)[:0]
		return false, "", false

	case b[0] == 11:
		// Ctrl+K: kill to end of line (at end of buf — nothing visible to erase in terminal).
		// Since we display sequentially, Ctrl+K is a no-op here unless we track cursor.
		// For now, clear from cursor to end by reprinting spaces; simple: just clear all.
		// Future: add cursor position tracking. For now, noop.
		return false, "", false

	case b[0] == 1:
		// Ctrl+A: move to beginning of line (visual only — reprint prompt).
		// Without cursor tracking we can't move cursor. No-op for now.
		return false, "", false

	case b[0] == 5:
		// Ctrl+E: move to end of line. No-op without cursor tracking.
		return false, "", false

	default:
		if b[0] >= 32 {
			*buf = append(*buf, b[:n]...)
			fmt.Printf("%s", string(b[:n]))
		}
		return false, "", false
	}
}

// handleLineSubmit processes Enter key, handling backslash continuation.
func handleLineSubmit(buf *[]byte, histSnapshot *[]string, histIdx *int, savedBuf *[]byte) (done bool, result string, ok bool) {
	fmt.Print("\r\n")
	text := string(*buf)
	if strings.HasSuffix(strings.TrimRight(text, " "), "\\") {
		text = strings.TrimRight(text, " ")
		text = text[:len(text)-1] + "\n"
		*buf = []byte(text)
		fmt.Printf("%s  ...%s ", ui.ColorDim, ui.ColorReset)
		return false, "", false
	}
	text = strings.TrimSpace(text)
	appendHistory(text)
	return true, text, true
}

// handleFuzzyHistorySearch handles Ctrl+R for interactive history search.
func handleFuzzyHistorySearch(buf *[]byte, histSnapshot *[]string, histIdx *int, replacePrompt func(string), fd int, oldState *term.State) (done bool, result string, ok bool) {
	term.Restore(fd, oldState)
	chosen := fuzzyHistorySearch()
	term.MakeRaw(fd) //nolint
	if chosen != "" {
		replacePrompt(chosen)
		*histSnapshot = getInputHistory()
		*histIdx = len(*histSnapshot)
	} else {
		fmt.Printf("\r")
		PrintPrompt()
		fmt.Printf("%s", string(*buf))
	}
	return false, "", false
}

// handleArrowKeys processes Up/Down arrow for history navigation.
func handleArrowKeys(key byte, buf *[]byte, histSnapshot *[]string, histIdx *int, savedBuf *[]byte, replacePrompt func(string)) {
	switch key {
	case 'A':
		if *histIdx == len(*histSnapshot) {
			*savedBuf = append([]byte(nil), *buf...)
		}
		if *histIdx > 0 {
			*histIdx--
			replacePrompt((*histSnapshot)[*histIdx])
		}
	case 'B':
		if *histIdx < len(*histSnapshot) {
			*histIdx++
			if *histIdx == len(*histSnapshot) {
				replacePrompt(string(*savedBuf))
			} else {
				replacePrompt((*histSnapshot)[*histIdx])
			}
		}
	}
}

// handleTabCompletion processes Tab key for command/file completion.
func handleTabCompletion(buf *[]byte) {
	current := string(*buf)
	var completion string
	if strings.HasPrefix(current, "/") {
		completion = TabCompleteWithArgs(current)
	} else {
		completion = CompleteFilePath(current)
	}
	if completion != "" && completion != current {
		for range *buf {
			fmt.Print("\b \b")
		}
		*buf = []byte(completion)
		fmt.Printf("%s", completion)
	}
}

// PromptLine shows a prompt and reads a line of input using raw mode char-by-char.
func PromptLine(prompt string) (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", false
	}
	defer term.Restore(fd, oldState)

	fmt.Printf("%s%s%s ", ui.ColorDim, prompt, ui.ColorReset)

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
