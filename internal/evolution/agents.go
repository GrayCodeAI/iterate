package evolution

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

type AgentType string

const (
	AgentPlan   AgentType = "plan"
	AgentBuild  AgentType = "build"
	AgentReview AgentType = "review"
	AgentTest   AgentType = "test"
)

type AgentConfig struct {
	Type         AgentType
	Name         string
	Description  string
	Tools        []string
	SystemPrompt string
	MaxSteps     int
	Timeout      time.Duration
}

var DefaultAgentConfigs = map[AgentType]AgentConfig{
	AgentPlan: {
		Type:        AgentPlan,
		Name:        "planner",
		Description: "Analyzes codebase and creates implementation plans without making changes",
		Tools:       []string{"glob", "grep", "read", "bash"},
		MaxSteps:    20,
		Timeout:     5 * time.Minute,
	},
	AgentBuild: {
		Type:        AgentBuild,
		Name:        "builder",
		Description: "Implements code changes based on plans",
		Tools:       []string{"glob", "grep", "read", "write", "edit", "bash", "todo"},
		MaxSteps:    50,
		Timeout:     15 * time.Minute,
	},
	AgentReview: {
		Type:        AgentReview,
		Name:        "reviewer",
		Description: "Reviews code changes for quality and correctness",
		Tools:       []string{"glob", "grep", "read", "bash"},
		MaxSteps:    30,
		Timeout:     10 * time.Minute,
	},
	AgentTest: {
		Type:        AgentTest,
		Name:        "tester",
		Description: "Writes and verifies tests",
		Tools:       []string{"glob", "grep", "read", "write", "edit", "bash"},
		MaxSteps:    40,
		Timeout:     10 * time.Minute,
	},
}

type AgentResult struct {
	Success     bool
	Output      string
	Changes     []string
	TestResults string
	Error       error
	StepsUsed   int
	Duration    time.Duration
}

type MultiAgentEngine struct {
	engine       *Engine
	agentType    AgentType
	config       AgentConfig
	skills       *iteragent.SkillSet
	tools        []iteragent.Tool
	systemPrompt string
}

func (e *Engine) NewMultiAgent(agentType AgentType) *MultiAgentEngine {
	config, ok := DefaultAgentConfigs[agentType]
	if !ok {
		config = DefaultAgentConfigs[AgentBuild]
	}

	ma := &MultiAgentEngine{
		engine:    e,
		agentType: agentType,
		config:    config,
		tools:     e.tools,
		skills:    e.skills,
	}

	ma.systemPrompt = ma.buildSystemPrompt()
	return ma
}

func (ma *MultiAgentEngine) buildSystemPrompt() string {
	identity, _ := loadIdentity(ma.engine.repoPath)

	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf(`You are iterate %s agent. %s

## CORE RULES

1. You MUST use unified diff format for all file changes:
--- a/path/to/file.go
+++ b/path/to/file.go
@@ ... @@
-old code
+new code

2. After making changes, ALWAYS verify:
   - go build ./...
   - go test ./... -short

3. If tests fail, fix the issues before finishing.

4. Never describe what you'll do - just do it.

## TOOLS AVAILABLE

`, ma.config.Name, ma.config.Description))

	for _, tool := range ma.config.Tools {
		switch tool {
		case "read":
			prompt.WriteString("- read_file: Read file contents\n")
		case "write":
			prompt.WriteString("- write_file: Create or overwrite files\n")
		case "edit":
			prompt.WriteString("- edit_file: Edit existing files\n")
		case "glob":
			prompt.WriteString("- list_files: Find files by pattern\n")
		case "grep":
			prompt.WriteString("- grep: Search in files\n")
		case "bash":
			prompt.WriteString("- bash: Run shell commands\n")
		case "todo":
			prompt.WriteString("- todo: Manage task list\n")
		}
	}

	if len(identity) > 0 {
		prompt.WriteString(fmt.Sprintf("\n## IDENTITY\n%s\n", identity))
	}

	return prompt.String()
}

func (ma *MultiAgentEngine) Execute(ctx context.Context, p iteragent.Provider, task string) *AgentResult {
	start := time.Now()
	result := &AgentResult{
		Success:   false,
		StepsUsed: 0,
	}

	userPrompt := ma.buildUserPrompt(task)

	ma.engine.logger.Info("Starting multi-agent execution",
		"agent", ma.config.Name,
		"task", task,
		"tools", len(ma.config.Tools))

	a := ma.engine.newAgent(
		p,
		ma.tools,
		ma.systemPrompt,
		ma.skills,
	)

	var outputBuilder strings.Builder

	for ev := range a.Prompt(ctx, userPrompt) {
		if ma.engine.eventSink != nil {
			select {
			case ma.engine.eventSink <- ev:
			default:
			}
		}

		if ev.Type == string(iteragent.EventMessageUpdate) {
			outputBuilder.WriteString(ev.Content)
			result.StepsUsed++
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			result.Output = ev.Content
		}
		if ev.Type == string(iteragent.EventError) {
			result.Error = fmt.Errorf("%s", ev.Content)
		}
	}
	a.Finish()

	result.Duration = time.Since(start)

	if result.Error != nil {
		ma.engine.logger.Error("Agent execution error",
			"agent", ma.config.Name,
			"error", result.Error)
		return result
	}

	modifiedFiles, err := ma.applyChanges(result.Output)
	if err != nil {
		result.Error = err
		return result
	}

	result.Changes = modifiedFiles

	if len(modifiedFiles) == 0 {
		result.Error = fmt.Errorf("no changes applied")
		return result
	}

	verifyResult := ma.engine.verify(ctx)
	if !verifyResult.BuildPassed || !verifyResult.TestPassed {
		result.Error = fmt.Errorf("verification failed: build=%v test=%v\n%s",
			verifyResult.BuildPassed, verifyResult.TestPassed, verifyResult.Output)
		return result
	}

	result.Success = true
	result.TestResults = verifyResult.Output

	ma.engine.logger.Info("Agent execution completed",
		"agent", ma.config.Name,
		"changes", len(modifiedFiles),
		"duration", result.Duration)

	return result
}

func (ma *MultiAgentEngine) buildUserPrompt(task string) string {
	var prompt strings.Builder

	switch ma.agentType {
	case AgentPlan:
		prompt.WriteString("## TASK: Create Implementation Plan\n\n")
		prompt.WriteString("Analyze the codebase and create a detailed plan.\n")
		prompt.WriteString("DO NOT make any code changes - just analyze and plan.\n\n")

	case AgentReview:
		prompt.WriteString("## TASK: Review Code Changes\n\n")
		prompt.WriteString("Review the changes made and verify quality.\n")
		prompt.WriteString("Check for: bugs, performance, security, tests.\n\n")

	case AgentTest:
		prompt.WriteString("## TASK: Write Tests\n\n")
		prompt.WriteString("Write tests for the code changes.\n")
		prompt.WriteString("Use TDD: Write failing test first, then make it pass.\n\n")

	default:
		prompt.WriteString("## TASK: Implement Changes\n\n")
	}

	prompt.WriteString(fmt.Sprintf("## YOUR TASK\n%s\n\n", task))
	prompt.WriteString("Use unified diff format for all changes.\n")
	prompt.WriteString("After changes, verify with: go build && go test\n")

	return prompt.String()
}

func (ma *MultiAgentEngine) applyChanges(output string) ([]string, error) {
	diffs := ParseUnifiedDiffs(output)

	if len(diffs) == 0 {
		diffs = parseLegacyEdits(output)
	}

	if len(diffs) == 0 {
		return nil, fmt.Errorf("no unified diffs found in output")
	}

	modifiedFiles, err := ma.engine.ApplyUnifiedDiffs(diffs)
	if err != nil {
		return nil, fmt.Errorf("failed to apply diffs: %w", err)
	}

	return modifiedFiles, nil
}

func parseLegacyEdits(output string) []UnifiedDiff {
	var diffs []UnifiedDiff

	filePattern := "FILE: "
	lines := strings.Split(output, "\n")

	var currentFile string
	var content []string
	inBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, filePattern) {
			if currentFile != "" && len(content) > 0 {
				diffs = append(diffs, createDiffFromContent(currentFile, content))
			}
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, filePattern))
			content = nil
			inBlock = false
			continue
		}

		if strings.Contains(line, "<<<<<<< SEARCH") {
			inBlock = true
			continue
		}
		if strings.Contains(line, "=======") {
			continue
		}
		if strings.Contains(line, ">>>>>>>") {
			inBlock = false
			continue
		}

		if inBlock && strings.TrimSpace(line) != "" {
			content = append(content, line)
		}
	}

	if currentFile != "" && len(content) > 0 {
		diffs = append(diffs, createDiffFromContent(currentFile, content))
	}

	return diffs
}

func createDiffFromContent(file string, lines []string) UnifiedDiff {
	diff := UnifiedDiff{
		NewFile: file,
		OldFile: file,
	}

	var contextLines []string
	var addedLines []string
	var removedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") {
			addedLines = append(addedLines, strings.TrimPrefix(trimmed, "+"))
		} else if strings.HasPrefix(trimmed, "-") {
			removedLines = append(removedLines, strings.TrimPrefix(trimmed, "-"))
		} else if trimmed != "" {
			contextLines = append(contextLines, trimmed)
		}
	}

	if len(removedLines) > 0 || len(addedLines) > 0 {
		diff.Hunks = append(diff.Hunks, DiffHunk{
			Context: contextLines,
			Added:   addedLines,
			Removed: removedLines,
		})
	}

	return diff
}

func loadIdentity(repoPath string) (string, error) {
	identityPath := filepath.Join(repoPath, "docs", "IDENTITY.md")
	identity, err := loadFile(identityPath)
	if err != nil {
		return "", nil
	}
	return string(identity), nil
}

func loadFile(path string) ([]byte, error) {
	return nil, nil
}

type SequentialAgent struct {
	agents []*MultiAgentEngine
}

func NewSequentialAgent(e *Engine, agentTypes ...AgentType) *SequentialAgent {
	agents := make([]*MultiAgentEngine, len(agentTypes))
	for i, at := range agentTypes {
		agents[i] = e.NewMultiAgent(at)
	}
	return &SequentialAgent{agents: agents}
}

func (sa *SequentialAgent) Execute(ctx context.Context, p iteragent.Provider, task string) *AgentResult {
	planResult := sa.agents[0].Execute(ctx, p, task)
	if !planResult.Success {
		return &AgentResult{
			Success: false,
			Error:   fmt.Errorf("planning failed: %w", planResult.Error),
		}
	}

	buildResult := sa.agents[1].Execute(ctx, p, task+" Plan: "+planResult.Output)
	if !buildResult.Success {
		return buildResult
	}

	if len(sa.agents) > 2 {
		reviewResult := sa.agents[2].Execute(ctx, p, "Review changes: "+strings.Join(buildResult.Changes, ", "))
		if !reviewResult.Success {
			return reviewResult
		}
	}

	return buildResult
}

type ParallelAgent struct {
	agents []*MultiAgentEngine
}

func NewParallelAgent(e *Engine, agentTypes ...AgentType) *ParallelAgent {
	agents := make([]*MultiAgentEngine, len(agentTypes))
	for i, at := range agentTypes {
		agents[i] = e.NewMultiAgent(at)
	}
	return &ParallelAgent{agents: agents}
}

func (pa *ParallelAgent) Execute(ctx context.Context, p iteragent.Provider, tasks []string) []*AgentResult {
	results := make([]*AgentResult, len(tasks))

	for i, task := range tasks {
		results[i] = pa.agents[i%len(pa.agents)].Execute(ctx, p, task)
	}

	return results
}
