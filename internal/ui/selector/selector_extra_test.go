package selector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatTokenCountZero(t *testing.T) {
	if got := formatTokenCount(0); got != "" {
		t.Errorf("expected empty string for 0, got %q", got)
	}
}

func TestFormatTokenCountSmall(t *testing.T) {
	got := formatTokenCount(500)
	if got == "" {
		t.Error("expected token count for 500")
	}
}

func TestFormatTokenCountLarge(t *testing.T) {
	got := formatTokenCount(1500)
	if got == "" {
		t.Error("expected token count for 1500")
	}
}

func TestFormatCostUSD_EdgeCases(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, ""},
		{0.00001, "<$0.0001"},
		{0.0005, "$0.0005"},
		{0.005, "$0.005"},
		{0.123, "$0.12"},
		{1.50, "$1.50"},
		{12.34, "$12.34"},
	}
	for _, tt := range tests {
		if got := formatCostUSD(tt.cost); got != tt.want {
			t.Errorf("formatCostUSD(%v) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}

func TestTabComplete_MultipleCo(t *testing.T) {
	got := TabComplete("/co")
	if got == "" {
		t.Error("expected partial match for /co")
	}
	if got[0:2] != "/c" {
		t.Errorf("expected /c prefix, got %q", got)
	}
}

func TestTabCompleteWithArgs_ArgumentCompletion(t *testing.T) {
	if got := TabCompleteWithArgs("/thinking o"); got != "/thinking off " {
		t.Errorf("expected /thinking off , got %q", got)
	}
}

func TestTabCompleteWithArgs_FilePathCompletion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.go"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "hello_test.go"), nil, 0644)
	os.Mkdir(filepath.Join(dir, "pkg"), 0755)

	oldPath := RepoPath
	RepoPath = dir
	defer func() { RepoPath = oldPath }()

	got := TabCompleteWithArgs("/add ")
	if got == "" {
		t.Error("expected file completion for /add ")
	}
}

func TestTabCompleteWithArgs_UnknownCommandArg(t *testing.T) {
	if got := TabCompleteWithArgs("/unknown arg"); got != "/unknown arg" {
		t.Errorf("expected /unknown arg, got %q", got)
	}
}

func TestCommandArgCompletions(t *testing.T) {
	if completions, ok := commandArgCompletions["/thinking"]; !ok || len(completions) == 0 {
		t.Error("expected thinking completions")
	}
	if completions, ok := commandArgCompletions["/theme"]; !ok || len(completions) == 0 {
		t.Error("expected theme completions")
	}
	if completions, ok := commandArgCompletions["/provider"]; !ok || len(completions) == 0 {
		t.Error("expected provider completions")
	}
	if completions, ok := commandArgCompletions["/git"]; !ok || len(completions) == 0 {
		t.Error("expected git completions")
	}
}

func TestFindPathMatches_Prefix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.go"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "beta.go"), nil, 0644)
	os.WriteFile(filepath.Join(dir, "alpha_test.go"), nil, 0644)
	os.Mkdir(filepath.Join(dir, "pkg"), 0755)

	matches := findPathMatches(filepath.Join(dir, "al"))
	if len(matches) != 2 {
		t.Errorf("expected 2 matches for 'al', got %d", len(matches))
	}
	dirMatches := findPathMatches(filepath.Join(dir, "p"))
	if len(dirMatches) != 1 {
		t.Errorf("expected 1 match for 'p', got %d", len(dirMatches))
	}
	if dirMatches[0][len(dirMatches[0])-1] != '/' {
		t.Errorf("expected directory match to end with /, got %q", dirMatches[0])
	}
	noMatches := findPathMatches(filepath.Join(dir, "xyz"))
	if len(noMatches) != 0 {
		t.Errorf("expected 0 matches for 'xyz', got %d", len(noMatches))
	}
}

func TestFindPathMatches_InvalidDir(t *testing.T) {
	matches := findPathMatches("/nonexistent/path/foo")
	if matches != nil {
		t.Errorf("expected nil for invalid dir, got %v", matches)
	}
}

func TestBuildCompletionResult_SingleMatch(t *testing.T) {
	result := buildCompletionResult([]string{"/add"}, []string{"main.go"})
	if result != "/add main.go" {
		t.Errorf("expected /add main.go, got %q", result)
	}
}

func TestBuildCompletionResult_MultipleMatches(t *testing.T) {
	matches := []string{"hello.go", "hello_test.go"}
	result := buildCompletionResult([]string{"/add"}, matches)
	if result == "" {
		t.Error("expected common prefix for multiple matches")
	}
}

func TestBuildCompletionResult_EmptyPrefix(t *testing.T) {
	result := buildCompletionResult([]string{}, []string{"file.go"})
	if result != "file.go" {
		t.Errorf("expected file.go, got %q", result)
	}
}

func TestPrintPrompt_Modes(t *testing.T) {
	orig := CurrentMode
	defer func() { CurrentMode = orig }()

	CurrentMode = ModeAsk
	PrintPrompt()

	CurrentMode = ModeArchitect
	PrintPrompt()

	CurrentMode = ModeNormal
	PrintPrompt()
}

func TestGitStatus_InvalidRepo(t *testing.T) {
	old := RepoPath
	RepoPath = "/nonexistent"
	defer func() { RepoPath = old }()

	staged, unstaged := gitStatus()
	if staged != 0 || unstaged != 0 {
		t.Errorf("expected 0,0 for invalid repo, got %d,%d", staged, unstaged)
	}
}

func TestGitStatus_EmptyRepo(t *testing.T) {
	old := RepoPath
	RepoPath = ""
	defer func() { RepoPath = old }()

	staged, unstaged := GitStatus()
	if staged != 0 || unstaged != 0 {
		t.Errorf("expected 0,0 for empty repo, got %d,%d", staged, unstaged)
	}
}

func TestFormatGitStatus_NoChanges(t *testing.T) {
	orig := RepoPath
	RepoPath = ""
	defer func() { RepoPath = orig }()

	gs := formatGitStatus()
	if gs != "" {
		t.Errorf("expected empty git status, got %q", gs)
	}
}

func TestSlashCommands_Exist(t *testing.T) {
	if len(slashCommands) < 100 {
		t.Errorf("expected at least 100 slash commands, got %d", len(slashCommands))
	}

	expected := []string{"/help", "/clear", "/add", "/save", "/commit", "/push", "/pull", "/pr", "/model", "/theme"}
	for _, cmd := range expected {
		found := false
		for _, sc := range slashCommands {
			if sc == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected slash command %q not found", cmd)
		}
	}
}

func TestSelectItem_EmptyList(t *testing.T) {
	if got, ok := SelectItem("test", []string{}); got != "" || ok != false {
		t.Errorf("expected empty string and false for empty list, got %q, %v", got, ok)
	}
}

func TestSelectItem_NilList(t *testing.T) {
	if got, ok := SelectItem("test", nil); got != "" || ok != false {
		t.Errorf("expected empty string and false for nil list, got %q, %v", got, ok)
	}
}

func TestFormatCostUSD_Boundaries(t *testing.T) {
	if got := formatCostUSD(0.00009); got != "<$0.0001" {
		t.Errorf("expected <$0.0001, got %q", got)
	}
	if got := formatCostUSD(0.0001); got != "$0.0001" {
		t.Errorf("expected $0.0001, got %q", got)
	}
	if got := formatCostUSD(0.009); got != "$0.009" {
		t.Errorf("expected $0.009, got %q", got)
	}
}
