package game

import (
	"rackwars/internal/content"
)

// TravelPreview is the exact quote the player confirms before spending a day.
// Freight and the day spent waiting both count against a spread, so a big
// apparent price gap is not automatically a profitable journey.
type TravelPreview struct {
	From          content.MarketID `json:"from"`
	To            content.MarketID `json:"to"`
	Freight       Money            `json:"freight"`
	PerSlotCharge Money            `json:"perSlotCharge"`
	RouteFee      Money            `json:"routeFee"`
	OccupiedSlots int              `json:"occupiedSlots"`
	SlotsTotal    int              `json:"slotsTotal"`
	// NextLease is shown as information: arriving does not charge another day's
	// rent, and the trading prototype charges no lease at all.
	NextLease  Money `json:"nextLease"`
	Interest   Money `json:"interest"`
	HostedHere Money `json:"hostedHere"`
	CashAfter  Money `json:"cashAfter"`
	DebtAfter  Money `json:"debtAfter"`
	// ArrivalSeenDay tags the destination's last-seen quotes with the day they
	// were observed. Never presented as a guaranteed arrival price.
	ArrivalSeenDay int    `json:"arrivalSeenDay"`
	ArrivalNote    string `json:"arrivalNote"`
	RiskNote       string `json:"riskNote"`
	Valid          bool   `json:"valid"`
	Reason         Reason `json:"reason"`
}

// PreviewTravel quotes a one-day move. Moving to a smaller site is blocked
// before confirmation: there is no hidden storage penalty to discover later.
func (r *Run) PreviewTravel(to content.MarketID) TravelPreview {
	mi := r.MarketIndex(to)
	pv := TravelPreview{From: r.State.Market, To: to, Reason: ReasonRunOver}
	if r.State.Over != EndOpen {
		return pv
	}
	if mi < 0 || to == r.State.Market {
		pv.Reason = ReasonNoRoute
		return pv
	}
	dest := content.MarketOf(to)
	pv.RouteFee = Money(content.RouteFee(r.State.Market, to))
	pv.OccupiedSlots = r.SlotsUsed()
	pv.PerSlotCharge = Money(content.PerSlotFreight * pv.OccupiedSlots)
	pv.Freight = pv.RouteFee + pv.PerSlotCharge
	pv.SlotsTotal = dest.Slots
	pv.NextLease = Money(dest.DailyLease)
	pv.Interest = ceil10(r.State.Debt)
	pv.CashAfter = r.State.Cash - pv.Freight
	pv.DebtAfter = r.State.Debt + pv.Interest
	pv.ArrivalSeenDay = r.SeenDayOf(to)
	pv.HostedHere = 0
	if pv.ArrivalSeenDay > 0 {
		pv.ArrivalNote = "last seen quotes are from day " + itoa(pv.ArrivalSeenDay)
	} else {
		pv.ArrivalNote = "never traded here; prices unknown until arrival"
	}
	if r.cfg.Rules.Encounters && (r.State.Market == content.Huaqiangbei || to == content.Huaqiangbei) {
		chance := 5
		if r.State.Heat >= 60 {
			chance = 25
		}
		if pv.OccupiedSlots > 0 {
			pv.RiskNote = "customs risk " + itoa(chance) + "% on this shipment"
		}
	}

	if pv.OccupiedSlots > dest.Slots {
		pv.Reason = ReasonOvercapacity
		return pv
	}
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

// StayPreview is the cost of spending the day where the player is.
type StayPreview struct {
	Market    content.MarketID `json:"market"`
	Lease     Money            `json:"lease"`
	Hosted    Money            `json:"hosted"`
	Interest  Money            `json:"interest"`
	Heat      int              `json:"heat"`
	CashAfter Money            `json:"cashAfter"`
	DebtAfter Money            `json:"debtAfter"`
	Valid     bool             `json:"valid"`
}

// PreviewStay shows the net cost of a stay. Hosting is switched off in the
// trading prototype, so a stay costs the site lease and nothing else, and earns
// nothing.
func (r *Run) PreviewStay() StayPreview {
	pv := StayPreview{Market: r.State.Market, Interest: ceil10(r.State.Debt), Heat: r.State.Heat, Valid: r.State.Over == EndOpen}
	if r.cfg.Rules.Leases {
		pv.Lease = Money(content.MarketOf(r.State.Market).DailyLease)
	}
	pv.CashAfter = r.State.Cash - pv.Lease + pv.Hosted
	pv.DebtAfter = r.State.Debt + pv.Interest
	return pv
}

// CommitDay is the only way time moves. Travel and Stay each consume exactly
// one day, and the order of operations below is the authoritative one from the
// design: lock the action and its displayed costs, resolve travel or the stay,
// resolve at most one encounter, charge operating costs and accrue interest
// once, then either close the run or advance and run the credit check.
func (r *Run) CommitDay(a Action) (Receipt, Result) {
	if r.State.Over != EndOpen {
		return Receipt{}, Result{Reason: ReasonRunOver}
	}
	if a.Kind != ActTravel && a.Kind != ActStay {
		return Receipt{}, Result{Reason: ReasonBadQuantity}
	}

	rec := Receipt{Kind: a.Kind, Day: r.State.Day, DebtBefore: r.State.Debt}

	if a.Kind == ActTravel {
		pv := r.PreviewTravel(a.Market)
		if !pv.Valid {
			return Receipt{}, Result{Reason: pv.Reason}
		}
		rec.Freight = pv.Freight
		rec.Market = a.Market
		rec.MarketName = content.MarketOf(a.Market).Name
		r.State.Cash -= pv.Freight
		r.State.Stats.FreightPaid += pv.Freight
		r.State.Stats.TravelDays++
		// An uncovered charge becomes debt before the credit check rather than
		// being refused mid-day.
		if r.State.Cash < 0 {
			r.State.Debt += -r.State.Cash
			r.State.Cash = 0
		}
		r.State.Market = a.Market
	} else {
		pv := r.PreviewStay()
		rec.Lease = pv.Lease
		rec.Hosted = pv.Hosted
		rec.MarketName = content.MarketOf(r.State.Market).Name
		if r.cfg.Rules.Leases {
			r.State.Cash -= pv.Lease
			r.State.Stats.LeasePaid += pv.Lease
		}
		if pv.Hosted > 0 {
			r.State.Cash += pv.Hosted
			r.State.Stats.HostingIncome += pv.Hosted
		}
		r.State.Stats.StayDays++
		if r.State.Cash < 0 {
			r.State.Debt += -r.State.Cash
			r.State.Cash = 0
		}
		if r.cfg.Rules.Heat {
			// A quiet stay with the GPUs off cools the meter; hosting cools
			// less. Travel does not cool anything.
			cool := 10
			if pv.Hosted > 0 {
				cool = 5
			}
			r.State.Heat = clampInt(r.State.Heat-cool, 0, 100)
		}
	}

	// Encounters would resolve here, at most one per day. Switched off in the
	// trading prototype, so nothing happens and no hidden roll is made.

	// Interest accrues exactly once per elapsed day, after any charge that had
	// to become debt.
	interest := ceil10(r.State.Debt)
	r.State.Debt += interest
	r.State.Stats.InterestPaid += interest
	rec.Interest = interest
	r.State.Stats.Commitments++

	if r.State.Day >= content.DayCount {
		return r.closeRun(rec), Result{OK: true}
	}

	r.State.Day++
	r.refreshMarket(r.State.Day)
	cure := r.runCreditCheck()
	rec.Forced = cure.Sales
	rec.CashUsed = cure.CashUsed
	r.State.Stats.PeakNetWorth = maxMoney(r.State.Stats.PeakNetWorth, r.NetWorth())
	rec = r.record(rec)
	rec.Headline = headlineText(r.State.Day)
	if cure.Repossessed {
		r.endRepossessed()
		rec.Repossessed = true
		rec.CashOut = r.State.End.CashOut
		rec.Closed = true
	}
	return rec, Result{OK: true}
}

func headlineText(day int) string {
	if h := content.HeadlineFor(day); h != nil {
		return h.Text
	}
	return ""
}

// closeRun is the day-30 closing auction: everything the player still holds is
// sold at the displayed closing bids, all cash goes to the debt, and what is
// left over is the score. No day-31 interest, event, or hidden reroll.
func (r *Run) closeRun(rec Receipt) Receipt {
	mi := r.MarketIndex(r.State.Market)
	auction := Money(0)
	for pi, p := range r.cfg.Products {
		held := r.State.Owned[pi]
		if held <= 0 {
			continue
		}
		bid := r.quoteAt(mi, pi).Bid
		gross := bid * Money(held)
		avg := r.AvgCost(p)
		realized := (bid - avg) * Money(held)
		auction += gross
		r.State.Stats.TradeProfit += realized
		if realized > r.State.Stats.BestFlip {
			r.State.Stats.BestFlip = realized
		}
		r.sellUnits(pi, held)
	}
	r.State.Cash += auction
	rec.Gross += auction
	rec.CashAfter = r.State.Cash
	rec.DebtAfter = r.State.Debt

	cashOut := r.State.Cash - r.State.Debt
	end := End{Kind: EndClosed, CashOut: cashOut}
	if cashOut < 0 {
		end.Deficiency = -cashOut
		r.State.Cash = 0
		// The written-off remainder is still reported as a deficiency; the
		// score never hides what the player owed.
		r.State.Debt = 0
	} else {
		// The note is settled out of the sale proceeds and the player walks away
		// holding the rest as cash. Debt is never negative: the score is what the
		// player is owed, not a loan from the player to the lender.
		r.State.Debt = 0
		r.State.Cash = cashOut
	}
	end.Rank = RankFor(cashOut)
	end.Epitaph = epitaph(end, r.State.Stats)
	r.State.Over = EndClosed
	r.State.End = end
	rec.Closed = true
	rec.CashOut = cashOut
	rec.NetWorth = cashOut
	rec.MarketName = content.MarketOf(r.State.Market).Name
	return rec
}

// endRepossessed finishes a run when cash and GPUs are gone and the balance is
// still above the unsecured floor.
func (r *Run) endRepossessed() {
	debt := r.State.Debt
	end := End{
		Kind:       EndRepossessed,
		CashOut:    -debt,
		Deficiency: debt,
		Rank:       "Shoreline's new subsidiary",
		Epitaph:    epitaph(End{Kind: EndRepossessed, CashOut: -debt, Deficiency: debt}, r.State.Stats),
	}
	r.State.Over = EndRepossessed
	r.State.End = end
}

// RankFor awards the highest qualifying cash-out rank. These tiers are the
// design sheet's provisional thresholds; Phase 6 may recalibrate them with
// recorded evidence, and the test that pins them exists so a change is a
// deliberate act.
func RankFor(cashOut Money) string {
	switch {
	case cashOut < 0:
		return "Shoreline's new subsidiary"
	case cashOut == 0:
		return "Walked away"
	case cashOut >= 10000000:
		return "Neocloud"
	case cashOut >= 1000000:
		return "Regional problem"
	case cashOut >= 100000:
		return "Garage graduate"
	default:
		return "Still independent"
	}
}

func epitaph(e End, s Stats) string {
	switch e.Kind {
	case EndRepossessed:
		return "Shoreline sold the rack and kept the difference: " + FormatMoney(e.Deficiency) +
			" unpaid, " + FormatMoney(s.InterestPaid) + " of it interest."
	case EndClosed:
		if e.CashOut < 0 {
			return "Sold everything and still owed " + FormatMoney(e.Deficiency) +
				"; interest took " + FormatMoney(s.InterestPaid) + " of the run."
		}
		if e.CashOut == 0 {
			return "Out exactly even after " + FormatMoney(s.InterestPaid) + " in interest and " +
				FormatMoney(s.FreightPaid) + " in freight."
		}
		return "Finished with " + FormatMoney(e.CashOut) + " after " + FormatMoney(s.InterestPaid) +
			" in interest, best flip " + FormatMoney(s.BestFlip) + "."
	}
	return ""
}

func maxMoney(a, b Money) Money {
	if a > b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
