// Package suggest provides inline code completions
package suggest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// Suggester provides intelligent code completions
type Suggester struct {
	provider iteragent.Provider
	tools    []iteragent.Tool
}

// NewSuggester creates a new code suggester
func NewSuggester(p iteragent.Provider, tools []iteragent.Tool) *Suggester {
	return &Suggester{
		provider: p,
		tools:    tools,
	}
}

// Context holds information about the current coding context
type Context struct {
	CurrentLine string
	Prefix      string
	FilePath    string
	Language    string
	RepoPath    string
}

// Suggestion represents a code completion suggestion
type Suggestion struct {
	Text        string
	Label       string
	Description string
	Kind        string // "function", "variable", "import", "snippet"
}

// GetSuggestions returns completions for the given context
func (s *Suggester) GetSuggestions(ctx context.Context, c Context) ([]Suggestion, error) {
	// Simple pattern matching for now
	suggestions := []Suggestion{}

	// Check for common patterns
	if strings.HasSuffix(c.Prefix, "func ") {
		suggestions = append(suggestions, Suggestion{
			Text:        "main() {\n\t\n}",
			Label:       "main function",
			Description: "Create a main function",
			Kind:        "snippet",
		})
	}

	if strings.HasSuffix(c.Prefix, "if ") {
		suggestions = append(suggestions, Suggestion{
			Text:        "err != nil {\n\treturn err\n}",
			Label:       "if err != nil",
			Description: "Error check pattern",
			Kind:        "snippet",
		})
	}

	// File-based suggestions
	if strings.HasSuffix(c.Prefix, "@") {
		files, _ := listGoFiles(c.RepoPath)
		for _, f := range files[:min(5, len(files))] {
			suggestions = append(suggestions, Suggestion{
				Text:        f,
				Label:       filepath.Base(f),
				Description: "Go file",
				Kind:        "file",
			})
		}
	}

	return suggestions, nil
}

// listGoFiles finds Go files in the repo (excluding test files)
func listGoFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			// Skip vendor and hidden dirs
			if strings.Contains(path, "/vendor/") || strings.Contains(path, "/.") {
				return nil
			}
			rel, _ := filepath.Rel(repoPath, path)
			files = append(files, rel)
		}
		return nil
	})

	return files, err
}

// GetCompletionsForQuery returns fuzzy-matched completions for a query string
func GetCompletionsForQuery(query string, repoPath string) ([]Suggestion, error) {
	suggester := NewSuggester(nil, nil)
	ctx := context.Background()

	// Detect trigger character
	triggerChar := ""
	if len(query) > 0 {
		lastChar := query[len(query)-1]
		if lastChar == '@' {
			triggerChar = "@"
		}
	}

	// Simple context detection
	prefix := query
	if triggerChar != "" {
		prefix = triggerChar
	}

	return suggester.GetSuggestions(ctx, Context{
		Prefix:   prefix,
		RepoPath: repoPath,
	})
}

// GetCompletionsForLine provides suggestions based on cursor position in a line
func GetCompletionsForLine(line string, col int, repoPath string) ([]Suggestion, error) {
	if col < 0 || col > len(line) {
		return nil, fmt.Errorf("invalid column position")
	}

	prefix := line[:col]
	ctx := Context{
		CurrentLine: line,
		Prefix:      prefix,
		RepoPath:    repoPath,
		Language:    "go",
	}

	suggester := NewSuggester(nil, nil)
	return suggester.GetSuggestions(context.Background(), ctx)
}

// AnalyzeContext examines the code around cursor to provide contextual suggestions
func AnalyzeContext(lines []string, lineNum, col int) (Context, error) {
	if lineNum < 0 || lineNum >= len(lines) {
		return Context{}, fmt.Errorf("invalid line number")
	}

	line := lines[lineNum]
	if col < 0 || col > len(line) {
		return Context{}, fmt.Errorf("invalid column position")
	}

	prefix := line[:col]

	// Detect context based on surrounding code
	ctx := Context{
		CurrentLine: line,
		Prefix:      prefix,
		Language:    "go",
	}

	// Check if we're inside a function
	inFunc := false
	braceCount := 0
	for i := 0; i <= lineNum && i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				braceCount++
				inFunc = true
			} else if ch == '}' {
				braceCount--
				if braceCount <= 0 {
					inFunc = false
				}
			}
		}
	}

	_ = inFunc // Will be used for context-aware suggestions

	return ctx, nil
}

// MatchSuggestion ranks how well a suggestion matches the current context
func MatchSuggestion(s Suggestion, ctx Context) float64 {
	score := 0.0

	// Exact prefix match gets highest score
	if strings.HasPrefix(strings.ToLower(s.Text), strings.ToLower(ctx.Prefix)) {
		score += 1.0
	}

	// Contains match
	if strings.Contains(strings.ToLower(s.Text), strings.ToLower(ctx.Prefix)) {
		score += 0.5
	}

	// Boost for certain kinds
	switch s.Kind {
	case "snippet":
		score += 0.3
	case "function":
		score += 0.2
	}

	return score
}

// IsValidIdentifier checks if text is a valid Go identifier
func IsValidIdentifier(text string) bool {
	if text == "" {
		return false
	}

	// First char must be letter or underscore
	runes := []rune(text)
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return false
	}

	// Rest can be letters, digits, or underscores
	for i := 1; i < len(runes); i++ {
		if !unicode.IsLetter(runes[i]) && !unicode.IsDigit(runes[i]) && runes[i] != '_' {
			return false
		}
	}

	return true
}

// CommonGoSnippets provides built-in Go code snippets
var CommonGoSnippets = map[string]Suggestion{
	"func": {
		Text:        "func Name() {\n\t\n}",
		Label:       "function",
		Description: "Function declaration",
		Kind:        "snippet",
	},
	"for": {
		Text:        "for i := 0; i < count; i++ {\n\t\n}",
		Label:       "for loop",
		Description: "For loop",
		Kind:        "snippet",
	},
	"forr": {
		Text:        "for i, v := range items {\n\t\n}",
		Label:       "for range",
		Description: "Range loop",
		Kind:        "snippet",
	},
	"if": {
		Text:        "if condition {\n\t\n}",
		Label:       "if statement",
		Description: "If condition",
		Kind:        "snippet",
	},
	"ifer": {
		Text:        "if err != nil {\n\treturn err\n}",
		Label:       "if err != nil",
		Description: "Error check",
		Kind:        "snippet",
	},
	"struct": {
		Text:        "type Name struct {\n\tField Type\n}",
		Label:       "struct",
		Description: "Struct definition",
		Kind:        "snippet",
	},
	"method": {
		Text:        "func (r *Receiver) MethodName() error {\n\treturn nil\n}",
		Label:       "method",
		Description: "Method receiver",
		Kind:        "snippet",
	},
	"test": {
		Text:        "func TestName(t *testing.T) {\n\t// TODO: test\n}",
		Label:       "test function",
		Description: "Test case",
		Kind:        "snippet",
	},
	"main": {
		Text:        "func main() {\n\t// TODO: implement\n}",
		Label:       "main",
		Description: "Main function",
		Kind:        "snippet",
	},
	"ctx": {
		Text:        "ctx context.Context",
		Label:       "context parameter",
		Description: "Context parameter",
		Kind:        "snippet",
	},
}

// GetSnippet returns a snippet by name
func GetSnippet(name string) (Suggestion, bool) {
	snippet, ok := CommonGoSnippets[name]
	return snippet, ok
}

// ShowSuggestions displays suggestions in the terminal
func ShowSuggestions(suggestions []Suggestion) {
	if len(suggestions) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("💡 Suggestions:")
	for i, s := range suggestions {
		if i >= 5 {
			fmt.Printf("   ... and %d more\n", len(suggestions)-5)
			break
		}
		kindIcon := "📝"
		switch s.Kind {
		case "function":
			kindIcon = "🔧"
		case "snippet":
			kindIcon = "📦"
		case "file":
			kindIcon = "📄"
		}
		fmt.Printf("   %s %s - %s\n", kindIcon, s.Label, s.Description)
	}
	fmt.Println()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
