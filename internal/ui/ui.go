// Package ui provides terminal rendering helpers for iterate.
package ui

import "fmt"

// Color helpers (reassignable for /theme support)
var (
	ColorReset  = "\033[0m"
	ColorLime   = "\033[38;5;154m"
	ColorYellow = "\033[38;5;220m"
	ColorDim    = "\033[2m"
	ColorBold   = "\033[1m"
	ColorCyan   = "\033[36m"
	ColorRed    = "\033[31m"
)

// PrintSuccess prints a success message with a checkmark.
func PrintSuccess(format string, args ...any) {
	fmt.Printf("%s✓ %s%s\n", ColorLime, fmt.Sprintf(format, args...), ColorReset)
}

// PrintError prints an error message.
func PrintError(format string, args ...any) {
	fmt.Printf("%serror: %s%s\n", ColorRed, fmt.Sprintf(format, args...), ColorReset)
}

// PrintDim prints a dimmed message.
func PrintDim(format string, args ...any) {
	fmt.Printf("%s%s%s\n", ColorDim, fmt.Sprintf(format, args...), ColorReset)
}
