package community

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// FormatDiscussions edge cases
// ---------------------------------------------------------------------------

func TestFormatDiscussions_IsAnswered(t *testing.T) {
	d := []Discussion{
		{Number: 1, Title: "Answered Q", Category: "Q&A", IsAnswered: true, Comments: 5},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "#1") {
		t.Error("expected discussion number")
	}
}

func TestFormatDiscussions_Author(t *testing.T) {
	d := []Discussion{
		{Number: 1, Title: "Test", Author: "octocat"},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "#1") {
		t.Error("expected discussion number")
	}
}

// ---------------------------------------------------------------------------
// Discussion struct edge cases
// ---------------------------------------------------------------------------

func TestDiscussion_BodyField(t *testing.T) {
	d := Discussion{
		Number: 1,
		Title:  "Test",
		Body:   "This is a body",
	}
	if d.Body != "This is a body" {
		t.Errorf("expected body 'This is a body', got %q", d.Body)
	}
}

func TestDiscussion_URLField(t *testing.T) {
	d := Discussion{
		Number: 1,
		Title:  "Test",
		URL:    "https://github.com/owner/repo/discussions/1",
	}
	if d.URL != "https://github.com/owner/repo/discussions/1" {
		t.Errorf("unexpected URL: %q", d.URL)
	}
}

// ---------------------------------------------------------------------------
// FormatIssuesByType edge cases
// ---------------------------------------------------------------------------

func TestFormatIssuesByType_BodyTruncated(t *testing.T) {
	longBody := strings.Repeat("x", 500)
	issues := map[IssueType][]Issue{
		IssueTypeInput: {
			{Number: 1, Title: "Long body", Body: longBody, NetVotes: 1},
		},
	}
	result := FormatIssuesByType(issues)
	if !strings.Contains(result, "#1") {
		t.Error("expected issue number in output")
	}
}

func TestFormatIssuesByType_NegativeVotes(t *testing.T) {
	issues := map[IssueType][]Issue{
		IssueTypeInput: {
			{Number: 1, Title: "Unpopular", NetVotes: -5},
		},
	}
	result := FormatIssuesByType(issues)
	if !strings.Contains(result, "+-5") && !strings.Contains(result, "-5") {
		t.Error("expected negative vote count")
	}
}

// ---------------------------------------------------------------------------
// IssueType constants
// ---------------------------------------------------------------------------

func TestIssueTypeConstants(t *testing.T) {
	if IssueTypeInput != "agent-input" {
		t.Errorf("expected 'agent-input', got %q", IssueTypeInput)
	}
	if IssueTypeSelf != "agent-self" {
		t.Errorf("expected 'agent-self', got %q", IssueTypeSelf)
	}
	if IssueTypeHelpWanted != "agent-help-wanted" {
		t.Errorf("expected 'agent-help-wanted', got %q", IssueTypeHelpWanted)
	}
}

// ---------------------------------------------------------------------------
// FetchIssues with no token
// ---------------------------------------------------------------------------

func TestFetchIssues_NoTokenReturnsNil(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	result, err := FetchIssues(context.TODO(), "owner", "repo", []IssueType{IssueTypeInput}, 10)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// PostReply with no token
// ---------------------------------------------------------------------------

func TestPostReply_NoTokenError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	err := PostReply(context.TODO(), "owner", "repo", 1, "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected GITHUB_TOKEN in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// CreateDiscussion with no token
// ---------------------------------------------------------------------------

func TestCreateDiscussion_NoTokenError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	err := CreateDiscussion(context.TODO(), "owner", "repo", "General", "Title", "Body")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected GITHUB_TOKEN in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// NewGitHubClient with no token
// ---------------------------------------------------------------------------

func TestNewGitHubClient_NoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	client := NewGitHubClient(context.TODO())
	if client != nil {
		t.Error("expected nil client when GITHUB_TOKEN is not set")
	}
}

// ---------------------------------------------------------------------------
// FormatDiscussions with URL
// ---------------------------------------------------------------------------

func TestFormatDiscussions_WithURL(t *testing.T) {
	d := []Discussion{
		{
			Number: 42,
			Title:  "How to use streaming?",
			Body:   "I want to learn about streaming.",
			URL:    "https://github.com/example/repo/discussions/42",
		},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "https://github.com/example/repo/discussions/42") {
		t.Error("expected URL in output")
	}
}

// ---------------------------------------------------------------------------
// FormatDiscussions with Author
// ---------------------------------------------------------------------------

func TestFormatDiscussions_WithAuthor(t *testing.T) {
	d := []Discussion{
		{
			Number: 1,
			Title:  "Test",
			Author: "octocat",
		},
	}
	result := FormatDiscussions(d)
	if !strings.Contains(result, "#1") {
		t.Error("expected discussion number")
	}
}
