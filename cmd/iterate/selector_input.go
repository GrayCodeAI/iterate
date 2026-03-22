package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

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
	histSnapshot := getInputHistory()
	histIdx := len(histSnapshot)
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
				histSnapshot = getInputHistory()
				histIdx = len(histSnapshot)
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
				if histIdx == len(histSnapshot) {
					savedBuf = append([]byte(nil), buf...) // save current draft
				}
				if histIdx > 0 {
					histIdx--
					replacePrompt(histSnapshot[histIdx])
				}
			case 'B': // Down arrow — go forward in history
				if histIdx < len(histSnapshot) {
					histIdx++
					if histIdx == len(histSnapshot) {
						replacePrompt(string(savedBuf))
					} else {
						replacePrompt(histSnapshot[histIdx])
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
