package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolStyle_Bash(t *testing.T) {
	icon, label, col := toolStyle("bash")
	if icon != "❯" {
		t.Errorf("expected bash icon, got %q", icon)
	}
	if label != "bash" {
		t.Errorf("expected bash label, got %q", label)
	}
	if col == "" {
		t.Error("expected non-empty color")
	}
}

func TestToolStyle_ReadFile(t *testing.T) {
	icon, label, _ := toolStyle("read_file")
	if icon != "◎" {
		t.Errorf("expected read icon, got %q", icon)
	}
	if label != "read" {
		t.Errorf("expected read label, got %q", label)
	}
}

func TestToolStyle_WriteFile(t *testing.T) {
	_, label, _ := toolStyle("write_file")
	if label != "write" {
		t.Errorf("expected write label, got %q", label)
	}
}

func TestToolStyle_EditFile(t *testing.T) {
	_, label, _ := toolStyle("edit_file")
	if label != "edit" {
		t.Errorf("expected edit label, got %q", label)
	}
}

func TestToolStyle_SearchFiles(t *testing.T) {
	_, label, _ := toolStyle("search_files")
	if label != "search" {
		t.Errorf("expected search label, got %q", label)
	}
}

func TestToolStyle_WebFetch(t *testing.T) {
	_, label, _ := toolStyle("web_fetch")
	if label != "fetch" {
		t.Errorf("expected fetch label, got %q", label)
	}
}

func TestToolStyle_DeleteFile(t *testing.T) {
	_, label, _ := toolStyle("delete_file")
	if label != "delete" {
		t.Errorf("expected delete label, got %q", label)
	}
}

func TestToolStyle_Default(t *testing.T) {
	icon, label, _ := toolStyle("unknown_tool")
	if icon != "⚙" {
		t.Errorf("expected default icon, got %q", icon)
	}
	if label != "unknown_tool" {
		t.Errorf("expected tool name as label, got %q", label)
	}
}

func TestToolStyle_GitCommit(t *testing.T) {
	_, label, _ := toolStyle("git_commit")
	if label != "git_commit" {
		t.Errorf("expected git_commit label, got %q", label)
	}
}

func TestToolStyle_RunCommand(t *testing.T) {
	_, label, _ := toolStyle("run_command")
	if label != "bash" {
		t.Errorf("expected run_command to map to bash, got %q", label)
	}
}

func TestToolStyle_RunTerminalCmd(t *testing.T) {
	_, label, _ := toolStyle("run_terminal_cmd")
	if label != "bash" {
		t.Errorf("expected run_terminal_cmd to map to bash, got %q", label)
	}
}

func TestToolStyle_ListDir(t *testing.T) {
	_, label, _ := toolStyle("list_dir")
	if label != "ls" {
		t.Errorf("expected list_dir to map to ls, got %q", label)
	}
}

func TestFormatToolCallResult_Short(t *testing.T) {
	result := formatToolCallResult("ok", 100*time.Millisecond)
	if !strings.Contains(result, "ok") {
		t.Errorf("should contain result text, got %q", result)
	}
	if !strings.Contains(result, "100ms") {
		t.Errorf("should contain elapsed time, got %q", result)
	}
}

func TestFormatToolCallResult_Long(t *testing.T) {
	longResult := strings.Repeat("a", 100)
	result := formatToolCallResult(longResult, 50*time.Millisecond)
	if !strings.Contains(result, "…") {
		t.Errorf("should truncate long results, got %q", result)
	}
}

func TestFormatToolCallResult_Error(t *testing.T) {
	result := formatToolCallResult("Error: something went wrong", 5*time.Millisecond)
	if !strings.Contains(result, "Error") {
		t.Errorf("should contain error text, got %q", result)
	}
}

func TestFormatToolCallResult_NewlinesReplaced(t *testing.T) {
	result := formatToolCallResult("line1\nline2\nline3", 10*time.Millisecond)
	if strings.Contains(result, "\nline2") {
		t.Errorf("should replace newlines with spaces in snippet")
	}
}

func TestFormatToolCallResult_ZeroDuration(t *testing.T) {
	result := formatToolCallResult("fast", 0)
	if !strings.Contains(result, "fast") {
		t.Errorf("should contain result, got %q", result)
	}
}

// TestInjectAtFileContext_NoDeferLeak verifies that file handles are properly
// closed after reading, not leaked via defer in a loop.
// This was a real bug where defer f.Close() inside a loop caused file descriptor leaks.
// See: https://github.com/GrayCodeAI/iterate/pull/8
func TestInjectAtFileContext_NoDeferLeak(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create multiple test files
	for i := 0; i < 10; i++ {
		file := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".txt")
		content := "line1\nline2\nline3\n"
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Test with multiple @file references in prompt
	prompt := "Check these files: @testa.txt @testb.txt @testc.txt @testd.txt @teste.txt @testf.txt @testg.txt @testh.txt @testi.txt @testj.txt"

	// This should not leak file descriptors
	result := injectAtFileContext(prompt, tmpDir)

	// Verify files were injected
	if result == prompt {
		t.Error("Expected file content to be injected into prompt")
	}

	// Verify all file references are in result
	for i := 0; i < 10; i++ {
		filename := "test" + string(rune('a'+i)) + ".txt"
		if !strings.Contains(result, filename) {
			t.Errorf("Expected result to contain reference to %s", filename)
		}
	}
}
