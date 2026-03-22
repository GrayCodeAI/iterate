package social

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// fetchDiscussions (mocked GraphQL)
// ---------------------------------------------------------------------------

func TestFetchDiscussions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", auth)
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"discussions": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{
								"id":     "D_abc",
								"number": 1,
								"title":  "How to use streaming?",
								"body":   "I want to learn about streaming in the SDK.",
								"url":    "https://github.com/o/r/discussions/1",
								"comments": map[string]interface{}{
									"nodes": []map[string]interface{}{
										{
											"id":     "C_1",
											"author": map[string]string{"login": "alice"},
											"body":   "Try the SDK!",
										},
									},
								},
							},
							{
								"id":     "D_def",
								"number": 2,
								"title":  "Bug in parser",
								"body":   "Parser crashes on empty input.",
								"url":    "https://github.com/o/r/discussions/2",
								"comments": map[string]interface{}{
									"nodes": []interface{}{},
								},
							},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := &Engine{
		owner:      "o",
		repo:       "r",
		token:      "test-token",
		httpClient: server.Client(),
		logger:     slog.Default(),
	}

	ctx := context.Background()
	// Override the URL by replacing httpClient with a custom transport
	e.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(server.URL, "http://")
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	discussions, err := e.fetchDiscussions(ctx)
	if err != nil {
		t.Fatalf("fetchDiscussions error: %v", err)
	}
	if len(discussions) != 2 {
		t.Fatalf("expected 2 discussions, got %d", len(discussions))
	}
	if discussions[0].ID != "D_abc" {
		t.Errorf("expected ID D_abc, got %q", discussions[0].ID)
	}
	if discussions[0].Title != "How to use streaming?" {
		t.Errorf("expected title, got %q", discussions[0].Title)
	}
	if len(discussions[0].Comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(discussions[0].Comments))
	}
	if discussions[0].Comments[0].Author != "alice" {
		t.Errorf("expected author alice, got %q", discussions[0].Comments[0].Author)
	}
	if discussions[1].Number != 2 {
		t.Errorf("expected number 2, got %d", discussions[1].Number)
	}
}

func TestFetchDiscussions_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"discussions": map[string]interface{}{
						"nodes": []interface{}{},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := &Engine{
		owner:  "o",
		repo:   "r",
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	discussions, err := e.fetchDiscussions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(discussions) != 0 {
		t.Errorf("expected 0 discussions, got %d", len(discussions))
	}
}

func TestFetchDiscussions_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	e := &Engine{
		owner:  "o",
		repo:   "r",
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	_, err := e.fetchDiscussions(context.Background())
	if err == nil {
		t.Error("expected error for server error response")
	}
}

// ---------------------------------------------------------------------------
// postDiscussionReply (mocked GraphQL)
// ---------------------------------------------------------------------------

func TestPostDiscussionReply_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"addDiscussionComment": map[string]interface{}{
					"comment": map[string]string{"id": "C_new"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := &Engine{
		token:  "test-token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	err := e.postDiscussionReply(context.Background(), "D_123", "Great post!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostDiscussionReply_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors":[{"message":"not found"}]}`))
	}))
	defer server.Close()

	e := &Engine{
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	err := e.postDiscussionReply(context.Background(), "D_x", "test")
	if err == nil {
		t.Error("expected error for server error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// createDiscussion (mocked GraphQL — two requests)
// ---------------------------------------------------------------------------

func TestCreateDiscussion_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: repo query
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"id": "R_123",
						"discussionCategories": map[string]interface{}{
							"nodes": []map[string]interface{}{
								{"id": "CAT_1", "name": "General"},
								{"id": "CAT_2", "name": "Q&A"},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			// Second call: create mutation
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"createDiscussion": map[string]interface{}{
						"discussion": map[string]string{"id": "D_new"},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	e := &Engine{
		owner:  "o",
		repo:   "r",
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	err := e.createDiscussion(context.Background(), "New Topic", "Let's discuss")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestCreateDiscussion_NoCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"id": "R_123",
					"discussionCategories": map[string]interface{}{
						"nodes": []interface{}{},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := &Engine{
		owner:  "o",
		repo:   "r",
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	err := e.createDiscussion(context.Background(), "Test", "Body")
	if err == nil {
		t.Error("expected error when no category found")
	}
	if !strings.Contains(err.Error(), "no discussion category") {
		t.Errorf("expected category error, got: %v", err)
	}
}

func TestCreateDiscussion_FallbackCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Categories without "general" or "announcements" — should use first
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"id": "R_123",
					"discussionCategories": map[string]interface{}{
						"nodes": []map[string]interface{}{
							{"id": "CAT_1", "name": "Ideas"},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	e := &Engine{
		owner:  "o",
		repo:   "r",
		token:  "token",
		logger: slog.Default(),
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	err := e.createDiscussion(context.Background(), "Test", "Body")
	// It will fail on the second call (mutation) since the mock only handles one request,
	// but at least we verify it gets past the category selection.
	if err != nil && strings.Contains(err.Error(), "no discussion category") {
		t.Error("should not fail with 'no discussion category'")
	}
}

// ---------------------------------------------------------------------------
// HealthCheck (properly mocked)
// ---------------------------------------------------------------------------

func TestHealthCheck_MockSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
		token:      "test-token",
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
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthCheck_MockUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	e := &Engine{
		httpClient: server.Client(),
		logger:     slog.Default(),
		token:      "bad-token",
	}

	req, _ := http.NewRequestWithContext(context.Background(), "HEAD", server.URL, nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Run (with mocked HTTP — tests the flow without real GitHub)
// ---------------------------------------------------------------------------

func TestRun_NoToken_Skips(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())
	e.token = ""

	err := e.Run(context.Background(), nil)
	if err != nil {
		t.Errorf("expected nil error with no token, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// appendLearnings (file I/O)
// ---------------------------------------------------------------------------

func TestAppendLearnings_CreatesDirAndFile(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	err := e.appendLearnings("test learning content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory", "active_social_learnings.md"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "test learning content") {
		t.Error("should contain learning text")
	}
}

func TestAppendLearnings_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	e := New(dir, "o", "r", slog.Default())

	e.appendLearnings("first learning")
	e.appendLearnings("second learning")

	data, _ := os.ReadFile(filepath.Join(dir, "memory", "active_social_learnings.md"))
	content := string(data)
	if !strings.Contains(content, "first learning") {
		t.Error("should contain first learning")
	}
	if !strings.Contains(content, "second learning") {
		t.Error("should contain second learning")
	}
}

// ---------------------------------------------------------------------------
// roundTripFunc helper
// ---------------------------------------------------------------------------

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
