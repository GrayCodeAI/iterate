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

// listGoFiles finds Go files in the repo
func listGoFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Skip test files
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
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

// CompleteWord tries to complete the current word
func CompleteWord(line string, cursorPos int) (string, int) {
	if cursorPos <= 0 || cursorPos > len(line) {
		return "", 0
	}

	// Find word boundaries
	start := cursorPos - 1
	for start >= 0 && isWordChar(rune(line[start])) {
		start--
	}
	start++

	end := cursorPos
	for end < len(line) && isWordChar(rune(line[end])) {
		end++
	}

	word := line[start:end]
	return word, start
}

// isWordChar checks if a rune is part of a word
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// CommonGoSnippets provides Go-specific code snippets
var CommonGoSnippets = map[string]Suggestion{
	"func": {
		Text:        "func name(params) returnType {\n\t// TODO: implement\n\treturn\n}",
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
