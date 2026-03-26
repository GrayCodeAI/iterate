package selector

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

// inMultilineBlock is set when the user opens a triple-backtick block.
// It is cleared when the closing ``` is entered or the input is cancelled.
var inMultilineBlock bool

// ReadInput reads user input in raw mode.
// Enter submits. Ctrl+J inserts a literal newline (multiline input).
// Starting a line with ``` enters block mode; another ``` closes it.
// Up/Down arrow navigates history. Left/Right arrows move the cursor.
// Delete removes the char under the cursor.
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
	cursorPos := 0
	b := make([]byte, 4)
	histSnapshot := getInputHistory()
	histIdx := len(histSnapshot)
	savedBuf := []byte(nil)
	killRing := ""

	replacePrompt := func(newText string) {
		// Erase existing visible text: move left to start, overwrite, clear to end.
		if cursorPos > 0 {
			fmt.Printf("%s", strings.Repeat("\b", cursorPos))
		}
		tail := len(buf) - cursorPos
		fmt.Printf("%s%s", newText, strings.Repeat(" ", tail))
		// Move back to erase any chars that were longer.
		if tail > 0 || len(newText) < len(buf) {
			extra := len(buf) - len(newText)
			if extra < 0 {
				extra = 0
			}
			fmt.Printf("%s", strings.Repeat("\b", tail+extra))
		}
		// Cursor is now at beginning of new text, advance to end.
		fmt.Printf("%s", newText)
		buf = []byte(newText)
		cursorPos = len(buf)
	}

	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			return "", false
		}

		done, result, ok := handleRawInput(b, n, &buf, &cursorPos, &histSnapshot, &histIdx, &savedBuf, &killRing, replacePrompt, fd, oldState)
		if done {
			return result, ok
		}
	}
}

// moveWordBackward moves cursorPos back to the start of the previous word and
// emits the required backspace characters so the terminal cursor follows.
func moveWordBackward(buf *[]byte, cursorPos *int) {
	i := *cursorPos
	// Skip trailing spaces.
	for i > 0 && (*buf)[i-1] == ' ' {
		i--
	}
	// Skip the word.
	for i > 0 && (*buf)[i-1] != ' ' {
		i--
	}
	delta := *cursorPos - i
	if delta > 0 {
		fmt.Printf("%s", strings.Repeat("\b", delta))
		*cursorPos = i
	}
}

// moveWordForward moves cursorPos to the end of the next word and emits the
// required character output so the terminal cursor follows.
func moveWordForward(buf *[]byte, cursorPos *int) {
	i := *cursorPos
	n := len(*buf)
	// Skip leading spaces.
	for i < n && (*buf)[i] == ' ' {
		i++
	}
	// Skip the word.
	for i < n && (*buf)[i] != ' ' {
		i++
	}
	if i > *cursorPos {
		fmt.Printf("%s", string((*buf)[*cursorPos:i]))
		*cursorPos = i
	}
}

// redrawFromCursor redraws buf[cursorPos:] on the terminal, appends erased
// spaces to clear old characters, then repositions the cursor back to cursorPos.
func redrawFromCursor(buf []byte, cursorPos, erased int) {
	tail := buf[cursorPos:]
	fmt.Printf("%s%s", string(tail), strings.Repeat(" ", erased))
	fmt.Printf("%s", strings.Repeat("\b", len(tail)+erased))
}

// handleRawInput processes a single raw-mode key event and returns (done, text, ok).
func handleRawInput(b []byte, n int, buf *[]byte, cursorPos *int, histSnapshot *[]string, histIdx *int, savedBuf *[]byte, killRing *string, replacePrompt func(string), fd int, oldState *term.State) (done bool, result string, ok bool) {
	switch {
	case b[0] == '\r':
		return handleLineSubmit(buf, cursorPos, histSnapshot, histIdx, savedBuf)

	case b[0] == '\n': // Ctrl+J — insert literal newline (multiline input)
		*buf = append(append((*buf)[:*cursorPos:*cursorPos], '\n'), (*buf)[*cursorPos:]...)
		*cursorPos++
		fmt.Print("\r\n")
		fmt.Printf("%s  ...%s ", ui.ColorDim, ui.ColorReset)
		// Reprint any text after the cursor on the new line.
		if *cursorPos < len(*buf) {
			fmt.Printf("%s", string((*buf)[*cursorPos:]))
			fmt.Printf("%s", strings.Repeat("\b", len(*buf)-*cursorPos))
		}
		return false, "", false

	case b[0] == 3:
		// Two-stage Ctrl+C: first press clears the line, second press exits.
		if len(*buf) > 0 || inMultilineBlock {
			inMultilineBlock = false
			if *cursorPos > 0 {
				fmt.Printf("%s", strings.Repeat("\b", *cursorPos))
			}
			fmt.Printf("%s ", strings.Repeat(" ", len(*buf)))
			fmt.Printf("%s", strings.Repeat("\b", len(*buf)+1))
			*buf = (*buf)[:0]
			*cursorPos = 0
			return false, "", false
		}
		fmt.Print("\r\n")
		return true, "", false

	case b[0] == 18:
		return handleFuzzyHistorySearch(buf, cursorPos, histSnapshot, histIdx, replacePrompt, fd, oldState)

	case b[0] == 4:
		fmt.Print("\r\n")
		return true, "", false

	case n >= 2 && b[0] == 13 && b[1] == 10:
		*buf = append(*buf, '\n')
		fmt.Print("\r\n")
		return false, "", false

	case b[0] == 27 && n >= 3 && b[1] == '[':
		handleEscSeq(b, n, buf, cursorPos, histSnapshot, histIdx, savedBuf, replacePrompt)
		return false, "", false

	case b[0] == 27 && n == 2 && b[1] == 'b':
		// Alt+Left (ESC b) — move backward one word.
		moveWordBackward(buf, cursorPos)
		return false, "", false

	case b[0] == 27 && n == 2 && b[1] == 'f':
		// Alt+Right (ESC f) — move forward one word.
		moveWordForward(buf, cursorPos)
		return false, "", false

	case b[0] == 27 && n == 1:
		return false, "", false

	case b[0] == '\t':
		handleTabCompletion(buf, cursorPos)
		return false, "", false

	case b[0] == 127 || b[0] == 8:
		// Backspace: delete the character before the cursor.
		if *cursorPos > 0 {
			*cursorPos--
			*buf = append((*buf)[:*cursorPos], (*buf)[*cursorPos+1:]...)
			fmt.Print("\b")
			redrawFromCursor(*buf, *cursorPos, 1)
		}
		return false, "", false

	case b[0] == 23:
		// Ctrl+W: kill word backward (delete back to last whitespace boundary).
		if *cursorPos > 0 {
			i := *cursorPos - 1
			for i > 0 && (*buf)[i] == ' ' {
				i--
			}
			for i > 0 && (*buf)[i-1] != ' ' {
				i--
			}
			deleted := *cursorPos - i
			*killRing = string((*buf)[i:*cursorPos])
			*buf = append((*buf)[:i], (*buf)[*cursorPos:]...)
			fmt.Printf("%s", strings.Repeat("\b", deleted))
			*cursorPos = i
			redrawFromCursor(*buf, *cursorPos, deleted)
		}
		return false, "", false

	case b[0] == 21:
		// Ctrl+U: kill to beginning of line.
		if *cursorPos > 0 {
			*killRing = string((*buf)[:*cursorPos])
			*buf = (*buf)[*cursorPos:]
			fmt.Printf("%s", strings.Repeat("\b", *cursorPos))
			*cursorPos = 0
			redrawFromCursor(*buf, 0, len(*killRing))
		}
		return false, "", false

	case b[0] == 11:
		// Ctrl+K: kill from cursor to end of line.
		if *cursorPos < len(*buf) {
			*killRing = string((*buf)[*cursorPos:])
			erased := len(*killRing)
			*buf = (*buf)[:*cursorPos]
			fmt.Printf("%s%s", strings.Repeat(" ", erased), strings.Repeat("\b", erased))
		}
		return false, "", false

	case b[0] == 25:
		// Ctrl+Y: yank (paste kill ring).
		if *killRing != "" {
			yank := []byte(*killRing)
			newBuf := make([]byte, 0, len(*buf)+len(yank))
			newBuf = append(newBuf, (*buf)[:*cursorPos]...)
			newBuf = append(newBuf, yank...)
			newBuf = append(newBuf, (*buf)[*cursorPos:]...)
			*buf = newBuf
			fmt.Printf("%s", *killRing)
			*cursorPos += len(yank)
			redrawFromCursor(*buf, *cursorPos, 0)
		}
		return false, "", false

	case b[0] == 1:
		// Ctrl+A: move to beginning of line.
		if *cursorPos > 0 {
			fmt.Printf("%s", strings.Repeat("\b", *cursorPos))
			*cursorPos = 0
		}
		return false, "", false

	case b[0] == 5:
		// Ctrl+E: move to end of line.
		if *cursorPos < len(*buf) {
			fmt.Printf("%s", string((*buf)[*cursorPos:]))
			*cursorPos = len(*buf)
		}
		return false, "", false

	default:
		if b[0] >= 32 {
			// Insert character(s) at cursor position.
			ch := b[:n]
			newBuf := make([]byte, 0, len(*buf)+len(ch))
			newBuf = append(newBuf, (*buf)[:*cursorPos]...)
			newBuf = append(newBuf, ch...)
			newBuf = append(newBuf, (*buf)[*cursorPos:]...)
			*buf = newBuf
			fmt.Printf("%s", string(ch))
			*cursorPos += len(ch)
			// Redraw the tail (chars after the new insertion).
			redrawFromCursor(*buf, *cursorPos, 0)
		}
		return false, "", false
	}
}

// handleEscSeq handles ESC [ sequences: arrows, Delete key, Home, End.
func handleEscSeq(b []byte, n int, buf *[]byte, cursorPos *int, histSnapshot *[]string, histIdx *int, savedBuf *[]byte, replacePrompt func(string)) {
	if n < 3 {
		return
	}
	switch b[2] {
	case 'A': // Up arrow — history back
		handleArrowKeys('A', buf, histSnapshot, histIdx, savedBuf, replacePrompt)
	case 'B': // Down arrow — history forward
		handleArrowKeys('B', buf, histSnapshot, histIdx, savedBuf, replacePrompt)
	case 'C': // Right arrow — move cursor forward
		if *cursorPos < len(*buf) {
			fmt.Printf("%s", string((*buf)[*cursorPos]))
			*cursorPos++
		}
	case 'D': // Left arrow — move cursor backward
		if *cursorPos > 0 {
			fmt.Print("\b")
			*cursorPos--
		}
	case '3': // Delete key: ESC [ 3 ~
		if n >= 4 && b[3] == '~' && *cursorPos < len(*buf) {
			*buf = append((*buf)[:*cursorPos], (*buf)[*cursorPos+1:]...)
			redrawFromCursor(*buf, *cursorPos, 1)
		}
	case '1', 'H': // Home: ESC [ 1 ~ or ESC [ H
		if *cursorPos > 0 {
			fmt.Printf("%s", strings.Repeat("\b", *cursorPos))
			*cursorPos = 0
		}
	case '4', 'F': // End: ESC [ 4 ~ or ESC [ F
		if *cursorPos < len(*buf) {
			fmt.Printf("%s", string((*buf)[*cursorPos:]))
			*cursorPos = len(*buf)
		}
	}
}

// handleLineSubmit processes Enter key, handling backslash continuation and
// triple-backtick multiline blocks.
func handleLineSubmit(buf *[]byte, cursorPos *int, histSnapshot *[]string, histIdx *int, savedBuf *[]byte) (done bool, result string, ok bool) {
	fmt.Print("\r\n")
	text := string(*buf)
	trimmedText := strings.TrimSpace(text)

	// Inside a triple-backtick block: ``` closes it, otherwise accumulate.
	if inMultilineBlock {
		if trimmedText == "```" {
			// Close the block — strip the closing fence and submit accumulated content.
			inMultilineBlock = false
			// Remove the closing ``` from the buffer tail.
			idx := strings.LastIndex(text, "```")
			if idx >= 0 {
				text = strings.TrimRight(text[:idx], "\n")
			}
			appendHistory(text)
			return true, text, true
		}
		// Continue accumulating.
		text = text + "\n"
		*buf = []byte(text)
		*cursorPos = len(*buf)
		fmt.Printf("%s  ...%s ", ui.ColorDim, ui.ColorReset)
		return false, "", false
	}

	// Opening triple-backtick block.
	if trimmedText == "```" {
		inMultilineBlock = true
		fmt.Printf("%s  (multiline block — end with a line containing only ``` )%s\n", ui.ColorDim, ui.ColorReset)
		fmt.Printf("%s  ...%s ", ui.ColorDim, ui.ColorReset)
		*buf = []byte{}
		*cursorPos = 0
		return false, "", false
	}

	// Backslash continuation: "some text\" → append newline and keep reading.
	if strings.HasSuffix(strings.TrimRight(text, " "), "\\") {
		text = strings.TrimRight(text, " ")
		text = text[:len(text)-1] + "\n"
		*buf = []byte(text)
		*cursorPos = len(*buf)
		fmt.Printf("%s  ...%s ", ui.ColorDim, ui.ColorReset)
		return false, "", false
	}

	// Already in backslash continuation (buf has embedded newlines from prior \-Enter)?
	if strings.Contains(text, "\n") {
		// Keep accumulating: no trailing \ means next Enter will submit.
		// (The user can end by pressing Enter without a trailing \.)
		text = strings.TrimSpace(text)
		appendHistory(text)
		return true, text, true
	}

	text = strings.TrimSpace(text)
	appendHistory(text)
	return true, text, true
}

// handleFuzzyHistorySearch handles Ctrl+R for interactive history search.
func handleFuzzyHistorySearch(buf *[]byte, cursorPos *int, histSnapshot *[]string, histIdx *int, replacePrompt func(string), fd int, oldState *term.State) (done bool, result string, ok bool) {
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
		// Reposition cursor if not at end.
		if *cursorPos < len(*buf) {
			fmt.Printf("%s", strings.Repeat("\b", len(*buf)-*cursorPos))
		}
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
func handleTabCompletion(buf *[]byte, cursorPos *int) {
	current := string(*buf)
	var completion string
	if strings.HasPrefix(current, "/") {
		completion = TabCompleteWithArgs(current)
	} else {
		completion = CompleteFilePath(current)
	}
	if completion != "" && completion != current {
		if *cursorPos > 0 {
			fmt.Printf("%s", strings.Repeat("\b", *cursorPos))
		}
		erased := len(*buf) - len(completion)
		if erased < 0 {
			erased = 0
		}
		fmt.Printf("%s%s%s", completion, strings.Repeat(" ", erased), strings.Repeat("\b", erased))
		*buf = []byte(completion)
		*cursorPos = len(*buf)
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
