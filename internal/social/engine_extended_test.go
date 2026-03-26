package social

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Engine.Run with no token (short-circuit path)
// ---------------------------------------------------------------------------

func TestRun_NoToken(t *testing.T) {
	e := New(t.TempDir(), "o", "r", slog.Default())
	e.token = "" // ensure no token
	err := e.Run(context.TODO(), nil)
	if err != nil {
		t.Errorf("expected nil error when token is empty, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Engine fields
// ---------------------------------------------------------------------------

func TestEngine_Fields(t *testing.T) {
	e := New("/test/path", "owner", "repo", slog.Default())
	if e.repoPath != "/test/path" {
		t.Errorf("expected repoPath '/test/path', got %q", e.repoPath)
	}
	if e.owner != "owner" {
		t.Errorf("expected owner 'owner', got %q", e.owner)
	}
	if e.repo != "repo" {
		t.Errorf("expected repo 'repo', got %q", e.repo)
	}
	if e.logger == nil {
		t.Error("logger should not be nil")
	}
	if e.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

// ---------------------------------------------------------------------------
// Discussion struct
// ---------------------------------------------------------------------------

func TestDiscussion_Fields(t *testing.T) {
	d := Discussion{
		ID:     "D_123",
		Number: 42,
		Title:  "Test Discussion",
		Body:   "This is the body",
		URL:    "https://github.com/test/repo/discussions/42",
		Comments: []Comment{
			{ID: "C1", Author: "alice", Body: "Great point!"},
			{ID: "C2", Author: "bob", Body: "I agree"},
		},
	}
	if d.ID != "D_123" {
		t.Errorf("expected ID 'D_123', got %q", d.ID)
	}
	if len(d.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(d.Comments))
	}
}

// ---------------------------------------------------------------------------
// Comment struct
// ---------------------------------------------------------------------------

func TestComment_Fields(t *testing.T) {
	c := Comment{ID: "C1", Author: "user", Body: "hello"}
	if c.Author != "user" {
		t.Errorf("expected author 'user', got %q", c.Author)
	}
}

// ---------------------------------------------------------------------------
// appendLearnings edge cases
// ---------------------------------------------------------------------------

func TestAppendLearnings_EmptyText(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	err := e.appendLearnings("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppendLearnings_Multiple(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	for i := 0; i < 3; i++ {
		if err := e.appendLearnings("learning " + string(rune('A'+i))); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "active_social_learnings.md"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Each write adds a block with date header
	if len(lines) < 3 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// appendLearningsJSONL edge cases
// ---------------------------------------------------------------------------

func TestAppendLearningsJSONL_AllEmpty(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	decisions := []socialDecision{
		{Learning: ""},
		{Learning: ""},
	}
	err := e.appendLearningsJSONL(decisions, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "memory", "social_learnings.jsonl")
	if data, err := os.ReadFile(path); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			t.Error("expected empty file for all-empty learnings")
		}
	}
}

func TestAppendLearningsJSONL_InvalidDayCount(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	decisions := []socialDecision{{Learning: "test"}}
	err := e.appendLearningsJSONL(decisions, "not-a-number")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppendLearningsJSONL_EmptyDayCount(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	decisions := []socialDecision{{Learning: "test insight"}}
	err := e.appendLearningsJSONL(decisions, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	if !strings.Contains(string(data), "test insight") {
		t.Error("expected insight in output")
	}
}

// ---------------------------------------------------------------------------
// WriteLearningsToMemory edge cases
// ---------------------------------------------------------------------------

func TestWriteLearningsToMemory_WithWho(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	err := e.WriteLearningsToMemory("alice", "streaming is fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	content := string(data)
	if !strings.Contains(content, "alice") {
		t.Error("expected 'alice' in output")
	}
	if !strings.Contains(content, "streaming is fast") {
		t.Error("expected 'streaming is fast' in output")
	}
}

func TestWriteLearningsToMemory_WithoutWho(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	err := e.WriteLearningsToMemory("", "insight only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	if !strings.Contains(string(data), "insight only") {
		t.Error("expected 'insight only' in output")
	}
}

// ---------------------------------------------------------------------------
// min function
// ---------------------------------------------------------------------------

func TestMin_EdgeCases(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{0, 0, 0},
		{-10, -5, -10},
		{1, 0, 0},
		{0, 1, 0},
		{100, 200, 100},
		{-1, 1, -1},
	}
	for _, c := range cases {
		if got := min(c.a, c.b); got != c.want {
			t.Errorf("min(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// socialDecision struct
// ---------------------------------------------------------------------------

func TestSocialDecision_Fields(t *testing.T) {
	d := socialDecision{
		DiscussionID: "D_1",
		Reply:        "Great post!",
		Learning:     "users like streaming",
	}
	if d.DiscussionID != "D_1" {
		t.Errorf("expected DiscussionID 'D_1', got %q", d.DiscussionID)
	}
	if d.Reply != "Great post!" {
		t.Errorf("expected Reply 'Great post!', got %q", d.Reply)
	}
	if d.Learning != "users like streaming" {
		t.Errorf("expected Learning, got %q", d.Learning)
	}
}

func TestSocialDecision_WithNewDiscussion(t *testing.T) {
	d := socialDecision{
		DiscussionID: "D_1",
		NewDiscussion: &struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}{
			Title: "New Topic",
			Body:  "Let's discuss this",
		},
	}
	if d.NewDiscussion == nil {
		t.Fatal("expected NewDiscussion to be set")
	}
	if d.NewDiscussion.Title != "New Topic" {
		t.Errorf("expected title 'New Topic', got %q", d.NewDiscussion.Title)
	}
}
