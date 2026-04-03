package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Session state — token counters, timing, request cancellation
// ---------------------------------------------------------------------------

type sessionState struct {
	mu            sync.Mutex
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

// budgetLimit is the per-session spending cap in USD (0 = no limit).
// Set by --budget flag or /budget command at runtime.
var budgetLimit float64

func init() {
	sess.Start = time.Now()
}

func (s *sessionState) RecordToolCall()      { s.mu.Lock(); s.ToolCalls++; s.mu.Unlock() }
func (s *sessionState) RecordMessage()       { s.mu.Lock(); s.Messages++; s.mu.Unlock() }
func (s *sessionState) AddTokens(n int)      { s.mu.Lock(); s.Tokens += n; s.mu.Unlock() }
func (s *sessionState) AddCostUSD(c float64) { s.mu.Lock(); s.CostUSD += c; s.mu.Unlock() }

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
