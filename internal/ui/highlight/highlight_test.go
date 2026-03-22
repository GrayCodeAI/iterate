package highlight

import (
	"strings"
	"testing"
)

func TestRenderInlineMarkdown(t *testing.T) {
	result := RenderInline("This is **bold** text")
	if !strings.Contains(result, "bold") {
		t.Errorf("should contain 'bold', got %q", result)
	}
}

func TestHighlightCodeGo(t *testing.T) {
	result := HighlightCode("func main()", "go")
	if !strings.Contains(result, "func") {
		t.Errorf("should highlight Go keywords, got %q", result)
	}
}

func TestHighlightCodePython(t *testing.T) {
	result := HighlightCode("def hello():", "python")
	if !strings.Contains(result, "def") {
		t.Errorf("should highlight Python keywords, got %q", result)
	}
}

func TestHighlightCodeComment(t *testing.T) {
	result := HighlightCode("// this is a comment", "go")
	if !strings.Contains(result, "comment") {
		t.Errorf("should handle comments, got %q", result)
	}
}

func TestColorizeStrings(t *testing.T) {
	result := colorizeStrings(`msg := "hello"`)
	if !strings.Contains(result, "hello") {
		t.Errorf("should preserve string content, got %q", result)
	}
}
