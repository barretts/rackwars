package game

import (
	"rackwars/internal/content"
)

// Reason explains why an action is impossible. The engine always supplies it,
// so the TUI never invents an explanation and never lets the player discover
// the rule by failing.
type Reason int

const (
	ReasonOK Reason = iota
	ReasonRunOver
	ReasonBadQuantity
	ReasonUnknownProduct
	ReasonOvercapacity
	ReasonInsufficientStock
	ReasonInsufficientCash
	ReasonInsufficientOwned
	ReasonInsufficientCredit
	ReasonNoRoute
	ReasonNotForSale
	ReasonCurtailUnavailable
	ReasonEncounterPending
	ReasonNoEncounter
	ReasonPowerOver
	ReasonAllocationBlocked
)

func (re Reason) String() string {
	switch re {
	case ReasonOK:
		return "ok"
	case ReasonRunOver:
		return "run over"
	case ReasonBadQuantity:
		return "bad quantity"
	case ReasonUnknownProduct:
		return "unknown product"
	case ReasonOvercapacity:
		return "over capacity"
	case ReasonInsufficientStock:
		return "insufficient stock"
	case ReasonInsufficientCash:
		return "insufficient cash"
	case ReasonInsufficientOwned:
		return "not enough owned"
	case ReasonInsufficientCredit:
		return "no credit left"
	case ReasonNoRoute:
		return "no such route"
	case ReasonNotForSale:
		return "not for sale here"
	case ReasonCurtailUnavailable:
		return "no curtailment offer open here"
	case ReasonEncounterPending:
		return "answering a police beat first"
	case ReasonNoEncounter:
		return "nothing is pending"
	case ReasonPowerOver:
		return "over the site's meter"
	case ReasonAllocationBlocked:
		return "allocation refused"
	}
	return "refused"
}

// QuantityLimits is the most the engine will accept right now, and which
// constraint binds. This is the single source of truth for the quantity box:
// the TUI must not compute its own clamp.
type QuantityLimits struct {
	Max     int    `json:"max"`
	LimitBy Reason `json:"limitBy"`
}

// BuyLimits returns the largest purchasable quantity of a card here: the
// intersection of shop stock, free slots, and cash.
func (r *Run) BuyLimits(p content.ProductID) QuantityLimits {
	pi := r.Index(p)
	if pi < 0 || r.State.Over != EndOpen {
		return QuantityLimits{Max: 0, LimitBy: ReasonRunOver}
	}
	perUnit := content.ProductOf(p).SlotsPerUnit
	ask := r.QuoteHere(p).Ask

	fromStock := r.State.Stock[pi]
	fromSlots := 0
	if perUnit > 0 {
		fromSlots = r.SlotsAvailable() / perUnit
	}
	fromCash := 0
	if ask > 0 {
		fromCash = int(r.State.Cash / ask)
	}

	max := fromStock
	blocked := ReasonInsufficientStock
	if fromSlots < max {
		max, blocked = fromSlots, ReasonOvercapacity
	}
	if fromCash < max {
		max, blocked = fromCash, ReasonInsufficientCash
	}
	if max < 0 {
		max = 0
		blocked = ReasonOvercapacity
	}
	return QuantityLimits{Max: max, LimitBy: blocked}
}

// SellLimits returns the largest sellable quantity: everything held, since the
// town buys any owned quantity at the posted bid in this release.
func (r *Run) SellLimits(p content.ProductID) QuantityLimits {
	pi := r.Index(p)
	if pi < 0 || r.State.Over != EndOpen {
		return QuantityLimits{Max: 0, LimitBy: ReasonRunOver}
	}
	if r.State.Owned[pi] <= 0 {
		return QuantityLimits{Max: 0, LimitBy: ReasonInsufficientOwned}
	}
	return QuantityLimits{Max: r.State.Owned[pi], LimitBy: ReasonOK}
}

// PreviewBuy is the confirmation screen: cost, cash and capacity afterward.
type PreviewBuy struct {
	Product    content.ProductID `json:"product"`
	Qty        int               `json:"qty"`
	UnitAsk    Money             `json:"unitAsk"`
	Cost       Money             `json:"cost"`
	CashBefore Money             `json:"cashBefore"`
	CashAfter  Money             `json:"cashAfter"`
	SlotsUsed  int               `json:"slotsUsed"`
	SlotsAfter int               `json:"slotsAfter"`
	SlotsTotal int               `json:"slotsTotal"`
	StockLeft  int               `json:"stockLeft"`
	HeatAfter  int               `json:"heatAfter"`
	Valid      bool              `json:"valid"`
	Reason     Reason            `json:"reason"`
	ShortBy    Money             `json:"shortBy"`
	SlotsShort int               `json:"slotsShort"`
	StockShort int               `json:"stockShort"`
}

// PreviewBuy shows what a purchase of qty units would do, without changing
// anything. A preview and the applied action must agree exactly, so both use
// these same numbers.
func (r *Run) PreviewBuy(p content.ProductID, qty int) PreviewBuy {
	pi := r.Index(p)
	pv := PreviewBuy{Product: p, Qty: qty, CashBefore: r.State.Cash, Reason: ReasonRunOver}
	if pi < 0 {
		pv.Reason = ReasonUnknownProduct
		return pv
	}
	if r.State.Over != EndOpen {
		return pv
	}
	def := content.ProductOf(p)
	q := r.QuoteHere(p)
	pv.UnitAsk = q.Ask
	pv.Cost = q.Ask * Money(qty)
	pv.CashAfter = r.State.Cash - pv.Cost
	pv.SlotsUsed = r.SlotsUsed()
	pv.SlotsTotal = content.MarketOf(r.State.Market).Slots
	pv.SlotsAfter = pv.SlotsUsed + qty*def.SlotsPerUnit
	pv.StockLeft = r.State.Stock[pi] - qty
	pv.HeatAfter = r.State.Heat

	if qty <= 0 {
		pv.Reason = ReasonBadQuantity
		return pv
	}
	if r.cfg.Rules.Heat && p == content.Mod40948 {
		pv.HeatAfter = clampInt(pv.HeatAfter+10*qty, 0, 100)
	}
	// Report every constraint that binds, so the message can name the money,
	// the slots, or the stock as appropriate rather than guessing. Cash is
	// checked first: an unaffordable purchase is the most common refusal and
	// the player needs the exact shortfall.
	if pv.Cost > r.State.Cash {
		pv.Reason = ReasonInsufficientCash
		pv.ShortBy = pv.Cost - r.State.Cash
		return pv
	}
	if pv.SlotsAfter > pv.SlotsTotal {
		pv.Reason = ReasonOvercapacity
		pv.SlotsShort = pv.SlotsAfter - pv.SlotsTotal
		return pv
	}
	if qty > r.State.Stock[pi] {
		pv.Reason = ReasonInsufficientStock
		pv.StockShort = qty - r.State.Stock[pi]
		return pv
	}
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// PreviewSell is the sale confirmation. Selling reduces collateral and can
// therefore reduce the credit ceiling, so the proceeds split between Shoreline
// and the player is shown before the player confirms.
type PreviewSell struct {
	Product          content.ProductID `json:"product"`
	Qty              int               `json:"qty"`
	UnitBid          Money             `json:"unitBid"`
	Gross            Money             `json:"gross"`
	ToLender         Money             `json:"toLender"`
	CashIn           Money             `json:"cashIn"`
	CashBefore       Money             `json:"cashBefore"`
	CashAfter        Money             `json:"cashAfter"`
	DebtBefore       Money             `json:"debtBefore"`
	DebtAfter        Money             `json:"debtAfter"`
	CollateralBefore Money             `json:"collateralBefore"`
	CollateralAfter  Money             `json:"collateralAfter"`
	CeilingAfter     Money             `json:"ceilingAfter"`
	Realized         Money             `json:"realized"`
	Valid            bool              `json:"valid"`
	Reason           Reason            `json:"reason"`
}

// PreviewSell computes the lender-first split: the sale releases enough cash to
// bring the loan back inside the ceiling recalculated on the smaller collateral
// base, and the remainder is the player's.
func (r *Run) PreviewSell(p content.ProductID, qty int) PreviewSell {
	pi := r.Index(p)
	pv := PreviewSell{Product: p, Qty: qty, CashBefore: r.State.Cash, DebtBefore: r.State.Debt, Reason: ReasonRunOver}
	if pi < 0 {
		pv.Reason = ReasonUnknownProduct
		return pv
	}
	if r.State.Over != EndOpen {
		return pv
	}
	bid := r.QuoteHere(p).Bid
	pv.UnitBid = bid
	pv.Gross = bid * Money(qty)
	pv.CollateralBefore = r.CollateralValue()
	pv.Realized = (bid - r.AvgCost(p)) * Money(qty)

	if qty <= 0 {
		pv.Reason = ReasonBadQuantity
		return pv
	}
	if qty > r.State.Owned[pi] {
		pv.Reason = ReasonInsufficientOwned
		return pv
	}

	// Collateral after the sale, at the same posted bids.
	mi := r.MarketIndex(r.State.Market)
	collateral := Money(0)
	for j := range r.cfg.Products {
		held := r.State.Owned[j]
		if j == pi {
			held -= qty
		}
		collateral += Money(held) * r.quoteAt(mi, j).Bid
	}
	pv.CollateralAfter = collateral
	ceiling := Money(UnsecuredFloor) + collateral*CollateralLoanRatePct/100
	pv.CeilingAfter = ceiling

	required := r.State.Debt - ceiling
	if required < 0 {
		required = 0
	}
	pv.ToLender = pv.Gross
	if pv.ToLender > required {
		pv.ToLender = required
	}
	pv.CashIn = pv.Gross - pv.ToLender
	pv.DebtAfter = r.State.Debt - pv.ToLender
	pv.CashAfter = r.State.Cash + pv.CashIn
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

// PreviewBorrow and PreviewRepay drive the lender screen.
type PreviewLoan struct {
	Amount           Money  `json:"amount"`
	CashBefore       Money  `json:"cashBefore"`
	CashAfter        Money  `json:"cashAfter"`
	DebtBefore       Money  `json:"debtBefore"`
	DebtAfter        Money  `json:"debtAfter"`
	Ceiling          Money  `json:"ceiling"`
	Available        Money  `json:"available"`
	InterestTomorrow Money  `json:"interestTomorrow"`
	Valid            bool   `json:"valid"`
	Reason           Reason `json:"reason"`
}

// PreviewBorrow bounds a loan by the ceiling recalculated on current collateral.
// A borrow/buy/resell loop therefore cannot create net worth: the ask is always
// above the bid, and every unit bought is collateral valued at the bid.
func (r *Run) PreviewBorrow(amount Money) PreviewLoan {
	pv := PreviewLoan{Amount: amount, CashBefore: r.State.Cash, DebtBefore: r.State.Debt,
		Ceiling: r.CreditCeiling(), Available: r.AvailableCredit(), InterestTomorrow: ceil10(r.State.Debt + amount),
		Reason: ReasonRunOver}
	if r.State.Over != EndOpen {
		return pv
	}
	if amount <= 0 {
		pv.Reason = ReasonBadQuantity
		return pv
	}
	if amount > pv.Available {
		pv.Reason = ReasonInsufficientCredit
		return pv
	}
	pv.CashAfter = r.State.Cash + amount
	pv.DebtAfter = r.State.Debt + amount
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

// PreviewRepay repays any amount up to the balance and the cash on hand. The
// interest preview changes immediately, because it is always derived from the
// current balance.
func (r *Run) PreviewRepay(amount Money) PreviewLoan {
	pv := PreviewLoan{Amount: amount, CashBefore: r.State.Cash, DebtBefore: r.State.Debt,
		Ceiling: r.CreditCeiling(), Available: r.AvailableCredit(), InterestTomorrow: ceil10(r.State.Debt - amount),
		Reason: ReasonRunOver}
	if r.State.Over != EndOpen {
		return pv
	}
	if amount <= 0 {
		pv.Reason = ReasonBadQuantity
		return pv
	}
	if amount > r.State.Debt {
		pv.Reason = ReasonBadQuantity
		return pv
	}
	if amount > r.State.Cash {
		pv.Reason = ReasonInsufficientCash
		return pv
	}
	pv.CashAfter = r.State.Cash - amount
	pv.DebtAfter = r.State.Debt - amount
	pv.InterestTomorrow = ceil10(pv.DebtAfter)
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

// ActionKind names an action. Morning actions cost no time; Travel and Stay are
// the only two that advance the day.
type ActionKind int

const (
	ActBuy ActionKind = iota
	ActSell
	ActBorrow
	ActRepay
	ActTravel
	ActStay
	// ActAllocation accepts a morning's below-street allocation offer. Like
	// other morning actions it costs no day; unlike buys it is a one-off
	// deal Andy remembers refusing.
	ActAllocation
	// ActEncounter answers the police beat the calendar stopped for. Only a
	// pending encounter accepts it.
	ActEncounter
)

func (k ActionKind) String() string {
	switch k {
	case ActBuy:
		return "buy"
	case ActSell:
		return "sell"
	case ActBorrow:
		return "borrow"
	case ActRepay:
		return "repay"
	case ActTravel:
		return "travel"
	case ActStay:
		return "stay"
	case ActAllocation:
		return "allocation"
	case ActEncounter:
		return "encounter"
	}
	return "?"
}

// StayMode is what the player intends to do with the meter overnight.
type StayMode int

const (
	// StayAuto hosts the best subset the site's meter can carry: the default
	// a prototype trace means, and the best play a hosting player wants most
	// nights. Zero value on purpose — every recorded prototype Action decodes
	// to it unchanged.
	StayAuto StayMode = iota
	// StayPowerOff keeps the GPUs switched off: lease only, no meter.
	StayPowerOff
	// StayCurtail takes the grid operator's disclosed payment for staying
	// off; legal only while such an offer is open for this town.
	StayCurtail
)

func (m StayMode) String() string {
	switch m {
	case StayAuto:
		return "auto"
	case StayPowerOff:
		return "off"
	case StayCurtail:
		return "curtail"
	}
	return "?"
}

// Action is a player move. It is a value, so a trace is a list of these and a
// replay of a trace is a loop over it.
type Action struct {
	Kind    ActionKind        `json:"kind"`
	Product content.ProductID `json:"product,omitempty"`
	Market  content.MarketID  `json:"market,omitempty"`
	Qty     int               `json:"qty,omitempty"`
	Amount  Money             `json:"amount,omitempty"`
	// Mode is the overnight intent for a stay. Zero value is StayAuto, so
	// every recorded prototype trace decodes to exactly the night it meant.
	Mode StayMode `json:"mode,omitempty"`
	// Powered overrides the auto subset on a stay: a per-product count the
	// site's meter must be able to carry. nil means take the engine's best
	// packing, deterministically, so replay never re-puzzles a solved night.
	Powered []int `json:"powered,omitempty"`
	// Resp answers a pending encounter.
	Resp EncounterResponse `json:"resp,omitempty"`
}

// Receipt is the engine's account of what happened. The end screen's
// one-big-dollar-line plus expandable receipt reads from these fields.
type Receipt struct {
	Kind        ActionKind        `json:"kind"`
	Product     content.ProductID `json:"product,omitempty"`
	Market      content.MarketID  `json:"market,omitempty"`
	Qty         int               `json:"qty,omitempty"`
	Cost        Money             `json:"cost,omitempty"`
	Gross       Money             `json:"gross,omitempty"`
	Realized    Money             `json:"realized,omitempty"`
	ToLender    Money             `json:"toLender,omitempty"`
	CashIn      Money             `json:"cashIn,omitempty"`
	Freight     Money             `json:"freight,omitempty"`
	Lease       Money             `json:"lease,omitempty"`
	Interest    Money             `json:"interest,omitempty"`
	Hosted      Money             `json:"hosted,omitempty"`
	CashAfter   Money             `json:"cashAfter"`
	DebtAfter   Money             `json:"debtAfter"`
	DebtBefore  Money             `json:"debtBefore,omitempty"`
	NetWorth    Money             `json:"netWorth"`
	Day         int               `json:"day"`
	MarketName  string            `json:"marketName,omitempty"`
	Headline    string            `json:"headline,omitempty"`
	Forced      []ForcedSale      `json:"forced,omitempty"`
	CashUsed    Money             `json:"cashUsed,omitempty"`
	Repossessed bool              `json:"repossessed,omitempty"`
	Closed      bool              `json:"closed,omitempty"`
	CashOut     Money             `json:"cashOut,omitempty"`
	// Phase 5 itemizations: every new number the settlement moved gets its
	// own line, so a receipt answers "where did the money go" without a
	// second screen.
	Electricity Money             `json:"electricity,omitempty"`
	Curtail     Money             `json:"curtail,omitempty"`
	Clawback    Money             `json:"clawback,omitempty"`
	Powered     []int             `json:"powered,omitempty"`
	Encounter   string            `json:"encounter,omitempty"`
	Surrendered content.ProductID `json:"surrendered,omitempty"`
	Fine        Money             `json:"fine,omitempty"`
	// Pending marks the receipt that stopped the calendar for an encounter;
	// the day's account continues on the resolution receipt.
	Pending bool `json:"pending,omitempty"`
}

// Result reports whether an action was applied and, if not, why.
type Result struct {
	OK     bool   `json:"ok"`
	Reason Reason `json:"reason"`
	// PendingEncounter marks the commitment that stopped the calendar: the
	// day is not over, the beat is not answered, and the interface must ask
	// before anything else may happen.
	PendingEncounter bool `json:"pendingEncounter,omitempty"`
	// Short fields let the TUI name the exact gap in the player's own numbers.
	ShortCash   Money `json:"shortCash,omitempty"`
	ShortSlots  int   `json:"shortSlots,omitempty"`
	ShortStock  int   `json:"shortStock,omitempty"`
	MaxAccepted int   `json:"maxAccepted,omitempty"`
	MaxAmount   Money `json:"maxAmount,omitempty"`
}

// Apply runs a morning action: buy, sell, borrow, repay. None of them costs a
// day, and none of them can advance headlines, refresh stock, or reroll prices.
func (r *Run) Apply(a Action) (Receipt, Result) {
	if r.State.PendingEncounter != nil && a.Kind != ActEncounter {
		// The calendar is stopped at a police beat: nobody shops around a
		// seizure, and no second night commits under an unanswered stop.
		return Receipt{}, Result{Reason: ReasonEncounterPending}
	}
	switch a.Kind {
	case ActBuy:
		return r.applyBuy(a)
	case ActSell:
		return r.applySell(a)
	case ActBorrow:
		return r.applyBorrow(a)
	case ActRepay:
		return r.applyRepay(a)
	case ActTravel, ActStay:
		return r.CommitDay(a)
	case ActAllocation:
		return r.applyAllocation(a)
	case ActEncounter:
		return r.ResolveEncounter(a.Resp)
	}
	return Receipt{}, Result{Reason: ReasonUnknownProduct}
}

// applyAllocation takes Andy's constrained supply: whole dollars at the
// offered share of the local ask, cash-only like every buy, slots and all,
// and once per run no matter how good tomorrow looks. It costs no day.
func (r *Run) applyAllocation(a Action) (Receipt, Result) {
	off := r.PreviewAllocation()
	if !off.Eligible {
		switch {
		case r.State.AllocTaken:
			return Receipt{}, Result{Reason: ReasonNotForSale}
		case r.State.Heat >= 60:
			return Receipt{}, Result{Reason: ReasonAllocationBlocked}
		case off.Stock <= 0:
			return Receipt{}, Result{Reason: ReasonInsufficientStock}
		default:
			return Receipt{}, Result{Reason: ReasonAllocationBlocked}
		}
	}
	qty := a.Qty
	if qty <= 0 {
		return Receipt{}, Result{Reason: ReasonBadQuantity, MaxAccepted: off.Stock}
	}
	if qty > off.Stock {
		return Receipt{}, Result{Reason: ReasonInsufficientStock, ShortStock: qty - off.Stock, MaxAccepted: off.Stock}
	}
	cost := off.UnitPrice * Money(qty)
	if cost > r.State.Cash {
		return Receipt{}, Result{Reason: ReasonInsufficientCash, ShortCash: cost - r.State.Cash,
			MaxAccepted: int(r.State.Cash / maxMoney(off.UnitPrice, 1))}
	}
	def := content.ProductOf(off.Product)
	pi := r.Index(off.Product)
	if qty*def.SlotsPerUnit > r.SlotsAvailable() {
		return Receipt{}, Result{Reason: ReasonOvercapacity, ShortSlots: qty*def.SlotsPerUnit - r.SlotsAvailable()}
	}
	r.State.Cash -= cost
	r.State.Owned[pi] += qty
	r.State.CostBasis[pi] += cost
	r.State.AllocStock -= qty
	r.State.AllocTaken = true
	if r.cfg.Rules.Heat && off.Product == content.Mod40948 {
		// The same hot-card rule as any purchase: below-street does not mean
		// below-the-meter quiet.
		r.State.Heat = clampInt(r.State.Heat+10*qty, 0, 100)
	}
	rec := r.record(Receipt{Kind: ActAllocation, Product: off.Product, Qty: qty, Cost: cost})
	return rec, Result{OK: true}
}

func (r *Run) applyBuy(a Action) (Receipt, Result) {
	pv := r.PreviewBuy(a.Product, a.Qty)
	if !pv.Valid {
		res := Result{Reason: pv.Reason, ShortCash: pv.ShortBy, ShortSlots: pv.SlotsShort, ShortStock: pv.StockShort}
		res.MaxAccepted = r.BuyLimits(a.Product).Max
		return Receipt{}, res
	}
	pi := r.Index(a.Product)
	r.State.Cash -= pv.Cost
	r.State.Stock[pi] -= a.Qty
	r.State.Owned[pi] += a.Qty
	r.State.CostBasis[pi] += pv.Cost
	r.State.Heat = pv.HeatAfter
	rec := r.record(Receipt{Kind: ActBuy, Product: a.Product, Qty: a.Qty, Cost: pv.Cost})
	return rec, Result{OK: true}
}

func (r *Run) applySell(a Action) (Receipt, Result) {
	pv := r.PreviewSell(a.Product, a.Qty)
	if !pv.Valid {
		return Receipt{}, Result{Reason: pv.Reason, MaxAccepted: r.SellLimits(a.Product).Max}
	}
	r.sellUnits(r.Index(a.Product), a.Qty)
	r.State.Cash += pv.CashIn
	r.State.Debt -= pv.ToLender
	r.State.Stats.TradeProfit += pv.Realized
	if pv.Realized > r.State.Stats.BestFlip {
		r.State.Stats.BestFlip = pv.Realized
	}
	rec := r.record(Receipt{Kind: ActSell, Product: a.Product, Qty: a.Qty, Gross: pv.Gross,
		Realized: pv.Realized, ToLender: pv.ToLender, CashIn: pv.CashIn})
	return rec, Result{OK: true}
}

func (r *Run) applyBorrow(a Action) (Receipt, Result) {
	pv := r.PreviewBorrow(a.Amount)
	if !pv.Valid {
		return Receipt{}, Result{Reason: pv.Reason, MaxAmount: pv.Available}
	}
	r.State.Cash += a.Amount
	r.State.Debt += a.Amount
	r.State.Stats.Borrowed += a.Amount
	rec := r.record(Receipt{Kind: ActBorrow, Cost: a.Amount, DebtBefore: pv.DebtBefore})
	return rec, Result{OK: true}
}

func (r *Run) applyRepay(a Action) (Receipt, Result) {
	pv := r.PreviewRepay(a.Amount)
	if !pv.Valid {
		max := r.State.Debt
		if r.State.Cash < max {
			max = r.State.Cash
		}
		return Receipt{}, Result{Reason: pv.Reason, MaxAmount: max}
	}
	r.State.Cash -= a.Amount
	r.State.Debt -= a.Amount
	r.State.Stats.Repaid += a.Amount
	rec := r.record(Receipt{Kind: ActRepay, Cost: a.Amount, DebtBefore: pv.DebtBefore})
	return rec, Result{OK: true}
}

// record stamps the resulting position onto a receipt and updates the derived
// peak-net-worth record.
func (r *Run) record(rec Receipt) Receipt {
	rec.Day = r.State.Day
	rec.CashAfter = r.State.Cash
	rec.DebtAfter = r.State.Debt
	rec.NetWorth = r.NetWorth()
	if nw := r.NetWorth(); nw > r.State.Stats.PeakNetWorth {
		r.State.Stats.PeakNetWorth = nw
	}
	return rec
}
