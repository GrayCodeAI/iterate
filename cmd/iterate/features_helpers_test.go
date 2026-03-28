package main

import (
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestContextStats(t *testing.T) {
	msgs := []iteragent.Message{
		{Role: "user", Content: strings.Repeat("a", 400)},
		{Role: "assistant", Content: strings.Repeat("b", 400)},
	}
	got := contextStats(msgs)
	if !strings.Contains(got, "Messages: 2") {
		t.Errorf("expected message count, got %q", got)
	}
	if !strings.Contains(got, "200 tokens") {
		t.Errorf("expected ~200 tokens (800 chars / 4), got %q", got)
	}
}

func TestContextStatsEmpty(t *testing.T) {
	got := contextStats(nil)
	if !strings.Contains(got, "Messages: 0") {
		t.Errorf("expected 0 messages, got %q", got)
	}
}

func TestCompactHard(t *testing.T) {
	msgs := make([]iteragent.Message, 10)
	for i := range msgs {
		msgs[i] = iteragent.Message{Role: "user", Content: strings.Repeat("x", i+1)}
	}

	t.Run("keeps last N + first 2", func(t *testing.T) {
		got := compactHard(msgs, 3)
		// first 2 + last 3 = 5
		if len(got) != 5 {
			t.Errorf("expected 5 messages, got %d", len(got))
		}
		if got[0].Content != msgs[0].Content {
			t.Error("expected first message preserved")
		}
		if got[len(got)-1].Content != msgs[len(msgs)-1].Content {
			t.Error("expected last message preserved")
		}
	})

	t.Run("no-op when len <= keepLast", func(t *testing.T) {
		got := compactHard(msgs, 20)
		if len(got) != len(msgs) {
			t.Errorf("expected unchanged length %d, got %d", len(msgs), len(got))
		}
	})

	t.Run("keepLast 0 returns only first 2", func(t *testing.T) {
		got := compactHard(msgs, 0)
		if len(got) != 2 {
			t.Errorf("expected 2 messages, got %d", len(got))
		}
	})
}

func TestFormatPinnedMessages(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := formatPinnedMessages(nil)
		if got != "No pinned messages." {
			t.Errorf("expected empty message, got %q", got)
		}
	})

	t.Run("single short message", func(t *testing.T) {
		msgs := []iteragent.Message{{Role: "user", Content: "remember this"}}
		got := formatPinnedMessages(msgs)
		if !strings.Contains(got, "1") || !strings.Contains(got, "remember this") {
			t.Errorf("unexpected output: %q", got)
		}
	})

	t.Run("long content truncated", func(t *testing.T) {
		long := strings.Repeat("x", 100)
		msgs := []iteragent.Message{{Role: "user", Content: long}}
		got := formatPinnedMessages(msgs)
		if !strings.Contains(got, "…") {
			t.Errorf("expected truncation ellipsis, got %q", got)
		}
	})

	t.Run("multiple messages numbered", func(t *testing.T) {
		msgs := []iteragent.Message{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "second"},
		}
		got := formatPinnedMessages(msgs)
		if !strings.Contains(got, "1") || !strings.Contains(got, "2") {
			t.Errorf("expected numbered entries, got %q", got)
		}
	})
}
