package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSelectionMovesWithArrowsAndVimKeys(t *testing.T) {
	m := testModel(NewChrome(false, false))
	if got := send(m, "k").sel; got != len(m.board.Rows)-1 {
		t.Errorf("k from the first row selected %d, want the last row", got)
	}
	if got := send(m, "up").sel; got != len(m.board.Rows)-1 {
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
	before := m.board
	got := send(m, "j", "j", "k")
	if !reflect.DeepEqual(got.board, before) {
		t.Errorf("navigation mutated the run:\nbefore %+v\nafter  %+v", before, got.board)
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
	for name, overlay := range map[string]Model{"help": send(m, "?"), "quantity": send(m, "b")} {
		if !quits(overlay, "ctrl+c") {
			t.Errorf("ctrl+c did not quit from %s mode", name)
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
	if !reflect.DeepEqual(got.board, m.board) {
		t.Errorf("q mutated the run: %+v", got.board)
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
	m := testModel(NewChrome(false, false)) // 3090 SLED: ask 1540, stock 12, 2 slots, 8 free, $10,000 cash
	cases := map[string]string{"a": "1", "b": "5", "c": "4"}
	for k, want := range cases {
		if got := send(m, "b", k).qtyDraft; got != want {
			t.Errorf("quick-set %q set %q, want %q", k, got, want)
		}
	}
}

func TestQuantityMaxRespectsStockSlotsAndCash(t *testing.T) {
	m := testModel(NewChrome(false, false))
	// 3090 SLED: 12 in stock, 8 free slots at 2 each = 4, $10,000 at $1,540 = 6.
	if got := m.maxQuantity(m.board.Rows[0]); got != 4 {
		t.Errorf("max buy of the sled = %d, want 4 (slots bind first)", got)
	}
	m.board.SlotsTotal, m.board.SlotsUsed = 40, 0
	m.board.Cash = 3000
	// 3000/1540 = 1 unit, and 1 unit fits the free slots.
	if got := m.maxQuantity(m.board.Rows[0]); got != 1 {
		t.Errorf("max buy of the sled with $3,000 = %d, want 1 (cash binds)", got)
	}
	// Selling is limited to what is owned.
	sell := m
	sell.qtyAction = actionSell
	if got := sell.maxQuantity(m.board.Rows[0]); got != 0 {
		t.Errorf("max sell of the sled = %d, want 0", got)
	}
}

func TestEscCancelsWithoutChangingRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := m.board
	got := send(m, "j", "b", "1", "2", "esc")
	if got.mode != modeBoard {
		t.Fatalf("esc left mode %v, want the board", got.mode)
	}
	if got.qtyDraft != "" {
		t.Errorf("esc kept draft %q", got.qtyDraft)
	}
	if got.sel != 1 {
		t.Errorf("esc lost the selection: %d, want 1", got.sel)
	}
	if !reflect.DeepEqual(got.board, before) {
		t.Errorf("esc mutated the run: %+v", got.board)
	}
}

func TestInvalidQuantityIsExplainedAndKeepsEntry(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "b", "0")
	got, _ := m.Update(key("enter"))
	mm := got.(Model)
	if mm.mode != modeQty {
		t.Fatalf("invalid quantity left the box (mode %v)", mm.mode)
	}
	if !strings.Contains(mm.status, "whole number") {
		t.Errorf("no explanation for a zero quantity: %q", mm.status)
	}
	if mm.qtyDraft != "0" {
		t.Errorf("invalid quantity discarded the entry %q", mm.qtyDraft)
	}
}

func TestOverLimitQuantityIsExplainedAndKeepsEntry(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "down", "down") // H100 costs more than the cash on hand
	if got := m.maxQuantity(m.selectedRow()); got != 0 {
		t.Fatalf("max buy of the H100 = %d, want 0", got)
	}
	got := send(m, "b", "1")
	um, cmd := got.Update(key("enter"))
	if cmd != nil {
		t.Fatalf("an unaffordable purchase returned a command: %v", cmd())
	}
	after := um.(Model)
	if after.mode != modeQty {
		t.Fatalf("unaffordable purchase left the box (mode %v)", after.mode)
	}
	if !strings.Contains(after.status, "will not fit") {
		t.Errorf("no explanation for an unaffordable quantity: %q", after.status)
	}
	if !strings.Contains(frame(after), "1") {
		t.Errorf("the rejected quantity disappeared from the box:\n%s", frame(after))
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
	m := send(testModel(NewChrome(false, false)), "down", "down") // H100
	got := frame(send(m, "b", "1"))
	if !strings.Contains(got, "you need") || !strings.Contains(got, "$25,200") {
		t.Errorf("preview did not name the missing money:\n%s", got)
	}
	if !strings.Contains(got, "cost $35,200") {
		t.Errorf("preview did not say how much is missing:\n%s", got)
	}
}

func TestHelpOpensAndClosesWithoutChangingRun(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := m.board
	opened := send(m, "?")
	if opened.mode != modeHelp {
		t.Fatalf("? opened mode %v", opened.mode)
	}
	for _, k := range []string{"esc", "q", "j", "x", "?"} {
		closed := send(opened, k)
		if closed.mode != modeBoard {
			t.Errorf("%q did not close help (mode %v)", k, closed.mode)
		}
		if !reflect.DeepEqual(closed.board, before) {
			t.Errorf("%q mutated the run while help was open: %+v", k, closed.board)
		}
	}
}

func TestUnwiredActionsExplainInsteadOfFaking(t *testing.T) {
	m := testModel(NewChrome(false, false))
	before := m.board
	for _, k := range []string{"l", "t", "e"} {
		got := send(m, k)
		if got.mode != modeBoard {
			t.Errorf("%q opened mode %v before the engine exists", k, got.mode)
		}
		if !strings.Contains(got.status, "trading engine") {
			t.Errorf("%q said %q instead of naming the missing engine", k, got.status)
		}
		if !strings.Contains(got.status, "nothing changed") {
			t.Errorf("%q did not say the run was untouched: %q", k, got.status)
		}
	}
	if !reflect.DeepEqual(send(m, "l", "t", "e").board, before) {
		t.Errorf("unwired actions mutated the run")
	}
}

func TestCommitKeysCannotAdvanceTheDay(t *testing.T) {
	m := testModel(NewChrome(false, false))
	got := send(m, "enter", "enter", "enter", "e", "enter")
	if got.board.Day != 1 {
		t.Fatalf("the day advanced to %d without the engine", got.board.Day)
	}
	if !strings.Contains(got.status, "nothing changed") {
		t.Errorf("repeated commit keys said %q", got.status)
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
	um, cmd := after.Update(tea.KeyPressMsg{Code: '\r'})
	if cmd != nil {
		t.Fatalf("carriage return quit the program")
	}
	if mm := um.(Model); !strings.Contains(mm.status, "will not fit") {
		t.Errorf("carriage return did not submit the draft: %q", mm.status)
	}
	// A modified digit is a command, not a number.
	typing := send(testModel(NewChrome(false, false)), "b")
	um, _ = typing.Update(tea.KeyPressMsg{Code: '7', Mod: tea.ModAlt, Text: "7"})
	if got := um.(Model); got.qtyDraft != "" {
		t.Errorf("alt+7 typed %q into the quantity field", got.qtyDraft)
	}
}

func TestUnrelatedMessagesDoNotChangeTheRun(t *testing.T) {
	m := send(testModel(NewChrome(false, false)), "j")
	before := m.board
	for _, msg := range []tea.Msg{tea.MouseWheelMsg{}, tea.QuitMsg{}, struct{}{}} {
		um, cmd := m.Update(msg)
		if cmd != nil {
			t.Errorf("%T returned a command %v", msg, cmd())
		}
		if mm, ok := um.(Model); ok && !reflect.DeepEqual(mm.board, before) {
			t.Errorf("%T mutated the run: %+v", msg, mm.board)
		}
	}
}
