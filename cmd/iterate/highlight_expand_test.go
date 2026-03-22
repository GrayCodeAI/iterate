package main

import (
	"strings"
	"testing"
)

func TestHighlightCode_Bash(t *testing.T) {
	result := highlightCode("echo hello", "bash")
	if !strings.Contains(result, "echo") {
		t.Errorf("should highlight bash keyword, got %q", result)
	}
}

func TestHighlightCode_BashShell(t *testing.T) {
	result := highlightCode("ls -la", "sh")
	if !strings.Contains(result, "ls") {
		t.Errorf("should highlight sh keyword, got %q", result)
	}
}

func TestHighlightCode_BashComment(t *testing.T) {
	result := highlightCode("# this is a comment", "bash")
	if !strings.Contains(result, "comment") {
		t.Errorf("should handle bash comments, got %q", result)
	}
}

func TestHighlightCode_JSON(t *testing.T) {
	result := highlightCode(`"key": "value"`, "json")
	if !strings.Contains(result, "key") {
		t.Errorf("should highlight JSON, got %q", result)
	}
}

func TestHighlightCode_UnknownLang(t *testing.T) {
	result := highlightCode("some code", "ruby")
	if !strings.Contains(result, "some code") {
		t.Errorf("should preserve content for unknown language, got %q", result)
	}
}

func TestHighlightCode_Python(t *testing.T) {
	result := highlightCode("class MyClass:", "python")
	if !strings.Contains(result, "class") {
		t.Errorf("should highlight python keyword, got %q", result)
	}
}

func TestHighlightCode_PythonComment(t *testing.T) {
	result := highlightCode("# comment", "py")
	if !strings.Contains(result, "comment") {
		t.Errorf("should handle python comments, got %q", result)
	}
}

func TestHighlightCode_GoMultipleKeywords(t *testing.T) {
	result := highlightCode("func main() error {", "go")
	if !strings.Contains(result, "func") {
		t.Errorf("should highlight func, got %q", result)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("should highlight error, got %q", result)
	}
}

func TestHighlightCode_GoString(t *testing.T) {
	result := highlightCode(`msg := "hello world"`, "go")
	if !strings.Contains(result, "hello world") {
		t.Errorf("should preserve string content, got %q", result)
	}
}

func TestColorize_NoMatch(t *testing.T) {
	result := colorize("plain text here", []string{"func", "return"}, "\033[34m")
	if !strings.Contains(result, "plain text here") {
		t.Errorf("should preserve text when no keywords match, got %q", result)
	}
}

func TestColorize_Match(t *testing.T) {
	result := colorize("func main", []string{"func"}, "\033[34m")
	if !strings.Contains(result, "func") {
		t.Errorf("should contain matched keyword, got %q", result)
	}
}

func TestColorize_MultipleKeywords(t *testing.T) {
	result := colorize("func return if", []string{"func", "return", "if"}, "\033[34m")
	if !strings.Contains(result, "func") || !strings.Contains(result, "return") || !strings.Contains(result, "if") {
		t.Errorf("should colorize all keywords, got %q", result)
	}
}

func TestColorize_PunctuationWrapped(t *testing.T) {
	result := colorize("func(", []string{"func"}, "\033[34m")
	if !strings.Contains(result, "func") {
		t.Errorf("should match keyword even with punctuation, got %q", result)
	}
}

func TestColorizeStrings_NoStrings(t *testing.T) {
	result := colorizeStrings("plain text")
	if !strings.Contains(result, "plain text") {
		t.Errorf("should preserve text without strings, got %q", result)
	}
}

func TestColorizeStrings_SingleQuote(t *testing.T) {
	result := colorizeStrings(`x := 'a'`)
	if !strings.Contains(result, "a") {
		t.Errorf("should handle single quotes, got %q", result)
	}
}

func TestColorizeStrings_EmptyString(t *testing.T) {
	result := colorizeStrings(`msg := ""`)
	if !strings.Contains(result, `""`) {
		t.Errorf("should handle empty strings, got %q", result)
	}
}

func TestReplacePairs(t *testing.T) {
	result := replacePairs("hello **bold** world", "**", "[", "]")
	if !strings.Contains(result, "[bold]") {
		t.Errorf("should replace pairs, got %q", result)
	}
}

func TestReplacePairs_NoMatch(t *testing.T) {
	result := replacePairs("hello world", "**", "[", "]")
	if result != "hello world" {
		t.Errorf("should not change when no delimiter found, got %q", result)
	}
}

func TestReplacePairs_OddParts(t *testing.T) {
	result := replacePairs("**a** b **c**", "**", "[", "]")
	if !strings.Contains(result, "[a]") {
		t.Errorf("should wrap first bold, got %q", result)
	}
	if !strings.Contains(result, "[c]") {
		t.Errorf("should wrap second bold, got %q", result)
	}
}

func TestRenderInline_Italic(t *testing.T) {
	result := renderInline("This is *italic* text")
	if !strings.Contains(result, "italic") {
		t.Errorf("should contain italic text, got %q", result)
	}
}

func TestRenderInline_Code(t *testing.T) {
	result := renderInline("Use `fmt.Println` here")
	if !strings.Contains(result, "fmt.Println") {
		t.Errorf("should contain inline code, got %q", result)
	}
}

func TestRenderInline_Empty(t *testing.T) {
	result := renderInline("")
	if result == "" {
		t.Error("should return non-empty result even for empty input")
	}
}

func TestRenderInline_Plain(t *testing.T) {
	result := renderInline("just plain text")
	if !strings.Contains(result, "just plain text") {
		t.Errorf("should preserve plain text, got %q", result)
	}
}
