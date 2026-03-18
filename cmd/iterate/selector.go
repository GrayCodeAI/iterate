package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const maxVisible = 12

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
// Enter submits. Shift+Enter (or \n in raw mode via alt sequences) adds a newline.
// Returns (text, true) or ("", false) on Ctrl+C/EOF.
func readInput() (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// fallback to simple line read
		fmt.Printf("%s❯%s ", colorLime, colorReset)
		var line string
		fmt.Scanln(&line)
		return strings.TrimSpace(line), true
	}
	defer term.Restore(fd, oldState)

	fmt.Printf("%s❯%s ", colorLime, colorReset)

	var buf []byte
	b := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n == 0 {
			return "", false
		}

		switch {
		case b[0] == '\r' || b[0] == '\n':
			// Enter — submit
			fmt.Print("\r\n")
			return strings.TrimSpace(string(buf)), true

		case b[0] == 3: // Ctrl+C
			fmt.Print("\r\n")
			return "", false

		case b[0] == 4: // Ctrl+D EOF
			fmt.Print("\r\n")
			return "", false

		case n >= 2 && b[0] == 13 && b[1] == 10: // Shift+Enter on some terminals
			buf = append(buf, '\n')
			fmt.Print("\r\n")

		case b[0] == 27 && n >= 3 && b[1] == '[':
			// Arrow keys / escape sequences — ignore
		case b[0] == 27 && n == 1:
			// bare ESC — ignore

		case b[0] == 127 || b[0] == 8: // backspace
			if len(buf) > 0 {
				// Handle multi-byte UTF-8 backspace
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}

		default:
			if b[0] >= 32 || b[0] == '\t' {
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
