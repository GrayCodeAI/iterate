package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// Repo index (codebase summary for system prompt)
// ---------------------------------------------------------------------------

// buildRepoIndex returns a compact file-tree string for the repo.
// Limited to 200 files and 4 levels deep to avoid bloating the prompt.
func buildRepoIndex(repoPath string) string {
	var lines []string
	count := 0
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || count >= 200 {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		if rel == "." {
			return nil
		}
		// Skip hidden dirs (except .iterate), vendor, node_modules, build artefacts
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, p := range parts {
			if (strings.HasPrefix(p, ".") && p != ".iterate") ||
				p == "vendor" || p == "node_modules" || p == "dist" || p == "build" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if len(parts) > 4 {
			return nil
		}
		indent := strings.Repeat("  ", len(parts)-1)
		if d.IsDir() {
			lines = append(lines, indent+d.Name()+"/")
		} else {
			lines = append(lines, indent+d.Name())
			count++
		}
		return nil
	})
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Error helpers for /fix and /explain-error
// ---------------------------------------------------------------------------

func buildFixPrompt(errText string) string {
	return fmt.Sprintf(
		"Fix the following error. Read the relevant files first, then apply the minimal fix. "+
			"Run `go build ./...` to verify.\n\nError:\n```\n%s\n```", errText)
}

func buildExplainErrorPrompt(errText string) string {
	return fmt.Sprintf(
		"Explain this error clearly: what it means, why it happens, and how to fix it.\n\n```\n%s\n```",
		errText)
}

// ---------------------------------------------------------------------------
// /diagram — ASCII architecture diagram prompt
// ---------------------------------------------------------------------------

func buildDiagramPrompt(repoPath string) string {
	index := buildRepoIndex(repoPath)
	return fmt.Sprintf(
		"Generate an ASCII architecture diagram for this codebase. "+
			"Show: packages, their relationships, data flow, and key interfaces. "+
			"Use ASCII box-drawing characters. Be clear and concise.\n\n"+
			"Repo structure:\n```\n%s\n```", index)
}

// ---------------------------------------------------------------------------
// /mock — generate interface mocks prompt
// ---------------------------------------------------------------------------

func buildMockPrompt(filePath string) string {
	return fmt.Sprintf(
		"Read %s and generate Go mock implementations for all interfaces found. "+
			"Use a simple struct-based mock with recorded calls and configurable return values. "+
			"Write mocks to a *_mock_test.go file in the same package.", filePath)
}

// ---------------------------------------------------------------------------
// /review — code review prompt builder
// ---------------------------------------------------------------------------

func buildReviewPrompt(repoPath string) string {
	out, _ := exec.Command("git", "-C", repoPath, "diff", "HEAD").Output()
	diff := strings.TrimSpace(string(out))
	if diff == "" {
		out, _ = exec.Command("git", "-C", repoPath, "diff").Output()
		diff = strings.TrimSpace(string(out))
	}
	base := "Review the current code changes. Look for: bugs, security issues, performance problems, " +
		"missing error handling, and style violations. Be concise and actionable.\n\n"
	if diff != "" {
		if len(diff) > 6000 {
			diff = diff[:6000] + "\n…[truncated]"
		}
		return base + "```diff\n" + diff + "\n```"
	}
	return base + "No diff found — review the overall codebase structure and quality."
}

// ---------------------------------------------------------------------------
// /changelog — generate CHANGELOG from git log
// ---------------------------------------------------------------------------

func buildChangelogPrompt(repoPath string, since string) string {
	args := []string{"-C", repoPath, "log", "--oneline"}
	if since != "" {
		args = append(args, since+"..HEAD")
	} else {
		args = append(args, "-50")
	}
	out, _ := exec.Command("git", args...).Output()
	log := strings.TrimSpace(string(out))
	if log == "" {
		return "No commits found."
	}
	return fmt.Sprintf(
		"Generate a CHANGELOG.md entry from these git commits. "+
			"Group into: Added, Changed, Fixed, Removed. Be concise.\n\n```\n%s\n```", log)
}

// ---------------------------------------------------------------------------
// /summarize — ask the model to summarize the conversation
// ---------------------------------------------------------------------------

func buildSummarizePrompt(messages []iteragent.Message) string {
	if len(messages) == 0 {
		return "The conversation is empty."
	}
	return fmt.Sprintf(
		"Summarize this conversation in 3-5 bullet points. Focus on: what was asked, "+
			"what was implemented, and any decisions made. Be brief.\n\n"+
			"(Conversation has %d messages)", len(messages))
}

// ---------------------------------------------------------------------------
// /generate-readme — AI-generated README
// ---------------------------------------------------------------------------

func buildReadmePrompt(repoPath string) string {
	index := buildRepoIndex(repoPath)
	existing, _ := os.ReadFile(filepath.Join(repoPath, "README.md"))
	base := fmt.Sprintf(
		"Generate a comprehensive README.md for this project. Include: "+
			"title, description, features, installation, usage, configuration, "+
			"and contributing sections. Make it compelling and clear.\n\nRepo:\n```\n%s\n```", index)
	if len(existing) > 0 {
		base += fmt.Sprintf("\n\nExisting README (improve this):\n```\n%s\n```", string(existing))
	}
	return base
}

// ---------------------------------------------------------------------------
// /generate-commit — AI-suggested commit message from current diff
// ---------------------------------------------------------------------------

func buildGenerateCommitPrompt(repoPath string) string {
	diff, _ := exec.Command("git", "-C", repoPath, "diff", "--cached").Output()
	if len(strings.TrimSpace(string(diff))) == 0 {
		diff, _ = exec.Command("git", "-C", repoPath, "diff", "HEAD").Output()
	}
	d := strings.TrimSpace(string(diff))
	if d == "" {
		return "No diff found. Stage or make changes first."
	}
	if len(d) > 6000 {
		d = d[:6000] + "\n…[truncated]"
	}
	return fmt.Sprintf(
		"Write a concise git commit message for this diff. "+
			"Format: <type>(<scope>): <summary> — one line, under 72 chars. "+
			"Types: feat/fix/refactor/docs/test/chore. Reply with ONLY the commit message, no explanation.\n\n```diff\n%s\n```", d)
}

// ---------------------------------------------------------------------------
// /release — create a GitHub release
// ---------------------------------------------------------------------------

func buildReleaseNotes(repoPath, from, to string) string {
	args := []string{"-C", repoPath, "log", "--oneline"}
	if from != "" {
		args = append(args, from+".."+to)
	} else {
		args = append(args, "-30")
	}
	out, _ := exec.Command("git", args...).Output()
	log := strings.TrimSpace(string(out))
	if log == "" {
		return "No commits found."
	}
	return fmt.Sprintf(
		"Write release notes for a GitHub release. Group into: ## Features, ## Bug Fixes, ## Other. "+
			"Be user-friendly, not technical. End with installation instructions: `go install github.com/GrayCodeAI/iterate@latest`\n\n```\n%s\n```", log)
}
