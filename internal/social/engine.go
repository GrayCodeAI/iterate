package social

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/iterate/internal/provider"
)

// Discussion represents a GitHub Discussion thread.
type Discussion struct {
	ID       string
	Number   int
	Title    string
	Body     string
	URL      string
	Comments []Comment
}

// Comment is a single reply in a discussion.
type Comment struct {
	ID     string
	Author string
	Body   string
}

// Engine handles the social loop.
type Engine struct {
	repoPath string
	owner    string
	repo     string
	token    string
	client   *http.Client
	logger   *slog.Logger
}

// New creates a new social engine.
func New(repoPath, owner, repo string, logger *slog.Logger) *Engine {
	return &Engine{
		repoPath: repoPath,
		owner:    owner,
		repo:     repo,
		token:    os.Getenv("GITHUB_TOKEN"),
		client:   &http.Client{Timeout: 30 * time.Second},
		logger:   logger,
	}
}

// Run executes one social session:
// reads discussions, replies where useful, learns from humans.
func (e *Engine) Run(ctx context.Context, p provider.Provider) error {
	if e.token == "" {
		e.logger.Warn("GITHUB_TOKEN not set, skipping social loop")
		return nil
	}

	// Fetch open discussions
	discussions, err := e.fetchDiscussions(ctx)
	if err != nil {
		return fmt.Errorf("fetch discussions: %w", err)
	}
	e.logger.Info("fetched discussions", "count", len(discussions))

	if len(discussions) == 0 {
		e.logger.Info("no discussions to process")
		return nil
	}

	// Build prompt for the agent
	personality, _ := os.ReadFile(filepath.Join(e.repoPath, "PERSONALITY.md"))
	socialSkill, _ := os.ReadFile(filepath.Join(e.repoPath, "skills/social.md"))
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))

	systemPrompt := fmt.Sprintf(`You are iterate, a self-evolving coding agent.
%s

## Social skill
%s

Today is day %s.
You will be given a list of GitHub Discussions. For each one, decide whether to reply.
Output your decisions as JSON only — no prose.`, string(personality), string(socialSkill), strings.TrimSpace(string(dayCount)))

	userMessage := buildSocialPrompt(discussions)

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	response, err := p.Complete(ctx, messages)
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	// Parse decisions
	decisions, err := parseSocialDecisions(response)
	if err != nil {
		e.logger.Warn("failed to parse social decisions", "err", err, "response", response[:min(200, len(response))])
		return nil
	}

	// Execute decisions
	for _, d := range decisions {
		if d.Reply == "" {
			continue
		}
		if err := e.postDiscussionReply(ctx, d.DiscussionID, d.Reply); err != nil {
			e.logger.Warn("failed to post reply", "discussion", d.DiscussionID, "err", err)
		} else {
			e.logger.Info("posted reply", "discussion", d.DiscussionID)
		}
	}

	// Extract learnings
	learnings := extractLearnings(decisions)
	if learnings != "" {
		if err := e.appendLearnings(learnings); err != nil {
			e.logger.Warn("failed to append learnings", "err", err)
		}
	}

	// Optionally start a new discussion
	for _, d := range decisions {
		if d.NewDiscussion != nil {
			if err := e.createDiscussion(ctx, d.NewDiscussion.Title, d.NewDiscussion.Body); err != nil {
				e.logger.Warn("failed to create discussion", "err", err)
			} else {
				e.logger.Info("created discussion", "title", d.NewDiscussion.Title)
			}
		}
	}

	return nil
}

// ReplyToIssues posts a comment on each addressed issue.
func (e *Engine) ReplyToIssues(ctx context.Context, p provider.Provider, issueNumbers []int) error {
	if e.token == "" || len(issueNumbers) == 0 {
		return nil
	}

	personality, _ := os.ReadFile(filepath.Join(e.repoPath, "PERSONALITY.md"))
	communicateSkill, _ := os.ReadFile(filepath.Join(e.repoPath, "skills/communicate.md"))
	journal, _ := os.ReadFile(filepath.Join(e.repoPath, "JOURNAL.md"))
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))

	// Get last journal entry (last 800 chars)
	journalSnippet := string(journal)
	if len(journalSnippet) > 800 {
		journalSnippet = journalSnippet[len(journalSnippet)-800:]
	}

	for _, num := range issueNumbers {
		issue, err := e.fetchIssue(ctx, num)
		if err != nil {
			e.logger.Warn("failed to fetch issue", "number", num, "err", err)
			continue
		}

		systemPrompt := fmt.Sprintf(`You are iterate, a self-evolving coding agent.
%s

## Communicate skill
%s

Day: %s
Recent journal: %s

Write a reply to this GitHub issue. Output ONLY the reply text, nothing else.`,
			string(personality), string(communicateSkill),
			strings.TrimSpace(string(dayCount)), journalSnippet)

		userMessage := fmt.Sprintf("Issue #%d: %s\n\n%s", issue.Number, issue.Title, issue.Body)

		messages := []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		}

		reply, err := p.Complete(ctx, messages)
		if err != nil {
			e.logger.Warn("LLM error for issue reply", "issue", num, "err", err)
			continue
		}

		if err := e.postIssueComment(ctx, num, strings.TrimSpace(reply)); err != nil {
			e.logger.Warn("failed to post issue comment", "issue", num, "err", err)
		} else {
			e.logger.Info("replied to issue", "number", num)
		}
	}
	return nil
}

// --- GitHub GraphQL (Discussions) ---

type socialDecision struct {
	DiscussionID  string `json:"discussion_id"`
	Reply         string `json:"reply,omitempty"`
	Learning      string `json:"learning,omitempty"`
	NewDiscussion *struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"new_discussion,omitempty"`
}

func buildSocialPrompt(discussions []Discussion) string {
	var sb strings.Builder
	sb.WriteString("Here are the current GitHub Discussions. For each one, decide what to do.\n\n")
	sb.WriteString("Respond ONLY with a JSON array of decisions:\n")
	sb.WriteString(`[{"discussion_id":"ID","reply":"text or empty string","learning":"insight or empty","new_discussion":null}]`)
	sb.WriteString("\n\n## Discussions\n\n")

	for _, d := range discussions {
		sb.WriteString(fmt.Sprintf("### ID: %s | #%d: %s\n", d.ID, d.Number, d.Title))
		sb.WriteString(d.Body + "\n")
		for _, c := range d.Comments {
			sb.WriteString(fmt.Sprintf("  [%s]: %s\n", c.Author, c.Body))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseSocialDecisions(response string) ([]socialDecision, error) {
	// Strip markdown code fences if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var decisions []socialDecision
	if err := json.Unmarshal([]byte(response), &decisions); err != nil {
		return nil, err
	}
	return decisions, nil
}

func extractLearnings(decisions []socialDecision) string {
	var parts []string
	for _, d := range decisions {
		if d.Learning != "" {
			parts = append(parts, d.Learning)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func (e *Engine) appendLearnings(text string) error {
	path := filepath.Join(e.repoPath, "SOCIAL_LEARNINGS.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf("\n## %s\n\n%s\n\n---\n", time.Now().Format("2006-01-02"), text)
	_, err = f.WriteString(entry)
	return err
}

// --- GitHub REST API calls ---

type githubIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func (e *Engine) fetchIssue(ctx context.Context, number int) (*githubIssue, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", e.owner, e.repo, number)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issue githubIssue
	return &issue, json.NewDecoder(resp.Body).Decode(&issue)
}

func (e *Engine) postIssueComment(ctx context.Context, number int, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", e.owner, e.repo, number)
	payload, _ := json.Marshal(map[string]string{"body": body})

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// fetchDiscussions uses GitHub GraphQL API to get open discussions.
func (e *Engine) fetchDiscussions(ctx context.Context) ([]Discussion, error) {
	query := `{
		"query": "query($owner:String!,$repo:String!){repository(owner:$owner,name:$repo){discussions(first:20,states:[OPEN]){nodes{id,number,title,body,url,comments(first:10){nodes{id,author{login},body}}}}}}}",
		"variables": {"owner": "` + e.owner + `", "repo": "` + e.repo + `"}
	}`

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(query))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Repository struct {
				Discussions struct {
					Nodes []struct {
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
					} `json:"nodes"`
				} `json:"discussions"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var discussions []Discussion
	for _, node := range result.Data.Repository.Discussions.Nodes {
		d := Discussion{
			ID:     node.ID,
			Number: node.Number,
			Title:  node.Title,
			Body:   truncate(node.Body, 500),
			URL:    node.URL,
		}
		for _, c := range node.Comments.Nodes {
			d.Comments = append(d.Comments, Comment{
				ID:     c.ID,
				Author: c.Author.Login,
				Body:   truncate(c.Body, 300),
			})
		}
		discussions = append(discussions, d)
	}
	return discussions, nil
}

func (e *Engine) postDiscussionReply(ctx context.Context, discussionID, body string) error {
	mutation := fmt.Sprintf(`{"query":"mutation{addDiscussionComment(input:{discussionId:\"%s\",body:\"%s\"}){comment{id}}}"}`,
		discussionID, strings.ReplaceAll(body, `"`, `\"`))

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(mutation))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
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
	// First get repo ID and category ID
	repoQuery := fmt.Sprintf(`{"query":"{repository(owner:\"%s\",name:\"%s\"){id,discussionCategories(first:5){nodes{id,name}}}}"}`,
		e.owner, e.repo)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(repoQuery))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return err
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
		return err
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
		return fmt.Errorf("no discussion category found")
	}

	mutation := fmt.Sprintf(`{"query":"mutation{createDiscussion(input:{repositoryId:\"%s\",categoryId:\"%s\",title:\"%s\",body:\"%s\"}){discussion{id}}}"}`,
		repoID, categoryID,
		strings.ReplaceAll(title, `"`, `\"`),
		strings.ReplaceAll(body, `"`, `\"`))

	req2, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(mutation))
	req2.Header.Set("Authorization", "Bearer "+e.token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := e.client.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
