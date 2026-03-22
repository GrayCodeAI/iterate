// Package highlight provides syntax highlighting for terminal output.
package highlight

import (
	"fmt"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

var (
	colorGreen  = "\033[38;5;114m"
	colorAmber  = "\033[38;5;221m"
	colorBlue   = "\033[38;5;75m"
	colorPurple = "\033[38;5;141m"
)

// contextWindow is the assumed token context window size used for % calculations.
const ContextWindow = 200_000

// RenderResponse prints a markdown response with syntax highlighting.
func RenderResponse(text string) {
	lines := strings.Split(text, "\n")
	inCode := false
	lang := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCode {
				fmt.Printf("%s```%s\n", ui.ColorDim, ui.ColorReset)
				inCode = false
				lang = ""
			} else {
				lang = strings.TrimPrefix(line, "```")
				fmt.Printf("%s```%s%s%s\n", ui.ColorDim, ui.ColorCyan, lang, ui.ColorReset)
				inCode = true
			}
		} else if inCode {
			fmt.Println(HighlightCode(line, lang))
		} else {
			fmt.Println(RenderInline(line))
		}
	}
}

// RenderInline handles bold, italic, inline code in a line.
func RenderInline(line string) string {
	// Bold **text**
	line = ReplacePairs(line, "**", ui.ColorBold, ui.ColorReset+"\033[97m")
	// Italic *text*
	line = ReplacePairs(line, "*", "\033[3m", ui.ColorReset+"\033[97m")
	// Inline code `text`
	line = ReplacePairs(line, "`", colorAmber, ui.ColorReset+"\033[97m")
	return "\033[97m" + line + ui.ColorReset
}

// ReplacePairs replaces delimited spans with color codes.
func ReplacePairs(s, delim, open, close string) string {
	var b strings.Builder
	parts := strings.Split(s, delim)
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString(open + p + close)
		} else {
			b.WriteString(p)
		}
	}
	return b.String()
}

// HighlightCode applies basic keyword highlighting for common languages.
func HighlightCode(line, lang string) string {
	switch strings.ToLower(lang) {
	case "go":
		return highlightGo(line)
	case "python", "py":
		return highlightPython(line)
	case "bash", "sh", "shell":
		return highlightBash(line)
	case "json":
		return highlightJSON(line)
	default:
		return colorGreen + line + ui.ColorReset
	}
}

var goKeywords = []string{
	"func", "type", "struct", "interface", "import", "package",
	"var", "const", "return", "if", "else", "for", "range",
	"switch", "case", "default", "go", "defer", "select",
	"chan", "map", "make", "new", "nil", "true", "false",
	"error", "string", "int", "int64", "bool", "byte",
}

func highlightGo(line string) string {
	// Comments
	if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
		return ui.ColorDim + line + ui.ColorReset
	}
	result := colorize(line, goKeywords, colorBlue)
	result = colorizeStrings(result)
	return result
}

var pyKeywords = []string{
	"def", "class", "import", "from", "return", "if", "elif",
	"else", "for", "while", "in", "not", "and", "or", "True",
	"False", "None", "with", "as", "try", "except", "finally",
	"pass", "break", "continue", "lambda", "yield", "async", "await",
}

func highlightPython(line string) string {
	if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") {
		return ui.ColorDim + line + ui.ColorReset
	}
	result := colorize(line, pyKeywords, colorBlue)
	result = colorizeStrings(result)
	return result
}

var bashKeywords = []string{
	"if", "then", "else", "fi", "for", "do", "done", "while",
	"function", "return", "export", "echo", "cd", "ls", "grep",
	"sudo", "chmod", "mkdir", "rm", "cp", "mv",
}

func highlightBash(line string) string {
	if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") {
		return ui.ColorDim + line + ui.ColorReset
	}
	result := colorize(line, bashKeywords, colorBlue)
	result = colorizeStrings(result)
	return result
}

func highlightJSON(line string) string {
	// Keys in lime, values in amber
	result := line
	result = strings.ReplaceAll(result, `"`, colorAmber+`"`+ui.ColorReset+colorGreen)
	return colorGreen + result + ui.ColorReset
}

// colorize wraps whole-word keywords with color codes.
func colorize(line string, keywords []string, color string) string {
	words := strings.Fields(line)
	for i, word := range words {
		clean := strings.TrimFunc(word, func(r rune) bool {
			return strings.ContainsRune("(){}[],;:.", r)
		})
		for _, kw := range keywords {
			if clean == kw {
				words[i] = strings.Replace(word, clean, color+clean+ui.ColorReset+colorGreen, 1)
				break
			}
		}
	}
	return colorGreen + strings.Join(words, " ") + ui.ColorReset
}

// colorizeStrings highlights quoted strings in amber.
func colorizeStrings(line string) string {
	var b strings.Builder
	inStr := false
	quote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if !inStr && (c == '"' || c == '\'') {
			inStr = true
			quote = c
			b.WriteString(colorAmber)
			b.WriteByte(c)
		} else if inStr && c == quote {
			b.WriteByte(c)
			b.WriteString(ui.ColorReset + colorGreen)
			inStr = false
			quote = 0
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}
