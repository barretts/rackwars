package sim

import (
	"fmt"
	"os"
	"path/filepath"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

// Config is a headless run request. MaxActions bounds the loop so a policy that
// refuses to commit a day cannot hang the balance suite; a run that hits the
// budget is reported as unfinished, never as a win.
type Config struct {
	Seed       uint64
	Products   []content.ProductID
	Markets    []content.MarketID
	Rules      content.Rules
	MaxActions int
}

// Prototype is the Phase 2-3 content set: three cards, three towns, trading
// rules only.
func Prototype(seed uint64) Config {
	return Config{Seed: seed, Products: content.PrototypeProducts, Markets: content.PrototypeMarkets, Rules: content.PrototypeRules(), MaxActions: 400}
}

// Play drives a policy until the run is over or the action budget is spent, and
// returns the finished run with its step list.
func Play(cfg Config, p Policy) (*game.Run, []game.Step, error) {
	run, err := game.New(game.Config{Seed: cfg.Seed, Products: cfg.Products, Markets: cfg.Markets, Rules: cfg.Rules})
	if err != nil {
		return nil, nil, err
	}
	limit := cfg.MaxActions
	if limit <= 0 {
		limit = 400
	}
	actions := make([]game.Action, 0, limit)
	for run.State.Over == game.EndOpen && len(actions) < limit {
		a := p.Next(run)
		actions = append(actions, a)
		if _, res := run.Apply(a); !res.OK {
			break
		}
		// Every sim run polices the ledger: money is integer, cash and debt are
		// never negative, and inventory never exceeds capacity. A run that breaks
		// one of these is reported instead of being scored.
		if run.State.Debt < 0 || run.State.Cash < 0 {
			return nil, nil, fmt.Errorf("seed %d day %d: cash %s debt %s after %v",
				cfg.Seed, run.State.Day, game.FormatMoney(run.State.Cash), game.FormatMoney(run.State.Debt), a)
		}
		if used, free := run.SlotsUsed(), run.SlotsAvailable(); used < 0 || free < 0 {
			return nil, nil, fmt.Errorf("seed %d day %d: %d slots used, %d free", cfg.Seed, run.State.Day, used, free)
		}
	}
	// Re-derive the step log by replaying the same actions against an identical
	// run: the recorded trace has to be reproducible, and a step log that only
	// exists in the memory of the driven run is not evidence.
	replay, err := game.New(game.Config{Seed: cfg.Seed, Products: cfg.Products, Markets: cfg.Markets, Rules: cfg.Rules})
	if err != nil {
		return nil, nil, err
	}
	return run, replay.Play(actions), nil
}

// Outcome is one row of the balance table.
type Outcome struct {
	Policy      string
	Seed        uint64
	Actions     int
	Days        int
	Unfinished  bool
	EndKind     game.EndKind
	Refusal     game.Reason
	Cash        game.Money
	Debt        game.Money
	NetWorth    game.Money
	CashOut     game.Money
	Slots       int
	Borrowed    game.Money
	Repaid      game.Money
	Interest    game.Money
	TradeProfit game.Money
	BestFlip    game.Money
	ForcedUnits int
	ForcedLoss  game.Money
	ForcedDay   int // first day a forced sale ran, 0 if none
}

// Won reports the design's win condition: the run closed at day 30 with cash
// left after the note is settled.
func (o Outcome) Won() bool { return !o.Unfinished && o.EndKind == game.EndClosed && o.CashOut > 0 }

func (o Outcome) String() string {
	return fmt.Sprintf("%-12s seed %5d  %-11s day %2d  cash %10s debt %9s net %11s cashout %11s slots %2d  forced %d (%s)  refused: %s",
		o.Policy, o.Seed, o.EndKind.String(), o.Days,
		game.FormatMoney(o.Cash), game.FormatMoney(o.Debt), game.FormatMoney(o.NetWorth), game.FormatMoney(o.CashOut),
		o.Slots, o.ForcedUnits, game.FormatMoney(o.ForcedLoss), o.Refusal)
}

// Run plays one policy on one seed and summarizes the result.
func Run(cfg Config, p Policy) (Outcome, *game.Run, []game.Step, error) {
	run, steps, err := Play(cfg, p)
	if err != nil {
		return Outcome{}, nil, nil, err
	}
	st := run.State
	o := Outcome{
		Policy:      p.Name,
		Seed:        cfg.Seed,
		Actions:     len(steps),
		Days:        st.Day,
		EndKind:     st.End.Kind,
		Cash:        st.Cash,
		Debt:        st.Debt,
		NetWorth:    run.NetWorth(),
		CashOut:     st.End.CashOut,
		Slots:       run.SlotsUsed(),
		Borrowed:    st.Stats.Borrowed,
		Repaid:      st.Stats.Repaid,
		Interest:    st.Stats.InterestPaid,
		TradeProfit: st.Stats.TradeProfit,
		BestFlip:    st.Stats.BestFlip,
		ForcedUnits: st.Stats.ForcedUnits,
		ForcedLoss:  st.Stats.ForcedLoss,
	}
	if st.Over == game.EndOpen {
		o.Unfinished = true
		for _, s := range steps {
			if !s.OK {
				o.Refusal = s.Reason
			}
		}
	}
	for _, s := range steps {
		if len(s.Receipt.Forced) > 0 && o.ForcedDay == 0 {
			o.ForcedDay = s.Receipt.Day
		}
	}
	return o, run, steps, nil
}

// Scan runs a policy across seeds and returns the outcomes in order. This is the
// shape the Phase 6 twenty-seed balance suite uses.
func Scan(cfg Config, seeds []uint64, policies ...Policy) []Outcome {
	var out []Outcome
	for _, p := range policies {
		for _, seed := range seeds {
			c := cfg
			c.Seed = seed
			o, _, _, err := Run(c, p)
			if err != nil {
				panic(fmt.Sprintf("sim: seed %d policy %s: %v", seed, p.Name, err))
			}
			out = append(out, o)
		}
	}
	return out
}

// WriteTraces records a run as both the JSON action trace used for replay and
// the text trace a human reads, under dir/ with the given base name.
func WriteTraces(dir, base string, run *game.Run, steps []game.Step) (jsonPath, textPath string, err error) {
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	cfg := run.Config()
	actions := make([]game.Action, len(steps))
	for i, s := range steps {
		actions[i] = s.Action
	}
	tr := game.Trace{
		Config:  game.Config{Seed: cfg.Seed, Products: cfg.Products, Markets: cfg.Markets, Rules: cfg.Rules},
		Actions: actions,
	}
	raw, err := game.EncodeTrace(tr)
	if err != nil {
		return "", "", err
	}
	jsonPath = filepath.Join(dir, base+".json")
	if err = os.WriteFile(jsonPath, raw, 0o644); err != nil {
		return "", "", err
	}
	textPath = filepath.Join(dir, base+".txt")
	err = os.WriteFile(textPath, []byte(game.TextTrace(run, steps)), 0o644)
	return jsonPath, textPath, err
}
