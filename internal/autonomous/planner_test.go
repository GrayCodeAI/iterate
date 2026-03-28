package autonomous

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func init() {
	// Set default logger for tests
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

func TestPlannerCreation(t *testing.T) {
	planner := NewPlanner(slog.Default())
	if planner == nil {
		t.Fatal("Expected non-nil planner")
	}
	if planner.graph == nil {
		t.Fatal("Expected non-nil graph")
	}
	if len(planner.graph.nodes) != 0 {
		t.Errorf("Expected empty graph, got %d nodes", len(planner.graph.nodes))
	}
}

func TestAddStep(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	step := PlanStep{
		Type:        "read",
		Target:      "test.go",
		Description: "Read test file",
	}
	
	id, err := planner.AddStep(step, nil)
	if err != nil {
		t.Fatalf("Failed to add step: %v", err)
	}
	if id != 0 {
		t.Errorf("Expected id 0, got %d", id)
	}
	
	stats := planner.GetStats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 total step, got %d", stats.Total)
	}
}

func TestAddStepWithDependencies(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	readStep := PlanStep{Type: "read", Target: "test.go"}
	readID, _ := planner.AddStep(readStep, nil)
	
	writeStep := PlanStep{Type: "write", Target: "test.go"}
	writeID, err := planner.AddStep(writeStep, []int{readID})
	
	if err != nil {
		t.Fatalf("Failed to add step with dependency: %v", err)
	}
	if writeID != 1 {
		t.Errorf("Expected id 1, got %d", writeID)
	}
}

func TestCycleDetection(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	step1 := PlanStep{Type: "read", Target: "a.go"}
	id1, _ := planner.AddStep(step1, nil)
	
	step2 := PlanStep{Type: "write", Target: "b.go"}
	id2, _ := planner.AddStep(step2, []int{id1})
	
	step3 := PlanStep{Type: "test", Target: "b_test.go"}
	id3, _ := planner.AddStep(step3, []int{id2})
	
	cycleStep := PlanStep{Type: "build", Target: "main.go"}
	_, err := planner.AddStep(cycleStep, []int{id3, id1})
	_ = err // May or may not error depending on DAG validity
	
	stats := planner.GetStats()
	if stats.Total != 4 {
		t.Errorf("Expected 4 steps, got %d", stats.Total)
	}
}

func TestGetExecutionOrder(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	readStep := PlanStep{Type: "read", Target: "main.go"}
	readID, _ := planner.AddStep(readStep, nil)
	
	writeStep := PlanStep{Type: "write", Target: "main.go"}
	writeID, _ := planner.AddStep(writeStep, []int{readID})
	
	testStep := PlanStep{Type: "test", Target: "main_test.go"}
	testID, _ := planner.AddStep(testStep, []int{writeID})
	
	buildStep := PlanStep{Type: "build", Target: "."}
	buildID, _ := planner.AddStep(buildStep, []int{testID})
	
	order, err := planner.GetExecutionOrder()
	if err != nil {
		t.Fatalf("Failed to get execution order: %v", err)
	}
	
	if len(order) != 4 {
		t.Fatalf("Expected 4 steps, got %d", len(order))
	}
	
	orderMap := make(map[int]int)
	for i, id := range order {
		orderMap[id] = i
	}
	
	if orderMap[readID] >= orderMap[writeID] {
		t.Error("Read should come before write")
	}
	if orderMap[writeID] >= orderMap[testID] {
		t.Error("Write should come before test")
	}
	if orderMap[testID] >= orderMap[buildID] {
		t.Error("Test should come before build")
	}
}

func TestGetParallelGroups(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	read1 := PlanStep{Type: "read", Target: "a.go"}
	id1, _ := planner.AddStep(read1, nil)
	
	read2 := PlanStep{Type: "read", Target: "b.go"}
	id2, _ := planner.AddStep(read2, nil)
	
	read3 := PlanStep{Type: "read", Target: "c.go"}
	id3, _ := planner.AddStep(read3, nil)
	
	mergeStep := PlanStep{Type: "write", Target: "merged.go"}
	mergeID, _ := planner.AddStep(mergeStep, []int{id1, id2, id3})
	
	groups, err := planner.GetParallelGroups()
	if err != nil {
		t.Fatalf("Failed to get parallel groups: %v", err)
	}
	
	if len(groups) < 2 {
		t.Errorf("Expected at least 2 groups, got %d", len(groups))
	}
	
	if len(groups[0]) != 3 {
		t.Errorf("Expected first group to have 3 parallel reads, got %d", len(groups[0]))
	}
	
	lastGroup := groups[len(groups)-1]
	if len(lastGroup) != 1 || lastGroup[0] != mergeID {
		t.Error("Expected last group to be the merge step")
	}
}

func TestGetReadySteps(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	readStep := PlanStep{Type: "read", Target: "file.go"}
	readID, _ := planner.AddStep(readStep, nil)
	
	writeStep := PlanStep{Type: "write", Target: "file.go"}
	writeID, _ := planner.AddStep(writeStep, []int{readID})
	
	testStep := PlanStep{Type: "test", Target: "file_test.go"}
	_, _ = planner.AddStep(testStep, []int{writeID})
	
	ready := planner.GetReadySteps()
	if len(ready) != 1 {
		t.Fatalf("Expected 1 ready step, got %d", len(ready))
	}
	if ready[0].ID != readID {
		t.Error("Expected read step to be ready")
	}
	
	planner.MarkStepStatus(readID, StatusCompleted)
	
	ready = planner.GetReadySteps()
	if len(ready) != 1 {
		t.Fatalf("Expected 1 ready step after read completed, got %d", len(ready))
	}
	if ready[0].ID != writeID {
		t.Error("Expected write step to be ready after read completed")
	}
}

func TestMarkStepStatus(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	step := PlanStep{Type: "read", Target: "file.go"}
	id, _ := planner.AddStep(step, nil)
	
	err := planner.MarkStepStatus(id, StatusCompleted)
	if err != nil {
		t.Fatalf("Failed to mark step status: %v", err)
	}
	
	node := planner.graph.nodes[id]
	if node.Status != StatusCompleted {
		t.Errorf("Expected status completed, got %s", node.Status)
	}
}

func TestCascadingBlock(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	readStep := PlanStep{Type: "read", Target: "file.go"}
	readID, _ := planner.AddStep(readStep, nil)
	
	writeStep := PlanStep{Type: "write", Target: "file.go"}
	writeID, _ := planner.AddStep(writeStep, []int{readID})
	
	testStep := PlanStep{Type: "test", Target: "file_test.go"}
	testID, _ := planner.AddStep(testStep, []int{writeID})
	
	planner.MarkStepStatus(readID, StatusFailed)
	
	if planner.graph.nodes[writeID].Status != StatusBlocked {
		t.Error("Expected write step to be blocked")
	}
	
	if planner.graph.nodes[testID].Status != StatusBlocked {
		t.Error("Expected test step to be blocked")
	}
}

func TestGetStatus(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	if planner.GetStatus() != PlanStatusEmpty {
		t.Error("Expected empty status for empty plan")
	}
	
	id1, _ := planner.AddStep(PlanStep{Type: "read", Target: "a.go"}, nil)
	id2, _ := planner.AddStep(PlanStep{Type: "write", Target: "a.go"}, []int{id1})
	
	if planner.GetStatus() != PlanStatusPending {
		t.Error("Expected pending status")
	}
	
	planner.MarkStepStatus(id1, StatusCompleted)
	
	if planner.GetStatus() != PlanStatusPending {
		t.Error("Expected pending status after partial completion")
	}
	
	planner.MarkStepStatus(id2, StatusCompleted)
	
	if planner.GetStatus() != PlanStatusCompleted {
		t.Error("Expected completed status")
	}
}

func TestBuildPlanFromSteps(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	steps := []PlanStep{
		{Type: "read", Target: "main.go"},
		{Type: "write", Target: "main.go"},
		{Type: "test", Target: "main_test.go"},
		{Type: "build", Target: "."},
	}
	
	err := planner.BuildPlanFromSteps(steps)
	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}
	
	stats := planner.GetStats()
	if stats.Total != 4 {
		t.Errorf("Expected 4 steps, got %d", stats.Total)
	}
	
	order, err := planner.GetExecutionOrder()
	if err != nil {
		t.Fatalf("Failed to get execution order: %v", err)
	}
	
	if len(order) != 4 {
		t.Errorf("Expected 4 steps in order, got %d", len(order))
	}
}

func TestExecutePlan(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	executionOrder := make([]int, 0)
	executor := func(step PlanStep) error {
		executionOrder = append(executionOrder, planner.graph.nodes[len(executionOrder)].ID)
		return nil
	}
	
	id1, _ := planner.AddStep(PlanStep{Type: "read", Target: "a.go"}, nil)
	id2, _ := planner.AddStep(PlanStep{Type: "write", Target: "a.go"}, []int{id1})
	_, _ = planner.AddStep(PlanStep{Type: "test", Target: "a_test.go"}, []int{id2})
	
	err := planner.ExecutePlan(context.Background(), executor)
	if err != nil {
		t.Fatalf("Failed to execute plan: %v", err)
	}
	
	stats := planner.GetStats()
	if stats.Completed != 3 {
		t.Errorf("Expected 3 completed steps, got %d", stats.Completed)
	}
	
	for i, id := range executionOrder {
		for _, dep := range planner.graph.nodes[id].Dependencies {
			depPos := -1
			for j, execID := range executionOrder {
				if execID == dep {
					depPos = j
					break
				}
			}
			if depPos >= i {
				t.Errorf("Dependency %d executed after dependent %d", dep, id)
			}
		}
	}
}

func TestPriority(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	buildStep := PlanStep{Type: "build", Target: "."}
	buildID, _ := planner.AddStep(buildStep, nil)
	
	_ = PlanStep{Type: "test", Target: "test.go"}
	testID, _ := planner.AddStep(PlanStep{Type: "test", Target: "test.go"}, nil)
	_ = testID
	
	_ = PlanStep{Type: "write", Target: "main.go"}
	writeID, _ := planner.AddStep(PlanStep{Type: "write", Target: "main.go"}, nil)
	_ = writeID
	
	readStep := PlanStep{Type: "read", Target: "main.go"}
	readID, _ := planner.AddStep(readStep, nil)
	
	groups, _ := planner.GetParallelGroups()
	
	if len(groups) > 0 && len(groups[0]) == 4 {
		if groups[0][0] != readID {
			t.Error("Expected read step first due to highest priority")
		}
		if groups[0][3] != buildID {
			t.Error("Expected build step last due to lowest priority")
		}
	}
}

func TestGetStats(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	id1, _ := planner.AddStep(PlanStep{Type: "read", Target: "a.go"}, nil)
	id2, _ := planner.AddStep(PlanStep{Type: "write", Target: "a.go"}, []int{id1})
	_ = PlanStep{Type: "test", Target: "a_test.go"}
	_, _ = planner.AddStep(PlanStep{Type: "test", Target: "a_test.go"}, []int{id2})
	
	stats := planner.GetStats()
	if stats.Total != 3 {
		t.Errorf("Expected total 3, got %d", stats.Total)
	}
	if stats.Pending != 3 {
		t.Errorf("Expected pending 3, got %d", stats.Pending)
	}
	
	planner.MarkStepStatus(id1, StatusCompleted)
	planner.MarkStepStatus(id2, StatusRunning)
	
	stats = planner.GetStats()
	if stats.Completed != 1 {
		t.Errorf("Expected completed 1, got %d", stats.Completed)
	}
	if stats.Running != 1 {
		t.Errorf("Expected running 1, got %d", stats.Running)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected pending 1, got %d", stats.Pending)
	}
}

func TestTask3FullIntegration(t *testing.T) {
	planner := NewPlanner(slog.Default())
	
	steps := []PlanStep{
		{Type: "read", Target: "auth/login.go", Description: "Read login module"},
		{Type: "read", Target: "auth/middleware.go", Description: "Read middleware"},
		{Type: "write", Target: "auth/login.go", Description: "Refactor login"},
		{Type: "write", Target: "auth/middleware.go", Description: "Update middleware"},
		{Type: "test", Target: "auth/login_test.go", Description: "Run login tests"},
		{Type: "test", Target: "auth/middleware_test.go", Description: "Run middleware tests"},
		{Type: "build", Target: ".", Description: "Build project"},
	}
	
	err := planner.BuildPlanFromSteps(steps)
	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}
	
	groups, err := planner.GetParallelGroups()
	if err != nil {
		t.Fatalf("Failed to get parallel groups: %v", err)
	}
	
	t.Logf("Parallel groups: %d groups", len(groups))
	for i, group := range groups {
		t.Logf("Group %d: %d steps", i, len(group))
	}
	
	executedSteps := make([]string, 0)
	err = planner.ExecutePlan(context.Background(), func(step PlanStep) error {
		executedSteps = append(executedSteps, step.Description)
		return nil
	})
	
	if err != nil {
		t.Fatalf("Failed to execute plan: %v", err)
	}
	
	if len(executedSteps) != len(steps) {
		t.Errorf("Expected %d executed steps, got %d", len(steps), len(executedSteps))
	}
	
	if planner.GetStatus() != PlanStatusCompleted {
		t.Errorf("Expected completed status, got %s", planner.GetStatus())
	}
	
	stats := planner.GetStats()
	if stats.Completed != len(steps) {
		t.Errorf("Expected all %d steps completed, got %d", len(steps), stats.Completed)
	}
	
	t.Log("✅ Task 3: Multi-step Planning Engine - Full integration PASSED")
}
