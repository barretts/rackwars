package game

import (
	"testing"

	"rackwars/internal/content"
)

// Phase 5 mechanic tests: the arithmetic the player sees is the arithmetic the
// engine charges, and every gate refuses with the reason it advertises.

func fullRun(t *testing.T, seed uint64, m content.MarketID, day int, cash Money) *Run {
	t.Helper()
	r, err := New(Config{Seed: seed, Products: content.AllProducts, Markets: content.AllMarkets, Rules: content.FullRules()})
	if err != nil {
		t.Fatal(err)
	}
	r.State.Day = day
	r.State.Market = m
	r.State.Cash = cash
	// A run always opens carrying the bank's $20,000 note; these tests probe
	// one mechanic's arithmetic, so the note is cleared here. Its own compounding
	// is covered by the recorded prototype traces and the sim gates.
	r.State.Debt = 0
	r.refreshMarket(day)
	return r
}

func give(r *Run, p content.ProductID, n int) {
	pi := r.Index(p)
	r.State.Owned[pi] += n
	r.State.CostBasis[pi] += r.QuoteHere(p).Ask * Money(n)
}

func TestHostingPreviewMatchesSettlement(t *testing.T) {
	r := fullRun(t, 11, content.WestTexas, 5, 1_000_000)
	give(r, content.CardH100, 3)
	give(r, content.CardMI300, 2)
	give(r, content.Card4090, 5)
	pv := r.PreviewStayMode(StayAuto, nil)
	if !pv.Valid {
		t.Fatalf("auto stay refused: %v", pv.Reason)
	}
	if want := []int{0, 5, 0, 0, 3, 2}; !sameInts(pv.Powered, want) {
		t.Fatalf("powered = %v, want the whole fleet packed %v", pv.Powered, want)
	}
	if pv.Hosted != 1900 || pv.Electric != 26 || pv.Lease != 250 {
		t.Fatalf("bill: hosted %s electric %s lease %s, want 1900/26/250",
			FormatMoney(pv.Hosted), FormatMoney(pv.Electric), FormatMoney(pv.Lease))
	}
	before := r.State.Cash
	if res := resOnly(r.Apply(Action{Kind: ActStay})); !res.OK {
		t.Fatalf("stay refused: %v", res.Reason)
	}
	if r.State.Cash != before+1900-26-250 {
		t.Fatalf("settlement cash %s != preview %s", FormatMoney(r.State.Cash), FormatMoney(pv.CashAfter))
	}
	if r.State.Cash != pv.CashAfter || r.State.Debt != pv.DebtAfter {
		t.Fatalf("cash/debt drifted from preview")
	}
}

func TestExplicitSubsetOverMeterRefused(t *testing.T) {
	r := fullRun(t, 12, content.WestTexas, 4, 200_000)
	give(r, content.Sled3090, 12) // 12 x 160 = 1920 c-kW over a 1000 cap
	pv := r.PreviewStayMode(StayAuto, []int{12, 0, 0, 0, 0, 0})
	if pv.Valid || pv.Reason != ReasonPowerOver {
		t.Fatalf("unsafe subset accepted: valid=%v reason=%v", pv.Valid, pv.Reason)
	}
	day := r.State.Day
	res := resOnly(r.Apply(Action{Kind: ActStay, Powered: []int{12, 0, 0, 0, 0, 0}}))
	if res.Reason != ReasonPowerOver || r.State.Day != day {
		t.Fatalf("commit = %v (day %d), want %v with no day burned", res.Reason, r.State.Day, ReasonPowerOver)
	}
	pv = r.PreviewStayMode(StayAuto, nil)
	if pv.PowerKW > content.MarketOf(content.WestTexas).PowerKW {
		t.Fatalf("auto packed %d over the meter", pv.PowerKW)
	}
}

func TestGarageMeterForcesPartialHost(t *testing.T) {
	r := fullRun(t, 13, content.Garage, 2, 1_000_000)
	give(r, content.CardH100, 2)
	give(r, content.Card4090, 1)
	pv := r.PreviewStayMode(StayAuto, nil)
	if !sameInts(pv.Powered, []int{0, 0, 0, 0, 2, 0}) {
		t.Fatalf("powered = %v, want two H100s on a 200 c-kW meter", pv.Powered)
	}
	if pv.Hosted != 700 || pv.Electric != 7 {
		t.Fatalf("garage night: hosted %s elec %s, want 700/7",
			FormatMoney(pv.Hosted), FormatMoney(pv.Electric))
	}
}

func TestElectricityRoundsUpOnly(t *testing.T) {
	r := fullRun(t, 14, content.Garage, 2, 50_000)
	give(r, content.Card4090, 1) // 50 c-kW x 24h x 14c = $1.68
	pv := r.PreviewStayMode(StayAuto, nil)
	if pv.Electric != 2 {
		t.Fatalf("electric = %s, want the 1.68 bill rounded up to 2", FormatMoney(pv.Electric))
	}
	before := r.State.Cash
	if res := resOnly(r.Apply(Action{Kind: ActStay})); !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.Cash != before+70-2 { // home lease is zero
		t.Fatalf("cash delta %s, want +68", FormatMoney(r.State.Cash-before))
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func resOnly(_ Receipt, res Result) Result { return res }

func TestCurtailmentOnlyInsideOfferWindow(t *testing.T) {
	r := fullRun(t, 21, content.WestTexas, 21, 20_000)
	give(r, content.CardMI300, 1)
	r.State.Heat = 40
	pv := r.PreviewStayMode(StayCurtail, nil)
	if !pv.Valid || pv.Curtail != 8000 || pv.Hosted != 0 || pv.Electric != 0 || pv.Lease != 250 {
		t.Fatalf("curtail preview = %+v, want 8000 credit, zero meter, lease due", pv)
	}
	before := r.State.Cash
	if res := resOnly(r.Apply(Action{Kind: ActStay, Mode: StayCurtail})); !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.Cash != before+8000-250 {
		t.Fatalf("curtail settlement cash %s, want +7750", FormatMoney(r.State.Cash-before))
	}
	if r.State.Stats.CurtailIncome != 8000 {
		t.Fatalf("CurtailIncome %s, want 8000", FormatMoney(r.State.Stats.CurtailIncome))
	}
	if r.State.Heat != 30 { // a switched-off night removes 10, even a paid one
		t.Fatalf("heat after a dark night = %d, want 30", r.State.Heat)
	}
	if r.State.Day != 22 {
		t.Fatalf("curtailed day did not advance: day %d", r.State.Day)
	}
	// Day 23 is outside the two-day window; the offer is gone, the mode is not.
	r.State.Day = 23
	if res := resOnly(r.Apply(Action{Kind: ActStay, Mode: StayCurtail})); res.Reason != ReasonCurtailUnavailable {
		t.Fatalf("curtail outside window = %v, want %v", res.Reason, ReasonCurtailUnavailable)
	}
	// Power off spends the day with no payout and no meter.
	r.State.Day = 21
	before = r.State.Cash
	if res := resOnly(r.Apply(Action{Kind: ActStay, Mode: StayPowerOff})); !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.Cash != before-250 {
		t.Fatalf("power-off cash delta %s, want just the lease", FormatMoney(r.State.Cash-before))
	}
}

func TestClawbackFollowsEnterpriseHostingOnly(t *testing.T) {
	// A consumer night leaves nothing for the reviewers to reach for.
	r := fullRun(t, 31, content.Ashburn, 13, 500_000)
	give(r, content.Card4090, 4)
	if res := resOnly(r.Apply(Action{Kind: ActStay})); !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.LastHostEnt != 0 {
		t.Fatalf("consumer night stamped enterprise gross %s", FormatMoney(r.State.LastHostEnt))
	}
	if got := r.clawbackDue(); got != 0 {
		t.Fatalf("clawback after a consumer night = %s, want 0", FormatMoney(got))
	}

	// An enterprise night stamps the number the reviewers can reach for,
	// capped at the 2500 the disclosure names.
	r = fullRun(t, 32, content.Ashburn, 13, 1_000_000)
	give(r, content.CardH100, 10) // 10 x 350 = 3500 enterprise gross
	if res := resOnly(r.Apply(Action{Kind: ActStay})); !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.LastHostEnt != 3500 {
		t.Fatalf("LastHostEnt = %s, want 3500", FormatMoney(r.State.LastHostEnt))
	}
	if got := r.clawbackDue(); got != 2500 { // 100% of 3500, capped
		t.Fatalf("clawback = %s, want the 2500 cap", FormatMoney(got))
	}
	before, debt := r.State.Cash, r.State.Debt
	// Travel charges the clawback on the way out and clears the stamp. The
	// garage would not fit ten H100s, and an undersized lease blocks the
	// road, so the exit goes to West Texas.
	rec, res := r.Apply(Action{Kind: ActTravel, Market: content.WestTexas})
	if !res.OK {
		t.Fatal(res.Reason)
	}
	if rec.Clawback != 2500 {
		t.Fatalf("travel receipt clawback %s, want 2500", FormatMoney(rec.Clawback))
	}
	if r.State.LastHostEnt != 0 {
		t.Fatalf("travel left the review stamp %s lit", FormatMoney(r.State.LastHostEnt))
	}
	spent := before - r.State.Cash + r.State.Debt - debt
	if want := Money(2500) + rec.Freight; spent != want {
		t.Fatalf("travel cost %s, want clawback + freight %s", FormatMoney(spent), FormatMoney(want))
	}

	// Staying charges it too, against last night's number, then re-stamps.
	r = fullRun(t, 33, content.Ashburn, 13, 1_000_000)
	give(r, content.CardH100, 1) // 350 gross, under the cap
	if res := resOnly(r.Apply(Action{Kind: ActStay})); !res.OK {
		t.Fatal(res.Reason)
	}
	rec = Receipt{}
	res = Result{}
	rec, res = r.Apply(Action{Kind: ActStay})
	if !res.OK || rec.Clawback != 350 {
		t.Fatalf("stay clawback %s (%v), want 350", FormatMoney(rec.Clawback), res.Reason)
	}
	if r.State.LastHostEnt != 350 {
		t.Fatalf("re-stamp LastHostEnt = %s, want tonight's 350", FormatMoney(r.State.LastHostEnt))
	}
}

func TestAllocationHeatGateAndOncePerRun(t *testing.T) {
	r := fullRun(t, 41, content.TheDalles, 10, 1_000_000)
	off := r.PreviewAllocation()
	if !off.Eligible || off.Product != content.CardH100 || off.Pct != 85 || off.Stock != 3 {
		t.Fatalf("day-10 Dalles offer = %+v, want 3 H100 at 85%%", off)
	}
	if off.UnitPrice != off.Ask*85/100 || off.UnitPrice > off.Ask {
		t.Fatalf("unit price %s is not 85%% of ask %s, floored", FormatMoney(off.UnitPrice), FormatMoney(off.Ask))
	}
	// The heat gate: at 60 the counter is closed to a hot rack.
	r.State.Heat = 60
	if off := r.PreviewAllocation(); off.Eligible {
		t.Fatal("a hot rack was served below-street supply")
	}
	if res := resOnly(r.Apply(Action{Kind: ActAllocation, Qty: 1})); res.Reason != ReasonAllocationBlocked {
		t.Fatalf("hot allocation = %v, want %v", res.Reason, ReasonAllocationBlocked)
	}
	r.State.Heat = 0
	day := r.State.Day
	cost := off.UnitPrice * 2
	before := r.State.Cash
	rec, res := r.Apply(Action{Kind: ActAllocation, Qty: 2})
	if !res.OK {
		t.Fatal(res.Reason)
	}
	if r.State.Day != day || r.State.Cash != before-cost || r.OwnedOf(content.CardH100) != 2 {
		t.Fatalf("allocation landed %s for 2 units (day %d): %+v", FormatMoney(before-r.State.Cash), r.State.Day, rec)
	}
	// Once per run: the remaining unit is someone else's now.
	if off := r.PreviewAllocation(); off.Eligible {
		t.Fatal("a second allocation was offered after the first was taken")
	}
	if res := resOnly(r.Apply(Action{Kind: ActAllocation, Qty: 1})); res.Reason != ReasonNotForSale {
		t.Fatalf("second allocation = %v, want %v", res.Reason, ReasonNotForSale)
	}
	// Stock never refills mid-window, and the window itself ends.
	if r.State.AllocStock != 1 {
		t.Fatalf("stock after taking 2 of 3 = %d, want 1 (no refill)", r.State.AllocStock)
	}
	r.State.Day = 13 // the window ran days 10..12; day 13 the counter is dark
	r.refreshMarket(13)
	if off := r.PreviewAllocation(); off.Eligible || off.Stock != 0 {
		t.Fatalf("day-13 offer = %+v, want the window closed and empty", off)
	}
	// A fresh window does not reopen a run that already traded with Andy.
	r.State.Day = 40
	r.refreshMarket(40) // no window out of campaign; stays shut
	if off := r.PreviewAllocation(); off.Eligible {
		t.Fatal("an allocation existed outside its campaign window")
	}
}

func TestHeatGatesFeaturedWorkload(t *testing.T) {
	r := fullRun(t, 45, content.TheDalles, 25, 200_000)
	give(r, content.CardH100, 2)
	pv := r.PreviewStayMode(StayAuto, nil)
	// The strawberry window pays 125%: 350 x 1.25 = 437 per H100, floored.
	if pv.Hosted != 874 {
		t.Fatalf("featured night hosted %s, want 2 x 437", FormatMoney(pv.Hosted))
	}
	if !pv.Clean {
		t.Fatal("a cold rack is somehow dirty")
	}
	r.State.Heat = 60
	pv = r.PreviewStayMode(StayAuto, nil)
	if pv.Hosted != 700 {
		t.Fatalf("hot featured night hosted %s, want the ordinary 700", FormatMoney(pv.Hosted))
	}
	if pv.Clean {
		t.Fatal("preview calls heat 60 clean")
	}
	if pv.Valid != true {
		t.Fatal("a dirty rack was refused ordinary hosting entirely")
	}
}

// findCustomsSeed finds a seed whose first Huaqiangbei exit is stopped at the
// border, and a seed whose is not, so the test proves both sides of the coin
// without ever rigging the draw.
func findCustomsSeeds(t *testing.T) (hit, miss uint64) {
	t.Helper()
	probe := func(seed uint64) bool {
		r, err := New(Config{Seed: seed, Products: content.AllProducts, Markets: content.AllMarkets, Rules: content.FullRules()})
		if err != nil {
			t.Fatal(err)
		}
		r.State.Market = content.Huaqiangbei
		give(r, content.Mod40948, 1)
		_, res := r.Apply(Action{Kind: ActTravel, Market: content.TheDalles})
		return res.PendingEncounter
	}
	for s := uint64(1); s < 200; s++ {
		if probe(s) {
			hit = s
		} else if miss == 0 {
			miss = s
		}
		if hit != 0 && miss != 0 {
			return hit, miss
		}
	}
	t.Fatalf("no customs stop among seeds 1..200 (hit %d, miss %d): the 5%% chance must land sometimes", hit, miss)
	return 0, 0
}

func TestCustomsStopBlocksEverythingElse(t *testing.T) {
	hit, miss := findCustomsSeeds(t)
	// The quiet coin: nothing pending, the day just moves.
	r := fullRun(t, miss, content.Huaqiangbei, 1, 10_000)
	give(r, content.Mod40948, 1)
	if _, res := r.Apply(Action{Kind: ActTravel, Market: content.TheDalles}); !res.OK || res.PendingEncounter {
		t.Fatalf("quiet border: %+v", res)
	}
	// The stopped coin.
	r = fullRun(t, hit, content.Huaqiangbei, 1, 10_000)
	give(r, content.Mod40948, 1)
	// The border prices the fine on the arrival town's known bids, so the
	// number disclosed at the stop is one the player could have looked up.
	var collateral Money
	mi := r.MarketIndex(content.TheDalles)
	for pi := range content.AllProducts {
		collateral += Money(r.State.Owned[pi]) * r.quoteAt(mi, pi).Bid
	}
	fineWant := collateral / 5
	if fineWant < 200 {
		fineWant = 200
	}
	dayBefore := r.State.Day
	if _, res := r.Apply(Action{Kind: ActTravel, Market: content.TheDalles}); !res.PendingEncounter {
		t.Fatal("no stop at the border on the stopped seed")
	}
	if r.State.Day != dayBefore {
		t.Fatalf("the calendar advanced while stopped: day %d", r.State.Day)
	}
	if r.State.Market != content.TheDalles {
		t.Fatalf("arrival did not happen before the stop")
	}
	// Every other question is refused until this one is answered.
	if res := resOnly(r.Apply(Action{Kind: ActBuy, Product: content.Card4090, Qty: 1})); res.Reason != ReasonEncounterPending {
		t.Fatalf("shopping during a stop = %v, want %v", res.Reason, ReasonEncounterPending)
	}
	if res := resOnly(r.Apply(Action{Kind: ActStay})); res.Reason != ReasonEncounterPending {
		t.Fatalf("a second night during a stop = %v, want %v", res.Reason, ReasonEncounterPending)
	}
	if e := r.State.PendingEncounter; e.Kind != EncounterCustoms || e.Fine != fineWant || e.Named != content.Mod40948 {
		t.Fatalf("border terms = %+v, want fine %s naming one MOD", e, FormatMoney(fineWant))
	}
	// Paying: fine + interest, one day, exactly once.
	before, debt := r.State.Cash, r.State.Debt
	rec, res := r.ResolveEncounter(EncPay)
	if !res.OK {
		t.Fatalf("pay refused: %v", res.Reason)
	}
	if rec.Fine != fineWant || r.State.PendingEncounter != nil {
		t.Fatalf("resolution: fine %s pending=%v", FormatMoney(rec.Fine), r.State.PendingEncounter != nil)
	}
	if r.State.Day != dayBefore+1 {
		t.Fatalf("day %d, want exactly one advance", r.State.Day)
	}
	if spent := before - r.State.Cash + (r.State.Debt - debt); spent != fineWant {
		t.Fatalf("border cost %s, want the %s fine and nothing else", FormatMoney(spent), FormatMoney(fineWant))
	}
	if r.State.Stats.InterestPaid != 0 && debt == 0 && r.State.Debt == 0 {
		t.Fatal("interest charged on a debt-free day")
	}
}

func TestContestCoinDoesNotReroll(t *testing.T) {
	hit, _ := findCustomsSeeds(t)
	start := func() *Run {
		r := fullRun(t, hit, content.Huaqiangbei, 1, 10_000)
		give(r, content.Mod40948, 1)
		r.Apply(Action{Kind: ActTravel, Market: content.TheDalles})
		return r
	}
	// The answer was drawn at the border; reload is Resume of the copied
	// state, which is exactly what a save round-trip reconstructs.
	a := start()
	b := func() *Run {
		saved := start()
		r, err := Resume(saved.cfg, saved.State)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}()
	if a.State.PendingEncounter.Contest != b.State.PendingEncounter.Contest {
		t.Fatal("the contest coin flipped again across a reload")
	}
	recA, _ := a.ResolveEncounter(EncContest)
	recB, _ := b.ResolveEncounter(EncContest)
	if recA.Encounter != recB.Encounter {
		t.Fatalf("reload changed the outcome:\n a: %s\n b: %s", recA.Encounter, recB.Encounter)
	}
	if a.State.Cash != b.State.Cash || a.State.Debt != b.State.Debt || a.State.Day != b.State.Day {
		t.Fatal("reloaded resolution drifted from the uninterrupted one")
	}
}

func TestSurrenderTakesTheNamedUnitOnly(t *testing.T) {
	hit, _ := findCustomsSeeds(t)
	r := fullRun(t, hit, content.Huaqiangbei, 1, 10_000)
	give(r, content.Mod40948, 1)
	r.Apply(Action{Kind: ActTravel, Market: content.TheDalles})
	e := r.State.PendingEncounter
	named, owned := e.Named, r.OwnedOf(e.Named)
	cashBefore, fine := r.State.Cash, e.Fine
	rec, res := r.ResolveEncounter(EncSurrender)
	if !res.OK {
		t.Fatal(res.Reason)
	}
	if r.OwnedOf(named) != owned-1 || r.OwnedOf(content.Mod40948) != owned-1 {
		t.Fatalf("surrender took %+v, want exactly one %s", r.State.Owned, named)
	}
	if r.State.Cash != cashBefore || r.State.Debt != 0 {
		t.Fatalf("surrender charged cash: cash %s debt %s", FormatMoney(r.State.Cash), FormatMoney(r.State.Debt))
	}
	if rec.Surrendered != named || rec.Fine != 0 {
		t.Fatalf("surrender receipt %+v, want the named card and no fine", rec)
	}
	if r.State.Stats.ForcedUnits != 1 {
		t.Fatalf("ForcedUnits %d, want 1 (the fine %s was never paid)", r.State.Stats.ForcedUnits, FormatMoney(fine))
	}
}

func TestFineShortfallBecomesDebt(t *testing.T) {
	hit, _ := findCustomsSeeds(t)
	r := fullRun(t, hit, content.Huaqiangbei, 1, 1) // one dollar: barely a coat
	give(r, content.Mod40948, 1)
	r.Apply(Action{Kind: ActTravel, Market: content.TheDalles})
	fine := r.State.PendingEncounter.Fine
	_, res := r.ResolveEncounter(EncPay)
	if !res.OK {
		t.Fatal(res.Reason) // there is always a legal answer
	}
	if r.State.Cash != 0 || r.State.Debt < fine-1 {
		t.Fatalf("fine shortfall: cash %s debt %s, want the unpaid %s lent",
			FormatMoney(r.State.Cash), FormatMoney(r.State.Debt), FormatMoney(fine-1))
	}
	if r.State.Stats.InterestPaid <= 0 {
		t.Fatal("the lent fine escaped the one daily interest strike")
	}
}

func TestEncountersAndPrototypeRulesStayAsleep(t *testing.T) {
	// A hot, full garage for a week under full rules must hear at most the
	// noise complaint — and a prototype run must hear nothing ever.
	r := fullRun(t, 61, content.Garage, 2, 500_000)
	give(r, content.Mod40948, 8)
	r.State.Heat = 100
	for d := 2; d <= 8; d++ {
		if _, res := r.Apply(Action{Kind: ActStay, Mode: StayPowerOff}); !res.OK {
			t.Fatalf("stay day %d: %v", d, res.Reason)
		}
		if r.State.PendingEncounter != nil {
			if r.State.PendingEncounter.Kind != EncounterNoise {
				t.Fatalf("garage produced %v", r.State.PendingEncounter.Kind)
			}
			r.ResolveEncounter(EncPay)
		}
	}
	// Prototype: full fleet, hot intent, zero encounters by ruleset. The
	// note it opens with is not repaid by free garage stays, so the run may
	// end in repossession — what must never appear is a police beat.
	p, err := New(DefaultPrototypeConfig(62))
	if err != nil {
		t.Fatal(err)
	}
	p.State.Heat = 100
	stayed := 0
	for d := 1; d <= 10; d++ {
		_, res := p.Apply(Action{Kind: ActStay})
		if p.State.PendingEncounter != nil {
			t.Fatalf("prototype encountered something: %+v", p.State.PendingEncounter)
		}
		if !res.OK {
			if res.Reason != ReasonRunOver || p.State.Over != EndRepossessed {
				t.Fatalf("prototype stay refused outside an ending: %v", res.Reason)
			}
			break
		}
		stayed++
	}
	if stayed == 0 {
		t.Fatal("prototype never stayed a night")
	}
}

func TestTraceCodecPreservesPhase5Actions(t *testing.T) {
	cfg := Config{Seed: 7, Products: content.AllProducts, Markets: content.AllMarkets, Rules: content.FullRules()}
	acts := []Action{
		{Kind: ActStay, Mode: StayPowerOff},
		{Kind: ActStay, Mode: StayCurtail},
		{Kind: ActStay, Powered: []int{1, 0, 0, 0, 2, 0}},
		{Kind: ActAllocation, Qty: 2},
		{Kind: ActEncounter, Resp: EncContest},
	}
	raw, err := EncodeTrace(Trace{Config: cfg, Actions: acts})
	if err != nil {
		t.Fatal(err)
	}
	tr, err := DecodeTrace(raw)
	if err != nil {
		t.Fatalf("phase 5 steps refused their own encoding: %v", err)
	}
	if len(tr.Actions) != len(acts) {
		t.Fatalf("lost steps in the round trip")
	}
	if tr.Actions[1].Mode != StayCurtail || tr.Actions[2].Powered[4] != 2 || tr.Actions[4].Resp != EncContest {
		t.Fatalf("round trip lost fields: %+v", tr.Actions)
	}
	// A trace that names impossible extras is refused at the door, before
	// anyone plays it.
	for _, bad := range []Action{
		{Kind: ActStay, Mode: StayMode(9)},
		{Kind: ActStay, Powered: []int{1, 2}},
		{Kind: ActEncounter, Resp: EncounterResponse(9)},
	} {
		raw, err := EncodeTrace(Trace{Config: cfg, Actions: []Action{bad}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeTrace(raw); err == nil {
			t.Fatalf("decode accepted %+v", bad)
		}
	}
}
