package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"rackwars/internal/content"
)

// Step is one recorded move: the action, the engine's receipt, and the
// position after it. A list of Steps is enough to reproduce a run, to compare
// two policies on one seed, and to write the balance report without a TUI.
type Step struct {
	Index    int              `json:"index"`
	Day      int              `json:"day"`
	Market   content.MarketID `json:"market"`
	Action   Action           `json:"action"`
	Receipt  Receipt          `json:"receipt"`
	OK       bool             `json:"ok"`
	Reason   Reason           `json:"reason"`
	Cash     Money            `json:"cash"`
	Debt     Money            `json:"debt"`
	NetWorth Money            `json:"netWorth"`
	Slots    int              `json:"slots"`
}

// Play applies actions in order and stops at the first action the engine
// refuses, recording why. Stopping rather than skipping matters: a policy that
// would have needed a different action is a different policy, and a balance
// report has to say which one ran.
func (r *Run) Play(actions []Action) []Step {
	var steps []Step
	for i, a := range actions {
		rec, res := r.Apply(a)
		st := Step{Index: i, Day: r.State.Day, Market: r.State.Market, Action: a,
			Receipt: rec, OK: res.OK, Reason: res.Reason,
			Cash: r.State.Cash, Debt: r.State.Debt, NetWorth: r.NetWorth(), Slots: r.SlotsUsed()}
		steps = append(steps, st)
		if !res.OK {
			break
		}
		if r.State.Over != EndOpen {
			break
		}
	}
	return steps
}

// Trace is a seed plus a list of actions: the whole of a run's explanation.
type Trace struct {
	Config  Config   `json:"config"`
	Actions []Action `json:"actions"`
}

// Replay runs a trace from the beginning and returns the run plus its steps.
// Same seed, same content, same actions, same result, on any machine.
func (t Trace) Replay() (*Run, []Step, error) {
	r, err := New(t.Config)
	if err != nil {
		return nil, nil, err
	}
	return r, r.Play(t.Actions), nil
}

// EncodeTrace renders a trace as JSON.
func EncodeTrace(t Trace) ([]byte, error) { return json.MarshalIndent(t, "", "  ") }

// DecodeTrace reads a trace back. An unknown action kind or product is a hard
// error, never a silently skipped line.
func DecodeTrace(b []byte) (Trace, error) {
	var t Trace
	if err := json.Unmarshal(b, &t); err != nil {
		return Trace{}, err
	}
	if len(t.Config.Products) == 0 || len(t.Config.Markets) == 0 {
		return Trace{}, fmt.Errorf("trace is missing its content set")
	}
	for i, a := range t.Actions {
		switch a.Kind {
		case ActBuy, ActSell:
			if r := newRunForCheck(t.Config); r.Index(a.Product) < 0 {
				return Trace{}, fmt.Errorf("action %d names unknown product %q", i, a.Product)
			}
		case ActTravel:
			if r := newRunForCheck(t.Config); r.MarketIndex(a.Market) < 0 {
				return Trace{}, fmt.Errorf("action %d names unknown market %q", i, a.Market)
			}
		case ActBorrow, ActRepay, ActStay:
		default:
			return Trace{}, fmt.Errorf("action %d has unknown kind %d", i, a.Kind)
		}
	}
	return t, nil
}

func newRunForCheck(cfg Config) *Run {
	r, err := New(cfg)
	if err != nil {
		// DecodeTrace reports the content problem through the index checks.
		return &Run{cfg: cfg}
	}
	return r
}

// TextTrace renders steps in the compact form the design asks a player to be
// able to paste into a bug report: one line per action, dollar amounts visible.
func TextTrace(r *Run, steps []Step) string {
	var b strings.Builder
	fmt.Fprintf(&b, "seed %d day %d %s cash %s debt %s\n",
		r.cfg.Seed, r.State.Day, r.State.Market, FormatMoney(r.State.Cash), FormatMoney(r.State.Debt))
	cast := make([]string, 0, len(r.cfg.Products))
	for _, p := range r.cfg.Products {
		cast = append(cast, string(p))
	}
	towns := make([]string, 0, len(r.cfg.Markets))
	for _, m := range r.cfg.Markets {
		towns = append(towns, string(m))
	}
	fmt.Fprintf(&b, "cast %s markets %s rules %s\n", strings.Join(cast, ","), strings.Join(towns, ","), rulesLabel(r.cfg.Rules))
	for _, s := range steps {
		fmt.Fprintf(&b, "%s\n", formatStep(r, s))
	}
	if r.State.Over != EndOpen {
		fmt.Fprintf(&b, "over: %s cash-out %s rank %s\n", endKindLabel(r.State.Over),
			FormatMoney(r.State.End.CashOut), r.State.End.Rank)
	}
	return b.String()
}

func rulesLabel(r content.Rules) string {
	if r == (content.Rules{}) {
		return "prototype"
	}
	var names []string
	if r.Hosting {
		names = append(names, "hosting")
	}
	if r.Power {
		names = append(names, "power")
	}
	if r.Heat {
		names = append(names, "heat")
	}
	if r.Encounters {
		names = append(names, "encounters")
	}
	if r.Leases {
		names = append(names, "leases")
	}
	if r.Allocations {
		names = append(names, "allocations")
	}
	return strings.Join(names, "+")
}

func endKindLabel(k EndKind) string {
	return k.String()
}

// String names the ending for reports and the ending screen.
func (e EndKind) String() string {
	switch e {
	case EndClosed:
		return "closed"
	case EndRepossessed:
		return "repossessed"
	}
	return "open"
}

func formatStep(r *Run, s Step) string {
	moves := ""
	switch s.Action.Kind {
	case ActBuy:
		moves = fmt.Sprintf("buy %s x%d", s.Action.Product, s.Action.Qty)
	case ActSell:
		moves = fmt.Sprintf("sell %s x%d", s.Action.Product, s.Action.Qty)
	case ActBorrow:
		moves = fmt.Sprintf("borrow %s", FormatMoney(s.Action.Amount))
	case ActRepay:
		moves = fmt.Sprintf("repay %s", FormatMoney(s.Action.Amount))
	case ActTravel:
		moves = fmt.Sprintf("travel -> %s", s.Action.Market)
	case ActStay:
		moves = "stay"
	}
	state := ""
	if s.OK {
		state = fmt.Sprintf("cash %s debt %s net %s slots %d", FormatMoney(s.Cash), FormatMoney(s.Debt),
			FormatMoney(s.NetWorth), s.Slots)
		if s.Receipt.Interest > 0 {
			state += " interest " + FormatMoney(s.Receipt.Interest)
		}
		if s.Receipt.Freight > 0 {
			state += " freight " + FormatMoney(s.Receipt.Freight)
		}
		if len(s.Receipt.Forced) > 0 {
			state += fmt.Sprintf(" forced %d", len(s.Receipt.Forced))
		}
		if s.Receipt.Gross > 0 {
			state += " gross " + FormatMoney(s.Receipt.Gross)
		}
		if s.Receipt.Cost > 0 && (s.Action.Kind == ActBuy || s.Action.Kind == ActRepay) {
			state += " cost " + FormatMoney(s.Receipt.Cost)
		}
	} else {
		state = "refused: " + s.Reason.String()
	}
	return fmt.Sprintf("d%-2d %-10s %-22s %s", s.Day, s.Market, moves, state)
}

// Summary condenses a finished run into the columns the balance report uses.
type Summary struct {
	Seed        uint64 `json:"seed"`
	Finished    bool   `json:"finished"`
	EndKind     string `json:"endKind"`
	FailureDay  int    `json:"failureDay"`
	CashOut     Money  `json:"cashOut"`
	Rank        string `json:"rank"`
	Interest    Money  `json:"interest"`
	TradeProfit Money  `json:"tradeProfit"`
	Hosting     Money  `json:"hosting"`
	ForcedLoss  Money  `json:"forcedLoss"`
	ForcedUnits int    `json:"forcedUnits"`
	BestFlip    Money  `json:"bestFlip"`
	PeakNet     Money  `json:"peakNet"`
	Freight     Money  `json:"freight"`
	LargestPos  int    `json:"largestPosition"`
	Commits     int    `json:"commits"`
	Refusals    int    `json:"refusals"`
}

// Summarize builds the balance-report row for a run and its steps.
func (r *Run) Summarize(steps []Step) Summary {
	s := Summary{
		Seed:        r.cfg.Seed,
		Finished:    r.State.Over != EndOpen,
		EndKind:     endKindLabel(r.State.Over),
		CashOut:     r.State.End.CashOut,
		Rank:        r.State.End.Rank,
		Interest:    r.State.Stats.InterestPaid,
		TradeProfit: r.State.Stats.TradeProfit,
		Hosting:     r.State.Stats.HostingIncome,
		ForcedLoss:  r.State.Stats.ForcedLoss,
		ForcedUnits: r.State.Stats.ForcedUnits,
		BestFlip:    r.State.Stats.BestFlip,
		PeakNet:     r.State.Stats.PeakNetWorth,
		Freight:     r.State.Stats.FreightPaid,
		Commits:     r.State.Stats.Commitments,
	}
	for _, st := range steps {
		if !st.OK {
			s.Refusals++
			if s.FailureDay == 0 {
				s.FailureDay = st.Day
			}
			continue
		}
		for _, n := range r.State.Owned {
			if n > s.LargestPos {
				s.LargestPos = n
			}
		}
	}
	if s.FailureDay == 0 && r.State.Over == EndRepossessed {
		s.FailureDay = r.State.Day
	}
	return s
}

// QuoteBoard is the engine's own rendering of the current market, used by
// headless traces and by the TUI alike. Sharing it is what makes the board the
// engine's, not the interface's.
func (r *Run) QuoteBoard() []BoardRow {
	rows := make([]BoardRow, 0, len(r.cfg.Products))
	for pi, p := range r.cfg.Products {
		q := r.quoteAt(r.MarketIndex(r.State.Market), pi)
		rows = append(rows, BoardRow{
			Product: p,
			Name:    content.ProductOf(p).Name,
			Ask:     q.Ask,
			Bid:     q.Bid,
			Stock:   r.State.Stock[pi],
			Owned:   r.State.Owned[pi],
			Slots:   content.ProductOf(p).SlotsPerUnit,
			AvgCost: r.AvgCost(p),
		})
	}
	return rows
}

// BoardRow is one line of the market board.
type BoardRow struct {
	Product content.ProductID `json:"product"`
	Name    string            `json:"name"`
	Ask     Money             `json:"ask"`
	Bid     Money             `json:"bid"`
	Stock   int               `json:"stock"`
	Owned   int               `json:"owned"`
	Slots   int               `json:"slotsPerUnit"`
	AvgCost Money             `json:"avgCost"`
}

// SortedProducts returns the cast in a stable order for display.
func (r *Run) SortedProducts() []content.ProductID {
	out := append([]content.ProductID(nil), r.cfg.Products...)
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
