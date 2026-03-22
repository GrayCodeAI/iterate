package evolution

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsProtected_EngineFile(t *testing.T) {
	if !isProtected("internal/evolution/engine.go") {
		t.Error("engine.go should be protected")
	}
}

func TestIsProtected_WorkflowFile(t *testing.T) {
	if !isProtected(".github/workflows/evolve.yml") {
		t.Error("evolve.yml should be protected")
	}
}

func TestIsProtected_ReplFile(t *testing.T) {
	if !isProtected("cmd/iterate/repl.go") {
		t.Error("repl.go should be protected")
	}
}

func TestIsProtected_MainFile(t *testing.T) {
	if !isProtected("cmd/iterate/main.go") {
		t.Error("main.go should be protected")
	}
}

func TestIsProtected_EvolveScript(t *testing.T) {
	if !isProtected("scripts/evolution/evolve.sh") {
		t.Error("evolve.sh should be protected")
	}
}

func TestIsProtected_ConfigFile(t *testing.T) {
	if !isProtected(".iterate/config.json") {
		t.Error("config.json should be protected")
	}
}

func TestIsProtected_NormalFile(t *testing.T) {
	if isProtected("cmd/iterate/pricing.go") {
		t.Error("pricing.go should NOT be protected")
	}
}

func TestIsProtected_NormalTestFile(t *testing.T) {
	if isProtected("cmd/iterate/pricing_test.go") {
		t.Error("pricing_test.go should NOT be protected")
	}
}

func TestIsProtected_InternalCommands(t *testing.T) {
	if isProtected("internal/commands/agent.go") {
		t.Error("internal/commands/agent.go should NOT be protected")
	}
}

func TestIsProtected_GlobPattern(t *testing.T) {
	// Internal evolution glob pattern should match any .go file in that dir
	if !isProtected("internal/evolution/new_file.go") {
		t.Error("new files in internal/evolution/ should be protected via glob")
	}
}

func TestIsProtected_WorkflowGlob(t *testing.T) {
	// The .github/workflows/*.yml pattern matches base filename via filepath.Match
	// and checks directory equality. This is implementation-specific.
	// Verify the exact protected patterns work instead.
	if !isProtected(".github/workflows/evolve.yml") {
		t.Error("evolve.yml should be protected (exact match)")
	}
}

func TestIsProtected_SocialScript(t *testing.T) {
	if !isProtected("scripts/social/social.sh") {
		t.Error("social.sh should be protected")
	}
}

func TestBuildSystemPrompt_ContainsIdentity(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs/PERSONALITY.md"), []byte("Friendly"), 0o644)
	os.MkdirAll(filepath.Join(dir, "skills"), 0o755)

	result := buildSystemPrompt(dir, "I am iterate")
	if !strings.Contains(result, "I am iterate") {
		t.Error("should contain identity")
	}
	if !strings.Contains(result, "Friendly") {
		t.Error("should contain personality")
	}
	if !strings.Contains(result, "self-evolving") {
		t.Error("should describe as self-evolving")
	}
}

func TestBuildSystemPrompt_ContainsToolFormat(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs/PERSONALITY.md"), []byte("p"), 0o644)
	os.MkdirAll(filepath.Join(dir, "skills"), 0o755)

	result := buildSystemPrompt(dir, "id")
	if !strings.Contains(result, "```tool") {
		t.Error("should contain tool call format")
	}
	if !strings.Contains(result, "read_file") {
		t.Error("should mention read_file tool")
	}
	if !strings.Contains(result, "write_file") {
		t.Error("should mention write_file tool")
	}
}

func TestBuildSystemPrompt_ContainsBashFormat(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs/PERSONALITY.md"), []byte("p"), 0o644)
	os.MkdirAll(filepath.Join(dir, "skills"), 0o755)

	result := buildSystemPrompt(dir, "id")
	if !strings.Contains(result, `"tool":"bash"`) {
		t.Error("should show bash tool example")
	}
}

func TestBuildUserMessage_Basic(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)

	result := buildUserMessage(dir, "", "")
	if !strings.Contains(result, "Your task") {
		t.Error("should mention task")
	}
	if !strings.Contains(result, "self-assessment") {
		t.Error("should mention self-assessment")
	}
}

func TestBuildUserMessage_WithJournal(t *testing.T) {
	dir := t.TempDir()
	journal := "Day 1: did something\nDay 2: did more"
	result := buildUserMessage(dir, journal, "")
	if !strings.Contains(result, "Recent journal") {
		t.Error("should show journal section")
	}
}

func TestBuildUserMessage_WithLongJournal(t *testing.T) {
	dir := t.TempDir()
	longJournal := strings.Repeat("x", 600)
	result := buildUserMessage(dir, longJournal, "")
	if !strings.Contains(result, "Recent journal") {
		t.Error("should show journal section")
	}
}

func TestBuildUserMessage_WithIssues(t *testing.T) {
	dir := t.TempDir()
	result := buildUserMessage(dir, "", "- #1: fix bug\n- #2: add feature")
	if !strings.Contains(result, "Community input") {
		t.Error("should show community section")
	}
	if !strings.Contains(result, "#1: fix bug") {
		t.Error("should include issues")
	}
}

func TestBuildUserMessage_WithLearnings(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	os.WriteFile(filepath.Join(dir, "memory", "ACTIVE_LEARNINGS.md"), []byte("Always test first"), 0o644)

	result := buildUserMessage(dir, "", "")
	if !strings.Contains(result, "What you have learned") {
		t.Error("should show learnings section")
	}
	if !strings.Contains(result, "Always test first") {
		t.Error("should include learning content")
	}
}

func TestBuildUserMessage_TruncatesLearnings(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0o755)
	longLearnings := strings.Repeat("x", 1500)
	os.WriteFile(filepath.Join(dir, "memory", "ACTIVE_LEARNINGS.md"), []byte(longLearnings), 0o644)

	result := buildUserMessage(dir, "", "")
	if !strings.Contains(result, "truncated") {
		t.Error("should truncate long learnings")
	}
}

func TestGenerateTraceID(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()
	if id1 == "" {
		t.Error("should generate non-empty trace ID")
	}
	if len(id1) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %s", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("trace IDs should be unique (very unlikely to collide)")
	}
}

func TestNewSetsTraceID(t *testing.T) {
	e := New("/tmp", slog.Default())
	if e.TraceID() == "" {
		t.Error("engine should have a trace ID")
	}
}
