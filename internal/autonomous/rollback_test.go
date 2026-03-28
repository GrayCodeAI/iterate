// Package autonomous - Task 9: Rollback Stack tests
package autonomous

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRollbackConfig(t *testing.T) {
	config := DefaultRollbackConfig()

	if config.MaxEntries != 50 {
		t.Errorf("Expected MaxEntries 50, got %d", config.MaxEntries)
	}
}

func TestNewRollbackStack(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	if rs == nil {
		t.Fatal("Expected non-nil rollback stack")
	}
	if rs.maxEntries != 50 {
		t.Errorf("Expected maxEntries 50, got %d", rs.maxEntries)
	}
}

func TestPushFileEdit(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.PushFileEdit("test.go", "original content")

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}
	if entry.Type != RollbackTypeFileEdit {
		t.Errorf("Expected type file_edit, got %s", entry.Type)
	}
	if entry.Path != "test.go" {
		t.Errorf("Expected path 'test.go', got %s", entry.Path)
	}
	if entry.Original != "original content" {
		t.Errorf("Expected original content, got %s", entry.Original)
	}

	if rs.Size() != 1 {
		t.Errorf("Expected size 1, got %d", rs.Size())
	}
}

func TestPushFileCreate(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.PushFileCreate("new_file.go")

	if entry.Type != RollbackTypeFileCreate {
		t.Errorf("Expected type file_create, got %s", entry.Type)
	}
	if entry.Path != "new_file.go" {
		t.Errorf("Expected path 'new_file.go', got %s", entry.Path)
	}
}

func TestPushFileDelete(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.PushFileDelete("old_file.go", "content to restore")

	if entry.Type != RollbackTypeFileDelete {
		t.Errorf("Expected type file_delete, got %s", entry.Type)
	}
	if entry.Original != "content to restore" {
		t.Errorf("Expected original content, got %s", entry.Original)
	}
}

func TestPushGitCommit(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.PushGitCommit("abc123")

	if entry.Type != RollbackTypeGitCommit {
		t.Errorf("Expected type git_commit, got %s", entry.Type)
	}
	if entry.CommitHash != "abc123" {
		t.Errorf("Expected commit hash 'abc123', got %s", entry.CommitHash)
	}
}

func TestPop(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	rs.PushFileEdit("file1.go", "content1")
	rs.PushFileEdit("file2.go", "content2")

	entry := rs.Pop()

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}
	if entry.Path != "file2.go" {
		t.Errorf("Expected 'file2.go', got %s", entry.Path)
	}

	if rs.Size() != 1 {
		t.Errorf("Expected size 1 after pop, got %d", rs.Size())
	}
}

func TestPopEmpty(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.Pop()

	if entry != nil {
		t.Error("Expected nil for empty stack")
	}
}

func TestPeek(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	rs.PushFileEdit("file.go", "content")

	entry := rs.Peek()

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}
	if rs.Size() != 1 {
		t.Errorf("Peek should not remove entry, size should be 1, got %d", rs.Size())
	}
}

func TestRollbackFileEdit(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("modified content"), 0644)

	config := RollbackConfig{MaxEntries: 10, BaseDir: tmpDir}
	rs := NewRollbackStack(config)

	rs.PushFileEdit(testFile, "original content")

	err := rs.Rollback()
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify content restored
	data, _ := os.ReadFile(testFile)
	if string(data) != "original content" {
		t.Errorf("Expected 'original content', got %s", string(data))
	}
}

func TestRollbackFileCreate(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "new_file.go")
	os.WriteFile(testFile, []byte("new content"), 0644)

	rs := NewRollbackStack(RollbackConfig{MaxEntries: 10})
	rs.PushFileCreate(testFile)

	err := rs.Rollback()
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// File should be deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}
}

func TestRollbackFileDelete(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_test")
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "deleted.go")
	// File doesn't exist initially

	config := RollbackConfig{MaxEntries: 10, BaseDir: tmpDir}
	rs := NewRollbackStack(config)

	rs.PushFileDelete(testFile, "restored content")

	err := rs.Rollback()
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// File should be restored
	data, _ := os.ReadFile(testFile)
	if string(data) != "restored content" {
		t.Errorf("Expected 'restored content', got %s", string(data))
	}
}

func TestRollbackTo(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_test")
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	file3 := filepath.Join(tmpDir, "file3.go")

	os.WriteFile(file1, []byte("modified1"), 0644)
	os.WriteFile(file2, []byte("modified2"), 0644)
	os.WriteFile(file3, []byte("modified3"), 0644)

	rs := NewRollbackStack(RollbackConfig{MaxEntries: 10})
	rs.PushFileEdit(file1, "original1")
	entry2 := rs.PushFileEdit(file2, "original2")
	rs.PushFileEdit(file3, "original3")

	// Rollback to entry2 (should rollback entry3 and entry2)
	err := rs.RollbackTo(entry2.ID)
	if err != nil {
		t.Fatalf("RollbackTo failed: %v", err)
	}

	// file1 should still be modified
	data1, _ := os.ReadFile(file1)
	if string(data1) != "modified1" {
		t.Error("file1 should not be rolled back")
	}

	// file2 and file3 should be restored
	data2, _ := os.ReadFile(file2)
	if string(data2) != "original2" {
		t.Errorf("Expected file2 restored, got %s", string(data2))
	}

	data3, _ := os.ReadFile(file3)
	if string(data3) != "original3" {
		t.Errorf("Expected file3 restored, got %s", string(data3))
	}

	if rs.Size() != 1 {
		t.Errorf("Expected size 1 after rollbackTo, got %d", rs.Size())
	}
}

func TestRollbackAll(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_test")
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")

	os.WriteFile(file1, []byte("modified1"), 0644)
	os.WriteFile(file2, []byte("modified2"), 0644)

	rs := NewRollbackStack(RollbackConfig{MaxEntries: 10, BaseDir: tmpDir})
	rs.PushFileEdit(file1, "original1")
	rs.PushFileEdit(file2, "original2")

	err := rs.RollbackAll()
	if err != nil {
		t.Fatalf("RollbackAll failed: %v", err)
	}

	if rs.Size() != 0 {
		t.Errorf("Expected size 0 after rollbackAll, got %d", rs.Size())
	}

	// Both files should be restored
	data1, _ := os.ReadFile(file1)
	if string(data1) != "original1" {
		t.Error("file1 not restored")
	}

	data2, _ := os.ReadFile(file2)
	if string(data2) != "original2" {
		t.Error("file2 not restored")
	}
}

func TestMaxEntries(t *testing.T) {
	rs := NewRollbackStack(RollbackConfig{MaxEntries: 3})

	rs.PushFileEdit("file1.go", "c1")
	rs.PushFileEdit("file2.go", "c2")
	rs.PushFileEdit("file3.go", "c3")
	rs.PushFileEdit("file4.go", "c4")
	rs.PushFileEdit("file5.go", "c5")

	// Should only keep last 3
	if rs.Size() != 3 {
		t.Errorf("Expected size 3, got %d", rs.Size())
	}

	entries := rs.GetEntries()
	if entries[0].Path != "file3.go" {
		t.Errorf("Expected first entry to be file3.go, got %s", entries[0].Path)
	}
}

func TestGetEntry(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	entry := rs.PushFileEdit("test.go", "content")

	found := rs.GetEntry(entry.ID)
	if found == nil {
		t.Fatal("Expected to find entry")
	}
	if found.ID != entry.ID {
		t.Error("Entry ID mismatch")
	}

	notFound := rs.GetEntry("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for nonexistent entry")
	}
}

func TestCanRollback(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	if rs.CanRollback() {
		t.Error("Empty stack should not be able to rollback")
	}

	rs.PushFileEdit("test.go", "content")

	if !rs.CanRollback() {
		t.Error("Non-empty stack should be able to rollback")
	}
}

func TestClear(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	rs.PushFileEdit("file1.go", "c1")
	rs.PushFileEdit("file2.go", "c2")

	rs.Clear()

	if rs.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", rs.Size())
	}
}

func TestGetLastOfType(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	rs.PushFileEdit("file1.go", "c1")
	rs.PushFileCreate("file2.go")
	rs.PushFileEdit("file3.go", "c3")

	lastEdit := rs.GetLastOfType(RollbackTypeFileEdit)
	if lastEdit == nil {
		t.Fatal("Expected to find file edit")
	}
	if lastEdit.Path != "file3.go" {
		t.Errorf("Expected file3.go, got %s", lastEdit.Path)
	}

	lastCreate := rs.GetLastOfType(RollbackTypeFileCreate)
	if lastCreate == nil {
		t.Fatal("Expected to find file create")
	}
	if lastCreate.Path != "file2.go" {
		t.Errorf("Expected file2.go, got %s", lastCreate.Path)
	}
}

func TestCreateSnapshot(t *testing.T) {
	rs := NewRollbackStack(DefaultRollbackConfig())

	rs.PushFileEdit("file1.go", "c1")
	rs.PushFileEdit("file2.go", "c2")

	snapshot, err := rs.CreateSnapshot("test_snapshot")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snapshot.Name != "test_snapshot" {
		t.Errorf("Expected name 'test_snapshot', got %s", snapshot.Name)
	}
	if len(snapshot.Entries) != 2 {
		t.Errorf("Expected 2 entries in snapshot, got %d", len(snapshot.Entries))
	}
}

func TestTask9FullIntegration(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rollback_integration")
	defer os.RemoveAll(tmpDir)

	config := RollbackConfig{MaxEntries: 20, BaseDir: tmpDir}
	rs := NewRollbackStack(config)

	// Simulate a series of autonomous operations
	mainFile := filepath.Join(tmpDir, "main.go")
	configFile := filepath.Join(tmpDir, "config.yaml")
	newFile := filepath.Join(tmpDir, "new_feature.go")

	// Create original files
	originalMain := "package main\n\nfunc main() {}"
	originalConfig := "debug: false"
	os.WriteFile(mainFile, []byte(originalMain), 0644)
	os.WriteFile(configFile, []byte(originalConfig), 0644)

	// Operation 1: Edit main.go
	rs.PushFileEdit(mainFile, originalMain)
	newMain := "package main\n\nfunc main() { println(\"hello\") }"
	os.WriteFile(mainFile, []byte(newMain), 0644)
	t.Logf("✓ Pushed file edit for main.go")

	// Operation 2: Edit config.yaml
	rs.PushFileEdit(configFile, originalConfig)
	os.WriteFile(configFile, []byte("debug: true"), 0644)
	t.Logf("✓ Pushed file edit for config.yaml")

	// Operation 3: Create new file
	rs.PushFileCreate(newFile)
	os.WriteFile(newFile, []byte("package main\n\nfunc NewFeature() {}"), 0644)
	t.Logf("✓ Pushed file create for new_feature.go")

	// Verify stack state
	if rs.Size() != 3 {
		t.Errorf("Expected 3 entries, got %d", rs.Size())
	}

	// Simulate detection of an issue - rollback the last operation
	err := rs.Rollback()
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	t.Logf("✓ Rolled back last operation")

	// Verify new_file.go was deleted
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("Expected new_file.go to be deleted after rollback")
	}

	// Rollback all remaining operations
	err = rs.RollbackAll()
	if err != nil {
		t.Fatalf("RollbackAll failed: %v", err)
	}
	t.Logf("✓ Rolled back all operations")

	// Verify files restored to original state
	mainData, _ := os.ReadFile(mainFile)
	if string(mainData) != originalMain {
		t.Error("main.go not restored to original")
	}

	configData, _ := os.ReadFile(configFile)
	if string(configData) != originalConfig {
		t.Error("config.yaml not restored to original")
	}

	// Verify stack is empty
	if rs.Size() != 0 {
		t.Errorf("Expected empty stack, got %d", rs.Size())
	}

	t.Log("✅ Task 9: Rollback Stack - Full integration PASSED")
}
