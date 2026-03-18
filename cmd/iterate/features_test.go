package main

import (
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestBuildRepoIndex(t *testing.T) {
	index := buildRepoIndex(".")
	if len(index) == 0 {
		t.Errorf("repo index should not be empty")
	}
}

func TestContextBar(t *testing.T) {
	bar := contextBar([]iteragent.Message{}, 4000)
	if len(bar) == 0 {
		t.Errorf("context bar should not be empty")
	}
}

func TestBuildPrompts(t *testing.T) {
	prompts := []string{
		buildFixPrompt("error: undefined variable"),
		buildExplainErrorPrompt("panic: nil pointer"),
		buildGenerateCommitPrompt("."),
		buildReviewPrompt("."),
		buildSummarizePrompt([]iteragent.Message{}),
	}

	for i, prompt := range prompts {
		if len(prompt) == 0 {
			t.Errorf("prompt %d should not be empty", i)
		}
	}
}

func TestBuildDiagramPrompt(t *testing.T) {
	prompt := buildDiagramPrompt(".")
	if !strings.Contains(prompt, "diagram") && !strings.Contains(prompt, "ASCII") {
		t.Errorf("diagram prompt should mention diagrams")
	}
}

func TestBuildReadmePrompt(t *testing.T) {
	prompt := buildReadmePrompt(".")
	if !strings.Contains(prompt, "README") {
		t.Errorf("readme prompt should mention README")
	}
}
