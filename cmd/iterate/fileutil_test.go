package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := []byte(`{"key":"value"}`)

	if err := atomicWriteFile(path, data, 0o644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("want %q, got %q", data, got)
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write initial content.
	if err := atomicWriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Overwrite with new content.
	if err := atomicWriteFile(path, []byte("updated"), 0o644); err != nil {
		t.Fatalf("overwrite failed: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "updated" {
		t.Errorf("want %q, got %q", "updated", got)
	}
}

func TestAtomicWriteFile_NoTempFilesLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := atomicWriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// No .tmp- files should remain in the directory.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_PermissionSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := atomicWriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("want perm 0600, got %04o", info.Mode().Perm())
	}
}
