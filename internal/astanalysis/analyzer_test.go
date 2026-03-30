package astanalysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeFile_ValidGoCode(t *testing.T) {
	// Create a temporary Go file with valid code
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	
	validCode := `package testpkg

import "fmt"

type Person struct {
	Name string
	Age  int
}

func (p Person) Greet() string {
	return fmt.Sprintf("Hello, I'm %s", p.Name)
}

func NewPerson(name string, age int) Person {
	return Person{Name: name, Age: age}
}
`
	
	if err := os.WriteFile(tmpFile, []byte(validCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	if info == nil {
		t.Fatal("Expected FileInfo, got nil")
	}
	
	if info.Path != tmpFile {
		t.Errorf("Expected Path=%s, got %s", tmpFile, info.Path)
	}
}

func TestAnalyzeFile_InvalidSyntax(t *testing.T) {
	// Create a temporary file with invalid Go syntax
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.go")
	
	invalidCode := `package testpkg

func invalid syntax here {
	return nothing
`
	
	if err := os.WriteFile(tmpFile, []byte(invalidCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err == nil {
		t.Error("Expected error for invalid syntax, got nil")
	}
	
	if info != nil {
		t.Error("Expected nil FileInfo for invalid syntax")
	}
}

func TestExtractFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "funcs.go")
	
	code := `package testpkg

func PublicFunc() {}
func privateFunc() {}

func FuncWithParams(a int, b string) bool {
	return true
}

func FuncWithReturn() (int, error) {
	return 0, nil
}

type MyStruct struct{}

func (m MyStruct) Method() {}
`
	
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	expectedFuncs := map[string]struct {
		exported   bool
		hasReceiver bool
	}{
		"PublicFunc":       {exported: true, hasReceiver: false},
		"privateFunc":      {exported: false, hasReceiver: false},
		"FuncWithParams":   {exported: true, hasReceiver: false},
		"FuncWithReturn":   {exported: true, hasReceiver: false},
		"Method":           {exported: true, hasReceiver: true},
	}
	
	if len(info.Functions) != len(expectedFuncs) {
		t.Errorf("Expected %d functions, got %d", len(expectedFuncs), len(info.Functions))
	}
	
	found := make(map[string]bool)
	for _, fn := range info.Functions {
		found[fn.Name] = true
		
		exp, ok := expectedFuncs[fn.Name]
		if !ok {
			t.Errorf("Unexpected function: %s", fn.Name)
			continue
		}
		
		if fn.IsExported != exp.exported {
			t.Errorf("Function %s: expected exported=%v, got %v", fn.Name, exp.exported, fn.IsExported)
		}
		
		if exp.hasReceiver && fn.Receiver == "" {
			t.Errorf("Function %s: expected receiver, got none", fn.Name)
		}
	}
	
	for name := range expectedFuncs {
		if !found[name] {
			t.Errorf("Expected function not found: %s", name)
		}
	}
}

func TestExtractStructs(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "structs.go")
	
	code := `package testpkg

type PublicStruct struct {
	Field1 string
	Field2 int
}

type privateStruct struct {
	Name string
}

type StructWithTags struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\" db:\"username\"`" + `
}

type NestedStruct struct {
	Inner PublicStruct
	Ptr   *PublicStruct
}
`
	
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	expectedStructs := map[string]struct {
		exported bool
		fields   int
	}{
		"PublicStruct":     {exported: true, fields: 2},
		"privateStruct":    {exported: false, fields: 1},
		"StructWithTags":   {exported: true, fields: 2},
		"NestedStruct":     {exported: true, fields: 2},
	}
	
	if len(info.Structs) != len(expectedStructs) {
		t.Errorf("Expected %d structs, got %d", len(expectedStructs), len(info.Structs))
	}
	
	found := make(map[string]bool)
	for _, s := range info.Structs {
		found[s.Name] = true
		
		exp, ok := expectedStructs[s.Name]
		if !ok {
			t.Errorf("Unexpected struct: %s", s.Name)
			continue
		}
		
		if s.IsExported != exp.exported {
			t.Errorf("Struct %s: expected exported=%v, got %v", s.Name, exp.exported, s.IsExported)
		}
		
		if len(s.Fields) != exp.fields {
			t.Errorf("Struct %s: expected %d fields, got %d", s.Name, exp.fields, len(s.Fields))
		}
	}
	
	for name := range expectedStructs {
		if !found[name] {
			t.Errorf("Expected struct not found: %s", name)
		}
	}
}

func TestGetPackageName(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "pkg.go")
	
	code := `package myspecialpackage

func Dummy() {}
`
	
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	if info.Package != "myspecialpackage" {
		t.Errorf("Expected Package='myspecialpackage', got '%s'", info.Package)
	}
}

func TestExtractImports(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "imports.go")
	
	code := `package testpkg

import (
	"fmt"
	"os"
	"strings"
)

import "path/filepath"

func UseImports() {
	fmt.Println(os.Getenv("HOME"))
	strings.Split("a,b", ",")
	filepath.Join("a", "b")
}
`
	
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	expectedImports := map[string]bool{
		"fmt":          true,
		"os":           true,
		"strings":      true,
		"path/filepath": true,
	}
	
	found := make(map[string]bool)
	for _, imp := range info.Imports {
		found[imp] = true
	}
	
	for imp := range expectedImports {
		if !found[imp] {
			t.Errorf("Expected import not found: %s", imp)
		}
	}
	
	if len(info.Imports) != len(expectedImports) {
		t.Errorf("Expected %d imports, got %d", len(expectedImports), len(info.Imports))
	}
}

func TestExtractInterfaces(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "interfaces.go")
	
	code := `package testpkg

type PublicInterface interface {
	Method1() error
	Method2(param string) int
}

type privateInterface interface {
	DoSomething()
}
`
	
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile(tmpFile)
	
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	if len(info.Interfaces) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(info.Interfaces))
	}
	
	for _, iface := range info.Interfaces {
		switch iface.Name {
		case "PublicInterface":
			if !iface.IsExported {
				t.Error("PublicInterface should be exported")
			}
			if len(iface.Methods) != 2 {
				t.Errorf("Expected 2 methods, got %d", len(iface.Methods))
			}
		case "privateInterface":
			if iface.IsExported {
				t.Error("privateInterface should not be exported")
			}
			if len(iface.Methods) != 1 {
				t.Errorf("Expected 1 method, got %d", len(iface.Methods))
			}
		default:
			t.Errorf("Unexpected interface: %s", iface.Name)
		}
	}
}

func TestAnalyzePackage(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple Go files
	files := map[string]string{
		"file1.go": `package pkg1
func Func1() {}
`,
		"file2.go": `package pkg1
func Func2() {}
`,
		"sub/file3.go": `package subpkg
func Func3() {}
`,
	}
	
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}
	
	analyzer := NewAnalyzer(tmpDir)
	infos, err := analyzer.AnalyzePackage(tmpDir)
	
	if err != nil {
		t.Fatalf("AnalyzePackage failed: %v", err)
	}
	
	// Should find 2 files (pkg1 files), subpkg is a different package
	// Note: The actual behavior depends on the implementation
	if len(infos) < 1 {
		t.Errorf("Expected at least 1 file info, got %d", len(infos))
	}
}

func TestNewAnalyzer(t *testing.T) {
	repoPath := "/some/path"
	analyzer := NewAnalyzer(repoPath)
	
	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}
	
	if analyzer.RepoPath != repoPath {
		t.Errorf("Expected RepoPath=%s, got %s", repoPath, analyzer.RepoPath)
	}
	
	if analyzer.Fset == nil {
		t.Error("Expected Fset to be initialized")
	}
}

func TestAnalyzeFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	analyzer := NewAnalyzer(tmpDir)
	info, err := analyzer.AnalyzeFile("/nonexistent/path/file.go")
	
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	
	if info != nil {
		t.Error("Expected nil info for non-existent file")
	}
}
