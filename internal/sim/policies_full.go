package sim

import (
	"rackwars/internal/content"
	"rackwars/internal/game"
)

// The three archetypes the Phase 5 exit gate asks the full game to survive:
// cautious trading with the whole rule set on, a hosting business run inside
// a trading one, and a risky import that prices the border into the trade.
// A police stop is part of the economy, not a special case: every full-game
// policy answers by rule — pay the disclosed fine — because an instrument
// that gambles cannot explain its own numbers.

// answerFull is the standing rule at a checkpoint: pay the fine.
func answerFull(r *game.Run) (game.Action, bool) {
	if r.State.PendingEncounter == nil {
		return game.Action{}, false
	}
	return game.Action{Kind: game.ActEncounter, Resp: game.EncPay}, true
}

// merchantFloat is the cash the trader refuses to touch: below it, one bad
// morning forces a fire sale, and the lender's schedule is not a plan.
const merchantFloat = game.Money(1000)

func relBid(r *game.Run, m content.MarketID, p content.ProductID) int64 {
	q, ok := r.QuoteOf(m, p)
	if !ok {
		return 0
	}
	return relQuote(q.Bid, m, p)
}

func relAsk(r *game.Run, m content.MarketID, p content.ProductID) int64 {
	q, ok := r.QuoteOf(m, p)
	if !ok {
		return 0
	}
	return relQuote(q.Ask, m, p)
}

// sellHereOrNever decides whether the standing town is where this card
// belongs today: a rush pays, the closing board liquidates, and a note at
// nine tenths of its ceiling has stopped being somebody else's problem.
func sellHereNow(r *game.Run, p content.ProductID) bool {
	if r.State.Day >= content.DayCount-1 || r.State.Debt > r.CreditCeiling()*9/10 {
		return true
	}
	return relBid(r, r.State.Market, p) >= sellRelMin
}

// repayFloat shrinks the note with everything above the working bankroll:
// a trader who repays into a wallet cannot buy the crash that pays the
// interest off, and the lender does not care which bad morning it is. The
// closing morning keeps nothing — the last float dollar goes to the note
// before the auctioneer takes it.
func repayFloat(r *game.Run) (game.Action, bool) {
	keep := merchantFloat
	switch {
	case r.State.Day >= content.DayCount-1:
		keep = 0
	case r.SlotsUsed() == 0:
		keep = game.Money(4000) // the bankroll a crash order is made of
	}
	if r.State.Debt <= 0 || r.State.Cash <= keep {
		return game.Action{}, false
	}
	amount := r.State.Cash - keep
	if amount > r.State.Debt {
		amount = r.State.Debt
	}
	if pv := r.PreviewRepay(amount); pv.Valid {
		return game.Action{Kind: game.ActRepay, Amount: amount}, true
	}
	return game.Action{}, false
}

// sellOwned dumps one product into the standing board: the closing morning
// does not pay a second price for holding.
func sellOwned(r *game.Run, p content.ProductID) game.Action {
	lim := r.SellLimits(p)
	return game.Action{Kind: game.ActSell, Product: p, Qty: lim.Max}
}

// crashBuyer picks the deepest discount on the standing board and sizes the
// order to cash, shelf stock, and slots in one honest preview pass. The
// skip set marks products the caller keeps for another purpose (a host's
// plant) — the merchant may not build a warehouse of them.
func crashBuyerSkip(r *game.Run, maxRel int64, skip func(content.ProductID) bool) (game.Action, bool) {
	var pick content.ProductID
	best := maxRel + 1
	for _, p := range r.Products() {
		if skip != nil && skip(p) {
			continue
		}
		if q, ok := r.QuoteOf(r.State.Market, p); ok && relQuote(q.Ask, r.State.Market, p) < best {
			best = relQuote(q.Ask, r.State.Market, p)
			pick = p
		}
	}
	if pick == "" || best > maxRel || r.State.Cash <= merchantFloat {
		return game.Action{}, false
	}
	q := r.QuoteHere(pick)
	if q.Ask <= 0 {
		return game.Action{}, false
	}
	qty := int((r.State.Cash - merchantFloat) / q.Ask)
	for ; qty > 0; qty-- {
		if pv := r.PreviewBuy(pick, qty); pv.Valid {
			return game.Action{Kind: game.ActBuy, Product: pick, Qty: qty}, true
		}
	}
	return game.Action{}, false
}

// discountTrip walks the run toward the deepest crash on the board when the
// standing town has nothing to buy and the fleet can pay its freight back:
// the crash must be deep enough that a tankful of units outweighs the mile.
func discountTrip(r *game.Run, maxRel int64, skip func(content.ProductID) bool) (game.Action, bool) {
	type cand struct {
		m   content.MarketID
		rel int64
	}
	var best cand
	best.rel = maxRel + 1
	for _, m := range r.Markets() {
		if m == r.State.Market {
			continue
		}
		for _, p := range r.Products() {
			if skip != nil && skip(p) {
				continue
			}
			if rl := relAsk(r, m, p); rl < best.rel {
				best = cand{m, rl}
			}
		}
	}
	if best.m == "" {
		return game.Action{}, false
	}
	pv := r.PreviewTravel(best.m)
	if !pv.Valid {
		return game.Action{}, false
	}
	// Units must outnumber the freight-to-discount leak: at 450-relative a
	// card is bought for about half of the town's mid, so every $100 of
	// freight needs two units in the pocket to still be worth the mile.
	cheap := game.Money(100)
	if r.State.Cash-merchantFloat < pv.Freight+cheap*2 {
		return game.Action{}, false
	}
	return game.Action{Kind: game.ActTravel, Market: best.m}, true
}

// merchant is the shared trader underneath all three archetypes: answer the
// border, sell into a rush, buy the deepest crash underfoot, otherwise walk
// toward one, and let nothing above the float sit idle while the note
// compounds. The garage is home because its meter hosts and its lease is $0.
func merchant(r *game.Run) game.Action { return merchantWith(r, nil) }

// merchantWith is merchant with a keep-set: products the rule may not sell
// before the closing board, used by the host to hold its plant.
func merchantWith(r *game.Run, keep func(content.ProductID) bool) game.Action {
	if a, ok := answerFull(r); ok {
		return a
	}
	closing := r.State.Day >= content.DayCount-1
	for _, p := range r.Products() {
		if r.OwnedOf(p) > 0 && (closing || keep == nil || !keep(p)) {
			if sellHereNow(r, p) {
				if lim := r.SellLimits(p); lim.Max > 0 {
					return sellOwned(r, p)
				}
			}
		}
	}
	if r.State.Day <= content.DayCount-3 {
		if a, ok := crashBuyerSkip(r, buyRelMax, keep); ok {
			return a
		}
	}
	// The note outranks the next trip: the lender charges every morning
	// whether the trader moved or not, so a dollar that cannot buy a crash
	// today buys a quieter tomorrow instead. The closing board settles in
	// full — cash on hand on the last morning is worth more than any
	// discount two towns over.
	if a, ok := repayFloat(r); ok {
		return a
	}
	if r.SlotsUsed() == 0 {
		// A crash worth buying underfoot is a crash worth driving to; the
		// only number that changes between those two sentences is the
		// freight.
		if a, ok := discountTrip(r, buyRelMax, keep); ok {
			return a
		}
	}
	// The way home comes before the note: a morning stranded on leased
	// ground costs more than the repayment it would have made.
	if r.State.Market != content.Garage {
		if pv := r.PreviewTravel(content.Garage); pv.Valid && r.State.Cash > pv.Freight {
			return game.Action{Kind: game.ActTravel, Market: content.Garage}
		}
	}
	if a, ok := repayFloat(r); ok {
		return a
	}
	return game.Action{Kind: game.ActStay}
}

// TradeRepayFull is the cautious archetype: the merchant rule with the whole
// board, every rule on, and a dollar that never bets bigger than the note.
var TradeRepayFull = Policy{Name: "trade-repay-full", Next: merchant}

// HostRun runs a small hosting business inside the trading one: three
// 4090-48 mods are the most the garage meter can feed, and the meter is why
// the plan is called hosting and not storage. The fleet is bought with the
// first week's trading profit — starting cash spent on racks is a run that
// starves on interest before the rent arrives.
var HostRun = Policy{Name: "host-run", Next: hostRun}

// hostFleet is the garage-meter dream: three mods, 180 of its 200 c-kW,
// bought whole or not at all.
const hostFleet = 3

// hostRun keeps the merchant on top, with a power plant underneath. The
// fleet waits until the note is under $2,500 — hosting nets about $260 a
// night and the lender takes ten percent of the debt every morning, so a
// plant bought over a big debt is a plant that was eaten by the interest
// on the way to the meter. From there it is a bond: three mods in the
// garage (zero lease, 200 c-kW meter), bought in Huaqiangbei where the
// wholesale tags live, held to the closing board, never sold to paper over
// a bad morning — that trade gives up the annuity for the panic.
func hostRun(r *game.Run) game.Action {
	if a, ok := answerFull(r); ok {
		return a
	}
	st := r.State
	mods := r.OwnedOf(content.Mod40948)
	settled := st.Debt <= game.Money(2500)
	building := settled && mods < hostFleet && st.Day >= 6 && st.Day <= 18
	switch {
	case building && st.Market == content.Huaqiangbei:
		if mods < hostFleet && relAsk(r, content.Huaqiangbei, content.Mod40948) <= 900 {
			// A plant is bought at wholesale prices or not at all: the
			// meter does not care what the tags said, and a rush-week
			// tag pays the broker's year of rent to the broker.
			pv := r.PreviewBuy(content.Mod40948, 1)
			if pv.Valid && st.Cash > pv.Cost+merchantFloat {
				return game.Action{Kind: game.ActBuy, Product: content.Mod40948, Qty: 1}
			}
		}
		if pv := r.PreviewTravel(content.Garage); pv.Valid && st.Cash > pv.Freight {
			return game.Action{Kind: game.ActTravel, Market: content.Garage}
		}
		return game.Action{Kind: game.ActStay}
	case building && st.Cash >= game.Money(9500):
		// Fleet money is fleet money: the merchant's crash can wait an
		// hour, the rent schedule cannot.
		if pv := r.PreviewTravel(content.Huaqiangbei); pv.Valid && st.Cash > pv.Freight+game.Money(6500) {
			return game.Action{Kind: game.ActTravel, Market: content.Huaqiangbei}
		}
	case mods > 0 && st.Market != content.Garage:
		// The plant earns only where the lease is free; every other
		// town's board is the merchant's errand, not the fleet's home.
		if pv := r.PreviewTravel(content.Garage); pv.Valid && st.Cash > pv.Freight {
			return game.Action{Kind: game.ActTravel, Market: content.Garage}
		}
	}
	return merchantWith(r, func(p content.ProductID) bool { return p == content.Mod40948 })
}

// RiskyImport is the smuggler's read: the wholesale town's board goes on
// sale more often than the enterprise town pays up, and the border fine —
// one fifth of the cargo's collateral, answered every time in cash, never
// as a gamble — is just the toll on the cheapest board on the map. The
// mule crosses only with the note small and the whole cycle funded, and
// only onto a board where the mod is genuinely cheap: the margin has to
// pay the toll, the freight, and the lender before it pays the smuggler.
// Past the border the cargo is ordinary merchant stock: loaded under half
// of town mid, sold into any town's rush, settled on the closing board.
var RiskyImport = Policy{Name: "risky-import", Next: riskyImport}

// importDebtMax is the note size below which smuggling is the merchant's
// own money at risk rather than the lender's auction in waiting.
const importDebtMax = game.Money(5000)

// importCashMin is freight both ways, a tankful of cards at a crash
// board, the worst-case fine, and a float that still buys breakfast at
// either end.
const importCashMin = game.Money(6500)

func riskyImport(r *game.Run) game.Action {
	if a, ok := answerFull(r); ok {
		return a
	}
	st := r.State
	mods := r.OwnedOf(content.Mod40948)
	switch {
	case st.Market == content.Huaqiangbei && st.Debt <= importDebtMax:
		// The smuggler's load line is a tenth deeper than the
		// merchant's: the border toll eats margin the merchant never
		// pays for, so a slightly less perfect board is still worth
		// filling — and the mule only crosses with a hold full enough
		// for the toll to be worth the risk.
		if a, ok := loadCargo(r, 700); ok {
			return a
		}
		if r.SlotsUsed() > 0 {
			if pv := r.PreviewTravel(content.Garage); pv.Valid && st.Cash > pv.Freight {
				return game.Action{Kind: game.ActTravel, Market: content.Garage}
			}
		}
	case r.SlotsUsed() > 0 && st.Market != content.Garage:
		// Cargo that cannot be sold where it stands goes home: the
		// garage lease is free and its meter pays for the wait.
		if !sellHereNow(r, content.Mod40948) || mods == 0 {
			if pv := r.PreviewTravel(content.Garage); pv.Valid && st.Cash > pv.Freight {
				return game.Action{Kind: game.ActTravel, Market: content.Garage}
			}
		}
	case r.SlotsUsed() == 0 && st.Day >= 3 && st.Day <= 20 &&
		st.Debt <= importDebtMax && st.Cash >= importCashMin && hqbHasCrash(r):
		// The border town is chosen even when a domestic board is
		// equal: that crossing is the whole point of the archetype,
		// and the toll is known before the first mile.
		if pv := r.PreviewTravel(content.Huaqiangbei); pv.Valid && st.Cash > pv.Freight+game.Money(4000) {
			return game.Action{Kind: game.ActTravel, Market: content.Huaqiangbei}
		}
	}
	// The merchant leg carries the rest: it buys the crash (mods included
	// — at the wholesale town that is the cargo), sells the rush, and
	// settles the note. The gamble the archetype's name promises is the
	// border crossing, and it is a priced gamble, not a blind one.
	return merchant(r)
}

// loadCargo fills the hold at the wholesale town: the deepest discount on
// its board, sized to the wallet and the slots, in honest preview steps.
func loadCargo(r *game.Run, maxRel int64) (game.Action, bool) {
	var pick content.ProductID
	best := maxRel + 1
	for _, p := range r.Products() {
		if q, ok := r.QuoteOf(r.State.Market, p); ok {
			if rl := relQuote(q.Ask, r.State.Market, p); rl < best {
				best = rl
				pick = p
			}
		}
	}
	if pick == "" || r.State.Cash <= merchantFloat+game.Money(500) {
		return game.Action{}, false
	}
	q := r.QuoteHere(pick)
	if q.Ask <= 0 {
		return game.Action{}, false
	}
	qty := int((r.State.Cash - merchantFloat) / q.Ask)
	for ; qty > 0; qty-- {
		if pv := r.PreviewBuy(pick, qty); pv.Valid {
			return game.Action{Kind: game.ActBuy, Product: pick, Qty: qty}, true
		}
	}
	return game.Action{}, false
}

// hqbHasCrash reads the wholesale town's board from wherever the run
// stands: something at or under seven tenths of its usual midpoint is
// worth a border, customs and all — the smuggler's line, not the
// merchant's half.
func hqbHasCrash(r *game.Run) bool {
	for _, p := range r.Products() {
		if rl := relAsk(r, content.Huaqiangbei, p); rl > 0 && rl <= 700 {
			return true
		}
	}
	return false
}

// AllFull is the full game's instrument set. It deliberately does not join
// All(): the prototype balance table must keep measuring exactly three rows.
func AllFull() []Policy { return []Policy{TradeRepayFull, HostRun, RiskyImport} }
