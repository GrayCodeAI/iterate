package selector

import (
	"strings"
	"testing"
)

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		total     int
		wantSub   string // substring expected in output
		wantEmpty bool
	}{
		{0, "", true},
		{500, "500 tok", false},
		{1000, "1.0k tok", false},
		{1500, "1.5k tok", false},
		{10000, "10.0k tok", false},
	}
	for _, tt := range tests {
		got := formatTokenCount(tt.total)
		if tt.wantEmpty {
			if got != "" {
				t.Errorf("formatTokenCount(%d) = %q, want empty", tt.total, got)
			}
			continue
		}
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("formatTokenCount(%d) = %q, want substring %q", tt.total, got, tt.wantSub)
		}
	}
}

func TestFormatContextWindow(t *testing.T) {
	ContextWindow = 200_000

	tests := []struct {
		total   int
		wantSub string
	}{
		{0, "0.0%"},
		{100_000, "50.0%"},
		{180_000, "90.0%"},
		{200_000, "100.0%"},
	}
	for _, tt := range tests {
		got := formatContextWindow(tt.total)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("formatContextWindow(%d) = %q, want substring %q", tt.total, got, tt.wantSub)
		}
	}
}

func TestFormatContextWindowColors(t *testing.T) {
	ContextWindow = 200_000

	// Below 75% — blue
	low := formatContextWindow(100_000)
	if !strings.Contains(low, "\033[38;5;75m") {
		t.Errorf("expected blue color for low usage, got %q", low)
	}

	// Above 75% — yellow
	mid := formatContextWindow(160_000)
	if !strings.Contains(mid, "\033[38;5;220m") && !strings.Contains(mid, "\033[33m") {
		t.Errorf("expected yellow color for mid usage, got %q", mid)
	}

	// Above 90% — red
	high := formatContextWindow(185_000)
	if !strings.Contains(high, "\033[31m") {
		t.Errorf("expected red color for high usage, got %q", high)
	}
}

func TestFormatCostUSD(t *testing.T) {
	tests := []struct {
		usd  float64
		want string
	}{
		{0, ""},
		{-1, ""},
		{0.00001, "<$0.0001"},
		{0.0005, "$0.0005"},
		{0.005, "$0.005"},
		{0.05, "$0.05"},
		{1.23, "$1.23"},
	}
	for _, tt := range tests {
		got := formatCostUSD(tt.usd)
		if got != tt.want {
			t.Errorf("formatCostUSD(%v) = %q, want %q", tt.usd, got, tt.want)
		}
	}
}
