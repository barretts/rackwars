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
	FleetSlots    int              `json:"fleetSlots"`
	// HostCap is the destination's hosting slots: how much of the fleet can
	// run there at night. It never blocks arrival; it only bounds hosting.
	HostCap int `json:"hostCap"`
	// NextLease is shown as information: arriving does not charge another
	// day's rent, and the trading prototype charges no lease at all.
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

// PreviewTravel quotes a one-day move. The fleet always travels — a small
// site never turns the truck away; it just hosts less of the fleet when the
// night comes. Slots bill freight, so the footprint is shown, not hidden.
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
	pv.FleetSlots = r.SlotsUsed()
	pv.PerSlotCharge = Money(content.PerSlotFreight * pv.FleetSlots)
	pv.Freight = pv.RouteFee + pv.PerSlotCharge
	pv.HostCap = dest.Slots
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
	// Customs checks departures from Huaqiangbei carrying GPUs — arriving is
	// shopping, and the empty coat is exempt by rule. The number shown here
	// is the exact number the roll uses (customsRoll).
	if r.cfg.Rules.Encounters && r.State.Market == content.Huaqiangbei && to != content.Huaqiangbei && pv.FleetSlots > 0 {
		pv.RiskNote = "customs risk " + itoa(r.customsRoll()) + "% on this shipment"
	}
	pv.Valid = true
	pv.Reason = ReasonOK
	return pv
}

// StayPreview is the itemized price of spending a night where you are:
// lease, meter, hosting, curtailment, invoice review, interest — every number
// the settlement will use, computed by the same functions before the player
// commits, so preview and receipt are one arithmetic. A preview that could
// drift from settlement would be the oldest con in datacenter history.
type StayPreview struct {
	Market   content.MarketID `json:"market"`
	Mode     StayMode         `json:"mode"`
	Lease    Money            `json:"lease"`
	Electric Money            `json:"electric"`
	Hosted   Money            `json:"hosted"`
	Curtail  Money            `json:"curtail"`
	Clawback Money            `json:"clawback"`
	Interest Money            `json:"interest"`
	Powered  []int            `json:"powered,omitempty"`
	// HostedSlots and SlotCap put the night's rack footprint against the
	// site's hosting capacity; the meter has its pair above.
	HostedSlots int    `json:"hostedSlots"`
	SlotCap     int    `json:"slotCap"`
	PowerKW     int64  `json:"powerKw100"`
	PowerCap    int64  `json:"powerCapKw100"`
	Eligible    bool   `json:"eligible"`
	Clean       bool   `json:"clean"`
	CashAfter   Money  `json:"cashAfter"`
	DebtAfter   Money  `json:"debtAfter"`
	Heat        int    `json:"heat"`
	Reason      Reason `json:"reason,omitempty"`
	Valid       bool   `json:"valid"`
}

// stayPlan resolves the night: which GPUs run, whether the grid operator's
// money is being taken, and why a plan is illegal. One plan function feeds
// both the preview and the settlement, so neither can drift from the other,
// and an explicit powered subset is validated here, once, in one place.
func (r *Run) stayPlan(mode StayMode, powered []int) (chosen []int, curtail Money, reason Reason) {
	m := r.State.Market
	site := content.MarketOf(m).Site
	elig := r.hostEligible(m)
	np := len(r.cfg.Products)
	chosen = make([]int, np)
	switch mode {
	case StayAuto:
		if elig {
			chosen = r.bestPowered(r.State.Day, m)
		}
	case StayPowerOff:
	case StayCurtail:
		if !site || !r.cfg.Rules.Hosting || !r.cfg.Rules.Power {
			return nil, 0, ReasonCurtailUnavailable
		}
		offer, ok := r.curtailOffer(r.State.Day, m)
		if !ok {
			return nil, 0, ReasonCurtailUnavailable
		}
		curtail = offer
	default:
		return nil, 0, ReasonBadQuantity
	}
	if len(powered) > 0 {
		// An explicit subset is allowed only where hosting lives, and never
		// beside curtailment: the operator pays for a switched-off meter, not
		// for a different packing puzzle.
		if mode == StayCurtail || !elig || len(powered) != np {
			return nil, 0, ReasonBadQuantity
		}
		for pi, n := range powered {
			if n < 0 || n > r.State.Owned[pi] {
				return nil, 0, ReasonBadQuantity
			}
		}
		if content.MarketOf(m).PowerKW > 0 && poweredKW100(r.cfg.Products, powered) > content.MarketOf(m).PowerKW {
			return nil, 0, ReasonPowerOver
		}
		if hostSlots(r.cfg.Products, powered) > content.MarketOf(m).Slots {
			return nil, 0, ReasonOvercapacity
		}
		chosen = powered
	}
	return chosen, curtail, ReasonOK
}

// PreviewStay prices a default (host-the-best-that-fits) night. With prototype
// rules every new number is zero and the old columns hold: a stay at the free
// garage costs nothing and earns nothing, exactly as the recorded traces show.
// Mode-aware callers pass the exact action they will commit.
func (r *Run) PreviewStay() StayPreview {
	return r.PreviewStayMode(StayAuto, nil)
}

func (r *Run) PreviewStayMode(mode StayMode, powered []int) StayPreview {
	m := r.State.Market
	pv := StayPreview{Market: m, Mode: mode, Heat: r.State.Heat, Reason: ReasonRunOver}
	if r.State.Over != EndOpen {
		return pv
	}
	if r.State.PendingEncounter != nil {
		pv.Reason = ReasonEncounterPending
		return pv
	}
	if r.cfg.Rules.Leases {
		pv.Lease = Money(content.MarketOf(m).DailyLease)
	}
	chosen, curtail, reason := r.stayPlan(mode, powered)
	if reason != ReasonOK {
		pv.Reason = reason
		return pv
	}
	pv.Curtail = curtail
	pv.Powered = chosen
	pv.PowerKW = poweredKW100(r.cfg.Products, chosen)
	pv.PowerCap = content.MarketOf(m).PowerKW
	pv.HostedSlots = hostSlots(r.cfg.Products, chosen)
	pv.SlotCap = content.MarketOf(m).Slots
	pv.Eligible = r.hostEligible(m)
	pv.Clean = r.State.Heat < 60
	pv.Electric = elecCostDollars(pv.PowerKW, content.MarketOf(m).ElecCentsPerKWh)
	pv.Hosted, _ = r.hostNight(r.State.Day, m, chosen)
	pv.Clawback = r.clawbackDue()
	// Settlement order, walked here too: credits and charges land first; an
	// uncovered charge becomes debt; interest is then struck once on the
	// grown debt. So debt-after here is debt-after on the receipt.
	net := pv.Hosted + pv.Curtail - pv.Lease - pv.Electric - pv.Clawback
	cash := r.State.Cash + net
	debt := r.State.Debt
	if cash < 0 {
		debt -= cash
		cash = 0
	}
	pv.Interest = ceil10(debt)
	pv.CashAfter, pv.DebtAfter = cash, debt+pv.Interest
	pv.Valid = true
	return pv
}

// clawbackDue is today's invoice review: if yesterday's hosting paid out on
// the affected contract, the reinterpretation takes its stated share, capped.
// The cap is what keeps a reinterpreted contract a haircut, not a second rent.
func (r *Run) clawbackDue() Money {
	pct, cap, ok := r.clawbackActive(r.State.Day)
	if !ok || r.State.LastHostEnt <= 0 {
		return 0
	}
	cut := r.State.LastHostEnt * pct / 100
	if cut > cap {
		cut = cap
	}
	return cut
}

// CommitDay is the only way time moves. Travel and Stay each consume exactly
// one day, and the order of operations below is the authoritative one from the
// design: lock the action and its displayed costs, resolve travel or the stay
// (hosting pays out here, before any police beat can), resolve at most one
// encounter — which may pause the calendar before the rest of the day exists —
// then accrue interest once, and either close the run or advance and run the
// credit check.
func (r *Run) CommitDay(a Action) (Receipt, Result) {
	if r.State.Over != EndOpen {
		return Receipt{}, Result{Reason: ReasonRunOver}
	}
	if r.State.PendingEncounter != nil {
		return Receipt{}, Result{Reason: ReasonEncounterPending}
	}
	if a.Kind != ActTravel && a.Kind != ActStay {
		return Receipt{}, Result{Reason: ReasonBadQuantity}
	}

	rec := Receipt{Kind: a.Kind, Day: r.State.Day, DebtBefore: r.State.Debt}
	from := r.State.Market

	if a.Kind == ActTravel {
		pv := r.PreviewTravel(a.Market)
		if !pv.Valid {
			return Receipt{}, Result{Reason: pv.Reason}
		}
		rec.Freight = pv.Freight
		rec.Market = a.Market
		rec.MarketName = content.MarketOf(a.Market).Name
		// An invoice review catches up with players on the road too: the
		// counterparty reinterprets yesterday's hosting contract wherever
		// yesterday happened, and travel does not outrun today's letter.
		rec.Clawback = r.clawbackDue()
		r.State.Cash -= pv.Freight
		r.State.Stats.FreightPaid += pv.Freight
		r.State.Stats.TravelDays++
		if rec.Clawback > 0 {
			r.State.Cash -= rec.Clawback
			r.State.Stats.ClawbackPaid += rec.Clawback
		}
		r.State.LastHostEnt = 0 // the review window over last night closes
		// An uncovered charge becomes debt before the credit check rather than
		// being refused mid-day.
		if r.State.Cash < 0 {
			r.State.Debt += -r.State.Cash
			r.State.Cash = 0
		}
		r.State.Market = a.Market
	} else {
		pv := r.PreviewStayMode(a.Mode, a.Powered)
		if !pv.Valid {
			return Receipt{}, Result{Reason: pv.Reason}
		}
		rec.Lease = pv.Lease
		rec.Hosted = pv.Hosted
		rec.Electricity = pv.Electric
		rec.Curtail = pv.Curtail
		rec.Clawback = pv.Clawback
		rec.Powered = pv.Powered
		rec.MarketName = content.MarketOf(r.State.Market).Name
		// Credits first, then charges; the preview already priced this exact
		// net, and the numbers below are the preview's, not a recomputation
		// that could drift.
		r.State.Cash += pv.Hosted + pv.Curtail
		r.State.Cash -= pv.Lease + pv.Electric + pv.Clawback
		r.State.Stats.LeasePaid += pv.Lease
		r.State.Stats.PowerCost += pv.Electric
		r.State.Stats.HostingIncome += pv.Hosted
		if pv.Curtail > 0 {
			r.State.Stats.CurtailIncome += pv.Curtail
		}
		if rec.Clawback > 0 {
			r.State.Stats.ClawbackPaid += rec.Clawback
		}
		// The clawback reads last night's number, so the review window is
		// stamped out only after the bill has been written.
		_, ent := r.hostNight(r.State.Day, r.State.Market, pv.Powered)
		r.State.LastHostEnt = ent
		r.State.Stats.StayDays++
		if r.State.Cash < 0 {
			r.State.Debt += -r.State.Cash
			r.State.Cash = 0
		}
		if r.cfg.Rules.Heat {
			// A quiet stay with the GPUs off cools the meter; hosting cools
			// less because the fans never stopped. Travel does not cool.
			cool := 10
			if pv.Hosted > 0 {
				cool = 5
			}
			r.State.Heat = clampInt(r.State.Heat-cool, 0, 100)
		}
	}

	// At most one police beat per day, resolved after the stay pays out and
	// before interest: the calendar stops here until the player answers.
	if e := r.rollEncounter(a, from); e != nil {
		r.State.PendingEncounter = e
		r.State.PendingKind = a.Kind
		rec.Pending = true
		r.State.PendingRec = &rec
		return rec, Result{OK: true, PendingEncounter: true}
	}
	return r.dayTail(rec)
}

// ResolveEncounter answers the pending police beat and finishes the day the
// paused CommitDay started: outcome, then interest, then the close or the
// advance. Reloading cannot reroll the stop: the outcome was drawn at trigger
// time and lives in the save.
func (r *Run) ResolveEncounter(resp EncounterResponse) (Receipt, Result) {
	if r.State.Over != EndOpen {
		return Receipt{}, Result{Reason: ReasonRunOver}
	}
	e := r.State.PendingEncounter
	if e == nil || r.State.PendingRec == nil {
		return Receipt{}, Result{Reason: ReasonNoEncounter}
	}
	rec := r.resolveEncounter(*r.State.PendingRec, e, resp)
	r.State.PendingEncounter = nil
	r.State.PendingKind = 0
	r.State.PendingRec = nil
	return r.dayTail(rec)
}

// dayTail is the half of the day an encounter was allowed to interrupt:
// interest exactly once on whatever the debt has become, then the day-30
// close or the advance with its headline, market refresh, and credit check.
func (r *Run) dayTail(rec Receipt) (Receipt, Result) {
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
