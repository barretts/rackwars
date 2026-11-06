package game

import (
	"rackwars/internal/content"
)

// Price anchors implement the design rule exactly:
//
//	mid = base x town factor x headline factor x seeded daily variation
//	ask = ceil(mid x 1.10)
//	bid = floor(mid x 0.90)
//
// with 0 < bid < ask always. Integer arithmetic only, so a preview shown to the
// player and the money actually charged are the same number.

const (
	// ordinaryVariationLow/High bound the seeded daily move: 0.65-1.35x.
	ordinaryVariationLow  = 65
	ordinaryVariationHigh = 135
	// shockFloor/Ceiling bound the total effect of headlines plus daily noise
	// against a town's usual midpoint: 0.25x to 3.0x. A temporary bad market
	// therefore cannot erase every affordable early-game opportunity.
	shockFloor   = 25
	shockCeiling = 300
	// campaignFloor/Ceiling bounds one compounded headline stack before noise.
	campaignFloor   = 25
	campaignCeiling = 300
)

func clampPct(pct, lo, hi int) int {
	if pct < lo {
		return lo
	}
	if pct > hi {
		return hi
	}
	return pct
}

// campaignPct compounds every headline effect active on a day for a card in a
// town. Effects are dated and bounded, never multipliers left stacked forever.
func (r *Run) campaignPct(day int, m content.MarketID, p content.ProductID) int {
	pct := 100
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectPriceFactor {
				continue
			}
			if day >= h.Day+e.Days {
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

// shockPct is the campaign factor times the seeded daily variation, clamped to
// the announced 0.25-3.0x band.
func (r *Run) shockPct(day int, m content.MarketID, p content.ProductID) int {
	daily := r.drawRange(priceKey(day, string(m), string(p)), ordinaryVariationLow, ordinaryVariationHigh)
	return clampPct(r.campaignPct(day, m, p)*daily/100, shockFloor, shockCeiling)
}

// quoteFor prices one card in one town on one day.
func (r *Run) quoteFor(day int, m content.MarketID, p content.ProductID) Quote {
	base := Money(content.ProductOf(p).BaseMidpoint)
	town := Money(content.MarketOf(m).TownFactorPct)
	mid := base * town / 100
	mid = mid * Money(r.shockPct(day, m, p)) / 100

	ask := (mid*11 + 9) / 10 // ceil(mid x 1.10)
	bid := mid * 9 / 10      // floor(mid x 0.90)
	if bid < 1 {
		bid = 1
	}
	if ask <= bid {
		ask = bid + 1
	}
	return Quote{Ask: ask, Bid: bid}
}

// stockFor replenishes a shop. Stock never refreshes during a decision phase,
// which is what stops a player from farming a town by trading in place.
func (r *Run) stockFor(day int, m content.MarketID, p content.ProductID) int {
	town := content.MarketOf(m)
	base := town.StockMid * content.ProductOf(p).StockBiasPct / 100

	pct := 100
	for _, h := range r.camp {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectStock || day >= h.Day+e.Days {
				continue
			}
			if !e.Matches(m, p) {
				continue
			}
			pct = clampPct(pct*e.Pct/100, 0, 200)
		}
	}
	base = base * pct / 100
	if base < 0 {
		base = 0
	}
	// Finite stock with a bounded spread: a garage that always had exactly the
	// same inventory would be a lookup table, not a market.
	n := r.drawRange(stockKey(day, string(m), string(p)), base-2, base+5)
	if n < 0 {
		n = 0
	}
	return n
}

// refreshMarket prices and stocks the current town for a day and records the
// observation day. Travel refreshes the arrival town; a stay re-prices the town
// the player is already standing in.
func (r *Run) refreshMarket(day int) {
	mi := r.MarketIndex(r.State.Market)
	for pi, p := range r.cfg.Products {
		r.State.Quotes[mi*len(r.cfg.Products)+pi] = r.quoteFor(day, r.State.Market, p)
		r.State.Stock[pi] = r.stockFor(day, r.State.Market, p)
	}
	r.State.SeenDay[mi] = day
	// Andy's counter opens with the morning it advertises, in the town it
	// names, and nowhere else.
	r.syncAllocationStock(day)
}
