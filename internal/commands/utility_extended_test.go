package commands

import (
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

// ---------------------------------------------------------------------------
// cmdContext
// ---------------------------------------------------------------------------

func TestCmdContext_WithAgent(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
		iteragent.Message{Role: "assistant", Content: "hi"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/context"},
	}
	result := cmdContext(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdContext_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/context"},
	}
	result := cmdContext(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdContext_WithTokenCounts(t *testing.T) {
	inputTokens := 1000
	outputTokens := 500
	ctx := Context{
		Agent:               newTestAgent(),
		Parts:               []string{"/context"},
		SessionInputTokens:  &inputTokens,
		SessionOutputTokens: &outputTokens,
	}
	result := cmdContext(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdExport
// ---------------------------------------------------------------------------

func TestCmdExport_NoAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/export"},
	}
	result := cmdExport(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdExport_EmptyConversation(t *testing.T) {
	ctx := Context{
		Agent: newTestAgent(),
		Parts: []string{"/export"},
	}
	result := cmdExport(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdExport_WithMessages(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.md"
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
		iteragent.Message{Role: "assistant", Content: "hi there"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/export", path},
	}
	result := cmdExport(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdRetry
// ---------------------------------------------------------------------------

func TestCmdRetry_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/retry"},
	}
	result := cmdRetry(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdRetry_TooFewMessages(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/retry"},
	}
	result := cmdRetry(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdRetry_LastNotAssistant(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
		iteragent.Message{Role: "user", Content: "again"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/retry"},
	}
	result := cmdRetry(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 2 {
		t.Error("messages should not be modified when last is not assistant")
	}
}

func TestCmdRetry_RemovesLastAssistant(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
		iteragent.Message{Role: "assistant", Content: "hi"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/retry"},
	}
	result := cmdRetry(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 1 {
		t.Errorf("expected 1 message after retry, got %d", len(agent.Messages))
	}
}

// ---------------------------------------------------------------------------
// cmdCopy
// ---------------------------------------------------------------------------

func TestCmdCopy_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/copy"},
	}
	result := cmdCopy(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCopy_EmptyMessages(t *testing.T) {
	ctx := Context{
		Agent: newTestAgent(),
		Parts: []string{"/copy"},
	}
	result := cmdCopy(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCopy_LastNotAssistant(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/copy"},
	}
	result := cmdCopy(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdPin / cmdUnpin
// ---------------------------------------------------------------------------

func TestCmdPin_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/pin"},
	}
	result := cmdPin(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdPin_EmptyMessages(t *testing.T) {
	ctx := Context{
		Agent: newTestAgent(),
		Parts: []string{"/pin"},
	}
	result := cmdPin(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdUnpin(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{
		RepoPath: dir,
		Parts:    []string{"/unpin"},
	}
	result := cmdUnpin(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

// ---------------------------------------------------------------------------
// cmdCompact
// ---------------------------------------------------------------------------

func TestCmdCompact_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/compact"},
	}
	result := cmdCompact(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCompact_EmptyMessages(t *testing.T) {
	ctx := Context{
		Agent: newTestAgent(),
		Parts: []string{"/compact"},
	}
	result := cmdCompact(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdCompact_WithMessages(t *testing.T) {
	dir := t.TempDir()
	msgs := make([]iteragent.Message, 30)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = iteragent.Message{Role: role, Content: "message content here"}
	}
	agent := newTestAgent(msgs...)
	ctx := Context{
		Agent:    agent,
		RepoPath: dir,
		Parts:    []string{"/compact"},
	}
	result := cmdCompact(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) >= 30 {
		t.Error("expected compaction to reduce messages")
	}
}

// ---------------------------------------------------------------------------
// cmdRewind edge cases
// ---------------------------------------------------------------------------

func TestCmdRewind_NilAgentMultiple(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/rewind", "5"},
	}
	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdRewind_ZeroMessages(t *testing.T) {
	agent := newTestAgent()
	ctx := Context{
		Agent: agent,
		Parts: []string{"/rewind", "1"},
	}
	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 0 {
		t.Error("expected 0 messages")
	}
}

// ---------------------------------------------------------------------------
// cmdInject edge cases
// ---------------------------------------------------------------------------

func TestCmdInject_SingleArg(t *testing.T) {
	agent := newTestAgent()
	ctx := Context{
		Agent: agent,
		Parts: []string{"/inject", "hello"},
	}
	result := cmdInject(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(agent.Messages))
	}
}
