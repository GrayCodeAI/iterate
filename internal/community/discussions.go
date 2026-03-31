package community

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
)

type Discussion struct {
	Number     int
	Title      string
	Body       string
	Category   string
	Author     string
	URL        string
	Comments   int
	IsAnswered bool
}

type discussionGraphQLResponse struct {
	Data struct {
		Repository struct {
			Discussions struct {
				Nodes []struct {
					Number      int    `json:"number"`
					Title       string `json:"title"`
					Body        string `json:"body"`
					AuthorLogin string `json:"authorLogin"`
					URL         string `json:"url"`
					Comments    struct {
						TotalCount int `json:"totalCount"`
					} `json:"comments"`
					IsAnswered   bool   `json:"isAnswered"`
					CategoryName string `json:"categoryName"`
				} `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	} `json:"data"`
}

func FetchDiscussions(ctx context.Context, owner, repo string, limit int) ([]Discussion, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		slog.Warn("GITHUB_TOKEN not set, skipping discussions")
		return nil, nil
	}

	query := `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			discussions(first: 50, orderBy: {field: UPDATED_AT, direction: DESC}) {
				nodes {
					number
					title
					body
					authorLogin
					url
					comments { totalCount }
					isAnswered
					categoryName
				}
			}
		}
	}
	`

	body, err := runGraphQLRequest(ctx, token, query, map[string]string{
		"owner": owner,
		"repo":  repo,
	})
	if err != nil {
		return nil, err
	}

	return parseDiscussionNodes(body, limit)
}

func parseDiscussionNodes(body []byte, limit int) ([]Discussion, error) {
	var graphqlResp discussionGraphQLResponse
	if err := json.Unmarshal(body, &graphqlResp); err != nil {
		return nil, fmt.Errorf("parse graphql response: %w", err)
	}

	nodes := graphqlResp.Data.Repository.Discussions.Nodes
	if limit > 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}

	discussions := make([]Discussion, 0, len(nodes))
	for _, n := range nodes {
		body := n.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		discussions = append(discussions, Discussion{
			Number:     n.Number,
			Title:      n.Title,
			Body:       body,
			Category:   n.CategoryName,
			Author:     n.AuthorLogin,
			URL:        n.URL,
			Comments:   n.Comments.TotalCount,
			IsAnswered: n.IsAnswered,
		})
	}

	sort.Slice(discussions, func(i, j int) bool {
		return discussions[i].Comments > discussions[j].Comments
	})

	return discussions, nil
}

func FormatDiscussions(discussions []Discussion) string {
	if len(discussions) == 0 {
		return ""
	}

	var result string
	result += "## GitHub Discussions\n\n"

	for _, d := range discussions {
		result += fmt.Sprintf("- [%s] #%d: %s\n  %s\n  %s\n  %d comments\n",
			d.Category, d.Number, d.Title, d.Body, d.URL, d.Comments)
	}

	return result
}

func PostDiscussionReply(ctx context.Context, owner, repo string, discussionNumber int, body string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	repoID := fmt.Sprintf("R_repo:%s/%s", owner, repo)
	discussionID := fmt.Sprintf("D:%s:%d", repoID, discussionNumber)

	mutation := `
	mutation($repoId: ID!, $discussionId: ID!, $body: String!) {
		addDiscussionComment(input: {discussionId: $discussionId, body: $body}) {
			comment {
				id
			}
		}
	}
	`

	_, err := runGraphQLRequest(ctx, token, mutation, map[string]interface{}{
		"repoId":       repoID,
		"discussionId": discussionID,
		"body":         body,
	})
	return err
}

func CreateDiscussion(ctx context.Context, owner, repo, category, title, body string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	mutation := `
	mutation($repoId: ID!, $categoryId: ID!, $title: String!, $body: String!) {
		createDiscussion(input: {repositoryId: $repoId, categoryId: $categoryId, title: $title, body: $body}) {
			discussion {
				number
				url
			}
		}
	}
	`

	repoID := fmt.Sprintf("R_repo:%s/%s", owner, repo)
	categoryID := category
	if categoryID == "" {
		categoryID = "DIC_kwDOJ4DmMc4B_2_p" // default: General
	}

	_, err := runGraphQLRequest(ctx, token, mutation, map[string]interface{}{
		"repoId":     repoID,
		"categoryId": categoryID,
		"title":      title,
		"body":       body,
	})
	return err
}

func runGraphQLRequest(ctx context.Context, token, query string, variables interface{}) ([]byte, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphql request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("graphql request failed (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}
