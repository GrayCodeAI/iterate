package evolution

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
)

func newTestEngine() *Engine {
	return &Engine{
		repoPath: ".",
		logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}
}

func TestDefaultSafetyCheck(t *testing.T) {
	sc := DefaultSafetyCheck()
	if !sc.LintCheck {
		t.Error("LintCheck should be true by default")
	}
	if !sc.TestModificationCheck {
		t.Error("TestModificationCheck should be true by default")
	}
	if !sc.SmokeTestCheck {
		t.Error("SmokeTestCheck should be true by default")
	}
	if sc.RequireHumanReview {
		t.Error("RequireHumanReview should be false by default")
	}
}

func TestCheckTestModification(t *testing.T) {
	e := newTestEngine()

	// Test files are allowed now (warning only)
	files := []string{"internal/agent/handler_test.go"}
	blocked, _ := e.checkTestModification(files)
	if blocked {
		t.Error("test file changes should not be blocked anymore")
	}

	// Mix of test and non-test files
	files = []string{"internal/agent/handler.go", "internal/agent/handler_test.go"}
	blocked, _ = e.checkTestModification(files)
	if blocked {
		t.Error("mixed files should not be blocked")
	}

	// No test files - clean pass
	files = []string{"internal/agent/handler.go", "cmd/main.go"}
	blocked, _ = e.checkTestModification(files)
	if blocked {
		t.Error("non-test files should not be blocked")
	}
}

func TestCheckDiffSize(t *testing.T) {
	e := newTestEngine()

	// Small diff should pass
	files := make([]string, 5)
	for i := range files {
		files[i] = fmt.Sprintf("file%d.go", i)
	}
	blocked, _ := e.checkDiffSize(context.Background(), files)
	if blocked {
		t.Error("small diff should not be blocked")
	}

	// Large diff should block
	files = make([]string, 21)
	for i := range files {
		files[i] = fmt.Sprintf("file%d.go", i)
	}
	blocked, _ = e.checkDiffSize(context.Background(), files)
	if !blocked {
		t.Error("large diff (>20 files) should be blocked")
	}
}

func TestCheckSensitiveFiles(t *testing.T) {
	e := newTestEngine()

	// Clean files should pass
	files := []string{"internal/agent/handler.go", "cmd/main.go"}
	blocked, _ := e.checkSensitiveFiles(files)
	if blocked {
		t.Error("normal files should not trigger sensitive check")
	}

	// Sensitive files should be blocked
	sensitiveFiles := [][]string{
		{".github/workflows/ci.yml"},
		{"docs/IDENTITY.md"},
		{".env"},
		{"config/credentials.json"},
		{"scripts/evolution/run.sh"},
		{"secrets/password.txt"},
	}

	for _, sf := range sensitiveFiles {
		blocked, reason := e.checkSensitiveFiles(sf)
		if !blocked {
			t.Errorf("sensitive file %s should be blocked", sf[0])
		}
		if reason == "" {
			t.Errorf("sensitive file %s should have a reason", sf[0])
		}
	}
}

func TestCheckSensitiveFilesCaseInsensitive(t *testing.T) {
	e := newTestEngine()

	files := []string{"config/SECRET_KEY.json"}
	blocked, _ := e.checkSensitiveFiles(files)
	if !blocked {
		t.Error("case-insensitive sensitive match should block SECRET_KEY")
	}
}
