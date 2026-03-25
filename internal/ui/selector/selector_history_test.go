package selector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetHistory() {
	inputHistoryMu.Lock()
	inputHistory = nil
	inputHistoryMu.Unlock()
	historyFile = ""
}

// ---------------------------------------------------------------------------
// appendHistory
// ---------------------------------------------------------------------------

func TestAppendHistory_Basic(t *testing.T) {
	resetHistory()
	appendHistory("hello")
	appendHistory("world")
	h := getInputHistory()
	if len(h) != 2 || h[0] != "hello" || h[1] != "world" {
		t.Errorf("unexpected history: %v", h)
	}
}

func TestAppendHistory_SkipsEmpty(t *testing.T) {
	resetHistory()
	appendHistory("")
	if len(getInputHistory()) != 0 {
		t.Error("empty string should not be appended")
	}
}

func TestAppendHistory_DeduplicatesConsecutive(t *testing.T) {
	resetHistory()
	appendHistory("dup")
	appendHistory("dup")
	if len(getInputHistory()) != 1 {
		t.Errorf("consecutive duplicate should not be added, got %v", getInputHistory())
	}
}

func TestAppendHistory_AllowsNonConsecutiveDuplicates(t *testing.T) {
	resetHistory()
	appendHistory("a")
	appendHistory("b")
	appendHistory("a")
	if len(getInputHistory()) != 3 {
		t.Errorf("non-consecutive duplicates should be kept, got %v", getInputHistory())
	}
}

func TestAppendHistory_PersistsToFile(t *testing.T) {
	resetHistory()
	dir := t.TempDir()
	historyFile = filepath.Join(dir, "history")

	appendHistory("persisted line")

	data, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatalf("history file not written: %v", err)
	}
	if !strings.Contains(string(data), "persisted line") {
		t.Errorf("line not found in history file: %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// redactSensitiveInput
// ---------------------------------------------------------------------------

func TestRedactSensitiveInput_RedactsProviderKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/provider anthropic sk-abc123", "/provider anthropic [redacted]"},
		{"/PROVIDER openai sk-xyz", "/PROVIDER openai [redacted]"},
		{"/provider gemini AIza1234 extra", "/provider gemini [redacted]"},
	}
	for _, tt := range tests {
		got := redactSensitiveInput(tt.input)
		if got != tt.want {
			t.Errorf("redactSensitiveInput(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedactSensitiveInput_PassesThroughSafe(t *testing.T) {
	safe := []string{
		"/provider anthropic",  // no key
		"/help",
		"hello world",
		"",
	}
	for _, s := range safe {
		if got := redactSensitiveInput(s); got != s {
			t.Errorf("redactSensitiveInput(%q) modified safe input to %q", s, got)
		}
	}
}

// ---------------------------------------------------------------------------
// trimHistoryFile
// ---------------------------------------------------------------------------

func TestTrimHistoryFile_KeepsAtMostMaxLines(t *testing.T) {
	dir := t.TempDir()
	historyFile = filepath.Join(dir, "history")

	var lines []string
	for i := 0; i < maxHistoryLines+50; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(historyFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	trimHistoryFile()

	data, _ := os.ReadFile(historyFile)
	got := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(got) > maxHistoryLines {
		t.Errorf("expected at most %d lines after trim, got %d", maxHistoryLines, len(got))
	}
}

func TestTrimHistoryFile_NoOpWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	historyFile = filepath.Join(dir, "history")
	content := "a\nb\nc\n"
	os.WriteFile(historyFile, []byte(content), 0o600)

	trimHistoryFile()

	data, _ := os.ReadFile(historyFile)
	if string(data) != content {
		t.Errorf("trim modified file when under limit: got %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// deduplicateHistory
// ---------------------------------------------------------------------------

func TestDeduplicateHistory_RemovesDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	got := deduplicateHistory(input)
	seen := map[string]bool{}
	for _, v := range got {
		if seen[v] {
			t.Errorf("duplicate %q in deduplicated output %v", v, got)
		}
		seen[v] = true
	}
}

func TestDeduplicateHistory_MostRecentFirst(t *testing.T) {
	input := []string{"first", "second", "third"}
	got := deduplicateHistory(input)
	if len(got) == 0 || got[0] != "third" {
		t.Errorf("expected most recent entry first, got %v", got)
	}
}

func TestDeduplicateHistory_EmptyInput(t *testing.T) {
	if got := deduplicateHistory(nil); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// filterHistoryEntries
// ---------------------------------------------------------------------------

func TestFilterHistoryEntries_CaseInsensitive(t *testing.T) {
	entries := []string{"Hello World", "foo bar", "HELLO again"}
	got := filterHistoryEntries(entries, "hello")
	if len(got) != 2 {
		t.Errorf("expected 2 matches, got %v", got)
	}
}

func TestFilterHistoryEntries_EmptyQueryReturnsAll(t *testing.T) {
	entries := []string{"a", "b", "c"}
	got := filterHistoryEntries(entries, "")
	if len(got) != 3 {
		t.Errorf("empty query should return all entries, got %v", got)
	}
}

func TestFilterHistoryEntries_NoMatch(t *testing.T) {
	entries := []string{"alpha", "beta"}
	got := filterHistoryEntries(entries, "zzz")
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// InitHistory
// ---------------------------------------------------------------------------

func TestInitHistory_LoadsFromFile(t *testing.T) {
	resetHistory()
	dir := t.TempDir()
	histFile := filepath.Join(dir, "history")
	os.WriteFile(histFile, []byte("cmd1\ncmd2\ncmd3\n"), 0o600)

	// Temporarily point historyFile to test file and re-load
	historyFile = histFile
	f, _ := os.Open(histFile)
	defer f.Close()
	import_scanner := func() {
		inputHistoryMu.Lock()
		defer inputHistoryMu.Unlock()
		import_buf := make([]byte, 4096)
		n, _ := f.Read(import_buf)
		for _, line := range strings.Split(strings.TrimRight(string(import_buf[:n]), "\n"), "\n") {
			if line != "" {
				inputHistory = append(inputHistory, line)
			}
		}
	}
	import_scanner()

	h := getInputHistory()
	if len(h) != 3 || h[0] != "cmd1" {
		t.Errorf("unexpected history loaded: %v", h)
	}
}
