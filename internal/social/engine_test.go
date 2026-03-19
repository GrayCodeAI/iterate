package social

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Engine instantiation
// ---------------------------------------------------------------------------

func TestNew_NonNil(t *testing.T) {
	e := New("/tmp/repo", "GrayCodeAI", "iterate", slog.Default())
	if e == nil {
		t.Fatal("New returned nil")
	}
	if e.repoPath != "/tmp/repo" {
		t.Errorf("expected repoPath /tmp/repo, got %q", e.repoPath)
	}
	if e.owner != "GrayCodeAI" {
		t.Errorf("expected owner GrayCodeAI, got %q", e.owner)
	}
	if e.repo != "iterate" {
		t.Errorf("expected repo iterate, got %q", e.repo)
	}
}

func TestNew_ClientNotNil(t *testing.T) {
	e := New("/tmp/repo", "o", "r", slog.Default())
	if e.client == nil {
		t.Error("http client should not be nil")
	}
}

// ---------------------------------------------------------------------------
// buildSocialPrompt
// ---------------------------------------------------------------------------

func TestBuildSocialPrompt_Empty(t *testing.T) {
	prompt := buildSocialPrompt(nil)
	if !strings.Contains(prompt, "JSON array") {
		t.Error("prompt should mention JSON array format")
	}
}

func TestBuildSocialPrompt_WithDiscussions(t *testing.T) {
	discussions := []Discussion{
		{
			ID:     "D_abc",
			Number: 42,
			Title:  "How does streaming work?",
			Body:   "I want to understand streaming.",
			Comments: []Comment{
				{Author: "alice", Body: "Great question!"},
			},
		},
	}
	prompt := buildSocialPrompt(discussions)

	if !strings.Contains(prompt, "D_abc") {
		t.Error("prompt should contain discussion ID")
	}
	if !strings.Contains(prompt, "#42") {
		t.Error("prompt should contain discussion number")
	}
	if !strings.Contains(prompt, "How does streaming work?") {
		t.Error("prompt should contain discussion title")
	}
	if !strings.Contains(prompt, "alice") {
		t.Error("prompt should contain comment author")
	}
	if !strings.Contains(prompt, "Great question!") {
		t.Error("prompt should contain comment body")
	}
}

func TestBuildSocialPrompt_MultipleDiscussions(t *testing.T) {
	discussions := []Discussion{
		{ID: "D_1", Number: 1, Title: "First"},
		{ID: "D_2", Number: 2, Title: "Second"},
	}
	prompt := buildSocialPrompt(discussions)

	if !strings.Contains(prompt, "D_1") || !strings.Contains(prompt, "D_2") {
		t.Error("prompt should contain both discussion IDs")
	}
	if !strings.Contains(prompt, "#1") || !strings.Contains(prompt, "#2") {
		t.Error("prompt should contain both discussion numbers")
	}
}

// ---------------------------------------------------------------------------
// parseSocialDecisions
// ---------------------------------------------------------------------------

func TestParseSocialDecisions_ValidJSON(t *testing.T) {
	input := `[{"discussion_id":"D_abc","reply":"Hello!","learning":"users like streaming"}]`
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].DiscussionID != "D_abc" {
		t.Errorf("expected DiscussionID D_abc, got %q", decisions[0].DiscussionID)
	}
	if decisions[0].Reply != "Hello!" {
		t.Errorf("expected reply 'Hello!', got %q", decisions[0].Reply)
	}
	if decisions[0].Learning != "users like streaming" {
		t.Errorf("expected learning, got %q", decisions[0].Learning)
	}
}

func TestParseSocialDecisions_MarkdownFence(t *testing.T) {
	input := "```json\n[{\"discussion_id\":\"D_1\",\"reply\":\"\"}]\n```"
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error for markdown-fenced JSON: %v", err)
	}
	if len(decisions) != 1 || decisions[0].DiscussionID != "D_1" {
		t.Errorf("unexpected decisions: %+v", decisions)
	}
}

func TestParseSocialDecisions_BareCodeFence(t *testing.T) {
	input := "```\n[{\"discussion_id\":\"D_2\",\"reply\":\"hi\"}]\n```"
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Reply != "hi" {
		t.Errorf("unexpected decisions: %+v", decisions)
	}
}

func TestParseSocialDecisions_InvalidJSON(t *testing.T) {
	_, err := parseSocialDecisions("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseSocialDecisions_Empty(t *testing.T) {
	decisions, err := parseSocialDecisions("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestParseSocialDecisions_WithNewDiscussion(t *testing.T) {
	input := `[{"discussion_id":"D_x","reply":"","new_discussion":{"title":"New Topic","body":"Let us discuss"}}]`
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decisions[0].NewDiscussion == nil {
		t.Fatal("expected NewDiscussion to be set")
	}
	if decisions[0].NewDiscussion.Title != "New Topic" {
		t.Errorf("expected title 'New Topic', got %q", decisions[0].NewDiscussion.Title)
	}
}

// ---------------------------------------------------------------------------
// extractLearnings
// ---------------------------------------------------------------------------

func TestExtractLearnings_Empty(t *testing.T) {
	if got := extractLearnings(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractLearnings_NoLearnings(t *testing.T) {
	decisions := []socialDecision{
		{DiscussionID: "D_1", Reply: "hello", Learning: ""},
		{DiscussionID: "D_2", Reply: "", Learning: ""},
	}
	if got := extractLearnings(decisions); got != "" {
		t.Errorf("expected empty string when no learnings, got %q", got)
	}
}

func TestExtractLearnings_WithLearnings(t *testing.T) {
	decisions := []socialDecision{
		{Learning: "users want more examples"},
		{Learning: ""},
		{Learning: "streaming is popular"},
	}
	result := extractLearnings(decisions)
	if !strings.Contains(result, "users want more examples") {
		t.Error("expected first learning")
	}
	if !strings.Contains(result, "streaming is popular") {
		t.Error("expected third learning")
	}
}

func TestExtractLearnings_JoinedWithNewline(t *testing.T) {
	decisions := []socialDecision{
		{Learning: "insight A"},
		{Learning: "insight B"},
	}
	result := extractLearnings(decisions)
	parts := strings.Split(result, "\n")
	if len(parts) != 2 {
		t.Errorf("expected 2 lines joined by newline, got %d parts: %q", len(parts), result)
	}
}

// ---------------------------------------------------------------------------
// truncate helper
// ---------------------------------------------------------------------------

func TestTruncate_Short(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should pass through, got %q", got)
	}
}

func TestTruncate_Long(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := truncate(long, 10)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected '...' suffix, got %q", got)
	}
	if len(got) > 15 {
		t.Errorf("truncated string too long: %d", len(got))
	}
}

func TestTruncate_Exact(t *testing.T) {
	s := "abcde"
	if got := truncate(s, 5); got != "abcde" {
		t.Errorf("string at limit should not be truncated, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// WriteLearningsToMemory (disk I/O smoke test)
// ---------------------------------------------------------------------------

func TestWriteLearningsToMemory_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	if err := e.WriteLearningsToMemory("alice", "streaming is fast"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	if err != nil {
		t.Fatalf("social_learnings.jsonl not created: %v", err)
	}
	if !strings.Contains(string(data), "streaming is fast") {
		t.Errorf("expected insight in file, got: %s", string(data))
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("expected who (alice) in file, got: %s", string(data))
	}
}

func TestWriteLearningsToMemory_Appends(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	for i := 0; i < 3; i++ {
		if err := e.WriteLearningsToMemory("", "insight"); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d", len(lines))
	}
}

func TestWriteLearningsToMemory_EmptyInsight(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	// Empty insight (and no who) should not write any JSON lines.
	if err := e.WriteLearningsToMemory("", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "memory", "social_learnings.jsonl")
	if data, err := os.ReadFile(path); err == nil {
		// File exists — it should be empty (no JSON lines written).
		if len(strings.TrimSpace(string(data))) > 0 {
			t.Errorf("expected no content for empty insight, got: %s", string(data))
		}
	}
	// File may not exist at all — also fine.
}
