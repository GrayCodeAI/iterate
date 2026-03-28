// Package context provides git-related context capabilities.
// Task 47: @git reference for commit/diff context

package context

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GitContextType defines the type of git context to retrieve.
type GitContextType string

const (
	GitContextDiff     GitContextType = "diff"     // Unstaged changes
	GitContextStaged   GitContextType = "staged"   // Staged changes
	GitContextCommit   GitContextType = "commit"   // Specific commit
	GitContextBranch   GitContextType = "branch"   // Branch info
	GitContextLog      GitContextType = "log"      // Recent commits
	GitContextBlame    GitContextType = "blame"    // Blame for file
	GitContextStatus   GitContextType = "status"   // Working tree status
	GitContextConflict GitContextType = "conflict" // Merge conflicts
)

// GitCommitInfo represents information about a git commit.
type GitCommitInfo struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Author      string    `json:"author"`
	AuthorEmail string    `json:"author_email"`
	Date        time.Time `json:"date"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body,omitempty"`
	Parents     []string  `json:"parents,omitempty"`
}

// GitDiffInfo represents a diff between commits or working tree.
type GitDiffInfo struct {
	FromRef     string         `json:"from_ref,omitempty"`
	ToRef       string         `json:"to_ref,omitempty"`
	Files       []*GitDiffFile `json:"files"`
	TotalAdd    int            `json:"total_add"`
	TotalDel    int            `json:"total_del"`
	DiffContent string         `json:"diff_content,omitempty"`
	TokenEst    int            `json:"token_est"`
	Authors     map[string]int `json:"authors,omitempty"`
	Commits     map[string]int `json:"commits,omitempty"`
}

// GitDiffFile represents a single file in a diff.
type GitDiffFile struct {
	Path        string `json:"path"`
	OldPath     string `json:"old_path,omitempty"` // For renames
	Status      string `json:"status"`             // A(dded), M(odified), D(eleted), R(enamed)
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	DiffContent string `json:"diff_content,omitempty"`
}

// GitBranchInfo represents information about a git branch.
type GitBranchInfo struct {
	Name       string         `json:"name"`
	IsCurrent  bool           `json:"is_current"`
	IsRemote   bool           `json:"is_remote"`
	Upstream   string         `json:"upstream,omitempty"`
	Ahead      int            `json:"ahead"`
	Behind     int            `json:"behind"`
	LastCommit *GitCommitInfo `json:"last_commit,omitempty"`
}

// GitBlameLine represents a single line from git blame.
type GitBlameLine struct {
	LineNumber int       `json:"line_number"`
	Commit     string    `json:"commit"`
	Author     string    `json:"author"`
	Date       time.Time `json:"date"`
	Content    string    `json:"content"`
}

// GitBlameInfo represents blame information for a file.
type GitBlameInfo struct {
	Path       string          `json:"path"`
	Lines      []*GitBlameLine `json:"lines"`
	Authors    map[string]int  `json:"authors"` // Author -> line count
	Commits    map[string]int  `json:"commits"` // Commit -> line count
	TotalLines int             `json:"total_lines"`
}

// GitStatusInfo represents the working tree status.
type GitStatusInfo struct {
	Staged    []*GitDiffFile `json:"staged"`
	Unstaged  []*GitDiffFile `json:"unstaged"`
	Untracked []string       `json:"untracked"`
	Conflicts []string       `json:"conflicts"`
	IsClean   bool           `json:"is_clean"`
	Branch    string         `json:"branch"`
}

// GitContextConfig holds configuration for git context retrieval.
type GitContextConfig struct {
	MaxDiffSize   int64         `json:"max_diff_size"`   // Max diff size in bytes
	MaxCommits    int           `json:"max_commits"`     // Max commits in log
	MaxBlameLines int           `json:"max_blame_lines"` // Max lines in blame
	IncludeDiff   bool          `json:"include_diff"`    // Include full diff content
	IncludeBody   bool          `json:"include_body"`    // Include commit body
	ContextLines  int           `json:"context_lines"`   // Context lines in diff
	CacheTTL      time.Duration `json:"cache_ttl"`       // Cache TTL
}

// DefaultGitContextConfig returns default configuration.
func DefaultGitContextConfig() *GitContextConfig {
	return &GitContextConfig{
		MaxDiffSize:   100 * 1024, // 100KB
		MaxCommits:    10,
		MaxBlameLines: 500,
		IncludeDiff:   true,
		IncludeBody:   false,
		ContextLines:  3,
		CacheTTL:      2 * time.Minute,
	}
}

// GitContextResult contains the result of a git context query.
type GitContextResult struct {
	Type      GitContextType `json:"type"`
	QueryTime time.Duration  `json:"query_time"`

	// One of these will be populated based on type
	Commit   *GitCommitInfo   `json:"commit,omitempty"`
	Diff     *GitDiffInfo     `json:"diff,omitempty"`
	Branch   *GitBranchInfo   `json:"branch,omitempty"`
	Branches []*GitBranchInfo `json:"branches,omitempty"`
	Log      []*GitCommitInfo `json:"log,omitempty"`
	Blame    *GitBlameInfo    `json:"blame,omitempty"`
	Status   *GitStatusInfo   `json:"status,omitempty"`

	// Markdown representation
	Markdown string `json:"markdown,omitempty"`
	TokenEst int    `json:"token_est"`
}

// GitContextManager provides git-related context capabilities.
type GitContextManager struct {
	config   *GitContextConfig
	repoPath string
	logger   *slog.Logger
	mu       sync.RWMutex

	// Caches
	diffCache   map[string]*GitContextResult
	commitCache map[string]*GitContextResult
	cacheExpiry time.Time
}

// NewGitContextManager creates a new git context manager.
func NewGitContextManager(config *GitContextConfig, repoPath string, logger *slog.Logger) *GitContextManager {
	if config == nil {
		config = DefaultGitContextConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &GitContextManager{
		config:      config,
		repoPath:    repoPath,
		logger:      logger,
		diffCache:   make(map[string]*GitContextResult),
		commitCache: make(map[string]*GitContextResult),
	}
}

// GetDiff returns the unstaged diff context.
func (g *GitContextManager) GetDiff(ctx context.Context, paths ...string) (*GitContextResult, error) {
	cacheKey := fmt.Sprintf("diff:%s", strings.Join(paths, ","))

	g.mu.RLock()
	if cached, ok := g.diffCache[cacheKey]; ok && time.Now().Before(g.cacheExpiry) {
		g.mu.RUnlock()
		return cached, nil
	}
	g.mu.RUnlock()

	start := time.Now()

	args := []string{"diff"}
	if g.config.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-U%d", g.config.ContextLines))
	}
	args = append(args, paths...)

	output, err := g.runGit(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	diff := g.parseDiff(output)
	result := &GitContextResult{
		Type:      GitContextDiff,
		Diff:      diff,
		QueryTime: time.Since(start),
	}

	if g.config.IncludeDiff {
		result.Markdown = g.diffToMarkdown(diff, "Unstaged Changes")
	}

	g.mu.Lock()
	g.diffCache[cacheKey] = result
	g.cacheExpiry = time.Now().Add(g.config.CacheTTL)
	g.mu.Unlock()

	return result, nil
}

// GetStaged returns the staged diff context.
func (g *GitContextManager) GetStaged(ctx context.Context) (*GitContextResult, error) {
	cacheKey := "staged"

	g.mu.RLock()
	if cached, ok := g.diffCache[cacheKey]; ok && time.Now().Before(g.cacheExpiry) {
		g.mu.RUnlock()
		return cached, nil
	}
	g.mu.RUnlock()

	start := time.Now()

	output, err := g.runGit(ctx, "diff", "--cached")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w", err)
	}

	diff := g.parseDiff(output)
	result := &GitContextResult{
		Type:      GitContextStaged,
		Diff:      diff,
		QueryTime: time.Since(start),
	}

	if g.config.IncludeDiff {
		result.Markdown = g.diffToMarkdown(diff, "Staged Changes")
	}

	g.mu.Lock()
	g.diffCache[cacheKey] = result
	g.cacheExpiry = time.Now().Add(g.config.CacheTTL)
	g.mu.Unlock()

	return result, nil
}

// GetCommit returns context for a specific commit.
func (g *GitContextManager) GetCommit(ctx context.Context, ref string) (*GitContextResult, error) {
	cacheKey := fmt.Sprintf("commit:%s", ref)

	g.mu.RLock()
	if cached, ok := g.commitCache[cacheKey]; ok && time.Now().Before(g.cacheExpiry) {
		g.mu.RUnlock()
		return cached, nil
	}
	g.mu.RUnlock()

	start := time.Now()

	// Get commit info
	format := "%H%n%h%n%an%n%ae%n%at%n%s"
	if g.config.IncludeBody {
		format += "%n%b"
	}
	format += "%n%P"

	output, err := g.runGit(ctx, "log", "-1", "--format="+format, ref)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	commit := g.parseCommit(output)

	// Get diff for this commit
	diffOutput, err := g.runGit(ctx, "show", "--format=", ref)
	if err == nil {
		commitDiff := g.parseDiff(diffOutput)
		result := &GitContextResult{
			Type:      GitContextCommit,
			Commit:    commit,
			Diff:      commitDiff,
			QueryTime: time.Since(start),
		}
		result.Markdown = g.commitToMarkdown(commit, commitDiff)

		g.mu.Lock()
		g.commitCache[cacheKey] = result
		g.cacheExpiry = time.Now().Add(g.config.CacheTTL)
		g.mu.Unlock()

		return result, nil
	}

	result := &GitContextResult{
		Type:      GitContextCommit,
		Commit:    commit,
		QueryTime: time.Since(start),
	}
	result.Markdown = g.commitToMarkdown(commit, nil)

	g.mu.Lock()
	g.commitCache[cacheKey] = result
	g.cacheExpiry = time.Now().Add(g.config.CacheTTL)
	g.mu.Unlock()

	return result, nil
}

// GetLog returns recent commit log.
func (g *GitContextManager) GetLog(ctx context.Context, limit int) (*GitContextResult, error) {
	if limit <= 0 {
		limit = g.config.MaxCommits
	}

	start := time.Now()

	format := "%H%n%h%n%an%n%ae%n%at%n%s%n---"
	output, err := g.runGit(ctx, "log", fmt.Sprintf("-%d", limit), "--format="+format)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	commits := g.parseLog(output)
	result := &GitContextResult{
		Type:      GitContextLog,
		Log:       commits,
		QueryTime: time.Since(start),
	}
	result.Markdown = g.logToMarkdown(commits)

	return result, nil
}

// GetBranch returns current branch info.
func (g *GitContextManager) GetBranch(ctx context.Context, branchName string) (*GitContextResult, error) {
	start := time.Now()

	if branchName == "" {
		// Get current branch
		output, err := g.runGit(ctx, "branch", "--show-current")
		if err != nil {
			return nil, fmt.Errorf("git branch failed: %w", err)
		}
		branchName = strings.TrimSpace(output)
	}

	branch := &GitBranchInfo{Name: branchName, IsCurrent: true}

	// Get upstream info
	upstream, _ := g.runGit(ctx, "rev-parse", "--abbrev-ref", branchName+"@{upstream}")
	if upstream != "" {
		branch.Upstream = strings.TrimSpace(upstream)

		// Get ahead/behind
		counts, _ := g.runGit(ctx, "rev-list", "--left-right", "--count", branchName+"..."+branch.Upstream)
		parts := strings.Fields(counts)
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &branch.Ahead)
			fmt.Sscanf(parts[1], "%d", &branch.Behind)
		}
	}

	result := &GitContextResult{
		Type:      GitContextBranch,
		Branch:    branch,
		QueryTime: time.Since(start),
	}
	result.Markdown = g.branchToMarkdown(branch)

	return result, nil
}

// GetBlame returns blame info for a file.
func (g *GitContextManager) GetBlame(ctx context.Context, filePath string) (*GitContextResult, error) {
	start := time.Now()

	// git blame with porcelain format
	output, err := g.runGit(ctx, "blame", "--line-porcelain", filePath)
	if err != nil {
		return nil, fmt.Errorf("git blame failed: %w", err)
	}

	blame := g.parseBlame(output)
	blame.Path = filePath

	result := &GitContextResult{
		Type:      GitContextBlame,
		Blame:     blame,
		QueryTime: time.Since(start),
	}
	result.Markdown = g.blameToMarkdown(blame)

	return result, nil
}

// GetStatus returns the working tree status.
func (g *GitContextManager) GetStatus(ctx context.Context) (*GitContextResult, error) {
	start := time.Now()

	// Get porcelain status
	output, err := g.runGit(ctx, "status", "--porcelain=v1")
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	status := g.parseStatus(output)

	// Get current branch
	branch, _ := g.runGit(ctx, "branch", "--show-current")
	status.Branch = strings.TrimSpace(branch)
	status.IsClean = len(status.Staged) == 0 && len(status.Unstaged) == 0 && len(status.Untracked) == 0 && len(status.Conflicts) == 0

	result := &GitContextResult{
		Type:      GitContextStatus,
		Status:    status,
		QueryTime: time.Since(start),
	}
	result.Markdown = g.statusToMarkdown(status)

	return result, nil
}

// GetConflictFiles returns files with merge conflicts.
func (g *GitContextManager) GetConflictFiles(ctx context.Context) ([]string, error) {
	output, err := g.runGit(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	files := strings.Split(strings.TrimSpace(output), "\n")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// runGit executes a git command.
func (g *GitContextManager) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// parseDiff parses git diff output.
func (g *GitContextManager) parseDiff(output string) *GitDiffInfo {
	diff := &GitDiffInfo{
		Files:   []*GitDiffFile{},
		Authors: make(map[string]int),
		Commits: make(map[string]int),
	}

	lines := strings.Split(output, "\n")
	var currentFile *GitDiffFile
	var diffContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				currentFile.DiffContent = diffContent.String()
				diff.Files = append(diff.Files, currentFile)
			}

			parts := strings.Fields(line)
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[2], "a/")
				currentFile = &GitDiffFile{Path: path}
			}
			diffContent.Reset()
			continue
		}

		if currentFile == nil {
			continue
		}

		// Check for status
		if strings.HasPrefix(line, "new file mode") {
			currentFile.Status = "A"
		} else if strings.HasPrefix(line, "deleted file mode") {
			currentFile.Status = "D"
		} else if strings.HasPrefix(line, "rename from ") {
			currentFile.Status = "R"
			currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
		} else if strings.HasPrefix(line, "index ") && currentFile.Status == "" {
			currentFile.Status = "M"
		}

		// Count additions/deletions
		if len(line) > 0 {
			if line[0] == '+' && !strings.HasPrefix(line, "+++") {
				currentFile.Additions++
				diff.TotalAdd++
			} else if line[0] == '-' && !strings.HasPrefix(line, "---") {
				currentFile.Deletions++
				diff.TotalDel++
			}
		}

		diffContent.WriteString(line + "\n")
	}

	if currentFile != nil {
		currentFile.DiffContent = diffContent.String()
		diff.Files = append(diff.Files, currentFile)
	}

	diff.DiffContent = output
	diff.TokenEst = len(output) / 4 // Rough estimate

	return diff
}

// parseCommit parses git log output for a single commit.
func (g *GitContextManager) parseCommit(output string) *GitCommitInfo {
	lines := strings.Split(output, "\n")
	if len(lines) < 6 {
		return nil
	}

	commit := &GitCommitInfo{
		Hash:        strings.TrimSpace(lines[0]),
		ShortHash:   strings.TrimSpace(lines[1]),
		Author:      strings.TrimSpace(lines[2]),
		AuthorEmail: strings.TrimSpace(lines[3]),
	}

	// Parse timestamp
	var timestamp int64
	fmt.Sscanf(lines[4], "%d", &timestamp)
	commit.Date = time.Unix(timestamp, 0)

	commit.Subject = strings.TrimSpace(lines[5])

	// Body and parents
	for i := 6; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) == 40 && isHex(line) {
				commit.Parents = append(commit.Parents, line)
			} else if g.config.IncludeBody {
				commit.Body += line + "\n"
			}
		}
	}

	return commit
}

// parseLog parses git log output for multiple commits.
func (g *GitContextManager) parseLog(output string) []*GitCommitInfo {
	var commits []*GitCommitInfo
	blocks := strings.Split(output, "---\n")

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		if len(lines) < 6 {
			continue
		}

		commit := &GitCommitInfo{
			Hash:        strings.TrimSpace(lines[0]),
			ShortHash:   strings.TrimSpace(lines[1]),
			Author:      strings.TrimSpace(lines[2]),
			AuthorEmail: strings.TrimSpace(lines[3]),
		}

		var timestamp int64
		fmt.Sscanf(lines[4], "%d", &timestamp)
		commit.Date = time.Unix(timestamp, 0)
		commit.Subject = strings.TrimSpace(lines[5])

		commits = append(commits, commit)
	}

	return commits
}

// parseStatus parses git status porcelain output.
func (g *GitContextManager) parseStatus(output string) *GitStatusInfo {
	status := &GitStatusInfo{
		Staged:    []*GitDiffFile{},
		Unstaged:  []*GitDiffFile{},
		Untracked: []string{},
		Conflicts: []string{},
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if len(line) < 1 {
			continue
		}

		path := strings.TrimSpace(line[3:])

		switch line[0] {
		case 'M', 'A', 'D', 'R':
			status.Staged = append(status.Staged, &GitDiffFile{
				Path:   path,
				Status: string(line[0]),
			})
		case 'U':
			status.Conflicts = append(status.Conflicts, path)
		}

		if len(line) > 1 {
			switch line[1] {
			case 'M', 'D':
				status.Unstaged = append(status.Unstaged, &GitDiffFile{
					Path:   path,
					Status: string(line[1]),
				})
			}
		}

		if line[0:2] == "??" {
			status.Untracked = append(status.Untracked, path)
		}
	}

	return status
}

// parseBlame parses git blame porcelain output.
func (g *GitContextManager) parseBlame(output string) *GitBlameInfo {
	blame := &GitBlameInfo{
		Lines:   []*GitBlameLine{},
		Authors: make(map[string]int),
		Commits: make(map[string]int),
	}

	lines := strings.Split(output, "\n")
	var currentLine *GitBlameLine
	lineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "author ") {
			if currentLine != nil {
				currentLine.Author = strings.TrimPrefix(line, "author ")
			}
		} else if strings.HasPrefix(line, "author-time ") {
			if currentLine != nil {
				var t int64
				fmt.Sscanf(line, "author-time %d", &t)
				currentLine.Date = time.Unix(t, 0)
			}
		} else if strings.HasPrefix(line, "\t") {
			if currentLine != nil {
				currentLine.Content = strings.TrimPrefix(line, "\t")
				blame.Lines = append(blame.Lines, currentLine)
				blame.Authors[currentLine.Author]++
				blame.Commits[currentLine.Commit]++
				lineNum++

				if lineNum >= g.config.MaxBlameLines {
					break
				}
			}
			// Start new line
			parts := strings.Fields(line)
			if len(parts) > 0 {
				currentLine = &GitBlameLine{
					Commit: parts[0],
				}
			}
		}
	}

	blame.TotalLines = len(blame.Lines)
	return blame
}

// Markdown generation methods

func (g *GitContextManager) diffToMarkdown(diff *GitDiffInfo, title string) string {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("## %s\n\n", title))
	md.WriteString(fmt.Sprintf("**Files Changed:** %d  \n", len(diff.Files)))
	md.WriteString(fmt.Sprintf("**Additions:** +%d  **Deletions:** -%d\n\n", diff.TotalAdd, diff.TotalDel))

	for _, file := range diff.Files {
		md.WriteString(fmt.Sprintf("### `%s` [%s]\n", file.Path, file.Status))
		md.WriteString(fmt.Sprintf("+%d/-%d lines\n\n", file.Additions, file.Deletions))

		if g.config.IncludeDiff && file.DiffContent != "" {
			md.WriteString("```diff\n")
			md.WriteString(file.DiffContent)
			md.WriteString("\n```\n\n")
		}
	}

	return md.String()
}

func (g *GitContextManager) commitToMarkdown(commit *GitCommitInfo, diff *GitDiffInfo) string {
	var md strings.Builder

	md.WriteString("## Commit\n\n")
	md.WriteString(fmt.Sprintf("**%s** `%s`\n", commit.ShortHash, commit.Subject))
	md.WriteString(fmt.Sprintf("\nAuthor: %s <%s>\n", commit.Author, commit.AuthorEmail))
	md.WriteString(fmt.Sprintf("Date: %s\n\n", commit.Date.Format(time.RFC3339)))

	if commit.Body != "" {
		md.WriteString(commit.Body + "\n\n")
	}

	if diff != nil && len(diff.Files) > 0 {
		md.WriteString(fmt.Sprintf("### Changes (%d files, +%d/-%d)\n", len(diff.Files), diff.TotalAdd, diff.TotalDel))
		for _, f := range diff.Files {
			md.WriteString(fmt.Sprintf("- `%s` [%s] +%d/-%d\n", f.Path, f.Status, f.Additions, f.Deletions))
		}
	}

	return md.String()
}

func (g *GitContextManager) logToMarkdown(commits []*GitCommitInfo) string {
	var md strings.Builder

	md.WriteString("## Recent Commits\n\n")

	for _, c := range commits {
		md.WriteString(fmt.Sprintf("- **%s** %s — *%s*\n", c.ShortHash, c.Subject, c.Author))
		md.WriteString(fmt.Sprintf("  %s\n\n", c.Date.Format("2006-01-02 15:04")))
	}

	return md.String()
}

func (g *GitContextManager) branchToMarkdown(branch *GitBranchInfo) string {
	var md strings.Builder

	md.WriteString("## Current Branch\n\n")
	md.WriteString(fmt.Sprintf("**%s**", branch.Name))

	if branch.Upstream != "" {
		md.WriteString(fmt.Sprintf(" → %s", branch.Upstream))
		if branch.Ahead > 0 || branch.Behind > 0 {
			md.WriteString(fmt.Sprintf(" (ahead %d, behind %d)", branch.Ahead, branch.Behind))
		}
	}

	return md.String()
}

func (g *GitContextManager) blameToMarkdown(blame *GitBlameInfo) string {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("## Blame: `%s`\n\n", blame.Path))
	md.WriteString(fmt.Sprintf("**Total Lines:** %d\n\n", blame.TotalLines))

	// Author summary
	md.WriteString("### Authors\n")
	for author, count := range blame.Authors {
		pct := float64(count) / float64(blame.TotalLines) * 100
		md.WriteString(fmt.Sprintf("- %s: %d lines (%.1f%%)\n", author, count, pct))
	}

	return md.String()
}

func (g *GitContextManager) statusToMarkdown(status *GitStatusInfo) string {
	var md strings.Builder

	md.WriteString("## Git Status\n\n")
	md.WriteString(fmt.Sprintf("**Branch:** %s\n", status.Branch))
	md.WriteString(fmt.Sprintf("**Clean:** %v\n\n", status.IsClean))

	if len(status.Staged) > 0 {
		md.WriteString("### Staged\n")
		for _, f := range status.Staged {
			md.WriteString(fmt.Sprintf("- [%s] `%s`\n", f.Status, f.Path))
		}
		md.WriteString("\n")
	}

	if len(status.Unstaged) > 0 {
		md.WriteString("### Unstaged\n")
		for _, f := range status.Unstaged {
			md.WriteString(fmt.Sprintf("- [%s] `%s`\n", f.Status, f.Path))
		}
		md.WriteString("\n")
	}

	if len(status.Untracked) > 0 {
		md.WriteString("### Untracked\n")
		for _, f := range status.Untracked {
			md.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		md.WriteString("\n")
	}

	if len(status.Conflicts) > 0 {
		md.WriteString("### ⚠️ Conflicts\n")
		for _, f := range status.Conflicts {
			md.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	return md.String()
}

// ClearCache clears all caches.
func (g *GitContextManager) ClearCache() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.diffCache = make(map[string]*GitContextResult)
	g.commitCache = make(map[string]*GitContextResult)
	g.cacheExpiry = time.Time{}
}

// UpdateConfig updates the configuration.
func (g *GitContextManager) UpdateConfig(config *GitContextConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if config != nil {
		g.config = config
	}
}

// GetStats returns cache statistics.
func (g *GitContextManager) GetStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"diff_cache_size":   len(g.diffCache),
		"commit_cache_size": len(g.commitCache),
		"cache_ttl":         g.config.CacheTTL,
		"cache_expiry":      g.cacheExpiry,
	}
}

// isHex checks if a string is a valid hex string.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
