// Package autonomous - Task 16: Agent Playbooks for common task patterns
package autonomous

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PlaybookType represents the category of playbook
type PlaybookType string

const (
	PlaybookTypeRefactor   PlaybookType = "refactor"
	PlaybookTypeAddTest    PlaybookType = "add_test"
	PlaybookTypeFixBug     PlaybookType = "fix_bug"
	PlaybookTypeAddFeature PlaybookType = "add_feature"
	PlaybookTypeOptimize   PlaybookType = "optimize"
	PlaybookTypeDocument   PlaybookType = "document"
	PlaybookTypeMigrate    PlaybookType = "migrate"
	PlaybookTypeSecurity   PlaybookType = "security"
	PlaybookTypeCustom     PlaybookType = "custom"
)

// PlaybookStep represents a single step in a playbook
type PlaybookStep struct {
	ID         string            `json:"id"`
	Order      int               `json:"order"`
	Type       string            `json:"type"`     // "read", "write", "test", "build", "command"
	Action     string            `json:"action"`   // Action description template
	Target     string            `json:"target"`   // Target template (file, command, etc.)
	Required   bool              `json:"required"` // Must succeed for playbook success
	Timeout    time.Duration     `json:"timeout"`
	RetryCount int               `json:"retry_count"`
	Variables  map[string]string `json:"variables"`  // Variables to substitute
	Conditions []string          `json:"conditions"` // Conditions to execute this step
	OnFailure  string            `json:"on_failure"` // "abort", "skip", "retry", "continue"
}

// PlaybookTrigger defines when a playbook should be suggested
type PlaybookTrigger struct {
	Keywords  []string `json:"keywords"`   // Keywords in task description
	Patterns  []string `json:"patterns"`   // Regex patterns to match
	FileTypes []string `json:"file_types"` // Relevant file extensions
	Priority  int      `json:"priority"`   // Higher = more relevant
}

// Playbook represents a reusable task template
type Playbook struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Type            PlaybookType      `json:"type"`
	Description     string            `json:"description"`
	Version         string            `json:"version"`
	Author          string            `json:"author"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Steps           []PlaybookStep    `json:"steps"`
	SuccessCriteria []string          `json:"success_criteria"`
	Triggers        PlaybookTrigger   `json:"triggers"`
	Variables       map[string]string `json:"variables"` // Default variable values
	Metadata        map[string]any    `json:"metadata"`
	Tags            []string          `json:"tags"`
	Enabled         bool              `json:"enabled"`
}

// PlaybookRegistry manages all available playbooks
type PlaybookRegistry struct {
	mu        sync.RWMutex
	playbooks map[string]*Playbook
	byType    map[PlaybookType][]string
	logger    interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}
}

// NewPlaybookRegistry creates a new playbook registry
func NewPlaybookRegistry() *PlaybookRegistry {
	r := &PlaybookRegistry{
		playbooks: make(map[string]*Playbook),
		byType:    make(map[PlaybookType][]string),
	}

	// Register default playbooks
	r.registerDefaultPlaybooks()

	return r
}

// registerDefaultPlaybooks registers built-in playbooks
func (r *PlaybookRegistry) registerDefaultPlaybooks() {
	now := time.Now()

	// Fix Bug Playbook
	r.Register(&Playbook{
		ID:          "fix-bug",
		Name:        "Fix Bug",
		Type:        PlaybookTypeFixBug,
		Description: "Systematic bug fixing workflow: reproduce, analyze, fix, verify",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "fb-1", Order: 1, Type: "read", Action: "Read error logs and stack traces", Target: "{{error_source}}", Required: true},
			{ID: "fb-2", Order: 2, Type: "read", Action: "Read relevant source files", Target: "{{affected_files}}", Required: true},
			{ID: "fb-3", Order: 3, Type: "command", Action: "Reproduce the bug", Target: "{{reproduce_command}}", Required: true},
			{ID: "fb-4", Order: 4, Type: "write", Action: "Apply fix to affected files", Target: "{{affected_files}}", Required: true},
			{ID: "fb-5", Order: 5, Type: "test", Action: "Run tests to verify fix", Target: "{{test_target}}", Required: true},
		},
		SuccessCriteria: []string{"Bug is fixed", "Tests pass", "No regressions"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"fix", "bug", "error", "crash", "issue", "problem"},
			Patterns: []string{`fix\s+(bug|issue)`, `resolve\s+(error|crash)`, `debug\s+\w+`},
			Priority: 10,
		},
		Tags:    []string{"bugfix", "debugging", "quality"},
		Enabled: true,
	})

	// Add Test Playbook
	r.Register(&Playbook{
		ID:          "add-test",
		Name:        "Add Test",
		Type:        PlaybookTypeAddTest,
		Description: "Add comprehensive tests for existing code",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "at-1", Order: 1, Type: "read", Action: "Read source code to test", Target: "{{source_file}}", Required: true},
			{ID: "at-2", Order: 2, Type: "read", Action: "Read existing tests for patterns", Target: "{{test_dir}}", Required: false},
			{ID: "at-3", Order: 3, Type: "write", Action: "Write unit tests", Target: "{{test_file}}", Required: true},
			{ID: "at-4", Order: 4, Type: "test", Action: "Run new tests", Target: "{{test_file}}", Required: true},
			{ID: "at-5", Order: 5, Type: "command", Action: "Check test coverage", Target: "go test -cover {{package}}", Required: false},
		},
		SuccessCriteria: []string{"Tests pass", "Coverage improved", "Edge cases covered"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"test", "coverage", "unit test", "integration test"},
			Patterns: []string{`add\s+test`, `write\s+test`, `create\s+test`, `increase\s+coverage`},
			Priority: 9,
		},
		Tags:    []string{"testing", "quality", "coverage"},
		Enabled: true,
	})

	// Refactor Playbook
	r.Register(&Playbook{
		ID:          "refactor",
		Name:        "Refactor Code",
		Type:        PlaybookTypeRefactor,
		Description: "Safe refactoring with behavior preservation",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "rf-1", Order: 1, Type: "read", Action: "Read code to refactor", Target: "{{source_file}}", Required: true},
			{ID: "rf-2", Order: 2, Type: "read", Action: "Read existing tests", Target: "{{test_file}}", Required: true},
			{ID: "rf-3", Order: 3, Type: "test", Action: "Run existing tests for baseline", Target: "{{test_file}}", Required: true},
			{ID: "rf-4", Order: 4, Type: "write", Action: "Apply refactoring", Target: "{{source_file}}", Required: true},
			{ID: "rf-5", Order: 5, Type: "test", Action: "Verify tests still pass", Target: "{{test_file}}", Required: true},
			{ID: "rf-6", Order: 6, Type: "build", Action: "Build project", Target: "{{build_target}}", Required: true},
		},
		SuccessCriteria: []string{"All tests pass", "Build succeeds", "No behavior changes"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"refactor", "clean", "restructure", "reorganize"},
			Patterns: []string{`refactor\s+\w+`, `clean\s+up\s+code`, `simplify\s+\w+`},
			Priority: 8,
		},
		Tags:    []string{"refactoring", "quality", "maintainability"},
		Enabled: true,
	})

	// Add Feature Playbook
	r.Register(&Playbook{
		ID:          "add-feature",
		Name:        "Add Feature",
		Type:        PlaybookTypeAddFeature,
		Description: "Feature development workflow: design, implement, test, document",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "af-1", Order: 1, Type: "read", Action: "Read related code and interfaces", Target: "{{related_files}}", Required: true},
			{ID: "af-2", Order: 2, Type: "write", Action: "Implement feature", Target: "{{source_file}}", Required: true},
			{ID: "af-3", Order: 3, Type: "write", Action: "Write feature tests", Target: "{{test_file}}", Required: true},
			{ID: "af-4", Order: 4, Type: "test", Action: "Run feature tests", Target: "{{test_file}}", Required: true},
			{ID: "af-5", Order: 5, Type: "build", Action: "Build project", Target: "{{build_target}}", Required: true},
			{ID: "af-6", Order: 6, Type: "write", Action: "Update documentation", Target: "{{doc_file}}", Required: false},
		},
		SuccessCriteria: []string{"Feature implemented", "Tests pass", "Build succeeds", "Documented"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"add", "implement", "feature", "new", "create"},
			Patterns: []string{`add\s+(new\s+)?feature`, `implement\s+\w+`, `create\s+new\s+\w+`},
			Priority: 7,
		},
		Tags:    []string{"feature", "development", "new"},
		Enabled: true,
	})

	// Optimize Playbook
	r.Register(&Playbook{
		ID:          "optimize",
		Name:        "Optimize Performance",
		Type:        PlaybookTypeOptimize,
		Description: "Performance optimization workflow: profile, optimize, benchmark",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "op-1", Order: 1, Type: "read", Action: "Read code to optimize", Target: "{{source_file}}", Required: true},
			{ID: "op-2", Order: 2, Type: "command", Action: "Profile current performance", Target: "{{profile_command}}", Required: true},
			{ID: "op-3", Order: 3, Type: "write", Action: "Apply optimizations", Target: "{{source_file}}", Required: true},
			{ID: "op-4", Order: 4, Type: "command", Action: "Benchmark after optimization", Target: "{{benchmark_command}}", Required: true},
			{ID: "op-5", Order: 5, Type: "test", Action: "Run tests to verify correctness", Target: "{{test_target}}", Required: true},
		},
		SuccessCriteria: []string{"Performance improved", "Tests pass", "No regressions"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"optimize", "performance", "speed", "faster", "slow"},
			Patterns: []string{`optimize\s+\w+`, `improve\s+performance`, `make\s+\w+\s+faster`},
			Priority: 8,
		},
		Tags:    []string{"performance", "optimization", "speed"},
		Enabled: true,
	})

	// Document Playbook
	r.Register(&Playbook{
		ID:          "document",
		Name:        "Add Documentation",
		Type:        PlaybookTypeDocument,
		Description: "Documentation workflow: read, document, verify",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "doc-1", Order: 1, Type: "read", Action: "Read code to document", Target: "{{source_file}}", Required: true},
			{ID: "doc-2", Order: 2, Type: "write", Action: "Write code comments and docstrings", Target: "{{source_file}}", Required: true},
			{ID: "doc-3", Order: 3, Type: "write", Action: "Update README or docs", Target: "{{doc_file}}", Required: false},
		},
		SuccessCriteria: []string{"Code documented", "Examples provided"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"document", "docs", "comment", "readme", "explain"},
			Patterns: []string{`add\s+documentation`, `document\s+\w+`, `write\s+docs`, `add\s+comments`},
			Priority: 6,
		},
		Tags:    []string{"documentation", "comments", "readability"},
		Enabled: true,
	})

	// Security Playbook
	r.Register(&Playbook{
		ID:          "security",
		Name:        "Security Fix",
		Type:        PlaybookTypeSecurity,
		Description: "Security vulnerability fix workflow",
		Version:     "1.0.0",
		Author:      "iterate",
		CreatedAt:   now,
		UpdatedAt:   now,
		Steps: []PlaybookStep{
			{ID: "sec-1", Order: 1, Type: "read", Action: "Identify vulnerability", Target: "{{source_file}}", Required: true},
			{ID: "sec-2", Order: 2, Type: "command", Action: "Run security scanner", Target: "{{security_scan}}", Required: true},
			{ID: "sec-3", Order: 3, Type: "write", Action: "Apply security fix", Target: "{{source_file}}", Required: true},
			{ID: "sec-4", Order: 4, Type: "test", Action: "Run tests", Target: "{{test_target}}", Required: true},
			{ID: "sec-5", Order: 5, Type: "command", Action: "Verify fix with scanner", Target: "{{security_scan}}", Required: true},
		},
		SuccessCriteria: []string{"Vulnerability fixed", "Security scan passes", "Tests pass"},
		Triggers: PlaybookTrigger{
			Keywords: []string{"security", "vulnerability", "cve", "exploit", "sanitize"},
			Patterns: []string{`fix\s+security`, `fix\s+vulnerability`, `address\s+cve`, `sanitize\s+input`},
			Priority: 10,
		},
		Tags:    []string{"security", "vulnerability", "safety"},
		Enabled: true,
	})
}

// Register adds a playbook to the registry
func (r *PlaybookRegistry) Register(playbook *Playbook) error {
	if playbook.ID == "" {
		return fmt.Errorf("playbook ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	playbook.UpdatedAt = time.Now()
	r.playbooks[playbook.ID] = playbook
	r.byType[playbook.Type] = append(r.byType[playbook.Type], playbook.ID)

	return nil
}

// Unregister removes a playbook from the registry
func (r *PlaybookRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, exists := r.playbooks[id]; exists {
		// Remove from byType index
		typeIDs := r.byType[p.Type]
		for i, tid := range typeIDs {
			if tid == id {
				r.byType[p.Type] = append(typeIDs[:i], typeIDs[i+1:]...)
				break
			}
		}
		delete(r.playbooks, id)
	}
}

// Get retrieves a playbook by ID
func (r *PlaybookRegistry) Get(id string) (*Playbook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.playbooks[id]
	if !exists {
		return nil, fmt.Errorf("playbook not found: %s", id)
	}
	return p, nil
}

// GetByType retrieves all playbooks of a given type
func (r *PlaybookRegistry) GetByType(playbookType PlaybookType) []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Playbook
	for _, id := range r.byType[playbookType] {
		if p, exists := r.playbooks[id]; exists {
			result = append(result, p)
		}
	}
	return result
}

// List returns all registered playbooks
func (r *PlaybookRegistry) List() []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Playbook, 0, len(r.playbooks))
	for _, p := range r.playbooks {
		result = append(result, p)
	}
	return result
}

// ListEnabled returns all enabled playbooks
func (r *PlaybookRegistry) ListEnabled() []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Playbook
	for _, p := range r.playbooks {
		if p.Enabled {
			result = append(result, p)
		}
	}
	return result
}

// Match finds the best matching playbook for a task description
func (r *PlaybookRegistry) Match(taskDescription string) *Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskLower := strings.ToLower(taskDescription)
	var bestMatch *Playbook
	bestScore := 0

	for _, p := range r.playbooks {
		if !p.Enabled {
			continue
		}

		score := r.calculateMatchScore(taskLower, p)
		if score > bestScore {
			bestScore = score
			bestMatch = p
		}
	}

	return bestMatch
}

// calculateMatchScore calculates how well a playbook matches a task
func (r *PlaybookRegistry) calculateMatchScore(taskLower string, playbook *Playbook) int {
	score := 0

	// Check keyword matches
	for _, keyword := range playbook.Triggers.Keywords {
		if strings.Contains(taskLower, strings.ToLower(keyword)) {
			score += playbook.Triggers.Priority
		}
	}

	// Check pattern matches
	for _, pattern := range playbook.Triggers.Patterns {
		matched, err := regexp.MatchString("(?i)"+pattern, taskLower)
		if err == nil && matched {
			score += playbook.Triggers.Priority * 2
		}
	}

	// Type name match
	if strings.Contains(taskLower, string(playbook.Type)) {
		score += 5
	}

	return score
}

// MatchAll returns all matching playbooks sorted by relevance
func (r *PlaybookRegistry) MatchAll(taskDescription string) []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskLower := strings.ToLower(taskDescription)
	type scored struct {
		playbook *Playbook
		score    int
	}

	var matches []scored
	for _, p := range r.playbooks {
		if !p.Enabled {
			continue
		}
		score := r.calculateMatchScore(taskLower, p)
		if score > 0 {
			matches = append(matches, scored{playbook: p, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].score > matches[i].score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	result := make([]*Playbook, len(matches))
	for i, m := range matches {
		result[i] = m.playbook
	}
	return result
}

// SetLogger sets the logger for the registry
func (r *PlaybookRegistry) SetLogger(logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// Enable enables a playbook
func (r *PlaybookRegistry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.playbooks[id]
	if !exists {
		return fmt.Errorf("playbook not found: %s", id)
	}
	p.Enabled = true
	p.UpdatedAt = time.Now()
	return nil
}

// Disable disables a playbook
func (r *PlaybookRegistry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.playbooks[id]
	if !exists {
		return fmt.Errorf("playbook not found: %s", id)
	}
	p.Enabled = false
	p.UpdatedAt = time.Now()
	return nil
}

// InstantiatePlaybook creates a concrete execution plan from a playbook
func (r *PlaybookRegistry) InstantiatePlaybook(ctx context.Context, playbookID string, variables map[string]string) (*PlaybookInstance, error) {
	playbook, err := r.Get(playbookID)
	if err != nil {
		return nil, err
	}

	return NewPlaybookInstance(playbook, variables), nil
}

// PlaybookInstance represents a concrete instance of a playbook with resolved variables
type PlaybookInstance struct {
	Playbook    *Playbook
	Variables   map[string]string
	Steps       []ResolvedStep
	CreatedAt   time.Time
	Status      InstanceStatus
	CurrentStep int
}

// InstanceStatus represents the status of a playbook instance
type InstanceStatus string

const (
	InstanceStatusPending   InstanceStatus = "pending"
	InstanceStatusRunning   InstanceStatus = "running"
	InstanceStatusCompleted InstanceStatus = "completed"
	InstanceStatusFailed    InstanceStatus = "failed"
	InstanceStatusCancelled InstanceStatus = "cancelled"
)

// ResolvedStep is a playbook step with variables resolved
type ResolvedStep struct {
	PlaybookStep
	ResolvedAction string
	ResolvedTarget string
	Status         PlaybookStepStatus
	Output         string
	Error          error
}

// PlaybookStepStatus represents the status of a step
type PlaybookStepStatus string

const (
	PlaybookStepStatusPending   PlaybookStepStatus = "pending"
	PlaybookStepStatusRunning   PlaybookStepStatus = "running"
	PlaybookStepStatusCompleted PlaybookStepStatus = "completed"
	PlaybookStepStatusFailed    PlaybookStepStatus = "failed"
	PlaybookStepStatusSkipped   PlaybookStepStatus = "skipped"
)

// NewPlaybookInstance creates a new instance from a playbook
func NewPlaybookInstance(playbook *Playbook, variables map[string]string) *PlaybookInstance {
	// Merge variables with defaults
	mergedVars := make(map[string]string)
	for k, v := range playbook.Variables {
		mergedVars[k] = v
	}
	for k, v := range variables {
		mergedVars[k] = v
	}

	instance := &PlaybookInstance{
		Playbook:    playbook,
		Variables:   mergedVars,
		Steps:       make([]ResolvedStep, len(playbook.Steps)),
		CreatedAt:   time.Now(),
		Status:      InstanceStatusPending,
		CurrentStep: 0,
	}

	// Resolve steps
	for i, step := range playbook.Steps {
		instance.Steps[i] = ResolvedStep{
			PlaybookStep:   step,
			ResolvedAction: resolveVariables(step.Action, mergedVars),
			ResolvedTarget: resolveVariables(step.Target, mergedVars),
			Status:         PlaybookStepStatusPending,
		}
	}

	return instance
}

// resolveVariables substitutes {{variable}} patterns
func resolveVariables(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// Start begins execution of the playbook instance
func (pi *PlaybookInstance) Start() {
	pi.Status = InstanceStatusRunning
}

// AdvanceStep moves to the next step
func (pi *PlaybookInstance) AdvanceStep() bool {
	pi.CurrentStep++
	if pi.CurrentStep >= len(pi.Steps) {
		pi.Status = InstanceStatusCompleted
		return false
	}
	return true
}

// GetCurrentStep returns the current step
func (pi *PlaybookInstance) GetCurrentStep() *ResolvedStep {
	if pi.CurrentStep < len(pi.Steps) {
		return &pi.Steps[pi.CurrentStep]
	}
	return nil
}

// MarkStepCompleted marks the current step as completed
func (pi *PlaybookInstance) MarkStepCompleted(output string) {
	if pi.CurrentStep < len(pi.Steps) {
		pi.Steps[pi.CurrentStep].Status = PlaybookStepStatusCompleted
		pi.Steps[pi.CurrentStep].Output = output
	}
}

// MarkStepFailed marks the current step as failed
func (pi *PlaybookInstance) MarkStepFailed(err error) {
	if pi.CurrentStep < len(pi.Steps) {
		pi.Steps[pi.CurrentStep].Status = PlaybookStepStatusFailed
		pi.Steps[pi.CurrentStep].Error = err
	}
	pi.Status = InstanceStatusFailed
}

// MarkStepSkipped marks the current step as skipped
func (pi *PlaybookInstance) MarkStepSkipped(reason string) {
	if pi.CurrentStep < len(pi.Steps) {
		pi.Steps[pi.CurrentStep].Status = PlaybookStepStatusSkipped
		pi.Steps[pi.CurrentStep].Output = reason
	}
}

// IsComplete returns true if the instance has completed all steps
func (pi *PlaybookInstance) IsComplete() bool {
	return pi.Status == InstanceStatusCompleted
}

// IsFailed returns true if the instance has failed
func (pi *PlaybookInstance) IsFailed() bool {
	return pi.Status == InstanceStatusFailed
}

// GetProgress returns the completion percentage
func (pi *PlaybookInstance) GetProgress() float64 {
	if len(pi.Steps) == 0 {
		return 100.0
	}
	completed := 0
	for _, step := range pi.Steps {
		if step.Status == PlaybookStepStatusCompleted || step.Status == PlaybookStepStatusSkipped {
			completed++
		}
	}
	return float64(completed) / float64(len(pi.Steps)) * 100
}

// GetSummary returns a summary of the instance execution
func (pi *PlaybookInstance) GetSummary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Playbook: %s\n", pi.Playbook.Name))
	sb.WriteString(fmt.Sprintf("Status: %s\n", pi.Status))
	sb.WriteString(fmt.Sprintf("Progress: %.0f%%\n", pi.GetProgress()))
	sb.WriteString("Steps:\n")

	for i, step := range pi.Steps {
		marker := " "
		if i == pi.CurrentStep && pi.Status == InstanceStatusRunning {
			marker = ">"
		}
		statusIcon := "○"
		switch step.Status {
		case PlaybookStepStatusCompleted:
			statusIcon = "✓"
		case PlaybookStepStatusFailed:
			statusIcon = "✗"
		case PlaybookStepStatusRunning:
			statusIcon = "►"
		case PlaybookStepStatusSkipped:
			statusIcon = "○"
		}
		sb.WriteString(fmt.Sprintf("  %s %s [%s] %s\n", marker, statusIcon, step.Type, step.ResolvedAction))
	}

	return sb.String()
}

// PlaybookBuilder helps create custom playbooks
type PlaybookBuilder struct {
	playbook *Playbook
}

// NewPlaybookBuilder creates a new playbook builder
func NewPlaybookBuilder(id, name string, playbookType PlaybookType) *PlaybookBuilder {
	now := time.Now()
	return &PlaybookBuilder{
		playbook: &Playbook{
			ID:        id,
			Name:      name,
			Type:      playbookType,
			Version:   "1.0.0",
			Author:    "custom",
			CreatedAt: now,
			UpdatedAt: now,
			Steps:     []PlaybookStep{},
			Variables: make(map[string]string),
			Metadata:  make(map[string]any),
			Tags:      []string{},
			Enabled:   true,
		},
	}
}

// WithDescription sets the description
func (b *PlaybookBuilder) WithDescription(desc string) *PlaybookBuilder {
	b.playbook.Description = desc
	return b
}

// WithVersion sets the version
func (b *PlaybookBuilder) WithVersion(version string) *PlaybookBuilder {
	b.playbook.Version = version
	return b
}

// WithAuthor sets the author
func (b *PlaybookBuilder) WithAuthor(author string) *PlaybookBuilder {
	b.playbook.Author = author
	return b
}

// AddStep adds a step to the playbook
func (b *PlaybookBuilder) AddStep(stepType, action, target string, required bool) *PlaybookBuilder {
	step := PlaybookStep{
		ID:        fmt.Sprintf("%s-%d", b.playbook.ID, len(b.playbook.Steps)+1),
		Order:     len(b.playbook.Steps) + 1,
		Type:      stepType,
		Action:    action,
		Target:    target,
		Required:  required,
		Variables: make(map[string]string),
	}
	b.playbook.Steps = append(b.playbook.Steps, step)
	return b
}

// AddSuccessCriteria adds success criteria
func (b *PlaybookBuilder) AddSuccessCriteria(criteria string) *PlaybookBuilder {
	b.playbook.SuccessCriteria = append(b.playbook.SuccessCriteria, criteria)
	return b
}

// AddTriggerKeywords adds trigger keywords
func (b *PlaybookBuilder) AddTriggerKeywords(keywords ...string) *PlaybookBuilder {
	b.playbook.Triggers.Keywords = append(b.playbook.Triggers.Keywords, keywords...)
	return b
}

// AddTriggerPatterns adds trigger regex patterns
func (b *PlaybookBuilder) AddTriggerPatterns(patterns ...string) *PlaybookBuilder {
	b.playbook.Triggers.Patterns = append(b.playbook.Triggers.Patterns, patterns...)
	return b
}

// SetTriggerPriority sets the trigger priority
func (b *PlaybookBuilder) SetTriggerPriority(priority int) *PlaybookBuilder {
	b.playbook.Triggers.Priority = priority
	return b
}

// AddVariable adds a default variable
func (b *PlaybookBuilder) AddVariable(key, value string) *PlaybookBuilder {
	b.playbook.Variables[key] = value
	return b
}

// AddTag adds a tag
func (b *PlaybookBuilder) AddTag(tag string) *PlaybookBuilder {
	b.playbook.Tags = append(b.playbook.Tags, tag)
	return b
}

// SetEnabled sets the enabled status
func (b *PlaybookBuilder) SetEnabled(enabled bool) *PlaybookBuilder {
	b.playbook.Enabled = enabled
	return b
}

// Build returns the built playbook
func (b *PlaybookBuilder) Build() *Playbook {
	b.playbook.UpdatedAt = time.Now()
	return b.playbook
}

// GetStats returns statistics about the registry
func (r *PlaybookRegistry) GetStats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typeCount := make(map[PlaybookType]int)
	enabled := 0
	for _, p := range r.playbooks {
		typeCount[p.Type]++
		if p.Enabled {
			enabled++
		}
	}

	return map[string]any{
		"total_playbooks":   len(r.playbooks),
		"enabled_playbooks": enabled,
		"playbooks_by_type": typeCount,
	}
}
