package social

import (
	"encoding/json"
	"fmt"
	"strings"
)

type socialDecision struct {
	DiscussionID  string `json:"discussion_id"`
	Reply         string `json:"reply,omitempty"`
	Learning      string `json:"learning,omitempty"`
	NewDiscussion *struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"new_discussion,omitempty"`
}

func buildSocialPrompt(discussions []Discussion) string {
	var sb strings.Builder
	sb.WriteString("Here are the current GitHub Discussions. For each one, decide what to do.\n\n")
	sb.WriteString("Respond ONLY with a JSON array of decisions:\n")
	sb.WriteString(`[{"discussion_id":"ID","reply":"text or empty string","learning":"insight or empty","new_discussion":null}]`)
	sb.WriteString("\n\n## Discussions\n\n")

	for _, d := range discussions {
		sb.WriteString(fmt.Sprintf("### ID: %s | #%d: %s\n", d.ID, d.Number, d.Title))
		sb.WriteString(d.Body + "\n")
		for _, c := range d.Comments {
			sb.WriteString(fmt.Sprintf("  [%s]: %s\n", c.Author, c.Body))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseSocialDecisions(response string) ([]socialDecision, error) {
	// Strip markdown code fences if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var decisions []socialDecision
	if err := json.Unmarshal([]byte(response), &decisions); err != nil {
		return nil, err
	}
	return decisions, nil
}

func extractLearnings(decisions []socialDecision) string {
	var parts []string
	for _, d := range decisions {
		if d.Learning != "" {
			parts = append(parts, d.Learning)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}
