package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Token-missing paths (no network required)
// ---------------------------------------------------------------------------

func TestFetchIssues_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	result, err := FetchIssues(context.Background(), "owner", "repo", []IssueType{IssueTypeInput}, 10)
	if err != nil {
		t.Errorf("expected nil error when token absent, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when token absent, got %v", result)
	}
}

func TestFetchDiscussions_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	result, err := FetchDiscussions(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Errorf("expected nil error when token absent, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when token absent, got %v", result)
	}
}

func TestPostReply_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	err := PostReply(context.Background(), "owner", "repo", 1, "hello")
	if err == nil {
		t.Fatal("expected error when GITHUB_TOKEN is not set")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected GITHUB_TOKEN in error message, got %q", err.Error())
	}
}

func TestPostDiscussionReply_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	err := PostDiscussionReply(context.Background(), "owner", "repo", 1, "reply text")
	if err == nil {
		t.Fatal("expected error when GITHUB_TOKEN is not set")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected GITHUB_TOKEN in error message, got %q", err.Error())
	}
}

func TestCreateDiscussion_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	err := CreateDiscussion(context.Background(), "owner", "repo", "Ideas", "My Title", "My Body")
	if err == nil {
		t.Fatal("expected error when GITHUB_TOKEN is not set")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected GITHUB_TOKEN in error message, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// FetchDiscussions — via http.DefaultClient mock
// ---------------------------------------------------------------------------

// withMockHTTPClient temporarily replaces the GraphQL client with a mock server.
func withMockHTTPClient(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	mockClient := &http.Client{
		Transport: &mockTransportRedirect{target: srv.URL},
	}
	SetGraphQLClientForTests(mockClient)
	return func() {
		SetGraphQLClientForTests(nil)
		srv.Close()
	}
}

// mockTransportRedirect rewrites all requests to a fixed target URL.
type mockTransportRedirect struct {
	target string
}

func (m *mockTransportRedirect) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace host with test server, keep path and query.
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(m.target, "http://")
	return http.DefaultTransport.RoundTrip(req2)
}

// buildDiscussionsGraphQLResponse builds a mock GraphQL response for discussions.
func buildDiscussionsResponse(nodes []map[string]interface{}) []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"repository": map[string]interface{}{
				"discussions": map[string]interface{}{
					"nodes": nodes,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestFetchDiscussions_Success(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	nodes := []map[string]interface{}{
		{
			"number":       float64(10),
			"title":        "How to stream?",
			"body":         "Streaming question body",
			"authorLogin":  "alice",
			"url":          "https://github.com/owner/repo/discussions/10",
			"comments":     map[string]interface{}{"totalCount": float64(3)},
			"isAnswered":   false,
			"categoryName": "Q&A",
		},
	}

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nodes))
	})
	defer restore()

	result, err := FetchDiscussions(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 discussion, got %d", len(result))
	}
	if result[0].Title != "How to stream?" {
		t.Errorf("expected title 'How to stream?', got %q", result[0].Title)
	}
	if result[0].Author != "alice" {
		t.Errorf("expected author 'alice', got %q", result[0].Author)
	}
	if result[0].Category != "Q&A" {
		t.Errorf("expected category 'Q&A', got %q", result[0].Category)
	}
	if result[0].Comments != 3 {
		t.Errorf("expected 3 comments, got %d", result[0].Comments)
	}
}

func TestFetchDiscussions_MultipleResults(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	nodes := []map[string]interface{}{
		{"number": float64(1), "title": "Alpha", "body": "", "authorLogin": "a", "url": "", "comments": map[string]interface{}{"totalCount": float64(5)}, "isAnswered": false, "categoryName": "General"},
		{"number": float64(2), "title": "Beta", "body": "", "authorLogin": "b", "url": "", "comments": map[string]interface{}{"totalCount": float64(10)}, "isAnswered": false, "categoryName": "General"},
	}

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nodes))
	})
	defer restore()

	result, err := FetchDiscussions(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 discussions, got %d", len(result))
	}
	// Result is sorted by comment count descending: Beta(10) before Alpha(5)
	if result[0].Title != "Beta" {
		t.Errorf("expected 'Beta' first (more comments), got %q", result[0].Title)
	}
}

func TestFetchDiscussions_LimitApplied(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	nodes := make([]map[string]interface{}, 5)
	for i := range nodes {
		nodes[i] = map[string]interface{}{
			"number": float64(i + 1), "title": "d", "body": "", "authorLogin": "u",
			"url": "", "comments": map[string]interface{}{"totalCount": float64(0)},
			"isAnswered": false, "categoryName": "General",
		}
	}

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nodes))
	})
	defer restore()

	result, err := FetchDiscussions(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 results after limit, got %d", len(result))
	}
}

func TestFetchDiscussions_BodyTruncated(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	longBody := strings.Repeat("x", 600)
	nodes := []map[string]interface{}{
		{
			"number": float64(1), "title": "Long body", "body": longBody,
			"authorLogin": "u", "url": "",
			"comments":     map[string]interface{}{"totalCount": float64(0)},
			"isAnswered":   false,
			"categoryName": "General",
		},
	}

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nodes))
	})
	defer restore()

	result, err := FetchDiscussions(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected 1 discussion")
	}
	if len(result[0].Body) > 510 {
		t.Errorf("expected body truncated to ~503 chars, got %d", len(result[0].Body))
	}
	if !strings.HasSuffix(result[0].Body, "...") {
		t.Errorf("expected body to end with '...', got %q", result[0].Body[len(result[0].Body)-10:])
	}
}

func TestFetchDiscussions_EmptyNodes(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nil))
	})
	defer restore()

	result, err := FetchDiscussions(context.Background(), "owner", "repo", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 discussions, got %d", len(result))
	}
}

func TestFetchDiscussions_RequestContainsAuthHeader(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "my-secret-token")

	var capturedAuth string
	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildDiscussionsResponse(nil))
	})
	defer restore()

	FetchDiscussions(context.Background(), "owner", "repo", 10)

	if !strings.Contains(capturedAuth, "my-secret-token") {
		t.Errorf("expected token in Authorization header, got %q", capturedAuth)
	}
}
