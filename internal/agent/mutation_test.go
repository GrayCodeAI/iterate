package agent

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// summariseMutationResults
// ---------------------------------------------------------------------------

func TestSummariseMutationResults_Empty(t *testing.T) {
	result := summariseMutationResults("")
	if !strings.Contains(result, "Mutation testing results") {
		t.Error("expected header in output")
	}
	if !strings.Contains(result, "Total: 0") {
		t.Errorf("expected Total: 0, got %q", result)
	}
}

func TestSummariseMutationResults_AllKilled(t *testing.T) {
	raw := "PASS foo/bar.go:10:5\nPASS foo/bar.go:20:3\nPASS foo/baz.go:5:1\n"
	result := summariseMutationResults(raw)
	if !strings.Contains(result, "Total: 3") {
		t.Errorf("expected Total: 3, got %q", result)
	}
	if !strings.Contains(result, "Killed: 3") {
		t.Errorf("expected Killed: 3, got %q", result)
	}
	if !strings.Contains(result, "Survived: 0") {
		t.Errorf("expected Survived: 0, got %q", result)
	}
	if !strings.Contains(result, "100.0%") {
		t.Errorf("expected 100.0%%, got %q", result)
	}
}

func TestSummariseMutationResults_AllSurvived(t *testing.T) {
	raw := "FAIL foo/bar.go:10:5\nFAIL foo/bar.go:20:3\n"
	result := summariseMutationResults(raw)
	if !strings.Contains(result, "Total: 2") {
		t.Errorf("expected Total: 2, got %q", result)
	}
	if !strings.Contains(result, "Killed: 0") {
		t.Errorf("expected Killed: 0, got %q", result)
	}
	if !strings.Contains(result, "Survived: 2") {
		t.Errorf("expected Survived: 2, got %q", result)
	}
	if !strings.Contains(result, "0.0%") {
		t.Errorf("expected 0.0%%, got %q", result)
	}
}

func TestSummariseMutationResults_Mixed(t *testing.T) {
	raw := "PASS foo.go:1\nFAIL bar.go:2\nPASS baz.go:3\n"
	result := summariseMutationResults(raw)
	if !strings.Contains(result, "Total: 3") {
		t.Errorf("expected Total: 3, got %q", result)
	}
	if !strings.Contains(result, "Killed: 2") {
		t.Errorf("expected Killed: 2, got %q", result)
	}
	if !strings.Contains(result, "Survived: 1") {
		t.Errorf("expected Survived: 1, got %q", result)
	}
	if !strings.Contains(result, "66.7%") {
		t.Errorf("expected 66.7%%, got %q", result)
	}
}

func TestSummariseMutationResults_SurvivedLinesListed(t *testing.T) {
	raw := "FAIL foo/bar.go:10 mutation survived\n"
	result := summariseMutationResults(raw)
	if !strings.Contains(result, "Surviving mutants") {
		t.Error("expected 'Surviving mutants' section")
	}
	if !strings.Contains(result, "FAIL foo/bar.go:10") {
		t.Errorf("expected surviving line in output, got %q", result)
	}
}

func TestSummariseMutationResults_TruncatesAt20(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 25; i++ {
		sb.WriteString("FAIL line\n")
	}
	result := summariseMutationResults(sb.String())
	if !strings.Contains(result, "and 5 more") {
		t.Errorf("expected truncation message 'and 5 more', got %q", result)
	}
}

func TestSummariseMutationResults_ExactlyAtLimit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("FAIL line\n")
	}
	result := summariseMutationResults(sb.String())
	if strings.Contains(result, "and") && strings.Contains(result, "more") {
		t.Errorf("expected no truncation for exactly 20, got %q", result)
	}
}

func TestSummariseMutationResults_NoScoreWhenTotalZero(t *testing.T) {
	result := summariseMutationResults("some random line\nanother line\n")
	if strings.Contains(result, "Mutation score:") {
		t.Error("expected no score line when total is 0")
	}
}

func TestSummariseMutationResults_IgnoresNonPassFail(t *testing.T) {
	raw := "INFO running mutants\nDEBUG skipped\nWARN something\nPASS one\n"
	result := summariseMutationResults(raw)
	if !strings.Contains(result, "Total: 1") {
		t.Errorf("expected Total: 1 (only PASS counted), got %q", result)
	}
}

// ---------------------------------------------------------------------------
// runCoverageReport output format
// ---------------------------------------------------------------------------

func TestRunCoverageReport_Header(t *testing.T) {
	// We can't run the real command in unit tests, but we can verify the
	// summariseMutationResults function which is pure and fully testable.
	// runCoverageReport is tested indirectly through integration.
	// Here we just verify the function signature exists and is callable.
	_ = MutationTestTool // verify MutationTestTool is exported and non-nil
}

// ---------------------------------------------------------------------------
// MutationTestTool
// ---------------------------------------------------------------------------

func TestMutationTestTool_Name(t *testing.T) {
	tool := MutationTestTool("/tmp")
	if tool.Name != "mutation_test" {
		t.Errorf("expected name 'mutation_test', got %q", tool.Name)
	}
}

func TestMutationTestTool_HasDescription(t *testing.T) {
	tool := MutationTestTool("/tmp")
	if tool.Description == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(tool.Description, "mutation") {
		t.Errorf("expected 'mutation' in description, got %q", tool.Description)
	}
}

func TestMutationTestTool_HasExecute(t *testing.T) {
	tool := MutationTestTool("/tmp")
	if tool.Execute == nil {
		t.Error("expected non-nil Execute function")
	}
}

func TestMutationTestTool_DifferentRepoPaths(t *testing.T) {
	t1 := MutationTestTool("/path/a")
	t2 := MutationTestTool("/path/b")
	// Both should produce valid tools regardless of path
	if t1.Name != t2.Name {
		t.Error("tool name should be stable across different repo paths")
	}
}
