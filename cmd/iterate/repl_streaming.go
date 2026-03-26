package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	iteragent "github.com/GrayCodeAI/iteragent"
	"github.com/GrayCodeAI/iterate/internal/ui/highlight"
	"github.com/GrayCodeAI/iterate/internal/ui/selector"
)

// toolStyle returns the display icon, label, and ANSI color for a tool name.
func toolStyle(name string) (icon, label, col string) {
	switch name {
	case "bash", "run_command", "run_terminal_cmd":
		return "❯", "bash", colorLime
	case "read_file", "read":
		return "◎", "read", colorCyan
	case "write_file", "create_file":
		return "✎", "write", colorYellow
	case "edit_file":
		return "✎", "edit", colorAmber
	case "search_files", "grep_search", "find_files":
		return "⌕", "search", colorCyan
	case "list_dir", "list_directory":
		return "◈", "ls", colorBlue
	case "web_fetch", "fetch_url":
		return "↓", "fetch", colorBlue
	case "delete_file", "remove_file":
		return "✗", "delete", colorRed
	case "move_file", "rename_file":
		return "→", "move", colorAmber
	case "make_dir", "create_dir":
		return "+", "mkdir", colorGreen
	case "git_commit", "git_push", "git_pull":
		return "⎇", name, colorLime
	default:
		return "⚙", name, colorDim
	}
}

// spinner runs a spinner in the terminal until stop() is called, signals done when exited.
// label is shown next to the spinner (e.g. "thinking" or "bash").
func spinner(stop <-chan struct{}, done chan<- struct{}, label string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if label == "" {
		label = "thinking"
	}
	i := 0
	start := time.Now()
	spinnerActive.Store(1)
	for {
		select {
		case <-stop:
			spinnerActive.Store(0)
			fmt.Print("\r\033[K")
			close(done)
			return
		default:
			elapsed := time.Since(start).Round(time.Millisecond)
			fmt.Printf("\r%s%s%s  %s%s%s  %s%s%s",
				colorLime, frames[i%len(frames)], colorReset,
				colorBold, label, colorReset,
				colorDim, elapsed, colorReset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

// formatToolCallResult formats the result snippet and elapsed time for a completed tool call.
func formatToolCallResult(result string, elapsed time.Duration) string {
	snippet := result
	if len(snippet) > 72 {
		snippet = snippet[:72] + "…"
	}
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippetColor := colorDim
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(result)), "error") {
		snippetColor = colorRed
	}
	return fmt.Sprintf("%s  %s%s%s  %s%s%s%s\n",
		colorDim, colorReset,
		snippetColor, snippet, colorReset,
		colorDim, elapsed, colorReset)
}

// logTokenDelta prints the per-request token usage delta to the status line.
func logTokenDelta(beforeTokens int) {
	delta := sess.Tokens - beforeTokens
	if delta <= 0 {
		return
	}
	fmt.Printf("%s%s+%d tok%s", colorDim, colorPurple, delta, colorReset)
}

// streamAndPrint runs the agent and prints the streamed response.
func streamAndPrint(ctx context.Context, a *iteragent.Agent, prompt string, repoPath string) {
	recordMessage()

	reqCtx, cancel := context.WithCancel(ctx)
	sess.RequestCancel = cancel
	defer func() {
		sess.RequestCancel = nil
		cancel()
	}()

	events := a.Prompt(reqCtx, prompt)
	start := time.Now()

	stopOnce, newSpinner := newSpinnerController()
	newSpinner("thinking")
	defer func() { stopOnce() }()

	var fullContent string
	var toolStart time.Time
	beforeTokens := sess.Tokens

	for e := range events {
		fullContent, toolStart = processStreamEvent(e, fullContent, toolStart, stopOnce, newSpinner, repoPath)
	}
	a.Finish()
	stopOnce()

	if fullContent != "" {
		fmt.Print("\r\033[K")
		highlight.RenderResponse(fullContent)
		fmt.Println()
	}
	maybeNotify()
	elapsed := time.Since(start).Round(time.Millisecond)

	updateSessionTokens(a, fullContent)
	printFinalStats(elapsed, beforeTokens, fullContent)
}

// newSpinnerController creates a spinner control pair (stopOnce, newSpinner).
func newSpinnerController() (func(), func(string)) {
	var (
		stopSpinner  chan struct{}
		spinnerDone  chan struct{}
		spinnerOnce  sync.Once
		stopOnce     func()
		spinnerLabel string
	)
	newSpinner := func(label string) {
		spinnerLabel = label
		stopSpinner = make(chan struct{})
		spinnerDone = make(chan struct{})
		spinnerOnce = sync.Once{}
		stopOnce = func() {
			spinnerOnce.Do(func() {
				close(stopSpinner)
				<-spinnerDone
			})
		}
		go spinner(stopSpinner, spinnerDone, spinnerLabel)
	}
	_ = spinnerLabel
	return stopOnce, newSpinner
}

// processStreamEvent handles a single agent stream event and returns updated state.
func processStreamEvent(e iteragent.Event, fullContent string, toolStart time.Time, stopOnce func(), newSpinner func(string), repoPath string) (string, time.Time) {
	switch iteragent.EventType(e.Type) {
	case iteragent.EventTokenUpdate:
		fullContent += e.Content
	case iteragent.EventMessageUpdate:
	case iteragent.EventToolExecutionStart:
		toolStart = time.Now()
		recordToolCall()
		stopOnce()
		if fullContent != "" {
			fmt.Print("\r\033[K")
			highlight.RenderResponse(fullContent)
			fmt.Println()
			fullContent = ""
		}
		icon, label, col := toolStyle(e.ToolName)
		fmt.Printf("%s%s %s%s", col, icon, label, colorReset)
	case iteragent.EventToolExecutionEnd:
		elapsed := time.Since(toolStart).Round(time.Millisecond)
		printToolResult(e.Result, elapsed, e.ToolName, repoPath)
		newSpinner("thinking")
	case iteragent.EventContextCompacted:
		fmt.Printf("\r\033[K%s[context compacted]%s\n", colorDim, colorReset)
	case iteragent.EventError:
		fmt.Printf("\r\033[K%sError: %s%s\n", colorRed, e.Content, colorReset)
	}
	return fullContent, toolStart
}

// printToolResult formats and displays a completed tool call result.
func printToolResult(result string, elapsed time.Duration, toolName string, repoPath string) {
	fmt.Print(formatToolCallResult(result, elapsed))
	if toolName == "write_file" || toolName == "edit_file" || toolName == "create_file" {
		showGitDiff(repoPath)
	}
}

// updateSessionTokens updates session token counters from the last agent message.
func updateSessionTokens(a *iteragent.Agent, fullContent string) {
	if len(a.Messages) > 0 {
		last := a.Messages[len(a.Messages)-1]
		if last.Usage != nil {
			sess.InputTokens += last.Usage.InputTokens
			sess.OutputTokens += last.Usage.OutputTokens
			sess.CacheRead += last.Usage.CacheRead
			sess.CacheWrite += last.Usage.CacheWrite
			sess.Tokens += last.Usage.TotalTokens
		}
	} else {
		approxTokens := len(fullContent) / 4
		sess.Tokens += approxTokens
		sess.OutputTokens += approxTokens
	}
}

// printFinalStats prints token delta, status line, and debug log.
func printFinalStats(elapsed time.Duration, beforeTokens int, fullContent string) {
	fmt.Println()
	logTokenDelta(beforeTokens)
	fmt.Println()
	selector.InputTokens = sess.InputTokens
	selector.OutputTokens = sess.OutputTokens
	selector.SafeMode = cfg.SafeMode
	selector.PrintStatusLine(elapsed)
	fmt.Println()

	slog.Debug("request completed", "elapsed_ms", elapsed.Milliseconds(), "response_chars", len(fullContent), "total_tokens", sess.Tokens)
}
