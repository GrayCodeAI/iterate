// Package astanalysis provides Go code analysis using AST parsing
package astanalysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Analyzer provides AST-based code analysis
type Analyzer struct {
	RepoPath string
	Fset     *token.FileSet
}

// NewAnalyzer creates a new AST analyzer
func NewAnalyzer(repoPath string) *Analyzer {
	return &Analyzer{
		RepoPath: repoPath,
		Fset:     token.NewFileSet(),
	}
}

// FileInfo contains information about a Go file
type FileInfo struct {
	Path       string
	Package    string
	Imports    []string
	Functions  []FunctionInfo
	Structs    []StructInfo
	Interfaces []InterfaceInfo
	Variables  []VariableInfo
}

// FunctionInfo contains function details
type FunctionInfo struct {
	Name       string
	Receiver   string // For methods
	Params     []string
	Returns    []string
	Line       int
	IsExported bool
	Doc        string
}

// StructInfo contains struct details
type StructInfo struct {
	Name       string
	Fields     []FieldInfo
	Line       int
	IsExported bool
	Doc        string
}

// FieldInfo represents a struct field
type FieldInfo struct {
	Name string
	Type string
	Tag  string
}

// InterfaceInfo contains interface details
type InterfaceInfo struct {
	Name       string
	Methods    []string
	Line       int
	IsExported bool
	Doc        string
}

// VariableInfo contains variable details
type VariableInfo struct {
	Name       string
	Type       string
	Line       int
	IsExported bool
	IsConst    bool
}

// AnalyzeFile parses a single Go file and returns its structure
func (a *Analyzer) AnalyzeFile(path string) (*FileInfo, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	f, err := parser.ParseFile(a.Fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	info := &FileInfo{
		Path:    path,
		Package: f.Name.Name,
	}

	// Extract imports
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		info.Imports = append(info.Imports, path)
	}

	// Walk the AST
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			info.Functions = append(info.Functions, a.extractFunctionInfo(x))
		case *ast.GenDecl:
			a.extractGenDecl(x, info)
		}
		return true
	})

	return info, nil
}

func (a *Analyzer) extractFunctionInfo(f *ast.FuncDecl) FunctionInfo {
	fi := FunctionInfo{
		Name:       f.Name.Name,
		Line:       a.Fset.Position(f.Pos()).Line,
		IsExported: ast.IsExported(f.Name.Name),
	}

	if f.Doc != nil {
		fi.Doc = f.Doc.Text()
	}

	// Extract receiver for methods
	if f.Recv != nil && len(f.Recv.List) > 0 {
		fi.Receiver = exprToString(f.Recv.List[0].Type)
	}

	// Extract parameters
	if f.Type.Params != nil {
		for _, param := range f.Type.Params.List {
			for _, name := range param.Names {
				fi.Params = append(fi.Params, fmt.Sprintf("%s %s", name.Name, exprToString(param.Type)))
			}
		}
	}

	// Extract return values
	if f.Type.Results != nil {
		for _, result := range f.Type.Results.List {
			fi.Returns = append(fi.Returns, exprToString(result.Type))
		}
	}

	return fi
}

func (a *Analyzer) extractGenDecl(d *ast.GenDecl, info *FileInfo) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			switch t := s.Type.(type) {
			case *ast.StructType:
				info.Structs = append(info.Structs, a.extractStructInfo(s, t, d))
			case *ast.InterfaceType:
				info.Interfaces = append(info.Interfaces, a.extractInterfaceInfo(s, t, d))
			}
		case *ast.ValueSpec:
			isConst := d.Tok == token.CONST
			for _, name := range s.Names {
				vi := VariableInfo{
					Name:       name.Name,
					Line:       a.Fset.Position(name.Pos()).Line,
					IsExported: ast.IsExported(name.Name),
					IsConst:    isConst,
				}
				if s.Type != nil {
					vi.Type = exprToString(s.Type)
				}
				info.Variables = append(info.Variables, vi)
			}
		}
	}
}

func (a *Analyzer) extractStructInfo(s *ast.TypeSpec, t *ast.StructType, d *ast.GenDecl) StructInfo {
	si := StructInfo{
		Name:       s.Name.Name,
		Line:       a.Fset.Position(s.Pos()).Line,
		IsExported: ast.IsExported(s.Name.Name),
	}

	if d.Doc != nil {
		si.Doc = d.Doc.Text()
	}

	for _, field := range t.Fields.List {
		fi := FieldInfo{
			Type: exprToString(field.Type),
		}
		if len(field.Names) > 0 {
			fi.Name = field.Names[0].Name
		}
		if field.Tag != nil {
			fi.Tag = field.Tag.Value
		}
		si.Fields = append(si.Fields, fi)
	}

	return si
}

func (a *Analyzer) extractInterfaceInfo(s *ast.TypeSpec, t *ast.InterfaceType, d *ast.GenDecl) InterfaceInfo {
	ii := InterfaceInfo{
		Name:       s.Name.Name,
		Line:       a.Fset.Position(s.Pos()).Line,
		IsExported: ast.IsExported(s.Name.Name),
	}

	if d.Doc != nil {
		ii.Doc = d.Doc.Text()
	}

	for _, method := range t.Methods.List {
		if len(method.Names) > 0 {
			ii.Methods = append(ii.Methods, method.Names[0].Name)
		}
	}

	return ii
}

// exprToString converts an expression to its string representation
func exprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + exprToString(x.X)
	case *ast.ArrayType:
		return "[]" + exprToString(x.Elt)
	case *ast.SelectorExpr:
		return exprToString(x.X) + "." + x.Sel.Name
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToString(x.Key), exprToString(x.Value))
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprToString(x.Value)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return fmt.Sprintf("%T", e)
	}
}

// AnalyzePackage analyzes all Go files in a package
func (a *Analyzer) AnalyzePackage(pkgPath string) ([]*FileInfo, error) {
	var files []*FileInfo

	err := filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and test files
		if strings.Contains(path, "/vendor/") || strings.Contains(path, "/_test.go") {
			return nil
		}

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			fileInfo, err := a.AnalyzeFile(path)
			if err != nil {
				return err
			}
			files = append(files, fileInfo)
		}

		return nil
	})

	return files, err
}

// FindUnusedCode finds potentially unused functions and variables
func (a *Analyzer) FindUnusedCode(pkgPath string) ([]string, error) {
	files, err := a.AnalyzePackage(pkgPath)
	if err != nil {
		return nil, err
	}

	// Build a map of all defined identifiers
	defined := make(map[string]bool)
	for _, f := range files {
		for _, fn := range f.Functions {
			if !fn.IsExported {
				defined[fn.Name] = true
			}
		}
		for _, v := range f.Variables {
			if !v.IsExported {
				defined[v.Name] = true
			}
		}
	}

	// For now, return unexported identifiers as potentially unused
	var unused []string
	for name := range defined {
		unused = append(unused, name)
	}

	return unused, nil
}
