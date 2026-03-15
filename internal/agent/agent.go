package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/yourusername/iterate/internal/provider"
)

// Agent is the core reasoning loop.
type Agent struct {
	provider provider.Provider
	tools    map[string]Tool
	logger   *slog.Logger
	Events   chan Event // streams events to subscribers (web UI)
}

// Event represents a step in the agent's reasoning.
type Event struct {
	Type    string // "thought", "tool_call", "tool_result", "done", "error"
	Content string
}

// New creates a new Agent.
func New(p provider.Provider, tools []Tool, logger *slog.Logger) *Agent {
	return &Agent{
		provider: p,
		tools:    ToolMap(tools),
		logger:   logger,
		Events:   make(chan Event, 64),
	}
}

// Run executes the agent loop with the given system prompt and user message.
// It continues looping until the LLM produces no more tool calls.
func (a *Agent) Run(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	allTools := make([]Tool, 0, len(a.tools))
	for _, t := range a.tools {
		allTools = append(allTools, t)
	}

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt + "\n\n" + ToolDescriptions(allTools)},
		{Role: "user", Content: userMessage},
	}

	const maxIterations = 20
	for i := range maxIterations {
		a.logger.Info("agent iteration", "step", i+1)

		response, err := a.provider.Complete(ctx, messages)
		if err != nil {
			a.emit(Event{Type: "error", Content: err.Error()})
			return "", fmt.Errorf("provider error at step %d: %w", i+1, err)
		}

		a.emit(Event{Type: "thought", Content: response})

		// Append assistant turn
		messages = append(messages, provider.Message{
			Role:    "assistant",
			Content: response,
		})

		// Parse and execute tool calls
		calls := ParseToolCalls(response)
		if len(calls) == 0 {
			// No more tool calls — agent is done
			a.emit(Event{Type: "done", Content: response})
			return response, nil
		}

		var toolResults strings.Builder
		for _, call := range calls {
			tool, ok := a.tools[call.Tool]
			if !ok {
				result := fmt.Sprintf("unknown tool: %s", call.Tool)
				toolResults.WriteString(fmt.Sprintf("Tool %s: %s\n", call.Tool, result))
				a.emit(Event{Type: "tool_result", Content: result})
				continue
			}

			a.emit(Event{Type: "tool_call", Content: fmt.Sprintf("%s(%v)", call.Tool, call.Args)})
			a.logger.Info("executing tool", "tool", call.Tool, "args", call.Args)

			result, err := tool.Execute(ctx, call.Args)
			if err != nil {
				result = fmt.Sprintf("ERROR: %s\nOutput: %s", err.Error(), result)
			}

			a.emit(Event{Type: "tool_result", Content: result})
			toolResults.WriteString(fmt.Sprintf("Tool %s result:\n%s\n\n", call.Tool, result))
		}

		// Feed results back as a user message
		messages = append(messages, provider.Message{
			Role:    "user",
			Content: toolResults.String(),
		})
	}

	return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}

func (a *Agent) emit(e Event) {
	select {
	case a.Events <- e:
	default:
		// Drop if buffer full — non-blocking
	}
}
