package game

import "rackwars/internal/content"

// encounters.go is the police beat. Customs, serial checks, and the homeowners'
// group chat each resolve as a short choice with visible stakes, and the whole
// system holds three design promises:
//
//   - at most one encounter per day, checked in a fixed order, so a shipment
//     never eats a seizure and a fire;
//   - the outcome is drawn the moment the encounter triggers, keyed by the
//     day, so reloading and committing the same action shows the same card —
//     a reload is not a slot machine;
//   - an encounter pauses the calendar between the commitment and the next
//     morning: the freight or lease already charged stays charged, and
//     interest accrues exactly once, in the resolution that closes the day.
//
// Contesting is the bounded gamble: losing costs no more than what was
// disclosed before choosing. The fine is money — unpayable fines become debt
// like any uncovered charge, which also guarantees a legal answer exists even
// when cash is zero and the fleet is empty.

// EncounterKind names which beat pulled the player over.
type EncounterKind string

const (
	// EncounterCustoms is leaving Huaqiangbei with GPUs.
	EncounterCustoms EncounterKind = "customs"
	// EncounterSerial is the Ashburn serial-number check.
	EncounterSerial EncounterKind = "serial"
	// EncounterNoise is Denise's homeowners' complaint against a hot garage.
	EncounterNoise EncounterKind = "noise"
)

// EncounterResponse is the player's answer to an encounter.
type EncounterResponse int

const (
	// EncPay pays the posted fine.
	EncPay EncounterResponse = iota
	// EncSurrender hands over the named unit.
	EncSurrender
	// EncContest risks the disclosed loss for a disclosed chance of nothing.
	EncContest
)

func (er EncounterResponse) String() string {
	switch er {
	case EncPay:
		return "pay"
	case EncSurrender:
		return "surrender"
	case EncContest:
		return "contest"
	}
	return "?"
}

// EncounterResponseFor parses a trace response token.
func EncounterResponseFor(tok string) (EncounterResponse, bool) {
	switch tok {
	case "pay":
		return EncPay, true
	case "surrender":
		return EncSurrender, true
	case "contest":
		return EncContest, true
	}
	return 0, false
}

// Encounter is the stopped clock mid-day: what was pulled over, what it costs,
// and — drawn at trigger time, never at answer time — how the coin was already
// lying. Storing it is what lets a save sit on the choice without rerolling it.
type Encounter struct {
	Kind    EncounterKind     `json:"kind"`
	Day     int               `json:"day"`
	Town    content.MarketID  `json:"town"`
	From    content.MarketID  `json:"from,omitempty"` // customs: the border crossed
	Named   content.ProductID `json:"named,omitempty"`
	Fine    Money             `json:"fine"`
	Chance  int               `json:"chance"`  // contest win, percent, disclosed
	Contest bool              `json:"contest"` // pre-drawn: contesting would win
}

// fine draws are keyed here; day and kind make every roll a fresh question the
// seed already knew the answer to.
func (r *Run) encFineKey(k EncounterKind, d int, m content.MarketID) string {
	return "encfine|" + string(k) + "|" + itoa(d) + "|" + string(m)
}
func (r *Run) encUnitKey(k EncounterKind, d int, m content.MarketID) string {
	return "encunit|" + string(k) + "|" + itoa(d) + "|" + string(m)
}
func (r *Run) encWinKey(k EncounterKind, d int, m content.MarketID) string {
	return "encwin|" + string(k) + "|" + itoa(d) + "|" + string(m)
}

// fleetCount is every unit held; customs exempts the empty coat by rule.
func (r *Run) fleetCount() int {
	n := 0
	for _, c := range r.State.Owned {
		n += c
	}
	return n
}

// namedUnit picks the surrender card: a unit chosen by the seed's draw among
// all held units, in product order, so "one named unit" is honest — the
// player sees exactly which card is on the table before choosing.
func (r *Run) namedUnit(key string) content.ProductID {
	total := r.fleetCount()
	if total <= 0 {
		return ""
	}
	idx := r.draw(key, total)
	for pi, p := range r.cfg.Products {
		if idx < r.State.Owned[pi] && r.State.Owned[pi] > 0 {
			return p
		}
		idx -= r.State.Owned[pi]
	}
	// Unreachable while the fleet is nonempty; return the first held card.
	for pi, p := range r.cfg.Products {
		if r.State.Owned[pi] > 0 {
			return p
		}
	}
	return ""
}

// customsRoll is the disclosed chance: 5% under 60 heat, 25% at or above it,
// which is the design's doubling, not a hidden second multiplier.
func (r *Run) customsRoll() int {
	if r.State.Heat >= 60 {
		return 25
	}
	return 5
}

// rollEncounter returns the one encounter a day can hold, checked in design
// order: the border, then the serial-number plate, then the neighbors. `from`
// is the town the player left the moment the commitment locked — for a stay
// that is where they still are; for travel it is the town before the road,
// because by roll time the player has already arrived. Prototype rule sets
// never roll: the trading prototype has no encounters and no heat, and this
// stays that way until both switches say otherwise.
func (r *Run) rollEncounter(a Action, from content.MarketID) *Encounter {
	// Customs: only a departure from Huaqiangbei with GPUs counts; arriving
	// there is a shopping trip, and the empty coat is exempt.
	if r.cfg.Rules.Encounters && a.Kind == ActTravel && from == content.Huaqiangbei && a.Market != content.Huaqiangbei &&
		r.fleetCount() > 0 {
		if r.chance("customs|"+itoa(r.State.Day)+"|"+string(a.Market), r.customsRoll()) {
			mi := r.MarketIndex(a.Market)
			collateral := Money(0)
			for pi := range r.cfg.Products {
				collateral += Money(r.State.Owned[pi]) * r.quoteAt(mi, pi).Bid
			}
			fine := collateral / 5
			if fine < 200 {
				fine = 200
			}
			e := &Encounter{
				Kind: EncounterCustoms, Day: r.State.Day, Town: a.Market, From: content.Huaqiangbei,
				Fine: fine, Chance: 40,
			}
			e.Named = r.namedUnit(r.encUnitKey(e.Kind, e.Day, a.Market))
			e.Contest = r.chance(r.encWinKey(e.Kind, e.Day, a.Market), e.Chance)
			return e
		}
	}
	if a.Kind != ActStay {
		return nil
	}
	if r.cfg.Rules.Encounters && r.State.Market == content.Ashburn && r.fleetCount() > 0 {
		pct := 5
		if r.State.Heat >= 60 {
			pct = 25
		}
		if r.chance("serial|"+itoa(r.State.Day), pct) {
			collateral := r.CollateralValue()
			fine := collateral / 10
			if fine < 100 {
				fine = 100
			}
			e := &Encounter{
				Kind: EncounterSerial, Day: r.State.Day, Town: content.Ashburn,
				Fine: fine, Chance: 50,
			}
			e.Named = r.namedUnit(r.encUnitKey(e.Kind, e.Day, content.Ashburn))
			e.Contest = r.chance(r.encWinKey(e.Kind, e.Day, content.Ashburn), e.Chance)
			return e
		}
	}
	// Denise's complaint needs a hot garage and a reason: heat at 40 or
	// above means the turbines have been audible, whatever tonight's plan is.
	if r.cfg.Rules.Encounters && r.cfg.Rules.Heat && r.State.Market == content.Garage &&
		r.State.Heat >= 40 {
		if r.chance("noise|"+itoa(r.State.Day), 15) {
			e := &Encounter{
				Kind: EncounterNoise, Day: r.State.Day, Town: content.Garage,
				Fine: 300, Chance: 60,
			}
			e.Contest = r.chance(r.encWinKey(e.Kind, e.Day, content.Garage), e.Chance)
			return e
		}
	}
	return nil
}

// resolveEncounter charges the outcome of the player's answer, then hands the
// day back to dayTail so interest, the advance, and the credit check happen
// exactly once no matter which door the day closed through. The bounded-gamble
// rule holds: losing a contest costs the disclosed fine and never the fleet;
// surrender is the zero-cash answer; and a fine with no cash behind it becomes
// debt like any uncovered charge, so an answer always exists.
func (r *Run) resolveEncounter(rec Receipt, e *Encounter, resp EncounterResponse) Receipt {
	named, namedOK := e.Named, e.Named != "" && r.OwnedOf(e.Named) > 0
	switch resp {
	case EncSurrender:
		if !namedOK {
			// Surrender with nothing to surrender degrades to the fine: the
			// encounter always has a legal, disclosed answer.
			resp = EncPay
			break
		}
		bid := r.quoteAt(r.MarketIndex(r.State.Market), r.Index(named)).Bid
		r.sellUnits(r.Index(named), 1)
		r.State.Stats.ForcedUnits++
		r.State.Stats.ForcedLoss += lossVsCost(r.AvgCost(named), bid)
		rec.Surrendered = named
		rec.Encounter = "surrendered one " + content.ProductOf(named).Name
	case EncContest:
		if e.Contest {
			rec.Encounter = "contested and won: nothing was taken"
		} else {
			r.chargeFine(e.Fine, &rec)
			rec.Encounter = "contested and lost: " + FormatMoney(e.Fine) + " changed hands"
		}
		return rec
	}
	if resp == EncPay {
		r.chargeFine(e.Fine, &rec)
		if rec.Encounter == "" {
			rec.Encounter = "paid the " + FormatMoney(e.Fine) + " fine"
		}
	}
	return rec
}

// chargeFine takes the fine from cash first and lends the shortfall, the same
// uncovered-charge rule as freight: a fine the player cannot pay is debt with
// a receipt, never a blocked morning.
func (r *Run) chargeFine(fine Money, rec *Receipt) {
	if fine <= 0 {
		return
	}
	rec.Fine = fine
	if fine > r.State.Cash {
		r.State.Debt += fine - r.State.Cash
		r.State.Cash = 0
	} else {
		r.State.Cash -= fine
	}
}

// EncounterPreview is what the paused screen shows: the beat, the stake, and
// every legal answer with its exact cost, so the choice is made from numbers,
// not from a guess about the rules.
type EncounterPreview struct {
	Kind          EncounterKind     `json:"kind"`
	Title         string            `json:"title"`
	Story         string            `json:"story"`
	Named         content.ProductID `json:"named,omitempty"`
	Fine          Money             `json:"fine"`
	Chance        int               `json:"chance"`
	CanSurrender  bool              `json:"canSurrender"`
	LoseNote      string            `json:"loseNote"`
	ContestNote   string            `json:"contestNote"`
	DebtAfterFine Money             `json:"debtAfterFine,omitempty"`
}

// PreviewEncounter renders the pending encounter. The engine has already drawn
// the coin; the preview never leaks it — the notes state the rules, not the
// result, so "contest" stays a gamble at a disclosed price.
func (r *Run) PreviewEncounter() (EncounterPreview, bool) {
	e := r.State.PendingEncounter
	if e == nil {
		return EncounterPreview{}, false
	}
	p := EncounterPreview{
		Kind: e.Kind, Named: e.Named, Fine: e.Fine, Chance: e.Chance,
		DebtAfterFine: r.State.Debt,
	}
	if e.Fine > r.State.Cash {
		p.DebtAfterFine = r.State.Debt + (e.Fine - r.State.Cash)
	}
	switch e.Kind {
	case EncounterCustoms:
		p.Title = "CUSTOMS: LEAVING HUAQIANGBEI"
		p.Story = "The manifest says " + itoa(r.fleetCount()) +
			" units crossed. They would like to make this go away."
		p.CanSurrender = e.Named != "" && r.OwnedOf(e.Named) > 0
	case EncounterSerial:
		p.Title = "SERIAL CHECK"
		p.Story = "A plate reader logged a rack it does not recognize. Someone wants paperwork."
		p.CanSurrender = e.Named != "" && r.OwnedOf(e.Named) > 0
	case EncounterNoise:
		p.Title = "NOISE COMPLAINT"
		p.Story = "Denise's group chat reached the city again: the turbines again."
		p.CanSurrender = false
	}
	p.LoseNote = "losing contests costs the " + FormatMoney(e.Fine) + " fine and nothing more"
	p.ContestNote = "contest at " + itoa(e.Chance) + "% to take nothing"
	return p, true
}
