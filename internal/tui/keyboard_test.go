package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/sim"
)

// mustModel returns an unsized model over a real run on the given seed; the
// program supplies the size through WindowSizeMsg, as a real terminal does.
func mustModel(t *testing.T, chrome Chrome, seed uint64) Model {
	t.Helper()
	m, err := NewRun(chrome, seed)
	if err != nil {
		t.Fatalf("NewRun(%d): %v", seed, err)
	}
	return m
}

// headOf keeps the top n rows of an emulator dump: rows below a shorter frame are
// stale leftovers from whatever taller frame was painted there before.
func headOf(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// sizedModel is a model over a real run that already knows its terminal size,
// which is all View needs to paint anything but the "too small" notice.
func sizedModel(t *testing.T, chrome Chrome, seed uint64, width, height int) Model {
	t.Helper()
	m := mustModel(t, chrome, seed)
	um, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return um.(Model)
}

// prototypeRoads lists the towns reachable from the town the action happens in,
// in content order -- the same order the travel box lists them.
func prototypeRoads(from content.MarketID) []content.MarketID {
	out := make([]content.MarketID, 0, 3)
	for _, id := range content.PrototypeMarkets {
		if id != from {
			out = append(out, id)
		}
	}
	return out
}

// productRow is the board row a product sits on: the TUI lists products in content
// order, so the script moves the selection there before opening buy/sell.
func productRow(p content.ProductID) int {
	for i, id := range content.PrototypeProducts {
		if id == p {
			return i
		}
	}
	return 0
}

// keysFor translates one engine action into the keystrokes that produce it, given
// the town the action happens in. This is how the scripted run below is built:
// the headless policy and the keyboard drive the same engine through the same
// actions, so a difference between the two is a bug in the interface.
func keysFor(from content.MarketID, a game.Action) string {
	selectRow := strings.Repeat("\x1b[B", productRow(a.Product))
	switch a.Kind {
	case game.ActBuy:
		return selectRow + "b" + fmt.Sprint(a.Qty) + "\r"
	case game.ActSell:
		return selectRow + "s" + fmt.Sprint(a.Qty) + "\r"
	case game.ActBorrow:
		return "l" + fmt.Sprint(int64(a.Amount)) + "\r"
	case game.ActRepay:
		return "l" + "r" + fmt.Sprint(int64(a.Amount)) + "\r"
	case game.ActTravel:
		idx := 0
		for i, id := range prototypeRoads(from) {
			if id == a.Market {
				idx = i
			}
		}
		return "t" + itoa(idx+1) + "\r"
	case game.ActStay:
		return "e" + "\r"
	}
	return ""
}

// TestScriptedKeyboardRunWinsOnSeedOne is the Phase 3 exit gate: a run driven only
// by keyboard bytes through the real renderer reaches the day-30 closing auction
// with cash left, on a seed with no borrowing kept to the end.
func TestScriptedKeyboardRunWinsOnSeedOne(t *testing.T) {
	// The same actions, driven headlessly first, prove the win is real and give
	// the closing number the screen has to match.
	run, steps, err := sim.Play(sim.Prototype(1), sim.TradeRepay)
	if err != nil {
		t.Fatalf("headless reference run: %v", err)
	}
	if run.State.End.Kind != game.EndClosed || run.State.End.CashOut <= 0 {
		t.Fatalf("the reference seed no longer produces a trading victory: %+v", run.State.End)
	}

	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pipeRead.Close()
	defer pipeWrite.Close()

	const cols, rows = 120, 30
	scr := newScreen(cols, rows)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := tea.NewProgram(mustModel(t, NewChrome(false, true), 1),
		tea.WithContext(ctx),
		tea.WithInput(pipeRead),
		tea.WithOutput(scr),
		tea.WithWindowSize(cols, rows),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	runErr := make(chan error, 1)
	go func() { _, err := p.Run(); runErr <- err }()

	mustWait := func(what string, pred func(string) bool) string {
		t.Helper()
		if !scr.waitUntil(20*time.Second, pred) {
			t.Fatalf("%s\nscreen was:\n%s", what, scr.text())
		}
		return scr.text()
	}
	mustWait("board never painted", func(s string) bool { return strings.Contains(s, "PRODUCT") })

	// Step.Market is the town the step ended in, so the road the travel box has to
	// pick is found from the town the step started in: track it as the script walks.
	var script strings.Builder
	cur := content.Garage
	for _, st := range steps {
		if !st.OK {
			continue
		}
		script.WriteString(keysFor(cur, st.Action))
		cur = st.Market
	}
	if _, err := pipeWrite.Write([]byte(script.String())); err != nil {
		t.Fatalf("send script: %v", err)
	}

	final := mustWait("the keyboard run never reached an ending",
		func(s string) bool { return strings.Contains(s, "CASH OUT") || strings.Contains(s, "DEFICIENCY") })

	want := "CASH OUT: " + game.FormatMoney(run.State.End.CashOut)
	if !strings.Contains(final, want) {
		t.Errorf("closing number disagrees with the same actions run headlessly: want %q\nscreen:\n%s", want, final)
	}
	if !strings.Contains(final, "RANK: "+run.State.End.Rank) {
		t.Errorf("ending screen is missing the rank %q:\n%s", run.State.End.Rank, final)
	}
	for _, want := range []string{"interest paid", "borrowed", "repaid", "forced sales"} {
		if !strings.Contains(final, want) {
			t.Errorf("ending receipts missing %q:\n%s", want, final)
		}
	}

	// Replay This Seed re-deals the identical market; New Run deals the next one.
	if _, err := pipeWrite.Write([]byte("r")); err != nil {
		t.Fatalf("send r: %v", err)
	}
	replay := mustWait("replay did not restart the run", func(s string) bool {
		return strings.Contains(s, "Day 1/30") && strings.Contains(s, "Replaying seed 1")
	})
	// The emulator keeps rows the new, shorter frame never overwrote, so only the
	// painted head of the screen counts as evidence that the ending is gone.
	if head := headOf(replay, 16); strings.Contains(head, "CASH OUT") {
		t.Errorf("replay left the ending screen on screen:\n%s", head)
	}
	if _, err := pipeWrite.Write([]byte("\x03")); err != nil {
		t.Fatalf("send ctrl+c: %v", err)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("ctrl+c ended the run with an error: %v", err)
	}
}

// TestNewRunDealsANewSeed checks the other half of the replay pair.
func TestNewRunDealsANewSeed(t *testing.T) {
	m := sizedModel(t, NewChrome(false, false), 1, 100, 30)
	// Finish the run the short way: holding onto nothing gets repossessed well
	// before day 30, which is the fastest legitimate ending to reach in a test.
	for i := 0; i < 40 && !m.over(); i++ {
		m = send(m, "e", "enter")
	}
	if !m.over() {
		t.Fatalf("holding did not end the run in 40 commits")
	}
	if m.mode != modeEnd {
		t.Fatalf("a finished run shows mode %v, want the ending screen", m.mode)
	}
	if got := frame(m); !strings.Contains(got, "DEFICIENCY") && !strings.Contains(got, "CASH OUT") {
		t.Errorf("ending screen shows no closing number:\n%s", got)
	}
	next := send(m, "n")
	if next.seed != 2 {
		t.Errorf("New Run dealt seed %d, want 2", next.seed)
	}
	if next.run.State.Day != 1 || next.over() {
		t.Errorf("New Run did not restart cleanly: day %d over %v", next.run.State.Day, next.over())
	}
	if !strings.Contains(next.status, "seed 2") {
		t.Errorf("New Run said %q", next.status)
	}
	again := send(m, "r")
	if again.seed != 1 || again.run.State.Day != 1 {
		t.Errorf("Replay This Seed dealt seed %d day %d, want seed 1 day 1", again.seed, again.run.State.Day)
	}
	// A finished run answers only its own keys: no trading, no day commits.
	if got := send(m, "b", "1", "enter", "e", "enter"); got.run.State.Day != m.run.State.Day {
		t.Errorf("a finished run accepted a new action: day %d -> %d", m.run.State.Day, got.run.State.Day)
	}
}

// TestDayCloseReceiptAppearsInPanel proves the player can see what a committed day
// did without rereading the whole board: the wire, the interest, and the money.
func TestDayCloseReceiptAppearsInPanel(t *testing.T) {
	m := sizedModel(t, NewChrome(false, false), 1, 120, 30)
	before := m.run.State.Day
	m = send(m, "e", "enter")
	panel := strings.Join(m.sideLines(), "\n")
	if !strings.Contains(panel, "DAY 1 CLOSE") {
		t.Fatalf("no close receipt in the panel:\n%s", panel)
	}
	charged := m.run.State.Stats.InterestPaid
	if charged <= 0 {
		t.Fatalf("the engine charged no interest on the committed day")
	}
	if !strings.Contains(panel, game.FormatMoney(charged)) {
		t.Errorf("close receipt omits the interest charged (%s):\n%s", game.FormatMoney(charged), panel)
	}
	if !strings.Contains(panel, game.FormatMoney(m.run.State.Cash)) ||
		!strings.Contains(panel, game.FormatMoney(m.run.State.Debt)) {
		t.Errorf("close receipt omits the money after the day:\n%s", panel)
	}
	if m.run.State.Day != before+1 {
		t.Errorf("day %d -> %d, want one day", before, m.run.State.Day)
	}
}

// TestLenderPanelUsesTheDesignFormat keeps the credit readout in the design's own
// words and numbers.
func TestLenderPanelUsesTheDesignFormat(t *testing.T) {
	m := mustModel(t, NewChrome(false, false), 1)
	box := strings.Join(m.lenderBox(m.snapshot()), "\n")
	if !strings.Contains(box, "LENDER: Ceiling "+game.FormatMoney(m.run.CreditCeiling())) {
		t.Errorf("lender line is not the design format:\n%s", box)
	}
	if !strings.Contains(box, "Tomorrow's interest: "+game.FormatMoney(m.run.TomorrowInterest())) {
		t.Errorf("lender line omits tomorrow's interest:\n%s", box)
	}
	if !strings.Contains(box, "if bids fall 20%") {
		t.Errorf("no stress preview in the lender box:\n%s", box)
	}
	// Borrowing is the engine's decision, shown before it is applied.
	pv := m.run.PreviewBorrow(1_000)
	typed := send(m, "l", "1", "0", "0", "0")
	lines := strings.Join(typed.lenderBox(typed.snapshot()), "\n")
	if !strings.Contains(lines, "Borrow $1,000") || !strings.Contains(lines, game.FormatMoney(pv.DebtAfter)) {
		t.Errorf("borrow preview does not show the resulting debt (%s):\n%s",
			game.FormatMoney(pv.DebtAfter), lines)
	}
}

// TestTravelBoxShowsKnownQuotes checks the rule that a player is never shown a
// number that is not tagged with how old it is.
func TestTravelBoxShowsKnownQuotes(t *testing.T) {
	m := mustModel(t, NewChrome(false, false), 1)
	box := strings.Join(m.travelBox(m.snapshot()), "\n")
	if !strings.Contains(box, "never been there") {
		t.Errorf("an unvisited town should say so, not show prices:\n%s", box)
	}
	// Once a town has been seen, its quotes appear with the day they were seen.
	from := m.run.State.Market
	seen := send(m, "t", "enter") // travel closes the day and records the new town
	if seen.run.State.Market == from {
		t.Fatalf("travel did not move the player")
	}
	back := strings.Join(seen.travelBox(seen.snapshot()), "\n")
	if !strings.Contains(back, "last seen day ") {
		t.Errorf("a town the player has left shows no dated quotes:\n%s", back)
	}
	if !strings.Contains(back, "not an arrival price") {
		t.Errorf("dated quotes were not labelled as stale:\n%s", back)
	}
}

// TestTravelAndStayChargeWhatThePreviewPromised is the interface half of the
// preview/apply agreement: the number on screen is the number the engine charged.
func TestTravelAndStayChargeWhatThePreviewPromised(t *testing.T) {
	m := mustModel(t, NewChrome(false, false), 1)
	dest := m.otherMarkets()[0]
	pv := m.run.PreviewTravel(dest)
	if !pv.Valid {
		t.Fatalf("the first road should be open, got %v", pv.Reason)
	}
	got := send(m, "t", "enter")
	st := got.run.State
	wantFreight := pv.Freight
	if got.run.State.Stats.FreightPaid != wantFreight {
		t.Errorf("freight charged %s, preview promised %s",
			game.FormatMoney(got.run.State.Stats.FreightPaid), game.FormatMoney(wantFreight))
	}
	if st.Debt != pv.DebtAfter {
		t.Errorf("debt after travel %s, preview promised %s",
			game.FormatMoney(st.Debt), game.FormatMoney(pv.DebtAfter))
	}

	stay := m.run.PreviewStay()
	gotStay := send(m, "e", "enter")
	if gotStay.run.State.Debt != stay.DebtAfter {
		t.Errorf("debt after a stay %s, preview promised %s",
			game.FormatMoney(gotStay.run.State.Debt), game.FormatMoney(stay.DebtAfter))
	}
	if gotStay.run.State.Cash != stay.CashAfter {
		t.Errorf("cash after a stay %s, preview promised %s",
			game.FormatMoney(gotStay.run.State.Cash), game.FormatMoney(stay.CashAfter))
	}
}

// TestScriptedKeyboardRunGetsRepossessedOnSeedOne drives the design's failure mode
// through the same keyboard path as the win, so the loss screen is the ending the
// engine computed rather than a restatement of it inside the TUI.
func TestScriptedKeyboardRunGetsRepossessedOnSeedOne(t *testing.T) {
	run, steps, err := sim.Play(sim.Prototype(1), sim.Leveraged)
	if err != nil {
		t.Fatalf("headless reference loss: %v", err)
	}
	if run.State.End.Kind != game.EndRepossessed {
		t.Fatalf("the reference seed no longer produces the warned crash: %+v", run.State.End)
	}

	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer pipeRead.Close()
	defer pipeWrite.Close()

	const cols, rows = 120, 30
	scr := newScreen(cols, rows)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := tea.NewProgram(mustModel(t, NewChrome(false, true), 1),
		tea.WithContext(ctx),
		tea.WithInput(pipeRead),
		tea.WithOutput(scr),
		tea.WithWindowSize(cols, rows),
		tea.WithColorProfile(colorprofile.ASCII),
	)
	runErr := make(chan error, 1)
	go func() { _, err := p.Run(); runErr <- err }()
	if !scr.waitUntil(20*time.Second, func(s string) bool { return strings.Contains(s, "PRODUCT") }) {
		t.Fatalf("board never painted:\n%s", scr.text())
	}

	var script strings.Builder
	cur := content.Garage
	for _, st := range steps {
		if !st.OK {
			continue
		}
		script.WriteString(keysFor(cur, st.Action))
		cur = st.Market
	}
	if _, err := pipeWrite.Write([]byte(script.String())); err != nil {
		t.Fatalf("send script: %v", err)
	}

	// "DEFICIENCY:" is written only by the ending screen. The word REPOSSESSED
	// also appears inside a repossession day's receipt, so it cannot mark the end.
	if !scr.waitUntil(20*time.Second, func(s string) bool { return strings.Contains(s, "DEFICIENCY:") }) {
		t.Fatalf("the keyboard loss never reached the ending screen:\n%s", scr.text())
	}
	final := scr.text()
	want := "DEFICIENCY: the lender is still owed " + game.FormatMoney(run.State.End.Deficiency)
	if !strings.Contains(final, want) {
		t.Errorf("the loss on screen disagrees with the same actions run headlessly: want %q\nscreen:\n%s", want, final)
	}
	if !strings.Contains(final, "RANK: "+run.State.End.Rank) {
		t.Errorf("ending screen is missing the rank %q:\n%s", run.State.End.Rank, final)
	}
	if forced := itoa(run.State.Stats.ForcedUnits) + " units"; !strings.Contains(final, forced) {
		t.Errorf("the ending receipts do not report the %s the engine forced:\n%s", forced, final)
	}

	// New Run deals a different seed, which is how a player walks away from a loss
	// without leaving the terminal.
	if _, err := pipeWrite.Write([]byte("n")); err != nil {
		t.Fatalf("send n: %v", err)
	}
	if !scr.waitUntil(20*time.Second, func(s string) bool {
		return strings.Contains(s, "Day 1/30") && strings.Contains(s, "New run, seed 2.")
	}) {
		t.Fatalf("new run did not deal a fresh seed:\n%s", scr.text())
	}
	if _, err := pipeWrite.Write([]byte("\x03")); err != nil {
		t.Fatalf("send ctrl+c: %v", err)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("ctrl+c ended the run with an error: %v", err)
	}
}
