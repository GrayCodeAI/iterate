package agent

import (
	"log/slog"

	"github.com/GrayCodeAI/iteragent"
)

type Event = iteragent.Event
type Message = iteragent.Message
type Tool = iteragent.Tool
type Provider = iteragent.Provider

type Agent struct {
	*iteragent.Agent
	Events chan Event
}

func New(p Provider, tools []Tool, logger *slog.Logger) *Agent {
	ag := iteragent.New(p, tools, logger)
	return &Agent{
		Agent:  ag,
		Events: ag.Events,
	}
}
