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

// hostSlots is the rack footprint of a powered subset: the slots the site
// must have standing to host it. Unlike the fleet, which travels whole and
// bills freight by footprint, only the hosted subset occupies a site.
func hostSlots(products []content.ProductID, powered []int) int {
	n := 0
	for pi := range products {
		if powered[pi] > 0 {
			n += powered[pi] * content.ProductOf(products[pi]).SlotsPerUnit
		}
	}
	return n
}

// bestPowered picks the most valuable subset of the fleet that fits the site
// for a hosted night: both the meter (centi-kW) and the site's rack slots.
// It is an exact knapsack over product groups, not a greedy hunch. Buying is
// unlimited, so the loop is bounded: units of one product are
// interchangeable, and a group never needs more items than the smaller of
// the two capacities can physically hold. A unit's value is its per-night
// hosting gross — identical across every cell of one product, so a product
// with zero gross earns no rack anywhere and per-cell pricing degenerates to
// the global values this used to carry.
//
// Units are expanded in product order and ties prefer the earlier product
// (the DP keeps the previous row on equality), so the same fleet always
// packs the same way — preview, settlement, and a replay months later never
// disagree about which subset "auto" meant on a day long past.
func (r *Run) bestPowered(day int, m content.MarketID) []int {
	np := len(r.cfg.Products)
	out := make([]int, np)
	if !r.hostEligible(m) {
		return out
	}
	mk := content.MarketOf(m)
	kwCap, slotCap := int(mk.PowerKW), mk.Slots
	if kwCap <= 0 || slotCap <= 0 {
		return out
	}
	// Unit items: value = one night's gross, weights = centi-kW and rack
	// slots. Cards that cannot fit alone in either dimension, idle cards,
	// and units with no gross are skipped.
	type unitItem struct {
		pi, kw, sl int
		v          int64
	}
	var items []unitItem
	for pi, p := range r.cfg.Products {
		def := content.ProductOf(p)
		if def.PoweredKW <= 0 || def.SlotsPerUnit <= 0 || r.State.Owned[pi] <= 0 {
			continue
		}
		if def.PoweredKW > int64(kwCap) || def.SlotsPerUnit > slotCap {
			continue
		}
		v := int64(hostUnitGross(p, r.hostingPct(day, m, p)))
		if v <= 0 {
			continue
		}
		count := r.State.Owned[pi]
		if fit := kwCap / int(def.PoweredKW); count > fit {
			count = fit
		}
		if fit := slotCap / def.SlotsPerUnit; count > fit {
			count = fit
		}
		for u := 0; u < count; u++ {
			items = append(items, unitItem{pi: pi, kw: int(def.PoweredKW), sl: def.SlotsPerUnit, v: v})
		}
	}
	if len(items) == 0 {
		return out
	}
	cols := slotCap + 1
	cells := (kwCap + 1) * cols
	// rows[i] is the value grid after the first i units; take[i] marks the
	// cells whose value improved by taking unit i, which is exactly what the
	// backward walk below needs, at one byte a cell instead of a whole
	// extra int64 grid. Descending loops in both dimensions keep every unit
	// 0/1: a unit builds only on states that do not already hold it.
	rows := make([][]int64, len(items)+1)
	for i := range rows {
		rows[i] = make([]int64, cells)
	}
	take := make([][]bool, len(items))
	for i, it := range items {
		prev, cur := rows[i], rows[i+1]
		copy(cur, prev)
		th := make([]bool, cells)
		for w := kwCap; w >= it.kw; w-- {
			base := w * cols
			for s := slotCap; s >= it.sl; s-- {
				if cand := prev[(w-it.kw)*cols+s-it.sl] + it.v; cand > cur[base+s] {
					cur[base+s] = cand
					th[base+s] = true
				}
			}
		}
		take[i] = th
	}
	// Recover: walk from the full site backward; a marked cell means unit i
	// is in the night's set, and its weights leave the remaining capacity.
	w, s := kwCap, slotCap
	for i := len(items); i > 0; i-- {
		it := items[i-1]
		if take[i-1][w*cols+s] {
			out[it.pi]++
			w -= it.kw
			s -= it.sl
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
