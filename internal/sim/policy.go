// Package sim runs the engine headlessly: deterministic policies play complete
// runs, and the resulting action traces are the evidence the balance gates are
// judged on. Nothing here renders anything or reads input; a policy sees exactly
// what a player sees in the terminal (current quotes, last-seen quotes in other
// towns, the headline, the warning line) and nothing else.
package sim

import (
	"rackwars/internal/content"
	"rackwars/internal/game"
)

// Policy is a named decision function. It must be a pure function of the run:
// no randomness of its own, no knowledge the terminal withholds, and no reading
// of the campaign table. A policy that loses is a finding, not a bug, so each
// policy stays small enough to read and argue with.
type Policy struct {
	Name string
	Next func(r *game.Run) game.Action
}

// relQuote returns a posted price against the town's usual midpoint for that
// card, scaled by 1000: 1000 is an ordinary day, 300 is a collapse, 1500 is a
// rush. This is the number a player can reconstruct by eye, and because campaign
// effects expire, a posted price far under 1000 is a discount with a known
// expiry rather than a permanent price change.
func relQuote(price game.Money, market content.MarketID, p content.ProductID) int64 {
	base := game.Money(content.ProductOf(p).BaseMidpoint) * game.Money(content.MarketOf(market).TownFactorPct) / 100
	if base <= 0 {
		return 0
	}
	return int64(price) * 1000 / int64(base)
}

// The trading plan's two thresholds, read off the posted board rather than off
// any hidden calendar: buy at or under half of a town's usual midpoint, sell at
// or above 140% of it. That is the merchant reading of a board, and it is the
// loop the design describes - buy what everybody hates, sell what everybody
// suddenly needs. The campaign's crash and rush days are exactly where those
// numbers appear, and the warning line tells the player one morning ahead that
// something is coming.
const (
	buyRelMax  = 500
	sellRelMin = 1400
)

// TradeRepay is the design's intended loop and the Phase 2 gate instrument: buy
// where the posted ask is far under the town's usual midpoint, hold through the
// announced rebound, realize the rush, and put the proceeds on the note before it
// compounds. It never borrows, which is what the plan requires of a prototype
// victory: a trade-only finish.
var TradeRepay = Policy{Name: "trade-repay", Next: tradeRepay}

func tradeRepay(r *game.Run) game.Action {
	// Learn the map while flat. Freight is small against thirty days of interest,
	// and a town whose board you have never seen is not a market you can plan
	// for; a crash board two towns over is only useful once you know it exists.
	if r.SlotsUsed() == 0 {
		for _, m := range r.Markets() {
			if m != r.State.Market && r.SeenDayOf(m) == 0 {
				if pv := r.PreviewTravel(m); pv.Valid {
					return game.Action{Kind: game.ActTravel, Market: m}
				}
			}
		}
	}

	// Close out before the auction: the closing board liquidates everything
	// anyway, so selling one morning early costs nothing and keeps the proceeds
	// off the lender's forced-sale schedule.
	closing := r.State.Day >= content.DayCount-1

	// Sell into a rush, or sell because the note needs cash before the lender
	// chooses the day.
	for _, p := range bestSellOrder(r) {
		if r.OwnedOf(p) <= 0 {
			continue
		}
		here := relQuote(r.QuoteHere(p).Bid, r.State.Market, p)
		desperate := r.State.Debt > r.CreditCeiling()*9/10
		if !closing && !desperate && here < sellRelMin {
			continue
		}
		if lim := r.SellLimits(p); lim.Max > 0 {
			return game.Action{Kind: game.ActSell, Product: p, Qty: lim.Max}
		}
	}

	// Buy the deepest crash on this board, sized by cash, shelf stock, and the
	// capacity available where the run is standing. Cheap per slot buys more
	// units, and units are what compound.
	if a, ok := buyCrash(r); ok {
		return a
	}

	// Carry the position toward a town that last posted a rush for it, or walk
	// toward a crash board this policy would buy while still flat.
	if a, ok := carryToBestBid(r); ok {
		return a
	}
	if r.SlotsUsed() == 0 {
		if a, ok := travelToDiscount(r); ok {
			return a
		}
	}

	// Nothing to buy and nothing to sell: shrink the note. Idle cash is the only
	// asset in the game that guarantees a loss, and the credit ceiling is what
	// stands between a bad morning and a repossession.
	if r.State.Debt > 0 {
		float := game.Money(1000)
		if r.State.Cash > float {
			amount := r.State.Cash - float
			if amount > r.State.Debt {
				amount = r.State.Debt
			}
			if pv := r.PreviewRepay(amount); pv.Valid {
				return game.Action{Kind: game.ActRepay, Amount: amount}
			}
		}
	}
	return game.Action{Kind: game.ActStay}
}

// bestSellOrder lists the cast with the best current relative bid first, so a
// forced or urgent sale realizes the strongest card first. Ties keep product
// order, which makes the choice reproducible.
func bestSellOrder(r *game.Run) []content.ProductID {
	ps := append([]content.ProductID(nil), r.Products()...)
	for i := range ps {
		for j := i + 1; j < len(ps); j++ {
			mine := relQuote(r.QuoteHere(ps[i]).Bid, r.State.Market, ps[i])
			theirs := relQuote(r.QuoteHere(ps[j]).Bid, r.State.Market, ps[j])
			if mine < theirs || (mine == theirs && ps[i] > ps[j]) {
				ps[i], ps[j] = ps[j], ps[i]
			}
		}
	}
	return ps
}

// carryToBestBid moves a position toward a town that last posted a rush for it.
// A board that merely pays a little more is not worth a day of interest and a
// freight bill, so the destination has to be a real sell target: at or above the
// policy's own rush threshold, and better than this town.
func carryToBestBid(r *game.Run) (game.Action, bool) {
	bestMargin := int64(0)
	var bestAction game.Action
	found := false
	for _, p := range r.Products() {
		if r.OwnedOf(p) <= 0 {
			continue
		}
		here := relQuote(r.QuoteHere(p).Bid, r.State.Market, p)
		for _, m := range r.Markets() {
			if m == r.State.Market || r.SeenDayOf(m) == 0 {
				continue
			}
			seen, ok := r.QuoteOf(m, p)
			if !ok {
				continue
			}
			there := relQuote(seen.Bid, m, p)
			if there < sellRelMin {
				continue
			}
			margin := there - here
			if margin <= bestMargin {
				continue
			}
			pv := r.PreviewTravel(m)
			if !pv.Valid {
				continue
			}
			// The trip has to pay for its own freight bill out of the extra the
			// destination is expected to add, or the position stays put.
			base := game.Money(content.ProductOf(p).BaseMidpoint) * game.Money(content.MarketOf(m).TownFactorPct) / 100
			gain := base * game.Money(margin) / 1000 * game.Money(r.OwnedOf(p))
			if gain <= pv.Freight {
				continue
			}
			bestMargin, bestAction, found = margin, game.Action{Kind: game.ActTravel, Market: m}, true
		}
	}
	return bestAction, found
}

// buyCrash purchases the deepest discount on this board, at the quantity cash,
// shelf stock, and the town's own capacity allow. BuyLimits already binds all
// three, and the engine reports which one bound.
func buyCrash(r *game.Run) (game.Action, bool) {
	bestRel := int64(buyRelMax + 1)
	var bestAction game.Action
	found := false
	for _, p := range r.Products() {
		rel := relQuote(r.QuoteHere(p).Ask, r.State.Market, p)
		if rel > bestRel {
			continue
		}
		units := r.BuyLimits(p).Max
		if units <= 0 || !r.PreviewBuy(p, units).Valid {
			continue
		}
		if rel < bestRel {
			bestRel = rel
			bestAction = game.Action{Kind: game.ActBuy, Product: p, Qty: units}
			found = true
		}
	}
	return bestAction, found
}

// travelToDiscount moves toward the town with the deepest posted discount for a
// card this policy would buy, so the next morning can purchase it.
func travelToDiscount(r *game.Run) (game.Action, bool) {
	bestRel := int64(buyRelMax + 1)
	var bestAction game.Action
	found := false
	for _, m := range r.Markets() {
		if m == r.State.Market || r.SeenDayOf(m) == 0 {
			continue
		}
		for _, p := range r.Products() {
			seen, ok := r.QuoteOf(m, p)
			if !ok {
				continue
			}
			rel := relQuote(seen.Ask, m, p)
			if rel >= bestRel {
				continue
			}
			if pv := r.PreviewTravel(m); !pv.Valid {
				continue
			}
			bestRel, bestAction, found = rel, game.Action{Kind: game.ActTravel, Market: m}, true
		}
	}
	return bestAction, found
}

// Hold is the passive control: buy one cheap card with cash on day 1, hold it,
// and keep the note paid down. It shows what churning is worth against simply
// sitting on collateral, and it is the policy that proves how fast the note
// compounds when nobody trades.
var Hold = Policy{Name: "hold", Next: hold}

func hold(r *game.Run) game.Action {
	if r.State.Day == 1 && r.OwnedOf(content.Card4090) == 0 {
		if lim := r.BuyLimits(content.Card4090); lim.Max > 0 {
			qty := lim.Max
			if qty > 4 {
				qty = 4
			}
			if pv := r.PreviewBuy(content.Card4090, qty); pv.Valid {
				return game.Action{Kind: game.ActBuy, Product: content.Card4090, Qty: qty}
			}
		}
	}
	for _, p := range r.Products() {
		if r.OwnedOf(p) <= 0 {
			continue
		}
		if relQuote(r.QuoteHere(p).Bid, r.State.Market, p) >= 1300 || r.State.Day >= content.DayCount-1 {
			if lim := r.SellLimits(p); lim.Max > 0 {
				return game.Action{Kind: game.ActSell, Product: p, Qty: lim.Max}
			}
		}
	}
	if r.State.Debt > 0 {
		float := game.Money(1000)
		if r.State.Cash > float {
			amount := r.State.Cash - float
			if amount > r.State.Debt {
				amount = r.State.Debt
			}
			if pv := r.PreviewRepay(amount); pv.Valid {
				return game.Action{Kind: game.ActRepay, Amount: amount}
			}
		}
	}
	return game.Action{Kind: game.ActStay}
}

// Leveraged is the design's failure mode written as a policy: borrow to the
// ceiling, buy the biggest card the ceiling can carry, and hold it through the
// announced enterprise collapse. It is what a player who reads only the upside
// does, and the trace it produces is the gate evidence that a warned crash forces
// sales.
var Leveraged = Policy{Name: "leveraged", Next: leveraged}

func leveraged(r *game.Run) game.Action {
	if avail := r.AvailableCredit(); avail > 0 {
		for _, p := range biggestFirst(r.Products()) {
			if avail >= r.QuoteHere(p).Ask {
				if pv := r.PreviewBorrow(avail); pv.Valid {
					return game.Action{Kind: game.ActBorrow, Amount: avail}
				}
				break
			}
		}
	}
	for _, p := range biggestFirst(r.Products()) {
		if lim := r.BuyLimits(p); lim.Max > 0 {
			if pv := r.PreviewBuy(p, lim.Max); pv.Valid {
				return game.Action{Kind: game.ActBuy, Product: p, Qty: lim.Max}
			}
		}
	}
	// Move the position toward the town that last paid best for it, but never
	// realize: the leveraged player is waiting for the keynote that never comes.
	for _, p := range biggestFirst(r.Products()) {
		if r.OwnedOf(p) <= 0 {
			continue
		}
		var target content.MarketID
		bestRel := relQuote(r.QuoteHere(p).Bid, r.State.Market, p)
		for _, m := range r.Markets() {
			if m == r.State.Market {
				continue
			}
			if seen, ok := r.QuoteOf(m, p); ok {
				if rel := relQuote(seen.Bid, m, p); rel > bestRel {
					bestRel, target = rel, m
				}
			}
		}
		if target != "" {
			if pv := r.PreviewTravel(target); pv.Valid {
				return game.Action{Kind: game.ActTravel, Market: target}
			}
		}
	}
	return game.Action{Kind: game.ActStay}
}

// biggestFirst orders the cast by base midpoint, dearest first, ties by product
// id, so the choice is reproducible and independent of map iteration order.
func biggestFirst(ps []content.ProductID) []content.ProductID {
	out := append([]content.ProductID(nil), ps...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if content.ProductOf(out[i]).BaseMidpoint < content.ProductOf(out[j]).BaseMidpoint ||
				(content.ProductOf(out[i]).BaseMidpoint == content.ProductOf(out[j]).BaseMidpoint && out[i] > out[j]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// All is the policy set the balance suite runs.
func All() []Policy { return []Policy{TradeRepay, Hold, Leveraged} }
