package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func testModel(chrome Chrome) Model {
	m := New(chrome, PreviewBoard())
	um, _ := m.Update(tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
	return um.(Model)
}

func frame(m Model) string { return m.View().Content }

func frameLines(t *testing.T, m Model) []string {
	t.Helper()
	return strings.Split(frame(m), "\n")
}

// key builds a key press message the way Bubble Tea delivers it, so the tests
// exercise the same fields the real input reader fills.
func key(name string) tea.KeyPressMsg {
	switch name {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(name)[0]
	return tea.KeyPressMsg{Code: r, Text: name}
}

func send(m Model, keys ...string) Model {
	cur := m
	for _, k := range keys {
		um, _ := cur.Update(key(k))
		cur = um.(Model)
	}
	return cur
}

func quits(m Model, k string) bool {
	_, cmd := m.Update(key(k))
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestKeyNamesMatchHandlers(t *testing.T) {
	// The handlers match on msg.String(); if a synthesized key did not stringify
	// the way a real terminal delivers it, every binding test below would be
	// testing a fiction.
	for _, k := range []string{"esc", "enter", "up", "down", "backspace", "ctrl+c", "b", "s", "j", "k", "?", "a", "c", "l", "t", "e", "q", "5"} {
		if got := key(k).String(); got != k {
			t.Errorf("key(%q).String() = %q", k, got)
		}
	}
}

func TestBoardFitsMinimumSize(t *testing.T) {
	m := testModel(NewChrome(false, false))
	lines := frameLines(t, m)
	if len(lines) > MinHeight {
		t.Fatalf("board renders %d lines, want at most %d:\n%s", len(lines), MinHeight, frame(m))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > MinWidth {
			t.Errorf("line %d is %d cells wide, want at most %d: %q", i, w, MinWidth, l)
		}
	}
	for _, want := range []string{"RACK WARS", "Day 1/30", "cash", "debt", "PRODUCT", "STOCK", "3090 SLED", "4090", "H100", "[b]uy"} {
		if !strings.Contains(frame(m), want) {
			t.Errorf("board frame missing %q\n%s", want, frame(m))
		}
	}
}

func TestWideLayoutAddsSidePanel(t *testing.T) {
	m := testModel(NewChrome(false, false))
	um, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	wide := um.(Model)
	if !strings.Contains(frame(wide), "DETAIL") {
		t.Fatalf("120-column frame has no side panel:\n%s", frame(wide))
	}
	for i, l := range frameLines(t, wide) {
		if w := ansi.StringWidth(l); w > 120 {
			t.Errorf("wide line %d is %d cells, want at most 120: %q", i, w, l)
		}
	}
	// The minimum layout must not show a panel.
	if strings.Contains(frame(m), "DETAIL") {
		t.Errorf("80-column frame should not show the side panel:\n%s", frame(m))
	}
}

func TestSmallTerminalShowsResizeMessageAndKeepsRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := m.board
	beforeSel := m.sel
	um, _ := m.Update(tea.WindowSizeMsg{Width: 62, Height: 10})
	small := um.(Model)
	got := frame(small)
	for _, want := range []string{"Terminal too small", "62x10", "80x24", "no day passes"} {
		if !strings.Contains(got, want) {
			t.Errorf("small frame missing %q:\n%s", want, got)
		}
	}
	if small.board.Day != before.Day || small.sel != beforeSel {
		t.Errorf("shrinking changed the run: day %d sel %d, want day %d sel %d",
			small.board.Day, small.sel, before.Day, beforeSel)
	}
	lines := frameLines(t, small)
	if len(lines) > 10 {
		t.Errorf("small frame is %d lines tall, want at most 10:\n%s", len(lines), got)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 62 {
			t.Errorf("small line %d is %d cells, want at most 62: %q", i, w, l)
		}
	}
}

func TestResizeNeverAdvancesRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	m = send(m, "j", "b") // move selection, open the quantity box
	before := m.board
	beforeSel, beforeMode := m.sel, m.mode
	for i := range 20 {
		um, _ := m.Update(tea.WindowSizeMsg{Width: 60 + i*4, Height: 12 + i})
		m = um.(Model)
	}
	if m.board.Day != before.Day || fmt.Sprint(m.board) != fmt.Sprint(before) {
		t.Errorf("resizing mutated the run:\nbefore %+v\nafter  %+v", before, m.board)
	}
	if m.sel != beforeSel || m.mode != beforeMode {
		t.Errorf("resizing moved the interface: sel %d->%d mode %v->%v", beforeSel, m.sel, beforeMode, m.mode)
	}
}

func TestNoColorSetsNoForegroundOrBackground(t *testing.T) {
	m := testModel(NewChrome(false, false))
	if hasColorSequence(frame(m)) {
		t.Fatalf("--no-color frame still sets a color: %q", frame(m))
	}
	// Bold and reverse are fine (and needed): they carry no hue, so monochrome
	// terminals still get the emphasis.
	if !strings.Contains(frame(m), "\x1b[") {
		t.Errorf("--no-color frame dropped all emphasis: %q", frame(m))
	}
	// Selection has to stay visible without color.
	if !strings.Contains(frame(send(m, "j")), ">") && !strings.Contains(frame(send(m, "j")), "»") {
		t.Errorf("monochrome frame has no selection marker:\n%s", frame(send(m, "j")))
	}
}

func TestColorFrameUsesStyleSequences(t *testing.T) {
	m := testModel(NewChrome(true, false))
	if !strings.Contains(frame(m), "\x1b[") {
		t.Errorf("color frame has no SGR sequences:\n%q", frame(m))
	}
}

// TestFrameStaysWithinEscapeBudget guards a Lip Gloss v2 rendering hazard: some
// attribute combinations re-emit SGR codes for every single cell, which multiplies
// the bytes written on every keypress and makes slow terminals lag. A frame here
// needs a handful of runs per line, not one per character.
func TestFrameStaysWithinEscapeBudget(t *testing.T) {
	const budget = 80
	for name, m := range map[string]Model{
		"board": testModel(NewChrome(true, false)),
		"wide": func() Model {
			um, _ := testModel(NewChrome(true, false)).Update(tea.WindowSizeMsg{Width: 140, Height: 30})
			return um.(Model)
		}(),
		"help": send(testModel(NewChrome(true, false)), "?"),
		"qty":  send(testModel(NewChrome(true, false)), "b"),
	} {
		f := frame(m)
		n := strings.Count(f, "\x1b[")
		if n > budget {
			t.Errorf("%s frame emits %d escape sequences, budget %d\n%q", name, n, budget, f)
		}
	}
}

// hasColorSequence reports whether s contains an SGR that sets or resets a
// foreground/background color. Emphasis attributes (bold, reverse, dim) do not
// count: they carry no hue.
func hasColorSequence(s string) bool {
	const prefix = "\x1b["
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], prefix)
		if j < 0 {
			return false
		}
		k := i + j + len(prefix)
		end := strings.IndexByte(s[k:], 'm')
		if end < 0 {
			return false
		}
		for _, p := range strings.Split(s[k:k+end], ";") {
			if p == "38" || p == "48" || p == "39" || p == "49" {
				return true
			}
			if n, err := strconv.Atoi(p); err == nil {
				if n >= 30 && n <= 37 || n >= 40 && n <= 47 || n >= 90 && n <= 107 {
					return true
				}
			}
		}
		i = k + end + 1
	}
	return false
}

func TestASCIIModeIsPlainASCII(t *testing.T) {
	for _, mode := range []func(Model) Model{
		func(m Model) Model { return m },
		func(m Model) Model { return send(m, "?") },
		func(m Model) Model { return send(m, "b") },
	} {
		m := mode(testModel(NewChrome(false, true)))
		um, _ := m.Update(tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
		m = um.(Model)
		f := frame(m)
		for i, r := range f {
			if r > 127 {
				t.Fatalf("non-ASCII %q at %d in ascii mode frame:\n%s", r, i, f)
			}
		}
	}
}

func TestWarningUsesTextNotOnlyColor(t *testing.T) {
	m := testModel(NewChrome(true, true))
	if !strings.Contains(frame(m), "[!]") {
		t.Errorf("warning line has no text marker:\n%s", frame(m))
	}
}

func TestLongHeadlineAndNumbersStayInsideColumns(t *testing.T) {
	m := testModel(NewChrome(false, false))
	m.board.Headline = strings.Repeat("STARGATE ANNOUNCES A VERY LARGE NUMBER ", 12)
	m.board.Cash = 123_456_789_012
	m.board.Debt = -98_765_432_100
	m.board.Rows[2].Ask = 999_999_999_999
	for _, width := range []int{80, 81, 99, 120, 200} {
		um, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: MinHeight})
		mm := um.(Model)
		for i, l := range frameLines(t, mm) {
			if w := ansi.StringWidth(l); w > width {
				t.Errorf("width %d line %d is %d cells: %q", width, i, w, l)
			}
		}
	}
}

func TestHelpOverlayFitsMinimumSize(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "?")
	lines := frameLines(t, m)
	if len(lines) > MinHeight {
		t.Fatalf("help frame is %d lines, want at most %d:\n%s", len(lines), MinHeight, frame(m))
	}
	for _, want := range []string{"KEYS", "buy or sell the selected product", "esc", "committing a day"} {
		if !strings.Contains(strings.ToLower(frame(m)), strings.ToLower(want)) {
			t.Errorf("help missing %q:\n%s", want, frame(m))
		}
	}
}

func TestQuantityOverlayFitsMinimumSize(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b")
	lines := frameLines(t, m)
	if len(lines) > MinHeight {
		t.Fatalf("quantity frame is %d lines, want at most %d:\n%s", len(lines), MinHeight, frame(m))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > MinWidth {
			t.Errorf("quantity line %d is %d cells: %q", i, w, l)
		}
	}
	for _, want := range []string{"BUY 3090 SLED", "quantity", "[c] max", "cancel"} {
		if !strings.Contains(frame(m), want) {
			t.Errorf("quantity box missing %q:\n%s", want, frame(m))
		}
	}
}

func TestOverlayKeepsBoardVisible(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b")
	for _, want := range []string{"RACK WARS", "3090 SLED"} {
		if !strings.Contains(frame(m), want) {
			t.Errorf("overlay hid %q:\n%s", want, frame(m))
		}
	}
}
