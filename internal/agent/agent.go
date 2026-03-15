package agent

import (
	"context"
	"log/slog"

	"github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/provider"
)

type Event = iteragent.Event
type Message = iteragent.Message

func ToolMap(tools []Tool) map[string]Tool {
	return iteragent.ToolMap(tools)
}

type Agent struct {
	*iteragent.Agent
	Events chan Event
}

func New(p provider.Provider, tools []Tool, logger *slog.Logger) *Agent {
	adp := &providerAdapter{p: p}
	ag := iteragent.New(adp, tools, logger)
	return &Agent{
		Agent:  ag,
		Events: ag.Events,
	}
}

type providerAdapter struct {
	p provider.Provider
}

func (a *providerAdapter) Complete(ctx context.Context, messages []Message) (string, error) {
	providerMsgs := make([]provider.Message, len(messages))
	for i, m := range messages {
		providerMsgs[i] = provider.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return a.p.Complete(ctx, providerMsgs)
}

func (a *providerAdapter) Name() string {
	return a.p.Name()
}
