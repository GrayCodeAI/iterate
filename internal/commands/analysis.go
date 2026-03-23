package commands

import (
	"fmt"
	"strings"
)

// SymbolLocation represents where a symbol is defined in Go code.
// Used by code intelligence features for go-to-definition.
type SymbolLocation struct {
	File      string // file path
	Line      int    // line number
	Column    int    // column number
	Kind      string // "func", "type", "method", "var", "const"
	Signature string // function signature or type definition
}

// String returns a human-readable location string.
func (loc SymbolLocation) String() string {
	return fmt.Sprintf("%s:%d:%d", loc.File, loc.Line, loc.Column)
}

// FormatContext returns the location formatted for inclusion in prompts.
func (loc SymbolLocation) FormatContext() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("File: %s\n", loc.File))
	b.WriteString(fmt.Sprintf("Line: %d, Column: %d\n", loc.Line, loc.Column))
	if loc.Kind != "" {
		b.WriteString(fmt.Sprintf("Kind: %s\n", loc.Kind))
	}
	if loc.Signature != "" {
		b.WriteString(fmt.Sprintf("Signature:\n```go\n%s\n```\n", loc.Signature))
	}
	return b.String()
}

// BuildGoDefPrompt builds a prompt asking the LLM to explain a found definition.
func BuildGoDefPrompt(symbol string, loc SymbolLocation) string {
	prompt := fmt.Sprintf(
		"Explain the following Go %s definition:\n\n"+
			"Symbol: %s\n"+
			"Location: %s\n",
		loc.Kind, symbol, loc.String())

	if loc.Signature != "" {
		prompt += fmt.Sprintf("\nSignature:\n```go\n%s\n```\n", loc.Signature)
	}

	prompt += "\nPlease explain:\n" +
		"1. What this symbol does and its purpose\n" +
		"2. The parameters and return values (if applicable)\n" +
		"3. How it fits into the overall codebase\n" +
		"4. Any important implementation details or patterns used"

	return prompt
}

// BuildCodeIntelligenceContext builds context about code intelligence capabilities.
// This can be included in the system prompt for /ask mode to enable smarter responses.
func BuildCodeIntelligenceContext() string {
	return "Code intelligence features available:\n" +
		"- Symbol resolution across the entire codebase\n" +
		"- Support for functions, types, methods, variables, and constants\n" +
		"- Go AST-based analysis for accurate navigation\n"
}

// FormatSymbolSearchResults formats multiple symbol locations for display.
func FormatSymbolSearchResults(symbol string, locations []SymbolLocation) string {
	if len(locations) == 0 {
		return fmt.Sprintf("No definitions found for '%s'", symbol)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d definition(s) for '%s':\n\n", len(locations), symbol))

	for i, loc := range locations {
		b.WriteString(fmt.Sprintf("%d. %s (%s) at %s\n", i+1, symbol, loc.Kind, loc.String()))
		if loc.Signature != "" {
			// Truncate long signatures
			sig := loc.Signature
			if len(sig) > 100 {
				sig = sig[:97] + "..."
			}
			b.WriteString(fmt.Sprintf("   %s\n", sig))
		}
	}

	return b.String()
}

// RegisterAnalysisCommands adds repository analysis commands.
func RegisterAnalysisCommands(r *Registry) {
	r.Register(Command{
		Name:        "/count-lines",
		Aliases:     []string{},
		Description: "count lines of code by language",
		Category:    "analysis",
		Handler:     cmdCountLines,
	})

	r.Register(Command{
		Name:        "/hotspots",
		Aliases:     []string{},
		Description: "most changed files in git",
		Category:    "analysis",
		Handler:     cmdHotspots,
	})

	r.Register(Command{
		Name:        "/contributors",
		Aliases:     []string{},
		Description: "show git contributors",
		Category:    "analysis",
		Handler:     cmdContributors,
	})

	r.Register(Command{
		Name:        "/languages",
		Aliases:     []string{},
		Description: "language breakdown",
		Category:    "analysis",
		Handler:     cmdLanguages,
	})
}

func cmdCountLines(ctx Context) Result {
	fmt.Printf("%sCounting lines…%s\n", ColorDim, ColorReset)
	prompt := "Run a line count analysis on this repository. For each programming language found, " +
		"count the number of files and lines of code. Present results in a table format " +
		"with columns: Language, Files, Lines. Include a total row at the bottom."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdHotspots(ctx Context) Result {
	n := 15
	if ctx.HasArg(1) {
		fmt.Sscanf(ctx.Arg(1), "%d", &n)
	}
	prompt := fmt.Sprintf("Analyze git log to find the %d most frequently changed files in this repository. "+
		"Use 'git log --pretty=format: --name-only' and count occurrences. "+
		"Present results as a ranked table with columns: Rank, File, Changes.", n)
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdContributors(ctx Context) Result {
	prompt := "Analyze git contributors in this repository. Use 'git shortlog -sne HEAD' to get " +
		"commit counts per author. Present results as a ranked table with columns: Rank, Author, Commits. " +
		"Sort by commit count descending."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}

func cmdLanguages(ctx Context) Result {
	prompt := "Analyze the programming languages used in this repository. For each language, " +
		"count files and lines. Present as a table sorted by lines descending. " +
		"Also calculate percentage of total for each language."
	if ctx.REPL.StreamAndPrint != nil {
		ctx.REPL.StreamAndPrint(nil, ctx.Agent, prompt, ctx.RepoPath)
	}
	return Result{Handled: true}
}
