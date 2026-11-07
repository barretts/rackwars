package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// testModel builds a real engine-backed model on seed 1 at the minimum size, so
// every test drives the same rules the shipped game runs on.
func testModel(chrome Chrome) Model {
	m, err := NewRun(chrome, 1)
	if err != nil {
		panic("testModel: " + err.Error())
	}
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
	before, beforeDay := facts(m), m.run.State.Day
	beforeSel := m.sel
	um, _ := m.Update(tea.WindowSizeMsg{Width: 62, Height: 10})
	small := um.(Model)
	got := frame(small)
	for _, want := range []string{"Terminal too small", "62x10", "80x24", "no day passes"} {
		if !strings.Contains(got, want) {
			t.Errorf("small frame missing %q:\n%s", want, got)
		}
	}
	if small.run.State.Day != beforeDay || small.sel != beforeSel || facts(small) != before {
		t.Errorf("shrinking changed the run: day %d sel %d facts %s, want day %d sel %d facts %s",
			small.run.State.Day, small.sel, facts(small), beforeDay, beforeSel, before)
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
	before := facts(m)
	beforeSel, beforeMode := m.sel, m.mode
	for i := range 20 {
		um, _ := m.Update(tea.WindowSizeMsg{Width: 60 + i*4, Height: 12 + i})
		m = um.(Model)
	}
	if facts(m) != before {
		t.Errorf("resizing mutated the run:\nbefore %s\nafter  %s", before, facts(m))
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

// TestLongHeadlineAndNumbersStayInsideColumns feeds the renderer a board whose
// text and figures are deliberately absurd. The engine cannot produce these, but
// the layout contract has to hold for anything the engine ever will, so this
// tests mainLines directly rather than poking at run state.
func TestLongHeadlineAndNumbersStayInsideColumns(t *testing.T) {
	m := testModel(NewChrome(false, false))
	b := m.snapshot()
	b.Headline = strings.Repeat("STARGATE ANNOUNCES A VERY LARGE NUMBER ", 12)
	b.Warning = strings.Repeat("EVERYTHING YOU OWN COULD BE TAKEN ", 8)
	b.Cash = 123_456_789_012
	b.Debt = -98_765_432_100
	if len(b.Rows) > 2 {
		b.Rows[2].Ask = 999_999_999_999
	}
	b.Side = append(b.Side, strings.Repeat("NESTED DETAIL TEXT ", 20))
	for _, width := range []int{80, 81, 99, 120, 200} {
		um, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: MinHeight})
		mm := um.(Model)
		for _, lines := range [][]string{mm.mainLines(b, width), mm.wideBoardLines(b)} {
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > width {
					t.Errorf("width %d line %d is %d cells: %q", width, i, w, l)
				}
			}
		}
	}
}

// TestProductTableCarriesCardFacts pins the owner's card-details request: every
// board row names the card's per-unit draw, ordinary per-night hosting, and
// class in fixed columns, so pricing a night never needs a second document.
func TestProductTableCarriesCardFacts(t *testing.T) {
	m := testModel(NewChrome(false, false))
	b := m.snapshot()
	um, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: MinHeight})
	mm := um.(Model)
	lines := mm.mainLines(b, 80)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "HOST/NGT") || !strings.Contains(joined, "CLASS") {
		t.Fatalf("the table header lost the card facts:\n%s", joined)
	}
	check := func(t *testing.T, lines []string, want struct {
		name, kw, host, class string
	}) {
		t.Helper()
		var found bool
		for _, l := range lines {
			cells := []rune(ansi.Strip(l))
			if len(cells) < 78 || string(cells[2:14]) != fmt.Sprintf("%12s", want.name) {
				continue
			}
			found = true
			if got := string(cells[57:63]); got != fmt.Sprintf("%6s", want.kw) {
				t.Errorf("%s kW column %q, want %q", want.name, got, want.kw)
			}
			if got := string(cells[64:72]); got != fmt.Sprintf("%8s", want.host) {
				t.Errorf("%s host column %q, want %q", want.name, got, want.host)
			}
			if got := string(cells[73:78]); got != fmt.Sprintf("%5s", want.class) {
				t.Errorf("%s class column %q, want %q", want.name, got, want.class)
			}
		}
		if !found {
			t.Errorf("no board row named %q:\n%s", want.name, strings.Join(lines, "\n"))
		}
	}
	for _, want := range []struct{ name, kw, host, class string }{
		{"3090 SLED", "1.60", "$160", "CON"},
		{"4090", "0.50", "$70", "CON"},
		{"H100", "0.90", "$350", "ENT"},
	} {
		check(t, lines, want)
	}
	// The mod and the second enterprise card only reach a board in the full
	// five-market game, where the shop shelves all six products.
	fm := mustFullModel(t, 1)
	flines := fm.mainLines(fm.snapshot(), 80)
	for _, want := range []struct{ name, kw, host, class string }{
		{"4090-48 MOD", "0.60", "$90", "MOD"},
		{"MI300", "1.00", "$250", "ENT"},
	} {
		check(t, flines, want)
	}
}

// TestProseWrapsInsteadOfTruncating is the owner's complaint made permanent:
// a news headline and a deal warning longer than the screen must reach the
// player whole, word for word, instead of dying at an ellipsis.
func TestProseWrapsInsteadOfTruncating(t *testing.T) {
	m := testModel(NewChrome(false, false))
	b := m.snapshot()
	b.Headline = strings.Repeat("STARGATE ANNOUNCES A VERY LARGE NUMBER ", 12)
	b.Warning = "CheapSeek's enterprise deal: sign now for $8,000,000 down; " +
		"the signed deal adds $8,000,000 cash and $6,000,000 debt; the site is " +
		"tied for 30 days; breaking early repays everything and costs $2,400,000."
	um, _ := m.Update(tea.WindowSizeMsg{Width: MinWidth, Height: 40})
	mm := um.(Model)
	lines := mm.mainLines(b, MinWidth)
	joined := strings.Join(lines, " ")
	if strings.Contains(joined, "…") || strings.Contains(joined, "...") {
		t.Errorf("wrapped prose lost words to an ellipsis:\n%s", strings.Join(lines, "\n"))
	}
	if got := strings.Count(joined, "STARGATE"); got != 12 {
		t.Errorf("headline repeats: got %d, want 12:\n%s", got, strings.Join(lines, "\n"))
	}
	if got := strings.Count(joined, "repays"); got != 1 {
		t.Errorf("warning tail missing (found %d):\n%s", got, strings.Join(lines, "\n"))
	}
	if !strings.Contains(joined, "$2,400,000.") {
		t.Errorf("warning's last figure cut:\n%s", strings.Join(lines, "\n"))
	}
	if got := strings.Count(joined, "tied for 30 days"); got != 1 {
		t.Errorf("deal terms missing:\n%s", strings.Join(lines, "\n"))
	}
}

// TestFitHeightKeepsKeysVisible: when wrapped prose overruns the screen the
// frame still ends on the key line — the board's hands must stay reachable —
// and the player is told how much fell off.
func TestFitHeightKeepsKeysVisible(t *testing.T) {
	m := testModel(NewChrome(false, false))
	b := m.snapshot()
	b.Headline = strings.Repeat("A LONG STORM OF NUMBERS AND RUIN ", 40)
	um, _ := m.Update(tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
	mm := um.(Model)
	lines := mm.fitHeight(mm.mainLines(b, MinWidth))
	if len(lines) != MinHeight {
		t.Fatalf("frame is %d lines, want %d", len(lines), MinHeight)
	}
	last := ansi.Strip(lines[len(lines)-1])
	if !strings.Contains(last, "[b]uy") {
		t.Errorf("key line missing from the bottom:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(ansi.Strip(lines[MinHeight-2]), "hidden") {
		t.Errorf("no notice of hidden lines:\n%s", strings.Join(lines, "\n"))
	}
}

// TestWrapFlowCutsOverlongWords: prose with no spaces at all (a board's
// headline could carry a long URL-like token) still cannot exceed the width,
// and no cells are lost by the cut.
func TestWrapFlowCutsOverlongWords(t *testing.T) {
	c := NewChrome(false, false)
	s := strings.Repeat("X", 255)
	lines := c.wrapFlow(s, 80)
	if got := strings.Join(lines, ""); got != s {
		t.Errorf("force-cut lost or reordered cells: %q", got)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 80 {
			t.Errorf("line %d is %d cells", i, w)
		}
	}
	words := strings.Repeat("alpha bravo charlie ", 30)
	out := c.wrapFlow(words, 40)
	if got := strings.Join(out, " "); got != strings.TrimSpace(words) {
		t.Errorf("word wrap lost words:\n%q", got)
	}
	for i, l := range out {
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("line %d is %d cells: %q", i, w, l)
		}
	}
}

func TestHelpOverlayFitsMinimumSize(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "?")
	lines := frameLines(t, m)
	if len(lines) > MinHeight {
		t.Fatalf("help frame is %d lines, want at most %d:\n%s", len(lines), MinHeight, frame(m))
	}
	for _, want := range []string{"KEYS", "buy or sell the selected product", "esc", "advances the clock"} {
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

// facts captures the run-level numbers a test must not perturb. The model holds
// a pointer to the run, so comparing a copied Board struct would let an in-place
// engine mutation slip through; these scalars cannot.
func facts(m Model) string {
	r := m.run
	if r == nil {
		return "no run"
	}
	st := r.State
	owned, stock := 0, 0
	for _, p := range r.Products() {
		owned += r.OwnedOf(p)
		stock += r.StockOf(p)
	}
	return fmt.Sprintf("day=%d town=%s cash=%d debt=%d heat=%d owned=%d stock=%d slots=%d",
		st.Day, st.Market, st.Cash, st.Debt, st.Heat, owned, stock, r.SlotsUsed())
}

// statusLineOf reads the board's live status text with styling turned off.
func statusLineOf(m Model) string {
	return ansi.Strip(m.statusLine(m.snapshot()))
}
