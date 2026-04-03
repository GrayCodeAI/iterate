package util

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"empty string", "", 5, ""},
		{"shorter than maxLen", "hi", 10, "hi"},
		{"equal to maxLen", "hello", 5, "hello"},
		{"longer than maxLen adds ellipsis", "hello world", 6, "hello…"},
		{"maxLen 0", "abc", 0, ""},
		{"maxLen 1", "abc", 1, "a"},
		{"maxLen 2", "abc", 2, "ab"},
		{"maxLen 3 no ellipsis", "abcd", 3, "abc"},
		{"longer than maxLen len 7", "hello world", 7, "hello …"},
		{"single char maxLen 5", "x", 5, "x"},
		{"unicode longer", "abcdef", 5, "abcd…"},
		{"unicode rune truncated by bytes", "caf\u00e9\u00e9\u00e9", 6, "caf\u00e9…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}
