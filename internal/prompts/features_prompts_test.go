package prompts

import (
	"strings"
	"testing"
)

func TestBuildEditPrompt_ContainsInstruction(t *testing.T) {
	content := "package main\n\nfunc main() {}"
	instruction := "Add a comment"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, instruction) {
		t.Errorf("expected result to contain instruction %q, got %q", instruction, result)
	}
}

func TestBuildEditPrompt_ContainsFileContent(t *testing.T) {
	content := "package main\n\nfunc main() {}"
	instruction := "Add a comment"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, content) {
		t.Errorf("expected result to contain file content, got %q", result)
	}
}

func TestBuildEditPrompt_ContainsConstraints(t *testing.T) {
	content := "package main"
	instruction := "fix"
	result := buildEditPrompt(content, instruction)

	constraints := []string{
		"Preserve original formatting",
		"minimal changes",
		"COMPLETE file content",
		"Do not add explanations",
		"syntactically valid",
	}

	for _, constraint := range constraints {
		if !strings.Contains(result, constraint) {
			t.Errorf("expected result to contain constraint %q, got %q", constraint, result)
		}
	}
}

func TestBuildEditPrompt_MarkedCodeBlock(t *testing.T) {
	content := "package main"
	instruction := "edit"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, "```") {
		t.Errorf("expected result to contain code block markers, got %q", result)
	}
}

func TestBuildEditPrompt_EmptyContent(t *testing.T) {
	content := ""
	instruction := "create a new file"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, instruction) {
		t.Errorf("expected result to contain instruction even with empty content, got %q", result)
	}
}

func TestBuildEditPrompt_EmptyInstruction(t *testing.T) {
	content := "package main"
	instruction := ""
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, content) {
		t.Errorf("expected result to contain content even with empty instruction, got %q", result)
	}
}

func TestBuildEditPrompt_MultilineContent(t *testing.T) {
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"hello\")\n}"
	instruction := "Add error handling"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, "import \"fmt\"") {
		t.Errorf("expected result to preserve multiline content, got %q", result)
	}
}

func TestBuildEditPrompt_MultilineInstruction(t *testing.T) {
	content := "package main"
	instruction := "Line 1\nLine 2\nLine 3"
	result := buildEditPrompt(content, instruction)

	if !strings.Contains(result, "Line 2") {
		t.Errorf("expected result to preserve multiline instruction, got %q", result)
	}
}

// Test exported version
func TestBuildEditPrompt_Exported(t *testing.T) {
	content := "package test"
	instruction := "test instruction"
	result := BuildEditPrompt(content, instruction)

	if !strings.Contains(result, content) {
		t.Errorf("expected exported BuildEditPrompt to contain content, got %q", result)
	}
	if !strings.Contains(result, instruction) {
		t.Errorf("expected exported BuildEditPrompt to contain instruction, got %q", result)
	}
}
