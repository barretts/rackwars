package game

import (
	"encoding/json"
	"testing"

	"rackwars/internal/content"
)

// scriptedActions is a small but complete run: buy in the garage, sell in
// another town, take a loan, and pay it down before the note compounds.
func scriptedActions() []Action {
	return []Action{
		{Kind: ActBuy, Product: content.Card4090, Qty: 2},
		{Kind: ActTravel, Market: content.Ashburn},
		{Kind: ActSell, Product: content.Card4090, Qty: 2},
		{Kind: ActBorrow, Amount: 8000},
		{Kind: ActBuy, Product: content.Sled3090, Qty: 1},
		{Kind: ActTravel, Market: content.WestTexas},
		{Kind: ActRepay, Amount: 5000},
		{Kind: ActStay},
		{Kind: ActSell, Product: content.Sled3090, Qty: 1},
		{Kind: ActTravel, Market: content.Ashburn},
		{Kind: ActStay},
	}
}

// Mechanical check: the same seed and content set produce the same quotes,
// resolution order, and ending on any machine; the TUI contributes no randomness.
func TestSameSeedSameActionsProduceIdenticalRuns(t *testing.T) {
	var first string
	var firstOut Money
	for range 3 {
		r := newTestRun(t, 1234)
		steps := r.Play(scriptedActions())
		text := TextTrace(r, steps)
		if first == "" {
			first, firstOut = text, r.State.End.CashOut
			if r.State.Over != EndOpen {
				t.Fatalf("script ended the run early: %+v", r.State.End)
			}
			continue
		}
		if text != first {
			t.Fatalf("replay differs:\n---\n%s\n===\n%s", first, text)
		}
		if r.State.End.CashOut != firstOut {
			t.Fatalf("cash-out changed: %s vs %s", FormatMoney(firstOut), FormatMoney(r.State.End.CashOut))
		}
	}
}

// Different seeds must actually change the board, or "seed" is decoration.
func TestDifferentSeedsGiveDifferentBoards(t *testing.T) {
	seen := map[string]bool{}
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		r := newTestRun(t, seed)
		key := ""
		for _, row := range r.QuoteBoard() {
			key += string(row.Product) + ":" + itoa(int(row.Ask)) + ","
		}
		seen[key] = true
	}
	if len(seen) < 6 {
		t.Fatalf("8 seeds produced only %d distinct day-1 boards", len(seen))
	}
}

// Mechanical check: previews change nothing, and the applied action agrees with
// the preview number for number.
func TestPreviewsArePureAndExact(t *testing.T) {
	r := newTestRun(t, 77)
	mustApply(t, r, Action{Kind: ActBuy, Product: content.Card4090, Qty: 1})

	// pure checks that a preview call leaves the whole engine state alone.
	pure := func(what string, preview func()) {
		t.Helper()
		before := jsonSnapshot(t, r.State)
		preview()
		if got := jsonSnapshot(t, r.State); got != before {
			t.Fatalf("%s preview mutated the run:\nbefore %s\nafter  %s", what, before, got)
		}
	}

	pBuy := PreviewBuy{}
	pure("buy", func() { pBuy = r.PreviewBuy(content.Sled3090, 2) })
	if !pBuy.Valid {
		t.Fatalf("sled buy invalid: %+v", pBuy)
	}
	rec := mustApply(t, r, Action{Kind: ActBuy, Product: content.Sled3090, Qty: 2})
	if rec.Cost != pBuy.Cost || rec.CashAfter != pBuy.CashAfter || r.State.Cash != pBuy.CashAfter {
		t.Fatalf("buy applied %+v != preview %+v", rec, pBuy)
	}

	pSell := PreviewSell{}
	pure("sell", func() { pSell = r.PreviewSell(content.Card4090, 1) })
	rec = mustApply(t, r, Action{Kind: ActSell, Product: content.Card4090, Qty: 1})
	if rec.Gross != pSell.Gross || rec.ToLender != pSell.ToLender || rec.CashIn != pSell.CashIn ||
		r.State.Cash != pSell.CashAfter || r.State.Debt != pSell.DebtAfter {
		t.Fatalf("sell applied %+v != preview %+v", rec, pSell)
	}

	pBorrow := PreviewLoan{}
	pure("borrow", func() { pBorrow = r.PreviewBorrow(5000) })
	rec = mustApply(t, r, Action{Kind: ActBorrow, Amount: 5000})
	if rec.CashAfter != pBorrow.CashAfter || rec.DebtAfter != pBorrow.DebtAfter {
		t.Fatalf("borrow applied %+v != preview %+v", rec, pBorrow)
	}

	pRepay := PreviewLoan{}
	pure("repay", func() { pRepay = r.PreviewRepay(1500) })
	rec = mustApply(t, r, Action{Kind: ActRepay, Amount: 1500})
	if rec.CashAfter != pRepay.CashAfter || rec.DebtAfter != pRepay.DebtAfter {
		t.Fatalf("repay applied %+v != preview %+v", rec, pRepay)
	}

	pTravel := TravelPreview{}
	pure("travel", func() { pTravel = r.PreviewTravel(content.Ashburn) })
	if !pTravel.Valid {
		t.Fatalf("travel preview invalid: %+v", pTravel)
	}
	rec = mustApply(t, r, Action{Kind: ActTravel, Market: content.Ashburn})
	if rec.Freight != pTravel.Freight || rec.Interest != pTravel.Interest {
		t.Fatalf("travel receipt %+v != preview %+v", rec, pTravel)
	}
	if r.State.Day != 2 {
		t.Fatalf("day = %d after one travel, want 2", r.State.Day)
	}

	pStay := StayPreview{}
	pure("stay", func() { pStay = r.PreviewStay() })
	if !pStay.Valid {
		t.Fatalf("stay preview invalid: %+v", pStay)
	}
}

// A refusal stops a scripted run instead of silently skipping, so a balance
// report describes the run that actually happened.
func TestPlayStopsAtFirstRefusal(t *testing.T) {
	r := newTestRun(t, 5)
	acts := []Action{
		{Kind: ActBuy, Product: content.Card4090, Qty: 1},
		{Kind: ActSell, Product: content.CardH100, Qty: 1}, // never held
		{Kind: ActStay},
	}
	steps := r.Play(acts)
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2 (stop at the refusal)", len(steps))
	}
	if steps[1].OK || steps[1].Reason != ReasonInsufficientOwned {
		t.Fatalf("second step = %+v, want refusal insufficient owned", steps[1])
	}
	if r.State.Day != 1 {
		t.Fatalf("day advanced past the refusal: %d", r.State.Day)
	}
}

// The trace format is for bug reports: encode, decode, replay, same result.
func TestTraceRoundTripReproducesRun(t *testing.T) {
	r := newTestRun(t, 909)
	// The final action is well-formed but impossible, so the run records a
	// refusal instead of being silently rewritten.
	actions := append(scriptedActions(), Action{Kind: ActRepay, Amount: 999999})
	steps := r.Play(actions)
	if r.State.Over != EndOpen {
		t.Fatalf("run ended early: %+v", r.State.End)
	}
	tr := Trace{Config: r.Config(), Actions: actions}
	b, err := EncodeTrace(tr)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTrace(b)
	if err != nil {
		t.Fatal(err)
	}
	r2, steps2, err := got.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if TextTrace(r2, steps2) != TextTrace(r, steps) {
		t.Fatalf("replayed trace differs:\n%s\n---\n%s", TextTrace(r, steps), TextTrace(r2, steps2))
	}
	if r2.State.Cash != r.State.Cash || r2.State.Debt != r.State.Debt || r2.State.Day != r.State.Day {
		t.Fatalf("replay state differs: %s/%s/day %d vs %s/%s/day %d",
			FormatMoney(r2.State.Cash), FormatMoney(r2.State.Debt), r2.State.Day,
			FormatMoney(r.State.Cash), FormatMoney(r.State.Debt), r.State.Day)
	}
	sum := r.Summarize(steps)
	if sum.Seed != 909 || sum.Refusals != 1 || sum.FailureDay == 0 {
		t.Fatalf("summary = %+v, want the travel refusal recorded", sum)
	}
	if sum.Commits != 5 {
		t.Fatalf("commits = %d, want 5 time commitments, steps: %s", sum.Commits, TextTrace(r, steps))
	}
}

func TestDecodeTraceRejectsBadInput(t *testing.T) {
	if _, err := DecodeTrace([]byte(`{"actions":[]}`)); err == nil {
		t.Fatal("trace without content set decoded")
	}
	if _, err := DecodeTrace([]byte(`{"config":{"seed":1,"products":["4090"],"markets":["garage"]},"actions":[{"kind":9}]}`)); err == nil {
		t.Fatal("unknown action kind decoded")
	}
	if _, err := DecodeTrace([]byte(`{"config":{"seed":1,"products":["4090"],"markets":["garage"]},"actions":[{"kind":0,"product":"h100","qty":1}]}`)); err == nil {
		t.Fatal("product outside the cast decoded")
	}
	if _, err := DecodeTrace([]byte(`not json`)); err == nil {
		t.Fatal("garbage decoded")
	}
}

// LenderStress is what the lender panel shows on rumor morning: if collateral
// bids fall 50% tomorrow, what does the note look like?
func TestLenderStressProjection(t *testing.T) {
	r := newTestRun(t, 12)
	pi := r.Index(content.Card4090)
	r.State.Owned[pi] = 4
	r.State.CostBasis[pi] = 7200
	r.State.Debt = 34000
	st := r.LenderStress(50)
	collateral := r.CollateralValue()
	wantCollateral := collateral / 2
	if st.Collateral != wantCollateral {
		t.Fatalf("stressed collateral = %s, want half of %s", FormatMoney(st.Collateral), FormatMoney(collateral))
	}
	wantCeiling := Money(30000) + wantCollateral/2
	if st.Ceiling != wantCeiling {
		t.Fatalf("stressed ceiling = %s, want %s", FormatMoney(st.Ceiling), FormatMoney(wantCeiling))
	}
	if st.Interest != ceil10(r.State.Debt) {
		t.Fatalf("forecast interest = %s, want %s", FormatMoney(st.Interest), FormatMoney(ceil10(r.State.Debt)))
	}
	if st.ProjectedDebt != r.State.Debt+st.Interest {
		t.Fatalf("projected debt = %s, want %s", FormatMoney(st.ProjectedDebt), FormatMoney(r.State.Debt+st.Interest))
	}
	if st.ProjectedBreach != st.ProjectedDebt-st.Ceiling {
		t.Fatalf("projected breach = %s, want %s", FormatMoney(st.ProjectedBreach), FormatMoney(st.ProjectedDebt-st.Ceiling))
	}
	if st.ProjectedBreach == 0 {
		t.Fatal("a half-price day against $34,000 owed should project a breach")
	}
}

// jsonSnapshot renders the whole engine state for preview-purity checks; the
// state is JSON-tagged for saves anyway, so this is the same view a save sees.
func jsonSnapshot(t *testing.T, s State) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
