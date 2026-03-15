package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool represents a capability the agent can invoke.
type Tool struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, args map[string]string) (string, error)
}

// DefaultTools returns all built-in tools available to the agent.
func DefaultTools(repoPath string) []Tool {
	return []Tool{
		bashTool(repoPath),
		readFileTool(repoPath),
		writeFileTool(repoPath),
		listFilesTool(repoPath),
		gitDiffTool(repoPath),
		gitCommitTool(repoPath),
		gitRevertTool(repoPath),
		runTestsTool(repoPath),
		MutationTestTool(repoPath),
	}
}

// ToolCall represents a parsed tool invocation from the LLM response.
type ToolCall struct {
	Tool string            `json:"tool"`
	Args map[string]string `json:"args"`
}

// ParseToolCalls extracts tool calls from LLM output.
// Format: ```tool\n{"tool":"name","args":{...}}\n```
func ParseToolCalls(output string) []ToolCall {
	var calls []ToolCall
	lines := strings.Split(output, "\n")
	inBlock := false
	var block strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "```tool") {
			inBlock = true
			block.Reset()
			continue
		}
		if inBlock && line == "```" {
			var call ToolCall
			if err := json.Unmarshal([]byte(block.String()), &call); err == nil {
				calls = append(calls, call)
			}
			inBlock = false
			continue
		}
		if inBlock {
			block.WriteString(line + "\n")
		}
	}
	return calls
}

// ToolMap converts a slice of tools to a name-indexed map.
func ToolMap(tools []Tool) map[string]Tool {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name] = t
	}
	return m
}

// ToolDescriptions returns a formatted string of all tool descriptions for the system prompt.
func ToolDescriptions(tools []Tool) string {
	var sb strings.Builder
	sb.WriteString("## Available tools\n\n")
	sb.WriteString("Call tools using:\n```tool\n{\"tool\":\"name\",\"args\":{\"key\":\"value\"}}\n```\n\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", t.Name, t.Description))
	}
	return sb.String()
}

// --- Tool implementations ---

func bashTool(repoPath string) Tool {
	return Tool{
		Name:        "bash",
		Description: "Run a shell command in the repo directory.\nArgs: {\"cmd\": \"go build ./...\"}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			cmd := args["cmd"]
			if cmd == "" {
				return "", fmt.Errorf("cmd is required")
			}
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			c := exec.CommandContext(ctx, "bash", "-c", cmd)
			c.Dir = repoPath
			var out bytes.Buffer
			c.Stdout = &out
			c.Stderr = &out
			_ = c.Run() // capture output even on error
			return out.String(), nil
		},
	}
}

func readFileTool(repoPath string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read a file from the repo.\nArgs: {\"path\": \"internal/agent/agent.go\"}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			path := filepath.Join(repoPath, args["path"])
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", args["path"], err)
			}
			return string(data), nil
		},
	}
}

func writeFileTool(repoPath string) Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write or overwrite a file in the repo.\nArgs: {\"path\": \"internal/agent/agent.go\", \"content\": \"...\"}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			path := filepath.Join(repoPath, args["path"])
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(args["content"]), 0o644); err != nil {
				return "", fmt.Errorf("write %s: %w", args["path"], err)
			}
			return fmt.Sprintf("wrote %s (%d bytes)", args["path"], len(args["content"])), nil
		},
	}
}

func listFilesTool(repoPath string) Tool {
	return Tool{
		Name:        "list_files",
		Description: "List all Go source files in the repo.\nArgs: {} (no args needed)",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			var files []string
			err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && (info.Name() == ".git" || info.Name() == "vendor") {
					return filepath.SkipDir
				}
				rel, _ := filepath.Rel(repoPath, path)
				if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".md") {
					files = append(files, rel)
				}
				return nil
			})
			return strings.Join(files, "\n"), err
		},
	}
}

func gitDiffTool(repoPath string) Tool {
	return Tool{
		Name:        "git_diff",
		Description: "Show current unstaged changes.\nArgs: {}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			c := exec.CommandContext(ctx, "git", "diff")
			c.Dir = repoPath
			out, err := c.Output()
			return string(out), err
		},
	}
}

func gitCommitTool(repoPath string) Tool {
	return Tool{
		Name:        "git_commit",
		Description: "Stage all changes and commit.\nArgs: {\"message\": \"feat: improve error handling\"}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			msg := args["message"]
			if msg == "" {
				msg = fmt.Sprintf("iterate: auto-improvement session %s", time.Now().Format("2006-01-02"))
			}

			add := exec.CommandContext(ctx, "git", "add", "-A")
			add.Dir = repoPath
			if out, err := add.CombinedOutput(); err != nil {
				return string(out), fmt.Errorf("git add: %w", err)
			}

			commit := exec.CommandContext(ctx, "git", "commit", "-m", msg)
			commit.Dir = repoPath
			commit.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=iterate[bot]",
				"GIT_AUTHOR_EMAIL=iterate@users.noreply.github.com",
				"GIT_COMMITTER_NAME=iterate[bot]",
				"GIT_COMMITTER_EMAIL=iterate@users.noreply.github.com",
			)
			out, err := commit.CombinedOutput()
			return string(out), err
		},
	}
}

func gitRevertTool(repoPath string) Tool {
	return Tool{
		Name:        "git_revert",
		Description: "Discard all unstaged changes (revert to last commit).\nArgs: {}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			c := exec.CommandContext(ctx, "git", "checkout", "--", ".")
			c.Dir = repoPath
			out, err := c.CombinedOutput()
			return string(out), err
		},
	}
}

func runTestsTool(repoPath string) Tool {
	return Tool{
		Name:        "run_tests",
		Description: "Run go build and go test across the whole repo.\nArgs: {}",
		Execute: func(ctx context.Context, args map[string]string) (string, error) {
			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			var results strings.Builder

			build := exec.CommandContext(ctx, "go", "build", "./...")
			build.Dir = repoPath
			out, err := build.CombinedOutput()
			results.WriteString("=== go build ===\n")
			results.Write(out)
			if err != nil {
				results.WriteString("\nBUILD FAILED\n")
				return results.String(), fmt.Errorf("build failed")
			}
			results.WriteString("BUILD OK\n\n")

			test := exec.CommandContext(ctx, "go", "test", "./...")
			test.Dir = repoPath
			out, err = test.CombinedOutput()
			results.WriteString("=== go test ===\n")
			results.Write(out)
			if err != nil {
				results.WriteString("\nTESTS FAILED\n")
				return results.String(), fmt.Errorf("tests failed")
			}
			results.WriteString("ALL TESTS PASSED\n")

			return results.String(), nil
		},
	}
}
