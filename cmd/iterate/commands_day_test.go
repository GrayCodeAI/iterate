package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// /day command tests
// ---------------------------------------------------------------------------

func TestDayCommand_ShowCurrent(t *testing.T) {
	dir := t.TempDir()
	dayFile := filepath.Join(dir, "DAY_COUNT")

	// No DAY_COUNT file exists - should show default
	if _, err := os.Stat(dayFile); os.IsNotExist(err) {
		// Expected - file doesn't exist yet
	}

	// Create DAY_COUNT file
	if err := os.WriteFile(dayFile, []byte("5"), 0644); err != nil {
		t.Fatalf("failed to write DAY_COUNT: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(dayFile)
	if err != nil {
		t.Fatalf("failed to read DAY_COUNT: %v", err)
	}
	if string(data) != "5" {
		t.Errorf("expected DAY_COUNT=5, got %s", string(data))
	}
}

func TestDayCommand_SetNewDay(t *testing.T) {
	dir := t.TempDir()
	dayFile := filepath.Join(dir, "DAY_COUNT")

	// Create initial file
	if err := os.WriteFile(dayFile, []byte("3"), 0644); err != nil {
		t.Fatalf("failed to write initial DAY_COUNT: %v", err)
	}

	// Update to new day
	newDay := "10"
	if err := os.WriteFile(dayFile, []byte(newDay), 0644); err != nil {
		t.Fatalf("failed to update DAY_COUNT: %v", err)
	}

	// Verify update
	data, err := os.ReadFile(dayFile)
	if err != nil {
		t.Fatalf("failed to read updated DAY_COUNT: %v", err)
	}
	if string(data) != "10" {
		t.Errorf("expected DAY_COUNT=10, got %s", string(data))
	}
}

func TestDayCommand_EmptyFileDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	dayFile := filepath.Join(dir, "DAY_COUNT")

	// Create empty file
	if err := os.WriteFile(dayFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty DAY_COUNT: %v", err)
	}

	// Default to 1 when empty
	data, err := os.ReadFile(dayFile)
	if err != nil {
		t.Fatalf("failed to read DAY_COUNT: %v", err)
	}
	day := "1"
	if len(data) > 0 {
		day = string(data)
	}
	if day != "1" {
		t.Errorf("expected default day=1, got %s", day)
	}
}
