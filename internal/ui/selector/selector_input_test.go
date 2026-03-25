package selector

import (
	"testing"
)

// ---------------------------------------------------------------------------
// handleLineSubmit
// ---------------------------------------------------------------------------

func TestHandleLineSubmit_BasicText(t *testing.T) {
	resetHistory()
	buf := []byte("hello world")
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	done, result, ok := handleLineSubmit(&buf, &hist, &idx, &saved)
	if !done || !ok {
		t.Fatalf("expected done=true, ok=true; got done=%v, ok=%v", done, ok)
	}
	if result != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", result)
	}
}

func TestHandleLineSubmit_TrimsWhitespace(t *testing.T) {
	resetHistory()
	buf := []byte("  spaced  ")
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	_, result, _ := handleLineSubmit(&buf, &hist, &idx, &saved)
	if result != "spaced" {
		t.Errorf("expected trimmed %q, got %q", "spaced", result)
	}
}

func TestHandleLineSubmit_BackslashContinuation(t *testing.T) {
	resetHistory()
	buf := []byte("continuing \\")
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	// Backslash continuation: done=false (keep reading), ok=false (not submitted yet)
	done, _, _ := handleLineSubmit(&buf, &hist, &idx, &saved)
	if done {
		t.Fatalf("backslash continuation should return done=false")
	}
	// buf should now end with newline (backslash replaced)
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		t.Errorf("expected buffer to end with newline after continuation, got %q", string(buf))
	}
}

func TestHandleLineSubmit_AddsToHistory(t *testing.T) {
	resetHistory()
	buf := []byte("my command")
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	handleLineSubmit(&buf, &hist, &idx, &saved)

	h := getInputHistory()
	if len(h) == 0 || h[len(h)-1] != "my command" {
		t.Errorf("expected command in history, got %v", h)
	}
}

// ---------------------------------------------------------------------------
// handleArrowKeys — Up/Down history navigation
// ---------------------------------------------------------------------------

func TestHandleArrowKeys_UpNavigatesBack(t *testing.T) {
	hist := []string{"first", "second", "third"}
	idx := 3
	buf := []byte{}
	saved := []byte(nil)
	var replaced string
	replace := func(s string) { replaced = s }

	handleArrowKeys('A', &buf, &hist, &idx, &saved, replace)

	if idx != 2 {
		t.Errorf("expected idx=2, got %d", idx)
	}
	if replaced != "third" {
		t.Errorf("expected replaced=%q, got %q", "third", replaced)
	}
}

func TestHandleArrowKeys_UpAtBoundaryDoesNotUnderflow(t *testing.T) {
	hist := []string{"only"}
	idx := 0
	buf := []byte{}
	saved := []byte(nil)
	var replaced string
	replace := func(s string) { replaced = s }

	handleArrowKeys('A', &buf, &hist, &idx, &saved, replace)

	if idx != 0 {
		t.Errorf("idx should stay at 0, got %d", idx)
	}
	if replaced != "" {
		t.Errorf("replace should not be called at boundary, got %q", replaced)
	}
}

func TestHandleArrowKeys_DownRestoresSavedBuf(t *testing.T) {
	hist := []string{"a", "b"}
	idx := 1 // already navigated back one step
	buf := []byte("b")
	saved := []byte("current")
	var replaced string
	replace := func(s string) { replaced = s }

	handleArrowKeys('B', &buf, &hist, &idx, &saved, replace)

	if idx != 2 {
		t.Errorf("expected idx=2, got %d", idx)
	}
	if replaced != "current" {
		t.Errorf("expected saved buf restored, got %q", replaced)
	}
}

func TestHandleArrowKeys_DownAtEndDoesNothing(t *testing.T) {
	hist := []string{"a"}
	idx := 1 // already at end
	buf := []byte{}
	saved := []byte(nil)
	var replaced string
	replace := func(s string) { replaced = s }

	handleArrowKeys('B', &buf, &hist, &idx, &saved, replace)

	if replaced != "" {
		t.Errorf("replace should not be called at end, got %q", replaced)
	}
}

// ---------------------------------------------------------------------------
// handleTabCompletion
// ---------------------------------------------------------------------------

func TestHandleTabCompletion_CompletesSlashCommand(t *testing.T) {
	buf := []byte("/hel")
	handleTabCompletion(&buf)
	result := string(buf)
	if result != "/help " {
		t.Errorf("expected /help , got %q", result)
	}
}

func TestHandleTabCompletion_NoChangeOnNoMatch(t *testing.T) {
	buf := []byte("/zzzzz")
	handleTabCompletion(&buf)
	if string(buf) != "/zzzzz" {
		t.Errorf("expected no change, got %q", string(buf))
	}
}

func TestHandleTabCompletion_EmptyBufNoChange(t *testing.T) {
	buf := []byte{}
	handleTabCompletion(&buf)
	if len(buf) != 0 {
		t.Errorf("empty buf should remain empty, got %q", string(buf))
	}
}

// ---------------------------------------------------------------------------
// handleRawInput — key dispatch
// ---------------------------------------------------------------------------

func TestHandleRawInput_CtrlC(t *testing.T) {
	buf := []byte("some text")
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	b := []byte{3, 0, 0, 0}

	done, result, ok := handleRawInput(b, 1, &buf, &hist, &idx, &saved, func(string) {}, 0, nil)
	if !done {
		t.Error("Ctrl+C should return done=true")
	}
	if ok {
		t.Error("Ctrl+C should return ok=false")
	}
	if result != "" {
		t.Errorf("Ctrl+C should return empty result, got %q", result)
	}
}

func TestHandleRawInput_CtrlD(t *testing.T) {
	buf := []byte{}
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	b := []byte{4, 0, 0, 0}

	done, _, ok := handleRawInput(b, 1, &buf, &hist, &idx, &saved, func(string) {}, 0, nil)
	if !done || ok {
		t.Errorf("Ctrl+D should return done=true, ok=false; got done=%v, ok=%v", done, ok)
	}
}

func TestHandleRawInput_Backspace(t *testing.T) {
	buf := []byte("abc")
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	b := []byte{127, 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &hist, &idx, &saved, func(string) {}, 0, nil)
	if done {
		t.Error("backspace should not finish input")
	}
	if string(buf) != "ab" {
		t.Errorf("expected buf=ab after backspace, got %q", string(buf))
	}
}

func TestHandleRawInput_BackspaceOnEmpty(t *testing.T) {
	buf := []byte{}
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	b := []byte{127, 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &hist, &idx, &saved, func(string) {}, 0, nil)
	if done {
		t.Error("backspace on empty should not finish input")
	}
	if len(buf) != 0 {
		t.Errorf("buf should remain empty, got %q", string(buf))
	}
}

func TestHandleRawInput_PrintableCharAppended(t *testing.T) {
	buf := []byte("hel")
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	b := []byte{'o', 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &hist, &idx, &saved, func(string) {}, 0, nil)
	if done {
		t.Error("printable char should not finish input")
	}
	if string(buf) != "helo" {
		t.Errorf("expected helo, got %q", string(buf))
	}
}
