package sim

import (
	"testing"

	"rackwars/internal/game"
)

// TestFullGateThreeArchetypesFinish pins the Phase 5 exit gate: the three
// play styles the design names - cautious trade-and-repay, legitimate hosting,
// and risky import - each finish a full-rules run on day 30 with the note
// settled and cash out, on the documented seeds below. These are the seeds the
// sweep found across 1..40 (11 of 40 close, 29 repossess; every win is a seed
// where all three styles clear, so no style wins on luck alone).
//
// The test is the record: a content or engine change that takes one of these
// seeds off the win list must fail here and force a re-judgement, exactly like
// the Phase 2 gate tests. The customs and import fine path is deliberately not
// claimed here - across these winning runs cargo crossed fine-free (fog of war
// keeps an unvisited border town's quotes at zero, so the import trips fired on
// local margin, not on a scored crossing). The fine paths are owned by
// internal/game (TestFineShortfallBecomesDebt, TestCustomsStopBlocksEverythingElse,
// TestSurrenderTakesTheNamedUnitOnly, TestContestCoinDoesNotReroll) and by the
// live stopped-calendar capture recorded in TUI-TASKS.md.
func TestFullGateThreeArchetypesFinish(t *testing.T) {
	seeds := []uint64{5, 16, 18, 25, 27, 33}
	for _, seed := range seeds {
		for _, p := range AllFull() {
			o, _, _, err := Run(Full(seed), p)
			if err != nil {
				t.Fatalf("%s seed %d: %v", p.Name, seed, err)
			}
			if o.Unfinished {
				t.Errorf("%s seed %d never finished: last refusal %s", p.Name, seed, o.Refusal)
				continue
			}
			if o.EndKind != game.EndClosed || !o.Won() {
				t.Errorf("%s seed %d ended %s (cash out %s); the gate needs closed with a positive cash-out",
					p.Name, seed, o.EndKind, game.FormatMoney(o.CashOut))
			}
		}
	}
}
