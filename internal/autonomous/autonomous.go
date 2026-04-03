// Package autonomous provides a "Computer Use" style autonomous agent loop.
// It implements a Plan → Execute → Verify → Retry cycle for autonomous coding tasks.
package autonomous

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// Task 1: Autonomous "Computer Use" Style Loop
// This implements a full autonomous loop that can plan, execute, verify, and retry.

// Config holds configuration for the autonomous engine.
type Config struct {
	MaxIterations     int           // Maximum iterations before stopping (default: 20)
	MaxCost           float64       // Maximum cost in USD before stopping (default: 5.0)
	MaxDuration       time.Duration // Maximum duration before stopping (default: 30min)
	VerificationRetry int           // Number of retries on verification failure (default: 3)
	SafetyMode        SafetyMode    // Safety level for operations
	Interruptible     bool          // Allow Ctrl+C to interrupt (default: true)
	ProgressCallback  func(status Status)
}

// SafetyMode defines how cautious the agent should be.
type SafetyMode int

const (
	SafetyStrict     SafetyMode = iota // Requires approval for all file changes
	SafetyBalanced                     // Auto-approves safe operations, asks for risky ones
	SafetyPermissive                   // Auto-approves most operations
)

// Status represents the current state of an autonomous run.
type Status struct {
	Phase         string        // current phase: "planning", "executing", "verifying", "retrying", "completed", "failed"
	Iteration     int           // current iteration number
	Task          string        // current task description
	FilesModified []string      // files modified so far
	CommandsRun   []string      // commands run so far
	SuccessRate   float64       // success rate of iterations
	StartTime     time.Time     // when the run started
	ElapsedTime   time.Duration // time elapsed since start
	EstimatedCost float64       // estimated cost so far
	LastError     string        // last error encountered
	Confidence    float64       // agent confidence score (0-1)
	PendingAction string        // next action to be taken (for approval)
	NeedsApproval bool          // whether approval is needed for next action
}

// Result holds the final result of an autonomous run.
type Result struct {
	Success       bool
	Status        string
	Iterations    int
	FilesModified []string
	CommandsRun   []string
	TotalCost     float64
	Duration      time.Duration
	FinalMessage  string
	Learnings     []string // lessons learned during execution
	Error         error
}

// Engine represents the autonomous agent engine.
type Engine struct {
	config      Config
	agent       *iteragent.Agent
	tools       []iteragent.Tool
	toolMap     map[string]iteragent.Tool
	repoPath    string
	logger      *slog.Logger
	status      Status
	result      *Result
	stopChan    chan struct{}
	interruptMu sync.Mutex
	interrupted bool
	rollbackMu  sync.Mutex
	rollbackOps []RollbackOp
	eventSink   chan<- iteragent.Event
}

// RollbackOp represents an operation that can be rolled back.
type RollbackOp struct {
	Type       string    // "file_edit", "file_create", "file_delete", "git_commit"
	Path       string    // file path affected
	Original   string    // original content (for edits/creates)
	Timestamp  time.Time // when the operation occurred
	CommitHash string    // for git commits
}

// NewEngine creates a new autonomous engine.
func NewEngine(repoPath string, agent *iteragent.Agent, tools []iteragent.Tool, logger *slog.Logger, config Config) *Engine {
	if config.MaxIterations == 0 {
		config.MaxIterations = 20
	}
	if config.MaxCost == 0 {
		config.MaxCost = 5.0
	}
	if config.MaxDuration == 0 {
		config.MaxDuration = 30 * time.Minute
	}
	if config.VerificationRetry == 0 {
		config.VerificationRetry = 3
	}

	// Use default logger if none provided
	if logger == nil {
		logger = slog.Default()
	}

	return &Engine{
		config:   config,
		agent:    agent,
		tools:    tools,
		toolMap:  iteragent.ToolMap(tools),
		repoPath: repoPath,
		logger:   logger.With("component", "autonomous"),
		status:   Status{Phase: "initialized"},
		result:   &Result{},
		stopChan: make(chan struct{}),
	}
}

// WithEventSink sets a channel to receive live agent events.
func (e *Engine) WithEventSink(sink chan<- iteragent.Event) *Engine {
	e.eventSink = sink
	return e
}

// Stop gracefully stops the autonomous engine.
func (e *Engine) Stop() {
	e.interruptMu.Lock()
	defer e.interruptMu.Unlock()

	if !e.interrupted {
		e.interrupted = true
		close(e.stopChan)
		e.logger.Info("Engine stopped")
	}
}

// Run executes the autonomous loop for the given task.
// This is the main entry point for Task 1.
func (e *Engine) Run(ctx context.Context, task string) *Result {
	startTime := time.Now()
	e.result = &Result{}
	e.status = Status{
		Phase:     "starting",
		Task:      task,
		StartTime: startTime,
	}

	// Set up interrupt handling
	if e.config.Interruptible {
		go e.handleInterrupt()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.config.MaxDuration)
	defer cancel()

	// Main autonomous loop
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		// Check for stop conditions
		select {
		case <-ctx.Done():
			e.result.Status = "timeout"
			e.result.Error = ctx.Err()
			return e.finalize(startTime)
		case <-e.stopChan:
			e.result.Status = "interrupted"
			return e.finalize(startTime)
		default:
		}

		// Check cost limit
		if e.result.TotalCost >= e.config.MaxCost {
			e.result.Status = "cost_limit_reached"
			e.logger.Info("Cost limit reached", "cost", e.result.TotalCost, "limit", e.config.MaxCost)
			return e.finalize(startTime)
		}

		e.status.Iteration = iteration
		e.updateProgress("planning", fmt.Sprintf("Iteration %d: Planning approach", iteration))

		// Phase 1: Plan
		plan, err := e.planPhase(ctx, task, iteration)
		if err != nil {
			e.logger.Error("Planning failed", "iteration", iteration, "err", err)
			continue
		}

		// Phase 2: Execute
		e.updateProgress("executing", fmt.Sprintf("Iteration %d: Executing plan", iteration))
		execResult, err := e.executePhase(ctx, plan)
		if err != nil {
			e.logger.Error("Execution failed", "iteration", iteration, "err", err)
			e.status.LastError = err.Error()
			continue
		}

		// Phase 3: Verify
		e.updateProgress("verifying", fmt.Sprintf("Iteration %d: Verifying results", iteration))
		verifyResult := e.verifyPhase(ctx, execResult)

		if verifyResult.Success {
			e.result.Success = true
			e.result.Status = "completed"
			e.result.FinalMessage = verifyResult.Message
			e.addLearning(fmt.Sprintf("Successfully completed: %s", task))
			return e.finalize(startTime)
		}

		// Phase 4: Retry with context
		if iteration < e.config.MaxIterations && iteration < e.config.VerificationRetry {
			e.updateProgress("retrying", fmt.Sprintf("Iteration %d: Retrying with error context", iteration))
			task = e.buildRetryPrompt(task, verifyResult.Error, iteration)
			e.addLearning(fmt.Sprintf("Retry needed: %s", verifyResult.Error))
		}
	}

	e.result.Status = "max_iterations_reached"
	e.result.Error = fmt.Errorf("max iterations (%d) reached without success", e.config.MaxIterations)
	return e.finalize(startTime)
}

// planPhase generates a plan for the current iteration.
func (e *Engine) planPhase(ctx context.Context, task string, iteration int) (*Plan, error) {
	prompt := e.buildPlanPrompt(task, iteration)

	var planContent string
	for ev := range e.agent.Prompt(ctx, prompt) {
		if e.eventSink != nil {
			e.eventSink <- ev
		}
		if ev.Type == string(iteragent.EventMessageEnd) {
			planContent = ev.Content
		}
		if ev.Type == string(iteragent.EventError) {
			return nil, fmt.Errorf("agent error: %s", ev.Content)
		}
	}

	return parsePlan(planContent)
}

// executePhase executes the plan and returns the results.
func (e *Engine) executePhase(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
	result := &ExecutionResult{
		Actions:      []Action{},
		FilesTouched: []string{},
	}

	for _, step := range plan.Steps {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-e.stopChan:
			return result, fmt.Errorf("interrupted")
		default:
		}

		// Check if approval is needed
		if e.config.SafetyMode == SafetyStrict || e.needsApproval(step) {
			e.status.NeedsApproval = true
			e.status.PendingAction = fmt.Sprintf("%s: %s", step.Type, step.Target)
			if !e.waitForApproval(step) {
				e.logger.Info("Action not approved, skipping", "step", step.Type)
				continue
			}
		}

		// Record rollback point before executing
		rollback := e.createRollbackPoint(step)

		// Execute the step
		action, err := e.executeStep(ctx, step)
		if err != nil {
			e.logger.Warn("Step failed", "step", step.Type, "err", err)
			if e.config.SafetyMode != SafetyPermissive {
				e.rollback(ctx, rollback)
			}
			continue
		}

		result.Actions = append(result.Actions, action)
		if step.Target != "" {
			result.FilesTouched = append(result.FilesTouched, step.Target)
		}
		e.result.FilesModified = append(e.result.FilesModified, step.Target)
	}

	return result, nil
}

// verifyPhase verifies the execution results.
func (e *Engine) verifyPhase(ctx context.Context, execResult *ExecutionResult) *VerificationResult {
	result := &VerificationResult{
		Success: false,
	}

	// Run build
	buildOutput, buildErr := e.runCommand(ctx, "go build ./...")
	if buildErr != nil {
		result.Error = fmt.Sprintf("Build failed: %s", buildOutput)
		result.Message = "Build verification failed"
		return result
	}
	result.BuildPassed = true

	// Run tests
	testOutput, testErr := e.runCommand(ctx, "go test ./...")
	if testErr != nil {
		result.Error = fmt.Sprintf("Tests failed: %s", testOutput)
		result.Message = "Test verification failed"
		return result
	}
	result.TestPassed = true

	// Run vet
	vetOutput, vetErr := e.runCommand(ctx, "go vet ./...")
	result.VetPassed = vetErr == nil
	if vetErr != nil {
		e.logger.Warn("Vet warnings", "output", vetOutput)
	}

	result.Success = true
	result.Message = "All verifications passed"
	return result
}

// Helper types

type Plan struct {
	Goal  string
	Steps []PlanStep
}

type PlanStep struct {
	Type        string // "edit_file", "create_file", "run_command", "git_operation"
	Target      string // file path or command
	Description string
	Content     string // for file operations
}

type ExecutionResult struct {
	Actions      []Action
	FilesTouched []string
}

type Action struct {
	Type     string
	Target   string
	Success  bool
	Output   string
	Duration time.Duration
}

type VerificationResult struct {
	Success     bool
	BuildPassed bool
	TestPassed  bool
	VetPassed   bool
	Message     string
	Error       string
}

// Helper methods

func (e *Engine) updateProgress(phase, message string) {
	e.status.Phase = phase
	if e.config.ProgressCallback != nil {
		e.config.ProgressCallback(e.status)
	}
	e.logger.Info("Autonomous progress", "phase", phase, "message", message)
}

func (e *Engine) buildPlanPrompt(task string, iteration int) string {
	var sb strings.Builder
	sb.WriteString("You are in autonomous mode. Create a detailed plan to accomplish the task.\n\n")
	sb.WriteString(fmt.Sprintf("Task: %s\n\n", task))
	sb.WriteString(fmt.Sprintf("Iteration: %d\n\n", iteration))

	if iteration > 1 {
		sb.WriteString("Previous attempts failed. Learn from the errors and try a different approach.\n\n")
		sb.WriteString(fmt.Sprintf("Last error: %s\n\n", e.status.LastError))
	}

	sb.WriteString("Create a step-by-step plan with specific actions.\n")
	sb.WriteString("Format each step as:\n")
	sb.WriteString("- STEP: <action_type> <target>\n")
	sb.WriteString("- DESC: <description>\n")
	sb.WriteString("- CONTENT: <file content if applicable>\n\n")
	sb.WriteString("Available action types: edit_file, create_file, run_command, git_operation\n")
	return sb.String()
}

func (e *Engine) buildRetryPrompt(task, errorMsg string, iteration int) string {
	return fmt.Sprintf("Previous attempt (iteration %d) failed.\n\nOriginal task: %s\n\nError: %s\n\nAnalyze the error and create a new approach.", iteration, task, errorMsg)
}

func (e *Engine) executeStep(ctx context.Context, step PlanStep) (Action, error) {
	action := Action{
		Type:   step.Type,
		Target: step.Target,
	}

	switch step.Type {
	case "edit_file", "create_file":
		output, err := e.runTool(ctx, step.Type, map[string]interface{}{
			"path":    step.Target,
			"content": step.Content,
		})
		action.Output = output
		action.Success = err == nil
		return action, err

	case "run_command":
		output, err := e.runCommand(ctx, step.Target)
		action.Output = output
		action.Success = err == nil
		e.result.CommandsRun = append(e.result.CommandsRun, step.Target)
		return action, err

	default:
		return action, fmt.Errorf("unknown step type: %s", step.Type)
	}
}

func (e *Engine) runTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	tool, ok := e.toolMap[name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, args)
}

func (e *Engine) runCommand(ctx context.Context, cmd string) (string, error) {
	return e.runTool(ctx, "bash", map[string]interface{}{"cmd": cmd})
}

func (e *Engine) needsApproval(step PlanStep) bool {
	// High-risk operations always need approval
	riskyPatterns := []string{"rm ", "delete", "drop", "truncate", "force", "-f"}
	for _, pattern := range riskyPatterns {
		if strings.Contains(strings.ToLower(step.Target), pattern) {
			return true
		}
	}
	return false
}

func (e *Engine) waitForApproval(step PlanStep) bool {
	// In non-interactive mode, auto-approve if safety mode allows
	if e.config.SafetyMode == SafetyPermissive {
		return true
	}
	// SafetyStrict/Balanced: prompt user via stdin
	e.status.NeedsApproval = true
	e.status.PendingAction = fmt.Sprintf("%s: %s", step.Type, step.Target)
	fmt.Printf("\n⚠ Approval needed: %s: %s — approve? (y/N): ", step.Type, step.Target)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		e.logger.Info("Approval not provided, defaulting to reject", "step", step.Type)
		return false
	}
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}

func (e *Engine) createRollbackPoint(step PlanStep) RollbackOp {
	rollback := RollbackOp{
		Type:      step.Type,
		Path:      step.Target,
		Timestamp: time.Now(),
	}

	// For file edits, save original content
	if step.Type == "edit_file" && step.Target != "" {
		fullPath := filepath.Join(e.repoPath, step.Target)
		if content, err := os.ReadFile(fullPath); err == nil {
			rollback.Original = string(content)
		}
	}

	e.rollbackMu.Lock()
	e.rollbackOps = append(e.rollbackOps, rollback)
	e.rollbackMu.Unlock()

	return rollback
}

func (e *Engine) rollback(ctx context.Context, op RollbackOp) error {
	e.logger.Info("Rolling back operation", "type", op.Type, "path", op.Path)

	switch op.Type {
	case "edit_file", "create_file":
		if op.Original != "" {
			fullPath := filepath.Join(e.repoPath, op.Path)
			return os.WriteFile(fullPath, []byte(op.Original), 0644)
		}
	case "file_delete":
		// Restore deleted file from original content
		if op.Original != "" {
			fullPath := filepath.Join(e.repoPath, op.Path)
			return os.WriteFile(fullPath, []byte(op.Original), 0644)
		}
	}
	return nil
}

func (e *Engine) handleInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	e.interruptMu.Lock()
	e.interrupted = true
	e.interruptMu.Unlock()

	close(e.stopChan)
	e.logger.Info("Autonomous run interrupted by user")
}

func (e *Engine) addLearning(learning string) {
	e.result.Learnings = append(e.result.Learnings, learning)
}

func (e *Engine) finalize(startTime time.Time) *Result {
	e.result.Duration = time.Since(startTime)
	e.status.ElapsedTime = e.result.Duration
	e.updateProgress(e.result.Status, "Run completed")
	return e.result
}

// parsePlan extracts a Plan from agent output.
func parsePlan(content string) (*Plan, error) {
	plan := &Plan{
		Steps: []PlanStep{},
	}

	lines := strings.Split(content, "\n")
	var currentStep *PlanStep

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "STEP:") {
			if currentStep != nil {
				plan.Steps = append(plan.Steps, *currentStep)
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, "STEP:"))
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) >= 1 {
				currentStep = &PlanStep{Type: strings.TrimSpace(parts[0])}
				if len(parts) >= 2 {
					currentStep.Target = strings.TrimSpace(parts[1])
				}
			}
		} else if strings.HasPrefix(line, "DESC:") && currentStep != nil {
			currentStep.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESC:"))
		} else if strings.HasPrefix(line, "CONTENT:") && currentStep != nil {
			currentStep.Content = strings.TrimSpace(strings.TrimPrefix(line, "CONTENT:"))
		}
	}

	if currentStep != nil {
		plan.Steps = append(plan.Steps, *currentStep)
	}

	if len(plan.Steps) == 0 {
		// Fallback: treat entire content as a single description
		plan.Goal = content
	}

	return plan, nil
}
