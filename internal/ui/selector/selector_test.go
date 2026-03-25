package selector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TabComplete
// ---------------------------------------------------------------------------

func TestTabComplete_ExactPrefix(t *testing.T) {
	got := TabComplete("/hel")
	if got != "/help " {
		t.Errorf("expected /help , got %q", got)
	}
}

func TestTabComplete_NoMatch(t *testing.T) {
	got := TabComplete("/zzznomatch")
	if got != "/zzznomatch" {
		t.Errorf("expected input unchanged, got %q", got)
	}
}

func TestTabComplete_CommonPrefixOnMultipleMatches(t *testing.T) {
	// "/co" matches /context, /cost, /config, /coverage, /copy, /commit, /count-lines, /contributors, /cd, /co...
	got := TabComplete("/co")
	if !strings.HasPrefix(got, "/co") {
		t.Errorf("expected common prefix starting with /co, got %q", got)
	}
}

func TestTabComplete_EmptyInput(t *testing.T) {
	got := TabComplete("")
	// Empty input: no matches since no command starts with ""... actually all do
	// Just verify it doesn't panic
	_ = got
}

func TestTabComplete_FullCommandReturnsWithSpace(t *testing.T) {
	got := TabComplete("/quit")
	// Exact match of a unique command should return with trailing space
	if got != "/quit " {
		t.Errorf("expected /quit , got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TabCompleteWithArgs
// ---------------------------------------------------------------------------

func TestTabCompleteWithArgs_CompletesArgument(t *testing.T) {
	got := TabCompleteWithArgs("/thinking of")
	if !strings.Contains(got, "off") {
		t.Errorf("expected off in completion, got %q", got)
	}
}

func TestTabCompleteWithArgs_FallsBackToTabComplete(t *testing.T) {
	// no space → treat as command completion
	got := TabCompleteWithArgs("/hel")
	if got != "/help " {
		t.Errorf("expected /help , got %q", got)
	}
}

func TestTabCompleteWithArgs_UnknownCommandNoArgCompletion(t *testing.T) {
	// /unknowncmd with arg → should return unchanged (no arg completions defined)
	got := TabCompleteWithArgs("/unknowncmd foo")
	if got != "/unknowncmd foo" {
		t.Errorf("expected no change, got %q", got)
	}
}

func TestTabCompleteWithArgs_ProviderArgs(t *testing.T) {
	got := TabCompleteWithArgs("/provider ant")
	if !strings.Contains(got, "anthropic") {
		t.Errorf("expected anthropic in completion, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// CompleteFilePath
// ---------------------------------------------------------------------------

func TestCompleteFilePath_CompletesExistingFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(""), 0o644)

	partial := "/add " + filepath.Join(dir, "main")
	got := CompleteFilePath(partial)
	if !strings.HasPrefix(got, "/add "+dir) {
		t.Errorf("expected completed path, got %q", got)
	}
}

func TestCompleteFilePath_NoMatchReturnsInput(t *testing.T) {
	got := CompleteFilePath("/add /no/such/path/xyz")
	if got != "/add /no/such/path/xyz" {
		t.Errorf("expected input unchanged, got %q", got)
	}
}

func TestCompleteFilePath_DirectoryGetsSlash(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "mysubdir")
	os.MkdirAll(subdir, 0o755)

	partial := "/add " + filepath.Join(dir, "mysub")
	got := CompleteFilePath(partial)
	if !strings.HasSuffix(got, "/") {
		t.Errorf("directory completion should end with /, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// GetHistory (exported)
// ---------------------------------------------------------------------------

func TestGetHistory_ReturnsACopy(t *testing.T) {
	resetHistory()
	appendHistory("entry1")
	appendHistory("entry2")

	h1 := GetHistory()
	h1[0] = "modified"

	h2 := GetHistory()
	if h2[0] == "modified" {
		t.Error("GetHistory should return an independent copy")
	}
}

// ---------------------------------------------------------------------------
// formatElapsed
// ---------------------------------------------------------------------------

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		input    int64 // nanoseconds
		contains string
	}{
		{500_000_000, "ms"},        // 500ms
		{5_500_000_000, "s"},       // 5.5s
		{65_000_000_000, "m"},      // 1m5s
	}
	for _, tt := range tests {
		got := formatElapsed(time.Duration(tt.input))
		if !strings.Contains(got, tt.contains) {
			t.Errorf("formatElapsed(%dns) = %q, expected to contain %q", tt.input, got, tt.contains)
		}
	}
}
