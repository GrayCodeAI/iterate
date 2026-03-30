package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestAtomicWriteFile_RaceCondition tests concurrent atomic writes to the same file.
// This catches races where the defer cleanup removes the file after rename.
func TestAtomicWriteFile_RaceCondition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.json")

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Spawn 10 concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte(`{"writer":` + string(rune('0'+id)) + `}`)
			if err := atomicWriteFile(path, data, 0o644); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent write failed: %v", err)
	}

	// The file should still exist after all writes
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist after atomic writes but got: %v", err)
	}
}

// TestAtomicWriteFile_FileStillExists verifies the file isn't deleted by defer cleanup.
func TestAtomicWriteFile_FileStillExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.json")
	data := []byte(`{"test":"data"}`)

	// Write the file
	if err := atomicWriteFile(path, data, 0o644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// Force GC to trigger any deferred cleanups
	// If the defer is buggy, it might delete the file here
	for i := 0; i < 5; i++ {
		// Read file multiple times to ensure it persists
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("file disappeared after write (read %d): %v", i, err)
		}
		if string(got) != string(data) {
			t.Errorf("file content changed: want %q, got %q", data, got)
		}
	}
}
