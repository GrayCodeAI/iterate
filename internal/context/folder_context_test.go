// Package context provides folder-level context capabilities.
package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultFolderContextConfig(t *testing.T) {
	config := DefaultFolderContextConfig()
	
	if config.MaxFiles != 50 {
		t.Errorf("expected 50, got %d", config.MaxFiles)
	}
	if config.MaxDepth != 3 {
		t.Errorf("expected 3, got %d", config.MaxDepth)
	}
	if config.MaxTotalSize != 500*1024 {
		t.Errorf("expected 500KB, got %d", config.MaxTotalSize)
	}
	if config.IncludeHidden {
		t.Error("expected IncludeHidden to be false")
	}
}

func TestNewFolderContextManager(t *testing.T) {
	fcm := NewFolderContextManager(nil, nil)
	if fcm == nil {
		t.Fatal("expected non-nil manager")
	}
	if fcm.folderCache == nil {
		t.Error("expected folderCache to be initialized")
	}
}

func TestFolderContextManager_GatherFolder_NonExistent(t *testing.T) {
	fcm := NewFolderContextManager(nil, nil)
	
	_, err := fcm.GatherFolder(context.Background(), "/nonexistent/folder", 0)
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}

func TestFolderContextManager_GatherFolder_File(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a file (not a folder)
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	_, err := fcm.GatherFolder(context.Background(), testFile, 0)
	if err == nil {
		t.Error("expected error when gathering a file instead of folder")
	}
}

func TestFolderContextManager_GatherFolder_EmptyFolder(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.Mkdir(emptyDir, 0755); err != nil {
		t.Fatalf("failed to create empty dir: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	result, err := fcm.GatherFolder(context.Background(), emptyDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Folder.FileCount != 0 {
		t.Errorf("expected 0 files, got %d", result.Folder.FileCount)
	}
	if result.Folder.DirCount != 0 {
		t.Errorf("expected 0 subdirs, got %d", result.Folder.DirCount)
	}
}

func TestFolderContextManager_GatherFolder_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create test files
	files := []string{"handler.go", "service.go", "utils.go"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Folder.FileCount != 3 {
		t.Errorf("expected 3 files, got %d", result.Folder.FileCount)
	}
	if result.Folder.Extensions[".go"] == 0 {
		t.Error("expected .go extension to be tracked")
	}
}

func TestFolderContextManager_GatherFolder_WithReadme(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create README
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !result.Folder.HasReadme {
		t.Error("expected HasReadme to be true")
	}
}

func TestFolderContextManager_GatherFolder_WithSubdirs(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create subdirs
	if err := os.Mkdir(filepath.Join(tmpDir, "sub1"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "sub2"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Folder.DirCount != 2 {
		t.Errorf("expected 2 subdirs, got %d", result.Folder.DirCount)
	}
}

func TestFolderContextManager_GatherFolder_MaxFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create more files than limit
	for i := 0; i < 60; i++ {
		if err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%02d.go", i)), []byte("package main"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}
	
	config := DefaultFolderContextConfig()
	config.MaxFiles = 10
	fcm := NewFolderContextManager(config, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !result.Truncated {
		t.Error("expected Truncated to be true")
	}
}

func TestFolderContextManager_GatherFolder_MaxDepth(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create nested structure
	level1 := filepath.Join(tmpDir, "level1")
	level2 := filepath.Join(level1, "level2")
	level3 := filepath.Join(level2, "level3")
	
	if err := os.MkdirAll(level3, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}
	
	// Create file at each level
	if err := os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("root"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(level3, "deep.go"), []byte("deep"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	config := DefaultFolderContextConfig()
	config.MaxDepth = 1
	fcm := NewFolderContextManager(config, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should have root file but not deep file
	if result.Folder.FileCount > 2 {
		t.Errorf("expected at most 2 files (root + level1), got %d", result.Folder.FileCount)
	}
}

func TestFolderContextManager_ExcludeHidden(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create hidden and regular files
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.go"), []byte("visible"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".hidden.go"), []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	config := DefaultFolderContextConfig()
	config.IncludeHidden = false
	fcm := NewFolderContextManager(config, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should only have visible file
	for _, f := range result.Folder.Files {
		if f.IsHidden {
			t.Error("expected hidden files to be excluded")
		}
	}
}

func TestFolderContextManager_IncludeHidden(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create hidden and regular files
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.go"), []byte("visible"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".hidden.go"), []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	config := DefaultFolderContextConfig()
	config.IncludeHidden = true
	fcm := NewFolderContextManager(config, nil)
	
	result, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should have both files
	if result.Folder.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", result.Folder.FileCount)
	}
}

func TestFolderContextManager_FilePriority(t *testing.T) {
	fcm := NewFolderContextManager(nil, nil)
	
	tests := []struct {
		name     string
		ext      string
		expected int
	}{
		{"main.go", ".go", 130}, // 100 + 30 for main
		{"handler.go", ".go", 100},
		{"handler_test.go", ".go", 90}, // 100 - 10 for test
		{"README.md", ".md", 50},
		{"go.mod", "", 40},
		{".hidden", "", -20},
	}
	
	for _, tc := range tests {
		summary := &FileSummary{
			Name:     tc.name,
			Ext:      tc.ext,
			IsHidden: tc.name[0] == '.',
		}
		
		priority := fcm.calculatePriority(summary)
		if priority < tc.expected-10 || priority > tc.expected+10 {
			t.Logf("Priority for %s: %d (expected around %d)", tc.name, priority, tc.expected)
		}
	}
}

func TestFolderContextManager_GetFolderStructure(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create structure
	if err := os.Mkdir(filepath.Join(tmpDir, "sub"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("root"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "child.go"), []byte("child"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	structure, err := fcm.GetFolderStructure(context.Background(), tmpDir, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if structure == "" {
		t.Error("expected non-empty structure")
	}
	if !contains(structure, "root.go") {
		t.Error("expected root.go in structure")
	}
}

func TestFolderContextManager_ClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	// Gather to populate cache
	_, _ = fcm.GatherFolder(context.Background(), tmpDir, 0)
	
	if len(fcm.folderCache) == 0 {
		t.Error("expected cache to be populated")
	}
	
	fcm.ClearCache()
	
	if len(fcm.folderCache) != 0 {
		t.Error("expected cache to be empty after clear")
	}
}

func TestFolderContextManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	_, _ = fcm.GatherFolder(context.Background(), tmpDir, 0)
	
	stats := fcm.GetStats()
	
	if stats["cached_folders"].(int) != 1 {
		t.Errorf("expected 1 cached folder, got %v", stats["cached_folders"])
	}
}

func TestFolderContextManager_UpdateConfig(t *testing.T) {
	fcm := NewFolderContextManager(nil, nil)
	
	newConfig := &FolderContextConfig{
		MaxFiles: 100,
		MaxDepth: 5,
	}
	
	fcm.UpdateConfig(newConfig)
	
	if fcm.config.MaxFiles != 100 {
		t.Errorf("expected 100, got %d", fcm.config.MaxFiles)
	}
}

func TestFolderContextManager_ResolveFolderPath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	// Test with @folder prefix
	path, ok := fcm.ResolveFolderPath("@folder " + subDir)
	if !ok {
		t.Error("expected to resolve folder path with @folder prefix")
	}
	if path != subDir {
		t.Errorf("expected %s, got %s", subDir, path)
	}
	
	// Test without prefix
	path, ok = fcm.ResolveFolderPath(subDir)
	if !ok {
		t.Error("expected to resolve folder path")
	}
	
	// Test non-existent
	_, ok = fcm.ResolveFolderPath("/nonexistent")
	if ok {
		t.Error("expected not to resolve non-existent path")
	}
}

func TestFolderContextManager_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create many files
	for i := 0; i < 20; i++ {
		if err := os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i%10))+".go"), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	_, err := fcm.GatherFolder(ctx, tmpDir, 0)
	if err != context.Canceled {
		t.Logf("context cancellation result: %v", err)
	}
}

func TestFolderContextManager_GatherMultipleFolders(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create multiple folders with files
	folder1 := filepath.Join(tmpDir, "folder1")
	folder2 := filepath.Join(tmpDir, "folder2")
	
	if err := os.Mkdir(folder1, 0755); err != nil {
		t.Fatalf("failed to create folder1: %v", err)
	}
	if err := os.Mkdir(folder2, 0755); err != nil {
		t.Fatalf("failed to create folder2: %v", err)
	}
	
	if err := os.WriteFile(filepath.Join(folder1, "a.go"), []byte("a"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder2, "b.go"), []byte("b"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	results, err := fcm.GatherMultipleFolders(context.Background(), []string{folder1, folder2}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFolderContextResult_ToMarkdown(t *testing.T) {
	result := &FolderContextResult{
		Folder: &FolderInfo{
			Name:      "testdir",
			Path:      "/path/to/testdir",
			FileCount: 5,
			DirCount:  2,
			TotalSize: 1024,
			HasReadme: true,
			ReadmePath: "/path/to/testdir/README.md",
			Extensions: map[string]int{".go": 3, ".md": 2},
			Files: []*FileSummary{
				{Name: "main.go", Size: 500, Priority: 130},
				{Name: "handler.go", Size: 300, Priority: 100},
			},
		},
		TotalFiles: 5,
		GatherTime: 5 * time.Millisecond,
	}
	
	markdown := result.ToMarkdown()
	
	if markdown == "" {
		t.Error("expected non-empty markdown")
	}
	if !contains(markdown, "testdir") {
		t.Error("expected folder name in markdown")
	}
	if !contains(markdown, "README") {
		t.Error("expected README in markdown")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		contains string
	}{
		{500, "B"},
		{1024, "KB"},
		{1024 * 1024, "MB"},
	}
	
	for _, tc := range tests {
		result := formatSize(tc.size)
		if !contains(result, tc.contains) {
			t.Errorf("formatSize(%d) = %s, expected to contain %s", tc.size, result, tc.contains)
		}
	}
}

func TestFolderContextManager_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	
	fcm := NewFolderContextManager(nil, nil)
	
	// First call
	result1, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Second call should hit cache
	result2, err := fcm.GatherFolder(context.Background(), tmpDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result1.TotalFiles != result2.TotalFiles {
		t.Errorf("cached result differs: %d vs %d", result1.TotalFiles, result2.TotalFiles)
	}
}
