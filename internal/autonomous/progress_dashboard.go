// Package autonomous - Task 13: Progress Dashboard for long-running tasks
package autonomous

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressDashboard provides real-time progress tracking for autonomous operations.
type ProgressDashboard struct {
	mu          sync.RWMutex
	taskName    string
	startTime   time.Time
	steps       []*ProgressStep
	currentStep *ProgressStep
	status      DashboardStatus
	progress    float64

	// Metrics
	totalOperations int
	completedOps    int
	failedOps       int
	retries         int
	tokensUsed      int64
	cost            float64

	// Display options
	output         io.Writer
	refreshRate    time.Duration
	showTimestamps bool
	compact        bool

	// Live updates
	stopChan   chan struct{}
	running    bool
	lastUpdate time.Time
}

// DashboardStatus represents the overall status.
type DashboardStatus string

const (
	DashboardStatusRunning   DashboardStatus = "running"
	DashboardStatusPaused    DashboardStatus = "paused"
	DashboardStatusCompleted DashboardStatus = "completed"
	DashboardStatusFailed    DashboardStatus = "failed"
	DashboardStatusCancelled DashboardStatus = "cancelled"
)

// ProgressStep represents a single step in the operation.
type ProgressStep struct {
	ID          string
	Name        string
	Description string
	Status      StepStatus
	StartedAt   time.Time
	CompletedAt time.Time
	Duration    time.Duration
	Progress    float64 // 0.0 - 1.0
	SubSteps    []*ProgressStep
	Error       string
	Metadata    map[string]any
}

// StepStatus represents the status of a step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepRetrying  StepStatus = "retrying"
)

// DashboardConfig configures the dashboard.
type DashboardConfig struct {
	TaskName       string
	Output         io.Writer
	RefreshRate    time.Duration
	ShowTimestamps bool
	Compact        bool
}

// NewProgressDashboard creates a new progress dashboard.
func NewProgressDashboard(config DashboardConfig) *ProgressDashboard {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.RefreshRate == 0 {
		config.RefreshRate = 500 * time.Millisecond
	}

	return &ProgressDashboard{
		taskName:       config.TaskName,
		startTime:      time.Now(),
		steps:          make([]*ProgressStep, 0),
		status:         DashboardStatusRunning,
		output:         config.Output,
		refreshRate:    config.RefreshRate,
		showTimestamps: config.ShowTimestamps,
		compact:        config.Compact,
		stopChan:       make(chan struct{}),
	}
}

// AddStep adds a new step to track.
func (d *ProgressDashboard) AddStep(id, name, description string) *ProgressStep {
	d.mu.Lock()
	defer d.mu.Unlock()

	step := &ProgressStep{
		ID:          id,
		Name:        name,
		Description: description,
		Status:      StepPending,
		StartedAt:   time.Now(),
		Progress:    0,
		SubSteps:    make([]*ProgressStep, 0),
		Metadata:    make(map[string]any),
	}

	d.steps = append(d.steps, step)
	d.totalOperations++

	return step
}

// StartStep marks a step as running.
func (d *ProgressDashboard) StartStep(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Status = StepRunning
			step.StartedAt = time.Now()
			d.currentStep = step
			break
		}
	}
	d.lastUpdate = time.Now()
}

// UpdateStepProgress updates the progress of a step.
func (d *ProgressDashboard) UpdateStepProgress(id string, progress float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Progress = progress
			break
		}
	}
	d.lastUpdate = time.Now()
}

// CompleteStep marks a step as completed.
func (d *ProgressDashboard) CompleteStep(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Status = StepCompleted
			step.CompletedAt = time.Now()
			step.Duration = step.CompletedAt.Sub(step.StartedAt)
			step.Progress = 1.0
			d.completedOps++
			break
		}
	}
	d.lastUpdate = time.Now()
}

// FailStep marks a step as failed.
func (d *ProgressDashboard) FailStep(id, errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Status = StepFailed
			step.CompletedAt = time.Now()
			step.Duration = step.CompletedAt.Sub(step.StartedAt)
			step.Error = errMsg
			d.failedOps++
			break
		}
	}
	d.lastUpdate = time.Now()
}

// RetryStep marks a step as retrying.
func (d *ProgressDashboard) RetryStep(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Status = StepRetrying
			d.retries++
			break
		}
	}
	d.lastUpdate = time.Now()
}

// SkipStep marks a step as skipped.
func (d *ProgressDashboard) SkipStep(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == id {
			step.Status = StepSkipped
			step.CompletedAt = time.Now()
			break
		}
	}
	d.lastUpdate = time.Now()
}

// AddSubStep adds a sub-step to an existing step.
func (d *ProgressDashboard) AddSubStep(parentID, id, name string) *ProgressStep {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, step := range d.steps {
		if step.ID == parentID {
			subStep := &ProgressStep{
				ID:        id,
				Name:      name,
				Status:    StepPending,
				StartedAt: time.Now(),
				SubSteps:  make([]*ProgressStep, 0),
				Metadata:  make(map[string]any),
			}
			step.SubSteps = append(step.SubSteps, subStep)
			return subStep
		}
	}
	return nil
}

// SetProgress sets the overall progress.
func (d *ProgressDashboard) SetProgress(progress float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progress = progress
	d.lastUpdate = time.Now()
}

// SetStatus sets the dashboard status.
func (d *ProgressDashboard) SetStatus(status DashboardStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status = status
	d.lastUpdate = time.Now()
}

// SetTokens updates token usage.
func (d *ProgressDashboard) SetTokens(tokens int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tokensUsed = tokens
}

// SetCost updates cost.
func (d *ProgressDashboard) SetCost(cost float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cost = cost
}

// IncrementRetries increments the retry counter.
func (d *ProgressDashboard) IncrementRetries() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.retries++
}

// Render renders the dashboard to the output.
func (d *ProgressDashboard) Render() {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sb strings.Builder

	// Clear screen if not compact
	if !d.compact {
		sb.WriteString("\033[2J\033[H")
	}

	// Header
	sb.WriteString(d.renderHeader())
	sb.WriteString("\n")

	// Progress bar
	sb.WriteString(d.renderProgressBar())
	sb.WriteString("\n\n")

	// Steps
	sb.WriteString(d.renderSteps())
	sb.WriteString("\n")

	// Metrics
	sb.WriteString(d.renderMetrics())
	sb.WriteString("\n")

	// Footer
	sb.WriteString(d.renderFooter())

	fmt.Fprint(d.output, sb.String())
}

// renderHeader renders the dashboard header.
func (d *ProgressDashboard) renderHeader() string {
	var sb strings.Builder

	// Title
	sb.WriteString("╭──────────────────────────────────────────────────────────────╮\n")
	title := fmt.Sprintf("│ %-60s │", truncate(d.taskName, 60))
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString("├──────────────────────────────────────────────────────────────┤\n")

	// Status and duration
	duration := time.Since(d.startTime)
	statusIcon := d.getStatusIcon()
	statusLine := fmt.Sprintf("│ Status: %s %-12s Duration: %-20s │",
		statusIcon, d.status, formatDuration(duration))
	sb.WriteString(statusLine)
	sb.WriteString("\n")

	return sb.String()
}

// renderProgressBar renders the progress bar.
func (d *ProgressDashboard) renderProgressBar() string {
	var sb strings.Builder

	progress := d.progress
	if progress == 0 && d.totalOperations > 0 {
		progress = float64(d.completedOps) / float64(d.totalOperations)
	}

	// Progress bar
	barWidth := 40
	filled := int(progress * float64(barWidth))

	sb.WriteString("│ Progress: [")
	for i := 0; i < barWidth; i++ {
		if i < filled {
			sb.WriteString("█")
		} else {
			sb.WriteString("░")
		}
	}
	sb.WriteString("] ")
	sb.WriteString(fmt.Sprintf("%5.1f%%", progress*100))
	sb.WriteString("     │\n")

	return sb.String()
}

// renderSteps renders the steps section.
func (d *ProgressDashboard) renderSteps() string {
	var sb strings.Builder

	sb.WriteString("├──────────────────────────────────────────────────────────────┤\n")
	sb.WriteString("│ Steps                                                        │\n")
	sb.WriteString("├──────────────────────────────────────────────────────────────┤\n")

	for _, step := range d.steps {
		sb.WriteString(d.renderStep(step, 0))
	}

	return sb.String()
}

// renderStep renders a single step.
func (d *ProgressDashboard) renderStep(step *ProgressStep, indent int) string {
	var sb strings.Builder

	prefix := strings.Repeat("  ", indent)
	icon := d.getStepIcon(step.Status)

	// Step line
	line := fmt.Sprintf("│ %s%s %-20s %s %s",
		prefix,
		icon,
		truncate(step.Name, 20),
		step.Status,
		formatDuration(step.Duration),
	)

	// Pad to width
	line = fmt.Sprintf("%-62s │", line)
	sb.WriteString(line)
	sb.WriteString("\n")

	// Error if present
	if step.Error != "" {
		errLine := fmt.Sprintf("│ %s   ⚠ Error: %-44s │", prefix, truncate(step.Error, 44))
		sb.WriteString(errLine)
		sb.WriteString("\n")
	}

	// Sub-steps
	for _, subStep := range step.SubSteps {
		sb.WriteString(d.renderStep(subStep, indent+1))
	}

	return sb.String()
}

// renderMetrics renders the metrics section.
func (d *ProgressDashboard) renderMetrics() string {
	var sb strings.Builder

	sb.WriteString("├──────────────────────────────────────────────────────────────┤\n")
	sb.WriteString("│ Metrics                                                      │\n")
	sb.WriteString("├──────────────────────────────────────────────────────────────┤\n")

	// Operations
	opsLine := fmt.Sprintf("│ Operations: %d total, %d completed, %d failed, %d retries %-9s │",
		d.totalOperations, d.completedOps, d.failedOps, d.retries, "")
	sb.WriteString(opsLine)
	sb.WriteString("\n")

	// Tokens and cost
	resourceLine := fmt.Sprintf("│ Tokens: %-8d Cost: $%-8.2f %-23s │",
		d.tokensUsed, d.cost, "")
	sb.WriteString(resourceLine)
	sb.WriteString("\n")

	return sb.String()
}

// renderFooter renders the footer.
func (d *ProgressDashboard) renderFooter() string {
	var sb strings.Builder

	sb.WriteString("╰──────────────────────────────────────────────────────────────╯\n")

	if d.showTimestamps {
		sb.WriteString(fmt.Sprintf("Last updated: %s\n", time.Now().Format("15:04:05")))
	}

	return sb.String()
}

// getStatusIcon returns the icon for a status.
func (d *ProgressDashboard) getStatusIcon() string {
	switch d.status {
	case DashboardStatusRunning:
		return "🔄"
	case DashboardStatusPaused:
		return "⏸️"
	case DashboardStatusCompleted:
		return "✅"
	case DashboardStatusFailed:
		return "❌"
	case DashboardStatusCancelled:
		return "🚫"
	default:
		return "❓"
	}
}

// getStepIcon returns the icon for a step status.
func (d *ProgressDashboard) getStepIcon(status StepStatus) string {
	switch status {
	case StepPending:
		return "⏳"
	case StepRunning:
		return "🔄"
	case StepCompleted:
		return "✅"
	case StepFailed:
		return "❌"
	case StepSkipped:
		return "⏭️"
	case StepRetrying:
		return "🔁"
	default:
		return "❓"
	}
}

// Start begins live updates.
func (d *ProgressDashboard) Start() {
	d.mu.Lock()
	d.running = true
	d.mu.Unlock()

	go func() {
		ticker := time.NewTicker(d.refreshRate)
		defer ticker.Stop()

		for {
			select {
			case <-d.stopChan:
				return
			case <-ticker.C:
				d.Render()
			}
		}
	}()
}

// Stop stops live updates.
func (d *ProgressDashboard) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		close(d.stopChan)
		d.running = false
	}
}

// Complete marks the dashboard as completed and renders final state.
func (d *ProgressDashboard) Complete() {
	d.SetStatus(DashboardStatusCompleted)
	d.Stop()
	d.Render()
}

// Fail marks the dashboard as failed.
func (d *ProgressDashboard) Fail(errMsg string) {
	d.SetStatus(DashboardStatusFailed)
	d.Stop()
	d.Render()
	fmt.Fprintf(d.output, "\n❌ Error: %s\n", errMsg)
}

// Cancel marks the dashboard as cancelled.
func (d *ProgressDashboard) Cancel() {
	d.SetStatus(DashboardStatusCancelled)
	d.Stop()
	d.Render()
}

// GetStats returns current dashboard statistics.
func (d *ProgressDashboard) GetStats() DashboardStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DashboardStats{
		TaskName:       d.taskName,
		Duration:       time.Since(d.startTime),
		Status:         d.status,
		Progress:       d.progress,
		TotalOps:       d.totalOperations,
		CompletedOps:   d.completedOps,
		FailedOps:      d.failedOps,
		Retries:        d.retries,
		TokensUsed:     d.tokensUsed,
		Cost:           d.cost,
		StepsCompleted: countCompletedSteps(d.steps),
		StepsTotal:     len(d.steps),
	}
}

// DashboardStats holds dashboard statistics.
type DashboardStats struct {
	TaskName       string
	Duration       time.Duration
	Status         DashboardStatus
	Progress       float64
	TotalOps       int
	CompletedOps   int
	FailedOps      int
	Retries        int
	TokensUsed     int64
	Cost           float64
	StepsCompleted int
	StepsTotal     int
}

// countCompletedSteps counts completed steps.
func countCompletedSteps(steps []*ProgressStep) int {
	count := 0
	for _, step := range steps {
		if step.Status == StepCompleted {
			count++
		}
		count += countCompletedSteps(step.SubSteps)
	}
	return count
}

// Helper functions

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// ExportJSON exports dashboard state as JSON-like string.
func (d *ProgressDashboard) ExportJSON() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"task_name\": \"%s\",\n", d.taskName))
	sb.WriteString(fmt.Sprintf("  \"status\": \"%s\",\n", d.status))
	sb.WriteString(fmt.Sprintf("  \"duration\": \"%s\",\n", formatDuration(time.Since(d.startTime))))
	sb.WriteString(fmt.Sprintf("  \"progress\": %.2f,\n", d.progress))
	sb.WriteString(fmt.Sprintf("  \"total_operations\": %d,\n", d.totalOperations))
	sb.WriteString(fmt.Sprintf("  \"completed_operations\": %d,\n", d.completedOps))
	sb.WriteString(fmt.Sprintf("  \"failed_operations\": %d,\n", d.failedOps))
	sb.WriteString(fmt.Sprintf("  \"retries\": %d,\n", d.retries))
	sb.WriteString(fmt.Sprintf("  \"tokens_used\": %d,\n", d.tokensUsed))
	sb.WriteString(fmt.Sprintf("  \"cost\": %.4f,\n", d.cost))
	sb.WriteString("  \"steps\": [\n")

	for i, step := range d.steps {
		sb.WriteString(d.exportStepJSON(step, 2))
		if i < len(d.steps)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  ]\n")
	sb.WriteString("}\n")

	return sb.String()
}

// exportStepJSON exports a step as JSON.
func (d *ProgressDashboard) exportStepJSON(step *ProgressStep, indent int) string {
	var sb strings.Builder
	prefix := strings.Repeat("  ", indent)

	sb.WriteString(fmt.Sprintf("%s{\n", prefix))
	sb.WriteString(fmt.Sprintf("%s  \"id\": \"%s\",\n", prefix, step.ID))
	sb.WriteString(fmt.Sprintf("%s  \"name\": \"%s\",\n", prefix, step.Name))
	sb.WriteString(fmt.Sprintf("%s  \"status\": \"%s\",\n", prefix, step.Status))
	sb.WriteString(fmt.Sprintf("%s  \"progress\": %.2f,\n", prefix, step.Progress))
	sb.WriteString(fmt.Sprintf("%s  \"duration\": \"%s\"", prefix, formatDuration(step.Duration)))

	if len(step.SubSteps) > 0 {
		sb.WriteString(",\n")
		sb.WriteString(fmt.Sprintf("%s  \"sub_steps\": [\n", prefix))
		for i, subStep := range step.SubSteps {
			sb.WriteString(d.exportStepJSON(subStep, indent+2))
			if i < len(step.SubSteps)-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%s  ]\n", prefix))
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("%s}", prefix))

	return sb.String()
}
