package evolution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplySearchReplaceBlocks applies the SEARCH/REPLACE blocks to files
// Returns list of modified files and any errors
func (e *Engine) ApplySearchReplaceBlocks(blocks []SearchReplaceBlock) ([]string, error) {
	var modifiedFiles []string

	for _, block := range blocks {
		fullPath := filepath.Join(e.repoPath, block.FilePath)

		// Read existing file
		content, err := os.ReadFile(fullPath)
		if err != nil {
			// File doesn't exist - create it if this is a new file
			if os.IsNotExist(err) && block.Search == "" {
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return modifiedFiles, fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
				if err := os.WriteFile(fullPath, []byte(block.Replace), 0644); err != nil {
					return modifiedFiles, fmt.Errorf("failed to create file %s: %w", block.FilePath, err)
				}
				modifiedFiles = append(modifiedFiles, block.FilePath)
				continue
			}
			return modifiedFiles, fmt.Errorf("failed to read file %s: %w", block.FilePath, err)
		}

		// Replace the search text with replace text
		oldContent := string(content)
		newContent := strings.Replace(oldContent, block.Search, block.Replace, 1)

		if oldContent == newContent {
			return modifiedFiles, fmt.Errorf("SEARCH block not found in %s (content may have changed)", block.FilePath)
		}

		// Write back
		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			return modifiedFiles, fmt.Errorf("failed to write file %s: %w", block.FilePath, err)
		}

		modifiedFiles = append(modifiedFiles, block.FilePath)
		e.logger.Info("Applied SEARCH/REPLACE", "file", block.FilePath)
	}

	return modifiedFiles, nil
}

// DetectSearchReplaceBlocks checks if the output contains SEARCH/REPLACE blocks
func DetectSearchReplaceBlocks(output string) bool {
	return strings.Contains(output, "<<<<<<< SEARCH") &&
		strings.Contains(output, "=======") &&
		strings.Contains(output, ">>>>>>>")
}

// CountSearchReplaceBlocks returns the number of valid SEARCH/REPLACE blocks
func CountSearchReplaceBlocks(output string) int {
	return strings.Count(output, "<<<<<<< SEARCH")
}

// ValidateSearchReplaceBlocks checks if blocks are well-formed
func ValidateSearchReplaceBlocks(blocks []SearchReplaceBlock) []string {
	var errors []string

	for i, block := range blocks {
		if block.FilePath == "" {
			errors = append(errors, fmt.Sprintf("Block %d: missing file path", i+1))
			continue
		}
		if block.Search == "" && block.Replace == "" {
			errors = append(errors, fmt.Sprintf("Block %d (%s): empty search and replace", i+1, block.FilePath))
			continue
		}
		if !strings.HasSuffix(block.FilePath, ".go") &&
			!strings.HasSuffix(block.FilePath, ".md") &&
			!strings.HasSuffix(block.FilePath, ".yml") &&
			!strings.HasSuffix(block.FilePath, ".json") {
			errors = append(errors, fmt.Sprintf("Block %d (%s): unusual file extension", i+1, block.FilePath))
		}
	}

	return errors
}
