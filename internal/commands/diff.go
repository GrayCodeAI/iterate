package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
)

// ErrUserQuit is returned when the user chooses to quit during an interactive
// diff review session. Callers should handle this gracefully instead of
// calling os.Exit directly from library code.
var ErrUserQuit = errors.New("user quit")

// DiffViewer shows unified diffs before applying changes
type DiffViewer struct {
	ColorAdded   *color.Color
	ColorRemoved *color.Color
	ColorContext *color.Color
	ColorHeader  *color.Color
}

// NewDiffViewer creates a new diff viewer with colors
func NewDiffViewer() *DiffViewer {
	return &DiffViewer{
		ColorAdded:   color.New(color.FgGreen),
		ColorRemoved: color.New(color.FgRed),
		ColorContext: color.New(color.FgWhite),
		ColorHeader:  color.New(color.FgCyan, color.Bold),
	}
}

// ShowDiff displays a unified diff between original and proposed content
func (dv *DiffViewer) ShowDiff(filename string, original []string, proposed []string) {
	fmt.Println()
	dv.ColorHeader.Printf("📄 %s\n", filename)
	fmt.Println(strings.Repeat("─", 60))

	// Simple line-by-line diff
	maxLines := len(original)
	if len(proposed) > maxLines {
		maxLines = len(proposed)
	}

	for i := 0; i < maxLines; i++ {
		origLine := ""
		if i < len(original) {
			origLine = original[i]
		}
		propLine := ""
		if i < len(proposed) {
			propLine = proposed[i]
		}

		if origLine != propLine {
			if origLine != "" {
				dv.ColorRemoved.Printf("-%s\n", origLine)
			}
			if propLine != "" {
				dv.ColorAdded.Printf("+%s\n", propLine)
			}
		} else {
			dv.ColorContext.Printf(" %s\n", origLine)
		}
	}
	fmt.Println()
}

// ShowGitDiff shows git-style diff for file changes
func (dv *DiffViewer) ShowGitDiff(filename string) error {
	cmd := exec.Command("git", "diff", "--no-color", filename)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if len(output) == 0 {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			dv.ColorAdded.Println(line)
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			dv.ColorRemoved.Println(line)
		} else if strings.HasPrefix(line, "@@") {
			dv.ColorHeader.Println(line)
		} else {
			fmt.Println(line)
		}
	}
	return nil
}

// ConfirmChange prompts user to confirm a change.
// Returns (apply bool, err error). err is ErrUserQuit when user chooses to quit.
func (dv *DiffViewer) ConfirmChange(filename string) (bool, error) {
	fmt.Printf("\n🤔 Apply changes to %s? [y/n/a(apply all)/q(quit)]: ", filename)

	var response string
	fmt.Scanln(&response)

	switch strings.ToLower(strings.TrimSpace(response)) {
	case "y", "yes":
		return true, nil
	case "a", "all":
		return true, nil
	case "q", "quit":
		return false, ErrUserQuit
	default:
		fmt.Printf("❌ Skipped %s\n", filename)
		return false, nil
	}
}

// BatchDiff shows diffs for multiple files.
// Returns ErrUserQuit if the user chooses to quit mid-review.
func (dv *DiffViewer) BatchDiff(files []string) (approved []string, rejected []string, err error) {
	applyAll := false

	for _, file := range files {
		if err := dv.ShowGitDiff(file); err != nil {
			fmt.Printf("⚠️  Could not show diff for %s: %v\n", file, err)
			continue
		}

		if applyAll {
			approved = append(approved, file)
			continue
		}

		ok, confirmErr := dv.ConfirmChange(file)
		if confirmErr != nil {
			return approved, rejected, confirmErr
		}
		if ok {
			approved = append(approved, file)
		} else {
			rejected = append(rejected, file)
		}
	}

	return approved, rejected, nil
}

// PreviewEdit shows a preview of an edit before applying.
// Returns ErrUserQuit if the user chooses to quit.
func PreviewEdit(filename string, oldContent string, newContent string) error {
	viewer := NewDiffViewer()

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	viewer.ShowDiff(filename, oldLines, newLines)

	ok, err := viewer.ConfirmChange(filename)
	if err != nil {
		return err
	}
	if ok {
		return os.WriteFile(filename, []byte(newContent), 0644)
	}

	return nil
}
