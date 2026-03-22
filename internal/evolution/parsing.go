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
				fmt.Sscanf(rest[:idx], "%d", &num)
				title = strings.TrimSpace(rest[idx+1:])
			} else {
				fmt.Sscanf(rest, "%d", &num)
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
		}
	}

	if current != nil {
		current.Description = strings.TrimSpace(strings.Join(descLines, "\n"))
		tasks = append(tasks, *current)
	}

	return tasks
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
			if num == 0 {
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
