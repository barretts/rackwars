package game

import (
	"rackwars/internal/content"
)

// Shoreline Credit: one visible balance, one daily rate, one collateral rule.
//
//	collateral value = sum(owned units x current local sell bid)
//	credit ceiling   = $30,000 + floor(0.50 x collateral value)
//	available credit = max(0, credit ceiling - outstanding debt)
//
// The unsecured floor is a fixed allowance, not free cash or income. Interest
// can push the balance above the ceiling, which is what makes the morning check
// real rather than decorative.
const (
	UnsecuredFloor          = 30000
	CollateralLoanRatePct   = 50
	DailyInterestRatePct    = 10
	StressCollateralFallPct = 50
)

// CreditCeiling is the most Shoreline will carry at current bids.
func (r *Run) CreditCeiling() Money {
	return Money(UnsecuredFloor) + r.CollateralValue()*CollateralLoanRatePct/100
}

// AvailableCredit is what can still be borrowed right now.
func (r *Run) AvailableCredit() Money {
	c := r.CreditCeiling() - r.State.Debt
	if c < 0 {
		return 0
	}
	return c
}

// BreachAmount is how far the balance is above the ceiling, or zero.
func (r *Run) BreachAmount() Money {
	d := r.State.Debt - r.CreditCeiling()
	if d < 0 {
		return 0
	}
	return d
}

// StressPreview answers the question a player actually asks on rumor day: if
// collateral bids fall by half tomorrow, what does the note look like?
type StressPreview struct {
	BidsFallPct     Money `json:"bidsFallPct"`
	Collateral      Money `json:"collateral"`
	Ceiling         Money `json:"ceiling"`
	Headroom        Money `json:"headroom"`
	Interest        Money `json:"interest"`
	ProjectedDebt   Money `json:"projectedDebt"`
	ProjectedBreach Money `json:"projectedBreach"`
}

// LenderStress projects the next morning at a stated bid fall, using the
// current balance. The lender panel always shows a 50% fall.
func (r *Run) LenderStress(fallPct int) StressPreview {
	if fallPct < 0 {
		fallPct = 0
	}
	if fallPct > 100 {
		fallPct = 100
	}
	collateral := r.CollateralValue() * Money(100-fallPct) / 100
	ceiling := Money(UnsecuredFloor) + collateral*CollateralLoanRatePct/100
	debt := r.State.Debt + ceil10(r.State.Debt)
	breach := debt - ceiling
	if breach < 0 {
		breach = 0
	}
	headroom := ceiling - debt
	if headroom < 0 {
		headroom = 0
	}
	return StressPreview{
		BidsFallPct:     Money(fallPct),
		Collateral:      collateral,
		Ceiling:         ceiling,
		Headroom:        headroom,
		Interest:        ceil10(r.State.Debt),
		ProjectedDebt:   debt,
		ProjectedBreach: breach,
	}
}

// ForcedSale is one unit Shoreline sold out of the rack. Every forced sale is
// itemized: a margin call that feels like stolen agency is a design failure, so
// the receipt has to show each unit and what it fetched.
type ForcedSale struct {
	Product    content.ProductID `json:"product"`
	Units      int               `json:"units"`
	UnitBid    Money             `json:"unitBid"`
	Proceeds   Money             `json:"proceeds"`
	DebtPaid   Money             `json:"debtPaid"`
	Surplus    Money             `json:"surplus"`
	LossVsCost Money             `json:"lossVsCost"`
}

// CureReport is the morning credit check. DebtBefore/After and the itemized
// sales give the before-and-after figures the lender receipt must show.
type CureReport struct {
	Triggered  bool         `json:"triggered"`
	CashUsed   Money        `json:"cashUsed"`
	Sales      []ForcedSale `json:"sales"`
	DebtBefore Money        `json:"debtBefore"`
	DebtAfter  Money        `json:"debtAfter"`
	// Repossessed is set only when cash and GPUs are gone and the balance is
	// still above the unsecured floor. That is the run-ending case; negative
	// net worth on its own never ends a run.
	Repossessed bool `json:"repossessed"`
}

// sellUnits removes n units of a card and returns the dollars of cost basis
// released, keeping the average acquisition price consistent for the holdings
// display.
func (r *Run) sellUnits(pi, n int) Money {
	held := r.State.Owned[pi]
	if n > held {
		n = held
	}
	if n <= 0 {
		return 0
	}
	basis := r.State.CostBasis[pi]
	// Whole dollars, and the last unit released always carries the remainder so
	// the basis reaches exactly zero when the position does.
	released := basis / Money(held) * Money(n)
	if n == held {
		released = basis
	}
	r.State.Owned[pi] -= n
	r.State.CostBasis[pi] -= released
	return released
}

// worstSaleOrder picks which card Shoreline sells first: highest bid per unit,
// with product order breaking ties so the same morning never resolves two ways.
func (r *Run) worstSaleOrder() int {
	mi := r.MarketIndex(r.State.Market)
	best, bestBid := -1, Money(-1)
	for pi := range r.cfg.Products {
		if r.State.Owned[pi] <= 0 {
			continue
		}
		if bid := r.quoteAt(mi, pi).Bid; bid > bestBid {
			best, bestBid = pi, bid
		}
	}
	return best
}

// runCreditCheck resolves Shoreline's morning check against the *new* bids,
// before the player can trade. Cash first, then whole units at the displayed
// bid, recalculating the shrinking collateral base after every sale.
func (r *Run) runCreditCheck() CureReport {
	rep := CureReport{DebtBefore: r.State.Debt, DebtAfter: r.State.Debt}
	need := r.BreachAmount()
	if need <= 0 {
		return rep
	}
	rep.Triggered = true

	if r.State.Cash > 0 {
		use := r.State.Cash
		if use > need {
			use = need
		}
		r.State.Cash -= use
		r.State.Debt -= use
		rep.CashUsed = use
	}

	for r.BreachAmount() > 0 {
		pi := r.worstSaleOrder()
		if pi < 0 {
			break
		}
		bid := r.quoteAt(r.MarketIndex(r.State.Market), pi).Bid
		avg := r.AvgCost(r.cfg.Products[pi])
		released := r.sellUnits(pi, 1)
		_ = released
		applied := bid
		if applied > r.State.Debt {
			applied = r.State.Debt
		}
		r.State.Debt -= applied
		r.State.Cash += bid - applied
		rep.Sales = append(rep.Sales, ForcedSale{
			Product:    r.cfg.Products[pi],
			Units:      1,
			UnitBid:    bid,
			Proceeds:   bid,
			DebtPaid:   applied,
			Surplus:    bid - applied,
			LossVsCost: lossVsCost(avg, bid),
		})
		r.State.Stats.ForcedUnits++
		r.State.Stats.ForcedLoss += lossVsCost(avg, bid)
	}

	rep.DebtAfter = r.State.Debt
	if r.BreachAmount() > 0 && r.CollateralValue() == 0 && r.State.Cash == 0 {
		rep.Repossessed = true
	}
	return rep
}

func lossVsCost(avg, bid Money) Money {
	if avg <= bid {
		return 0
	}
	return avg - bid
}
