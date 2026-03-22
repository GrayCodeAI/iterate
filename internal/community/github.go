package community

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/google/go-github/v61/github"
	"golang.org/x/oauth2"
)

// NewGitHubClient creates an authenticated go-github client using GITHUB_TOKEN.
// Returns nil if the token is not set.
func NewGitHubClient(ctx context.Context) *github.Client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

type IssueType string

const (
	IssueTypeInput      IssueType = "agent-input"
	IssueTypeSelf       IssueType = "agent-self"
	IssueTypeHelpWanted IssueType = "agent-help-wanted"
)

// Issue represents a community-submitted GitHub issue.
type Issue struct {
	Number   int
	Title    string
	Body     string
	NetVotes int // thumbsup - thumbsdown reactions
	URL      string
	Type     IssueType
}

// FetchIssues retrieves issues by label type, sorted by net vote score.
func FetchIssues(ctx context.Context, owner, repo string, issueTypes []IssueType, limit int) (map[IssueType][]Issue, error) {
	client := NewGitHubClient(ctx)
	if client == nil {
		slog.Warn("GITHUB_TOKEN not set, skipping community issues")
		return nil, nil
	}

	result := make(map[IssueType][]Issue)
	for _, issueType := range issueTypes {
		ghIssues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			Labels:      []string{string(issueType)},
			State:       "open",
			ListOptions: github.ListOptions{PerPage: 50},
		})
		if err != nil {
			return nil, fmt.Errorf("fetch issues for label %s: %w", issueType, err)
		}

		issues := buildIssuesFromGitHub(ctx, client, owner, repo, ghIssues, issueType)
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].NetVotes > issues[j].NetVotes
		})
		if limit > 0 && len(issues) > limit {
			issues = issues[:limit]
		}
		result[issueType] = issues
	}
	return result, nil
}

// buildIssuesFromGitHub converts GitHub API issues to Issue structs with reaction scores.
func buildIssuesFromGitHub(ctx context.Context, client *github.Client, owner, repo string, ghIssues []*github.Issue, issueType IssueType) []Issue {
	var issues []Issue
	for _, gi := range ghIssues {
		reactions, _, err := client.Reactions.ListIssueReactions(ctx, owner, repo, gi.GetNumber(), nil)
		if err != nil {
			continue
		}

		var up, down int
		for _, r := range reactions {
			switch r.GetContent() {
			case "heart", "+1":
				up++
			case "-1":
				down++
			}
		}

		body := gi.GetBody()
		if len(body) > 500 {
			body = body[:500] + "..."
		}

		issues = append(issues, Issue{
			Number:   gi.GetNumber(),
			Title:    gi.GetTitle(),
			Body:     body,
			NetVotes: up - down,
			URL:      gi.GetHTMLURL(),
			Type:     issueType,
		})
	}
	return issues
}

// FormatIssuesByType returns formatted strings for all issue types for the agent prompt.
func FormatIssuesByType(issues map[IssueType][]Issue) string {
	var result string

	if agentInput, ok := issues[IssueTypeInput]; ok && len(agentInput) > 0 {
		result += "## Community Suggestions (agent-input)\n"
		for _, issue := range agentInput {
			result += fmt.Sprintf("- [#%d +%d] %s\n  %s\n  %s\n",
				issue.Number, issue.NetVotes, issue.Title, issue.Body, issue.URL)
		}
		result += "\n"
	}

	if agentSelf, ok := issues[IssueTypeSelf]; ok && len(agentSelf) > 0 {
		result += "## Self-Generated TODOs (agent-self)\n"
		for _, issue := range agentSelf {
			result += fmt.Sprintf("- [#%d] %s\n  %s\n  %s\n",
				issue.Number, issue.Title, issue.Body, issue.URL)
		}
		result += "\n"
	}

	if agentHelp, ok := issues[IssueTypeHelpWanted]; ok && len(agentHelp) > 0 {
		result += "## Help Wanted (agent-help-wanted)\n"
		for _, issue := range agentHelp {
			result += fmt.Sprintf("- [#%d +%d] %s\n  %s\n  %s\n",
				issue.Number, issue.NetVotes, issue.Title, issue.Body, issue.URL)
		}
		result += "\n"
	}

	return result
}

// PostReply posts a comment on a GitHub issue as the bot.
func PostReply(ctx context.Context, owner, repo string, issueNumber int, body string) error {
	client := NewGitHubClient(ctx)
	if client == nil {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	_, _, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, &github.IssueComment{
		Body: &body,
	})
	return err
}
