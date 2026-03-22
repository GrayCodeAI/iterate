package ui_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/GrayCodeAI/iterate/internal/ui"
)

// captureStdout captures stdout output during fn execution.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintSuccess_WritesToStdout(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintSuccess("operation complete")
	})
	if out == "" {
		t.Error("PrintSuccess should write to stdout")
	}
	if len(out) == 0 {
		t.Error("PrintSuccess should produce non-empty output")
	}
}

func TestPrintError_WritesToStdout(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintError("something went wrong")
	})
	if out == "" {
		t.Error("PrintError should write to stdout")
	}
}

func TestPrintDim_WritesToStdout(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintDim("dim message")
	})
	if out == "" {
		t.Error("PrintDim should write to stdout")
	}
}

func TestPrintSuccess_ContainsMessage(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintSuccess("hello world")
	})
	if !bytes.Contains([]byte(out), []byte("hello world")) {
		t.Errorf("PrintSuccess output should contain the message, got %q", out)
	}
}

func TestPrintError_ContainsMessage(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintError("bad input")
	})
	if !bytes.Contains([]byte(out), []byte("bad input")) {
		t.Errorf("PrintError output should contain the message, got %q", out)
	}
}

func TestPrintDim_ContainsMessage(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintDim("quiet note")
	})
	if !bytes.Contains([]byte(out), []byte("quiet note")) {
		t.Errorf("PrintDim output should contain the message, got %q", out)
	}
}

func TestPrintSuccess_WithFormat(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintSuccess("saved %d items", 42)
	})
	if !bytes.Contains([]byte(out), []byte("42")) {
		t.Errorf("PrintSuccess should format args, got %q", out)
	}
}

func TestPrintError_WithFormat(t *testing.T) {
	out := captureStdout(func() {
		ui.PrintError("failed with code %d", 500)
	})
	if !bytes.Contains([]byte(out), []byte("500")) {
		t.Errorf("PrintError should format args, got %q", out)
	}
}
