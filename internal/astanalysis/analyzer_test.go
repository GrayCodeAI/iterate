package astanalysis

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestGoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

func TestNewAnalyzer(t *testing.T) {
	a := NewAnalyzer("/tmp/test")
	if a.RepoPath != "/tmp/test" {
		t.Errorf("expected RepoPath /tmp/test, got %s", a.RepoPath)
	}
	if a.Fset == nil {
		t.Error("expected non-nil Fset")
	}
}

func TestAnalyzeFile(t *testing.T) {
	dir := t.TempDir()
	content := `package mypkg

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	Name    string
	Timeout int
}

// Greeter defines the greeting interface
type Greeter interface {
	Greet(name string) string
}

var globalCounter int
const maxRetries = 3

// SayHello prints a greeting
func SayHello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func (c *Config) Validate() error {
	if c.Name == "" {
		return os.ErrInvalid
	}
	return nil
}
`
	path := setupTestGoFile(t, dir, "sample.go", content)

	a := NewAnalyzer(dir)
	info, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Check package name
	if info.Package != "mypkg" {
		t.Errorf("expected package mypkg, got %s", info.Package)
	}

	// Check imports
	if len(info.Imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(info.Imports))
	}

	// Check structs
	if len(info.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(info.Structs))
	}
	if info.Structs[0].Name != "Config" {
		t.Errorf("expected struct Config, got %s", info.Structs[0].Name)
	}
	if !info.Structs[0].IsExported {
		t.Error("expected Config to be exported")
	}
	if len(info.Structs[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(info.Structs[0].Fields))
	}

	// Check interfaces
	if len(info.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(info.Interfaces))
	}
	if info.Interfaces[0].Name != "Greeter" {
		t.Errorf("expected interface Greeter, got %s", info.Interfaces[0].Name)
	}
	if len(info.Interfaces[0].Methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(info.Interfaces[0].Methods))
	}

	// Check functions
	if len(info.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(info.Functions))
	}

	// Check SayHello
	var sayHello, validate *FunctionInfo
	for i := range info.Functions {
		if info.Functions[i].Name == "SayHello" {
			sayHello = &info.Functions[i]
		}
		if info.Functions[i].Name == "Validate" {
			validate = &info.Functions[i]
		}
	}

	if sayHello == nil {
		t.Fatal("expected SayHello function")
	}
	if !sayHello.IsExported {
		t.Error("expected SayHello to be exported")
	}
	if sayHello.Doc == "" {
		t.Error("expected doc comment for SayHello")
	}

	if validate == nil {
		t.Fatal("expected Validate method")
	}
	if validate.Receiver != "*Config" {
		t.Errorf("expected receiver *Config, got %s", validate.Receiver)
	}

	// Check variables
	if len(info.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(info.Variables))
	}
}

func TestAnalyzeFileInvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	path := setupTestGoFile(t, dir, "bad.go", "package broken\nfunc(")

	a := NewAnalyzer(dir)
	_, err := a.AnalyzeFile(path)
	if err == nil {
		t.Error("expected error for invalid Go syntax")
	}
}

func TestAnalyzeFileNotFound(t *testing.T) {
	a := NewAnalyzer("/tmp")
	_, err := a.AnalyzeFile("/tmp/nonexistent_file_12345.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestAnalyzePackage(t *testing.T) {
	dir := t.TempDir()

	// Create multiple Go files
	_ = setupTestGoFile(t, dir, "alpha.go", `package testpkg
func Alpha() string { return "alpha" }
`)
	_ = setupTestGoFile(t, dir, "beta.go", `package testpkg
func Beta() string { return "beta" }
`)
	// Create a test file that should be skipped
	_ = setupTestGoFile(t, dir, "types_test.go", `package testpkg
func TestSomething(t *testing.T) {}
`)

	a := NewAnalyzer(dir)
	files, err := a.AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("AnalyzePackage failed: %v", err)
	}

	// Should return 2 files (not the _test.go)
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Check that both files were analyzed
	names := make(map[string]bool)
	for _, f := range files {
		names[f.Package] = true
	}
	if !names["testpkg"] {
		t.Error("expected testpkg package")
	}
}

func TestAnalyzePackageSkipsVendor(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor", "example.com", "pkg")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}
	_ = setupTestGoFile(t, vendorDir, "vendor.go", `package vendorpkg
func VendorFunc() {}
`)
	_ = setupTestGoFile(t, dir, "main.go", `package main
func main() {}
`)

	a := NewAnalyzer(dir)
	files, err := a.AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("AnalyzePackage failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file (vendor skipped), got %d", len(files))
	}
}

func TestFindUnusedCode(t *testing.T) {
	dir := t.TempDir()
	_ = setupTestGoFile(t, dir, "code.go", `package mypkg
import "fmt"

type MyStruct struct{}

func usedFunc() { fmt.Println("used") }
func unusedFunc() { fmt.Println("unused") }

var usedVar = "test"
var unusedVar = "ignored"
`)

	a := NewAnalyzer(dir)
	unused, err := a.FindUnusedCode(dir)
	if err != nil {
		t.Fatalf("FindUnusedCode failed: %v", err)
	}

	// Should return unexported identifiers as potentially unused
	if len(unused) < 2 {
		t.Errorf("expected at least 2 potentially unused identifiers, got %d", len(unused))
	}

	// Verify the expected names are present
	found := make(map[string]bool)
	for _, name := range unused {
		found[name] = true
	}
	expected := []string{"usedFunc", "unusedFunc", "usedVar", "unusedVar"}
	for _, exp := range expected {
		if !found[exp] {
			t.Errorf("expected %q in unused list", exp)
		}
	}
}

func TestEmptyPackage(t *testing.T) {
	dir := t.TempDir()

	a := NewAnalyzer(dir)
	files, err := a.AnalyzePackage(dir)
	if err != nil {
		t.Fatalf("AnalyzePackage failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestExprToString(t *testing.T) {
	dir := t.TempDir()
	content := `package exprtest

type Complex struct {
	IntField    int
	PtrField    *int
	SliceField  []string
	MapField    map[string]int
	ChanField   chan bool
	IfaceField  interface{}
	FuncField   func(int) string
}
`
	path := setupTestGoFile(t, dir, "expr.go", content)

	a := NewAnalyzer(dir)
	info, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(info.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(info.Structs))
	}

	fields := make(map[string]string)
	for _, f := range info.Structs[0].Fields {
		fields[f.Name] = f.Type
	}

	expected := map[string]string{
		"IntField":   "int",
		"PtrField":   "*int",
		"SliceField": "[]string",
		"MapField":   "map[string]int",
		"ChanField":  "chan bool",
		"IfaceField": "interface{}",
		"FuncField":  "func(...)",
	}

	for name, expectedType := range expected {
		if actual := fields[name]; actual != expectedType {
			t.Errorf("field %s: expected type %q, got %q", name, expectedType, actual)
		}
	}
}

func TestConstVsVar(t *testing.T) {
	dir := t.TempDir()
	content := `package consttest

const MyConst = 42
var MyVar = "hello"
var UntypedVar = true
`
	path := setupTestGoFile(t, dir, "const.go", content)

	a := NewAnalyzer(dir)
	info, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	consts := 0
	vars := 0
	for _, v := range info.Variables {
		if v.IsConst {
			consts++
		} else {
			vars++
		}
	}

	if consts != 1 {
		t.Errorf("expected 1 const, got %d", consts)
	}
	if vars != 2 {
		t.Errorf("expected 2 vars, got %d", vars)
	}
}

func TestMethodsWithDifferentReceivers(t *testing.T) {
	dir := t.TempDir()
	content := `package rect
type Rectangle struct {
	Width  float64
	Height float64
}
func (r Rectangle) Area() float64 { return r.Width * r.Height }
func (r *Rectangle) Scale(f float64) { r.Width *= f; r.Height *= f }
`
	path := setupTestGoFile(t, dir, "rect.go", content)

	a := NewAnalyzer(dir)
	info, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(info.Functions) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(info.Functions))
	}

	for _, fn := range info.Functions {
		if fn.Name == "Area" && fn.Receiver != "Rectangle" {
			t.Errorf("Area: expected value receiver, got %q", fn.Receiver)
		}
		if fn.Name == "Scale" && fn.Receiver != "*Rectangle" {
			t.Errorf("Scale: expected pointer receiver, got %q", fn.Receiver)
		}
	}
}

func TestSelectorExprImport(t *testing.T) {
	dir := t.TempDir()
	content := `package seltest
import (
	"net/http"
	"os"
)
var Client http.Client
var Stdout *os.File
`
	path := setupTestGoFile(t, dir, "sel.go", content)

	a := NewAnalyzer(dir)
	info, err := a.AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(info.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(info.Variables))
	}

	typesMap := make(map[string]string)
	for _, v := range info.Variables {
		typesMap[v.Name] = v.Type
	}

	if typesMap["Client"] != "http.Client" {
		t.Errorf("expected http.Client, got %s", typesMap["Client"])
	}
	if typesMap["Stdout"] != "*os.File" {
		t.Errorf("expected *os.File, got %s", typesMap["Stdout"])
	}
}

func BenchmarkAnalyzeFile(b *testing.B) {
	dir := b.TempDir()
	content := `package bench
import (
	"fmt"
	"os"
	"strings"
	"net/http"
)
type S struct {
	A int
	B string
	C []byte
	D map[string]int
}
func (s *S) M1() {}
func (s *S) M2() {}
func (s *S) M3() {}
func Helper(x int) string { return fmt.Sprintf("%d", x) }
func Main() error { return os.ErrInvalid }
var v1, v2, v3 = 1, 2, 3
const C1, C2 = "a", "b"
`
	path := filepath.Join(dir, "bench.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatal(err)
	}
	a := NewAnalyzer(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := a.AnalyzeFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}
