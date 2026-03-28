// Package context provides repo map tests.
// Task 36: Repo Map generator tests

package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRepoMapGenerator(t *testing.T) {
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	if gen == nil {
		t.Fatal("expected non-nil generator")
	}
	
	if !gen.config.GoEnabled {
		t.Error("expected Go enabled by default")
	}
}

func TestDefaultRepoMapConfig(t *testing.T) {
	config := DefaultRepoMapConfig()
	
	if !config.GoEnabled {
		t.Error("expected Go enabled")
	}
	if !config.TypeScriptEnabled {
		t.Error("expected TypeScript enabled")
	}
	if !config.PythonEnabled {
		t.Error("expected Python enabled")
	}
	if !config.RustEnabled {
		t.Error("expected Rust enabled")
	}
	if config.IncludePrivate {
		t.Error("expected IncludePrivate false by default")
	}
	if !config.IncludeImports {
		t.Error("expected IncludeImports true by default")
	}
	if config.MaxFiles != 1000 {
		t.Errorf("expected MaxFiles 1000, got %d", config.MaxFiles)
	}
}

func TestRepoMapGenerator_GoFile(t *testing.T) {
	// Create a temporary Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

import (
	"fmt"
	"strings"
)

// TestStruct is a test struct.
type TestStruct struct {
	Name  string
	Value int
}

// TestInterface is a test interface.
type TestInterface interface {
	DoSomething() error
}

// TestFunc is a test function.
func TestFunc(a int, b string) error {
	return nil
}

// privateFunc is private.
func privateFunc() {}

func (s *TestStruct) Method() string {
	return s.Name
}

const TestConst = "value"

var TestVar = 42
`
	err := os.WriteFile(goFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	config.IncludePrivate = true
	
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate repo map: %v", err)
	}
	
	if repoMap == nil {
		t.Fatal("expected non-nil repo map")
	}
	
	if repoMap.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", repoMap.TotalFiles)
	}
	
	// Check symbols were extracted
	if repoMap.TotalSymbols == 0 {
		t.Error("expected some symbols to be extracted")
	}
	
	// Check we can look up symbols
	symbols := repoMap.LookupSymbol("TestFunc")
	if len(symbols) == 0 {
		t.Error("expected to find TestFunc")
	}
	
	symbols = repoMap.LookupSymbol("TestStruct")
	if len(symbols) == 0 {
		t.Error("expected to find TestStruct")
	}
}

func TestRepoMapGenerator_PrivateSymbols(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

func PublicFunc() {}
func privateFunc() {}
type PublicType struct {}
type privateType struct {}
`
	err := os.WriteFile(goFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	t.Run("exclude_private", func(t *testing.T) {
		config := DefaultRepoMapConfig()
		config.IncludePrivate = false
		
		gen := NewRepoMapGenerator(config, nil)
		
		ctx := context.Background()
		repoMap, err := gen.Generate(ctx, tmpDir)
		if err != nil {
			t.Fatalf("failed to generate: %v", err)
		}
		
		// Should have public symbols only
		publicSymbols := repoMap.LookupSymbol("PublicFunc")
		if len(publicSymbols) == 0 {
			t.Error("expected to find PublicFunc")
		}
		
		privateSymbols := repoMap.LookupSymbol("privateFunc")
		if len(privateSymbols) > 0 {
			t.Error("expected privateFunc to be excluded")
		}
	})
	
	t.Run("include_private", func(t *testing.T) {
		config := DefaultRepoMapConfig()
		config.IncludePrivate = true
		
		gen := NewRepoMapGenerator(config, nil)
		
		ctx := context.Background()
		repoMap, err := gen.Generate(ctx, tmpDir)
		if err != nil {
			t.Fatalf("failed to generate: %v", err)
		}
		
		// Should have both public and private
		publicSymbols := repoMap.LookupSymbol("PublicFunc")
		if len(publicSymbols) == 0 {
			t.Error("expected to find PublicFunc")
		}
		
		privateSymbols := repoMap.LookupSymbol("privateFunc")
		if len(privateSymbols) == 0 {
			t.Error("expected to find privateFunc")
		}
	})
}

func TestRepoMapGenerator_Methods(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

type Service struct{}

func (s *Service) Method1() {}
func (s Service) Method2() {}
func NewService() *Service { return &Service{} }
`
	err := os.WriteFile(goFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Check methods were extracted with receiver
	methods := repoMap.LookupSymbol("Method1")
	if len(methods) == 0 {
		t.Fatal("expected to find Method1")
	}
	
	if methods[0].Kind != SymbolMethod {
		t.Errorf("expected kind method, got %s", methods[0].Kind)
	}
	
	if methods[0].Receiver != "Service" {
		t.Errorf("expected receiver Service, got %s", methods[0].Receiver)
	}
	
	// NewService should be a function, not a method
	newService := repoMap.LookupSymbol("NewService")
	if len(newService) == 0 {
		t.Fatal("expected to find NewService")
	}
	
	if newService[0].Kind != SymbolFunction {
		t.Errorf("expected kind function, got %s", newService[0].Kind)
	}
}

func TestRepoMapGenerator_Imports(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

import (
	"fmt"
	"strings"
	json "encoding/json"
)
`
	err := os.WriteFile(goFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Find the file
	file := repoMap.GetFile("test.go")
	if file == nil {
		t.Fatal("expected to find test.go")
	}
	
	if len(file.Imports) != 3 {
		t.Errorf("expected 3 imports, got %d", len(file.Imports))
	}
	
	// Check aliased import
	found := false
	for _, imp := range file.Imports {
		if imp.Path == "encoding/json" && imp.Alias == "json" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("expected to find aliased import encoding/json")
	}
}

func TestRepoMapGenerator_PythonFile(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	
	content := `def public_func():
    pass

def _protected_func():
    pass

def __private_func():
    pass

class TestClass:
    def method(self):
        pass
`
	err := os.WriteFile(pyFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Check Python symbols
	publicFunc := repoMap.LookupSymbol("public_func")
	if len(publicFunc) == 0 {
		t.Error("expected to find public_func")
	}
	
	testClass := repoMap.LookupSymbol("TestClass")
	if len(testClass) == 0 {
		t.Error("expected to find TestClass")
	}
}

func TestRepoMapGenerator_TypeScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "test.ts")
	
	content := `export function hello(name: string): void {
    console.log(name);
}

export class Service {
    doWork(): void {}
}

interface Config {
    port: number;
}

const arrow = () => 42;
`
	err := os.WriteFile(tsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Check TypeScript symbols
	hello := repoMap.LookupSymbol("hello")
	if len(hello) == 0 {
		t.Error("expected to find hello function")
	}
	
	service := repoMap.LookupSymbol("Service")
	if len(service) == 0 {
		t.Error("expected to find Service class")
	}
	
	configSym := repoMap.LookupSymbol("Config")
	if len(configSym) == 0 {
		t.Error("expected to find Config interface")
	}
}

func TestRepoMapGenerator_RustFile(t *testing.T) {
	tmpDir := t.TempDir()
	rsFile := filepath.Join(tmpDir, "test.rs")
	
	content := `pub fn public_function() {}

fn private_function() {}

pub struct PublicStruct {
    field: i32,
}

trait MyTrait {
    fn do_something(&self);
}
`
	err := os.WriteFile(rsFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	
	config := DefaultRepoMapConfig()
	config.IncludePrivate = true
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Check Rust symbols
	publicFn := repoMap.LookupSymbol("public_function")
	if len(publicFn) == 0 {
		t.Error("expected to find public_function")
	}
	
	publicStruct := repoMap.LookupSymbol("PublicStruct")
	if len(publicStruct) == 0 {
		t.Error("expected to find PublicStruct")
	}
	
	myTrait := repoMap.LookupSymbol("MyTrait")
	if len(myTrait) == 0 {
		t.Error("expected to find MyTrait")
	}
}

func TestRepoMapGenerator_SkipDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create Go file in root
	rootFile := filepath.Join(tmpDir, "root.go")
	os.WriteFile(rootFile, []byte("package main\nfunc Main() {}"), 0644)
	
	// Create Go file in vendor (should be skipped)
	vendorDir := filepath.Join(tmpDir, "vendor")
	os.Mkdir(vendorDir, 0755)
	vendorFile := filepath.Join(vendorDir, "vendor.go")
	os.WriteFile(vendorFile, []byte("package vendor\nfunc Vendor() {}"), 0644)
	
	// Create Go file in node_modules (should be skipped)
	nodeDir := filepath.Join(tmpDir, "node_modules")
	os.Mkdir(nodeDir, 0755)
	nodeFile := filepath.Join(nodeDir, "node.go")
	os.WriteFile(nodeFile, []byte("package node\nfunc Node() {}"), 0644)
	
	// Create Go file in hidden directory (should be skipped)
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	os.Mkdir(hiddenDir, 0755)
	hiddenFile := filepath.Join(hiddenDir, "hidden.go")
	os.WriteFile(hiddenFile, []byte("package hidden\nfunc Hidden() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Should only have root file
	if repoMap.TotalFiles != 1 {
		t.Errorf("expected 1 file (root only), got %d", repoMap.TotalFiles)
	}
	
	// Should find Main but not vendor/node/hidden functions
	if len(repoMap.LookupSymbol("Main")) == 0 {
		t.Error("expected to find Main")
	}
	if len(repoMap.LookupSymbol("Vendor")) > 0 {
		t.Error("expected Vendor to be skipped")
	}
	if len(repoMap.LookupSymbol("Node")) > 0 {
		t.Error("expected Node to be skipped")
	}
	if len(repoMap.LookupSymbol("Hidden")) > 0 {
		t.Error("expected Hidden to be skipped")
	}
}

func TestRepoMapGenerator_MaxFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple files
	for i := 0; i < 20; i++ {
		goFile := filepath.Join(tmpDir, "file"+string(rune('a'+i))+".go")
		os.WriteFile(goFile, []byte("package main\nfunc Func"+string(rune('a'+i))+"() {}"), 0644)
	}
	
	config := DefaultRepoMapConfig()
	config.MaxFiles = 5
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	if repoMap.TotalFiles > 5 {
		t.Errorf("expected at most 5 files, got %d", repoMap.TotalFiles)
	}
}

func TestRepoMapGenerator_MaxFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create small file
	smallFile := filepath.Join(tmpDir, "small.go")
	os.WriteFile(smallFile, []byte("package main\nfunc small() {}"), 0644)
	
	// Create large file (should be skipped)
	largeFile := filepath.Join(tmpDir, "large.go")
	largeContent := strings.Repeat("// comment\n", 10000) // ~100KB
	os.WriteFile(largeFile, []byte("package main\n"+largeContent+"\nfunc large() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	config.MaxFileSize = 500 // 500 bytes
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Should only have small file
	if repoMap.TotalFiles != 1 {
		t.Errorf("expected 1 file (small only), got %d", repoMap.TotalFiles)
	}
}

func TestRepoMapGenerator_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a file
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte("package main\nfunc main() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	_, err := gen.Generate(ctx, tmpDir)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestRepoMapGenerator_InvalidPath(t *testing.T) {
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	
	t.Run("nonexistent", func(t *testing.T) {
		_, err := gen.Generate(ctx, "/nonexistent/path")
		if err == nil {
			t.Error("expected error for nonexistent path")
		}
	})
	
	t.Run("file_not_dir", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "file.txt")
		os.WriteFile(tmpFile, []byte("test"), 0644)
		
		_, err := gen.Generate(ctx, tmpFile)
		if err == nil {
			t.Error("expected error for file path")
		}
	})
}

func TestRepoMap_ToMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

import "fmt"

func TestFunc() {}
type TestStruct struct{}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	markdown := repoMap.ToMarkdown()
	
	// Check markdown contains expected elements
	if !strings.Contains(markdown, "# Repository Map") {
		t.Error("expected markdown to contain header")
	}
	if !strings.Contains(markdown, "TestFunc") {
		t.Error("expected markdown to contain TestFunc")
	}
	if !strings.Contains(markdown, "TestStruct") {
		t.Error("expected markdown to contain TestStruct")
	}
}

func TestRepoMap_ToCompactString(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

func TestFunc() {}
type TestStruct struct{}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	compact := repoMap.ToCompactString()
	
	// Check compact format
	if !strings.Contains(compact, "root:") {
		t.Error("expected compact to contain root:")
	}
	if !strings.Contains(compact, "func TestFunc") {
		t.Error("expected compact to contain func TestFunc")
	}
	if !strings.Contains(compact, "struct TestStruct") {
		t.Error("expected compact to contain struct TestStruct")
	}
}

func TestRepoMap_GetPackage(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subpkg")
	os.Mkdir(subDir, 0755)
	goFile := filepath.Join(subDir, "test.go")
	os.WriteFile(goFile, []byte("package subpkg\nfunc Test() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	pkg := repoMap.GetPackage("subpkg")
	if pkg == nil {
		t.Fatal("expected to find subpkg package")
	}
	
	if pkg.Name != "subpkg" {
		t.Errorf("expected package name subpkg, got %s", pkg.Name)
	}
}

func TestSymbolKinds(t *testing.T) {
	kinds := []SymbolKind{
		SymbolFunction,
		SymbolMethod,
		SymbolStruct,
		SymbolInterface,
		SymbolType,
		SymbolConst,
		SymbolVar,
		SymbolField,
		SymbolImport,
	}
	
	for _, kind := range kinds {
		if string(kind) == "" {
			t.Errorf("kind should not be empty")
		}
	}
}

func TestSymbolVisibility(t *testing.T) {
	visibilities := []SymbolVisibility{
		VisibilityPublic,
		VisibilityPrivate,
		VisibilityProtected,
		VisibilityInternal,
	}
	
	for _, vis := range visibilities {
		if string(vis) == "" {
			t.Errorf("visibility should not be empty")
		}
	}
}

func TestRepoMap_Duration(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(goFile, []byte("package main\nfunc main() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	if repoMap.Duration <= 0 {
		t.Error("expected positive duration")
	}
	
	if repoMap.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRepoMap_ExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	
	genDir := filepath.Join(tmpDir, "generated")
	os.Mkdir(genDir, 0755)
	os.WriteFile(filepath.Join(genDir, "gen.go"), []byte("package gen\nfunc Gen() {}"), 0644)
	
	config := DefaultRepoMapConfig()
	config.ExcludePatterns = []string{"generated/*"}
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	// Should only have main.go
	if repoMap.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", repoMap.TotalFiles)
	}
	
	// Should find Main but not Gen
	if len(repoMap.LookupSymbol("Main")) == 0 {
		t.Error("expected to find Main")
	}
	if len(repoMap.LookupSymbol("Gen")) > 0 {
		t.Error("expected Gen to be excluded")
	}
}

func TestRepoMapGenerator_SignatureLength(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	// Function with long signature
	content := `package testpkg

func VeryLongFunction(a int, b string, c float64, d []string, e map[string]interface{}, f func(int, int) error, g context.Context, h time.Duration, i chan struct{}, j *SomeVeryLongTypeName) (error, context.Context, *VeryLongResult, []string, map[string]interface{}) {
	return nil, nil, nil, nil, nil
}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	config.MaxSignatureLen = 50
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	symbols := repoMap.LookupSymbol("VeryLongFunction")
	if len(symbols) == 0 {
		t.Fatal("expected to find VeryLongFunction")
	}
	
	sig := symbols[0].Signature
	if len(sig) > 53 { // max + "..."
		t.Errorf("expected signature to be truncated, got length %d: %s", len(sig), sig)
	}
}

func TestRepoMapGenerator_InterfaceMethods(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader
	Writer
}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	reader := repoMap.LookupSymbol("Reader")
	if len(reader) == 0 {
		t.Fatal("expected to find Reader")
	}
	
	if reader[0].Kind != SymbolInterface {
		t.Errorf("expected kind interface, got %s", reader[0].Kind)
	}
	
	readWriter := repoMap.LookupSymbol("ReadWriter")
	if len(readWriter) == 0 {
		t.Fatal("expected to find ReadWriter")
	}
}

func TestRepoMapGenerator_EmbeddedStructs(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

type Base struct {
	ID int
}

type Derived struct {
	Base
	Name string
}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	base := repoMap.LookupSymbol("Base")
	if len(base) == 0 {
		t.Fatal("expected to find Base")
	}
	
	derived := repoMap.LookupSymbol("Derived")
	if len(derived) == 0 {
		t.Fatal("expected to find Derived")
	}
}

func TestRepoMapGenerator_DocComments(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	
	content := `package testpkg

// TestFunc does something useful.
// It returns an error on failure.
func TestFunc() error { return nil }

// TestStruct is a test structure.
type TestStruct struct {
	// Field is a field.
	Field string
}
`
	os.WriteFile(goFile, []byte(content), 0644)
	
	config := DefaultRepoMapConfig()
	config.IncludeComments = true
	gen := NewRepoMapGenerator(config, nil)
	
	ctx := context.Background()
	repoMap, err := gen.Generate(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	
	fn := repoMap.LookupSymbol("TestFunc")
	if len(fn) == 0 {
		t.Fatal("expected to find TestFunc")
	}
	
	if !strings.Contains(fn[0].DocComment, "does something useful") {
		t.Errorf("expected doc comment, got: %s", fn[0].DocComment)
	}
}

func TestRepoMap_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	
	tmpDir := t.TempDir()
	
	// Create many files
	for i := 0; i < 100; i++ {
		goFile := filepath.Join(tmpDir, "file"+string(rune('a'+i%26))+string(rune('a'+i/26))+".go")
		os.WriteFile(goFile, []byte("package main\nfunc Func() {}"), 0644)
	}
	
	config := DefaultRepoMapConfig()
	gen := NewRepoMapGenerator(config, nil)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	repoMap, err := gen.Generate(ctx, tmpDir)
	
	// Should either succeed or return context error
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("unexpected error: %v", err)
	}
	
	if repoMap != nil {
		t.Logf("Generated repo map with %d files in %v", repoMap.TotalFiles, repoMap.Duration)
	}
}
