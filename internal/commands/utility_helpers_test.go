package commands

import (
	"strings"
	"testing"

	iteragent "github.com/GrayCodeAI/iteragent"
)

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tt := range tests {
		got := formatTokenCount(tt.n)
		if got != tt.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestHtmlEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a & b", "a &amp; b"},
		{`say "hi"`, "say &quot;hi&quot;"},
		{"<>&\"", "&lt;&gt;&amp;&quot;"},
	}
	for _, tt := range tests {
		got := htmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("htmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHighlightCodeBlocks(t *testing.T) {
	t.Run("with language tag", func(t *testing.T) {
		input := "```go\nfmt.Println()\n```\n"
		got := highlightCodeBlocks(input)
		if !strings.Contains(got, `<pre><code class="language-go">`) {
			t.Errorf("expected language class, got %q", got)
		}
		if !strings.Contains(got, "</code></pre>") {
			t.Errorf("expected closing tag, got %q", got)
		}
	})

	t.Run("without language tag", func(t *testing.T) {
		input := "```\nsome code\n```\n"
		got := highlightCodeBlocks(input)
		if !strings.Contains(got, "<pre><code>") {
			t.Errorf("expected plain pre/code, got %q", got)
		}
	})

	t.Run("no code blocks", func(t *testing.T) {
		input := "just plain text\n"
		got := highlightCodeBlocks(input)
		if !strings.Contains(got, "just plain text") {
			t.Errorf("expected plain text preserved, got %q", got)
		}
		if strings.Contains(got, "<pre>") || strings.Contains(got, "<code>") {
			t.Errorf("expected no code tags for plain text, got %q", got)
		}
	})

	t.Run("unclosed code block", func(t *testing.T) {
		input := "```go\nfmt.Println()"
		got := highlightCodeBlocks(input)
		if !strings.Contains(got, "</code></pre>") {
			t.Errorf("expected auto-closed tag for unclosed block, got %q", got)
		}
	})
}

func TestCompactMessages(t *testing.T) {
	msgs := func(roles ...string) []iteragent.Message {
		var out []iteragent.Message
		for i, r := range roles {
			out = append(out, iteragent.Message{Role: r, Content: r + string(rune('0'+i))})
		}
		return out
	}

	t.Run("keeps system messages", func(t *testing.T) {
		input := msgs("system", "user", "assistant")
		got := compactMessages(input, nil)
		hasSystem := false
		for _, m := range got {
			if m.Role == "system" {
				hasSystem = true
			}
		}
		if !hasSystem {
			t.Error("expected system message to be kept")
		}
	})

	t.Run("pins survive compaction", func(t *testing.T) {
		pin := iteragent.Message{Role: "user", Content: "important pinned message"}
		input := []iteragent.Message{
			{Role: "user", Content: "old1"},
			{Role: "assistant", Content: "old2"},
			pin,
		}
		got := compactMessages(input, []iteragent.Message{pin})
		found := false
		for _, m := range got {
			if m.Content == pin.Content {
				found = true
			}
		}
		if !found {
			t.Error("expected pinned message to survive compaction")
		}
	})

	t.Run("deduplicates system messages", func(t *testing.T) {
		sys := iteragent.Message{Role: "system", Content: "sys prompt"}
		input := []iteragent.Message{sys, sys, {Role: "user", Content: "hi"}}
		got := compactMessages(input, nil)
		count := 0
		for _, m := range got {
			if m.Role == "system" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 system message, got %d", count)
		}
	})
}
