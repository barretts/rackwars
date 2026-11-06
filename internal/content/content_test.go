package content

import (
	"strings"
	"testing"
)

func TestPrototypeSetValidatesUnderPrototypeRules(t *testing.T) {
	if err := Validate(PrototypeProducts, PrototypeMarkets, PrototypeRules()); err != nil {
		t.Fatalf("prototype set is invalid: %v", err)
	}
}

func TestFullSetValidatesUnderFullRules(t *testing.T) {
	if err := Validate(AllProducts, AllMarkets, FullRules()); err != nil {
		t.Fatalf("full set is invalid: %v", err)
	}
}

func TestFullRulesNeedTheFullMap(t *testing.T) {
	if err := Validate(AllProducts, PrototypeMarkets, FullRules()); err == nil {
		t.Fatal("full rules validated against the prototype map")
	}
}

// The Phase 2 gate: "a content test fails when a required field is missing, a
// town has zero capacity or power, a campaign entry has no headline, or an item
// is assigned to a category not present in the prototype."
func TestValidateRejectsBrokenTables(t *testing.T) {
	sled := ProductOf(Sled3090)
	garage := MarketOf(Garage)

	cases := []struct {
		name   string
		prods  []ProductID
		marks  []MarketID
		rules  Rules
		patchP func(*Product)
		patchM func(*Market)
		want   string
	}{
		{"missing price", []ProductID{Sled3090}, []MarketID{Garage}, PrototypeRules(),
			func(p *Product) { p.BaseMidpoint = 0 }, nil, "base midpoint 0"},
		{"missing slots per unit", []ProductID{Sled3090}, []MarketID{Garage}, PrototypeRules(),
			func(p *Product) { p.SlotsPerUnit = 0 }, nil, "0 slots per unit"},
		{"zero rack capacity", []ProductID{Sled3090}, []MarketID{Garage}, PrototypeRules(), nil,
			func(m *Market) { m.Slots = 0 }, "0 slot capacity"},
		{"zero power at a site", []ProductID{Sled3090}, []MarketID{Garage}, FullRules(), nil,
			func(m *Market) { m.PowerKW = 0 }, "0 power capacity"},
		{"product missing from the table", []ProductID{"nvidia-next"}, []MarketID{Garage}, PrototypeRules(),
			nil, nil, `unknown product "nvidia-next"`},
		{"market missing from the table", []ProductID{Sled3090}, []MarketID{"las-vegas"}, PrototypeRules(),
			nil, nil, `unknown market "las-vegas"`},
		{"no home town", []ProductID{Sled3090}, []MarketID{Ashburn}, PrototypeRules(), nil, nil,
			"exactly one home market"},
		{"customs with no Huaqiangbei", []ProductID{Sled3090}, []MarketID{Garage, Ashburn}, Rules{Encounters: true}, nil, nil,
			`encounters are enabled`},
		{"allocations with no The Dalles", []ProductID{Sled3090}, []MarketID{Garage, Ashburn}, Rules{Allocations: true}, nil, nil,
			`allocations are enabled`},
		{"town factor off the map", []ProductID{Sled3090}, []MarketID{Garage}, PrototypeRules(), nil,
			func(m *Market) { m.TownFactorPct = 190 }, "0.8-1.2x range"},
		{"powered load missing with power on", []ProductID{Sled3090}, []MarketID{Garage}, FullRules(),
			func(p *Product) { p.PoweredKW = 0 }, nil, "powered load 0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.patchP != nil {
				p := sled
				tc.patchP(&p)
				productDefs[Sled3090] = p
				defer func() { productDefs[Sled3090] = sled }()
			}
			if tc.patchM != nil {
				m := garage
				tc.patchM(&m)
				marketDefs[Garage] = m
				defer func() { marketDefs[Garage] = garage }()
			}
			err := Validate(tc.prods, tc.marks, tc.rules)
			if err == nil {
				t.Fatal("broken content passed validation")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateRejectsBrokenCampaign(t *testing.T) {
	base := Campaign()

	missingHeadline := cloneCampaign(base)
	missingHeadline[9].Text = ""
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), missingHeadline); err == nil {
		t.Fatal("campaign entry without a headline validated")
	} else if !strings.Contains(err.Error(), "no headline") {
		t.Fatalf("error %q does not mention the missing headline", err)
	}

	wrongDay := cloneCampaign(base)
	wrongDay[4].Day = 4
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), wrongDay); err == nil {
		t.Fatal("campaign entry with the wrong date validated")
	} else if !strings.Contains(err.Error(), "has day 4") {
		t.Fatalf("error %q does not mention the bad date", err)
	}

	noWarning := cloneCampaign(base)
	noWarning[8].Tomorrow = ""
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), noWarning); err == nil {
		t.Fatal("campaign day without a next-morning warning validated")
	} else if !strings.Contains(err.Error(), "no warning") {
		t.Fatalf("error %q does not mention the missing warning", err)
	}

	short := base[:20]
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), short); err == nil {
		t.Fatal("a 20-day campaign validated as complete")
	}

	unknownEffect := cloneCampaign(base)
	unknownEffect[5].Effects = []Effect{{Kind: EffectPriceFactor, Products: []ProductID{"nvidia-next"}, Pct: 90, Days: 1}}
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), unknownEffect); err == nil {
		t.Fatal("an effect naming a product that does not exist validated")
	} else if !strings.Contains(err.Error(), "outside the product table") {
		t.Fatalf("error %q does not mention the product table", err)
	}

	neverEnds := cloneCampaign(base)
	neverEnds[3].Effects = []Effect{{Kind: EffectPriceFactor, Products: []ProductID{Card4090}, Pct: 90, Days: 30}}
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), neverEnds); err == nil {
		t.Fatal("an effect active for the whole campaign validated")
	} else if !strings.Contains(err.Error(), "lasts 30 days") {
		t.Fatalf("error %q does not mention the unbounded duration", err)
	}

	// A 50%+ shock with nothing the morning before is the exact defect the
	// design forbids.
	// The morning that guards the day-26 rumor shock.
	unannounced := cloneCampaign(base)
	unannounced[25].Effects = []Effect{{Kind: EffectPriceFactor, Products: []ProductID{CardH100}, Pct: 220, Days: 2}}
	unannounced[24].Tomorrow = ""
	if err := validate(AllProducts, AllMarkets, FullRules(), unannounced); err == nil {
		t.Fatal("an unannounced 120% shock validated")
	} else if !strings.Contains(err.Error(), "campaign day 25 has no warning") {
		t.Fatalf("error %q does not name the missing warning on day 25", err)
	}

	unknownKind := cloneCampaign(base)
	unknownKind[7].Effects = []Effect{{Kind: EffectKind(99), Pct: 100, Days: 1}}
	if err := validate(PrototypeProducts, PrototypeMarkets, PrototypeRules(), unknownKind); err == nil {
		t.Fatal("an unknown effect kind validated")
	}
}

// The design's player check "no unannounced 50%+ supply shock", verified against
// the content itself rather than a single seeded run: any campaign factor move
// bigger than a halving or a doubling must be foreshadowed the day before or
// explained by that morning's own headline.
func TestEveryBigCampaignMoveIsAnnounced(t *testing.T) {
	heads := Campaign()
	factor := func(day int, m MarketID, p ProductID) int {
		pct := 100
		for _, h := range heads {
			if day < h.Day {
				continue
			}
			for _, e := range h.Effects {
				if e.Kind != EffectPriceFactor || day >= h.Day+e.Days || !e.Matches(m, p) {
					continue
				}
				pct = pct * e.Pct / 100
			}
		}
		return pct
	}

	for day := 2; day <= DayCount; day++ {
		for _, m := range AllMarkets {
			for _, p := range AllProducts {
				prev, now := factor(day-1, m, p), factor(day, m, p)
				if prev == 0 || (now*2 >= prev && now <= prev*2) {
					continue // within half-or-double: ordinary, not a shock
				}
				announced := strings.TrimSpace(heads[day-1].Text) != "" || strings.TrimSpace(heads[day-2].Tomorrow) != ""
				if !announced {
					t.Errorf("day %d %s %s factor %d -> %d is unannounced", day, m, p, prev, now)
				}
			}
		}
	}
}

// Effects must not leak outside the cast they were pruned for: the campaign an
// engine sees is only ever about the products it was handed.
func TestCampaignForPrunesForeignProducts(t *testing.T) {
	camp := CampaignFor([]ProductID{Card4090})
	if len(camp) != DayCount {
		t.Fatalf("campaign has %d rows, want %d", len(camp), DayCount)
	}
	for _, h := range camp {
		for _, e := range h.Effects {
			for _, p := range e.Products {
				if p != Card4090 {
					t.Errorf("day %d effect names %q, outside the cast", h.Day, p)
				}
			}
		}
	}
	// A consumer-only cast still gets the day and its headline. It keeps only
	// the effects it can actually price: the CheapSeek enterprise collapse is
	// pruned, while the CheapSeek consumer hosting window survives, narrowed
	// to the consumer cards in this cast.
	camp2 := CampaignFor([]ProductID{Sled3090, Card4090})
	if h := camp2[25]; h.Day != 26 || h.Text == "" || len(h.Effects) != 1 {
		t.Fatalf("day 26 row for a consumer-only cast = %+v, want the text and exactly the consumer hosting window", h)
	} else if e := h.Effects[0]; e.Kind != EffectHosting || len(e.Products) != 2 ||
		e.Products[0] != Sled3090 || e.Products[1] != Card4090 {
		t.Fatalf("day 26 survivor = %+v, want the consumer hosting window narrowed to this cast", e)
	}
}

func TestHeadlineLookups(t *testing.T) {
	if h := HeadlineFor(1); h == nil || h.Text == "" {
		t.Fatal("day 1 has no headline")
	}
	if h := HeadlineFor(0); h != nil {
		t.Fatalf("day 0 returned %+v, want none", h)
	}
	if h := HeadlineFor(DayCount + 1); h != nil {
		t.Fatalf("day %d returned %+v, want none", DayCount+1, h)
	}
	for day := 1; day <= DayCount; day++ {
		if HeadlineFor(day) == nil {
			t.Fatalf("day %d has no row", day)
		}
	}
	if got := HeadlineFor(DayCount).Tomorrow; got != "" {
		t.Fatalf("day %d promises a next morning: %q", DayCount, got)
	}
}

func TestRouteFeesAndFreight(t *testing.T) {
	if got := RouteFee(Garage, Ashburn); got != 250 {
		t.Errorf("garage->ashburn fee = %d, want 250", got)
	}
	if got := RouteFee(Huaqiangbei, Garage); got != 1000 {
		t.Errorf("huaqiangbei->garage fee = %d, want 1,000", got)
	}
	if got := PerSlotFreight; got != 50 {
		t.Errorf("per-slot freight = %d, want 50", got)
	}
}

func TestPrototypeRulesAreAPublicSwitchboard(t *testing.T) {
	if PrototypeRules() != (Rules{}) {
		t.Fatal("prototype rules are not all-off")
	}
	full := FullRules()
	if !full.Hosting || !full.Power || !full.Heat || !full.Encounters || !full.Leases || !full.Allocations {
		t.Fatalf("full rules = %+v, want every switch on", full)
	}
}

func cloneCampaign(h []Headline) []Headline {
	out := make([]Headline, len(h))
	copy(out, h)
	for i := range out {
		out[i].Effects = append([]Effect(nil), h[i].Effects...)
	}
	return out
}
