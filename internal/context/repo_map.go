// Package context provides intelligent context management for the Iterate agent.
// Task 36: Repo Map generator with AST-based signatures (exceeding Aider's Repo Map)

package context

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RepoMapConfig holds configuration for the repo map generator.
type RepoMapConfig struct {
	// Enabled languages
	GoEnabled         bool
	TypeScriptEnabled bool
	PythonEnabled     bool
	RustEnabled       bool

	// Extraction options
	IncludePrivate  bool     // Include private functions/methods
	IncludeImports  bool     // Include import statements
	IncludeComments bool     // Include doc comments
	MaxFileSize     int64    // Max file size to process (bytes)
	MaxFiles        int      // Max files to process
	ExcludePatterns []string // Glob patterns to exclude

	// Output options
	CompactMode     bool // Use compact output format
	MaxSignatureLen int  // Max signature length (truncate if longer)
	IncludeLineNums bool // Include line numbers in output
}

// DefaultRepoMapConfig returns the default configuration.
func DefaultRepoMapConfig() RepoMapConfig {
	return RepoMapConfig{
		GoEnabled:         true,
		TypeScriptEnabled: true,
		PythonEnabled:     true,
		RustEnabled:       true,
		IncludePrivate:    false,
		IncludeImports:    true,
		IncludeComments:   true,
		MaxFileSize:       500 * 1024, // 500KB
		MaxFiles:          1000,
		ExcludePatterns:   []string{"vendor/*", "node_modules/*", ".git/*", "dist/*", "build/*"},
		CompactMode:       false,
		MaxSignatureLen:   100,
		IncludeLineNums:   true,
	}
}

// SymbolKind represents the kind of a code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolType      SymbolKind = "type"
	SymbolConst     SymbolKind = "const"
	SymbolVar       SymbolKind = "var"
	SymbolField     SymbolKind = "field"
	SymbolImport    SymbolKind = "import"
)

// SymbolVisibility represents the visibility of a symbol.
type SymbolVisibility string

const (
	VisibilityPublic    SymbolVisibility = "public"
	VisibilityPrivate   SymbolVisibility = "private"
	VisibilityProtected SymbolVisibility = "protected"
	VisibilityInternal  SymbolVisibility = "internal"
)

// Symbol represents a code symbol (function, type, etc.).
type Symbol struct {
	Name       string           `json:"name"`
	Kind       SymbolKind       `json:"kind"`
	Visibility SymbolVisibility `json:"visibility"`
	Signature  string           `json:"signature"`
	DocComment string           `json:"doc_comment,omitempty"`
	File       string           `json:"file"`
	Line       int              `json:"line"`
	EndLine    int              `json:"end_line,omitempty"`
	Receiver   string           `json:"receiver,omitempty"` // For methods
	Parent     string           `json:"parent,omitempty"`   // Parent type for nested symbols
	Children   []Symbol         `json:"children,omitempty"` // Nested symbols
	Exported   bool             `json:"exported"`
}

// FileMap represents the symbols in a single file.
type FileMap struct {
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Package  string   `json:"package,omitempty"`
	Imports  []Import `json:"imports,omitempty"`
	Symbols  []Symbol `json:"symbols"`
}

// Import represents an import statement.
type Import struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
	Line  int    `json:"line"`
}

// PackageMap represents the symbols in a package/directory.
type PackageMap struct {
	Name      string              `json:"name"`
	Path      string              `json:"path"`
	Files     map[string]*FileMap `json:"files"`
	Exports   []Symbol            `json:"exports"` // Package-level exports
	Timestamp time.Time           `json:"timestamp"`
}

// RepoMap represents the complete map of a repository.
type RepoMap struct {
	RootPath     string                 `json:"root_path"`
	Packages     map[string]*PackageMap `json:"packages"`
	TotalFiles   int                    `json:"total_files"`
	TotalSymbols int                    `json:"total_symbols"`
	Timestamp    time.Time              `json:"timestamp"`
	Duration     time.Duration          `json:"duration"`

	// Index for quick lookups
	symbolIndex map[string][]*Symbol // name -> symbols
	mu          sync.RWMutex
}

// RepoMapGenerator generates repository maps using AST parsing.
type RepoMapGenerator struct {
	config RepoMapConfig
	logger *slog.Logger
	fset   *token.FileSet
}

// NewRepoMapGenerator creates a new repo map generator.
func NewRepoMapGenerator(config RepoMapConfig, logger *slog.Logger) *RepoMapGenerator {
	if logger == nil {
		logger = slog.Default()
	}

	return &RepoMapGenerator{
		config: config,
		logger: logger.With("component", "repo_map"),
		fset:   token.NewFileSet(),
	}
}

// Generate generates a complete repository map.
func (g *RepoMapGenerator) Generate(ctx context.Context, rootPath string) (*RepoMap, error) {
	startTime := time.Now()

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	g.logger.Info("Generating repo map", "path", rootPath)

	// Clean the path
	rootPath = filepath.Clean(rootPath)

	// Check if path exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}

	repoMap := &RepoMap{
		RootPath:    rootPath,
		Packages:    make(map[string]*PackageMap),
		symbolIndex: make(map[string][]*Symbol),
	}

	// Find all source files
	files, err := g.findSourceFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find source files: %w", err)
	}

	g.logger.Info("Found source files", "count", len(files))

	// Limit files if needed
	if len(files) > g.config.MaxFiles {
		files = files[:g.config.MaxFiles]
		g.logger.Warn("Limited files to max", "max", g.config.MaxFiles)
	}

	// Process files in parallel with worker pool
	const maxWorkers = 10
	fileChan := make(chan string, len(files))
	resultChan := make(chan *fileResult, len(files))

	var wg sync.WaitGroup

	// Start workers — each gets its own generator with its own FileSet,
	// because token.FileSet is not thread-safe.
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := NewRepoMapGenerator(g.config, g.logger)
			for path := range fileChan {
				select {
				case <-ctx.Done():
					resultChan <- &fileResult{path: path, err: ctx.Err()}
					return
				default:
				}

				fileMap, err := worker.parseFile(path)
				resultChan <- &fileResult{path: path, fileMap: fileMap, err: err}
			}
		}()
	}

	// Send files to workers
	for _, f := range files {
		fileChan <- f
	}
	close(fileChan)

	// Wait for workers
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var ctxErr error
	for result := range resultChan {
		if result.err != nil {
			g.logger.Debug("Failed to parse file", "path", result.path, "err", result.err)
			// Track context errors to return to caller
			if result.err == context.Canceled || result.err == context.DeadlineExceeded {
				ctxErr = result.err
			}
			continue
		}

		if result.fileMap == nil {
			continue
		}

		// Add to package map
		pkgPath := filepath.Dir(result.path)
		relPath, _ := filepath.Rel(rootPath, pkgPath)
		if relPath == "." {
			relPath = ""
		}

		pkg, exists := repoMap.Packages[relPath]
		if !exists {
			pkg = &PackageMap{
				Name:      result.fileMap.Package,
				Path:      relPath,
				Files:     make(map[string]*FileMap),
				Timestamp: time.Now(),
			}
			repoMap.Packages[relPath] = pkg
		}

		relFilePath, _ := filepath.Rel(rootPath, result.path)
		pkg.Files[relFilePath] = result.fileMap

		// Add exports
		for _, sym := range result.fileMap.Symbols {
			if sym.Exported {
				pkg.Exports = append(pkg.Exports, sym)
			}

			// Build symbol index
			repoMap.symbolIndex[sym.Name] = append(repoMap.symbolIndex[sym.Name], &sym)
		}

		repoMap.TotalFiles++
		repoMap.TotalSymbols += len(result.fileMap.Symbols)
	}

	repoMap.Timestamp = time.Now()
	repoMap.Duration = time.Since(startTime)

	g.logger.Info("Repo map generated",
		"packages", len(repoMap.Packages),
		"files", repoMap.TotalFiles,
		"symbols", repoMap.TotalSymbols,
		"duration", repoMap.Duration,
	)

	// Return context error if cancellation occurred
	if ctxErr != nil {
		return repoMap, ctxErr
	}

	return repoMap, nil
}

type fileResult struct {
	path    string
	fileMap *FileMap
	err     error
}

// findSourceFiles finds all source files in the repository.
func (g *RepoMapGenerator) findSourceFiles(rootPath string) ([]string, error) {
	var files []string

	extensions := map[string]bool{}
	if g.config.GoEnabled {
		extensions[".go"] = true
	}
	if g.config.TypeScriptEnabled {
		extensions[".ts"] = true
		extensions[".tsx"] = true
		extensions[".js"] = true
		extensions[".jsx"] = true
	}
	if g.config.PythonEnabled {
		extensions[".py"] = true
	}
	if g.config.RustEnabled {
		extensions[".rs"] = true
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			name := info.Name()
			// Skip common non-source directories
			if name == "vendor" || name == "node_modules" || name == ".git" ||
				name == "dist" || name == "build" || name == "target" ||
				name == "bin" || name == "pkg" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file size
		if info.Size() > g.config.MaxFileSize {
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(path))
		if !extensions[ext] {
			return nil
		}

		// Check exclude patterns
		relPath, _ := filepath.Rel(rootPath, path)
		for _, pattern := range g.config.ExcludePatterns {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// parseFile parses a single source file and extracts symbols.
func (g *RepoMapGenerator) parseFile(path string) (*FileMap, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return g.parseGoFile(path)
	case ".ts", ".tsx", ".js", ".jsx":
		return g.parseTypeScriptFile(path)
	case ".py":
		return g.parsePythonFile(path)
	case ".rs":
		return g.parseRustFile(path)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parseGoFile parses a Go source file using the go/parser AST.
func (g *RepoMapGenerator) parseGoFile(path string) (*FileMap, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the file
	f, err := parser.ParseFile(g.fset, path, content, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	fileMap := &FileMap{
		Path:     path,
		Language: "go",
		Package:  f.Name.Name,
		Imports:  make([]Import, 0),
		Symbols:  make([]Symbol, 0),
	}

	// Extract imports
	if g.config.IncludeImports {
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			fileMap.Imports = append(fileMap.Imports, Import{
				Path:  importPath,
				Alias: alias,
				Line:  g.fset.Position(imp.Pos()).Line,
			})
		}
	}

	// Extract declarations
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := g.extractFuncDecl(d)
			fileMap.Symbols = append(fileMap.Symbols, sym)

		case *ast.GenDecl:
			symbols := g.extractGenDecl(d)
			fileMap.Symbols = append(fileMap.Symbols, symbols...)
		}
	}

	return fileMap, nil
}

// extractFuncDecl extracts a function or method symbol.
func (g *RepoMapGenerator) extractFuncDecl(fn *ast.FuncDecl) Symbol {
	sym := Symbol{
		Name:    fn.Name.Name,
		Kind:    SymbolFunction,
		File:    g.fset.Position(fn.Pos()).Filename,
		Line:    g.fset.Position(fn.Pos()).Line,
		EndLine: g.fset.Position(fn.End()).Line,
	}

	// Check if it's a method
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sym.Kind = SymbolMethod
		// Extract receiver type
		for _, field := range fn.Recv.List {
			switch t := field.Type.(type) {
			case *ast.Ident:
				sym.Receiver = t.Name
			case *ast.StarExpr:
				if ident, ok := t.X.(*ast.Ident); ok {
					sym.Receiver = ident.Name
				}
			}
		}
	}

	// Check visibility
	sym.Exported = ast.IsExported(fn.Name.Name)
	if sym.Exported {
		sym.Visibility = VisibilityPublic
	} else {
		sym.Visibility = VisibilityPrivate
	}

	// Build signature
	sym.Signature = g.buildFuncSignature(fn)

	// Extract doc comment
	if g.config.IncludeComments && fn.Doc != nil {
		sym.DocComment = strings.TrimSpace(fn.Doc.Text())
	}

	// Skip private symbols if configured
	if !g.config.IncludePrivate && !sym.Exported {
		return Symbol{}
	}

	return sym
}

// buildFuncSignature builds a function signature string.
func (g *RepoMapGenerator) buildFuncSignature(fn *ast.FuncDecl) string {
	var sb strings.Builder

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sb.WriteString("(")
		for i, field := range fn.Recv.List {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(g.typeToString(field.Type))
		}
		sb.WriteString(") ")
	}

	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	// Parameters
	for i, param := range fn.Type.Params.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		for j, name := range param.Names {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(name.Name)
		}
		if len(param.Names) > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(g.typeToString(param.Type))
	}

	sb.WriteString(")")

	// Return types
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(" ")
			sb.WriteString(g.typeToString(fn.Type.Results.List[0].Type))
		} else {
			sb.WriteString(" (")
			for i, result := range fn.Type.Results.List {
				if i > 0 {
					sb.WriteString(", ")
				}
				for j, name := range result.Names {
					if j > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(name.Name)
					sb.WriteString(" ")
				}
				sb.WriteString(g.typeToString(result.Type))
			}
			sb.WriteString(")")
		}
	}

	sig := sb.String()
	if g.config.MaxSignatureLen > 0 && len(sig) > g.config.MaxSignatureLen {
		sig = sig[:g.config.MaxSignatureLen-3] + "..."
	}

	return sig
}

// typeToString converts an AST type to a string.
func (g *RepoMapGenerator) typeToString(t ast.Expr) string {
	switch tt := t.(type) {
	case *ast.Ident:
		return tt.Name
	case *ast.StarExpr:
		return "*" + g.typeToString(tt.X)
	case *ast.ArrayType:
		return "[]" + g.typeToString(tt.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", g.typeToString(tt.Key), g.typeToString(tt.Value))
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", g.typeToString(tt.X), tt.Sel.Name)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		switch tt.Dir {
		case ast.SEND:
			return "chan<- " + g.typeToString(tt.Value)
		case ast.RECV:
			return "<-chan " + g.typeToString(tt.Value)
		default:
			return "chan " + g.typeToString(tt.Value)
		}
	case *ast.FuncType:
		return "func(...)"
	case *ast.StructType:
		return "struct{...}"
	case *ast.Ellipsis:
		return "..." + g.typeToString(tt.Elt)
	default:
		return "any"
	}
}

// extractGenDecl extracts type, const, or var declarations.
func (g *RepoMapGenerator) extractGenDecl(decl *ast.GenDecl) []Symbol {
	var symbols []Symbol

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			sym := Symbol{
				Name:    s.Name.Name,
				File:    g.fset.Position(decl.Pos()).Filename,
				Line:    g.fset.Position(decl.Pos()).Line,
				EndLine: g.fset.Position(decl.End()).Line,
			}

			// Determine type kind
			switch s.Type.(type) {
			case *ast.StructType:
				sym.Kind = SymbolStruct
			case *ast.InterfaceType:
				sym.Kind = SymbolInterface
			default:
				sym.Kind = SymbolType
			}

			sym.Exported = ast.IsExported(s.Name.Name)
			if sym.Exported {
				sym.Visibility = VisibilityPublic
			} else {
				sym.Visibility = VisibilityPrivate
			}

			// Build signature
			sym.Signature = g.buildTypeSignature(s)

			// Extract doc comment
			if g.config.IncludeComments && decl.Doc != nil {
				sym.DocComment = strings.TrimSpace(decl.Doc.Text())
			}

			// Extract nested symbols (struct fields)
			if st, ok := s.Type.(*ast.StructType); ok && st.Fields != nil {
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						fieldSym := Symbol{
							Name:       name.Name,
							Kind:       SymbolField,
							Visibility: VisibilityPublic,
							Parent:     s.Name.Name,
							Line:       g.fset.Position(field.Pos()).Line,
						}
						if ast.IsExported(name.Name) {
							fieldSym.Visibility = VisibilityPublic
						} else {
							fieldSym.Visibility = VisibilityPrivate
						}
						fieldSym.Signature = g.typeToString(field.Type)
						sym.Children = append(sym.Children, fieldSym)
					}
				}
			}

			// Skip private if configured
			if !g.config.IncludePrivate && !sym.Exported {
				continue
			}

			symbols = append(symbols, sym)

		case *ast.ValueSpec:
			kind := SymbolConst
			if decl.Tok == token.VAR {
				kind = SymbolVar
			}

			for _, name := range s.Names {
				sym := Symbol{
					Name: name.Name,
					Kind: kind,
					File: g.fset.Position(decl.Pos()).Filename,
					Line: g.fset.Position(decl.Pos()).Line,
				}

				sym.Exported = ast.IsExported(name.Name)
				if sym.Exported {
					sym.Visibility = VisibilityPublic
				} else {
					sym.Visibility = VisibilityPrivate
				}

				if s.Type != nil {
					sym.Signature = g.typeToString(s.Type)
				} else if s.Values != nil {
					sym.Signature = "= ..."
				}

				if g.config.IncludeComments && decl.Doc != nil {
					sym.DocComment = strings.TrimSpace(decl.Doc.Text())
				}

				if !g.config.IncludePrivate && !sym.Exported {
					continue
				}

				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

// buildTypeSignature builds a type signature string.
func (g *RepoMapGenerator) buildTypeSignature(spec *ast.TypeSpec) string {
	var sb strings.Builder

	sb.WriteString("type ")
	sb.WriteString(spec.Name.Name)

	switch t := spec.Type.(type) {
	case *ast.StructType:
		sb.WriteString(" struct")
		if t.Fields != nil {
			sb.WriteString(fmt.Sprintf(" {/* %d fields */}", len(t.Fields.List)))
		}
	case *ast.InterfaceType:
		sb.WriteString(" interface")
		if t.Methods != nil {
			sb.WriteString(fmt.Sprintf(" {/* %d methods */}", len(t.Methods.List)))
		}
	default:
		sb.WriteString(" ")
		sb.WriteString(g.typeToString(t))
	}

	return sb.String()
}

// parseTypeScriptFile parses a TypeScript/JavaScript file (simplified regex-based).
func (g *RepoMapGenerator) parseTypeScriptFile(path string) (*FileMap, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fileMap := &FileMap{
		Path:     path,
		Language: "typescript",
		Symbols:  make([]Symbol, 0),
	}

	// Simple regex-based extraction for TypeScript
	// In a full implementation, we'd use a proper TypeScript parser
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Match function declarations
		if strings.HasPrefix(line, "function ") || strings.HasPrefix(line, "export function ") {
			name := extractName(line, "function")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolFunction,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Exported:   strings.HasPrefix(line, "export "),
					Visibility: VisibilityPublic,
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match const/let/var function assignments
		if strings.Contains(line, "=>") && strings.Contains(line, "function") == false {
			name := extractArrowFunctionName(line)
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolFunction,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Exported:   strings.HasPrefix(line, "export "),
					Visibility: VisibilityPublic,
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match class declarations
		if strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "export class ") {
			name := extractName(line, "class")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolStruct,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Exported:   strings.HasPrefix(line, "export "),
					Visibility: VisibilityPublic,
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match interface declarations
		if strings.HasPrefix(line, "interface ") || strings.HasPrefix(line, "export interface ") {
			name := extractName(line, "interface")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolInterface,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Exported:   strings.HasPrefix(line, "export "),
					Visibility: VisibilityPublic,
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}
	}

	return fileMap, nil
}

// parsePythonFile parses a Python file (simplified regex-based).
func (g *RepoMapGenerator) parsePythonFile(path string) (*FileMap, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fileMap := &FileMap{
		Path:     path,
		Language: "python",
		Symbols:  make([]Symbol, 0),
	}

	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Match function definitions
		if strings.HasPrefix(line, "def ") {
			name := extractName(line, "def")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolFunction,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Visibility: VisibilityPublic,
				}
				if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
					sym.Visibility = VisibilityProtected
				} else if strings.HasPrefix(name, "__") {
					sym.Visibility = VisibilityPrivate
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match class definitions
		if strings.HasPrefix(line, "class ") {
			name := extractName(line, "class")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolStruct,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Visibility: VisibilityPublic,
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}
	}

	return fileMap, nil
}

// parseRustFile parses a Rust file (simplified regex-based).
func (g *RepoMapGenerator) parseRustFile(path string) (*FileMap, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fileMap := &FileMap{
		Path:     path,
		Language: "rust",
		Symbols:  make([]Symbol, 0),
	}

	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Match function definitions
		if strings.HasPrefix(line, "pub fn ") || strings.HasPrefix(line, "fn ") {
			name := extractName(line, "fn")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolFunction,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Visibility: VisibilityPublic,
				}
				if !strings.HasPrefix(line, "pub ") {
					sym.Visibility = VisibilityPrivate
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match struct definitions
		if strings.HasPrefix(line, "pub struct ") || strings.HasPrefix(line, "struct ") {
			name := extractName(line, "struct")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolStruct,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Visibility: VisibilityPublic,
				}
				if !strings.HasPrefix(line, "pub ") {
					sym.Visibility = VisibilityPrivate
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}

		// Match trait definitions
		if strings.HasPrefix(line, "pub trait ") || strings.HasPrefix(line, "trait ") {
			name := extractName(line, "trait")
			if name != "" {
				sym := Symbol{
					Name:       name,
					Kind:       SymbolInterface,
					Line:       i + 1,
					Signature:  truncateLine(line, g.config.MaxSignatureLen),
					Visibility: VisibilityPublic,
				}
				if !strings.HasPrefix(line, "pub ") {
					sym.Visibility = VisibilityPrivate
				}
				fileMap.Symbols = append(fileMap.Symbols, sym)
			}
		}
	}

	return fileMap, nil
}

// Helper functions

func extractName(line, keyword string) string {
	prefix := keyword + " "
	if strings.HasPrefix(line, "export ") {
		prefix = "export " + prefix
	}
	if strings.HasPrefix(line, "pub ") {
		prefix = "pub " + prefix
	}

	if !strings.HasPrefix(line, prefix) {
		return ""
	}

	rest := strings.TrimPrefix(line, prefix)

	// Find the end of the name
	nameEnd := strings.IndexFunc(rest, func(r rune) bool {
		return r == '(' || r == ' ' || r == '{' || r == ':' || r == '<' || r == '['
	})

	if nameEnd == -1 {
		return rest
	}

	return rest[:nameEnd]
}

func extractArrowFunctionName(line string) string {
	// Match patterns like: const name =, let name =, var name =
	for _, prefix := range []string{"const ", "let ", "var "} {
		if strings.HasPrefix(line, prefix) {
			rest := strings.TrimPrefix(line, prefix)
			nameEnd := strings.Index(rest, " ")
			if nameEnd > 0 {
				name := rest[:nameEnd]
				if strings.Contains(rest, "=>") {
					return name
				}
			}
		}
	}
	return ""
}

func truncateLine(line string, maxLen int) string {
	if maxLen <= 0 || len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}

// LookupSymbol finds symbols by name.
func (rm *RepoMap) LookupSymbol(name string) []*Symbol {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.symbolIndex[name]
}

// GetPackage returns a package by path.
func (rm *RepoMap) GetPackage(path string) *PackageMap {
	return rm.Packages[path]
}

// GetFile returns a file map by path.
func (rm *RepoMap) GetFile(path string) *FileMap {
	for _, pkg := range rm.Packages {
		if f, ok := pkg.Files[path]; ok {
			return f
		}
	}
	return nil
}

// ToMarkdown generates a markdown representation of the repo map.
func (rm *RepoMap) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Repository Map\n\n")
	sb.WriteString(fmt.Sprintf("- **Root**: `%s`\n", rm.RootPath))
	sb.WriteString(fmt.Sprintf("- **Packages**: %d\n", len(rm.Packages)))
	sb.WriteString(fmt.Sprintf("- **Files**: %d\n", rm.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Symbols**: %d\n", rm.TotalSymbols))
	sb.WriteString(fmt.Sprintf("- **Generated**: %s\n", rm.Timestamp.Format(time.RFC3339)))
	sb.WriteString("\n---\n\n")

	// Sort packages by path
	var pkgPaths []string
	for path := range rm.Packages {
		pkgPaths = append(pkgPaths, path)
	}
	sort.Strings(pkgPaths)

	for _, pkgPath := range pkgPaths {
		pkg := rm.Packages[pkgPath]

		if pkgPath == "" {
			sb.WriteString("## . (root)\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("## %s\n\n", pkgPath))
		}

		// Sort files
		var filePaths []string
		for path := range pkg.Files {
			filePaths = append(filePaths, path)
		}
		sort.Strings(filePaths)

		for _, filePath := range filePaths {
			f := pkg.Files[filePath]
			sb.WriteString(fmt.Sprintf("### `%s`\n\n", filepath.Base(filePath)))

			if len(f.Imports) > 0 {
				sb.WriteString("**Imports:**\n")
				for _, imp := range f.Imports {
					if imp.Alias != "" {
						sb.WriteString(fmt.Sprintf("- `%s` as `%s`\n", imp.Path, imp.Alias))
					} else {
						sb.WriteString(fmt.Sprintf("- `%s`\n", imp.Path))
					}
				}
				sb.WriteString("\n")
			}

			if len(f.Symbols) > 0 {
				sb.WriteString("**Symbols:**\n\n")
				sb.WriteString("| Line | Kind | Name | Signature |\n")
				sb.WriteString("|------|------|------|----------|\n")

				for _, sym := range f.Symbols {
					visibility := ""
					if sym.Visibility == VisibilityPrivate {
						visibility = "🔒 "
					}
					sb.WriteString(fmt.Sprintf("| %d | %s | `%s%s` | `%s` |\n",
						sym.Line, sym.Kind, visibility, sym.Name, sym.Signature))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// ToCompactString generates a compact string representation (Aider-style).
func (rm *RepoMap) ToCompactString() string {
	var sb strings.Builder

	// Sort packages
	var pkgPaths []string
	for path := range rm.Packages {
		pkgPaths = append(pkgPaths, path)
	}
	sort.Strings(pkgPaths)

	for _, pkgPath := range pkgPaths {
		pkg := rm.Packages[pkgPath]

		if pkgPath == "" {
			sb.WriteString("root:\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s:\n", pkgPath))
		}

		// Sort files
		var filePaths []string
		for path := range pkg.Files {
			filePaths = append(filePaths, path)
		}
		sort.Strings(filePaths)

		for _, filePath := range filePaths {
			f := pkg.Files[filePath]
			sb.WriteString(fmt.Sprintf("  %s:\n", filepath.Base(filePath)))

			// Group symbols by kind
			functions := make([]Symbol, 0)
			types := make([]Symbol, 0)
			others := make([]Symbol, 0)

			for _, sym := range f.Symbols {
				switch sym.Kind {
				case SymbolFunction, SymbolMethod:
					functions = append(functions, sym)
				case SymbolStruct, SymbolInterface, SymbolType:
					types = append(types, sym)
				default:
					others = append(others, sym)
				}
			}

			// Output types first
			for _, sym := range types {
				sb.WriteString(fmt.Sprintf("    %s %s\n", sym.Kind, sym.Name))
			}

			// Then functions
			for _, sym := range functions {
				if sym.Kind == SymbolMethod {
					sb.WriteString(fmt.Sprintf("    method (%s) %s\n", sym.Receiver, sym.Name))
				} else {
					sb.WriteString(fmt.Sprintf("    func %s\n", sym.Name))
				}
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
