package community

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// PostDiscussionReply — via mock HTTP
// ---------------------------------------------------------------------------

func TestPostDiscussionReply_WithMock(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"repository":{"discussion":{"id":"D_test"}}}}`))
	})
	defer restore()

	err := PostDiscussionReply(context.Background(), "owner", "repo", 1, "test reply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostDiscussionReply_ServerError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	defer restore()

	err := PostDiscussionReply(context.Background(), "owner", "repo", 1, "test reply")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

// ---------------------------------------------------------------------------
// CreateDiscussion — via mock HTTP
// ---------------------------------------------------------------------------

func TestCreateDiscussion_WithMock(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"createDiscussion":{"discussion":{"number":1,"url":"https://github.com/test"}}}}`))
	})
	defer restore()

	err := CreateDiscussion(context.Background(), "owner", "repo", "General", "Test Title", "Test Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDiscussion_ServerError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	restore := withMockHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	defer restore()

	err := CreateDiscussion(context.Background(), "owner", "repo", "General", "Test Title", "Test Body")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

// ---------------------------------------------------------------------------
// FetchIssues — via mock HTTP (indirectly through NewGitHubClient)
// ---------------------------------------------------------------------------

// Note: FetchIssues uses go-github client which can't easily be mocked
// via httptest. The no-token path is already tested.

// ---------------------------------------------------------------------------
// FormatIssuesByType additional tests
// ---------------------------------------------------------------------------

func TestFormatIssuesByType_ZeroVotes(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeSelf: {
			{Number: 1, Title: "Zero votes", NetVotes: 0},
		},
	}
	result := FormatIssuesByType(issues)
	if !strings.Contains(result, "#1") {
		t.Error("expected issue number")
	}
}

func TestFormatIssuesByType_AllThreeTypes(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeInput:      {{Number: 1, Title: "Input", NetVotes: 5}},
		IssueTypeSelf:       {{Number: 2, Title: "Self", NetVotes: 0}},
		IssueTypeHelpWanted: {{Number: 3, Title: "Help", NetVotes: 3}},
	}
	result := FormatIssuesByType(issues)

	if !strings.Contains(result, "Community Suggestions") {
		t.Error("expected Community Suggestions header")
	}
	if !strings.Contains(result, "Self-Generated TODOs") {
		t.Error("expected Self-Generated TODOs header")
	}
	if !strings.Contains(result, "Help Wanted") {
		t.Error("expected Help Wanted header")
	}
}

// ---------------------------------------------------------------------------
// FormatDiscussions additional tests
// ---------------------------------------------------------------------------

func TestFormatDiscussions_CommentsField(t *testing.T) {
	d := []Discussion{
		{Number: 1, Title: "Test", Comments: 42, Category: "General"},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "42 comments") {
		t.Error("expected comment count in output")
	}
}

func TestFormatDiscussions_CategoryField(t *testing.T) {
	d := []Discussion{
		{Number: 1, Title: "Test", Category: "Ideas"},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "Ideas") {
		t.Error("expected category in output")
	}
}

// ---------------------------------------------------------------------------
// Discussion struct fields
// ---------------------------------------------------------------------------

func TestDiscussion_IsAnswered(t *testing.T) {
	d := Discussion{
		Number:     1,
		Title:      "Answered",
		IsAnswered: true,
	}
	if !d.IsAnswered {
		t.Error("IsAnswered should be true")
	}
}

func TestDiscussion_CommentsCount(t *testing.T) {
	d := Discussion{
		Number:   1,
		Title:    "Test",
		Comments: 10,
	}
	if d.Comments != 10 {
		t.Errorf("expected Comments 10, got %d", d.Comments)
	}
}

// ---------------------------------------------------------------------------
// Issue struct
// ---------------------------------------------------------------------------

func TestIssue_FieldsExtended(t *testing.T) {
	i := Issue{
		Number:   42,
		Title:    "Feature Request",
		Body:     "Please add X",
		NetVotes: 15,
		URL:      "https://github.com/example/repo/issues/42",
		Type:     IssueTypeInput,
	}
	if i.Number != 42 || i.NetVotes != 15 || i.Type != IssueTypeInput {
		t.Errorf("unexpected issue fields: %+v", i)
	}
}
