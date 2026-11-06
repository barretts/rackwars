package content

// The headline campaign is the learnable part of a run: the arc of the month is
// fixed, while prices, stock, and shock strength vary with the seed. Major
// shocks are always warned the morning before they land, in plain language.

// EffectKind selects what an effect changes.
type EffectKind int

const (
	// EffectPriceFactor multiplies the mid price of the affected cards.
	EffectPriceFactor EffectKind = iota
	// EffectStock scales the shop's replenished quantity (Pct 0 means the
	// card is unavailable for the duration).
	EffectStock
	// EffectHosting multiplies the hosting payout of the affected cards for
	// its duration: this is the featured workload window. The featured
	// payout is the one thing hosting has that trading does not, which is
	// why every featured window also lifts the card's resale price.
	EffectHosting
	// EffectCurtailment is the grid operator's disclosed offer: one night
	// switched off at the affected site pays Dollars instead of hosting.
	// The offer is bounded, named in the morning board, and compared with
	// the hosting preview so the player can choose with both numbers open.
	EffectCurtailment
	// EffectAllocation is the limited below-street offer (Pct of the local
	// ask, Dollars units in total): allocation Andy's constrained supply.
	// It is an offer, never a requirement, and clean operations only: a hot
	// rack is refused with the reason stated.
	EffectAllocation
	// EffectClawback reinterprets a hosting invoice after the fact: a stay
	// hosted with a card in the effect's class pays back Pct percent of the
	// previous night's payout on the following night's close, capped at
	// Dollars. A contract that can be revisited must say so the morning
	// before, and it does: the warning line carries the review.
	EffectClawback
)

// DemandClass narrows an effect to consumer or enterprise cards. An empty
// class covers every card the effect names.
type DemandClass string

const (
	ClassAll        DemandClass = ""
	ClassConsumer   DemandClass = "consumer"
	ClassEnterprise DemandClass = "enterprise"
)

// Effect is one dated, bounded piece of market mechanics. Pct is a percentage:
// 100 leaves a value alone, 250 triples it. Duration is in days counting the
// day the effect starts, so a two-night rush has Days 2. Dollars carries the
// whole-dollar amount for the kinds that pay or cap rather than scale
// (curtailment's offer, allocation's units, clawback's cap); it must be zero
// for every other kind, and Validate holds it to that rule.
type Effect struct {
	Kind     EffectKind
	Markets  []MarketID  // empty: every market in the run
	Products []ProductID // empty: every product in the run
	Class    DemandClass
	Pct      int
	Days     int
	Dollars  int
	// RequiresClean marks the offers that turn away a hot rack: at 60 heat
	// the engine skips these effects with the reason stated, which is the
	// design's "clean premium contracts and new allocation deals are
	// unavailable", while ordinary buying and selling stay open. Only
	// hosting and allocation effects may carry it.
	RequiresClean bool
}

// Headline is one day of the campaign. Tomorrow is the warning shown on the
// previous morning's board; it describes the risk in plain language, because a
// joke that hides a rule is a defect, not flavour.
type Headline struct {
	Day      int
	Text     string
	Tomorrow string
	Effects  []Effect
}

// campaign is the thirty-day anchor list. Entries apply from their Day until
// Day+effect.Days-1, so a shock can outlive its headline.
var campaign = []Headline{
	{Day: 1, Text: "Crypto looks tired; used-card listings pile up",
		Tomorrow: "Deal sites say the used-card aisle is full. Cheap days may be coming.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Products: []ProductID{Sled3090}, Pct: 95, Days: 3}}},
	{Day: 2, Text: "A warehouse liquidation nobody asked for",
		Tomorrow: "A merger announcement tomorrow is expected to call GPUs finished. Used-card prices are likely to fall.",
		Effects:  []Effect{{Kind: EffectStock, Class: ClassConsumer, Pct: 120, Days: 3}}},
	{Day: 3, Text: "THE MERGE: GPUs ARE OVER",
		Tomorrow: "Keynote vendors are already talking about demand coming back.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Products: []ProductID{Sled3090}, Pct: 45, Days: 3},
			{Kind: EffectPriceFactor, Markets: []MarketID{WestTexas}, Products: []ProductID{Sled3090}, Pct: 30, Days: 3},
		}},
	{Day: 4, Text: "Nobody is buying cards, says every keynote",
		Tomorrow: "Somebody is always buying cards eventually.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Products: []ProductID{Sled3090}, Pct: 50, Days: 2}}},
	{Day: 5, Text: "Used-card listings thin out",
		Tomorrow: "A chatbot launch is expected to create demand for ordinary consumer cards, in resale and in hosting alike.",
		Effects:  nil},
	{Day: 6, Text: "CHATBOT NEEDS EVERY CARD YOU JUST SOLD",
		Tomorrow: "Consumer demand usually fades within a few days.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassConsumer, Pct: 150, Days: 2},
			{Kind: EffectStock, Class: ClassConsumer, Pct: 55, Days: 2},
			{Kind: EffectHosting, Pct: 200, Days: 2},
		}},
	{Day: 7, Text: "Every small model needs a GPU",
		Tomorrow: "Consumer-card demand is still elevated but will not stay.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Class: ClassConsumer, Pct: 135, Days: 2}}},
	{Day: 8, Text: "Inference costs are the new rounding error",
		Tomorrow: "Enterprise allocation is expected to tighten.",
		Effects:  nil},
	{Day: 9, Text: "Cloud capacity planners blame each other",
		Tomorrow: "Enterprise cards are likely to be scarce and expensive; limited allocations may appear.",
		Effects:  nil},
	{Day: 10, Text: "ALLOCATION ANDY IS SUPPLY CONSTRAINED",
		Tomorrow: "Allocation windows close as fast as they open. Below-ask H100s at The Dalles, for clean operations only (heat under 60).",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 130, Days: 3},
			{Kind: EffectStock, Class: ClassEnterprise, Pct: 50, Days: 3},
			{Kind: EffectAllocation, Markets: []MarketID{TheDalles}, Products: []ProductID{CardH100}, Pct: 85, Dollars: 3, Days: 3, RequiresClean: true},
		}},
	{Day: 11, Text: "Lead times measured in quarters",
		Tomorrow: "Trade rules are being rewritten again.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 125, Days: 2},
			{Kind: EffectStock, Class: ClassEnterprise, Pct: 60, Days: 2},
		}},
	{Day: 12, Text: "EXPORT RULES UPDATED WHILE YOUR FLIGHT WAS BOARDING",
		Tomorrow: "Modded-card spreads and customs attention usually follow the rules.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Products: []ProductID{Mod40948, Sled3090}, Pct: 115, Days: 3}}},
	{Day: 13, Text: "Customs brokers are hiring",
		Tomorrow: "A hosting payment is under review somewhere: if you hosted with enterprise cards, some of last night's payout can be clawed back, capped at $2,500.",
		Effects:  nil},
	{Day: 14, Text: "SAM IS OUT. SAM IS BACK. YOUR INVOICE IS UNDER REVIEW.",
		Tomorrow: "Hosting contracts can be reinterpreted after the fact.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 92, Days: 3},
			{Kind: EffectClawback, Class: ClassEnterprise, Pct: 100, Dollars: 2500, Days: 1},
		}},
	{Day: 15, Text: "Your invoice has been clarified",
		Tomorrow: "Long-context models want long-context memory.",
		Effects:  nil},
	{Day: 16, Text: "Context windows keep growing",
		Tomorrow: "Memory-heavy cards tend to get bid up when long context is the story.",
		Effects:  nil},
	{Day: 17, Text: "Attention is the scarce resource again",
		Tomorrow: "A memory-heavy launch is expected to lift premium cards, in resale and in hosting rates.",
		Effects:  nil},
	{Day: 18, Text: "STRAWBERRY THINKS LONGER. YOUR METER DOES TOO.",
		Tomorrow: "Premium-card premiums fade.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 135, Days: 2},
			{Kind: EffectPriceFactor, Products: []ProductID{Mod40948}, Pct: 140, Days: 2},
			{Kind: EffectHosting, Products: []ProductID{Mod40948, CardA100, CardH100, CardMI300}, Pct: 300, Days: 2},
		}},
	{Day: 19, Text: "Everybody needs memory, briefly",
		Tomorrow: "The grid operator is watching the weather.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 120, Days: 2}}},
	{Day: 20, Text: "Demand curves argued about in public",
		Tomorrow: "The grid operator may ask West Texas sites to stop: if you stay there, a bounded curtailment offer to keep the night switched off will be on the table.",
		Effects:  nil},
	{Day: 21, Text: "THE GRID WOULD LIKE YOU TO STOP",
		Tomorrow: "Curtailment offers are bounded and disclosed.",
		Effects:  []Effect{{Kind: EffectCurtailment, Markets: []MarketID{WestTexas}, Dollars: 8000, Days: 2}}},
	{Day: 22, Text: "Curtailment season",
		Tomorrow: "Nothing scheduled; the market is on its own.",
		Effects:  nil},
	{Day: 23, Text: "A utility apology in six fonts",
		Tomorrow: "A very large announcement is expected; customers may wait for it.",
		Effects:  nil},
	{Day: 24, Text: "STARGATE ANNOUNCES $500,000,000,000",
		Tomorrow: "A large rumor about a small model is expected to hit enterprise resale and hosting rates.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 85, Days: 4}}},
	{Day: 25, Text: "HANGZHOU HAS A SMALL MODEL AND A LARGE RUMOR",
		Tomorrow: "CheapSeek launches tomorrow. Enterprise resale and rental rates are expected to fall sharply; consider reducing debt or enterprise exposure.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 90, Days: 3},
			{Kind: EffectHosting, Products: []ProductID{Mod40948, CardA100, CardH100, CardMI300}, Pct: 125, Days: 2, RequiresClean: true},
		}},
	{Day: 26, Text: "CHEAPSEEK MONDAY",
		Tomorrow: "CheapSeek pricing holds one more day; the lender checks collateral every morning.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 50, Days: 2},
			{Kind: EffectHosting, Products: []ProductID{Sled3090, Card4090, Mod40948}, Pct: 300, Days: 2},
			{Kind: EffectHosting, Class: ClassEnterprise, Pct: 50, Days: 2},
		}},
	{Day: 27, Text: "Small models, large collateral problems",
		Tomorrow: "Keynote season is not over.",
		Effects:  []Effect{{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 60, Days: 2}}},
	{Day: 28, Text: "Enterprise resale volume spikes",
		Tomorrow: "A final keynote is expected to reprice older enterprise cards; the effect will be shown before your last decision.",
		Effects:  nil},
	{Day: 29, Text: "THE MORE YOU BUY, THE MORE YOU SAVE",
		Tomorrow: "Tomorrow is the closing auction: whatever you hold is sold at the displayed bids.",
		Effects: []Effect{
			{Kind: EffectPriceFactor, Class: ClassEnterprise, Pct: 110, Days: 2},
			{Kind: EffectPriceFactor, Class: ClassConsumer, Pct: 90, Days: 2},
		}},
	{Day: 30, Text: "LAST CALL FOR YOUR LITTLE CLOUD",
		Tomorrow: "",
		Effects:  nil},
}

// DayCount is the length of a run.
const DayCount = 30

// Campaign returns the campaign rows for days 1..DayCount.
func Campaign() []Headline { return campaign }

// HeadlineFor returns the campaign row for a day, or nil outside the run.
func HeadlineFor(day int) *Headline {
	if day < 1 || day > DayCount {
		return nil
	}
	for i := range campaign {
		if campaign[i].Day == day {
			return &campaign[i]
		}
	}
	return nil
}

// CampaignFor returns the campaign pruned to a product cast. Headline rows stay
// so the arc remains learnable; effects that name no card in the cast are
// dropped, which is how hosting-era copy behaves in the trading prototype.
func CampaignFor(cast []ProductID) []Headline {
	known := make(map[ProductID]bool, len(cast))
	for _, p := range cast {
		known[p] = true
	}
	out := make([]Headline, 0, len(campaign))
	for _, h := range campaign {
		row := h
		row.Effects = nil
		for _, e := range h.Effects {
			if len(e.Products) == 0 {
				// A class-wide effect still needs a card in the cast that
				// belongs to that class, or it prices nothing.
				if classInCast(e.Class, cast) {
					row.Effects = append(row.Effects, e)
				}
				continue
			}
			var kept []ProductID
			for _, p := range e.Products {
				if known[p] {
					kept = append(kept, p)
				}
			}
			if len(kept) > 0 {
				e.Products = kept
				row.Effects = append(row.Effects, e)
			}
		}
		out = append(out, row)
	}
	return out
}

// classInCast reports whether any product in the cast belongs to the effect's
// demand class.
func classInCast(c DemandClass, cast []ProductID) bool {
	if c == ClassAll {
		return len(cast) > 0
	}
	for _, p := range cast {
		def := productDefs[p]
		if c == ClassEnterprise && def.Enterprise {
			return true
		}
		if c == ClassConsumer && def.Consumer {
			return true
		}
	}
	return false
}

// Matches reports whether this effect applies to a product at a market.
func (e Effect) Matches(m MarketID, p ProductID) bool {
	if len(e.Markets) > 0 {
		ok := false
		for _, m2 := range e.Markets {
			if m2 == m {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(e.Products) > 0 {
		ok := false
		for _, p2 := range e.Products {
			if p2 == p {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if e.Class != ClassAll {
		def := ProductOf(p)
		if e.Class == ClassEnterprise && !def.Enterprise {
			return false
		}
		if e.Class == ClassConsumer && !def.Consumer {
			return false
		}
	}
	return true
}
