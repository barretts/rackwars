// Package content holds the game's definitions: products, markets, and the
// thirty-day headline campaign. Everything here is data plus validation. No
// rule is computed in this package, and nothing here depends on a terminal,
// a file, or the clock, so a headless run and the TUI always agree.
package content

// ProductID names a commodity. The identifiers are stable content IDs: saves,
// traces, and tests refer to products by these strings, never by index.
type ProductID string

const (
	Sled3090  ProductID = "3090sled"
	Card4090  ProductID = "4090"
	Mod40948  ProductID = "4090-48"
	CardA100  ProductID = "a100"
	CardH100  ProductID = "h100"
	CardMI300 ProductID = "mi300"
)

// PrototypeProducts is the trading-prototype cast: 3090 sled, 4090, H100.
// The remaining cards join in Phase 5 without changing any rule.
var PrototypeProducts = []ProductID{Sled3090, Card4090, CardH100}

// AllProducts is the full six-card cast.
var AllProducts = []ProductID{Sled3090, Card4090, Mod40948, CardA100, CardH100, CardMI300}

// Product is one rack-ready bundle: slots and power are game abstractions, and
// the price is for the whole bundle, not one card inside it.
type Product struct {
	ID   ProductID
	Name string
	// BaseMidpoint is the unshocked mid price in whole dollars.
	BaseMidpoint int
	// SlotsPerUnit occupies storage whether or not the bundle is powered.
	SlotsPerUnit int
	// PoweredKW is drawn only while the bundle is switched on.
	PoweredKW int64
	// GrossHostingPerDay is ordinary, unmultiplied hosting income.
	GrossHostingPerDay int
	// StockBiasPct scales how much of this card a town typically shelves:
	// enterprise cards are scarce in a garage and consumer cards are not.
	StockBiasPct int
	// Enterprise marks cards whose shocks move with enterprise demand, and
	// Consumer marks the cheap cards that a volume story moves.
	Enterprise bool
	Consumer   bool
}

var productDefs = map[ProductID]Product{
	Sled3090:  {ID: Sled3090, Name: "3090 SLED", BaseMidpoint: 1400, SlotsPerUnit: 2, PoweredKW: 160, GrossHostingPerDay: 160, StockBiasPct: 100, Consumer: true},
	Card4090:  {ID: Card4090, Name: "4090", BaseMidpoint: 1800, SlotsPerUnit: 1, PoweredKW: 50, GrossHostingPerDay: 70, StockBiasPct: 110, Consumer: true},
	Mod40948:  {ID: Mod40948, Name: "4090-48 MOD", BaseMidpoint: 2800, SlotsPerUnit: 1, PoweredKW: 60, GrossHostingPerDay: 90, StockBiasPct: 60, Consumer: true},
	CardA100:  {ID: CardA100, Name: "A100", BaseMidpoint: 9000, SlotsPerUnit: 1, PoweredKW: 60, GrossHostingPerDay: 180, StockBiasPct: 30, Enterprise: true},
	CardH100:  {ID: CardH100, Name: "H100", BaseMidpoint: 32000, SlotsPerUnit: 1, PoweredKW: 90, GrossHostingPerDay: 350, StockBiasPct: 15, Enterprise: true},
	CardMI300: {ID: CardMI300, Name: "MI300", BaseMidpoint: 12000, SlotsPerUnit: 2, PoweredKW: 100, GrossHostingPerDay: 250, StockBiasPct: 25, Enterprise: true},
}

// Product returns the definition for id. Unknown ids are a programming error,
// and Validate rejects them before a run starts.
func ProductOf(id ProductID) Product { return productDefs[id] }

// LookupProduct reports whether id is a defined card.
func LookupProduct(id ProductID) (Product, bool) {
	def, ok := productDefs[id]
	return def, ok
}

// MarketID names a data-center town.
type MarketID string

const (
	Garage      MarketID = "garage"
	Ashburn     MarketID = "ashburn"
	WestTexas   MarketID = "westtexas"
	TheDalles   MarketID = "thedalles"
	Huaqiangbei MarketID = "huaqiangbei"
)

// PrototypeMarkets is the trading-prototype map: the garage plus two towns.
// The garage is the home market, not a town layered under the others.
var PrototypeMarkets = []MarketID{Garage, Ashburn, WestTexas}

// AllMarkets is the full five-market map.
var AllMarkets = []MarketID{Garage, Ashburn, WestTexas, TheDalles, Huaqiangbei}

// Market is a place with finite storage, a power budget, a daily lease, and a
// price bias. Town advantages bias prices; they never guarantee a route wins.
type Market struct {
	ID   MarketID
	Name string
	// Slots and PowerKW are the capacity of the active lease at this site.
	// PowerKW is in hundredths of a kilowatt, matching Product.PoweredKW.
	Slots   int
	PowerKW int64
	// DailyLease is charged for a stay with the GPUs off. The trading
	// prototype runs with leases switched off; the values are kept so the
	// number the player is eventually charged is the one on this sheet.
	DailyLease int
	// TownFactorPct scales mid prices: 0.8-1.2x of base, in percent.
	TownFactorPct int
	// StockMid is the typical daily replenishment quantity for a card here.
	StockMid int
	// ElecCentsPerKWh is what a powered night costs this town: a whole number
	// of cents per kilowatt-hour, always displayed before a stay is confirmed.
	// Cheap power is The Dalles' whole personality; Huaqiangbei pays city rates.
	ElecCentsPerKWh int
	// Site marks a town with a powered rack where GPUs can be kept and run.
	// Huaqiangbei is a buying/selling destination with customs risk, not a
	// site, so the power rule must not claim capacity it cannot supply.
	Site bool
	// Home reports the free starting site.
	Home bool
}

var marketDefs = map[MarketID]Market{
	Garage:      {ID: Garage, Name: "The garage", Site: true, Slots: 8, PowerKW: 200, DailyLease: 0, TownFactorPct: 100, StockMid: 14, ElecCentsPerKWh: 14, Home: true},
	Ashburn:     {ID: Ashburn, Name: "Ashburn", Site: true, Slots: 16, PowerKW: 1200, DailyLease: 800, TownFactorPct: 115, StockMid: 8, ElecCentsPerKWh: 22},
	WestTexas:   {ID: WestTexas, Name: "West Texas", Site: true, Slots: 24, PowerKW: 1000, DailyLease: 250, TownFactorPct: 85, StockMid: 18, ElecCentsPerKWh: 15},
	TheDalles:   {ID: TheDalles, Name: "The Dalles", Site: true, Slots: 16, PowerKW: 800, DailyLease: 350, TownFactorPct: 95, StockMid: 10, ElecCentsPerKWh: 8},
	Huaqiangbei: {ID: Huaqiangbei, Name: "Huaqiangbei", Slots: 8, PowerKW: 0, DailyLease: 100, TownFactorPct: 80, StockMid: 10, ElecCentsPerKWh: 30},
}

// Market returns the definition for id.
func MarketOf(id MarketID) Market { return marketDefs[id] }

// LookupMarket reports whether id is a defined town.
func LookupMarket(id MarketID) (Market, bool) {
	def, ok := marketDefs[id]
	return def, ok
}

// RouteFee is the freight fee for a one-day move, before the per-slot charge.
// Every route takes one day in the base game; Huaqiangbei costs more.
func RouteFee(from, to MarketID) int {
	if from == Huaqiangbei || to == Huaqiangbei {
		return 1000
	}
	return 250
}

// PerSlotFreight is the freight added for each occupied slot on departure.
const PerSlotFreight = 50
