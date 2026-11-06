package sim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

// traceDir is where recorded gate evidence lives: the Phase 2 gate is judged on
// traces that exist in the repository, not on ones a test produced and threw
// away.
const traceDir = "../../testdata/traces"

func TestPoliciesFinishEveryRun(t *testing.T) {
	seeds := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	rows := Scan(Prototype(0), seeds, All()...)
	for _, o := range rows {
		t.Log(o.String())
		if o.Unfinished {
			t.Errorf("%s seed %d never reached day 30 or a repossession (last refusal: %s)", o.Policy, o.Seed, o.Refusal)
		}
		if o.Days != content.DayCount && o.EndKind == game.EndClosed {
			t.Errorf("%s seed %d closed on day %d", o.Policy, o.Seed, o.Days)
		}
	}
}

// The Phase 2 gate, first half: a trade-and-repay finish is possible. The policy
// never borrows, so a win proves the trading economy alone can clear the note.
func TestGateTradeAndRepayFinish(t *testing.T) {
	o, run, steps := findWin(t, TradeRepay, 1, 40)
	if o.Borrowed != 0 {
		t.Fatalf("win on seed %d borrowed %s, so it is not a trade-only finish", o.Seed, game.FormatMoney(o.Borrowed))
	}
	if o.Repaid <= 0 {
		t.Fatalf("win on seed %d repaid nothing, so it is not a trade-and-repay finish", o.Seed)
	}
	if o.Interest <= 0 {
		t.Fatalf("win on seed %d paid no interest, so the note never compounded", o.Seed)
	}
	recordAndReplay(t, "trade-repay-win", TradeRepay, o.Seed, run, steps)
}

// The Phase 2 gate, second half: a warned leveraged crash can force sales. The
// morning before the forced sale must have carried a warning line, so the loss
// is a misread opportunity rather than an ambush.
func TestGateWarnedLeveragedCrash(t *testing.T) {
	var chosen Outcome
	var run *game.Run
	var steps []game.Step
	found := false
	for seed := uint64(1); seed <= 40 && !found; seed++ {
		cfg := Prototype(seed)
		got, gotRun, gotSteps, err := Run(cfg, Leveraged)
		if err != nil {
			t.Fatal(err)
		}
		if got.ForcedUnits == 0 {
			continue
		}
		if got.ForcedDay <= 1 || content.HeadlineFor(got.ForcedDay-1).Tomorrow == "" {
			t.Fatalf("seed %d forced a sale on day %d with no warning the morning before", seed, got.ForcedDay)
		}
		if got.Borrowed <= 0 {
			t.Fatalf("seed %d forced sales without borrowing, so it is not a leveraged crash", seed)
		}
		chosen, run, steps, found = got, gotRun, gotSteps, true
	}
	if !found {
		t.Fatal("no seed in 1..40 produced a forced sale under the leveraged policy")
	}
	t.Log(chosen.String())
	if chosen.ForcedLoss <= 0 {
		t.Fatalf("forced sales cost nothing on seed %d: %d units", chosen.Seed, chosen.ForcedUnits)
	}
	// Every forced sale in the recorded run, not just the first, has to have been
	// announced the morning before it ran. A player who loses the rack to a price
	// move they could not have seen was not making a decision, and the design
	// forbids that.
	for _, st := range steps {
		if len(st.Receipt.Forced) == 0 {
			continue
		}
		if st.Day <= 1 || content.HeadlineFor(st.Day-1).Tomorrow == "" {
			t.Errorf("forced sale of %d units on day %d had no warning the morning before",
				len(st.Receipt.Forced), st.Day)
		}
	}
	recordAndReplay(t, "leveraged-crash", Leveraged, chosen.Seed, run, steps)
}

// actionsAgree compares a recorded action with the decoded one. A powered
// subset written as "[]" reads back as an empty slice, and the policy passed
// nil: both mean "no explicit subset", so they must not be treated as two
// different actions.
func actionsAgree(a, b game.Action) bool {
	if len(a.Powered) != len(b.Powered) {
		return false
	}
	for i := range a.Powered {
		if a.Powered[i] != b.Powered[i] {
			return false
		}
	}
	return a.Kind == b.Kind && a.Product == b.Product && a.Market == b.Market &&
		a.Qty == b.Qty && a.Amount == b.Amount && a.Mode == b.Mode && a.Resp == b.Resp
}

// recordAndReplay writes the trace to testdata and proves the recorded actions
// reproduce the recorded result: the same seed and the same actions must land on
// the same ending, which is what makes a bug report reproducible.
func recordAndReplay(t *testing.T, base string, policy Policy, seed uint64, run *game.Run, steps []game.Step) {
	t.Helper()
	jsonPath, textPath, err := WriteTraces(traceDir, base, run, steps)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := strings.TrimSuffix(jsonPath, ".json") + ".golden.json"
	g := goldenTrace{
		Seed: seed, Policy: policy.Name, Day: run.State.Day, Cash: run.State.Cash, Debt: run.State.Debt,
		NetWorth: run.NetWorth(), EndKind: run.State.End.Kind, CashOut: run.State.End.CashOut,
		InterestPaid: run.State.Stats.InterestPaid, Borrowed: run.State.Stats.Borrowed,
		Repaid: run.State.Stats.Repaid, ForcedUnits: run.State.Stats.ForcedUnits,
		ForcedLoss: run.State.Stats.ForcedLoss, Actions: len(steps),
	}
	raw, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goldenPath, append(raw, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %s and %s (seed %d, %d actions)", jsonPath, textPath, seed, len(steps))

	// Replay from the recorded file itself, not from memory.
	tr, err := game.DecodeTrace(mustRead(t, jsonPath))
	if err != nil {
		t.Fatal(err)
	}
	replay, replaySteps, err := tr.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(replaySteps) != len(steps) {
		t.Fatalf("recorded trace replays %d steps, run had %d", len(replaySteps), len(steps))
	}
	for i, want := range steps {
		got := replaySteps[i]
		if !actionsAgree(want.Action, got.Action) {
			t.Errorf("step %d action mismatch: want %v, got %v", i, want.Action, got.Action)
		}
		if want.Cash != got.Cash || want.Debt != got.Debt || want.NetWorth != got.NetWorth {
			t.Errorf("step %d state mismatch: want cash %s debt %s net %s, got cash %s debt %s net %s",
				i, game.FormatMoney(want.Cash), game.FormatMoney(want.Debt), game.FormatMoney(want.NetWorth),
				game.FormatMoney(got.Cash), game.FormatMoney(got.Debt), game.FormatMoney(got.NetWorth))
		}
	}
	if replay.State.End.Kind != run.State.End.Kind || replay.State.End.CashOut != run.State.End.CashOut {
		t.Fatalf("ending mismatch: got %s cash-out %s, want %s cash-out %s",
			replay.State.End.Kind, game.FormatMoney(replay.State.End.CashOut),
			run.State.End.Kind, game.FormatMoney(run.State.End.CashOut))
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func findWin(t *testing.T, p Policy, from, to uint64) (Outcome, *game.Run, []game.Step) {
	t.Helper()
	for seed := from; seed <= to; seed++ {
		cfg := Prototype(seed)
		o, run, steps, err := Run(cfg, p)
		if err != nil {
			t.Fatal(err)
		}
		if o.Won() {
			t.Logf("%s wins on seed %d: %s", p.Name, seed, o.String())
			return o, run, steps
		}
	}
	t.Fatalf("%s won on no seed in %d..%d: a trade-only finish is not provable yet", p.Name, from, to)
	return Outcome{}, nil, nil
}

// TestRecordedTracesStillReplay keeps the recorded evidence honest: a content or
// engine change that moves a recorded run's outcome must fail here rather than
// silently invalidate the traces the balance gates were judged on.
func TestRecordedTracesStillReplay(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join(traceDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	for _, path := range matches {
		if strings.HasSuffix(path, ".golden.json") {
			continue
		}
		fixtures = append(fixtures, path)
	}
	if len(fixtures) == 0 {
		t.Fatalf("no recorded traces under %s; run the gate tests first", traceDir)
	}
	for _, path := range fixtures {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		tr, err := game.DecodeTrace(raw)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		run, steps, err := tr.Replay()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(steps) == 0 {
			t.Fatalf("%s: no steps", path)
		}
		if run.State.End.Kind == game.EndOpen {
			t.Errorf("%s: recorded trace does not finish", path)
		}

		// The sibling golden file records the state the run produced. It is a
		// summary, not a re-encoding of the trace: replaying the recorded actions
		// must still land on the numbers a phase gate was judged by, so an engine
		// or content change that moves an outcome fails here instead of quietly
		// invalidating the evidence a gate was signed off on.
		goldenPath := strings.TrimSuffix(path, ".json") + ".golden.json"
		goldenRaw, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("%s: %v", goldenPath, err)
		}
		var g goldenTrace
		if err := json.Unmarshal(goldenRaw, &g); err != nil {
			t.Fatalf("%s: %v", goldenPath, err)
		}
		if g.Day == 0 {
			t.Fatalf("%s: golden summary is empty", goldenPath)
		}
		st := run.State
		if st.Day != g.Day || st.Cash != g.Cash || st.Debt != g.Debt || run.NetWorth() != g.NetWorth ||
			run.State.End.Kind != g.EndKind || run.State.End.CashOut != g.CashOut ||
			st.Stats.InterestPaid != g.InterestPaid || st.Stats.Borrowed != g.Borrowed ||
			st.Stats.Repaid != g.Repaid || st.Stats.ForcedUnits != g.ForcedUnits || st.Stats.ForcedLoss != g.ForcedLoss {
			t.Fatalf("%s drifted: replay day %d %s cash %s debt %s net %s cashout %s interest %s borrowed %s repaid %s forced %d ($%s); recorded day %d %s cash %s debt %s net %s cashout %s interest %s borrowed %s repaid %s forced %d ($%s)",
				path, st.Day, run.State.End.Kind, game.FormatMoney(st.Cash), game.FormatMoney(st.Debt),
				game.FormatMoney(run.NetWorth()), game.FormatMoney(run.State.End.CashOut),
				game.FormatMoney(st.Stats.InterestPaid), game.FormatMoney(st.Stats.Borrowed),
				game.FormatMoney(st.Stats.Repaid), st.Stats.ForcedUnits, game.FormatMoney(st.Stats.ForcedLoss),
				g.Day, g.EndKind, game.FormatMoney(g.Cash), game.FormatMoney(g.Debt), game.FormatMoney(g.NetWorth),
				game.FormatMoney(g.CashOut), game.FormatMoney(g.InterestPaid), game.FormatMoney(g.Borrowed),
				game.FormatMoney(g.Repaid), g.ForcedUnits, game.FormatMoney(g.ForcedLoss))
		}
		if g.Actions != len(tr.Actions) {
			t.Errorf("%s: golden says %d actions, trace holds %d", goldenPath, g.Actions, len(tr.Actions))
		}
		t.Logf("%s replays: seed %d policy %s day %d %s cash %s debt %s cashout %s in %d actions",
			path, tr.Config.Seed, g.Policy, st.Day, run.State.End.Kind, game.FormatMoney(st.Cash),
			game.FormatMoney(st.Debt), game.FormatMoney(run.State.End.CashOut), len(tr.Actions))
	}
}

// goldenTrace is the summary a recorded run is checked against.
type goldenTrace struct {
	Seed         uint64       `json:"seed"`
	Policy       string       `json:"policy"`
	Day          int          `json:"day"`
	Cash         game.Money   `json:"cash"`
	Debt         game.Money   `json:"debt"`
	NetWorth     game.Money   `json:"netWorth"`
	EndKind      game.EndKind `json:"endKind"`
	CashOut      game.Money   `json:"cashOut"`
	InterestPaid game.Money   `json:"interestPaid"`
	Borrowed     game.Money   `json:"borrowed"`
	Repaid       game.Money   `json:"repaid"`
	ForcedUnits  int          `json:"forcedUnits"`
	ForcedLoss   game.Money   `json:"forcedLoss"`
	Actions      int          `json:"actions"`
}
