package content

import "fmt"

// Rules is the switchable feature profile of the engine. The trading prototype
// runs with hosting, power, heat, encounters, leases, and allocations off, so
// the same code path settles a prototype day and a full day; what changes is
// which charges exist. Flipping a switch is a Phase 5 change, and the tests
// that assert the prototype charges no lease depend on this being explicit
// rather than incidental.
func lookupIn(ids []MarketID, want MarketID) (MarketID, bool) {
	for _, id := range ids {
		if id == want {
			return id, true
		}
	}
	return "", false
}

type Rules struct {
	Hosting     bool
	Power       bool
	Heat        bool
	Encounters  bool
	Leases      bool
	Allocations bool
}

// PrototypeRules is the Phase 2-3 profile: freight, capacity, debt, price
// shocks, and the thirty-day close are live; everything else waits.
func PrototypeRules() Rules { return Rules{} }

// FullRules is the Phase 5 profile.
func FullRules() Rules {
	return Rules{Hosting: true, Power: true, Heat: true, Encounters: true, Leases: true, Allocations: true}
}

// Validate checks a content set for internal consistency before a run starts.
// A broken table must fail loudly here, not silently distort a run.
func Validate(products []ProductID, markets []MarketID, r Rules) error {
	// The full campaign table is checked, not the cast-pruned view: an effect
	// that names a card which does not exist anywhere is a broken table, while
	// an effect that names a card outside this cast is simply pruned later.
	return validate(products, markets, r, Campaign())
}

func validate(products []ProductID, markets []MarketID, r Rules, camp []Headline) error {
	for _, id := range products {
		def, ok := LookupProduct(id)
		if !ok {
			return fmt.Errorf("unknown product %q", id)
		}
		if def.BaseMidpoint <= 0 {
			return fmt.Errorf("product %q has base midpoint %d", id, def.BaseMidpoint)
		}
		if def.SlotsPerUnit <= 0 {
			return fmt.Errorf("product %q has %d slots per unit", id, def.SlotsPerUnit)
		}
		if r.Power && def.PoweredKW <= 0 {
			return fmt.Errorf("product %q has powered load %d with power enabled", id, def.PoweredKW)
		}
		if r.Hosting && def.GrossHostingPerDay <= 0 {
			return fmt.Errorf("product %q has gross hosting %d with hosting enabled", id, def.GrossHostingPerDay)
		}
	}
	if len(products) == 0 {
		return fmt.Errorf("no products in the run")
	}
	if len(markets) == 0 {
		return fmt.Errorf("no markets in the run")
	}
	home := 0
	for _, id := range markets {
		m, ok := LookupMarket(id)
		if !ok {
			return fmt.Errorf("unknown market %q", id)
		}
		if m.Slots <= 0 {
			return fmt.Errorf("market %q has %d slot capacity", id, m.Slots)
		}
		if r.Power && m.Site && m.PowerKW <= 0 {
			return fmt.Errorf("site %q has %d power capacity with power enabled", id, m.PowerKW)
		}
		if r.Leases && m.Site && m.DailyLease < 0 {
			return fmt.Errorf("site %q has %d daily lease with leases enabled", id, m.DailyLease)
		}
		if m.TownFactorPct < 80 || m.TownFactorPct > 120 {
			return fmt.Errorf("market %q town factor %d%% is outside the 0.8-1.2x range", id, m.TownFactorPct)
		}
		if m.Home {
			home++
		}
	}
	if home != 1 {
		return fmt.Errorf("expected exactly one home market, found %d", home)
	}
	// A feature switch with no town to supply it is a broken configuration:
	// customs only exists in Huaqiangbei, allocations only in The Dalles.
	if r.Encounters {
		if _, ok := lookupIn(markets, Huaqiangbei); !ok {
			return fmt.Errorf("encounters are enabled but %q is not in this market set", Huaqiangbei)
		}
	}
	if r.Allocations {
		if _, ok := lookupIn(markets, TheDalles); !ok {
			return fmt.Errorf("allocations are enabled but %q is not in this market set", TheDalles)
		}
	}

	if len(camp) != DayCount {
		return fmt.Errorf("campaign has %d rows, want %d", len(camp), DayCount)
	}
	for i, h := range camp {
		if h.Day != i+1 {
			return fmt.Errorf("campaign row %d has day %d", i+1, h.Day)
		}
		if h.Text == "" {
			return fmt.Errorf("campaign day %d has no headline", h.Day)
		}
		// A shock the player cannot see coming is a design defect, so the
		// warning line is mandatory every morning the campaign can still
		// change prices, not only the mornings that happen to carry one.
		if h.Day < DayCount && h.Tomorrow == "" {
			return fmt.Errorf("campaign day %d has no warning for the next morning", h.Day)
		}
		for _, e := range h.Effects {
			for _, ep := range e.Products {
				if _, ok := LookupProduct(ep); !ok {
					return fmt.Errorf("campaign day %d effect names %q, which is outside the product table", h.Day, ep)
				}
			}
			for _, em := range e.Markets {
				if _, ok := LookupMarket(em); !ok {
					return fmt.Errorf("campaign day %d effect names %q, which is outside the market table", h.Day, em)
				}
			}
			switch e.Kind {
			case EffectPriceFactor:
				if e.Pct <= 0 {
					return fmt.Errorf("day %d price factor is %d%%", h.Day, e.Pct)
				}
			case EffectStock:
				if e.Pct < 0 {
					return fmt.Errorf("day %d stock factor is %d%%", h.Day, e.Pct)
				}
			default:
				return fmt.Errorf("day %d has unknown effect kind %d", h.Day, e.Kind)
			}
			if e.Days < 1 || e.Days > 10 {
				return fmt.Errorf("day %d effect lasts %d days", h.Day, e.Days)
			}
			// No separate "was this shock warned" test is needed here: every
			// day before DayCount must carry a next-morning warning line, so
			// the morning before any shock is already covered by the row check
			// above. TestEveryBigCampaignMoveIsAnnounced verifies the same
			// invariant at the price-factor level, where compounding shows.
		}
	}
	return nil
}
