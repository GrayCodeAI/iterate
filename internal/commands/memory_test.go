package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestCmdMemo_NoArgs(t *testing.T) {
	ctx := Context{Parts: []string{"/memo"}}
	result := cmdMemo(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMemo_WithText(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/memo", "remember", "this"},
	}
	result := cmdMemo(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	data, err := os.ReadFile(filepath.Join(dir, "docs/JOURNAL.md"))
	if err != nil {
		t.Fatal("JOURNAL.md should be created")
	}
	if len(data) == 0 {
		t.Error("JOURNAL.md should have content")
	}
}

func TestCmdLearn_NoArgs(t *testing.T) {
	ctx := Context{Parts: []string{"/learn"}}
	result := cmdLearn(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdLearn_WithFact(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/learn", "always", "test", "first"},
	}
	result := cmdLearn(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	data, err := os.ReadFile(filepath.Join(dir, "memory", "learnings.jsonl"))
	if err != nil {
		t.Fatal("learnings.jsonl should be created")
	}
	if len(data) == 0 {
		t.Error("learnings.jsonl should have content")
	}
}

func TestCmdRemember_NoArgs(t *testing.T) {
	ctx := Context{Parts: []string{"/remember"}}
	result := cmdRemember(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdRemember_WithNote(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/remember", "this", "is", "important"},
	}
	result := cmdRemember(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".iterate", "memory.json"))
	if err != nil {
		t.Fatal("memory.json should be created")
	}
	if len(data) == 0 {
		t.Error("memory.json should have content")
	}
}

func TestCmdMemories_Empty(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{RepoPath: dir}
	result := cmdMemories(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMemories_WithNotes(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".iterate"), 0o755)
	os.WriteFile(filepath.Join(dir, ".iterate", "memory.json"),
		[]byte(`[{"timestamp":"2024-01-01","note":"test note"}]`), 0o644)
	ctx := Context{RepoPath: dir}
	result := cmdMemories(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdForget_NoArgs(t *testing.T) {
	ctx := Context{Parts: []string{"/forget"}}
	result := cmdForget(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdQuit_ReturnsDone(t *testing.T) {
	ctx := Context{Parts: []string{"/quit"}}
	result := cmdQuit(ctx)
	if !result.Done {
		t.Error("expected Done=true")
	}
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestCmdClear_NilAgent(t *testing.T) {
	ctx := Context{Agent: nil}
	result := cmdClear(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdClear_WithAgent(t *testing.T) {
	agent := newTestAgent()
	ctx := Context{Agent: agent}
	result := cmdClear(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSave_NilCallback(t *testing.T) {
	ctx := Context{
		Parts:   []string{"/save"},
		Session: SessionCallbacks{},
	}
	result := cmdSave(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSave_WithCallback(t *testing.T) {
	saved := false
	ctx := Context{
		Parts: []string{"/save", "test"},
		Agent: newTestAgent(),
		Session: SessionCallbacks{
			SaveSession: func(name string, msgs []iteragent.Message) error {
				saved = true
				return nil
			},
		},
	}
	result := cmdSave(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if !saved {
		t.Error("SaveSession should have been called")
	}
}

func TestCmdLoad_NilCallback(t *testing.T) {
	ctx := Context{Session: SessionCallbacks{}}
	result := cmdLoad(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdLoad_EmptySessions(t *testing.T) {
	ctx := Context{
		Session: SessionCallbacks{
			ListSessions: func() []string { return nil },
			LoadSession:  func(name string) ([]iteragent.Message, error) { return nil, nil },
		},
	}
	result := cmdLoad(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdBookmark_NilCallback(t *testing.T) {
	ctx := Context{
		Parts:   []string{"/bookmark"},
		Session: SessionCallbacks{},
	}
	result := cmdBookmark(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdHistory_Empty(t *testing.T) {
	ctx := Context{}
	result := cmdHistory(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdHistory_WithEntries(t *testing.T) {
	history := []string{"cmd1", "cmd2", "cmd3"}
	ctx := Context{InputHistory: &history}
	result := cmdHistory(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdCompactHard_NilAgent(t *testing.T) {
	ctx := Context{Parts: []string{"/compact-hard"}}
	result := cmdCompactHard(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdCompactHard_FewMessages(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "a"},
		iteragent.Message{Role: "assistant", Content: "b"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/compact-hard"},
	}
	result := cmdCompactHard(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdCompactHard_Compacts(t *testing.T) {
	msgs := make([]iteragent.Message, 20)
	for i := range msgs {
		msgs[i] = iteragent.Message{Role: "user", Content: "msg"}
	}
	agent := newTestAgent(msgs...)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/compact-hard", "5"},
	}
	result := cmdCompactHard(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if len(agent.Messages) != 5 {
		t.Errorf("expected 5 messages after compact, got %d", len(agent.Messages))
	}
}

func TestCmdPinList_Empty(t *testing.T) {
	ctx := Context{}
	result := cmdPinList(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdPinList_WithMessages(t *testing.T) {
	pinned := []iteragent.Message{
		{Role: "user", Content: "important"},
	}
	ctx := Context{
		State: StateAccessors{
			GetPinnedMessages: func() []iteragent.Message { return pinned },
		},
	}
	result := cmdPinList(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdChanges_NilCallback(t *testing.T) {
	ctx := Context{Templates: TemplateCallbacks{}}
	result := cmdChanges(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdChanges_WithFormatter(t *testing.T) {
	ctx := Context{
		Templates: TemplateCallbacks{
			FormatSessionChanges: func() string { return "file1.go\nfile2.go" },
		},
	}
	result := cmdChanges(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdTemplates_NilCallback(t *testing.T) {
	ctx := Context{Templates: TemplateCallbacks{}}
	result := cmdTemplates(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdTemplates_Empty(t *testing.T) {
	ctx := Context{
		Templates: TemplateCallbacks{
			LoadTemplates: func() []PromptTemplate { return nil },
		},
	}
	result := cmdTemplates(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdTemplates_WithTemplates(t *testing.T) {
	templates := []PromptTemplate{
		{Name: "test", Prompt: "do something", Created: time.Now()},
	}
	ctx := Context{
		Templates: TemplateCallbacks{
			LoadTemplates: func() []PromptTemplate { return templates },
		},
	}
	result := cmdTemplates(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSaveTemplate_NoPrompt(t *testing.T) {
	ctx := Context{LastPrompt: nil}
	result := cmdSaveTemplate(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSaveTemplate_EmptyPrompt(t *testing.T) {
	empty := ""
	ctx := Context{LastPrompt: &empty}
	result := cmdSaveTemplate(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdSaveTemplate_WithPrompt(t *testing.T) {
	prompt := "test prompt"
	added := false
	ctx := Context{
		LastPrompt: &prompt,
		Parts:      []string{"/save-template", "my", "template"},
		Templates: TemplateCallbacks{
			AddTemplate: func(name, p string) { added = true },
		},
	}
	result := cmdSaveTemplate(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if !added {
		t.Error("AddTemplate should have been called")
	}
}

func TestCmdTemplate_NoArg(t *testing.T) {
	ctx := Context{Parts: []string{"/template"}}
	result := cmdTemplate(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdTemplate_NotFound(t *testing.T) {
	ctx := Context{
		Parts: []string{"/template", "nonexistent"},
		Templates: TemplateCallbacks{
			LoadTemplates: func() []PromptTemplate { return nil },
		},
	}
	result := cmdTemplate(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMulti_NilCallback(t *testing.T) {
	ctx := Context{REPL: REPLCallbacks{}}
	result := cmdMulti(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdEnv_Show(t *testing.T) {
	ctx := Context{Parts: []string{"/env"}}
	result := cmdEnv(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdEnv_ShowVar(t *testing.T) {
	os.Setenv("TEST_ITERATE_VAR", "test_value")
	defer os.Unsetenv("TEST_ITERATE_VAR")
	ctx := Context{Parts: []string{"/env", "TEST_ITERATE_VAR"}}
	result := cmdEnv(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdEnv_SetVar(t *testing.T) {
	ctx := Context{Parts: []string{"/env", "TEST_ITERATE_NEW", "value"}}
	result := cmdEnv(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
	if os.Getenv("TEST_ITERATE_NEW") != "value" {
		t.Error("env var should be set")
	}
	os.Unsetenv("TEST_ITERATE_NEW")
}

func TestCmdMCPAdd_NoArgs(t *testing.T) {
	ctx := Context{
		Parts:  []string{"/mcp-add"},
		Config: ConfigCallbacks{},
	}
	result := cmdMCPAdd(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPAdd_NilCallbacks(t *testing.T) {
	ctx := Context{
		Parts:  []string{"/mcp-add"},
		Config: ConfigCallbacks{},
	}
	result := cmdMCPAdd(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPList_NilCallback(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{},
	}
	result := cmdMCPList(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPList_Empty(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{
			LoadMCPServers: func() []MCPServerEntry { return nil },
		},
	}
	result := cmdMCPList(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPList_WithServers(t *testing.T) {
	servers := []MCPServerEntry{
		{Name: "test", URL: "http://localhost:3000"},
	}
	ctx := Context{
		Config: ConfigCallbacks{
			LoadMCPServers: func() []MCPServerEntry { return servers },
		},
	}
	result := cmdMCPList(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPRemove_NilCallbacks(t *testing.T) {
	ctx := Context{
		Config: ConfigCallbacks{},
	}
	result := cmdMCPRemove(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestCmdMCPRemove_EmptyName(t *testing.T) {
	ctx := Context{
		Line:   "/mcp-remove",
		Parts:  []string{"/mcp-remove"},
		Config: ConfigCallbacks{},
	}
	result := cmdMCPRemove(ctx)
	if !result.Handled {
		t.Error("expected handled=true")
	}
}

func TestIsEmpty(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"  ", true},
		{"\t", true},
		{"hello", false},
		{" hello ", false},
	}
	for _, c := range cases {
		if got := isEmpty(c.input); got != c.want {
			t.Errorf("isEmpty(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
