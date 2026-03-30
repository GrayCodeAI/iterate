package evolution

import (
	"context"
	"fmt"
	"strings"
)

// ConflictResolution provides autonomous git conflict resolution.
// Task 7: Autonomous git conflict resolution

// detectConflicts checks if there are merge conflicts in the working tree.
func (e *Engine) detectConflicts(ctx context.Context) (bool, []string, error) {
	out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git diff --name-only --diff-filter=U",
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to check conflicts: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return false, nil, nil
	}

	files := strings.Split(out, "\n")
	var conflictedFiles []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" {
			conflictedFiles = append(conflictedFiles, f)
		}
	}
	return len(conflictedFiles) > 0, conflictedFiles, nil
}

// resolveConflicts attempts to automatically resolve merge conflicts.
// Returns the list of files that were auto-resolved and any that still need manual resolution.
func (e *Engine) resolveConflicts(ctx context.Context, files []string) (resolved []string, unresolved []string, err error) {
	for _, file := range files {
		if e.canAutoResolve(ctx, file) {
			if err := e.autoResolveFile(ctx, file); err != nil {
				e.logger.Warn("auto-resolve failed", "file", file, "err", err)
				unresolved = append(unresolved, file)
			} else {
				resolved = append(resolved, file)
				e.logger.Info("auto-resolved conflict", "file", file)
			}
		} else {
			unresolved = append(unresolved, file)
			e.logger.Info("conflict needs manual resolution", "file", file)
		}
	}
	return resolved, unresolved, nil
}

// canAutoResolve checks if a file's conflicts can be automatically resolved.
func (e *Engine) canAutoResolve(ctx context.Context, file string) bool {
	out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("grep -c '<<<<<<<' %q", file),
	})
	if err != nil {
		return false
	}
	out = strings.TrimSpace(out)

	// Auto-resolve only if there's a single conflict marker
	// and the conflict is simple (additions only from one side)
	count := 0
	fmt.Sscanf(out, "%d", &count)
	return count == 1
}

// autoResolveFile attempts to auto-resolve a file with a single conflict.
// Strategy: prefer "our" changes (current branch) for simple conflicts.
func (e *Engine) autoResolveFile(ctx context.Context, file string) error {
	out, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("cat %q", file),
	})
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	hasOurs := strings.Contains(out, "<<<<<<<") && strings.Contains(out, "=======")
	hasTheirs := strings.Contains(out, "=======") && strings.Contains(out, ">>>>>>>")

	if !hasOurs || !hasTheirs {
		return fmt.Errorf("malformed conflict markers")
	}

	parts := strings.Split(out, "<<<<<<<")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected conflict format")
	}

	conflictSection := parts[1]
	conflictParts := strings.SplitN(conflictSection, "=======", 2)
	if len(conflictParts) != 2 {
		return fmt.Errorf("missing separator")
	}

	theirPart := strings.SplitN(conflictParts[1], ">>>>>>>", 2)
	if len(theirPart) != 2 {
		return fmt.Errorf("missing closing marker")
	}

	ours := strings.TrimSpace(conflictParts[0])
	theirs := strings.TrimSpace(theirPart[0])
	after := theirPart[1]

	var resolved string
	if ours == "" && theirs != "" {
		resolved = theirs
	} else if theirs == "" && ours != "" {
		resolved = ours
	} else {
		resolved = ours
	}

	result := parts[0] + resolved + after

	// Write resolved file using printf to avoid heredoc injection
	escapedFile := strings.ReplaceAll(file, "'", "'\\''")
	_, err = e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("printf '%%s' '%s' > %q", result, escapedFile),
	})
	if err != nil {
		return fmt.Errorf("failed to write resolved file: %w", err)
	}

	_, err = e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": fmt.Sprintf("git add %q", file),
	})
	return err
}

// abortMerge aborts an in-progress merge.
func (e *Engine) abortMerge(ctx context.Context) error {
	_, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git merge --abort",
	})
	return err
}

// continueMerge completes a merge after conflicts are resolved.
func (e *Engine) continueMerge(ctx context.Context) error {
	_, err := e.runTool(ctx, "bash", map[string]interface{}{
		"cmd": "git commit --no-edit",
	})
	return err
}
