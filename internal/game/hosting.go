package game

import "rackwars/internal/content"

// hosting.go holds the meter and the payout side of a stay: electricity,
// ordinary hosting, the powered-subset picker, and the offers the grid operator
// and invoice reviewers extend. Everything here is whole-dollar integer math,
// and the stay preview and the night's settlement call these same functions,
// so the number shown before confirming is the number on the receipt.

// elecCostDollars is the night's meter reading:
//
//	power kW x 24h x local cents/kWh
//
// carried in hundredths of a kilowatt (the same unit as Product.PoweredKW and
// Market.PowerKW) and rounded up to whole dollars, so the engine never charges
// less than it displayed.
func elecCostDollars(kw100 int64, centsPerKWh int) Money {
	if kw100 <= 0 || centsPerKWh <= 0 {
		return 0
	}
	cents := kw100 * 24 * int64(centsPerKWh)
	return Money((cents + 9999) / 10000) // centi-kWh x cents / 10000 -> dollars
}

// hostingPct compounds the featured-workload windows active for a card in a
// town on a day. A window flagged RequiresClean is skipped when the meter runs
// hot: refusing a premium contract at 60+ heat is what keeps normal trading
// open as the way out. The prototype's campaign holds no hosting rows and the
// hosting rule is off, so this returns the neutral 100 there.
func (r *Run) hostingPct(day int, m content.MarketID, p content.ProductID) int {
	pct := 100
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectHosting || day >= h.Day+e.Days {
				continue
			}
			if e.RequiresClean && r.State.Heat >= 60 {
				continue
			}
			if !e.Matches(m, p) {
				continue
			}
			pct = clampPct(pct*e.Pct/100, campaignFloor, campaignCeiling)
		}
	}
	return pct
}

// hostEligible reports whether hosting can run at all on this site under this
// run's rules. Huaqiangbei has no meter to host behind; the garage has a small
// one, which is the whole reason its hosting strip stays modest.
func (r *Run) hostEligible(m content.MarketID) bool {
	return r.cfg.Rules.Hosting && r.cfg.Rules.Power && content.MarketOf(m).Site
}

// hostUnitGross is one unit's night of ordinary hosting at a town, after the
// featured windows. Integer per unit, so Σ preview equals Σ settlement.
func hostUnitGross(p content.ProductID, pct int) Money {
	return Money(content.ProductOf(p).GrossHostingPerDay) * Money(pct) / 100
}

// hostNight prices a powered subset for one night: total gross and the share
// earned by enterprise cards, which is the number an invoice review reaches
// for the next day. A subset above the site's meter is not priced: the caller
// blocks the stay before the player can confirm it.
func (r *Run) hostNight(day int, m content.MarketID, powered []int) (total, ent Money) {
	if !r.hostEligible(m) {
		return 0, 0
	}
	for pi, p := range r.cfg.Products {
		n := powered[pi]
		if n <= 0 {
			continue
		}
		g := hostUnitGross(p, r.hostingPct(day, m, p)) * Money(n)
		total += g
		if content.ProductOf(p).Enterprise {
			ent += g
		}
	}
	return total, ent
}

// poweredKW100 is the metered load of a subset.
func poweredKW100(products []content.ProductID, powered []int) int64 {
	var kw int64
	for pi := range products {
		if powered[pi] > 0 {
			kw += int64(powered[pi]) * content.ProductOf(products[pi]).PoweredKW
		}
	}
	return kw
}

// bestPowered picks the most valuable subset of the fleet that fits the site's
// meter: an exact knapsack over the expanded fleet, not a greedy hunch. Values
// are per-night hosting gross; weight is hundredths of a kilowatt. Units are
// expanded in product order and ties prefer the earlier decision (the DP keeps
// the previous row on equality), so the same fleet always packs the same way —
// preview, settlement, and a replay months later never disagree about which
// subset "auto" meant on a day long past.
func (r *Run) bestPowered(day int, m content.MarketID) []int {
	np := len(r.cfg.Products)
	out := make([]int, np)
	if !r.hostEligible(m) {
		return out
	}
	siteCap := int(content.MarketOf(m).PowerKW)
	if siteCap <= 0 {
		return out
	}
	// Expand the fleet into unit items: value = one night's gross, weight =
	// centi-kW. Units that cannot fit alone are skipped, as are idle cards
	// with no metered load.
	type unitItem struct {
		pi int
		v  int64
		w  int
	}
	var items []unitItem
	for pi, p := range r.cfg.Products {
		def := content.ProductOf(p)
		if def.PoweredKW <= 0 || def.PoweredKW > int64(siteCap) {
			continue
		}
		v := int64(hostUnitGross(p, r.hostingPct(day, m, p)))
		if v <= 0 || r.State.Owned[pi] <= 0 {
			continue
		}
		for u := 0; u < r.State.Owned[pi]; u++ {
			items = append(items, unitItem{pi: pi, v: v, w: int(def.PoweredKW)})
		}
	}
	if len(items) == 0 {
		return out
	}
	// rows[i] is the value vector after the first i items.
	rows := make([][]int64, len(items)+1)
	for i := range rows {
		rows[i] = make([]int64, siteCap+1)
	}
	for i, it := range items {
		prev, cur := rows[i], rows[i+1]
		copy(cur, prev)
		for c := it.w; c <= siteCap; c++ {
			if cand := prev[c-it.w] + it.v; cand > cur[c] {
				cur[c] = cand
			}
		}
	}
	// Recover: walk the table backward; a changed cell means item i is taken.
	c := siteCap
	for i := len(items); i > 0; i-- {
		if rows[i][c] != rows[i-1][c] {
			out[items[i-1].pi]++
			c -= items[i-1].w
		}
	}
	return out
}

// curtailOffer finds the grid operator's disclosed offer for a town, if one is
// active: staying switched off pays this instead of hosting, and the morning
// board says so before the player commits the night. A run with hosting or
// power switched off has no metered night to buy back, so it never hears the
// offer.
func (r *Run) curtailOffer(day int, m content.MarketID) (Money, bool) {
	if !r.cfg.Rules.Hosting || !r.cfg.Rules.Power || !content.MarketOf(m).Site {
		return 0, false
	}
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectCurtailment || day >= h.Day+e.Days || e.Dollars <= 0 {
				continue
			}
			for _, em := range e.Markets {
				if em == m {
					return Money(e.Dollars), true
				}
			}
		}
	}
	return 0, false
}

// clawbackActive reports the invoice review running on a day, if any: hosting
// with a matching card last night gives back a share of that payout, capped.
func (r *Run) clawbackActive(day int) (pct, cap Money, ok bool) {
	if !r.cfg.Rules.Hosting {
		return 0, 0, false
	}
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectClawback || day >= h.Day+e.Days {
				continue
			}
			return Money(e.Pct), Money(e.Dollars), true
		}
	}
	return 0, 0, false
}

// AllocationOffer prices Andy's constrained supply for a town on a day:
// up to stock units of one card below street, one offer per run, clean
// operations only. The answer embeds the eligibility so the TUI can refuse
// honestly without recomputing the reason.
type AllocationOffer struct {
	Product     content.ProductID `json:"product"`
	Pct         int               `json:"pct"` // percent of the local ask
	UnitPrice   Money             `json:"unitPrice"`
	Ask         Money             `json:"ask"`
	Stock       int               `json:"stock"` // units left in the window
	Eligible    bool              `json:"eligible"`
	BlockReason string            `json:"blockReason,omitempty"`
}

func (r *Run) allocationActive(day int, m content.MarketID) (content.ProductID, int, int, int, bool) {
	if !r.cfg.Rules.Allocations {
		return "", 0, 0, 0, false
	}
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectAllocation || day >= h.Day+e.Days {
				continue
			}
			matched := false
			for _, em := range e.Markets {
				if em == m {
					matched = true
					break
				}
			}
			if matched && len(e.Products) > 0 {
				return e.Products[0], e.Pct, e.Dollars, h.Day, true
			}
		}
	}
	return "", 0, 0, 0, false
}

// syncAllocationStock opens the window's counter on the morning it starts:
// shop-like stock a town's regular shelf never carries. Units never refill
// mid-window, and the once-per-run gate (AllocTaken) stays closed across
// windows.
func (r *Run) syncAllocationStock(day int) {
	if !r.cfg.Rules.Allocations {
		return
	}
	_, _, units, start, ok := r.allocationActive(day, r.State.Market)
	if !ok {
		r.State.AllocStock = 0
		r.State.AllocOpenDay = 0
		return
	}
	if start != r.State.AllocOpenDay {
		r.State.AllocOpenDay = start
		if !r.State.AllocTaken {
			r.State.AllocStock = units
		}
	}
}

// PreviewAllocation answers "what is Andy selling here today?" with money
// already rounded down in the player's favor and the exact reason a hot rack
// is refused. Costs no day; take it with ActAllocation.
func (r *Run) PreviewAllocation() AllocationOffer {
	a := AllocationOffer{}
	if r.State.Over != EndOpen || !r.cfg.Rules.Allocations {
		return a
	}
	p, pct, _, _, ok := r.allocationActive(r.State.Day, r.State.Market)
	if !ok {
		return a
	}
	a.Product, a.Pct = p, pct
	q := r.QuoteHere(p)
	a.Ask = q.Ask
	a.UnitPrice = q.Ask * Money(pct) / 100 // below street, rounded down
	a.Stock = r.State.AllocStock
	switch {
	case r.State.AllocTaken:
		a.BlockReason = "already accepted this run: Andy remembers faces"
	case r.State.Heat >= 60:
		a.BlockReason = "clean operations required: heat " + itoa(r.State.Heat) + " is at or over 60"
	case a.Stock <= 0:
		a.BlockReason = "the window is sold out"
	default:
		a.Eligible = true
	}
	return a
}
