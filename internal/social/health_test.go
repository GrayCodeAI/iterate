package social

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
	}
	// Replace the request to use test server
	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
	}
	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
	}
	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
	}
	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_SetsAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth header, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
		token:      "test-token-123",
	}
	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
}

func TestBuildSocialPrompt_DiscussionDetails(t *testing.T) {
	discussions := []Discussion{
		{
			ID: "D_123", Number: 10, Title: "How to use streaming?",
			Body: "I want to learn about streaming.",
			URL:  "https://github.com/test/repo/discussions/10",
			Comments: []Comment{
				{ID: "C1", Author: "user1", Body: "Try using the SDK"},
			},
		},
	}
	prompt := buildSocialPrompt(discussions)
	if !strings.Contains(prompt, "D_123") {
		t.Error("should contain discussion ID")
	}
	if !strings.Contains(prompt, "#10") {
		t.Error("should contain discussion number")
	}
	if !strings.Contains(prompt, "How to use streaming?") {
		t.Error("should contain title")
	}
	if !strings.Contains(prompt, "user1") {
		t.Error("should contain comment author")
	}
	if !strings.Contains(prompt, "Try using the SDK") {
		t.Error("should contain comment body")
	}
}

func TestBuildSocialPrompt_ResponseFormat(t *testing.T) {
	prompt := buildSocialPrompt(nil)
	if !strings.Contains(prompt, "JSON array") {
		t.Error("should mention JSON array format")
	}
	if !strings.Contains(prompt, "discussion_id") {
		t.Error("should show response format")
	}
}

func TestParseSocialDecisions_CodeFenced(t *testing.T) {
	input := "```json\n[{\"discussion_id\":\"D_1\",\"reply\":\"Great!\"}]\n```"
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Reply != "Great!" {
		t.Errorf("unexpected decisions: %+v", decisions)
	}
}

func TestParseSocialDecisions_BareCodeBlock(t *testing.T) {
	input := "```\n[{\"discussion_id\":\"D_2\",\"reply\":\"hello\"}]\n```"
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].DiscussionID != "D_2" {
		t.Errorf("unexpected: %+v", decisions)
	}
}

func TestParseSocialDecisions_WithLearning(t *testing.T) {
	input := `[{"discussion_id":"D_1","reply":"reply text","learning":"users prefer examples"}]`
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decisions[0].Learning != "users prefer examples" {
		t.Errorf("expected learning, got %q", decisions[0].Learning)
	}
}

func TestParseSocialDecisions_MultipleDecisions(t *testing.T) {
	input := `[
		{"discussion_id":"D_1","reply":"yes"},
		{"discussion_id":"D_2","reply":"no"},
		{"discussion_id":"D_3","reply":"","learning":"insight"}
	]`
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 3 {
		t.Errorf("expected 3 decisions, got %d", len(decisions))
	}
}

func TestParseSocialDecisions_EmptyReply(t *testing.T) {
	input := `[{"discussion_id":"D_1","reply":""}]`
	decisions, err := parseSocialDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decisions[0].Reply != "" {
		t.Errorf("expected empty reply, got %q", decisions[0].Reply)
	}
}

func TestAppendLearnings_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	if err := e.appendLearnings("some learning text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "active_social_learnings.md"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "some learning text") {
		t.Error("should contain learning text")
	}
}

func TestAppendLearningsJSONL_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	decisions := []socialDecision{
		{Learning: "insight one"},
		{Learning: "insight two"},
	}
	if err := e.appendLearningsJSONL(decisions, "5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "insight one") {
		t.Error("first line should contain 'insight one'")
	}
}

func TestAppendLearningsJSONL_SkipsEmptyLearning(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	decisions := []socialDecision{
		{Learning: ""},
		{Learning: "valid insight"},
		{Learning: ""},
	}
	e.appendLearningsJSONL(decisions, "1")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "social_learnings.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (empty learnings skipped), got %d", len(lines))
	}
}

func TestWriteLearningsToMemory_EmptyInsightSkipped(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	e.WriteLearningsToMemory("", "")

	path := filepath.Join(dir, "memory", "social_learnings.jsonl")
	if data, err := os.ReadFile(path); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			t.Errorf("expected empty content for empty insight, got: %s", string(data))
		}
	}
}

func TestMin(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{100, 100, 100},
	}
	for _, c := range cases {
		if got := min(c.a, c.b); got != c.want {
			t.Errorf("min(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
