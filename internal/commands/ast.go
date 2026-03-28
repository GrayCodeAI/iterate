package commands

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/GrayCodeAI/iterate/internal/astanalysis"
)

// RegisterASTCommands adds AST analysis commands
func RegisterASTCommands(r *Registry) {
	r.Register(Command{
		Name:        "/analyze",
		Aliases:     []string{"/ast"},
		Description: "analyze Go file structure using AST",
		Category:    "analysis",
		Handler:     cmdAnalyze,
	})

	r.Register(Command{
		Name:        "/outline",
		Description: "show file outline (functions, structs, interfaces)",
		Category:    "analysis",
		Handler:     cmdOutline,
	})

	r.Register(Command{
		Name:        "/imports",
		Description: "show file imports and dependencies",
		Category:    "analysis",
		Handler:     cmdASTImports,
	})

	r.Register(Command{
		Name:        "/unused",
		Description: "find potentially unused code",
		Category:    "analysis",
		Handler:     cmdUnused,
	})
}

func cmdAnalyze(ctx Context) Result {
	path := ctx.RepoPath
	if ctx.HasArg(1) {
		path = filepath.Join(ctx.RepoPath, ctx.Arg(1))
	}

	analyzer := astanalysis.NewAnalyzer(ctx.RepoPath)
	info, err := analyzer.AnalyzeFile(path)
	if err != nil {
		PrintError("Failed to analyze file: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("\n📊 Analysis: %s\n", filepath.Base(path))
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Package: %s\n", info.Package)
	fmt.Printf("Imports: %d\n", len(info.Imports))
	fmt.Printf("Functions: %d\n", len(info.Functions))
	fmt.Printf("Structs: %d\n", len(info.Structs))
	fmt.Printf("Interfaces: %d\n", len(info.Interfaces))
	fmt.Printf("Variables: %d\n", len(info.Variables))

	if len(info.Functions) > 0 {
		fmt.Println("\n🔧 Functions:")
		for _, fn := range info.Functions {
			receiver := ""
			if fn.Receiver != "" {
				receiver = fmt.Sprintf("(%s) ", fn.Receiver)
			}
			params := strings.Join(fn.Params, ", ")
			returns := ""
			if len(fn.Returns) > 0 {
				returns = fmt.Sprintf(" %s", strings.Join(fn.Returns, ", "))
				if len(fn.Returns) > 1 {
					returns = fmt.Sprintf(" (%s)", strings.Join(fn.Returns, ", "))
				}
			}
			fmt.Printf("  • %s%s(%s)%s  [line %d]\n", receiver, fn.Name, params, returns, fn.Line)
		}
	}

	if len(info.Structs) > 0 {
		fmt.Println("\n📦 Structs:")
		for _, s := range info.Structs {
			fmt.Printf("  • %s  [line %d, %d fields]\n", s.Name, s.Line, len(s.Fields))
		}
	}

	if len(info.Interfaces) > 0 {
		fmt.Println("\n🔌 Interfaces:")
		for _, i := range info.Interfaces {
			fmt.Printf("  • %s  [line %d, %d methods]\n", i.Name, i.Line, len(i.Methods))
		}
	}

	fmt.Println()
	return Result{Handled: true}
}

func cmdOutline(ctx Context) Result {
	path := ctx.RepoPath
	if ctx.HasArg(1) {
		path = filepath.Join(ctx.RepoPath, ctx.Arg(1))
	}

	analyzer := astanalysis.NewAnalyzer(ctx.RepoPath)
	info, err := analyzer.AnalyzeFile(path)
	if err != nil {
		PrintError("Failed to analyze file: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("\n📋 Outline: %s\n", filepath.Base(path))
	fmt.Println(strings.Repeat("─", 50))

	// Sort by line number
	type outlineItem struct {
		line int
		text string
	}
	var items []outlineItem

	for _, fn := range info.Functions {
		receiver := ""
		if fn.Receiver != "" {
			receiver = fmt.Sprintf("(%s) ", fn.Receiver)
		}
		items = append(items, outlineItem{fn.Line, fmt.Sprintf("🔧 %s%s()", receiver, fn.Name)})
	}

	for _, s := range info.Structs {
		items = append(items, outlineItem{s.Line, fmt.Sprintf("📦 type %s struct", s.Name)})
	}

	for _, i := range info.Interfaces {
		items = append(items, outlineItem{i.Line, fmt.Sprintf("🔌 type %s interface", i.Name)})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].line < items[j].line
	})

	for _, item := range items {
		fmt.Printf("  %s\n", item.text)
	}

	fmt.Println()
	return Result{Handled: true}
}

func cmdASTImports(ctx Context) Result {
	path := ctx.RepoPath
	if ctx.HasArg(1) {
		path = filepath.Join(ctx.RepoPath, ctx.Arg(1))
	}

	analyzer := astanalysis.NewAnalyzer(ctx.RepoPath)
	info, err := analyzer.AnalyzeFile(path)
	if err != nil {
		PrintError("Failed to analyze file: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("\n📦 Dependencies: %s\n", filepath.Base(path))
	fmt.Println(strings.Repeat("─", 50))

	// Group imports by category
	var stdlib, external []string
	for _, imp := range info.Imports {
		if !strings.Contains(imp, ".") {
			stdlib = append(stdlib, imp)
		} else {
			external = append(external, imp)
		}
	}

	if len(stdlib) > 0 {
		fmt.Println("\nStandard Library:")
		for _, imp := range stdlib {
			fmt.Printf("  • %s\n", imp)
		}
	}

	if len(external) > 0 {
		fmt.Println("\nExternal:")
		for _, imp := range external {
			fmt.Printf("  • %s\n", imp)
		}
	}

	fmt.Println()
	return Result{Handled: true}
}

func cmdUnused(ctx Context) Result {
	analyzer := astanalysis.NewAnalyzer(ctx.RepoPath)

	pkgPath := ctx.RepoPath
	if ctx.HasArg(1) {
		pkgPath = filepath.Join(ctx.RepoPath, ctx.Arg(1))
	}

	unused, err := analyzer.FindUnusedCode(pkgPath)
	if err != nil {
		PrintError("Failed to analyze: %v", err)
		return Result{Handled: true}
	}

	fmt.Printf("\n🔍 Potentially Unused Code in: %s\n", filepath.Base(pkgPath))
	fmt.Println(strings.Repeat("─", 50))

	if len(unused) == 0 {
		fmt.Println("✨ No unexported identifiers found (all code appears to be used)")
	} else {
		fmt.Printf("Found %d unexported identifiers (may be unused):\n\n", len(unused))
		for _, name := range unused {
			fmt.Printf("  • %s\n", name)
		}
		fmt.Println("\n💡 Note: This is a simple check. Some may be used via reflection.")
	}

	fmt.Println()
	return Result{Handled: true}
}
