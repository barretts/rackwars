package game

import (
	"strings"
	"testing"

	"rackwars/internal/content"
)

// savedRun plays a few real moves and hands back the configuration and state a
// Phase 4 save would carry, so the resume tests start from an honest position
// rather than a hand-written one.
func savedRun(t *testing.T) (Config, State) {
	t.Helper()
	r, err := New(DefaultPrototypeConfig(1))
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	actions := []Action{
		{Kind: ActTravel, Market: content.WestTexas},
		{Kind: ActBuy, Product: content.Sled3090, Qty: 2},
		{Kind: ActBorrow, Amount: 3000},
		{Kind: ActStay},
	}
	for i, a := range actions {
		_, res := r.Apply(a)
		if !res.OK {
			t.Fatalf("setup action %d (%v) refused: %s", i, a, res.Reason)
		}
	}
	return r.Config(), r.State
}

func TestResumeRestoresTheExactBoard(t *testing.T) {
	cfg, want := savedRun(t)
	r, err := Resume(cfg, want)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	got := r.State
	if got.Day != want.Day || got.Market != want.Market || got.Cash != want.Cash || got.Debt != want.Debt {
		t.Fatalf("resumed position differs: day %d/%d market %s/%s cash %s/%s debt %s/%s",
			got.Day, want.Day, got.Market, want.Market,
			FormatMoney(got.Cash), FormatMoney(want.Cash),
			FormatMoney(got.Debt), FormatMoney(want.Debt))
	}
	for i := range want.Quotes {
		if got.Quotes[i] != want.Quotes[i] {
			t.Fatalf("quote cell %d rerolled on resume: %+v vs %+v", i, got.Quotes[i], want.Quotes[i])
		}
	}
	for i := range want.Stock {
		if got.Stock[i] != want.Stock[i] || got.Owned[i] != want.Owned[i] {
			t.Fatalf("row %d changed on resume: stock %d/%d owned %d/%d",
				i, got.Stock[i], want.Stock[i], got.Owned[i], want.Owned[i])
		}
	}
	if got.Stats != want.Stats {
		t.Fatalf("statistics changed on resume: %+v vs %+v", got.Stats, want.Stats)
	}

	// A resumed morning behaves like the morning that was saved: the same quote
	// is offered, and the stay charges exactly the interest the preview promised
	// - once, not twice, which is the trap a naive save format falls into.
	p := cfg.Products[0]
	debtBefore, cashBefore := r.State.Debt, r.State.Cash
	charge := r.TomorrowInterest()
	if q := r.QuoteHere(p); q.Bid <= 0 {
		t.Fatalf("resumed run has no firm quote for %s", p)
	}
	_, res := r.Apply(Action{Kind: ActStay})
	if !res.OK {
		t.Fatalf("stay on the resumed morning refused: %s", res.Reason)
	}
	if r.State.Day != want.Day+1 {
		t.Fatalf("resumed run advanced to day %d, expected %d", r.State.Day, want.Day+1)
	}
	if got := r.State.Debt - debtBefore; got != charge {
		t.Fatalf("the resumed morning added %s of interest; the preview promised %s",
			FormatMoney(got), FormatMoney(charge))
	}
	if r.State.Cash != cashBefore {
		t.Fatalf("staying in place moved cash from %s to %s",
			FormatMoney(cashBefore), FormatMoney(r.State.Cash))
	}
}

// TestResumeRejectsIllegalStates is the save-validation gate: a file that does
// not describe a run this engine could have reached is refused, and each refusal
// says which part of the run is impossible.
func TestResumeRejectsIllegalStates(t *testing.T) {
	cfg, base := savedRun(t)

	cases := []struct {
		name   string
		mutate func(*State)
		want   string
	}{
		{"no market", func(s *State) { s.Market = "" }, "names no market"},
		{"foreign market", func(s *State) { s.Market = content.Huaqiangbei }, "does not contain"},
		{"day before the start", func(s *State) { s.Day = 0 }, "outside 1.."},
		{"day after the close", func(s *State) { s.Day = content.DayCount + 1 }, "outside 1.."},
		{"negative cash", func(s *State) { s.Cash = -1 }, "cash"},
		{"negative debt", func(s *State) { s.Debt = -500 }, "debt"},
		{"negative heat", func(s *State) { s.Heat = -1 }, "heat"},
		{"missing inventory row", func(s *State) { s.Owned = s.Owned[:1] }, "owned"},
		{"missing quote cells", func(s *State) { s.Quotes = s.Quotes[:2] }, "quote cells"},
		{"missing sighting row", func(s *State) { s.SeenDay = s.SeenDay[:1] }, "sighting"},
		{"negative owned count", func(s *State) { s.Owned[0] = -3 }, "negative"},
		{"negative stock", func(s *State) { s.Stock[0] = -2 }, "negative"},
		{"negative cost basis", func(s *State) { s.CostBasis[0] = -100 }, "negative"},
		{"cost basis without stock held", func(s *State) { s.Owned[0] = 0; s.CostBasis[0] = 5000 }, "cost basis"},
		{"impossible quote", func(s *State) { s.Quotes[0] = Quote{Ask: 100, Bid: 100} }, "impossible quote"},
		{"quote seen in the future", func(s *State) { s.SeenDay[0] = s.Day + 4 }, "not on or before"},
		{"over but no ending", func(s *State) { s.Over = EndRepossessed }, "ending"},
		{"ending on an unfinished day", func(s *State) { s.Over = EndClosed; s.End.Kind = EndClosed; s.Day = 12 }, "closing day"},
		{"unknown ending kind", func(s *State) { s.Over = EndKind(9) }, "unknown ending"},
		{"negative statistics", func(s *State) { s.Stats.Commitments = -2 }, "statistics"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := copyState(base)
			tc.mutate(&st)
			_, err := Resume(cfg, st)
			if err == nil {
				t.Fatalf("state with %s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal for %s does not mention %q: %v", tc.name, tc.want, err)
			}
		})
	}
}

// The fleet may be larger than any town's racks: slots fence hosting, never
// owning, so such a save is playable — not corrupt.
func TestResumeAcceptsFleetLargerThanAnyTown(t *testing.T) {
	cfg, base := savedRun(t)
	st := copyState(base)
	st.Owned[0] = 400
	st.CostBasis[0] = 1
	r, err := Resume(cfg, st)
	if err != nil {
		t.Fatalf("fleet of %d %s-sized rigs refused as corrupt: %v",
			st.Owned[0], content.ProductOf(cfg.Products[0]).Name, err)
	}
	if want := 400 * content.ProductOf(cfg.Products[0]).SlotsPerUnit; r.SlotsUsed() != want {
		t.Fatalf("fleet footprint = %d, want %d", r.SlotsUsed(), want)
	}
}

// TestResumeDoesNotAliasTheSavedRows guards the save layer: the slices decoded
// from a file stay the caller's, so nothing it still holds can quietly keep
// editing the live game.
func TestResumeDoesNotAliasTheSavedRows(t *testing.T) {
	cfg, st := savedRun(t)
	r, err := Resume(cfg, st)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	r.State.Owned[0] += 40
	if st.Owned[0] == r.State.Owned[0] {
		t.Fatalf("the resumed run shares its inventory with the caller's state")
	}
	r.State.Quotes[0].Ask += 1
	if st.Quotes[0].Ask == r.State.Quotes[0].Ask {
		t.Fatalf("the resumed run shares its quote cells with the caller's state")
	}
}
