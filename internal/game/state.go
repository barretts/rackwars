package game

import (
	"fmt"

	"rackwars/internal/content"
)

// Money is whole dollars throughout the engine; see internal/game/money.go.

// Config fixes the content set for a run. Seed, product cast, market map, and
// rule switches together identify the ruleset a save or trace refers to.
type Config struct {
	// The JSON field names are explicit: a save file states the world it was
	// dealt from in terms a player can read, and a Go field rename must not
	// silently change what existing saves mean.
	Seed     uint64              `json:"seed"`
	Products []content.ProductID `json:"products"`
	Markets  []content.MarketID  `json:"markets"`
	Rules    content.Rules       `json:"rules"`
}

// DefaultPrototypeConfig is the Phase 2-3 configuration: three cards, three
// towns, no hosting, no heat, no encounters, no leases.
func DefaultPrototypeConfig(seed uint64) Config {
	return Config{
		Seed:     seed,
		Products: content.PrototypeProducts,
		Markets:  content.PrototypeMarkets,
		Rules:    content.PrototypeRules(),
	}
}

// Quote is a firm local price pair. Invariant: 0 < Bid < Ask, so buying and
// immediately selling in the same town always loses money.
type Quote struct {
	Ask Money `json:"ask"`
	Bid Money `json:"bid"`
}

// Stats are the numbers the end screen reports. They are derived records, not
// rules: nothing here feeds back into a decision.
type Stats struct {
	InterestPaid  Money `json:"interestPaid"`
	Borrowed      Money `json:"borrowed"`
	Repaid        Money `json:"repaid"`
	TradeProfit   Money `json:"tradeProfit"`
	BestFlip      Money `json:"bestFlip"`
	HostingIncome Money `json:"hostingIncome"`
	PowerCost     Money `json:"powerCost"`
	LeasePaid     Money `json:"leasePaid"`
	// CurtailIncome is grid-operator money taken for a switched-off night;
	// ClawbackPaid is invoice reinterpretation returned to counterparties.
	// Both ride the end screen so every dollar a stay earned or gave back is
	// accounted for.
	CurtailIncome Money `json:"curtailIncome,omitempty"`
	ClawbackPaid  Money `json:"clawbackPaid,omitempty"`
	FreightPaid   Money `json:"freightPaid"`
	ForcedLoss    Money `json:"forcedLoss"`
	ForcedUnits   int   `json:"forcedUnits"`
	PeakNetWorth  Money `json:"peakNetWorth"`
	TravelDays    int   `json:"travelDays"`
	StayDays      int   `json:"stayDays"`
	Commitments   int   `json:"commitments"`
}

// EndKind classifies how a run finished.
type EndKind int

const (
	// EndOpen means the run is still playable.
	EndOpen EndKind = iota
	// EndClosed is the day-30 closing auction.
	EndClosed
	// EndRepossessed is an uncured credit breach.
	EndRepossessed
)

// End is the result of a finished run. CashOut is negative when the player
// could not settle the note; that deficiency is shown, never hidden.
type End struct {
	Kind       EndKind `json:"kind"`
	CashOut    Money   `json:"cashOut"`
	Deficiency Money   `json:"deficiency"`
	Rank       string  `json:"rank"`
	Epitaph    string  `json:"epitaph"`
}

// State is everything a run needs in order to continue: no terminal, no
// filesystem, no clock, no wall-clock timeout. JSON tags exist because Phase 4
// saves this structure directly.
type State struct {
	Day    int              `json:"day"`
	Market content.MarketID `json:"market"`
	Cash   Money            `json:"cash"`
	Debt   Money            `json:"debt"`
	Heat   int              `json:"heat"`

	// Owned, CostBasis, and Stock align with Config.Products. CostBasis is the
	// total dollars paid for the units still held, so the holdings detail can
	// show an average acquisition price without a transaction ledger.
	Owned     []int   `json:"owned"`
	CostBasis []Money `json:"costBasis"`
	Stock     []int   `json:"stock"`

	// Quotes is market-major then product-major: town i's quote for card j sits
	// at i*len(Products)+j. It holds the last board seen in each town, tagged
	// with the day it was observed. A stale quote is never presented as a
	// guaranteed arrival price.
	Quotes  []Quote `json:"quotes"`
	SeenDay []int   `json:"seenDay"`

	// LastHostEnt is last night's hosting gross earned by enterprise cards,
	// the number an invoice clawback can come back for. Travel clears it.
	LastHostEnt Money `json:"lastHostEnt,omitempty"`
	// AllocTaken marks that this run already accepted an allocation offer:
	// Andy's constrained supply is one offer per run, not a standing order.
	AllocTaken bool `json:"allocTaken,omitempty"`
	// AllocStock is the units the active allocation window still has.
	AllocStock int `json:"allocStock,omitempty"`
	// AllocOpenDay is the campaign day the current allocation window opened
	// on, so stock is refilled once per window, never mid-window.
	AllocOpenDay int `json:"allocOpenDay,omitempty"`
	// PendingEncounter is the mid-day police beat that halts the calendar:
	// travel/stay costs already applied, interest and the next morning wait
	// on the player's answer. PendingKind is the commitment that walked into
	// it; PendingRec is the receipt so far, so the resolution continues the
	// itemization instead of restarting it.
	PendingEncounter *Encounter `json:"pendingEncounter,omitempty"`
	PendingKind      ActionKind `json:"pendingKind,omitempty"`
	PendingRec       *Receipt   `json:"pendingRec,omitempty"`

	Over  EndKind `json:"over"`
	End   End     `json:"end"`
	Stats Stats   `json:"stats"`
}

// Run is the engine: the configuration, the immutable campaign rows for the
// product cast, and the mutable State. Rules are methods on *Run, so the TUI
// cannot reach a rule without going through the engine.
type Run struct {
	cfg   Config
	camp  []content.Headline
	State State
}

// New starts a run on day 1 at the home market with the starter note and the
// morning board already priced.
func New(cfg Config) (*Run, error) {
	if err := content.Validate(cfg.Products, cfg.Markets, cfg.Rules); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	home := ""
	for _, m := range cfg.Markets {
		if content.MarketOf(m).Home {
			home = string(m)
		}
	}
	if home == "" {
		return nil, fmt.Errorf("no home market in %v", cfg.Markets)
	}

	r := &Run{cfg: cfg, camp: content.CampaignFor(cfg.Products)}
	r.State = State{
		Day:    1,
		Market: content.MarketID(home),
		// The owner's opening position: $30,000 in the account on a $20,000
		// note with an empty garage - net worth +$10,000. Day 1 interest is
		// the first bill; nobody starts the run digging out of a hole.
		Cash:      30000,
		Debt:      20000,
		Owned:     make([]int, len(cfg.Products)),
		CostBasis: make([]Money, len(cfg.Products)),
		Stock:     make([]int, len(cfg.Products)),
		Quotes:    make([]Quote, len(cfg.Markets)*len(cfg.Products)),
		SeenDay:   make([]int, len(cfg.Markets)),
	}
	r.refreshMarket(1)
	r.State.Stats.PeakNetWorth = r.NetWorth()
	return r, nil
}

// Config returns the configuration the run was created with.
func (r *Run) Config() Config { return r.cfg }

// Products returns the card cast.
func (r *Run) Products() []content.ProductID { return r.cfg.Products }

// Markets returns the town map.
func (r *Run) Markets() []content.MarketID { return r.cfg.Markets }

// Index maps a product to its column in the aligned slices.
func (r *Run) Index(p content.ProductID) int {
	for i, id := range r.cfg.Products {
		if id == p {
			return i
		}
	}
	return -1
}

// MarketIndex maps a town to its column in the quote columns.
func (r *Run) MarketIndex(m content.MarketID) int {
	for i, id := range r.cfg.Markets {
		if id == m {
			return i
		}
	}
	return -1
}

// OwnedOf returns how many units of a card the player holds.
func (r *Run) OwnedOf(p content.ProductID) int {
	i := r.Index(p)
	if i < 0 {
		return 0
	}
	return r.State.Owned[i]
}

// StockOf returns how many units the current shop has for a card.
func (r *Run) StockOf(p content.ProductID) int {
	i := r.Index(p)
	if i < 0 {
		return 0
	}
	return r.State.Stock[i]
}

// CostBasisOf returns the dollars paid for the units held of a card.
func (r *Run) CostBasisOf(p content.ProductID) Money {
	i := r.Index(p)
	if i < 0 {
		return 0
	}
	return r.State.CostBasis[i]
}

// AvgCost returns the average acquisition price per held unit, or 0 if none.
func (r *Run) AvgCost(p content.ProductID) Money {
	n := r.OwnedOf(p)
	if n <= 0 {
		return 0
	}
	// Whole dollars: the average rounds down, which is the conservative side
	// for a profit claim.
	return r.CostBasisOf(p) / Money(n)
}

// quoteAt reads the stored quote for a town/product pair.
func (r *Run) quoteAt(mi, pi int) Quote {
	return r.State.Quotes[mi*len(r.cfg.Products)+pi]
}

// QuoteOf returns the last-seen quote for a card in a town.
func (r *Run) QuoteOf(m content.MarketID, p content.ProductID) (Quote, bool) {
	mi := r.MarketIndex(m)
	pi := r.Index(p)
	if mi < 0 || pi < 0 {
		return Quote{}, false
	}
	q := r.quoteAt(mi, pi)
	if q.Ask <= 0 {
		return Quote{}, false
	}
	return q, true
}

// QuoteHere returns the current town's firm quote for a card.
func (r *Run) QuoteHere(p content.ProductID) Quote {
	return r.quoteAt(r.MarketIndex(r.State.Market), r.Index(p))
}

// SeenDayOf returns the day a town's quotes were observed, or 0 if never.
func (r *Run) SeenDayOf(m content.MarketID) int {
	mi := r.MarketIndex(m)
	if mi < 0 {
		return 0
	}
	return r.State.SeenDay[mi]
}

// SlotsUsed is the fleet's slot footprint: every owned card's slots, whether
// hosted tonight or riding in the coat. It is what freight bills, and it is
// deliberately NOT what a site must contain — only the hosted subset does,
// and buying or travelling never runs out of slots.
func (r *Run) SlotsUsed() int {
	used := 0
	for i, p := range r.cfg.Products {
		used += r.State.Owned[i] * content.ProductOf(p).SlotsPerUnit
	}
	return used
}

// CollateralValue is what the current town would pay for everything held.
func (r *Run) CollateralValue() Money {
	mi := r.MarketIndex(r.State.Market)
	total := Money(0)
	for i := range r.cfg.Products {
		total += Money(r.State.Owned[i]) * r.quoteAt(mi, i).Bid
	}
	return total
}

// NetWorth is cash plus local sell value minus debt. Negative net worth is a
// normal starting condition, not a game over.
func (r *Run) NetWorth() Money {
	return r.State.Cash + r.CollateralValue() - r.State.Debt
}

// Headline returns the campaign row for the current day.
func (r *Run) Headline() *content.Headline { return content.HeadlineFor(r.State.Day) }

// Warning returns the headline text for tomorrow, which is also this morning's
// warning about the next decision phase.
func (r *Run) Warning() string {
	if r.State.Day >= content.DayCount {
		return ""
	}
	h := content.HeadlineFor(r.State.Day)
	if h == nil {
		return ""
	}
	return h.Tomorrow
}

// TomorrowInterest is the charge a stay or travel adds at the current balance:
// ceil(debt x 0.10).
func (r *Run) TomorrowInterest() Money { return ceil10(r.State.Debt) }

func ceil10(debt Money) Money {
	if debt <= 0 {
		return 0
	}
	return (debt + 9) / 10
}
