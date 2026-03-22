package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/util"
)

// fetchDiscussions uses GitHub GraphQL API to get open discussions.
func (e *Engine) fetchDiscussions(ctx context.Context) ([]Discussion, error) {
	query := `{
		"query": "query($owner:String!,$repo:String!){repository(owner:$owner,name:$repo){discussions(first:20,states:[OPEN]){nodes{id,number,title,body,url,comments(first:10){nodes{id,author{login},body}}}}}}}",
		"variables": {"owner": "` + e.owner + `", "repo": "` + e.repo + `"}
	}`

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result discussionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return parseDiscussionNodes(result.Data.Repository.Discussions.Nodes), nil
}

type discussionsResponse struct {
	Data struct {
		Repository struct {
			Discussions struct {
				Nodes []discussionNode `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	} `json:"data"`
}

type discussionNode struct {
	ID       string `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	URL      string `json:"url"`
	Comments struct {
		Nodes []struct {
			ID     string `json:"id"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

func parseDiscussionNodes(nodes []discussionNode) []Discussion {
	var discussions []Discussion
	for _, node := range nodes {
		d := Discussion{
			ID:     node.ID,
			Number: node.Number,
			Title:  node.Title,
			Body:   util.Truncate(node.Body, 500),
			URL:    node.URL,
		}
		for _, c := range node.Comments.Nodes {
			d.Comments = append(d.Comments, Comment{
				ID:     c.ID,
				Author: c.Author.Login,
				Body:   util.Truncate(c.Body, 300),
			})
		}
		discussions = append(discussions, d)
	}
	return discussions
}

func (e *Engine) postDiscussionReply(ctx context.Context, discussionID, body string) error {
	mutation := fmt.Sprintf(`{"query":"mutation{addDiscussionComment(input:{discussionId:\"%s\",body:\"%s\"}){comment{id}}}"}`,
		discussionID, strings.ReplaceAll(body, `"`, `\"`))

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(mutation))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GraphQL error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (e *Engine) createDiscussion(ctx context.Context, title, body string) error {
	repoID, categoryID, err := e.fetchRepoAndCategoryID(ctx)
	if err != nil {
		return err
	}

	mutation := fmt.Sprintf(`{"query":"mutation{createDiscussion(input:{repositoryId:\"%s\",categoryId:\"%s\",title:\"%s\",body:\"%s\"}){discussion{id}}}"}`,
		repoID, categoryID,
		strings.ReplaceAll(title, `"`, `\"`),
		strings.ReplaceAll(body, `"`, `\"`))

	return e.doGraphQLPost(ctx, mutation)
}

// fetchRepoAndCategoryID returns the repository node ID and a suitable discussion category ID.
func (e *Engine) fetchRepoAndCategoryID(ctx context.Context) (string, string, error) {
	repoQuery := fmt.Sprintf(`{"query":"{repository(owner:\"%s\",name:\"%s\"){id,discussionCategories(first:5){nodes{id,name}}}}"}`,
		e.owner, e.repo)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(repoQuery))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var repoResult struct {
		Data struct {
			Repository struct {
				ID                   string `json:"id"`
				DiscussionCategories struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"discussionCategories"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoResult); err != nil {
		return "", "", err
	}

	repoID := repoResult.Data.Repository.ID
	categoryID := ""
	for _, cat := range repoResult.Data.Repository.DiscussionCategories.Nodes {
		if strings.EqualFold(cat.Name, "general") || strings.EqualFold(cat.Name, "announcements") {
			categoryID = cat.ID
			break
		}
	}
	if categoryID == "" && len(repoResult.Data.Repository.DiscussionCategories.Nodes) > 0 {
		categoryID = repoResult.Data.Repository.DiscussionCategories.Nodes[0].ID
	}
	if categoryID == "" {
		return "", "", fmt.Errorf("no discussion category found")
	}
	return repoID, categoryID, nil
}

// doGraphQLPost sends a GraphQL mutation/query body and returns an error on failure.
func (e *Engine) doGraphQLPost(ctx context.Context, gqlBody string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(gqlBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
