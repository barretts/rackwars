package game

import (
	"fmt"

	"rackwars/internal/content"
)

// Resume rebuilds a run from a saved configuration and state without replaying
// anything: the saved state is the resume point, so quotes and stock cannot
// reroll under the player. The checks below are what let a corrupt, edited, or
// foreign-ruleset save be refused with a sentence instead of quietly distorted:
// every invariant a live run maintains is asserted before the state is handed
// back to a caller.
func Resume(cfg Config, st State) (*Run, error) {
	if err := content.Validate(cfg.Products, cfg.Markets, cfg.Rules); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	np, nm := len(cfg.Products), len(cfg.Markets)
	if st.Market == "" {
		return nil, fmt.Errorf("state names no market")
	}
	if r := runMarketIndex(cfg, st.Market); r < 0 {
		return nil, fmt.Errorf("state is in %q, which the saved cast does not contain", st.Market)
	}
	if st.Day < 1 || st.Day > content.DayCount {
		return nil, fmt.Errorf("state is on day %d, outside 1..%d", st.Day, content.DayCount)
	}
	if len(st.Owned) != np || len(st.CostBasis) != np || len(st.Stock) != np {
		return nil, fmt.Errorf("state holds %d owned/%d cost/%d stock rows for %d products",
			len(st.Owned), len(st.CostBasis), len(st.Stock), np)
	}
	if len(st.Quotes) != np*nm {
		return nil, fmt.Errorf("state holds %d quote cells for %d towns x %d products",
			len(st.Quotes), nm, np)
	}
	if len(st.SeenDay) != nm {
		return nil, fmt.Errorf("state holds %d sighting rows for %d towns", len(st.SeenDay), nm)
	}
	for i, p := range cfg.Products {
		if st.Owned[i] < 0 {
			return nil, fmt.Errorf("%s: owned count %d is negative", p, st.Owned[i])
		}
		if st.Stock[i] < 0 {
			return nil, fmt.Errorf("%s: stock count %d is negative", p, st.Stock[i])
		}
		if st.CostBasis[i] < 0 {
			return nil, fmt.Errorf("%s: cost basis %s is negative", p, FormatMoney(st.CostBasis[i]))
		}
		if st.Owned[i] == 0 && st.CostBasis[i] != 0 {
			return nil, fmt.Errorf("%s: no units held but a cost basis of %s",
				p, FormatMoney(st.CostBasis[i]))
		}
	}
	for mi := range cfg.Markets {
		for pi := range cfg.Products {
			q := st.Quotes[mi*np+pi]
			if q.Ask == 0 && q.Bid == 0 {
				continue // never seen in this town
			}
			if !(q.Bid > 0 && q.Bid < q.Ask) {
				return nil, fmt.Errorf("town %d product %s carries an impossible quote (%s ask, %s bid)",
					mi, cfg.Products[pi], FormatMoney(q.Ask), FormatMoney(q.Bid))
			}
		}
		if d := st.SeenDay[mi]; d < 0 || d > st.Day {
			return nil, fmt.Errorf("town %d is stamped day %d, which is not on or before day %d",
				mi, d, st.Day)
		}
	}
	if st.Cash < 0 {
		return nil, fmt.Errorf("cash %s is negative", FormatMoney(st.Cash))
	}
	if st.Debt < 0 {
		return nil, fmt.Errorf("debt %s is negative", FormatMoney(st.Debt))
	}
	if st.Heat < 0 {
		return nil, fmt.Errorf("heat %d is negative", st.Heat)
	}
	// Capacity is a rule of the active lease, and leases are switched off in
	// every ruleset that has a save format so far; while leases are off the
	// town's own storage is the limit, so a state that overflows it cannot have
	// come from this engine.
	if !cfg.Rules.Leases {
		limit := content.MarketOf(st.Market).Slots
		r := &Run{cfg: cfg, State: st}
		if used := r.SlotsUsed(); used > limit {
			return nil, fmt.Errorf("%s holds %d slots of hardware in %d-slot storage",
				st.Market, used, limit)
		}
	}
	switch st.Over {
	case EndOpen:
		if st.End.Kind != EndOpen {
			return nil, fmt.Errorf("state is still playable but carries ending kind %s", st.End.Kind)
		}
	case EndClosed, EndRepossessed:
		if st.End.Kind != st.Over {
			return nil, fmt.Errorf("state says over=%s but the ending is %s", st.Over, st.End.Kind)
		}
		if st.Day != content.DayCount && st.Over == EndClosed {
			return nil, fmt.Errorf("a closed run ended on day %d, not the closing day %d",
				st.Day, content.DayCount)
		}
	default:
		return nil, fmt.Errorf("unknown ending kind %d", int(st.Over))
	}
	if st.Stats.Commitments < 0 || st.Stats.TravelDays < 0 || st.Stats.StayDays < 0 ||
		st.Stats.ForcedUnits < 0 {
		return nil, fmt.Errorf("state carries negative run statistics")
	}

	// The state is legal. Copy the rows so the caller's slice cannot alias the
	// live run: a save file read once must not stay wired into the game.
	r := &Run{cfg: cfg, camp: content.CampaignFor(cfg.Products), State: copyState(st)}
	return r, nil
}

// runMarketIndex is the package-internal lookup, usable before a Run exists.
func runMarketIndex(cfg Config, m content.MarketID) int {
	for i, id := range cfg.Markets {
		if id == m {
			return i
		}
	}
	return -1
}

func copyState(st State) State {
	out := st
	out.Owned = append([]int(nil), st.Owned...)
	out.CostBasis = append([]Money(nil), st.CostBasis...)
	out.Stock = append([]int(nil), st.Stock...)
	out.Quotes = append([]Quote(nil), st.Quotes...)
	out.SeenDay = append([]int(nil), st.SeenDay...)
	return out
}
