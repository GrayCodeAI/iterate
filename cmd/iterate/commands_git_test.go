package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parsePRArgs
// ---------------------------------------------------------------------------

func TestParsePRArgs_Empty(t *testing.T) {
	p := parsePRArgs("")
	if p.sub != prSubList {
		t.Errorf("empty args: expected prSubList, got %v", p.sub)
	}
}

func TestParsePRArgs_List(t *testing.T) {
	for _, input := range []string{"list", "ls", "list  "} {
		p := parsePRArgs(input)
		if p.sub != prSubList {
			t.Errorf("input %q: expected prSubList, got %v", input, p.sub)
		}
	}
}

func TestParsePRArgs_View(t *testing.T) {
	p := parsePRArgs("view 42")
	if p.sub != prSubView {
		t.Errorf("expected prSubView, got %v", p.sub)
	}
	if p.number != "42" {
		t.Errorf("expected number 42, got %q", p.number)
	}
}

func TestParsePRArgs_ViewNoNumber(t *testing.T) {
	p := parsePRArgs("view")
	if p.sub != prSubView {
		t.Errorf("expected prSubView, got %v", p.sub)
	}
	if p.number != "" {
		t.Errorf("expected empty number, got %q", p.number)
	}
}

func TestParsePRArgs_BareNumber(t *testing.T) {
	p := parsePRArgs("123")
	if p.sub != prSubView {
		t.Errorf("bare number should map to prSubView, got %v", p.sub)
	}
	if p.number != "123" {
		t.Errorf("expected number 123, got %q", p.number)
	}
}

func TestParsePRArgs_Diff(t *testing.T) {
	p := parsePRArgs("diff 7")
	if p.sub != prSubDiff {
		t.Errorf("expected prSubDiff, got %v", p.sub)
	}
	if p.number != "7" {
		t.Errorf("expected number 7, got %q", p.number)
	}
}

func TestParsePRArgs_DiffNoNumber(t *testing.T) {
	p := parsePRArgs("diff")
	if p.sub != prSubDiff {
		t.Errorf("expected prSubDiff, got %v", p.sub)
	}
	if p.number != "" {
		t.Errorf("expected empty number, got %q", p.number)
	}
}

func TestParsePRArgs_Comment(t *testing.T) {
	p := parsePRArgs("comment 5 looks good")
	if p.sub != prSubComment {
		t.Errorf("expected prSubComment, got %v", p.sub)
	}
	if p.number != "5" {
		t.Errorf("expected number 5, got %q", p.number)
	}
	if p.text != "looks good" {
		t.Errorf("expected text 'looks good', got %q", p.text)
	}
}

func TestParsePRArgs_CommentMultiWord(t *testing.T) {
	p := parsePRArgs("comment 10 this looks great!")
	if p.text != "this looks great!" {
		t.Errorf("expected full comment text, got %q", p.text)
	}
}

func TestParsePRArgs_Checkout(t *testing.T) {
	for _, input := range []string{"checkout 3", "co 3"} {
		p := parsePRArgs(input)
		if p.sub != prSubCheckout {
			t.Errorf("input %q: expected prSubCheckout, got %v", input, p.sub)
		}
		if p.number != "3" {
			t.Errorf("input %q: expected number 3, got %q", input, p.number)
		}
	}
}

func TestParsePRArgs_Create(t *testing.T) {
	for _, input := range []string{"create", "new"} {
		p := parsePRArgs(input)
		if p.sub != prSubCreate {
			t.Errorf("input %q: expected prSubCreate, got %v", input, p.sub)
		}
		if p.draft {
			t.Errorf("input %q: expected draft=false", input)
		}
	}
}

func TestParsePRArgs_CreateDraft(t *testing.T) {
	for _, input := range []string{"create --draft", "new -d", "create -d"} {
		p := parsePRArgs(input)
		if p.sub != prSubCreate {
			t.Errorf("input %q: expected prSubCreate, got %v", input, p.sub)
		}
		if !p.draft {
			t.Errorf("input %q: expected draft=true", input)
		}
	}
}

func TestParsePRArgs_Help(t *testing.T) {
	p := parsePRArgs("notacommand")
	if p.sub != prSubHelp {
		t.Errorf("unknown subcommand should map to prSubHelp, got %v", p.sub)
	}
}

func TestParsePRArgs_WhitespaceOnly(t *testing.T) {
	p := parsePRArgs("   ")
	if p.sub != prSubList {
		t.Errorf("whitespace-only should map to prSubList, got %v", p.sub)
	}
}

func TestParsePRArgs_Review(t *testing.T) {
	p := parsePRArgs("review 99")
	if p.sub != prSubReview {
		t.Errorf("expected prSubReview, got %v", p.sub)
	}
	if p.number != "99" {
		t.Errorf("expected number 99, got %q", p.number)
	}
}

func TestParsePRArgs_ReviewNoNumber(t *testing.T) {
	p := parsePRArgs("review")
	if p.sub != prSubReview {
		t.Errorf("expected prSubReview, got %v", p.sub)
	}
	if p.number != "" {
		t.Errorf("expected empty number, got %q", p.number)
	}
}

// ---------------------------------------------------------------------------
// buildPRReviewPrompt
// ---------------------------------------------------------------------------

func TestBuildPRReviewPrompt_ContainsPRNumber(t *testing.T) {
	prompt := buildPRReviewPrompt("42", "- old line\n+ new line")
	if !strings.Contains(prompt, "PR #42") {
		t.Errorf("prompt missing PR number: %q", prompt)
	}
}

func TestBuildPRReviewPrompt_ContainsDiff(t *testing.T) {
	diff := "- removed\n+ added"
	prompt := buildPRReviewPrompt("1", diff)
	if !strings.Contains(prompt, diff) {
		t.Errorf("prompt missing diff content")
	}
}

func TestBuildPRReviewPrompt_ContainsReviewCriteria(t *testing.T) {
	prompt := buildPRReviewPrompt("7", "")
	for _, keyword := range []string{"correctness", "security", "performance"} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("prompt missing review criterion %q", keyword)
		}
	}
}

func TestBuildPRReviewPrompt_DiffInCodeFence(t *testing.T) {
	prompt := buildPRReviewPrompt("3", "some diff")
	if !strings.Contains(prompt, "```diff") {
		t.Errorf("prompt should wrap diff in a code fence")
	}
}
