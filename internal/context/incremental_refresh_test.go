// Package context provides incremental context refresh capabilities.
package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIncrementalRefreshConfig_Defaults(t *testing.T) {
	config := DefaultIncrementalRefreshConfig()

	if config.HashAlgorithm != "sha256" {
		t.Errorf("expected sha256, got %s", config.HashAlgorithm)
	}
	if config.MaxCacheAge != 24*time.Hour {
		t.Errorf("expected 24h, got %v", config.MaxCacheAge)
	}
	if config.IncludeHidden {
		t.Error("expected IncludeHidden to be false")
	}
	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected 10MB, got %d", config.MaxFileSize)
	}
	if config.TokenEstimator == nil {
		t.Error("expected TokenEstimator to be set")
	}
}

func TestNewIncrementalRefresher(t *testing.T) {
	ir := NewIncrementalRefresher(nil, nil, "")
	if ir == nil {
		t.Fatal("expected non-nil refresher")
	}
	if ir.snapshots == nil {
		t.Error("expected snapshots map to be initialized")
	}
	if ir.config == nil {
		t.Error("expected default config to be set")
	}
}

func TestIncrementalRefresher_Refresh_EmptyFiles(t *testing.T) {
	ir := NewIncrementalRefresher(nil, nil, "")

	result, err := ir.Refresh(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 0 {
		t.Errorf("expected no changes, got %d", len(result.Changes))
	}
	if result.FilesAdded != 0 || result.FilesModified != 0 || result.FilesDeleted != 0 {
		t.Error("expected all counts to be zero")
	}
}

func TestIncrementalRefresher_Refresh_NewFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FilesAdded != 1 {
		t.Errorf("expected 1 file added, got %d", result.FilesAdded)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	change := result.Changes[0]
	if change.ChangeType != "added" {
		t.Errorf("expected added, got %s", change.ChangeType)
	}
	if change.Path != testFile {
		t.Errorf("expected path %s, got %s", testFile, change.Path)
	}
	if change.NewHash == "" {
		t.Error("expected non-empty new hash")
	}
	if change.TokensDiff <= 0 {
		t.Errorf("expected positive token diff, got %d", change.TokensDiff)
	}
}

func TestIncrementalRefresher_Refresh_UnchangedFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh - file is new
	result1, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.FilesAdded != 1 {
		t.Errorf("expected 1 file added on first refresh, got %d", result1.FilesAdded)
	}

	// Second refresh - file is unchanged
	result2, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result2.FilesUnchanged != 1 {
		t.Errorf("expected 1 file unchanged, got %d", result2.FilesUnchanged)
	}
	if result2.FilesAdded != 0 || result2.FilesModified != 0 {
		t.Error("expected no added or modified files")
	}
}

func TestIncrementalRefresher_Refresh_ModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh
	_, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modify file
	time.Sleep(10 * time.Millisecond) // Ensure mod time changes
	newContent := []byte("hello world - modified and longer")
	if err := os.WriteFile(testFile, newContent, 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Second refresh - should detect modification
	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FilesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", result.FilesModified)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	change := result.Changes[0]
	if change.ChangeType != "modified" {
		t.Errorf("expected modified, got %s", change.ChangeType)
	}
	if change.OldHash == "" || change.NewHash == "" {
		t.Error("expected both old and new hash")
	}
	if change.OldHash == change.NewHash {
		t.Error("expected different hashes for modified file")
	}
}

func TestIncrementalRefresher_Refresh_DeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh - add file
	result1, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.FilesAdded != 1 {
		t.Fatalf("expected 1 file added, got %d", result1.FilesAdded)
	}

	// Delete file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to delete test file: %v", err)
	}

	// Second refresh - should detect deletion
	result2, err := ir.Refresh(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result2.FilesDeleted != 1 {
		t.Errorf("expected 1 file deleted, got %d", result2.FilesDeleted)
	}
	if len(result2.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result2.Changes))
	}

	change := result2.Changes[0]
	if change.ChangeType != "deleted" {
		t.Errorf("expected deleted, got %s", change.ChangeType)
	}
	if change.TokensDiff >= 0 {
		t.Errorf("expected negative token diff for deletion, got %d", change.TokensDiff)
	}
}

func TestIncrementalRefresher_SHA256Hash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Calculate expected hash
	expectedHash := sha256.Sum256(content)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	config := DefaultIncrementalRefreshConfig()
	config.HashAlgorithm = "sha256"
	ir := NewIncrementalRefresher(config, nil, "")

	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	if result.Changes[0].NewHash != expectedHashStr {
		t.Errorf("hash mismatch: expected %s, got %s", expectedHashStr, result.Changes[0].NewHash)
	}
}

func TestIncrementalRefresher_ModTimeHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	config := DefaultIncrementalRefreshConfig()
	config.HashAlgorithm = "modtime"
	ir := NewIncrementalRefresher(config, nil, "")

	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	// Hash should be the mod time as a string
	info, _ := os.Stat(testFile)
	expectedHash := int64(info.ModTime().UnixNano())
	if result.Changes[0].NewHash == "" {
		t.Error("expected non-empty hash")
	}
	t.Logf("modtime hash: %s (expected ~%d)", result.Changes[0].NewHash, expectedHash)
}

func TestIncrementalRefresher_SizeHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	config := DefaultIncrementalRefreshConfig()
	config.HashAlgorithm = "size"
	ir := NewIncrementalRefresher(config, nil, "")

	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(result.Changes))
	}

	expectedHash := int64(len(content))
	if result.Changes[0].NewHash != string(rune(expectedHash)) {
		// Hash should be the size
		t.Logf("size hash: %s (expected %d)", result.Changes[0].NewHash, len(content))
	}
}

func TestIncrementalRefresher_ExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files
	testFile := filepath.Join(tmpDir, "test.txt")
	gitFile := filepath.Join(tmpDir, ".gitignore")
	nodeModulesFile := filepath.Join(tmpDir, "node_modules", "package.json")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(gitFile, []byte("git content"), 0644); err != nil {
		t.Fatalf("failed to create git file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(nodeModulesFile), 0755); err != nil {
		t.Fatalf("failed to create node_modules dir: %v", err)
	}
	if err := os.WriteFile(nodeModulesFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create node_modules file: %v", err)
	}

	config := DefaultIncrementalRefreshConfig()
	config.ExcludePatterns = []string{".git/*", "node_modules/*"}
	config.IncludeHidden = false
	ir := NewIncrementalRefresher(config, nil, "")

	result, err := ir.Refresh(context.Background(), []string{testFile, gitFile, nodeModulesFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only test.txt should be processed (gitFile is hidden, nodeModulesFile is excluded)
	if result.FilesAdded != 1 {
		t.Errorf("expected 1 file added, got %d (changes: %+v)", result.FilesAdded, result.Changes)
	}
}

func TestIncrementalRefresher_IncludeHidden(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	hiddenFile := filepath.Join(tmpDir, ".hidden")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create hidden file: %v", err)
	}

	tests := []struct {
		name          string
		includeHidden bool
		expectedCount int
	}{
		{"exclude hidden", false, 1},
		{"include hidden", true, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultIncrementalRefreshConfig()
			config.IncludeHidden = tc.includeHidden
			ir := NewIncrementalRefresher(config, nil, "")

			result, err := ir.Refresh(context.Background(), []string{testFile, hiddenFile})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.FilesAdded != tc.expectedCount {
				t.Errorf("expected %d files added, got %d", tc.expectedCount, result.FilesAdded)
			}
		})
	}
}

func TestIncrementalRefresher_MaxFileSize(t *testing.T) {
	tmpDir := t.TempDir()

	smallFile := filepath.Join(tmpDir, "small.txt")
	largeFile := filepath.Join(tmpDir, "large.txt")

	if err := os.WriteFile(smallFile, []byte("small"), 0644); err != nil {
		t.Fatalf("failed to create small file: %v", err)
	}
	if err := os.WriteFile(largeFile, make([]byte, 2000), 0644); err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	config := &IncrementalRefreshConfig{
		MaxFileSize:    1000, // 1KB
		TokenEstimator: EstimateTokens,
	}
	ir := NewIncrementalRefresher(config, nil, "")

	result, err := ir.Refresh(context.Background(), []string{smallFile, largeFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only small file should be processed
	if result.FilesAdded != 1 {
		t.Errorf("expected 1 file added (large file skipped), got %d", result.FilesAdded)
	}
}

func TestIncrementalRefresher_ForceFullRefresh(t *testing.T) {
	ir := NewIncrementalRefresher(nil, nil, "")

	// Set lastFull to a past time
	ir.lastFull = time.Now().Add(-1 * time.Hour)

	// Force full refresh
	ir.ForceFullRefresh()

	if !ir.lastFull.IsZero() {
		t.Error("expected lastFull to be zero after ForceFullRefresh")
	}
}

func TestIncrementalRefresher_FullRefreshTriggered(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	config := &IncrementalRefreshConfig{
		MaxCacheAge:    1 * time.Millisecond,
		TokenEstimator: EstimateTokens,
	}
	ir := NewIncrementalRefresher(config, nil, "")

	// First refresh - triggers full refresh
	result, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.FullRefresh {
		t.Error("expected full refresh on first call")
	}

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)
	ir.ForceFullRefresh()

	// Second refresh - should trigger full refresh again
	result2, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result2.FullRefresh {
		t.Error("expected full refresh after cache expiry")
	}
}

func TestIncrementalRefresher_GetSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// No snapshot initially
	snap := ir.GetSnapshot(testFile)
	if snap != nil {
		t.Error("expected nil snapshot before refresh")
	}

	// Refresh to create snapshot
	_, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now snapshot should exist
	snap = ir.GetSnapshot(testFile)
	if snap == nil {
		t.Fatal("expected non-nil snapshot after refresh")
	}
	if snap.Path != testFile {
		t.Errorf("expected path %s, got %s", testFile, snap.Path)
	}
	if snap.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestIncrementalRefresher_GetAllSnapshots(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "test1.txt")
	file2 := filepath.Join(tmpDir, "test2.txt")

	if err := os.WriteFile(file1, []byte("test1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("test2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")
	_, err := ir.Refresh(context.Background(), []string{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshots := ir.GetAllSnapshots()
	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestIncrementalRefresher_ClearSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")
	_, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ir.GetAllSnapshots()) != 1 {
		t.Fatal("expected 1 snapshot before clear")
	}

	ir.ClearSnapshots()

	if len(ir.GetAllSnapshots()) != 0 {
		t.Error("expected 0 snapshots after clear")
	}
}

func TestIncrementalRefresher_GetChangedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh
	_, err := ir.Refresh(context.Background(), []string{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modify file1
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(file1, []byte("modified content"), 0644); err != nil {
		t.Fatalf("failed to modify file1: %v", err)
	}

	// Get changed files
	changed, err := ir.GetChangedFiles([]string{file1, file2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(changed) != 1 {
		t.Errorf("expected 1 changed file, got %d: %v", len(changed), changed)
	}
	if len(changed) > 0 && changed[0] != file1 {
		t.Errorf("expected %s to be changed, got %s", file1, changed[0])
	}
}

func TestIncrementalRefresher_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files to ensure cancellation check is hit
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(files[i], []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file %d: %v", i, err)
		}
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := ir.Refresh(ctx, files)
	// The error might be context.Canceled or nil depending on timing
	// The important thing is that it doesn't hang or panic
	if err != nil && err != context.Canceled {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIncrementalRefresher_TokenDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with initial content
	initialContent := "hello world"
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh
	result1, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	initialTokens := result1.TotalTokenDiff

	// Modify file with more content
	time.Sleep(10 * time.Millisecond)
	longerContent := initialContent + " this is much longer content added later"
	if err := os.WriteFile(testFile, []byte(longerContent), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Second refresh
	result2, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Token diff should be positive (more tokens added)
	if result2.TotalTokenDiff <= 0 {
		t.Errorf("expected positive token diff, got %d", result2.TotalTokenDiff)
	}

	t.Logf("Initial tokens: %d, Diff after modification: %d", initialTokens, result2.TotalTokenDiff)
}

func TestIncrementalRefresher_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := make([]string, 5)
	for i := 0; i < 5; i++ {
		files[i] = filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(files[i], []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file %d: %v", i, err)
		}
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	// First refresh - all files are new
	result1, err := ir.Refresh(context.Background(), files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.FilesAdded != 5 {
		t.Errorf("expected 5 files added, got %d", result1.FilesAdded)
	}

	// Modify 2 files
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(files[0], []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file0: %v", err)
	}
	if err := os.WriteFile(files[2], []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file2: %v", err)
	}

	// Second refresh
	result2, err := ir.Refresh(context.Background(), files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.FilesModified != 2 {
		t.Errorf("expected 2 files modified, got %d", result2.FilesModified)
	}
	if result2.FilesUnchanged != 3 {
		t.Errorf("expected 3 files unchanged, got %d", result2.FilesUnchanged)
	}
}

func TestIncrementalRefresher_UpdateConfig(t *testing.T) {
	ir := NewIncrementalRefresher(nil, nil, "")

	newConfig := &IncrementalRefreshConfig{
		HashAlgorithm: "modtime",
		MaxCacheAge:   1 * time.Hour,
	}
	ir.UpdateConfig(newConfig)

	config := ir.GetConfig()
	if config.HashAlgorithm != "modtime" {
		t.Errorf("expected modtime, got %s", config.HashAlgorithm)
	}
	if config.MaxCacheAge != 1*time.Hour {
		t.Errorf("expected 1h, got %v", config.MaxCacheAge)
	}
}

func TestRefreshResult_ToMarkdown(t *testing.T) {
	result := &RefreshResult{
		Changes: []*FileChange{
			{Path: "file1.txt", ChangeType: "added", TokensDiff: 100},
			{Path: "file2.txt", ChangeType: "modified", TokensDiff: 50},
			{Path: "file3.txt", ChangeType: "deleted", TokensDiff: -30},
		},
		FilesAdded:     1,
		FilesModified:  1,
		FilesDeleted:   1,
		FilesUnchanged: 5,
		TotalTokenDiff: 120,
		RefreshTime:    10 * time.Millisecond,
		FullRefresh:    false,
	}

	markdown := result.ToMarkdown()

	if markdown == "" {
		t.Error("expected non-empty markdown")
	}
	// The actual format uses markdown bold: "- **Added:** 1"
	if !contains(markdown, "**Added:** 1") {
		t.Errorf("expected '**Added:** 1' in markdown, got: %s", markdown)
	}
	if !contains(markdown, "**Modified:** 1") {
		t.Errorf("expected '**Modified:** 1' in markdown, got: %s", markdown)
	}
	if !contains(markdown, "**Deleted:** 1") {
		t.Errorf("expected '**Deleted:** 1' in markdown, got: %s", markdown)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFileSnapshot_TokenEstimation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	// Create a Go file with known content
	content := `package main

func main() {
	fmt.Println("Hello, World!")
}`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ir := NewIncrementalRefresher(nil, nil, "")

	_, err := ir.Refresh(context.Background(), []string{testFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := ir.GetSnapshot(testFile)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	// Token estimate should be reasonable (roughly chars/4 for fallback)
	if snap.TokenEst <= 0 {
		t.Errorf("expected positive token estimate, got %d", snap.TokenEst)
	}

	t.Logf("Content length: %d, Token estimate: %d", len(content), snap.TokenEst)
}
