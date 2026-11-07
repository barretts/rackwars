package game

import (
	"testing"

	"rackwars/internal/content"
)

func newTestRun(t *testing.T, seed uint64) *Run {
	t.Helper()
	r, err := New(Config{
		Seed:     seed,
		Products: content.PrototypeProducts,
		Markets:  content.PrototypeMarkets,
		Rules:    content.PrototypeRules(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func mustApply(t *testing.T, r *Run, a Action) Receipt {
	t.Helper()
	rec, res := r.Apply(a)
	if !res.OK {
		t.Fatalf("apply %v refused: %s (%+v)", a, res.Reason, res)
	}
	return rec
}

// Mechanical check: "An untouched $20,000 note becomes $22,000 after one day."
// And the day-30 closing auction does not apply another month of interest.
func TestUntouchedNoteGrowsTenPercentPerDay(t *testing.T) {
	r := newTestRun(t, 1)
	if r.State.Debt != 20000 {
		t.Fatalf("opening debt = %s, want $20,000", FormatMoney(r.State.Debt))
	}
	if r.State.Day != 1 {
		t.Fatalf("opening day = %d, want 1", r.State.Day)
	}
	rec := mustApply(t, r, Action{Kind: ActStay})
	if rec.Interest != 2000 {
		t.Fatalf("first interest = %s, want $2,000", FormatMoney(rec.Interest))
	}
	if r.State.Debt != 22000 {
		t.Fatalf("debt after one day = %s, want $22,000", FormatMoney(r.State.Debt))
	}
	mustApply(t, r, Action{Kind: ActStay})
	if r.State.Debt != 24200 {
		t.Fatalf("debt after two days = %s, want $24,200", FormatMoney(r.State.Debt))
	}
	if r.State.Stats.InterestPaid != 4200 {
		t.Fatalf("interest paid = %s, want $4,200", FormatMoney(r.State.Stats.InterestPaid))
	}
}

func TestInterestRoundsUpToNextDollar(t *testing.T) {
	for _, tc := range []struct {
		debt, want Money
	}{{0, 0}, {1, 1}, {10, 1}, {20000, 2000}, {20001, 2001}, {20009, 2001}} {
		if got := ceil10(tc.debt); got != tc.want {
			t.Errorf("ceil10(%s) = %s, want %s", FormatMoney(tc.debt), FormatMoney(got), FormatMoney(tc.want))
		}
	}
}

// Mechanical check: "0 < bid < ask" for every product, town, and day in the
// 30-day window, so an accidental free-money cycle cannot exist.
func TestQuotesAlwaysKeepSpread(t *testing.T) {
	r, err := New(Config{Seed: 77, Products: content.AllProducts, Markets: content.AllMarkets, Rules: content.FullRules()})
	if err != nil {
		t.Fatal(err)
	}
	for day := 1; day <= content.DayCount; day++ {
		for _, m := range content.PrototypeMarkets {
			for _, p := range content.AllProducts {
				q := r.quoteFor(day, m, p)
				if !(q.Bid > 0 && q.Bid < q.Ask) {
					t.Fatalf("day %d %s %s: bid %s ask %s violates 0 < bid < ask",
						day, m, p, FormatMoney(q.Bid), FormatMoney(q.Ask))
				}
			}
		}
	}
}

// Same-town round trips always lose money.
func TestSameTownRoundTripAlwaysLoses(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3, 4, 5} {
		r := newTestRun(t, seed)
		for _, p := range r.cfg.Products {
			lim := r.BuyLimits(p)
			if lim.Max < 1 {
				continue
			}
			before := r.State.Cash
			mustApply(t, r, Action{Kind: ActBuy, Product: p, Qty: 1})
			mustApply(t, r, Action{Kind: ActSell, Product: p, Qty: 1})
			if r.State.Cash >= before {
				t.Fatalf("seed %d %s: round trip cash %s -> %s did not lose money",
					seed, p, FormatMoney(before), FormatMoney(r.State.Cash))
			}
		}
	}
}

// Mechanical check: buying, selling, borrowing, and repaying never change the
// day counter.
func TestMorningActionsCostNoDay(t *testing.T) {
	r := newTestRun(t, 3)
	start := r.State.Day
	p := content.Card4090
	mustApply(t, r, Action{Kind: ActBorrow, Amount: 5000})
	mustApply(t, r, Action{Kind: ActBuy, Product: p, Qty: 1})
	mustApply(t, r, Action{Kind: ActSell, Product: p, Qty: 1})
	mustApply(t, r, Action{Kind: ActRepay, Amount: 1000})
	if r.State.Day != start {
		t.Fatalf("day moved %d -> %d on morning actions", start, r.State.Day)
	}
	// Travel and stay each consume exactly one day.
	mustApply(t, r, Action{Kind: ActTravel, Market: content.Ashburn})
	if r.State.Day != start+1 {
		t.Fatalf("travel advanced day by %d, want 1", r.State.Day-start)
	}
	mustApply(t, r, Action{Kind: ActStay})
	if r.State.Day != start+2 {
		t.Fatalf("stay advanced day by %d, want 1", r.State.Day-start-1)
	}
}

// Mechanical check of the owner rule: the fleet is never fenced by a site.
// A 16-slot fleet moves to the 8-slot garage, freight bills the whole
// footprint, and it is hosting — not arrival — that the small site bounds.
func TestOversizedFleetStillTravels(t *testing.T) {
	r := newTestRun(t, 5)
	mustApply(t, r, Action{Kind: ActTravel, Market: content.WestTexas}) // 24 slots there
	mustApply(t, r, Action{Kind: ActBorrow, Amount: r.AvailableCredit()})
	// Fill 16 slots with 3090 sleds (2 slots each) — the whole point is a
	// fleet that no smaller site could ever host.
	lim := r.BuyLimits(content.Sled3090)
	if lim.Max < 8 {
		t.Fatalf("could not reach 16 fleet slots, max sled buy %d (limits %+v)", lim.Max, lim)
	}
	mustApply(t, r, Action{Kind: ActBuy, Product: content.Sled3090, Qty: 8})
	if r.SlotsUsed() != 16 {
		t.Fatalf("slots used = %d, want 16", r.SlotsUsed())
	}
	pv := r.PreviewTravel(content.Garage)
	if !pv.Valid {
		t.Fatalf("garage travel preview = %+v, want valid", pv)
	}
	if pv.FleetSlots != 16 || pv.HostCap != content.MarketOf(content.Garage).Slots {
		t.Fatalf("preview footprint = %d fleet, %d host cap; want 16, %d",
			pv.FleetSlots, pv.HostCap, content.MarketOf(content.Garage).Slots)
	}
	wantFreight := pv.RouteFee + Money(16*content.PerSlotFreight)
	if pv.Freight != wantFreight {
		t.Fatalf("freight = %s, want route %s + 16 slots at %d = %s",
			FormatMoney(pv.Freight), FormatMoney(pv.RouteFee), int(content.PerSlotFreight), FormatMoney(wantFreight))
	}
	mustApply(t, r, Action{Kind: ActTravel, Market: content.Garage})
	if r.State.Market != content.Garage {
		t.Fatalf("oversized move did not land: market %s", r.State.Market)
	}
}

// Freight = route fee + per-slot charge.
func TestFreightIsRouteFeePlusPerSlot(t *testing.T) {
	r := newTestRun(t, 6)
	pv := r.PreviewTravel(content.Ashburn)
	if pv.RouteFee != 250 || pv.PerSlotCharge != 0 || pv.Freight != 250 {
		t.Fatalf("empty freight = %+v, want route $250 + 0", pv)
	}
	mustApply(t, r, Action{Kind: ActBorrow, Amount: 6000})
	mustApply(t, r, Action{Kind: ActBuy, Product: content.Sled3090, Qty: 2}) // 4 slots
	pv = r.PreviewTravel(content.Ashburn)
	if pv.PerSlotCharge != 200 || pv.Freight != 450 {
		t.Fatalf("freight with 4 slots = %+v, want $250 + $200 = $450", pv)
	}
	cashBefore := r.State.Cash
	rec := mustApply(t, r, Action{Kind: ActTravel, Market: content.Ashburn})
	if rec.Freight != 450 || r.State.Cash != cashBefore-450 {
		t.Fatalf("charged freight %s, cash %s; want 450 and %s",
			FormatMoney(rec.Freight), FormatMoney(r.State.Cash), FormatMoney(cashBefore-450))
	}
}

// Mechanical check: stock does not refresh while the player is acting, so
// buying out a shop and waiting in place cannot farm it.
func TestStockDoesNotRefreshDuringActions(t *testing.T) {
	r := newTestRun(t, 9)
	r.State.Cash = 1000000 // isolate the stock rule from the cash rule
	// The H100 shelf in a garage is deliberately thin, so shelf stock binds
	// long before the million in cash runs out.
	p := content.CardH100
	stock := r.StockOf(p)
	if stock <= 0 {
		t.Fatalf("day 1 %s stock = %d, want scarce but nonzero shelf stock", p, stock)
	}
	if lim := r.BuyLimits(p); lim.Max != stock {
		t.Fatalf("buy limit = %+v, want shelf stock %d to bind", lim, stock)
	}
	mustApply(t, r, Action{Kind: ActBuy, Product: p, Qty: stock})
	if r.StockOf(p) != 0 {
		t.Fatalf("stock = %d after buying out the shelf", r.StockOf(p))
	}
	if _, res := r.Apply(Action{Kind: ActBuy, Product: p, Qty: 1}); res.Reason != ReasonInsufficientStock {
		t.Fatalf("buying past shelf stock gave %+v, want insufficient stock", res)
	}
	// Repeated morning actions leave the shelf alone.
	for range 3 {
		mustApply(t, r, Action{Kind: ActRepay, Amount: 1})
	}
	if r.StockOf(p) != 0 {
		t.Fatalf("shelf restocked during morning actions: %d", r.StockOf(p))
	}
	// Only advancing the day replenishes it, and only through the engine.
	mustApply(t, r, Action{Kind: ActStay})
	if got := r.stockFor(r.State.Day, r.State.Market, p); got != r.StockOf(p) {
		t.Fatalf("day %d shelf = %d, engine says %d", r.State.Day, r.StockOf(p), got)
	}
}

// Mechanical check: loans are bounded by collateral and a borrow/buy/resell
// loop cannot create net worth.
func TestBorrowIsBoundedByCollateral(t *testing.T) {
	r := newTestRun(t, 11)
	if got := r.CreditCeiling(); got != 30000 {
		t.Fatalf("ceiling with no GPUs = %s, want $30,000", FormatMoney(got))
	}
	if got := r.AvailableCredit(); got != 10000 {
		t.Fatalf("available credit = %s, want $10,000", FormatMoney(got))
	}
	if _, res := r.Apply(Action{Kind: ActBorrow, Amount: 10001}); res.OK {
		t.Fatal("borrowed past the unsecured floor")
	} else if res.Reason != ReasonInsufficientCredit {
		t.Fatalf("reason = %s, want insufficient credit", res.Reason)
	} else if res.MaxAmount != 10000 {
		t.Fatalf("engine offered max %s, want $10,000", FormatMoney(res.MaxAmount))
	}
	mustApply(t, r, Action{Kind: ActBorrow, Amount: 10000})
	if got := r.AvailableCredit(); got != 0 {
		t.Fatalf("available after borrowing the floor = %s, want $0", FormatMoney(got))
	}

	// Collateral raises the ceiling: $30,000 + 50% of current local bid value.
	bid := r.QuoteHere(content.Card4090).Bid
	mustApply(t, r, Action{Kind: ActBuy, Product: content.Card4090, Qty: 2})
	want := Money(30000) + bid*2*CollateralLoanRatePct/100
	if got := r.CreditCeiling(); got != want {
		t.Fatalf("ceiling = %s, want %s", FormatMoney(got), FormatMoney(want))
	}
	if got := r.AvailableCredit(); got != bid {
		t.Fatalf("available = %s, want %s (2 x 50%% of bid %s)", FormatMoney(got), FormatMoney(bid), FormatMoney(bid))
	}

	// A borrow -> buy -> resell loop must lose money, never print it: the ask
	// is always above the bid, and the loan adds debt, not wealth.
	before := r.NetWorth()
	borrow := r.AvailableCredit()
	mustApply(t, r, Action{Kind: ActBorrow, Amount: borrow})
	lim := r.BuyLimits(content.Card4090)
	if lim.Max == 0 {
		t.Fatal("no room to test the loop")
	}
	mustApply(t, r, Action{Kind: ActBuy, Product: content.Card4090, Qty: lim.Max})
	mustApply(t, r, Action{Kind: ActSell, Product: content.Card4090, Qty: lim.Max})
	if r.NetWorth() >= before {
		t.Fatalf("borrow/buy/resell moved net worth %s -> %s", FormatMoney(before), FormatMoney(r.NetWorth()))
	}
}

// Mechanical check: a morning repricing that breaches the collateral floor
// sells collateral before the player can act, highest bid per unit first.
//
// Design worked example: two H100s, bids fall from $40,000 to $16,000.
// Shoreline sells both units, leaving $28,000 debt and $0 cash.
func TestForcedSaleWorkedExample(t *testing.T) {
	r := newTestRun(t, 13)
	mi := r.MarketIndex(content.Garage)
	pi := r.Index(content.CardH100)
	// Force the documented board: bids at $16,000 after the crash.
	r.State.Quotes[mi*len(r.cfg.Products)+pi] = Quote{Ask: 17600, Bid: 16000}
	r.State.Owned[pi] = 2
	r.State.CostBasis[pi] = 80000 // bought at $40,000 each
	r.State.Cash = 0
	r.State.Debt = 60000

	if got := r.CollateralValue(); got != 32000 {
		t.Fatalf("collateral = %s, want $32,000", FormatMoney(got))
	}
	if got := r.CreditCeiling(); got != 46000 {
		t.Fatalf("ceiling = %s, want $46,000", FormatMoney(got))
	}
	if got := r.BreachAmount(); got != 14000 {
		t.Fatalf("breach = %s, want $14,000", FormatMoney(got))
	}

	rep := r.runCreditCheck()
	if !rep.Triggered {
		t.Fatal("credit check did not trigger")
	}
	if len(rep.Sales) != 2 {
		t.Fatalf("forced sales = %d (%+v), want 2", len(rep.Sales), rep.Sales)
	}
	for _, s := range rep.Sales {
		if s.Product != content.CardH100 || s.UnitBid != 16000 || s.Units != 1 {
			t.Errorf("forced sale = %+v, want one H100 at $16,000", s)
		}
		if s.LossVsCost != 24000 {
			t.Errorf("loss vs cost = %s, want $24,000 (avg $40,000 vs bid $16,000)", FormatMoney(s.LossVsCost))
		}
	}
	if r.State.Debt != 28000 {
		t.Fatalf("debt after cure = %s, want $28,000", FormatMoney(r.State.Debt))
	}
	if r.State.Cash != 0 {
		t.Fatalf("cash after cure = %s, want $0", FormatMoney(r.State.Cash))
	}
	if r.OwnedOf(content.CardH100) != 0 {
		t.Fatalf("H100s left = %d, want 0", r.OwnedOf(content.CardH100))
	}
	if rep.Repossessed {
		t.Fatal("run ended after a curable breach")
	}
	if r.State.Stats.ForcedLoss != 48000 {
		t.Fatalf("recorded forced loss = %s, want $48,000", FormatMoney(r.State.Stats.ForcedLoss))
	}
}

// Cash cures before GPUs are sold.
func TestCashCureHappensBeforeForcedSale(t *testing.T) {
	r := newTestRun(t, 17)
	pi := r.Index(content.CardH100)
	mi := r.MarketIndex(content.Garage)
	r.State.Quotes[mi*len(r.cfg.Products)+pi] = Quote{Ask: 17600, Bid: 16000}
	r.State.Owned[pi] = 1
	r.State.CostBasis[pi] = 40000
	r.State.Debt = 50000 // ceiling is $38,000 with one H100 at $16,000
	r.State.Cash = 12000 // exactly the breach

	rep := r.runCreditCheck()
	if rep.CashUsed != 12000 {
		t.Fatalf("cash used = %s, want $12,000", FormatMoney(rep.CashUsed))
	}
	if len(rep.Sales) != 0 {
		t.Fatalf("forced sales = %+v, want none after a cash cure", rep.Sales)
	}
	if r.State.Debt != 38000 {
		t.Fatalf("debt = %s, want $38,000", FormatMoney(r.State.Debt))
	}
}

// Repossession needs all three: no cash, no GPUs, and a balance still above
// the unsecured floor. Negative net worth on its own must not end a run.
func TestRepossessionOnlyWhenNothingLeft(t *testing.T) {
	r := newTestRun(t, 19)
	r.State.Cash = 0
	r.State.Debt = 45000
	rep := r.runCreditCheck()
	if !rep.Triggered || !rep.Repossessed || len(rep.Sales) != 0 {
		t.Fatalf("empty rack with $45,000 owed: %+v", rep)
	}

	r2 := newTestRun(t, 20)
	r2.State.Cash = 0
	r2.State.Debt = 60000
	if r2.NetWorth() >= 0 {
		t.Fatal("setup should have negative net worth")
	}
	if _, res := r2.Apply(Action{Kind: ActStay}); !res.OK {
		t.Fatalf("negative net worth blocked a normal action: %+v", res)
	}
}

// Morning sales route to Shoreline first when the sale reduces the collateral
// below the loan.
func TestSaleProceedsRouteToLenderFirst(t *testing.T) {
	r := newTestRun(t, 23)
	pi := r.Index(content.CardH100)
	mi := r.MarketIndex(content.Garage)
	bid := Money(20000)
	r.State.Quotes[mi*len(r.cfg.Products)+pi] = Quote{Ask: 22000, Bid: bid}
	r.State.Owned[pi] = 1
	r.State.CostBasis[pi] = 30000
	r.State.Debt = 40000 // ceiling = 30,000 + 10,000 = 40,000, exactly at it
	pv := r.PreviewSell(content.CardH100, 1)
	if !pv.Valid {
		t.Fatalf("sell preview invalid: %+v", pv)
	}
	if pv.Gross != bid || pv.CeilingAfter != 30000 || pv.ToLender != 10000 || pv.CashIn != 10000 {
		t.Fatalf("sale split = %+v, want gross $20,000, to lender $10,000, to player $10,000", pv)
	}
	cashBefore, debtBefore := r.State.Cash, r.State.Debt
	rec := mustApply(t, r, Action{Kind: ActSell, Product: content.CardH100, Qty: 1})
	if rec.ToLender != 10000 || rec.CashIn != 10000 {
		t.Fatalf("receipt split = %+v", rec)
	}
	if r.State.Cash != cashBefore+10000 || r.State.Debt != debtBefore-10000 {
		t.Fatalf("state after sale: cash %s debt %s", FormatMoney(r.State.Cash), FormatMoney(r.State.Debt))
	}
	// The preview and the applied action are the same numbers.
	if pv.CashAfter != r.State.Cash || pv.DebtAfter != r.State.Debt {
		t.Fatalf("preview (%s/%s) != applied (%s/%s)", FormatMoney(pv.CashAfter), FormatMoney(pv.DebtAfter),
			FormatMoney(r.State.Cash), FormatMoney(r.State.Debt))
	}
}

// Refusals are data, not prose: the engine names the gap in the player's own
// numbers so the TUI never invents an explanation.
func TestRefusalsCarryExactGaps(t *testing.T) {
	r := newTestRun(t, 29)
	p := content.CardH100
	q := r.QuoteHere(p)
	// Not enough cash.
	r.State.Cash = q.Ask - 1
	_, res := r.Apply(Action{Kind: ActBuy, Product: p, Qty: 1})
	if res.Reason != ReasonInsufficientCash || res.ShortCash != 1 {
		t.Fatalf("cash refusal = %+v, want insufficient cash short by $1", res)
	}
	// Not enough owned.
	r.State.Owned[r.Index(p)] = 0
	if _, res := r.Apply(Action{Kind: ActSell, Product: p, Qty: 1}); res.Reason != ReasonInsufficientOwned {
		t.Fatalf("sell refusal = %+v, want insufficient owned", res)
	}
	// Bad quantity.
	for _, qty := range []int{0, -3} {
		if _, res := r.Apply(Action{Kind: ActBuy, Product: p, Qty: qty}); res.Reason != ReasonBadQuantity {
			t.Errorf("qty %d refusal = %+v, want bad quantity", qty, res)
		}
	}
	// Unknown product.
	if _, res := r.Apply(Action{Kind: ActBuy, Product: content.ProductID("4090-48"), Qty: 1}); res.Reason != ReasonUnknownProduct {
		t.Errorf("cast product refusal = %+v, want unknown product", res)
	}
	// Travel to the town already standing in.
	if _, res := r.Apply(Action{Kind: ActTravel, Market: r.State.Market}); res.Reason != ReasonNoRoute {
		t.Errorf("same-town travel refusal = %+v, want no route", res)
	}
}

// Mechanical check: the 30-day campaign ends with a closing auction and no
// day-31 interest, event, or hidden reroll.
func TestDay30ClosesTheRun(t *testing.T) {
	r := newTestRun(t, 31)
	// Keep the note inside the ceiling so the run survives to day 30; the
	// spiral that ends a passive run is covered by
	// TestUntouchedNoteEventuallyEndsInRepossession.
	r.State.Debt = 5000
	r.State.Cash = 100000
	for d := 1; d < content.DayCount; d++ {
		if _, res := r.Apply(Action{Kind: ActStay}); !res.OK {
			t.Fatalf("day %d stay refused: %+v", d, res)
		}
		if r.State.Over != EndOpen {
			t.Fatalf("run ended early on day %d: %+v", r.State.Day, r.State.End)
		}
	}
	if r.State.Day != content.DayCount {
		t.Fatalf("day = %d, want %d", r.State.Day, content.DayCount)
	}
	// Hold something: the auction must liquidate it at the posted bid.
	r.State.Owned[r.Index(content.Card4090)] = 2
	r.State.CostBasis[r.Index(content.Card4090)] = 3000
	debtBefore := r.State.Debt
	cashBefore := r.State.Cash
	bid := r.QuoteHere(content.Card4090).Bid
	wantInterest := ceil10(debtBefore)
	rec := mustApply(t, r, Action{Kind: ActStay})
	if !rec.Closed {
		t.Fatalf("day 30 receipt not a closing auction: %+v", rec)
	}
	wantAuction := bid * 2
	// The day spent accrues its interest; nothing applies after the auction.
	wantCashOut := cashBefore - rec.Lease + wantAuction - debtBefore - wantInterest
	if rec.Gross != wantAuction {
		t.Fatalf("auction gross = %s, want %s", FormatMoney(rec.Gross), FormatMoney(wantAuction))
	}
	if rec.Interest != wantInterest {
		t.Fatalf("closing day interest = %s, want %s", FormatMoney(rec.Interest), FormatMoney(wantInterest))
	}
	if rec.Freight != 0 || rec.Lease != 0 {
		t.Fatalf("prototype closing charged lease %s freight %s, want none",
			FormatMoney(rec.Lease), FormatMoney(rec.Freight))
	}
	if r.State.End.CashOut != wantCashOut {
		t.Fatalf("cash-out = %s, want %s", FormatMoney(r.State.End.CashOut), FormatMoney(Money(wantCashOut)))
	}
	// A finished run reads as a settled ledger: the note is paid out of the sale
	// proceeds and the player is holding what is left. Debt never goes negative,
	// which would otherwise show up in the ending screen as the lender owing the
	// player money.
	if r.State.Debt != 0 {
		t.Fatalf("debt after closing = %s, want settled to zero", FormatMoney(r.State.Debt))
	}
	if r.State.Cash != wantCashOut {
		t.Fatalf("cash after closing = %s, want the cash-out %s", FormatMoney(r.State.Cash), FormatMoney(Money(wantCashOut)))
	}
	if r.State.Over != EndClosed {
		t.Fatalf("over = %v, want closed", r.State.Over)
	}
	if r.State.Day != content.DayCount {
		t.Fatalf("day moved to %d; there is no day 31", r.State.Day)
	}
	if r.OwnedOf(content.Card4090) != 0 || r.CollateralValue() != 0 {
		t.Fatal("auction left inventory")
	}
	if _, res := r.Apply(Action{Kind: ActStay}); res.Reason != ReasonRunOver {
		t.Fatalf("action after close = %+v, want run over", res)
	}
	if _, res := r.Apply(Action{Kind: ActBuy, Product: content.Card4090, Qty: 1}); res.Reason != ReasonRunOver {
		t.Fatalf("buy after close = %+v, want run over", res)
	}
}

// A player who never trades and never buys collateral runs out of cash cure
// money and loses the rack. The design calls this the expected death spiral, so
// the engine has to end it, not let the balance fall forever.
func TestUntouchedNoteEventuallyEndsInRepossession(t *testing.T) {
	r := newTestRun(t, 41)
	for range content.DayCount {
		if _, res := r.Apply(Action{Kind: ActStay}); !res.OK {
			break
		}
		if r.State.Over != EndOpen {
			break
		}
	}
	if r.State.Over != EndRepossessed {
		t.Fatalf("passive run ended as %v (%+v), want repossession", r.State.Over, r.State.End)
	}
	if r.State.End.Rank != "Shoreline's new subsidiary" {
		t.Fatalf("rank = %q", r.State.End.Rank)
	}
	if r.State.End.Deficiency <= 30000 {
		t.Fatalf("deficiency = %s, want above the $30,000 unsecured floor", FormatMoney(r.State.End.Deficiency))
	}
	if r.State.Cash != 0 || r.CollateralValue() != 0 {
		t.Fatalf("repossession left cash %s collateral %s", FormatMoney(r.State.Cash), FormatMoney(r.CollateralValue()))
	}
	if r.State.End.Epitaph == "" {
		t.Fatal("repossession has no epitaph")
	}
}

func TestRankTiers(t *testing.T) {
	for _, tc := range []struct {
		v    Money
		want string
	}{{-1, "Shoreline's new subsidiary"}, {0, "Walked away"}, {1, "Still independent"},
		{99999, "Still independent"}, {100000, "Garage graduate"}, {999999, "Garage graduate"},
		{1000000, "Regional problem"}, {9999999, "Regional problem"}, {10000000, "Neocloud"}} {
		if got := RankFor(tc.v); got != tc.want {
			t.Errorf("RankFor(%s) = %q, want %q", FormatMoney(tc.v), got, tc.want)
		}
	}
}
