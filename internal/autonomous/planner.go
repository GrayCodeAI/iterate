// Package autonomous - Task 3: Multi-step planning engine with dependency graph
package autonomous

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// DependencyGraph represents the dependency relationships between plan steps.
type DependencyGraph struct {
	mu           sync.RWMutex
	nodes        map[int]*PlanNode
	edges        map[int][]int // node -> dependencies
	reverseEdges map[int][]int // node -> dependents
}

// PlanNode wraps a PlanStep with dependency and status information.
type PlanNode struct {
	Step         PlanStep
	ID           int
	Dependencies []int
	Status       NodeStatus
	Priority     int
}

// NodeStatus represents the execution status of a plan node.
type NodeStatus string

const (
	StatusPending   NodeStatus = "pending"
	StatusReady     NodeStatus = "ready" // All dependencies met
	StatusRunning   NodeStatus = "running"
	StatusCompleted NodeStatus = "completed"
	StatusFailed    NodeStatus = "failed"
	StatusSkipped   NodeStatus = "skipped"
	StatusBlocked   NodeStatus = "blocked" // Dependencies failed
)

// Planner manages multi-step plans with dependency resolution.
type Planner struct {
	graph      *DependencyGraph
	maxRetries int
	logger     *slog.Logger
	mu         sync.RWMutex
}

// NewPlanner creates a new planner instance.
func NewPlanner(logger *slog.Logger) *Planner {
	return &Planner{
		graph: &DependencyGraph{
			nodes:        make(map[int]*PlanNode),
			edges:        make(map[int][]int),
			reverseEdges: make(map[int][]int),
		},
		maxRetries: 3,
		logger:     logger,
	}
}

// AddStep adds a new step to the plan with optional dependencies.
func (p *Planner) AddStep(step PlanStep, dependencies []int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := len(p.graph.nodes)

	node := &PlanNode{
		Step:         step,
		ID:           id,
		Dependencies: dependencies,
		Status:       StatusPending,
		Priority:     calculatePriority(step),
	}

	p.graph.nodes[id] = node
	p.graph.edges[id] = dependencies

	// Build reverse edges for efficient lookups
	for _, dep := range dependencies {
		p.graph.reverseEdges[dep] = append(p.graph.reverseEdges[dep], id)
	}

	// Check for cycles
	if p.hasCycle(id, make(map[int]bool)) {
		delete(p.graph.nodes, id)
		delete(p.graph.edges, id)
		return -1, fmt.Errorf("adding step %d would create a cycle in the dependency graph", id)
	}

	return id, nil
}

// hasCycle performs DFS to detect cycles in the dependency graph.
func (p *Planner) hasCycle(nodeID int, visited map[int]bool) bool {
	if visited[nodeID] {
		return true
	}

	visited[nodeID] = true

	for _, dep := range p.graph.edges[nodeID] {
		if p.hasCycle(dep, visited) {
			return true
		}
	}

	delete(visited, nodeID)
	return false
}

// GetExecutionOrder returns steps in topological order (dependencies first).
func (p *Planner) GetExecutionOrder() ([]int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Kahn's algorithm for topological sort
	inDegree := make(map[int]int)
	for id := range p.graph.nodes {
		inDegree[id] = 0
	}

	for _, deps := range p.graph.edges {
		for _, dep := range deps {
			inDegree[dep]++ // Count how many nodes depend on this
		}
	}

	// Fix: inDegree should count dependencies, not dependents
	// Reset and recalculate correctly
	inDegree = make(map[int]int)
	for id := range p.graph.nodes {
		inDegree[id] = len(p.graph.edges[id])
	}

	// Queue of nodes with no dependencies
	queue := make([]int, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort queue by priority for deterministic ordering
	sort.Slice(queue, func(i, j int) bool {
		return p.graph.nodes[queue[i]].Priority > p.graph.nodes[queue[j]].Priority
	})

	result := make([]int, 0)

	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Process nodes that depend on current
		for _, dependent := range p.graph.reverseEdges[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				// Re-sort queue by priority
				sort.Slice(queue, func(i, j int) bool {
					return p.graph.nodes[queue[i]].Priority > p.graph.nodes[queue[j]].Priority
				})
			}
		}
	}

	if len(result) != len(p.graph.nodes) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return result, nil
}

// GetReadySteps returns all steps that are ready to execute (dependencies met).
func (p *Planner) GetReadySteps() []*PlanNode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ready := make([]*PlanNode, 0)

	for _, node := range p.graph.nodes {
		if node.Status != StatusPending {
			continue
		}

		allDependenciesMet := true
		for _, depID := range node.Dependencies {
			depNode, exists := p.graph.nodes[depID]
			if !exists || depNode.Status != StatusCompleted {
				allDependenciesMet = false
				break
			}
		}

		if allDependenciesMet {
			node.Status = StatusReady
			ready = append(ready, node)
		}
	}

	// Sort by priority (higher first)
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority > ready[j].Priority
	})

	return ready
}

// GetParallelGroups returns groups of steps that can be executed in parallel.
func (p *Planner) GetParallelGroups() ([][]int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Calculate in-degree for each node
	inDegree := make(map[int]int)
	for id := range p.graph.nodes {
		inDegree[id] = len(p.graph.edges[id])
	}

	groups := make([][]int, 0)
	processed := make(map[int]bool)

	for len(processed) < len(p.graph.nodes) {
		group := make([]int, 0)

		for id, degree := range inDegree {
			if processed[id] {
				continue
			}
			if degree == 0 {
				group = append(group, id)
			}
		}

		if len(group) == 0 && len(processed) < len(p.graph.nodes) {
			return nil, fmt.Errorf("cycle detected in dependency graph")
		}

		// Sort by priority within group
		sort.Slice(group, func(i, j int) bool {
			return p.graph.nodes[group[i]].Priority > p.graph.nodes[group[j]].Priority
		})

		groups = append(groups, group)

		// Mark as processed and update in-degrees
		for _, id := range group {
			processed[id] = true
			for _, dependent := range p.graph.reverseEdges[id] {
				inDegree[dependent]--
			}
		}
	}

	return groups, nil
}

// MarkStepStatus updates the status of a step and handles cascading effects.
func (p *Planner) MarkStepStatus(stepID int, status NodeStatus) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	node, exists := p.graph.nodes[stepID]
	if !exists {
		return fmt.Errorf("step %d not found", stepID)
	}

	node.Status = status

	// Handle cascading effects
	if status == StatusFailed {
		p.markDependentsBlocked(stepID)
	}

	return nil
}

// markDependentsBlocked marks all dependent steps as blocked when a step fails.
func (p *Planner) markDependentsBlocked(stepID int) {
	for _, dependent := range p.graph.reverseEdges[stepID] {
		node := p.graph.nodes[dependent]
		if node.Status == StatusPending || node.Status == StatusReady {
			node.Status = StatusBlocked
			p.markDependentsBlocked(dependent)
		}
	}
}

// GetStatus returns the overall status of the plan.
func (p *Planner) GetStatus() PlanStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.graph.nodes) == 0 {
		return PlanStatusEmpty
	}

	completed := 0
	failed := 0
	blocked := 0
	running := 0
	pending := 0

	for _, node := range p.graph.nodes {
		switch node.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		case StatusBlocked:
			blocked++
		case StatusRunning:
			running++
		case StatusPending, StatusReady:
			pending++
		}
	}

	if completed == len(p.graph.nodes) {
		return PlanStatusCompleted
	}
	if failed > 0 || blocked > 0 {
		if pending == 0 && running == 0 {
			return PlanStatusFailed
		}
		return PlanStatusPartialFailure
	}
	if running > 0 {
		return PlanStatusRunning
	}
	return PlanStatusPending
}

// PlanStatus represents the overall status of a plan.
type PlanStatus string

const (
	PlanStatusEmpty          PlanStatus = "empty"
	PlanStatusPending        PlanStatus = "pending"
	PlanStatusRunning        PlanStatus = "running"
	PlanStatusCompleted      PlanStatus = "completed"
	PlanStatusFailed         PlanStatus = "failed"
	PlanStatusPartialFailure PlanStatus = "partial_failure"
)

// BuildPlanFromSteps creates a plan with dependency analysis from steps.
func (p *Planner) BuildPlanFromSteps(steps []PlanStep) error {
	// First pass: add all steps
	ids := make([]int, len(steps))
	for i, step := range steps {
		id, err := p.AddStep(step, nil)
		if err != nil {
			return fmt.Errorf("failed to add step %d: %w", i, err)
		}
		ids[i] = id
	}

	// Second pass: analyze and add dependencies based on step types
	for i, step := range steps {
		deps := p.inferDependencies(step, steps[:i], ids[:i])
		if len(deps) > 0 {
			p.graph.edges[ids[i]] = deps
			p.graph.nodes[ids[i]].Dependencies = deps
			for _, dep := range deps {
				p.graph.reverseEdges[dep] = append(p.graph.reverseEdges[dep], ids[i])
			}
		}
	}

	return nil
}

// inferDependencies analyzes step types to infer logical dependencies.
func (p *Planner) inferDependencies(step PlanStep, previousSteps []PlanStep, previousIDs []int) []int {
	deps := make([]int, 0)

	stepType := strings.ToLower(step.Type)

	// Write operations depend on reads of the same file
	if strings.Contains(stepType, "write") || strings.Contains(stepType, "edit") {
		for i, prev := range previousSteps {
			prevType := strings.ToLower(prev.Type)
			if strings.Contains(prevType, "read") && p.sameTarget(step.Target, prev.Target) {
				deps = append(deps, previousIDs[i])
			}
		}
	}

	// Test operations depend on writes
	if strings.Contains(stepType, "test") {
		for i, prev := range previousSteps {
			prevType := strings.ToLower(prev.Type)
			if strings.Contains(prevType, "write") || strings.Contains(prevType, "edit") {
				deps = append(deps, previousIDs[i])
			}
		}
	}

	// Build operations depend on all writes
	if strings.Contains(stepType, "build") || strings.Contains(stepType, "compile") {
		for i, prev := range previousSteps {
			prevType := strings.ToLower(prev.Type)
			if strings.Contains(prevType, "write") || strings.Contains(prevType, "edit") {
				deps = append(deps, previousIDs[i])
			}
		}
	}

	return deps
}

// sameTarget checks if two steps target the same file or resource.
func (p *Planner) sameTarget(target1, target2 string) bool {
	target1 = strings.TrimSpace(target1)
	target2 = strings.TrimSpace(target2)
	return target1 != "" && target1 == target2
}

// calculatePriority assigns priority based on step type.
func calculatePriority(step PlanStep) int {
	stepType := strings.ToLower(step.Type)

	switch {
	case strings.Contains(stepType, "read"):
		return 100 // Read operations first
	case strings.Contains(stepType, "write") || strings.Contains(stepType, "edit"):
		return 50 // Write operations second
	case strings.Contains(stepType, "test"):
		return 30 // Test operations third
	case strings.Contains(stepType, "build"):
		return 20 // Build operations fourth
	default:
		return 10 // Default low priority
	}
}

// ExecutePlan executes all steps respecting dependencies.
func (p *Planner) ExecutePlan(ctx context.Context, executor func(step PlanStep) error) error {
	order, err := p.GetExecutionOrder()
	if err != nil {
		return fmt.Errorf("failed to get execution order: %w", err)
	}

	p.logger.Info("Executing plan", "steps", len(order))

	for _, stepID := range order {
		node := p.graph.nodes[stepID]

		// Check if dependencies are completed
		allDepsComplete := true
		for _, depID := range node.Dependencies {
			if p.graph.nodes[depID].Status != StatusCompleted {
				allDepsComplete = false
				break
			}
		}

		if !allDepsComplete {
			p.logger.Warn("Skipping step due to failed dependencies", "step", stepID)
			node.Status = StatusBlocked
			continue
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute step
		node.Status = StatusRunning
		p.logger.Info("Executing step", "id", stepID, "type", node.Step.Type)

		err := executor(node.Step)
		if err != nil {
			p.logger.Error("Step failed", "id", stepID, "error", err)
			node.Status = StatusFailed
			p.markDependentsBlocked(stepID)
			return fmt.Errorf("step %d failed: %w", stepID, err)
		}

		node.Status = StatusCompleted
		p.logger.Info("Step completed", "id", stepID)
	}

	return nil
}

// GetStats returns statistics about the plan.
func (p *Planner) GetStats() PlanStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PlanStats{
		Total: len(p.graph.nodes),
	}

	for _, node := range p.graph.nodes {
		switch node.Status {
		case StatusCompleted:
			stats.Completed++
		case StatusFailed:
			stats.Failed++
		case StatusBlocked:
			stats.Blocked++
		case StatusRunning:
			stats.Running++
		case StatusPending, StatusReady:
			stats.Pending++
		}
	}

	return stats
}

// PlanStats contains plan execution statistics.
type PlanStats struct {
	Total     int
	Completed int
	Failed    int
	Blocked   int
	Running   int
	Pending   int
}
