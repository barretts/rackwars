package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

func TestSelectionMovesWithArrowsAndVimKeys(t *testing.T) {
	m := testModel(NewChrome(false, false))
	n := len(m.run.Products())
	if got := send(m, "k").sel; got != n-1 {
		t.Errorf("k from the first row selected %d, want the last row", got)
	}
	if got := send(m, "up").sel; got != n-1 {
		t.Errorf("up from the first row selected %d, want the last row", got)
	}
	if got := send(m, "j", "j").sel; got != 2 {
		t.Errorf("j j selected %d, want 2", got)
	}
	if got := send(m, "down").sel; got != 1 {
		t.Errorf("down selected %d, want 1", got)
	}
	if got := send(m, "j", "j", "j").sel; got != 0 {
		t.Errorf("moving past the last row landed on %d, want 0", got)
	}
}

func TestSelectionOnlyChangesSelection(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := facts(m)
	got := send(m, "j", "j", "k")
	if facts(got) != before {
		t.Errorf("navigation mutated the run:\nbefore %s\nafter  %s", before, facts(got))
	}
}

func TestQuitKeysExitOrderly(t *testing.T) {
	m := testModel(NewChrome(false, false))
	for _, k := range []string{"q", "ctrl+c"} {
		if !quits(m, k) {
			t.Errorf("%q did not request an orderly quit", k)
		}
	}
}

func TestCtrlCQuitsFromEveryMode(t *testing.T) {
	m := testModel(NewChrome(false, false))
	overlays := map[string]Model{
		"help":     send(m, "?"),
		"quantity": send(m, "b"),
		"lender":   send(m, "l"),
		"travel":   send(m, "t"),
		"confirm":  send(m, "e"),
	}
	for name, overlay := range overlays {
		if want := overlayWanted[name]; overlay.mode != want {
			t.Fatalf("%s box did not open: mode %v", name, overlay.mode)
		}
		if !quits(overlay, "ctrl+c") {
			t.Errorf("ctrl+c did not quit from %s mode", name)
		}
	}
}

// overlayWanted documents that each key opens the box it names.
var overlayWanted = map[string]mode{
	"help":     modeHelp,
	"quantity": modeQty,
	"lender":   modeLender,
	"travel":   modeTravel,
	"confirm":  modeConfirm,
}

func TestBoardKeysOpenTheRightBox(t *testing.T) {
	m := testModel(NewChrome(false, false))
	for k, want := range map[string]mode{
		"b":     modeQty,
		"s":     modeQty,
		"l":     modeLender,
		"t":     modeTravel,
		"e":     modeConfirm,
		"enter": modeConfirm,
		"?":     modeHelp,
	} {
		if got := send(m, k).mode; got != want {
			t.Errorf("%q opened mode %v, want %v", k, got, want)
		}
	}
}

func TestLetterKeysInsideQuantityFieldAreNotShortcuts(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b")
	got := send(m, "q")
	if quits(m, "q") {
		t.Fatalf("q would have quit while the quantity box had focus")
	}
	if got.mode != modeQty {
		t.Errorf("q left quantity mode (%v); the box must keep focus", got.mode)
	}
	if got.qtyDraft != "" {
		t.Errorf("q typed %q into the quantity field", got.qtyDraft)
	}
	if facts(got) != facts(m) {
		t.Errorf("q mutated the run: %s -> %s", facts(m), facts(got))
	}
}

func TestDigitsAccumulateAndBackspaceEdits(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b", "1", "2", "3")
	if m.qtyDraft != "123" {
		t.Fatalf("draft = %q, want 123", m.qtyDraft)
	}
	if got := send(m, "backspace"); got.qtyDraft != "12" {
		t.Errorf("after backspace draft = %q, want 12", got.qtyDraft)
	}
	if got := send(testModel(NewChrome(false, false)), "b", "1", "2", "esc", "b"); got.qtyDraft != "" {
		t.Errorf("reopening the box kept the old draft %q", got.qtyDraft)
	}
}

func TestQuickSetQuantities(t *testing.T) {
	m := testModel(NewChrome(false, false))
	mmax := m.run.BuyLimits(m.selectedProduct()).Max
	cases := map[string]string{"a": "1", "b": "5"}
	for k, want := range cases {
		if got := send(m, "b", k).qtyDraft; got != want {
			t.Errorf("quick-set %q set %q, want %q", k, got, want)
		}
	}
	if got := send(m, "b", "c").qtyDraft; got != itoa(mmax) {
		t.Errorf("quick-set c set %q, want the engine limit %d", got, mmax)
	}
}

// TestQuickMaxUsesEngineLimitAndNamesTheConstraint is the Phase 3 replacement for
// the shell's own clamp: the interface asks the engine for the maximum and shows
// which rule binds it, so a stale hand-written limit cannot appear on screen.
func TestQuickMaxUsesEngineLimitAndNamesTheConstraint(t *testing.T) {
	m := testModel(NewChrome(false, false))
	p := m.selectedProduct()
	limits := m.run.BuyLimits(p)
	if limits.Max < 1 {
		t.Fatalf("seed 1 day 1 should allow at least one unit, got %+v", limits)
	}
	if got := send(m, "b", "c").qtyDraft; got != itoa(limits.Max) {
		t.Errorf("max quick-set = %q, want the engine's %d", got, limits.Max)
	}
	if s := statusLineOf(m); !strings.Contains(s, "most buy: "+itoa(limits.Max)) {
		t.Errorf("board status did not name the engine limit: %q", s)
	}
	sell := send(m, "s", "c")
	if want := itoa(m.run.SellLimits(p).Max); sell.qtyDraft != want {
		t.Errorf("max sell quick-set = %q, want the engine's %s", sell.qtyDraft, want)
	}
}

func TestBuyAppliesThroughTheEngine(t *testing.T) {
	m := testModel(NewChrome(false, false))
	p := m.selectedProduct()
	q := m.run.QuoteHere(p)
	before := m.run
	cash, stock := before.State.Cash, before.StockOf(p)

	got := send(m, "b", "1", "enter")
	if got.mode != modeBoard {
		t.Fatalf("a completed purchase left mode %v", got.mode)
	}
	if got.run.State.Cash != cash-q.Ask {
		t.Errorf("cash = %s, want %s less the ask %s", game.FormatMoney(got.run.State.Cash),
			game.FormatMoney(cash), game.FormatMoney(q.Ask))
	}
	if got.run.OwnedOf(p) != 1 {
		t.Errorf("owned = %d, want 1", got.run.OwnedOf(p))
	}
	if got.run.StockOf(p) != stock-1 {
		t.Errorf("shop stock = %d, want %d", got.run.StockOf(p), stock-1)
	}
	if !strings.Contains(got.status, "Bought 1") || !strings.Contains(got.status, game.FormatMoney(q.Ask)) {
		t.Errorf("no receipt for the purchase: %q", got.status)
	}
}

func TestMorningActionsCostNoDay(t *testing.T) {
	m := testModel(NewChrome(false, false))
	// Every Model copy shares one *game.Run, so the "before" numbers have to be
	// read out as scalars first: reading m.run afterwards reports the new state.
	day, debt := m.run.State.Day, m.run.State.Debt
	got := send(m, "b", "1", "enter", "l", "1", "0", "0", "0", "enter")
	if got.run.State.Day != day {
		t.Errorf("buy and borrow moved the day from %d to %d", day, got.run.State.Day)
	}
	if got.run.State.Debt <= debt {
		t.Errorf("borrowing did not raise the debt: %s -> %s",
			game.FormatMoney(debt), game.FormatMoney(got.run.State.Debt))
	}
}

func TestOversizedQuantityIsRefusedWithTheExactGap(t *testing.T) {
	m := testModel(NewChrome(false, false))
	p := m.selectedProduct()
	limits := m.run.BuyLimits(p)
	before := facts(m)

	// The engine accepts a number limited only by stock and cash, which can
	// be more than one digit; terminals deliver digits one key at a time.
	var keys []string
	keys = append(keys, "b")
	keys = append(keys, strings.Split(itoa(limits.Max+1), "")...)
	keys = append(keys, "enter")
	got := send(m, keys...)
	if got.mode != modeQty {
		t.Errorf("a refused purchase closed the box (mode %v)", got.mode)
	}
	if got.qtyDraft != itoa(limits.Max+1) {
		t.Errorf("the refused number vanished: %q", got.qtyDraft)
	}
	if facts(got) != before {
		t.Errorf("a refused purchase changed the run: %s -> %s", before, facts(got))
	}
	want := m.run.PreviewBuy(p, limits.Max+1)
	switch want.Reason {
	case game.ReasonInsufficientCash:
		if !strings.Contains(got.status, "short "+game.FormatMoney(want.ShortBy)) {
			t.Errorf("refusal did not name the missing money (%s): %q", game.FormatMoney(want.ShortBy), got.status)
		}
	case game.ReasonInsufficientStock:
		if !strings.Contains(got.status, itoa(want.StockShort)+" short") {
			t.Errorf("refusal did not name the missing stock (%d): %q", want.StockShort, got.status)
		}
	default:
		t.Errorf("unexpected refusal reason %v", want.Reason)
	}
}

func TestRejectionReasonRendersInsideTheBox(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b", "0")
	um, _ := m.Update(key("enter"))
	after := um.(Model)
	lines := frameLines(t, after)
	qtyAt, reasonAt := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "quantity:") {
			qtyAt = i
		}
		if strings.Contains(l, "whole number") {
			reasonAt = i
		}
	}
	// The quantity box paints over the board's status line, so a reason printed
	// above it would be hidden while the player is still typing.
	if qtyAt < 0 || reasonAt < 0 || reasonAt < qtyAt {
		t.Errorf("rejection reason is not inside the box (quantity line %d, reason line %d):\n%s",
			qtyAt, reasonAt, strings.Join(lines, "\n"))
	}
}

func TestUnaffordablePreviewNamesWhatIsMissing(t *testing.T) {
	m := testModel(NewChrome(false, false))
	p := m.selectedProduct()
	limits := m.run.BuyLimits(p)
	pv := m.run.PreviewBuy(p, limits.Max+1)
	if pv.Reason != game.ReasonInsufficientCash {
		t.Skipf("seed 1: cash is not the binding constraint (%v)", pv.Reason)
	}
	got := frame(send(m, "b", itoa(limits.Max+1)))
	if !strings.Contains(got, "short "+game.FormatMoney(pv.ShortBy)) {
		t.Errorf("preview did not name the missing money (%s):\n%s", game.FormatMoney(pv.ShortBy), got)
	}
	if !strings.Contains(got, "cost "+game.FormatMoney(pv.Cost)) {
		t.Errorf("preview did not say what the attempt costs:\n%s", got)
	}
}

func TestHelpOpensAndClosesWithoutChangingRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := facts(m)
	opened := send(m, "?")
	if opened.mode != modeHelp {
		t.Fatalf("? opened mode %v", opened.mode)
	}
	for _, k := range []string{"esc", "j", "x", "?"} {
		closed := send(opened, k)
		if closed.mode != modeBoard {
			t.Errorf("%q did not close help (mode %v)", k, closed.mode)
		}
		if facts(closed) != before {
			t.Errorf("%q mutated the run while help was open: %s -> %s", k, before, facts(closed))
		}
	}
}

func TestBoxesCancelWithoutTouchingTheRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := facts(m)
	for _, keys := range [][]string{
		{"b", "9", "9", "esc"},
		{"l", "5", "0", "0", "0", "0", "esc"},
		{"l", "b", "1", "0", "0", "esc"},
		{"t", "down", "esc"},
		{"e", "esc"},
		{"?", "esc"},
	} {
		got := send(m, keys...)
		if got.mode != modeBoard {
			t.Errorf("%v left mode %v", keys, got.mode)
		}
		if facts(got) != before {
			t.Errorf("%v mutated the run: %s -> %s", keys, before, facts(got))
		}
	}
}

// TestCommitNeedsConfirmation is the duplicate-day guard: Enter on the board only
// opens the confirmation, and a further Enter closes exactly one day. A third
// Enter opens a fresh confirmation rather than committing the new morning.
func TestCommitNeedsConfirmation(t *testing.T) {
	// Each case starts from a fresh run: Model copies share the *game.Run, so
	// reusing one model across cases would report the other case's day.
	m := testModel(NewChrome(false, false))
	day := m.run.State.Day

	first := send(m, "enter")
	if first.run.State.Day != day {
		t.Fatalf("the first Enter closed a day (%d -> %d)", day, first.run.State.Day)
	}
	if first.mode != modeConfirm {
		t.Fatalf("Enter did not open the confirmation (mode %v)", first.mode)
	}
	second := send(testModel(NewChrome(false, false)), "enter", "enter")
	if second.run.State.Day != day+1 {
		t.Fatalf("two Enters closed %d days, want 1", second.run.State.Day-day)
	}
	if second.mode != modeBoard {
		t.Errorf("after committing, mode = %v, want the board", second.mode)
	}
	third := send(testModel(NewChrome(false, false)), "enter", "enter", "enter")
	if third.run.State.Day != day+1 {
		t.Errorf("three Enters closed %d days, want 1", third.run.State.Day-day)
	}
	if third.mode != modeConfirm {
		t.Errorf("the third Enter left mode %v, want a fresh confirmation", third.mode)
	}
	if got := send(testModel(NewChrome(false, false)), "enter", "esc"); got.run.State.Day != day {
		t.Errorf("esc after the confirmation still closed the day")
	}
}

func TestRawTerminalKeyShapesAreUnderstood(t *testing.T) {
	// A byte-stream terminal reports a printable key as a code with no text, and
	// Enter as a bare carriage return. Both spellings have to work, or typing a
	// quantity silently does nothing on real hardware.
	m := send(testModel(NewChrome(false, false)), "b")
	um, _ := m.Update(tea.KeyPressMsg{Code: '7'})
	after := um.(Model)
	if after.qtyDraft != "7" {
		t.Fatalf("a digit that arrived as a bare code produced draft %q", after.qtyDraft)
	}
	// One unit is affordable on every seed, so the carriage return must be able to
	// submit it without any text on the key.
	typing := send(testModel(NewChrome(false, false)), "b", "1")
	um, cmd := typing.Update(tea.KeyPressMsg{Code: '\r'})
	if cmd != nil {
		t.Fatalf("carriage return quit the program")
	}
	sent := um.(Model)
	if sent.mode != modeBoard {
		t.Fatalf("carriage return did not submit the draft (mode %v)", sent.mode)
	}
	if sent.run.OwnedOf(sent.selectedProduct()) != 1 {
		t.Errorf("carriage return did not buy: status %q", sent.status)
	}
	// A modified digit is a command, not a number.
	mod := send(testModel(NewChrome(false, false)), "b")
	um, _ = mod.Update(tea.KeyPressMsg{Code: '7', Mod: tea.ModAlt, Text: "7"})
	if got := um.(Model); got.qtyDraft != "" {
		t.Errorf("alt+7 typed %q into the quantity field", got.qtyDraft)
	}
}

func TestUnrelatedMessagesDoNotChangeTheRun(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "j")
	before := facts(m)
	for _, msg := range []tea.Msg{tea.MouseWheelMsg{}, tea.QuitMsg{}, struct{}{}} {
		um, cmd := m.Update(msg)
		if cmd != nil {
			t.Errorf("%T returned a command %v", msg, cmd())
		}
		if mm, ok := um.(Model); ok && facts(mm) != before {
			t.Errorf("%T mutated the run: %s -> %s", msg, before, facts(mm))
		}
	}
}

// TestTravelNumberKeysPickTheRoad checks a destination can be chosen without an
// arrow key. Escape sequences do not always survive ssh and ConPTY links intact,
// and losing the only way to pick a road would strand the player at the depot.
func TestTravelNumberKeysPickTheRoad(t *testing.T) {
	m := testModel(NewChrome(false, false))
	box := strings.Join(m.travelBox(m.snapshot()), "\n")
	if !strings.Contains(box, "[1] ") || !strings.Contains(box, "[2] ") {
		t.Fatalf("the road list is not numbered, so a number key means nothing:\n%s", box)
	}

	// A number with no road behind it is refused inside the box, and the run is
	// untouched: the marker has to stay where the player can see it.
	before := facts(m)
	bad := send(m, "t", "9")
	got := strings.Join(bad.travelBox(bad.snapshot()), "\n")
	if !strings.Contains(got, "no road number 9") {
		t.Errorf("road 9 was accepted without a word:\n%s", got)
	}
	if bad.mode != modeTravel || facts(bad) != before {
		t.Errorf("a bad road number changed the run: mode %v, %s -> %s", bad.mode, before, facts(bad))
	}

	from := m.run.State.Market
	want := m.otherMarkets()[1]
	two := send(m, "t", "2")
	if two.travelSel != 1 {
		t.Fatalf("pressing 2 left the marker at %d, want 1", two.travelSel)
	}
	if two.run.State.Market != from {
		t.Fatalf("picking a road moved the player before the day was committed")
	}
	if named := content.MarketOf(two.destMarket()).Name; !strings.Contains(
		strings.Join(two.travelBox(two.snapshot()), "\n"), "[2] "+named) {
		t.Errorf("the marker is not on the [2] road (%s)", named)
	}
	travelled := send(two, "enter")
	if travelled.run.State.Market != want {
		t.Errorf("road 2 went to %v, want %v", travelled.run.State.Market, want)
	}

	// Arrows still work: the same road reached with the marker key instead.
	via := send(testModel(NewChrome(false, false)), "t", "down", "enter")
	if via.run.State.Market != want {
		t.Errorf("[down] went to %v, want %v", via.run.State.Market, want)
	}
}

// TestMorningReceiptNamesTheDayThePlayerWokeIn pins the morning status line to the
// header: the engine stamps a close receipt with the morning that follows, so a
// line built from the stamp must not run one day past the board.
func TestMorningReceiptNamesTheDayThePlayerWokeIn(t *testing.T) {
	for _, keys := range [][]string{{"e", "enter"}, {"t", "enter"}} {
		m := send(testModel(NewChrome(false, false)), keys...)
		want := "now day " + itoa(m.run.State.Day) + "."
		if line := statusLineOf(m); !strings.Contains(line, want) {
			t.Errorf("%v woke on day %d but the status said %q", keys, m.run.State.Day, line)
		}
	}
}
