// Package prompts provides prompt templates and context building for agent interactions.
package prompts

import "fmt"

// buildEditPrompt constructs a prompt for the AI to perform file edits.
// It includes the full file content, the instruction, and constraints
// to preserve formatting, make minimal changes, and return the complete file.
func buildEditPrompt(fileContent, instruction string) string {
	return fmt.Sprintf(
		"You are a code editor. Apply the following instruction to the file content.\n\n"+
			"INSTRUCTION:\n%s\n\n"+
			"FILE CONTENT:\n```\n%s\n```\n\n"+
			"CONSTRAINTS:\n"+
			"- Preserve original formatting (indentation, line endings, spacing)\n"+
			"- Make minimal changes - only modify what's necessary to fulfill the instruction\n"+
			"- Return the COMPLETE file content, not just the changes\n"+
			"- Do not add explanations or markdown code block markers around the output\n"+
			"- Ensure the result is syntactically valid and complete\n\n"+
			"Apply the instruction and return the complete updated file content:",
		instruction, fileContent)
}

// BuildEditPrompt is the exported version of buildEditPrompt for use by other packages.
func BuildEditPrompt(fileContent, instruction string) string {
	return buildEditPrompt(fileContent, instruction)
}
