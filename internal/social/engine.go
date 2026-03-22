package social

import (
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

	"github.com/GrayCodeAI/iterate/internal/community"
	"github.com/GrayCodeAI/iterate/internal/util"
	"github.com/google/go-github/v61/github"

	"github.com/GrayCodeAI/iteragent"
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
	repoPath   string
	owner      string
	repo       string
	token      string
	client     *github.Client
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new social engine.
func New(repoPath, owner, repo string, logger *slog.Logger) *Engine {
	ctx := context.Background()
	return &Engine{
		repoPath:   repoPath,
		owner:      owner,
		repo:       repo,
		token:      os.Getenv("GITHUB_TOKEN"),
		client:     community.NewGitHubClient(ctx),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// HealthCheck verifies that the GitHub API is reachable and the token is valid.
func (e *Engine) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://api.github.com", nil)
	if err != nil {
		e.logger.Error("health check: failed to create request", "error", err)
		return fmt.Errorf("health check request creation: %w", err)
	}
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.logger.Error("health check: GitHub API unreachable", "error", err)
		return fmt.Errorf("health check: GitHub API unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		e.logger.Error("health check: GitHub token invalid", "status", resp.StatusCode)
		return fmt.Errorf("health check: GitHub token invalid (status %d)", resp.StatusCode)
	}
	e.logger.Info("health check passed", "status", resp.StatusCode)
	return nil
}

// Run executes one social session:
// reads discussions, replies where useful, learns from humans.
func (e *Engine) Run(ctx context.Context, p iteragent.Provider) error {
	if e.token == "" {
		e.logger.Warn("GITHUB_TOKEN not set, skipping social loop")
		return nil
	}

	discussions, err := e.fetchDiscussions(ctx)
	if err != nil {
		return fmt.Errorf("fetch discussions: %w", err)
	}
	e.logger.Info("fetched discussions", "count", len(discussions))

	if len(discussions) == 0 {
		e.logger.Info("no discussions to process")
		return nil
	}

	systemPrompt, dayCount := e.buildSocialSystemPrompt()
	userMessage := buildSocialPrompt(discussions)

	response, err := p.Complete(ctx, []iteragent.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}

	decisions, err := parseSocialDecisions(response)
	if err != nil {
		e.logger.Warn("failed to parse social decisions", "err", err, "response", response[:min(200, len(response))])
		return nil
	}

	e.executeSocialDecisions(ctx, decisions)
	e.processSocialLearnings(decisions, dayCount)
	e.createDecisionsAsDiscussions(ctx, decisions)

	return nil
}

func (e *Engine) buildSocialSystemPrompt() (string, string) {
	personality, _ := os.ReadFile(filepath.Join(e.repoPath, "PERSONALITY.md"))
	socialSkill, _ := os.ReadFile(filepath.Join(e.repoPath, "skills/social/SKILL.md"))
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	day := strings.TrimSpace(string(dayCount))

	systemPrompt := fmt.Sprintf(`You are iterate, a self-evolving coding agent.
%s

## Social skill
%s

Today is day %s.
You will be given a list of GitHub Discussions. For each one, decide whether to reply.
Output your decisions as JSON only — no prose.`, string(personality), string(socialSkill), day)

	return systemPrompt, day
}

func (e *Engine) executeSocialDecisions(ctx context.Context, decisions []socialDecision) {
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
}

func (e *Engine) processSocialLearnings(decisions []socialDecision, dayCount string) {
	learnings := extractLearnings(decisions)
	if learnings == "" {
		return
	}
	if err := e.appendLearnings(learnings); err != nil {
		e.logger.Warn("failed to append learnings", "err", err)
	}
	if err := e.appendLearningsJSONL(decisions, dayCount); err != nil {
		e.logger.Warn("failed to append learnings jsonl", "err", err)
	}
}

func (e *Engine) createDecisionsAsDiscussions(ctx context.Context, decisions []socialDecision) {
	for _, d := range decisions {
		if d.NewDiscussion != nil {
			if err := e.createDiscussion(ctx, d.NewDiscussion.Title, d.NewDiscussion.Body); err != nil {
				e.logger.Warn("failed to create discussion", "err", err)
			} else {
				e.logger.Info("created discussion", "title", d.NewDiscussion.Title)
			}
		}
	}
}

// ReplyToIssues posts a comment on each addressed issue.
func (e *Engine) buildIssueReplyPrompt(personality, communicateSkill, dayCount, journalSnippet string) string {
	return fmt.Sprintf(`You are iterate, a self-evolving coding agent.
%s

## Communicate skill
%s

Day: %s
Recent journal: %s

Write a reply to this GitHub issue. Output ONLY the reply text, nothing else.`,
		string(personality), string(communicateSkill),
		strings.TrimSpace(string(dayCount)), journalSnippet)
}

func (e *Engine) replyToSingleIssue(ctx context.Context, p iteragent.Provider, num int, personality, communicateSkill, dayCount, journalSnippet string) {
	issue, err := e.fetchIssue(ctx, num)
	if err != nil {
		e.logger.Warn("failed to fetch issue", "number", num, "err", err)
		return
	}

	systemPrompt := e.buildIssueReplyPrompt(personality, communicateSkill, dayCount, journalSnippet)
	userMessage := fmt.Sprintf("Issue #%d: %s\n\n%s", issue.Number, issue.Title, issue.Body)

	messages := []iteragent.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	reply, err := p.Complete(ctx, messages)
	if err != nil {
		e.logger.Warn("LLM error for issue reply", "issue", num, "err", err)
		return
	}

	if err := e.postIssueComment(ctx, num, strings.TrimSpace(reply)); err != nil {
		e.logger.Warn("failed to post issue comment", "issue", num, "err", err)
	} else {
		e.logger.Info("replied to issue", "number", num)
	}
}

func (e *Engine) ReplyToIssues(ctx context.Context, p iteragent.Provider, issueNumbers []int) error {
	if e.token == "" || len(issueNumbers) == 0 {
		return nil
	}

	personality, _ := os.ReadFile(filepath.Join(e.repoPath, "PERSONALITY.md"))
	communicateSkill, _ := os.ReadFile(filepath.Join(e.repoPath, "skills/communicate/SKILL.md"))
	journal, _ := os.ReadFile(filepath.Join(e.repoPath, "JOURNAL.md"))
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))

	journalSnippet := string(journal)
	if len(journalSnippet) > 800 {
		journalSnippet = journalSnippet[len(journalSnippet)-800:]
	}

	for _, num := range issueNumbers {
		e.replyToSingleIssue(ctx, p, num, string(personality), string(communicateSkill), string(dayCount), journalSnippet)
	}
	return nil
}

func (e *Engine) appendLearnings(text string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	path := filepath.Join(memDir, "active_social_learnings.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf("\n## %s\n\n%s\n\n---\n", time.Now().Format("2006-01-02"), text)
	_, err = f.WriteString(entry)
	return err
}

// WriteLearningsToMemory is the public entry point for the synthesize workflow.
// It writes a social learning entry with the given who and insight fields.
func (e *Engine) WriteLearningsToMemory(who, insight string) error {
	dayCount, _ := os.ReadFile(filepath.Join(e.repoPath, "DAY_COUNT"))
	decisions := []socialDecision{{Learning: insight}}
	if who != "" {
		// Embed who into the learning text so it appears in the output
		decisions[0].Learning = fmt.Sprintf("[%s] %s", who, insight)
	}
	return e.appendLearningsJSONL(decisions, strings.TrimSpace(string(dayCount)))
}

// appendLearningsJSONL appends a social learning as a JSON line to memory/social_learnings.jsonl.
func (e *Engine) appendLearningsJSONL(decisions []socialDecision, dayCount string) error {
	memDir := filepath.Join(e.repoPath, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	path := filepath.Join(memDir, "social_learnings.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	day := 0
	fmt.Sscanf(strings.TrimSpace(dayCount), "%d", &day)
	ts := time.Now().UTC().Format(time.RFC3339)

	for _, d := range decisions {
		if d.Learning == "" {
			continue
		}
		entry := map[string]interface{}{
			"type":    "social",
			"day":     day,
			"ts":      ts,
			"source":  "social session",
			"who":     "",
			"insight": d.Learning,
		}
		line, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		f.Write(line)
		f.Write([]byte("\n"))
	}
	return nil
}

// --- GitHub REST API calls ---

type githubIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func (e *Engine) fetchIssue(ctx context.Context, number int) (*githubIssue, error) {
	if e.client == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}
	issue, _, err := e.client.Issues.Get(ctx, e.owner, e.repo, number)
	if err != nil {
		return nil, err
	}
	body := issue.GetBody()
	return &githubIssue{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   body,
	}, nil
}

func (e *Engine) postIssueComment(ctx context.Context, number int, body string) error {
	if e.client == nil {
		return fmt.Errorf("GitHub client not initialized")
	}
	_, _, err := e.client.Issues.CreateComment(ctx, e.owner, e.repo, number, &github.IssueComment{
		Body: &body,
	})
	return err
}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
