package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TDDPhase string

const (
	TDDPhaseWriteTest TDDPhase = "write_test"
	TDDPhaseRunTest   TDDPhase = "run_test"
	TDDPhaseWriteCode TDDPhase = "write_code"
	TDDPhaseVerify    TDDPhase = "verify"
)

type TDDResult struct {
	Phase    TDDPhase
	Success  bool
	TestFile string
	CodeFile string
	Output   string
	Error    error
}

type TDDEngine struct {
	engine *Engine
}

func NewTDDEngine(e *Engine) *TDDEngine {
	return &TDDEngine{engine: e}
}

func (t *TDDEngine) ExecuteTask(ctx context.Context, task string) *TDDResult {
	result := &TDDResult{}

	testFile := t.findTestFileForTask(task)
	if testFile == "" {
		testFile = t.suggestTestFileName(task)
	}

	result.TestFile = testFile

	testResult := t.writeFailingTest(ctx, testFile, task)
	if !testResult.Success {
		return testResult
	}

	runResult := t.runTest(ctx, testFile)
	if runResult.Success {
		result.Phase = TDDPhaseRunTest
		result.Success = false
		result.Error = fmt.Errorf("test passed unexpectedly - it should fail before the fix")
		return result
	}

	codeResult := t.writeCodeFix(ctx, testFile, task)
	if !codeResult.Success {
		return codeResult
	}

	result.CodeFile = codeResult.CodeFile

	verifyResult := t.runTest(ctx, testFile)
	if !verifyResult.Success {
		result.Phase = TDDPhaseVerify
		result.Success = false
		result.Error = fmt.Errorf("test still failing after code fix: %s", verifyResult.Output)
		return result
	}

	result.Phase = TDDPhaseVerify
	result.Success = true

	return result
}

func (t *TDDEngine) writeFailingTest(ctx context.Context, testFile string, task string) *TDDResult {
	result := &TDDResult{
		Phase:    TDDPhaseWriteTest,
		TestFile: testFile,
	}

	testContent := t.generateTestTemplate(testFile, task)

	absPath := filepath.Join(t.engine.repoPath, testFile)
	dir := filepath.Dir(absPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to create directory: %w", err)
		return result
	}

	if err := os.WriteFile(absPath, []byte(testContent), 0644); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to write test file: %w", err)
		return result
	}

	result.Success = true

	runResult := t.runTest(ctx, testFile)
	if runResult.Success {
		result.Success = false
		result.Error = fmt.Errorf("test passed - should fail before fix is applied")
	}

	return result
}

func (t *TDDEngine) runTest(ctx context.Context, testFile string) *TDDResult {
	result := &TDDResult{
		Phase: TDDPhaseRunTest,
	}

	testName := extractTestName(testFile)
	if testName == "" {
		result.Success = false
		result.Error = fmt.Errorf("could not extract test name from %s", testFile)
		return result
	}

	output, err := t.engine.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("go test -v -run %s ./...", testName),
	})
	result.Output = output

	if err != nil {
		result.Success = false
		result.Error = err
	} else {
		result.Success = strings.Contains(output, "PASS") || strings.Contains(output, "ok")
	}

	return result
}

func (t *TDDEngine) writeCodeFix(ctx context.Context, testFile string, task string) *TDDResult {
	result := &TDDResult{
		Phase:    TDDPhaseWriteCode,
		TestFile: testFile,
	}

	codeFile := testFileToCodeFile(testFile)
	if codeFile == "" {
		result.Success = false
		result.Error = fmt.Errorf("could not determine code file for test %s", testFile)
		return result
	}

	result.CodeFile = codeFile

	// Generate the code fix based on the task description
	codeContent := t.generateCodeFix(codeFile, task)

	absPath := filepath.Join(t.engine.repoPath, codeFile)
	dir := filepath.Dir(absPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to create directory: %w", err)
		return result
	}

	if err := os.WriteFile(absPath, []byte(codeContent), 0644); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to write code file: %w", err)
		return result
	}

	result.Success = true
	return result
}

func (t *TDDEngine) generateCodeFix(codeFile, task string) string {
	taskLower := strings.ToLower(task)
	packageName := "main"
	if strings.HasSuffix(codeFile, "_test.go") {
		packageName = extractPackageFromTest(codeFile)
	}

	funcName := extractFunctionName(task)

	// Generate appropriate code based on common task patterns
	if strings.Contains(taskLower, "fix") && strings.Contains(taskLower, "panic") {
		return t.generatePanicFix(packageName, funcName)
	}
	if strings.Contains(taskLower, "fix") && strings.Contains(taskLower, "defer") {
		return t.generateDeferFix(packageName, funcName)
	}
	if strings.Contains(taskLower, "fix") && strings.Contains(taskLower, "error") {
		return t.generateErrorHandling(packageName, funcName)
	}
	if strings.Contains(taskLower, "null pointer") || strings.Contains(taskLower, "nil") {
		return t.generateNilCheck(packageName, funcName)
	}
	if strings.Contains(taskLower, "timeout") {
		return t.generateTimeoutHandling(packageName, funcName)
	}
	if strings.Contains(taskLower, "resource leak") || strings.Contains(taskLower, "close") {
		return t.generateResourceCloseFix(packageName, funcName)
	}

	// Default: generate a basic implementation
	return fmt.Sprintf(`package %s

// %s handles the following task:
// %s
// TODO: Replace with actual implementation
func %s() {
	panic("not implemented")
}
`, packageName, funcName, task, funcName)
}

func (t *TDDEngine) generatePanicFix(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

// %s with bounds checking to prevent panics
func %s(slice []int, index int) int {
	if index < 0 || index >= len(slice) {
		return -1
	}
	return slice[index]
}
`, packageName, funcName, funcName)
}

func (t *TDDEngine) generateDeferFix(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

// %s with proper defer placement (after error check)
func %s(path string) (string, error) {
	f, err := openFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := readFile(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
`, packageName, funcName, funcName)
}

func (t *TDDEngine) generateErrorHandling(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

// %s with proper error handling
func %s() error {
	result, err := doSomething()
	if err != nil {
		return fmt.Errorf("%s failed: %%w", err)
	}

	if result == nil {
		return fmt.Errorf("%s: unexpected nil result")
	}

	return nil
}
`, packageName, funcName, funcName, funcName, funcName)
}

func (t *TDDEngine) generateNilCheck(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

// %s with nil pointer checks
func %s(input *SomeType) (*SomeType, error) {
	if input == nil {
		return nil, fmt.Errorf("%s: input cannot be nil")
	}

	result := &SomeType{
		Field: input.Field,
	}

	if result.Field == "" {
		return nil, fmt.Errorf("%s: field cannot be empty")
	}

	return result, nil
}
`, packageName, funcName, funcName, funcName, funcName)
}

func (t *TDDEngine) generateTimeoutHandling(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

import "context"

// %s with context timeout handling
func %s(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	result, err := doWork(ctx)
	if err != nil {
		return "", fmt.Errorf("%s failed: %%w", err)
	}

	return result, nil
}
`, packageName, funcName, funcName, funcName)
}

func (t *TDDEngine) generateResourceCloseFix(packageName, funcName string) string {
	return fmt.Sprintf(`package %s

// %s with proper resource cleanup
func %s(path string) error {
	f, err := openResource(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	return process(f)
}
`, packageName, funcName, funcName)
}

func extractPackageFromTest(testFile string) string {
	return "evolution"
}

func extractFunctionName(task string) string {
	words := strings.Fields(task)
	if len(words) == 0 {
		return "HandleTask"
	}
	name := words[0]
	for _, w := range words[1:] {
		if len(w) > 0 {
			name += strings.ToUpper(w[:1]) + w[1:]
		}
	}
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

func (t *TDDEngine) findTestFileForTask(task string) string {
	taskLower := strings.ToLower(task)

	for _, pattern := range []string{"fix", "bug", "error", "fail", "issue"} {
		if strings.Contains(taskLower, pattern) {
			return fmt.Sprintf("internal/evolution/task_fix_%s_test.go", pattern)
		}
	}

	return ""
}

func (t *TDDEngine) suggestTestFileName(task string) string {
	words := strings.Fields(task)
	if len(words) == 0 {
		return "internal/evolution/task_test.go"
	}

	name := words[0]
	for _, w := range words[1:] {
		if len(name) > 20 {
			break
		}
		name += strings.ToUpper(w[:1]) + w[1:]
	}

	return fmt.Sprintf("internal/evolution/task_%s_test.go", strings.ToLower(name))
}

func (t *TDDEngine) generateTestTemplate(testFile, task string) string {
	return fmt.Sprintf(`package evolution

import (
	"testing"
)

func Test%s(task) {
	// Test for: %s
	// This test should FAIL before the fix is applied
	
	t.Fatal("Test not implemented - write test that reproduces the bug")
}
`, extractTestName(testFile), task)
}

func extractTestName(file string) string {
	base := filepath.Base(file)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.TrimPrefix(name, "Test")
	return "Test" + name
}

func testFileToCodeFile(testFile string) string {
	testSuffix := "_test.go"
	if !strings.HasSuffix(testFile, testSuffix) {
		return ""
	}
	return strings.TrimSuffix(testFile, testSuffix) + ".go"
}

type TDDVerification struct {
	RequireTest        bool
	RequireFailingTest bool
	RequirePassingTest bool
}

var DefaultTDDVerification = TDDVerification{
	RequireTest:        true,
	RequireFailingTest: true,
	RequirePassingTest: true,
}

func (t *TDDEngine) Verify(result *TDDResult) error {
	verification := DefaultTDDVerification

	if verification.RequireTest && result.TestFile == "" {
		return fmt.Errorf("no test file created")
	}

	if verification.RequireFailingTest && result.Phase != TDDPhaseRunTest {
		return fmt.Errorf("test was not run to verify it fails")
	}

	if verification.RequirePassingTest && !result.Success {
		return fmt.Errorf("verification failed: %w", result.Error)
	}

	return nil
}
