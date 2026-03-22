package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// contextStats
// ---------------------------------------------------------------------------

func TestContextStats_Empty(t *testing.T) {
	result := contextStats([]iteragent.Message{})
	if !strings.Contains(result, "Messages: 0") {
		t.Errorf("expected 'Messages: 0', got %q", result)
	}
}

func TestContextStats_WithMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	result := contextStats(msgs)
	if !strings.Contains(result, "Messages: 2") {
		t.Errorf("expected 'Messages: 2', got %q", result)
	}
	if !strings.Contains(result, "tokens") {
		t.Error("expected 'tokens' in output")
	}
	if !strings.Contains(result, "context window") {
		t.Error("expected 'context window' in output")
	}
}

// ---------------------------------------------------------------------------
// exportConversation
// ---------------------------------------------------------------------------

func TestExportConversation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.md")
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	err := exportConversation(msgs, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Conversation Export") {
		t.Error("expected 'Conversation Export' header")
	}
	if !strings.Contains(content, "hello") {
		t.Error("expected 'hello' in export")
	}
	if !strings.Contains(content, "hi") {
		t.Error("expected 'hi' in export")
	}
}

func TestExportConversation_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.md")
	err := exportConversation([]iteragent.Message{}, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// findTodos
// ---------------------------------------------------------------------------

func TestFindTodos_NoFiles(t *testing.T) {
	dir := t.TempDir()
	todos := findTodos(dir)
	if len(todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(todos))
	}
}

func TestFindTodos_WithTODO(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("// TODO: fix this\npackage main\n// FIXME: later\n"), 0o644)

	todos := findTodos(dir)
	if len(todos) == 0 {
		t.Fatal("expected at least 1 todo")
	}
	found := false
	for _, todo := range todos {
		if strings.Contains(todo, "TODO") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TODO to be found")
	}
}

func TestFindTodos_WithHACK(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("// HACK: temporary workaround\npackage main\n"), 0o644)

	todos := findTodos(dir)
	if len(todos) == 0 {
		t.Fatal("expected at least 1 todo")
	}
}

func TestFindTodos_SkipsNonMatchingExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("TODO: this should not appear"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	todos := findTodos(dir)
	for _, todo := range todos {
		if strings.Contains(todo, "data.txt") {
			t.Error("should not find TODOs in .txt files")
		}
	}
}

func TestFindTodos_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "secret.go"), []byte("// TODO: secret todo\n"), 0o644)

	todos := findTodos(dir)
	for _, todo := range todos {
		if strings.Contains(todo, ".hidden") {
			t.Error("should not find TODOs in hidden directories")
		}
	}
}

// ---------------------------------------------------------------------------
// compactHard
// ---------------------------------------------------------------------------

func TestCompactHard_FewerMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	result := compactHard(msgs, 10)
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestCompactHard_ExactKeep(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	result := compactHard(msgs, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestCompactHard_MoreMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
		{Role: "assistant", Content: "a3"},
	}
	result := compactHard(msgs, 2)
	// Should keep first 2 (system) + last 2
	if len(result) != 4 {
		t.Errorf("expected 4 messages (2 system + 2 tail), got %d", len(result))
	}
	if result[0].Content != "sys1" {
		t.Errorf("expected first system message, got %q", result[0].Content)
	}
}

func TestCompactHard_OnlyTwoMessages(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "c"},
		{Role: "assistant", Content: "d"},
	}
	result := compactHard(msgs, 1)
	// keep first 2 + last 1 = 3
	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// formatPinnedMessages
// ---------------------------------------------------------------------------

func TestFormatPinnedMessages_Empty(t *testing.T) {
	result := formatPinnedMessages(nil)
	if result != "No pinned messages." {
		t.Errorf("expected 'No pinned messages.', got %q", result)
	}
}

func TestFormatPinnedMessages_ShortContent(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
	}
	result := formatPinnedMessages(msgs)
	if !strings.Contains(result, "hello") {
		t.Error("expected 'hello' in output")
	}
	if !strings.Contains(result, "[user]") {
		t.Error("expected '[user]' in output")
	}
}

func TestFormatPinnedMessages_LongContent(t *testing.T) {
	longContent := strings.Repeat("x", 100)
	msgs := []iteragent.Message{
		{Role: "assistant", Content: longContent},
	}
	result := formatPinnedMessages(msgs)
	if !strings.Contains(result, "…") {
		t.Error("expected truncation with ellipsis")
	}
}

func TestFormatPinnedMessages_Multiple(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
	}
	result := formatPinnedMessages(msgs)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// appendLearning
// ---------------------------------------------------------------------------

func TestAppendLearning(t *testing.T) {
	dir := t.TempDir()
	err := appendLearning(dir, "test learning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "test learning") {
		t.Error("expected learning to be in file")
	}
}

func TestAppendLearning_Multiple(t *testing.T) {
	dir := t.TempDir()
	for _, fact := range []string{"fact1", "fact2", "fact3"} {
		if err := appendLearning(dir, fact); err != nil {
			t.Fatalf("append %s: %v", fact, err)
		}
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// appendMemo
// ---------------------------------------------------------------------------

func TestAppendMemo(t *testing.T) {
	dir := t.TempDir()
	err := appendMemo(dir, "test memo content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "JOURNAL.md"))
	if err != nil {
		t.Fatalf("JOURNAL.md not created: %v", err)
	}
	if !strings.Contains(string(data), "test memo content") {
		t.Error("expected memo content in file")
	}
	if !strings.Contains(string(data), "Memo") {
		t.Error("expected 'Memo' header in file")
	}
}

// ---------------------------------------------------------------------------
// initProject
// ---------------------------------------------------------------------------

func TestInitProject_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	created := initProject(dir, "MyProject")

	if len(created) == 0 {
		t.Fatal("expected files to be created")
	}

	// Verify key files exist
	for _, file := range []string{"IDENTITY.md", "PERSONALITY.md", "JOURNAL.md", "DAY_COUNT"} {
		if _, err := os.Stat(filepath.Join(dir, file)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", file)
		}
	}
}

func TestInitProject_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("existing"), 0o644)

	created := initProject(dir, "MyProject")
	for _, file := range created {
		if file == "IDENTITY.md" {
			t.Error("should not re-create existing IDENTITY.md")
		}
	}
}

func TestInitProject_IDENTITYContent(t *testing.T) {
	dir := t.TempDir()
	initProject(dir, "TestProject")

	data, _ := os.ReadFile(filepath.Join(dir, "IDENTITY.md"))
	if !strings.Contains(string(data), "TestProject") {
		t.Error("IDENTITY.md should contain project name")
	}
}

// ---------------------------------------------------------------------------
// searchReplace
// ---------------------------------------------------------------------------

func TestSearchReplace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("hello world\nhello again\n"), 0o644)

	count, err := searchReplace(dir, "hello", "goodbye")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 file changed, got %d", count)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if strings.Contains(string(data), "hello") {
		t.Error("expected 'hello' to be replaced")
	}
	if !strings.Contains(string(data), "goodbye") {
		t.Error("expected 'goodbye' in output")
	}
}

func TestSearchReplace_NoMatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("hello world\n"), 0o644)

	count, err := searchReplace(dir, "nonexistent", "replacement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files changed, got %d", count)
	}
}

func TestSearchReplace_SkipsNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("hello world"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	count, err := searchReplace(dir, "hello", "goodbye")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 (.txt not matched), got %d", count)
	}
}
