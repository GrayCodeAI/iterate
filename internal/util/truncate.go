// Package util provides common utility functions used across the iterate codebase.
package util

// Truncate shortens s to maxLen characters, adding "…" if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
