package evolution

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// generateTraceID
// ---------------------------------------------------------------------------

func TestGenerateTraceID_NotEmpty(t *testing.T) {
	id := generateTraceID()
	if id == "" {
		t.Error("trace ID should not be empty")
	}
}

func TestGenerateTraceID_Unique(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()
	if id1 == id2 {
		t.Error("trace IDs should be unique")
	}
}

func TestGenerateTraceID_HexFormat(t *testing.T) {
	id := generateTraceID()
	if len(id) != 16 {
		t.Errorf("expected 16 hex chars, got %d", len(id))
	}
}

// ---------------------------------------------------------------------------
// New additional tests
// ---------------------------------------------------------------------------

func TestNew_SetsFields(t *testing.T) {
	e := New("/test/path", slog.Default())
	if e.repoPath != "/test/path" {
		t.Errorf("expected repoPath '/test/path', got %q", e.repoPath)
	}
	if e.logger == nil {
		t.Error("logger should not be nil")
	}
	if e.traceID == "" {
		t.Error("traceID should be generated")
	}
}

// ---------------------------------------------------------------------------
// TraceID
// ---------------------------------------------------------------------------

func TestTraceID_ReturnsValue(t *testing.T) {
	e := New("/tmp", slog.Default())
	id := e.TraceID()
	if id == "" {
		t.Error("TraceID should not be empty")
	}
	if id != e.traceID {
		t.Errorf("TraceID() = %q, want %q", id, e.traceID)
	}
}

// ---------------------------------------------------------------------------
// forwardEvents
// ---------------------------------------------------------------------------

func TestForwardEvents_DrainsChannel(t *testing.T) {
	e := New("/tmp", slog.Default())
	ch := make(chan iteragent.Event, 3)
	ch <- iteragent.Event{Type: "test1"}
	ch <- iteragent.Event{Type: "test2"}
	ch <- iteragent.Event{Type: "test3"}
	close(ch)

	e.forwardEvents(ch)
	if len(ch) != 0 {
		t.Error("channel should be drained")
	}
}

func TestForwardEvents_NilSink(t *testing.T) {
	e := New("/tmp", slog.Default())
	e.eventSink = nil

	ch := make(chan iteragent.Event, 2)
	ch <- iteragent.Event{Type: "test"}
	ch <- iteragent.Event{Type: "test2"}
	close(ch)

	e.forwardEvents(ch)
}

func TestForwardEvents_WithSink(t *testing.T) {
	sink := make(chan iteragent.Event, 10)
	e := New("/tmp", slog.Default()).WithEventSink(sink)

	ch := make(chan iteragent.Event, 2)
	ch <- iteragent.Event{Type: "a"}
	ch <- iteragent.Event{Type: "b"}
	close(ch)

	e.forwardEvents(ch)

	if len(sink) != 2 {
		t.Errorf("expected 2 events forwarded to sink, got %d", len(sink))
	}
}

func TestForwardEvents_FullSink(t *testing.T) {
	sink := make(chan iteragent.Event, 1)
	e := New("/tmp", slog.Default()).WithEventSink(sink)

	sink <- iteragent.Event{Type: "existing"}

	ch := make(chan iteragent.Event, 2)
	ch <- iteragent.Event{Type: "a"}
	ch <- iteragent.Event{Type: "b"}
	close(ch)

	e.forwardEvents(ch)
}

// ---------------------------------------------------------------------------
// withTimeout
// ---------------------------------------------------------------------------

func TestWithTimeout_CreatesContext(t *testing.T) {
	ctx := context.Background()
	timeoutCtx, cancel := withTimeout(ctx)
	defer cancel()

	deadline, ok := timeoutCtx.Deadline()
	if !ok {
		t.Error("expected deadline to be set")
	}
	if time.Until(deadline) > timeoutImplement {
		t.Error("deadline should be within implement timeout")
	}
}

func TestWithTimeout_DefaultTimeout(t *testing.T) {
	if timeoutImplement != 40*time.Minute {
		t.Errorf("expected 40 minutes, got %s", timeoutImplement)
	}
}

// ---------------------------------------------------------------------------
// savePRState / loadPRState / clearPRState
// ---------------------------------------------------------------------------

func TestSaveAndLoadPRState(t *testing.T) {
	// Skip GitHub validation in CI to test core save/load functionality
	t.Setenv("GITHUB_ACTIONS", "")

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".iterate"), 0o755)
	e := New(dir, slog.Default())

	e.prNumber = 42
	e.prURL = "https://github.com/test/repo/pull/42"
	e.branchName = "evolution/day-1"

	if err := e.savePRState(); err != nil {
		t.Fatalf("savePRState failed: %v", err)
	}

	path := filepath.Join(dir, ".iterate", "pr_state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("pr_state.json should exist")
	}

	e2 := New(dir, slog.Default())
	e2.loadPRState()
	if e2.prNumber != 42 {
		t.Errorf("expected prNumber 42, got %d", e2.prNumber)
	}
	if e2.prURL != "https://github.com/test/repo/pull/42" {
		t.Errorf("expected prURL, got %q", e2.prURL)
	}
	if e2.branchName != "evolution/day-1" {
		t.Errorf("expected branchName, got %q", e2.branchName)
	}
}

func TestLoadPRState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.loadPRState()
	if e.prNumber != 0 {
		t.Errorf("expected prNumber 0 for missing file, got %d", e.prNumber)
	}
}

func TestSavePRState_ZeroPR(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".iterate"), 0o755)
	e := New(dir, slog.Default())
	e.prNumber = 0
	err := e.savePRState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File should not be created when prNumber is 0
	path := filepath.Join(dir, ".iterate", "pr_state.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("pr_state.json should not exist when prNumber is 0")
	}
}

func TestClearPRState(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.prNumber = 42
	e.savePRState()

	e.clearPRState()

	path := filepath.Join(dir, ".iterate", "pr_state.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("pr_state.json should be removed")
	}
}

func TestClearPRState_NoFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, slog.Default())
	e.clearPRState()
}

// ---------------------------------------------------------------------------
// Engine struct fields
// ---------------------------------------------------------------------------

func TestEngine_Fields(t *testing.T) {
	e := New("/test", slog.Default())
	// repo may be auto-populated from git, so just check other fields
	if e.branchName != "" {
		t.Errorf("expected empty branchName, got %q", e.branchName)
	}
	if e.prNumber != 0 {
		t.Errorf("expected prNumber 0, got %d", e.prNumber)
	}
	if e.prURL != "" {
		t.Errorf("expected empty prURL, got %q", e.prURL)
	}
}

// ---------------------------------------------------------------------------
// isProtected
// ---------------------------------------------------------------------------

func TestIsProtected_Patterns(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"internal/evolution/engine.go", true},
		{"internal/evolution/git.go", true},
		{".github/workflows/evolve.yml", true},
		{"cmd/iterate/repl.go", true},
		{"cmd/iterate/main.go", true},
		{"internal/commands/registry.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		if got := isProtected(tt.path); got != tt.want {
			t.Errorf("isProtected(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ProtectedFiles list
// ---------------------------------------------------------------------------

func TestProtectedFiles_NotEmpty(t *testing.T) {
	if len(ProtectedFiles) == 0 {
		t.Error("ProtectedFiles should not be empty")
	}
}

func TestProtectedFiles_ContainsExpected(t *testing.T) {
	expected := []string{
		"internal/evolution/engine.go",
		".github/workflows/evolve.yml",
		"cmd/iterate/repl.go",
	}
	for _, pattern := range expected {
		found := false
		for _, p := range ProtectedFiles {
			if p == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in ProtectedFiles", pattern)
		}
	}
}
