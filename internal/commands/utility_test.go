package commands

import (
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func newTestAgent(msgs ...iteragent.Message) *iteragent.Agent {
	return &iteragent.Agent{Messages: msgs}
}

func TestCmdInject_AppendsMessage(t *testing.T) {
	agent := newTestAgent(
		iteragent.Message{Role: "user", Content: "hello"},
	)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/inject", "injected", "text"},
	}

	result := cmdInject(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(agent.Messages))
	}
	if agent.Messages[1].Role != "user" {
		t.Errorf("expected injected role 'user', got %q", agent.Messages[1].Role)
	}
	if agent.Messages[1].Content != "injected text" {
		t.Errorf("expected injected content 'injected text', got %q", agent.Messages[1].Content)
	}
}

func TestCmdInject_EmptyArgs(t *testing.T) {
	agent := newTestAgent()
	ctx := Context{
		Agent: agent,
		Parts: []string{"/inject"},
	}

	result := cmdInject(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 0 {
		t.Error("should not inject when args are empty")
	}
}

func TestCmdInject_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/inject", "text"},
	}

	result := cmdInject(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdRewind_RemovesExchanges(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
		{Role: "assistant", Content: "a3"},
	}
	agent := newTestAgent(msgs...)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/rewind", "2"},
	}

	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 2 {
		t.Fatalf("expected 2 messages after rewinding 2 exchanges, got %d", len(agent.Messages))
	}
	if agent.Messages[0].Content != "q1" {
		t.Errorf("expected first message to be 'q1', got %q", agent.Messages[0].Content)
	}
}

func TestCmdRewind_DefaultOneExchange(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
	}
	agent := newTestAgent(msgs...)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/rewind"},
	}

	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 2 {
		t.Fatalf("expected 2 messages after rewinding 1 exchange, got %d", len(agent.Messages))
	}
}

func TestCmdRewind_ClampsToZero(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "q1"},
	}
	agent := newTestAgent(msgs...)
	ctx := Context{
		Agent: agent,
		Parts: []string{"/rewind", "10"},
	}

	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if len(agent.Messages) != 0 {
		t.Fatalf("expected 0 messages after rewinding more than available, got %d", len(agent.Messages))
	}
}

func TestCmdRewind_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/rewind", "1"},
	}

	result := cmdRewind(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdFork_SavesAndResets(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	agent := newTestAgent(msgs...)

	var savedName string
	var savedMsgs []iteragent.Message
	ctx := Context{
		Agent: agent,
		Parts: []string{"/fork"},
		SaveSession: func(name string, m []iteragent.Message) error {
			savedName = name
			savedMsgs = m
			return nil
		},
	}

	result := cmdFork(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if savedName == "" {
		t.Error("expected session to be saved")
	}
	if len(savedMsgs) != 2 {
		t.Errorf("expected 2 messages saved, got %d", len(savedMsgs))
	}
	if len(agent.Messages) != 0 {
		t.Errorf("expected agent messages to be reset, got %d messages", len(agent.Messages))
	}
}

func TestCmdFork_NilAgent(t *testing.T) {
	ctx := Context{
		Agent: nil,
		Parts: []string{"/fork"},
	}

	result := cmdFork(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
}

func TestCmdFork_EmptyConversation(t *testing.T) {
	agent := newTestAgent()
	saved := false
	ctx := Context{
		Agent: agent,
		Parts: []string{"/fork"},
		SaveSession: func(name string, m []iteragent.Message) error {
			saved = true
			return nil
		},
	}

	result := cmdFork(ctx)
	if !result.Handled {
		t.Error("expected command to be handled")
	}
	if saved {
		t.Error("should not save when conversation is empty")
	}
}
