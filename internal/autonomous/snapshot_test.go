// Package autonomous - Task 30: Tests for Snapshot capability
package autonomous

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotStatus_Constants(t *testing.T) {
	if SnapshotStatusCreating != "creating" {
		t.Error("SnapshotStatusCreating should be 'creating'")
	}
	if SnapshotStatusComplete != "complete" {
		t.Error("SnapshotStatusComplete should be 'complete'")
	}
	if SnapshotStatusRestored != "restored" {
		t.Error("SnapshotStatusRestored should be 'restored'")
	}
	if SnapshotStatusFailed != "failed" {
		t.Error("SnapshotStatusFailed should be 'failed'")
	}
}

func TestSnapshotType_Constants(t *testing.T) {
	if SnapshotTypeFile != "file" {
		t.Error("SnapshotTypeFile should be 'file'")
	}
	if SnapshotTypeDirectory != "directory" {
		t.Error("SnapshotTypeDirectory should be 'directory'")
	}
	if SnapshotTypeProject != "project" {
		t.Error("SnapshotTypeProject should be 'project'")
	}
}

func TestDefaultSnapshotConfig(t *testing.T) {
	config := DefaultSnapshotConfig()
	
	if !config.Enabled {
		t.Error("Default config should be enabled")
	}
	if config.MaxSnapshots != 50 {
		t.Errorf("Expected 50 max snapshots, got: %d", config.MaxSnapshots)
	}
	if config.DefaultTTL != 24*time.Hour {
		t.Error("Default TTL should be 24 hours")
	}
	if !config.AutoSnapshot {
		t.Error("AutoSnapshot should be enabled by default")
	}
}

func TestNewSnapshotManager(t *testing.T) {
	config := DefaultSnapshotConfig()
	config.StoragePath = t.TempDir()
	
	sm := NewSnapshotManager(config)
	
	if sm == nil {
		t.Fatal("Expected non-nil manager")
	}
	
	if sm.snapshots == nil {
		t.Error("Snapshots map should be initialized")
	}
}

func TestSnapshotManager_CreateSnapshot_File(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create manager
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	// Create snapshot
	metadata, err := sm.CreateSnapshot("test-snapshot", SnapshotTypeFile, []string{testFile})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	if metadata.ID == "" {
		t.Error("Snapshot ID should be set")
	}
	if metadata.Status != SnapshotStatusComplete {
		t.Errorf("Expected complete status, got: %s", metadata.Status)
	}
	if metadata.FileCount != 1 {
		t.Errorf("Expected 1 file, got: %d", metadata.FileCount)
	}
	if metadata.Size == 0 {
		t.Error("Size should be greater than 0")
	}
}

func TestSnapshotManager_CreateSnapshot_Directory(t *testing.T) {
	// Create temp directory with files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(testDir, "file2.txt"), []byte("content2"), 0644)
	
	// Create manager
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	// Create snapshot
	metadata, err := sm.CreateSnapshot("dir-snapshot", SnapshotTypeDirectory, []string{testDir})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	if metadata.FileCount < 2 {
		t.Errorf("Expected at least 2 files, got: %d", metadata.FileCount)
	}
}

func TestSnapshotManager_RestoreSnapshot(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}
	
	// Create manager and snapshot
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, err := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	if err != nil {
		t.Fatal(err)
	}
	
	// Modify the file
	modifiedContent := []byte("modified content")
	if err := os.WriteFile(testFile, modifiedContent, 0644); err != nil {
		t.Fatal(err)
	}
	
	// Restore snapshot
	if err := sm.RestoreSnapshot(metadata.ID); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	
	// Verify content restored
	restoredContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(restoredContent) != string(originalContent) {
		t.Error("Content should be restored to original")
	}
}

func TestSnapshotManager_DeleteSnapshot(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	// Create manager and snapshot
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Delete snapshot
	if err := sm.DeleteSnapshot(metadata.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	
	// Verify deleted
	_, exists := sm.GetSnapshot(metadata.ID)
	if exists {
		t.Error("Snapshot should be deleted")
	}
}

func TestSnapshotManager_GetSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	retrieved, exists := sm.GetSnapshot(metadata.ID)
	if !exists {
		t.Fatal("Snapshot should exist")
	}
	
	if retrieved.ID != metadata.ID {
		t.Error("Retrieved snapshot should match")
	}
}

func TestSnapshotManager_ListSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	sm.CreateSnapshot("test1", SnapshotTypeFile, []string{testFile})
	sm.CreateSnapshot("test2", SnapshotTypeFile, []string{testFile})
	sm.CreateSnapshot("test3", SnapshotTypeFile, []string{testFile})
	
	list := sm.ListSnapshots()
	if len(list) != 3 {
		t.Errorf("Expected 3 snapshots, got: %d", len(list))
	}
}

func TestSnapshotManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	stats := sm.GetStats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("Expected 1 snapshot, got: %d", stats.TotalSnapshots)
	}
}

func TestSnapshotManager_MaxSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	config.MaxSnapshots = 2
	sm := NewSnapshotManager(config)
	
	// Create 5 snapshots
	for i := 0; i < 5; i++ {
		sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	}
	
	list := sm.ListSnapshots()
	if len(list) != 2 {
		t.Errorf("Expected 2 snapshots (max), got: %d", len(list))
	}
}

func TestSnapshotManager_ExpireSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	config.DefaultTTL = 1 * time.Millisecond
	sm := NewSnapshotManager(config)
	
	sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Wait for expiry
	time.Sleep(10 * time.Millisecond)
	
	expired := sm.ExpireSnapshots()
	if expired != 1 {
		t.Errorf("Expected 1 expired, got: %d", expired)
	}
}

func TestSnapshotManager_AutoSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	config.AutoSnapshot = true
	sm := NewSnapshotManager(config)
	
	// Destructive operation
	metadata, err := sm.AutoSnapshot("delete", testFile, []string{testFile})
	if err != nil {
		t.Fatalf("AutoSnapshot failed: %v", err)
	}
	
	if metadata == nil {
		t.Fatal("AutoSnapshot should create snapshot for destructive operation")
	}
	
	// Non-destructive operation
	metadata2, _ := sm.AutoSnapshot("read", testFile, []string{testFile})
	if metadata2 != nil {
		t.Error("AutoSnapshot should not create snapshot for non-destructive operation")
	}
}

func TestSnapshotManager_ExcludePatterns(t *testing.T) {
	// Create temp directory with excluded files
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(filepath.Join(testDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(testDir, "main.go"), []byte("code"), 0644)
	os.WriteFile(filepath.Join(testDir, "node_modules", "package.js"), []byte("js"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	config.ExcludePatterns = []string{"node_modules"}
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeDirectory, []string{testDir})
	
	// node_modules should be excluded
	for _, file := range metadata.Files {
		if filepath.Base(file.OriginalPath) == "package.js" {
			t.Error("node_modules files should be excluded")
		}
	}
}

func TestSnapshotOptions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	expires := time.Now().Add(1 * time.Hour)
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile},
		WithExpiry(expires),
		WithTags("important", "backup"),
		WithReason("manual backup"),
	)
	
	if metadata.ExpiresAt == nil {
		t.Error("Expiry should be set")
	}
	if len(metadata.Tags) != 2 {
		t.Error("Tags should be set")
	}
	if metadata.Reason != "manual backup" {
		t.Error("Reason should be set")
	}
}

func TestSnapshotManager_Disabled(t *testing.T) {
	config := DefaultSnapshotConfig()
	config.Enabled = false
	sm := NewSnapshotManager(config)
	
	_, err := sm.CreateSnapshot("test", SnapshotTypeFile, []string{"/some/path"})
	if err == nil {
		t.Error("Disabled manager should fail to create snapshot")
	}
}

func TestSnapshotManager_RestoreNonExistent(t *testing.T) {
	config := DefaultSnapshotConfig()
	config.StoragePath = t.TempDir()
	sm := NewSnapshotManager(config)
	
	err := sm.RestoreSnapshot("nonexistent")
	if err == nil {
		t.Error("Should fail for non-existent snapshot")
	}
}

func TestSnapshotManager_DeleteNonExistent(t *testing.T) {
	config := DefaultSnapshotConfig()
	config.StoragePath = t.TempDir()
	sm := NewSnapshotManager(config)
	
	err := sm.DeleteSnapshot("nonexistent")
	if err == nil {
		t.Error("Should fail for non-existent snapshot")
	}
}

func TestSnapshotManager_GetSnapshotsByType(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testDir := filepath.Join(tmpDir, "testdir")
	os.WriteFile(testFile, []byte("content"), 0644)
	os.MkdirAll(testDir, 0755)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	sm.CreateSnapshot("file1", SnapshotTypeFile, []string{testFile})
	sm.CreateSnapshot("file2", SnapshotTypeFile, []string{testFile})
	sm.CreateSnapshot("dir1", SnapshotTypeDirectory, []string{testDir})
	
	fileSnapshots := sm.GetSnapshotsByType(SnapshotTypeFile)
	if len(fileSnapshots) != 2 {
		t.Errorf("Expected 2 file snapshots, got: %d", len(fileSnapshots))
	}
	
	dirSnapshots := sm.GetSnapshotsByType(SnapshotTypeDirectory)
	if len(dirSnapshots) != 1 {
		t.Errorf("Expected 1 directory snapshot, got: %d", len(dirSnapshots))
	}
}

func TestSnapshotManager_ExportMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	data, err := sm.ExportMetadata(metadata.ID)
	if err != nil {
		t.Fatalf("ExportMetadata failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Export should produce data")
	}
}

func TestTask30Snapshot(t *testing.T) {
	// Comprehensive test for Task 30
	
	// Setup
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "important.txt")
	os.WriteFile(testFile, []byte("important data"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	// Test 1: Create snapshot
	metadata, err := sm.CreateSnapshot("important-backup", SnapshotTypeFile, []string{testFile})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	if metadata.Status != SnapshotStatusComplete {
		t.Errorf("Expected complete status, got: %s", metadata.Status)
	}
	
	// Test 2: Modify file
	os.WriteFile(testFile, []byte("modified data"), 0644)
	
	// Test 3: Restore snapshot
	if err := sm.RestoreSnapshot(metadata.ID); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	
	// Test 4: Verify restoration
	content, _ := os.ReadFile(testFile)
	if string(content) != "important data" {
		t.Error("File should be restored to original content")
	}
	
	// Test 5: Check stats
	stats := sm.GetStats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("Expected 1 snapshot, got: %d", stats.TotalSnapshots)
	}
	if stats.RestoredCount != 1 {
		t.Errorf("Expected 1 restore, got: %d", stats.RestoredCount)
	}
	
	// Test 6: List snapshots
	list := sm.ListSnapshots()
	if len(list) != 1 {
		t.Errorf("Expected 1 snapshot in list, got: %d", len(list))
	}
}

// Task 31: Rollback Verification Tests

func TestSnapshotManager_VerifyRestore_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Restore (which doesn't change content in this case)
	sm.RestoreSnapshot(metadata.ID)
	
	// Verify should succeed
	result, err := sm.VerifyRestore(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyRestore failed: %v", err)
	}
	
	if !result.Success {
		t.Error("VerifyRestore should succeed for valid restore")
	}
	if result.VerifiedFiles != 1 {
		t.Errorf("Expected 1 verified file, got: %d", result.VerifiedFiles)
	}
	if result.FailedFiles != 0 {
		t.Errorf("Expected 0 failed files, got: %d", result.FailedFiles)
	}
}

func TestSnapshotManager_VerifyRestore_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Delete the file after snapshot
	os.Remove(testFile)
	
	// Verify should detect missing file
	result, err := sm.VerifyRestore(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyRestore failed: %v", err)
	}
	
	if result.Success {
		t.Error("VerifyRestore should fail when file is missing")
	}
	if len(result.MissingFiles) != 1 {
		t.Errorf("Expected 1 missing file, got: %d", len(result.MissingFiles))
	}
	if result.FailedFiles != 1 {
		t.Errorf("Expected 1 failed file, got: %d", result.FailedFiles)
	}
}

func TestSnapshotManager_VerifyRestore_ChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Modify file after snapshot (simulating restore that didn't work or was corrupted)
	os.WriteFile(testFile, []byte("corrupted content"), 0644)
	
	// Verify should detect checksum mismatch
	result, err := sm.VerifyRestore(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyRestore failed: %v", err)
	}
	
	if result.Success {
		t.Error("VerifyRestore should fail when checksum mismatches")
	}
	if len(result.ChecksumMismatches) != 1 {
		t.Errorf("Expected 1 checksum mismatch, got: %d", len(result.ChecksumMismatches))
	}
	if result.ChecksumMismatches[0].ExpectedHash == result.ChecksumMismatches[0].ActualHash {
		t.Error("Expected and actual hash should differ")
	}
}

func TestSnapshotManager_VerifyRestore_PermissionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Change permissions after snapshot
	os.Chmod(testFile, 0600)
	
	// Verify should detect permission mismatch (but still succeed)
	result, err := sm.VerifyRestore(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyRestore failed: %v", err)
	}
	
	// Permission mismatch is a warning, not a failure
	if !result.Success {
		t.Error("VerifyRestore should succeed (permission is just a warning)")
	}
	if len(result.PermissionErrors) != 1 {
		t.Errorf("Expected 1 permission error, got: %d", len(result.PermissionErrors))
	}
}

func TestSnapshotManager_VerifyAndRepair(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Corrupt the file
	os.WriteFile(testFile, []byte("corrupted"), 0644)
	
	// Verify and repair
	result, err := sm.VerifyAndRepair(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyAndRepair failed: %v", err)
	}
	
	// Should be repaired
	if !result.Success {
		t.Error("VerifyAndRepair should succeed after repair")
	}
	
	// Verify content is restored
	content, _ := os.ReadFile(testFile)
	if string(content) != "original content" {
		t.Error("File should be repaired to original content")
	}
}

func TestSnapshotManager_VerifySnapshotIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Verify integrity should pass
	err := sm.VerifySnapshotIntegrity(metadata.ID)
	if err != nil {
		t.Errorf("VerifySnapshotIntegrity should pass: %v", err)
	}
}

func TestSnapshotManager_VerifySnapshotIntegrity_MissingSnapshotFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Delete the snapshot file
	for _, file := range metadata.Files {
		os.Remove(file.SnapshotPath)
	}
	
	// Verify integrity should fail
	err := sm.VerifySnapshotIntegrity(metadata.ID)
	if err == nil {
		t.Error("VerifySnapshotIntegrity should fail when snapshot file is missing")
	}
}

func TestSnapshotManager_RestoreSnapshot_WithVerifyAfter(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Modify file
	os.WriteFile(testFile, []byte("modified"), 0644)
	
	// Restore with VerifyAfter enabled (default)
	err := sm.RestoreSnapshot(metadata.ID)
	if err != nil {
		t.Errorf("RestoreSnapshot with verification should succeed: %v", err)
	}
	
	// Content should be restored
	content, _ := os.ReadFile(testFile)
	if string(content) != "original" {
		t.Error("File should be restored")
	}
}

func TestSnapshotManager_VerifyRestore_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(testDir, "file2.txt"), []byte("content2"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeDirectory, []string{testDir})
	
	// Verify should succeed
	result, err := sm.VerifyRestore(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyRestore failed: %v", err)
	}
	
	if !result.Success {
		t.Error("VerifyRestore should succeed for directory")
	}
	if result.VerifiedFiles < 2 {
		t.Errorf("Expected at least 2 verified files, got: %d", result.VerifiedFiles)
	}
}

func TestSnapshotManager_VerifyRestore_NonExistentSnapshot(t *testing.T) {
	config := DefaultSnapshotConfig()
	config.StoragePath = t.TempDir()
	sm := NewSnapshotManager(config)
	
	_, err := sm.VerifyRestore("nonexistent")
	if err == nil {
		t.Error("VerifyRestore should fail for non-existent snapshot")
	}
}

func TestSnapshotManager_VerifyAndRepair_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	metadata, _ := sm.CreateSnapshot("test", SnapshotTypeFile, []string{testFile})
	
	// Delete the file completely
	os.Remove(testFile)
	
	// Verify and repair should restore it
	_, err := sm.VerifyAndRepair(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyAndRepair failed: %v", err)
	}
	
	// File should be restored
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("File should be restored after VerifyAndRepair")
	}
}

func TestTask31RollbackVerification(t *testing.T) {
	// Comprehensive test for Task 31: Rollback Verification
	
	// Setup
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "critical.txt")
	os.WriteFile(testFile, []byte("critical data"), 0644)
	
	config := DefaultSnapshotConfig()
	config.StoragePath = filepath.Join(tmpDir, "snapshots")
	sm := NewSnapshotManager(config)
	
	// Test 1: Create snapshot
	metadata, err := sm.CreateSnapshot("critical-backup", SnapshotTypeFile, []string{testFile})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	// Test 2: Verify snapshot integrity
	if err := sm.VerifySnapshotIntegrity(metadata.ID); err != nil {
		t.Errorf("Snapshot integrity check failed: %v", err)
	}
	
	// Test 3: Corrupt file
	os.WriteFile(testFile, []byte("corrupted"), 0644)
	
	// Test 4: Verify detects corruption
	result, _ := sm.VerifyRestore(metadata.ID)
	if result.Success {
		t.Error("Verification should detect corruption")
	}
	if len(result.ChecksumMismatches) != 1 {
		t.Error("Should have checksum mismatch")
	}
	
	// Test 5: Repair
	repairResult, err := sm.VerifyAndRepair(metadata.ID)
	if err != nil {
		t.Fatalf("VerifyAndRepair failed: %v", err)
	}
	if !repairResult.Success {
		t.Error("Repair should succeed")
	}
	
	// Test 6: Verify content restored
	content, _ := os.ReadFile(testFile)
	if string(content) != "critical data" {
		t.Error("File should be repaired to original content")
	}
}
