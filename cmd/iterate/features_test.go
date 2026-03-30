package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindTodosNoDeferLeak(t *testing.T) {
	// Create a temp directory with many files to trigger the leak
	tmpDir := t.TempDir()

	// Create 100+ files to ensure we don't hit file descriptor limits
	for i := 0; i < 150; i++ {
		path := filepath.Join(tmpDir, "file", "sub", filepath.Join("deep", "nested", "path", "here"))
		_ = os.MkdirAll(path, 0o755)
		f, err := os.Create(filepath.Join(path, "test_"+string(rune('a'+i%26))+".go"))
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString("// TODO: fix this\n")
		f.Close()
	}

	// This should not exhaust file descriptors
	todos := findTodos(tmpDir)

	if len(todos) == 0 {
		t.Error("expected to find TODOs, found none")
	}
}
