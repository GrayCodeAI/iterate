package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	startTokens := streamingTokenCount.Load()
	spinnerActive.Store(1)
	for {
		select {
		case <-stop:
			spinnerActive.Store(0)
			fmt.Print("\r\033[K")
			close(done)
			return
		default:
			elapsed := time.Since(start)
			elapsedDisplay := elapsed.Round(time.Millisecond)
			toksDelta := streamingTokenCount.Load() - startTokens
			var toksStr string
			if toksDelta > 0 && elapsed.Seconds() > 0.1 {
				toksPerSec := float64(toksDelta) / elapsed.Seconds()
				toksStr = fmt.Sprintf("  %s%.0f tok/s%s", colorDim, toksPerSec, colorReset)
			}
			fmt.Printf("\r%s%s%s  %s%s%s  %s%s%s%s",
				colorLime, frames[i%len(frames)], colorReset,
				colorBold, label, colorReset,
				colorDim, elapsedDisplay, colorReset,
				toksStr)
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

// formatSessionCost formats total session cost for the session summary.
func formatSessionCost(usd float64) string {
	if usd < 0.0001 {
		return "<$0.0001"
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
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
	beginUndoFrame()
	defer commitUndoFrame()

	recordMessage()

	// Sync pinned messages into the agent before each request.
	a.SetPinnedMessages(getPinnedMessages())
	streamingTokenCount.Store(0)

	timeoutSecs := cfg.RequestTimeout
	if timeoutSecs <= 0 {
		timeoutSecs = 120
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
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
	var ttft time.Duration
	beforeTokens := sess.Tokens

	for e := range events {
		if ttft == 0 && iteragent.EventType(e.Type) == iteragent.EventTokenUpdate && e.Content != "" {
			ttft = time.Since(start).Round(time.Millisecond)
		}
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

	inDelta, outDelta, cacheRdDelta, cacheWrDelta := updateSessionTokens(a, fullContent)
	model := os.Getenv("ITERATE_MODEL")
	requestCost := estimateCost(inDelta, outDelta, cacheWrDelta, cacheRdDelta, model)
	if requestCost.Found {
		sess.CostUSD += requestCost.Total
	}
	printFinalStats(elapsed, ttft, beforeTokens, requestCost.Total, fullContent)

	// Autosave after each turn so a crash doesn't lose the session.
	if len(a.Messages) > 0 {
		_ = saveSession("autosave", a.Messages)
	}
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
	stopOnce = func() {} // no-op until a spinner is started
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
	// Return a wrapper that always calls the *current* stopOnce, not the
	// initial no-op captured at return time.
	return func() { stopOnce() }, newSpinner
}

// processStreamEvent handles a single agent stream event and returns updated state.
func processStreamEvent(e iteragent.Event, fullContent string, toolStart time.Time, stopOnce func(), newSpinner func(string), repoPath string) (string, time.Time) {
	switch iteragent.EventType(e.Type) {
	case iteragent.EventTokenUpdate:
		fullContent += e.Content
		streamingTokenCount.Add(1)
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
		if e.IsError {
			fmt.Printf("%s  ✗ %s%s  %s%s%s\n",
				colorDim, colorReset,
				colorRed, e.Result, colorReset,
				colorDim)
			fmt.Print(colorReset)
		} else {
			printToolResult(e.Result, elapsed, e.ToolName, repoPath)
		}
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

// updateSessionTokens updates session token counters from the last assistant
// message with usage data. Searches backwards since tool result messages (role
// user) may appear after the final assistant message.
// Returns the token delta for the request.
func updateSessionTokens(a *iteragent.Agent, fullContent string) (inputDelta, outputDelta, cacheReadDelta, cacheWriteDelta int) {
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if a.Messages[i].Usage != nil {
			u := a.Messages[i].Usage
			sess.InputTokens += u.InputTokens
			sess.OutputTokens += u.OutputTokens
			sess.CacheRead += u.CacheRead
			sess.CacheWrite += u.CacheWrite
			sess.Tokens += u.TotalTokens
			return u.InputTokens, u.OutputTokens, u.CacheRead, u.CacheWrite
		}
	}
	// Fallback: approximate from streamed content length.
	approxTokens := len(fullContent) / 4
	sess.Tokens += approxTokens
	sess.OutputTokens += approxTokens
	return 0, approxTokens, 0, 0
}

// printFinalStats prints the status line and debug log.
func printFinalStats(elapsed, ttft time.Duration, beforeTokens int, requestCostUSD float64, fullContent string) {
	delta := sess.Tokens - beforeTokens

	fmt.Println()
	selector.InputTokens = sess.InputTokens
	selector.OutputTokens = sess.OutputTokens
	selector.SafeMode = cfg.SafeMode
	selector.TTFT = ttft
	selector.RequestCostUSD = requestCostUSD
	selector.PrintStatusLine(elapsed, delta)
	fmt.Println()

	slog.Debug("request completed", "elapsed_ms", elapsed.Milliseconds(), "ttft_ms", ttft.Milliseconds(), "response_chars", len(fullContent), "total_tokens", sess.Tokens, "cost_usd", requestCostUSD)
}
