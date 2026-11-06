package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/save"
)

// The five-market game in the interface. The prototype suites next door pin
// that nothing here changed the old game; these tests drive the new screens
// with key presses only — no engine call inside an assertion is a second
// source of truth; where a number is checked, the engine preview is quoted
// once and the screen compared against it.

func mustFullModel(t *testing.T, seed uint64) Model {
	t.Helper()
	m, err := NewFullRun(NewChrome(false, true), seed)
	if err != nil {
		t.Fatalf("NewFullRun(%d): %v", seed, err)
	}
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return um.(Model)
}

func mustRunModel(t *testing.T, run *game.Run) Model {
	t.Helper()
	m := New(NewChrome(false, true), run)
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return um.(Model)
}

func TestFullBoardCarriesTheNewLines(t *testing.T) {
	m := mustFullModel(t, 1)
	if !m.full {
		t.Fatal("a full run must put the model in full-game mode")
	}
	out := m.render()
	for _, want := range []string{"five markets", "Meter:", "[a]llocation"} {
		if !strings.Contains(out, want) {
			t.Errorf("board is missing %q:\n%s", want, out)
		}
	}
	// The full cast is six products; all of them are on the board.
	if got := strings.Count(out, "\nH100") + strings.Count(out, "H100 "); got < 1 {
		t.Errorf("board does not list H100s:\n%s", out)
	}
}

func TestPrototypeKeepsItsBoardAndRefusesAllocation(t *testing.T) {
	m := mustModel(t, NewChrome(false, true), 1)
	m = send(m, "a")
	if m.mode != modeBoard || !m.statusBad {
		t.Fatalf("[a] must say no on the prototype, not open a box: %+v", m)
	}
	if !strings.Contains(m.status, "five-market") {
		t.Errorf("refusal should name the game it belongs to: %q", m.status)
	}
}

func TestNightPlanPreviewMatchesEngineAndClosesTheDay(t *testing.T) {
	m := mustFullModel(t, 1)
	m = send(m, "e")
	if m.mode != modeConfirm {
		t.Fatalf("e must open the confirm box, got %v", m.mode)
	}
	m = send(m, "enter")
	if m.mode != modeStay {
		t.Fatalf("enter in the five-market confirm must open the night plan, got %v", m.mode)
	}
	if m.run.State.Day != 1 {
		t.Fatalf("opening the night plan must not close the day (day %d)", m.run.State.Day)
	}
	off := m.run.PreviewStayMode(game.StayPowerOff, nil)
	m = send(m, "2")
	out := m.render()
	if !strings.Contains(out, "THE NIGHT AHEAD") ||
		!strings.Contains(out, "cash after "+game.FormatMoney(off.CashAfter)) {
		t.Errorf("the POWER OFF plan must show the engine's own cash-after "+
			"(%s):\n%s", game.FormatMoney(off.CashAfter), out)
	}
	// Curtail: the grid operator is not buying in the garage on day 1, and
	// the screen must say exactly that instead of picking a doomed plan.
	m = send(m, "3")
	if m.stayMode != game.StayPowerOff || !m.statusBad {
		t.Fatalf("3 must refuse with no offer standing: mode=%v status=%q", m.stayMode, m.status)
	}
	if !strings.Contains(m.status, "not buying") {
		t.Errorf("status should name the reason: %q", m.status)
	}
	m = send(m, "esc", "esc")
	if m.run.State.Day != 1 || m.mode != modeBoard {
		t.Fatalf("canceling the plan must leave the day uncommitted: %+v", m)
	}
	// Now really close the night on the AUTO plan.
	auto := m.run.PreviewStay()
	m = send(m, "e", "enter", "1", "enter")
	if m.run.State.Day != 2 {
		t.Fatalf("the plan must have closed the day, day is %d", m.run.State.Day)
	}
	if len(m.dayLog) == 0 || !strings.HasPrefix(m.dayLog[0], "DAY 1 CLOSE") {
		t.Fatalf("no closing receipt for day 1: %#v", m.dayLog)
	}
	if !strings.Contains(strings.Join(m.dayLog, "\n"), game.FormatMoney(auto.Interest)) {
		t.Errorf("receipt must contain the engine's interest (%s): %#v",
			game.FormatMoney(auto.Interest), m.dayLog)
	}
}

func TestPoweredSubsetPickerRoundTrip(t *testing.T) {
	m := mustFullModel(t, 1)
	m = send(m, "e", "enter", "p")
	if m.mode != modeSubset {
		t.Fatalf("p from the plan must open the powered-card editor, got %v", m.mode)
	}
	out := m.render()
	if !strings.Contains(out, "WHICH RACKS RUN TONIGHT") {
		t.Errorf("the editor screen is not on display:\n%s", out)
	}
	// Nothing owned yet: "+" may not invent racks, so the set stays legal
	// (empty) and enter takes it back to the plan as a hand-picked zero.
	m = send(m, "+", "+", "enter")
	if m.mode != modeStay {
		t.Fatalf("enter must carry the set back to the plan, got %v", m.mode)
	}
	if !strings.Contains(m.render(), "HAND-PICKED") {
		t.Errorf("the plan should show the hand-picked set: %+v", m.stayPowered)
	}
	m = send(m, "1")
	if len(m.stayPowered) != 0 {
		t.Fatalf("1 must fall back to the engine's own plan: %#v", m.stayPowered)
	}
}

func TestAllocationWindowOpensOnlyWhenTheEngineSaysSo(t *testing.T) {
	// Day 10 is Andy's Dalles window. Standing there before it opens must
	// say "no window"; standing in it must show the offer; the claim itself
	// is the engine's decision (and with $10,000 the H100 is out of reach —
	// the box must refuse inside itself, exactly like the quantity box).
	m := mustFullModel(t, 1)
	m = send(m, "a")
	if m.mode != modeAlloc {
		t.Fatalf("a must open Andy's box in the full game, got %v", m.mode)
	}
	if !strings.Contains(m.render(), "No allocation window") {
		t.Errorf("outside a window the box must say so:\n%s", m.render())
	}
	m = send(m, "esc")
	run, err := game.New(dealConfig(1, true))
	if err != nil {
		t.Fatal(err)
	}
	run.State.Day = 10
	run.State.Market = content.TheDalles
	m = mustRunModel(t, run)
	m = send(m, "a", "1", "enter")
	if m.mode != modeAlloc {
		t.Fatalf("a refused claim must keep the player in the box, got %v", m.mode)
	}
	if !m.statusBad {
		t.Fatalf("a refused claim must explain itself in red: %q", m.status)
	}
	out := m.render()
	if !strings.Contains(out, "ANDY'S ALLOCATION") || !strings.Contains(out, "85%") {
		t.Errorf("the box must show the offer as the engine priced it:\n%s", out)
	}
}

// fullStoppedRun sweeps seeds for a full-rules run that actually gets stopped
// at the border, so the encounter tests assert against a real pending state.
func fullStoppedRun(t *testing.T) (*game.Run, []game.Action) {
	t.Helper()
	acts := []game.Action{
		{Kind: game.ActTravel, Market: content.Huaqiangbei},
		{Kind: game.ActBuy, Product: content.Mod40948, Qty: 1},
		{Kind: game.ActTravel, Market: content.TheDalles},
	}
	for seed := uint64(1); seed <= 3000; seed++ {
		r, err := game.New(dealConfig(seed, true))
		if err != nil {
			t.Fatal(err)
		}
		accepted := make([]game.Action, 0, len(acts))
		pending := false
		for _, a := range acts {
			_, res := r.Apply(a)
			if !res.OK {
				break
			}
			accepted = append(accepted, a)
			pending = res.PendingEncounter
		}
		if pending && r.State.PendingEncounter != nil {
			return r, accepted
		}
	}
	t.Fatal("no seed up to 3000 got stopped at the border")
	return nil, nil
}

func TestStoppedCalendarOwnsEveryKey(t *testing.T) {
	run, accepted := fullStoppedRun(t)
	m := mustRunModel(t, run)
	// The engine run was driven outside the interface; the model's history
	// is the same accepted sequence the interface would have recorded.
	m.actions = append([]game.Action(nil), accepted...)
	// The model must show the stop with the engine's own disclosures even
	// when the state arrived mid-run (this is also the resume picture).
	out := m.render()
	if !strings.Contains(out, "CUSTOMS") {
		t.Errorf("the stop must be on screen:\n%s", out)
	}
	before := m.run.State
	m = send(m, "b", "3", "t", "e", "enter", "esc")
	after := m.run.State
	if after.Day != before.Day || after.Cash != before.Cash ||
		after.PendingEncounter == nil {
		t.Fatalf("no key may move a stopped run: %+v -> %+v", before, after)
	}
	m = send(m, "p")
	if m.run.State.PendingEncounter != nil {
		t.Fatal("paying must clear the stop")
	}
	if m.run.State.Day != before.Day+1 {
		t.Fatalf("the stopped day must finally close: %d -> %d", before.Day, m.run.State.Day)
	}
	if len(m.actions) != len(accepted)+1 || m.actions[len(m.actions)-1].Kind != game.ActEncounter {
		t.Fatalf("the answer must be recorded once: %#v", m.actions)
	}
	receipt := strings.Join(m.dayLog, "\n")
	if !strings.Contains(receipt, "DAY "+itoa(before.Day)+" CLOSE") ||
		!strings.Contains(receipt, "fine") {
		t.Errorf("the completed day must read like one receipt, fines inside: %#v", m.dayLog)
	}
	// And the board answers to keys again.
	m = send(m, "e")
	if m.mode != modeConfirm {
		t.Fatalf("the board must be playable after the answer, got %v", m.mode)
	}
}

func TestStoppedRunSavesTaggedAndResumesOnTheAnswer(t *testing.T) {
	store := saveStore(t)
	run, accepted := fullStoppedRun(t)
	kind := string(run.State.PendingEncounter.Kind)
	m := mustRunModel(t, run)
	m.actions = append([]game.Action(nil), accepted...)
	m = m.WithStore(SaveTarget{Store: store, RunID: "seed-stopped"})
	m = m.writeSave()
	if m.saveErr != "" {
		t.Fatalf("a stopped run must still be savable: %s", m.saveErr)
	}
	f, err := store.Load("seed-stopped")
	if err != nil {
		t.Fatalf("the tagged save must load: %v", err)
	}
	if f.Pending != save.PendingEncounterPrefix+kind {
		t.Fatalf("the cover must name the stop: %q", f.Pending)
	}
	// The first window puts the run down before the second picks it up —
	// the same [S]ave-and-quit a player does; two live windows on one run
	// stay refused, and that is the writer lock doing its job.
	m.releaseSave()
	// A second window opens the continue list, takes the tagged run, and
	// lands straight on the answer — then plays out like the first one did.
	second, err := save.NewStore(store.Dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	cm := Model{chrome: NewChrome(false, true)}
	cw, _ := cm.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	cm = cw.(Model)
	cm, offered := cm.WithStore(SaveTarget{Store: second, RunID: "whatever"}).OfferContinue()
	if !offered {
		t.Fatal("the stopped run must be offered on the continue list")
	}
	cm = send(cm, "enter")
	if !cm.full || cm.run.State.PendingEncounter == nil {
		t.Fatalf("the resumed run must be full-rules and still stopped: %+v", cm)
	}
	if !strings.Contains(cm.render(), "CUSTOMS") &&
		!strings.Contains(cm.render(), "SERIAL") && !strings.Contains(cm.render(), "NOISE") {
		t.Errorf("the resumed run must show the answer box:\n%s", cm.render())
	}
	twin, err := f.Run()
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, "p")
	cm = send(cm, "p")
	trec, tres := twin.ResolveEncounter(game.EncPay)
	if !tres.OK {
		t.Fatalf("the twin must be payable too: %v", tres.Reason)
	}
	if cm.run.State.Day != m.run.State.Day || cm.run.State.Cash != m.run.State.Cash ||
		cm.run.State.Debt != m.run.State.Debt {
		t.Fatalf("resume then answer must match the first window exactly:\n%+v\n%+v",
			cm.run.State, m.run.State)
	}
	if len(cm.actions) != len(accepted)+1 ||
		cm.actions[len(cm.actions)-1].Kind != game.ActEncounter {
		t.Fatalf("the answer from the resumed window must be recorded: %#v", cm.actions)
	}
	if cm.run.State.Cash != trec.CashAfter {
		t.Fatalf("the receipt and the screen disagree: %s vs %s",
			game.FormatMoney(cm.run.State.Cash), game.FormatMoney(trec.CashAfter))
	}
}

func TestHelpCoversTheNewKeys(t *testing.T) {
	m := mustFullModel(t, 1)
	m = send(m, "?")
	out := m.render()
	for _, want := range []string{"[p]ay", "[c]ontest", "NIGHT PLAN", "allocation window"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
}
