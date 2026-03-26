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
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	done, result, ok := handleLineSubmit(&buf, &cursorPos, &hist, &idx, &saved)
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
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	_, result, _ := handleLineSubmit(&buf, &cursorPos, &hist, &idx, &saved)
	if result != "spaced" {
		t.Errorf("expected trimmed %q, got %q", "spaced", result)
	}
}

func TestHandleLineSubmit_BackslashContinuation(t *testing.T) {
	resetHistory()
	buf := []byte("continuing \\")
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	// Backslash continuation: done=false (keep reading), ok=false (not submitted yet)
	done, _, _ := handleLineSubmit(&buf, &cursorPos, &hist, &idx, &saved)
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
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)

	handleLineSubmit(&buf, &cursorPos, &hist, &idx, &saved)

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
	cursorPos := len(buf)
	handleTabCompletion(&buf, &cursorPos)
	result := string(buf)
	if result != "/help " {
		t.Errorf("expected /help , got %q", result)
	}
	if cursorPos != len(buf) {
		t.Errorf("cursor should be at end after completion, got %d want %d", cursorPos, len(buf))
	}
}

func TestHandleTabCompletion_NoChangeOnNoMatch(t *testing.T) {
	buf := []byte("/zzzzz")
	cursorPos := len(buf)
	handleTabCompletion(&buf, &cursorPos)
	if string(buf) != "/zzzzz" {
		t.Errorf("expected no change, got %q", string(buf))
	}
}

func TestHandleTabCompletion_EmptyBufNoChange(t *testing.T) {
	buf := []byte{}
	cursorPos := 0
	handleTabCompletion(&buf, &cursorPos)
	if len(buf) != 0 {
		t.Errorf("empty buf should remain empty, got %q", string(buf))
	}
}

// ---------------------------------------------------------------------------
// handleRawInput — key dispatch
// ---------------------------------------------------------------------------

// mkState creates zeroed-out raw input state for testing.
func mkState() (*[]byte, *int, *[]string, *int, *[]byte, *string) {
	buf := &[]byte{}
	cursorPos := new(int)
	hist := &[]string{}
	idx := new(int)
	saved := &[]byte{}
	kill := new(string)
	return buf, cursorPos, hist, idx, saved, kill
}

func TestHandleRawInput_CtrlC(t *testing.T) {
	// First Ctrl+C with non-empty buf clears the line (stage 1).
	buf := []byte("some text")
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{3, 0, 0, 0}

	done, _, ok := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if done {
		t.Error("Ctrl+C with non-empty buf should NOT return done on first press (stage 1 clears line)")
	}
	if len(buf) != 0 {
		t.Errorf("Ctrl+C stage 1 should clear buf, got %q", string(buf))
	}
	if cursorPos != 0 {
		t.Errorf("cursor should be reset to 0, got %d", cursorPos)
	}

	// Second Ctrl+C with empty buf exits (stage 2).
	done, result, ok := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if !done {
		t.Error("Ctrl+C with empty buf should return done=true (stage 2)")
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
	cursorPos := 0
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{4, 0, 0, 0}

	done, _, ok := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if !done || ok {
		t.Errorf("Ctrl+D should return done=true, ok=false; got done=%v, ok=%v", done, ok)
	}
}

func TestHandleRawInput_Backspace(t *testing.T) {
	buf := []byte("abc")
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{127, 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if done {
		t.Error("backspace should not finish input")
	}
	if string(buf) != "ab" {
		t.Errorf("expected buf=ab after backspace, got %q", string(buf))
	}
	if cursorPos != 2 {
		t.Errorf("cursor should be at 2 after backspace, got %d", cursorPos)
	}
}

func TestHandleRawInput_BackspaceOnEmpty(t *testing.T) {
	buf := []byte{}
	cursorPos := 0
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{127, 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if done {
		t.Error("backspace on empty should not finish input")
	}
	if len(buf) != 0 {
		t.Errorf("buf should remain empty, got %q", string(buf))
	}
}

func TestHandleRawInput_BackspaceMidLine(t *testing.T) {
	// Backspace with cursor in the middle deletes char before cursor.
	buf := []byte("abcd")
	cursorPos := 2 // cursor after 'b'
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{127, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != "acd" {
		t.Errorf("expected acd, got %q", string(buf))
	}
	if cursorPos != 1 {
		t.Errorf("cursor should be at 1, got %d", cursorPos)
	}
}

func TestHandleRawInput_PrintableCharAppended(t *testing.T) {
	buf := []byte("hel")
	cursorPos := len(buf)
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{'o', 0, 0, 0}

	done, _, _ := handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if done {
		t.Error("printable char should not finish input")
	}
	if string(buf) != "helo" {
		t.Errorf("expected helo, got %q", string(buf))
	}
	if cursorPos != 4 {
		t.Errorf("cursor should be at 4, got %d", cursorPos)
	}
}

func TestHandleRawInput_InsertMidLine(t *testing.T) {
	// Insert a character in the middle of the buffer.
	buf := []byte("hllo")
	cursorPos := 1 // cursor after 'h'
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{'e', 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != "hello" {
		t.Errorf("expected hello after mid-insert, got %q", string(buf))
	}
	if cursorPos != 2 {
		t.Errorf("cursor should advance to 2, got %d", cursorPos)
	}
}

func TestHandleRawInput_LeftArrow(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 5
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	// ESC [ D = left arrow
	b := []byte{27, '[', 'D', 0}

	handleRawInput(b, 3, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 4 {
		t.Errorf("left arrow should move cursor from 5 to 4, got %d", cursorPos)
	}
}

func TestHandleRawInput_LeftArrowAtStart(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 0
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{27, '[', 'D', 0}

	handleRawInput(b, 3, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 0 {
		t.Errorf("left arrow at start should not go below 0, got %d", cursorPos)
	}
}

func TestHandleRawInput_RightArrow(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 2
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{27, '[', 'C', 0}

	handleRawInput(b, 3, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 3 {
		t.Errorf("right arrow should move cursor from 2 to 3, got %d", cursorPos)
	}
}

func TestHandleRawInput_RightArrowAtEnd(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 5
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{27, '[', 'C', 0}

	handleRawInput(b, 3, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 5 {
		t.Errorf("right arrow at end should stay at 5, got %d", cursorPos)
	}
}

func TestHandleRawInput_DeleteKey(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 2 // cursor after 'e', before 'l'
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	// ESC [ 3 ~ = Delete key
	b := []byte{27, '[', '3', '~'}

	handleRawInput(b, 4, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != "helo" {
		t.Errorf("delete key should remove char at cursor, expected helo, got %q", string(buf))
	}
	if cursorPos != 2 {
		t.Errorf("cursor should stay at 2 after delete, got %d", cursorPos)
	}
}

func TestHandleRawInput_CtrlA(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 5
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{1, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 0 {
		t.Errorf("Ctrl+A should move cursor to 0, got %d", cursorPos)
	}
}

func TestHandleRawInput_CtrlE(t *testing.T) {
	buf := []byte("hello")
	cursorPos := 2
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{5, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if cursorPos != 5 {
		t.Errorf("Ctrl+E should move cursor to end (5), got %d", cursorPos)
	}
}

func TestHandleRawInput_CtrlK(t *testing.T) {
	buf := []byte("hello world")
	cursorPos := 5
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{11, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != "hello" {
		t.Errorf("Ctrl+K should kill to end, expected hello, got %q", string(buf))
	}
	if cursorPos != 5 {
		t.Errorf("cursor should stay at 5 after Ctrl+K, got %d", cursorPos)
	}
	if kill != " world" {
		t.Errorf("kill ring should contain \" world\", got %q", kill)
	}
}

func TestHandleRawInput_CtrlU(t *testing.T) {
	buf := []byte("hello world")
	cursorPos := 5
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := ""
	b := []byte{21, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != " world" {
		t.Errorf("Ctrl+U should kill to beginning, expected \" world\", got %q", string(buf))
	}
	if cursorPos != 0 {
		t.Errorf("cursor should be at 0, got %d", cursorPos)
	}
	if kill != "hello" {
		t.Errorf("kill ring should contain \"hello\", got %q", kill)
	}
}

func TestHandleRawInput_CtrlY(t *testing.T) {
	buf := []byte("world")
	cursorPos := 0
	hist := []string{}
	idx := 0
	saved := []byte(nil)
	kill := "hello "
	b := []byte{25, 0, 0, 0}

	handleRawInput(b, 1, &buf, &cursorPos, &hist, &idx, &saved, &kill, func(string) {}, 0, nil)
	if string(buf) != "hello world" {
		t.Errorf("Ctrl+Y should yank kill ring, expected \"hello world\", got %q", string(buf))
	}
	if cursorPos != 6 {
		t.Errorf("cursor should be at 6 after yank, got %d", cursorPos)
	}
}

func TestMoveWordBackward(t *testing.T) {
	cases := []struct {
		buf       string
		startPos  int
		wantPos   int
	}{
		{"hello world", 11, 6},  // end → start of "world"
		{"hello world", 6, 0},   // start of "world" → start of "hello"
		{"hello world", 5, 0},   // space before "world" → start of "hello"
		{"hello world", 0, 0},   // already at start → no movement
		{"  hello", 7, 2},       // end of "hello" → start of "hello"
	}
	for _, tc := range cases {
		buf := []byte(tc.buf)
		pos := tc.startPos
		moveWordBackward(&buf, &pos)
		if pos != tc.wantPos {
			t.Errorf("moveWordBackward(%q, %d) = %d, want %d", tc.buf, tc.startPos, pos, tc.wantPos)
		}
	}
}

func TestMoveWordForward(t *testing.T) {
	cases := []struct {
		buf       string
		startPos  int
		wantPos   int
	}{
		{"hello world", 0, 5},   // start → end of "hello"
		{"hello world", 5, 11},  // space → end of "world"
		{"hello world", 11, 11}, // already at end → no movement
		{"hello  world", 5, 12}, // double space → end of "world"
	}
	for _, tc := range cases {
		buf := []byte(tc.buf)
		pos := tc.startPos
		moveWordForward(&buf, &pos)
		if pos != tc.wantPos {
			t.Errorf("moveWordForward(%q, %d) = %d, want %d", tc.buf, tc.startPos, pos, tc.wantPos)
		}
	}
}
