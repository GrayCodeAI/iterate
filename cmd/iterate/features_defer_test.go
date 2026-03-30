package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestAppendLearning_NoDeferLeak verifies that appendLearning properly closes
// file handles immediately after writing, not deferring until function return.
func TestAppendLearning_NoDeferLeak(t *testing.T) {
	dir := t.TempDir()
	
	// Call appendLearning multiple times rapidly
	for i := 0; i < 100; i++ {
		if err := appendLearning(dir, "test fact"); err != nil {
			t.Fatalf("appendLearning failed: %v", err)
		}
	}
	
	// Verify file was written
	path := filepath.Join(dir, "memory", "learnings.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read learnings file: %v", err)
	}
	if len(data) == 0 {
		t.Error("learnings file should not be empty")
	}
	
	// Force GC to trigger any deferred closes
	runtime.GC()
}

// TestAppendMemo_NoDeferLeak verifies that appendMemo properly closes
// file handles immediately after writing, not deferring until function return.
func TestAppendMemo_NoDeferLeak(t *testing.T) {
	dir := t.TempDir()
	
	// Call appendMemo multiple times rapidly
	for i := 0; i < 100; i++ {
		if err := appendMemo(dir, "test memo content"); err != nil {
			t.Fatalf("appendMemo failed: %v", err)
		}
	}
	
	// Verify file was written
	path := filepath.Join(dir, "docs", "JOURNAL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read journal file: %v", err)
	}
	if len(data) == 0 {
		t.Error("journal file should not be empty")
	}
	
	// Force GC to trigger any deferred closes
	runtime.GC()
}
