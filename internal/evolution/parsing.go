package evolution

import (
	"bufio"
	"fmt"
	"strings"
)

type planTask struct {
	Number      int
	Title       string
	Description string
	Files       []string // parsed from "Files: ..." line — used for parallel conflict detection
}

func parseSessionPlanTasks(plan string) []planTask {
	var tasks []planTask
	scanner := bufio.NewScanner(strings.NewReader(plan))

	var current *planTask
	var descLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "### Task ") {
			if current != nil {
				current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
				tasks = append(tasks, *current)
			}
			rest := strings.TrimPrefix(line, "### Task ")
			var num int
			var title string
			if idx := strings.IndexByte(rest, ':'); idx >= 0 {
				if n, _ := fmt.Sscanf(rest[:idx], "%d", &num); n == 0 {
					num = len(tasks) + 1
				}
				title = strings.TrimSpace(rest[idx+1:])
			} else {
				if n, _ := fmt.Sscanf(rest, "%d", &num); n == 0 {
					num = len(tasks) + 1
				}
				title = rest
			}
			current = &planTask{Number: num, Title: title}
			descLines = []string{line}
			continue
		}

		if strings.HasPrefix(line, "### Issue Responses") || strings.HasPrefix(line, "### Issue responses") {
			if current != nil {
				current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
				tasks = append(tasks, *current)
				current = nil
			}
			break
		}

		if current != nil {
			descLines = append(descLines, line)
			// Parse "Files: a, b, c" line to populate task.Files.
			if strings.HasPrefix(strings.TrimSpace(line), "Files:") {
				raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Files:"))
				for _, f := range strings.Split(raw, ",") {
					if f := strings.TrimSpace(f); f != "" {
						current.Files = append(current.Files, f)
					}
				}
			}
		}
	}

	if current != nil {
		current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
		tasks = append(tasks, *current)
	}

	return tasks
}

// groupTasksByFileOverlap splits tasks into sequential waves.
// Tasks within a wave have no overlapping declared files and can run in parallel.
// Tasks that share a file with any task already in the current wave start a new wave.
func groupTasksByFileOverlap(tasks []planTask) [][]planTask {
	var waves [][]planTask
	var currentWave []planTask
	waveFiles := map[string]bool{}

	for _, task := range tasks {
		// If task declares no files, always start a new wave (safe default).
		if len(task.Files) == 0 {
			if len(currentWave) > 0 {
				waves = append(waves, currentWave)
			}
			waves = append(waves, []planTask{task})
			currentWave = nil
			waveFiles = map[string]bool{}
			continue
		}

		// Check if any of this task's files are already claimed in the current wave.
		conflict := false
		for _, f := range task.Files {
			if waveFiles[f] {
				conflict = true
				break
			}
		}

		if conflict {
			// Start a new wave.
			if len(currentWave) > 0 {
				waves = append(waves, currentWave)
			}
			currentWave = []planTask{task}
			waveFiles = map[string]bool{}
			for _, f := range task.Files {
				waveFiles[f] = true
			}
		} else {
			// Add to current wave.
			currentWave = append(currentWave, task)
			for _, f := range task.Files {
				waveFiles[f] = true
			}
		}
	}

	if len(currentWave) > 0 {
		waves = append(waves, currentWave)
	}
	return waves
}

type issueResponse struct {
	IssueNum int
	Status   string
	Reason   string
}

func parseIssueResponses(plan string) []issueResponse {
	var responses []issueResponse
	inSection := false

	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "### Issue Responses") || strings.HasPrefix(line, "### Issue responses") {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "### ") {
			break
		}
		if inSection && strings.HasPrefix(line, "- #") {
			rest := strings.TrimPrefix(line, "- #")
			var num int
			fmt.Sscanf(rest, "%d", &num)
			if num <= 0 || num > 999999 {
				continue
			}
			status := "comment"
			reason := rest
			if strings.Contains(rest, "wontfix") {
				status = "wontfix"
			} else if strings.Contains(rest, "implement") {
				status = "implement"
			} else if strings.Contains(rest, "partial") {
				status = "partial"
			}
			if idx := strings.Index(rest, "—"); idx >= 0 {
				reason = strings.TrimSpace(rest[idx+len("—"):])
			} else if idx := strings.Index(rest, "--"); idx >= 0 {
				reason = strings.TrimSpace(rest[idx+2:])
			}
			responses = append(responses, issueResponse{IssueNum: num, Status: status, Reason: reason})
		}
	}
	return responses
}

func extractIssueNumbers(plan string) []int {
	var nums []int
	responses := parseIssueResponses(plan)
	for _, r := range responses {
		if r.Status == "implement" || r.Status == "partial" {
			nums = append(nums, r.IssueNum)
		}
	}
	return nums
}

func buildPRBody(plan, output string) string {
	var body strings.Builder

	sessionTitle := extractSessionTitle(plan)
	if sessionTitle != "" {
		body.WriteString("## Summary\n\n")
		body.WriteString(sessionTitle + "\n\n")
	}

	body.WriteString("## Changes\n\n")
	commitLines := extractCommitLines(output)
	for _, line := range commitLines {
		body.WriteString("- " + line + "\n")
	}
	if len(commitLines) == 0 {
		body.WriteString("- Self-improvement and bug fixes\n")
	}

	body.WriteString("\n## Tasks\n\n")
	tasks := parseSessionPlanTasks(plan)
	for _, task := range tasks {
		body.WriteString(fmt.Sprintf("- [ ] %s\n", task.Title))
	}

	return body.String()
}

func extractSessionTitle(plan string) string {
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "Session Title:") {
			return strings.TrimPrefix(line, "Session Title:")
		}
	}
	return ""
}
