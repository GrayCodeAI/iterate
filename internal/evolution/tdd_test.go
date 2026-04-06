package evolution

import (
	"strings"
	"testing"
)

func TestGenerateCodeFix_PanicFix(t *testing.T) {
	engine := &TDDEngine{}

	// Verify panic fix task generates bounds-checking code
	code := engine.generateCodeFix("safe.go", "fix panic on bad index")

	if !strings.Contains(code, "package") {
		t.Error("generated code missing package declaration")
	}
	if !strings.Contains(code, "index < 0") {
		t.Error("panic fix should check for negative index")
	}
	if !strings.Contains(code, "len(slice)") {
		t.Error("panic fix should check slice bounds")
	}
}

func TestGenerateCodeFix_DeferFix(t *testing.T) {
	engine := &TDDEngine{}

	code := engine.generateCodeFix("file.go", "fix defer before error check")

	if !strings.Contains(code, "defer f.Close()") {
		t.Error("defer fix should include proper defer placement")
	}
	if !strings.Contains(code, "if err != nil") {
		t.Error("defer fix should include error check before defer")
	}
}

func TestGenerateCodeFix_ErrorHandling(t *testing.T) {
	engine := &TDDEngine{}

	code := engine.generateCodeFix("handler.go", "fix error handling in handler")

	if !strings.Contains(code, "fmt.Errorf") {
		t.Error("error handling should wrap errors")
	}
	if !strings.Contains(code, "result == nil") {
		t.Error("error handling should check for nil result")
	}
}

func TestGenerateCodeFix_NilCheck(t *testing.T) {
	engine := &TDDEngine{}

	code := engine.generateCodeFix("nil.go", "fix null pointer dereference")

	if !strings.Contains(code, "input == nil") {
		t.Error("nil fix should check for nil input")
	}
	if !strings.Contains(code, "cannot be nil") {
		t.Error("nil fix should return descriptive error")
	}
}

func TestGenerateCodeFix_Timeout(t *testing.T) {
	engine := &TDDEngine{}

	code := engine.generateCodeFix("timeout.go", "add timeout handling")

	if !strings.Contains(code, `"context"`) {
		t.Error("timeout fix should import context")
	}
	if !strings.Contains(code, "context.WithTimeout") {
		t.Error("timeout fix should use WithTimeout")
	}
	if !strings.Contains(code, "defer cancel()") {
		t.Error("timeout fix should defer cancel()")
	}
}

func TestGenerateCodeFix_ResourceClose(t *testing.T) {
	engine := &TDDEngine{}

	code := engine.generateCodeFix("resource.go", "fix resource leak - close properly")

	if !strings.Contains(code, "defer func()") {
		t.Error("resource fix should use deferred closure for cleanup")
	}
	if !strings.Contains(code, "f.Close()") {
		t.Error("resource fix should call Close()")
	}
	if !strings.Contains(code, "closeErr") {
		t.Error("resource fix should capture close errors")
	}
}

func TestGenerateCodeFix_Default(t *testing.T) {
	engine := &TDDEngine{}

	// Task that doesn't match any known pattern should use default
	code := engine.generateCodeFix("unknown.go", "implement new feature")

	if !strings.Contains(code, "not yet implemented") {
		t.Error("default code generation should return error stub")
	}
	if !strings.Contains(code, "TODO") {
		t.Error("default code should include TODO comment")
	}
	// Should NOT contain panic — we return an error instead.
	if strings.Contains(code, "panic") {
		t.Error("default code should not contain panic")
	}
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "HandleTask"},
		{"fix bug", "fixBug"},
		{"add timeout handling", "addTimeoutHandling"},
		{"fix nil pointer in parser", "fixNilPointerInParser"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractFunctionName(tt.input)
			if result != tt.expected {
				t.Errorf("extractFunctionName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractFunctionName_Truncation(t *testing.T) {
	longInput := strings.Repeat("a ", 30) // Will produce a name > 40 chars
	result := extractFunctionName(longInput)
	if len(result) > 40 {
		t.Errorf("extractFunctionName should truncate to 40 chars, got %d chars", len(result))
	}
}

func TestExtractPackageFromTest(t *testing.T) {
	pkg := extractPackageFromTest("internal/evolution/some_test.go")
	if pkg != "evolution" {
		t.Errorf("expected package 'evolution', got %q", pkg)
	}
}

func TestTestFileToCodeFile(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"internal/evolution/parse_test.go", "internal/evolution/parse.go"},
		{"foo/bar_test.go", "foo/bar.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := testFileToCodeFile(tt.input)
			if result != tt.expected {
				t.Errorf("testFileToCodeFile(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTestFileToCodeFile_NotTest(t *testing.T) {
	result := testFileToCodeFile("source.go")
	if result != "" {
		t.Errorf("expected empty string for non-test file, got %q", result)
	}
}

func TestFindTestFileForTask(t *testing.T) {
	engine := &TDDEngine{}

	tests := []struct {
		task       string
		shouldFind bool
	}{
		{"fix bug in parser", true},
		{"handle error case", true},
		{"fix nil pointer issue", true},
		{"resolve failure in build", true},
		{"address github issue", true},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			result := engine.findTestFileForTask(tt.task)
			found := result != ""
			if found != tt.shouldFind {
				t.Errorf("findTestFileForTask(%q) found=%v, want %v", tt.task, found, tt.shouldFind)
			}
		})
	}
}

func TestSuggestTestFileName(t *testing.T) {
	engine := &TDDEngine{}

	tests := []struct {
		task     string
		expected string
	}{
		{"", "internal/evolution/task_test.go"},
		{"fix parser bugs", "internal/evolution/task_parser_test.go"},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			result := engine.suggestTestFileName(tt.task)
			if !strings.HasSuffix(result, "_test.go") {
				t.Errorf("suggested file should end with _test.go, got %q", result)
			}
			if tt.task == "" && result != tt.expected {
				t.Errorf("expected %q for empty task, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateTestTemplate(t *testing.T) {
	engine := &TDDEngine{}

	template := engine.generateTestTemplate("internal/evolution/parser_test.go", "fix parser bug")

	if !strings.Contains(template, "package evolution") {
		t.Error("test template should include package declaration")
	}
	if !strings.Contains(template, "testing") {
		t.Error("test template should import testing")
	}
	if !strings.Contains(template, "fix parser bug") {
		t.Error("test template should include task description")
	}
	if !strings.Contains(template, "t.Fatal") {
		t.Error("test template should include failing assertion")
	}
}

func TestGenerateCodeFix_CaseInsensitive(t *testing.T) {
	engine := &TDDEngine{}

	// All these should trigger panic fix (case insensitive)
	tasks := []string{
		"FIX PANIC on index",
		"Fix Panic in slice",
		"fix panic",
	}

	for _, task := range tasks {
		t.Run(task, func(t *testing.T) {
			code := engine.generateCodeFix("test.go", task)
			if !strings.Contains(code, "index < 0") {
				t.Errorf("case insensitive match failed for task %q", task)
			}
		})
	}
}

func TestTDDPhase_Values(t *testing.T) {
	phases := []TDDPhase{TDDPhaseWriteTest, TDDPhaseRunTest, TDDPhaseWriteCode, TDDPhaseVerify}
	expected := []string{"write_test", "run_test", "write_code", "verify"}

	for i, phase := range phases {
		if string(phase) != expected[i] {
			t.Errorf("TDDPhase %d = %q, want %q", i, phase, expected[i])
		}
	}
}

func TestTDDVerification_Default(t *testing.T) {
	if !DefaultTDDVerification.RequireTest {
		t.Error("RequireTest should be true by default")
	}
	if !DefaultTDDVerification.RequireFailingTest {
		t.Error("RequireFailingTest should be true by default")
	}
	if !DefaultTDDVerification.RequirePassingTest {
		t.Error("RequirePassingTest should be true by default")
	}
}
