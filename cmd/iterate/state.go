package main

import (
	"context"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Session state — token counters, timing, request cancellation
// ---------------------------------------------------------------------------

type sessionState struct {
	Tokens        int
	InputTokens   int
	OutputTokens  int
	CacheRead     int
	CacheWrite    int
	CostUSD       float64
	ToolCalls     int
	Messages      int
	Start         time.Time
	RequestCancel context.CancelFunc
}

var sess sessionState

func init() {
	sess.Start = time.Now()
}

func (s *sessionState) RecordToolCall() { s.ToolCalls++ }
func (s *sessionState) RecordMessage()  { s.Messages++ }

func (s *sessionState) Stats() string {
	elapsed := time.Since(s.Start).Round(time.Second)
	return fmt.Sprintf(
		"Duration: %s  |  Messages sent: %d  |  Tool calls: ~%d  |  Output tokens: ~%d",
		elapsed, s.Messages, s.ToolCalls, s.Tokens)
}

// ---------------------------------------------------------------------------
// REPL config — runtime toggles
// ---------------------------------------------------------------------------

type replConfig struct {
	SafeMode          bool
	DebugMode         bool
	NotifyEnabled     bool
	AutoCommitEnabled bool
	RequestTimeout    int // seconds; 0 = default 120s
}

var cfg replConfig
