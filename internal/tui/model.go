package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"rackwars/internal/content"
	"rackwars/internal/game"
	"rackwars/internal/save"
)

// Terminal size contract from the design document: 80x24 is the minimum
// playable size, and at WideWidth and beyond a side panel appears.
const (
	MinWidth  = 80
	MinHeight = 24
	WideWidth = 110
)

type mode int

const (
	modeBoard mode = iota
	modeQty
	modeHelp
	modeLender
	modeTravel
	modeConfirm
	modeEnd
	// modeContinue is the start-up list of saved runs. It is the only mode that
	// exists before a run is on screen, and it answers to nothing but its own
	// keys, so a stray Enter can never start or continue a run by accident.
	modeContinue
	// modeStay is the full game's night plan: host the best subset, put the
	// GPUs to bed, take the grid operator's money, or pick the powered cards
	// by hand. The prototype has no such screen; its stay is the plain
	// confirmation, exactly as the recorded traces play it.
	modeStay
	// modeSubset is the powered-card editor inside the night plan.
	modeSubset
	// modeAlloc is Andy's allocation window: constrained supply at a
	// disclosed discount, once a run, heat-gated.
	modeAlloc
)

type tradeAction int

const (
	actionBuy tradeAction = iota
	actionSell
)

type loanAction int

const (
	loanBorrow loanAction = iota
	loanRepay
)

// ProductRow is one market line as the board displays it.
type ProductRow struct {
	Product content.ProductID
	Name    string
	Ask     game.Money
	Bid     game.Money
	Stock   int
	Owned   int
	Slots   int
	// KW100 is the switched-on draw in hundredths of a kilowatt, Host is the
	// ordinary per-night hosting the card earns when it runs, and Class is
	// the one-word trading personality: CONsumer, ENTerprise, or MODded.
	// All three come straight from the content definition.
	KW100 int64
	Host  game.Money
	Class string
}

// Board is the board screen's read of the engine. Every field is copied from
// engine state or an engine preview, so the view recomputes no price, limit, or
// cost. It is rebuilt for each frame; it is never cached in the model, which is
// what keeps a stale quote from ever reaching the screen.
type Board struct {
	Tag        string
	Day        int
	DayTotal   int
	Town       string
	Cash       game.Money
	Debt       game.Money
	NetWorth   game.Money
	Heat       int
	SlotsUsed  int
	SlotsTotal int
	Headline   string
	Warning    string
	Rows       []ProductRow
	Hosting    string
	StayCost   string
	TravelCost string
	Side       []string
	// Full marks the five-market game. The extra lines below are only ever
	// filled in that mode, so the prototype board keeps its exact phase-3
	// layout — and its exact tests.
	Full bool
	// Power is the meter read for tonight's auto plan: what runs, on what
	// wire. Curtail is the grid operator's standing offer, Alloc Andy's
	// window; both are empty when nothing is on the table.
	Power   string
	Curtail string
	Alloc   string
}

// Model is the Bubble Tea model. run is the one authority on game state; the
// model holds only presentation intent (which box has focus, what is typed)
// plus the seed the run was dealt.
type Model struct {
	chrome Chrome
	run    *game.Run
	seed   uint64

	width  int
	height int
	mode   mode
	sel    int

	status    string
	statusBad bool

	qtyAction tradeAction
	qtyDraft  string

	loanAction loanAction
	loanDraft  string

	travelSel int

	// The full game's night plan. stayMode is the highlighted plan, stayPick
	// the counts being edited in the powered-card screen, and stayPowered the
	// subset accepted from that editor — nil means the engine's own best plan.
	full        bool
	stayMode    game.StayMode
	stayPick    []int
	stayPowered []int
	staySel     int
	allocDraft  string

	// Saving. store/runID identify where this run lives; actions is the history
	// of every action the engine accepted this run, which is what a save replays
	// to prove it is honest. lock is this instance's claim on the run, saveErr is
	// the last autosave failure (empty when the board on screen is on disk), and
	// quitNote is what the CLI prints after the screen is restored.
	store    *save.Store
	runID    string
	lock     *save.Lock
	actions  []game.Action
	savedTo  string
	saveErr  string
	quitNote string
	// restart is set when the player deliberately started a seed that already has
	// a save; that write is allowed to move the run backwards, because the old run
	// is the one being replaced, and the store keeps it in a .bak copy.
	restart bool
	// released records that this window gave up its claim, so a quit does not
	// leave a marker behind.
	released bool

	// The continue list, shown at start-up when the save folder holds a run.
	saves   []save.Entry
	saveSel int

	// dayLog holds the receipt lines from the most recent committed day: the
	// wire that moved, the interest charged, freight paid, and any forced sale.
	dayLog []string
}

// NewRun starts a trading-prototype run on the given seed: the phase-3
// game, the exact economy the recorded traces replay. The seed is what a
// player reports for a bug, and what "Replay This Seed" re-deals.
func NewRun(chrome Chrome, seed uint64) (Model, error) {
	return newRunAt(chrome, seed, false)
}

// NewFullRun starts the complete five-market game on the given seed.
func NewFullRun(chrome Chrome, seed uint64) (Model, error) {
	return newRunAt(chrome, seed, true)
}

func newRunAt(chrome Chrome, seed uint64, full bool) (Model, error) {
	run, err := game.New(dealConfig(seed, full))
	if err != nil {
		return Model{}, err
	}
	m := New(chrome, run)
	m.full = full
	return m, nil
}

// dealConfig deals one seed the cast and ruleset of its game.
func dealConfig(seed uint64, full bool) game.Config {
	if full {
		return game.Config{
			Seed: seed, Products: content.AllProducts,
			Markets: content.AllMarkets, Rules: content.FullRules(),
		}
	}
	return game.Config{
		Seed: seed, Products: content.PrototypeProducts,
		Markets: content.PrototypeMarkets, Rules: content.PrototypeRules(),
	}
}

// New wraps an existing run, which lets a test (or a resumed save) drive a
// model over engine state it prepared itself. The game it shows is the game
// the run was dealt: hosting on means the five-market game, whatever seed or
// save file it came out of.
func New(chrome Chrome, run *game.Run) Model {
	return Model{chrome: chrome, run: run, seed: run.Config().Seed,
		full: run.Config().Rules.Hosting}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. Only key handling touches the run, and it does so
// through one applied action at a time; resizing never advances time and never
// mutates run state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.releaseSave()
			return m, tea.Quit
		}
		// The continue list is a gate: until the player chooses, there is no run
		// to draw and nothing to play.
		if m.mode == modeContinue {
			return m.handleContinueKey(msg)
		}
		// A police stop owns the screen the same way: the calendar is standing
		// still until the answer is given, and no other key may touch the run.
		// This covers the stop that just fired and the reload that lands on one.
		if m.run != nil && m.run.State.PendingEncounter != nil {
			return m.handleEncounterKey(msg)
		}
		// A finished run answers only to the ending screen's keys.
		if m.over() {
			m.mode = modeEnd
			return m.handleEndKey(msg)
		}
		switch m.mode {
		case modeEnd:
			return m.handleEndKey(msg)
		case modeHelp:
			return m.handleHelpKey(msg)
		case modeQty:
			return m.handleQtyKey(msg)
		case modeLender:
			return m.handleLenderKey(msg)
		case modeTravel:
			return m.handleTravelKey(msg)
		case modeConfirm:
			return m.handleConfirmKey(msg)
		case modeStay:
			return m.handleStayKey(msg)
		case modeSubset:
			return m.handleSubsetKey(msg)
		case modeAlloc:
			return m.handleAllocKey(msg)
		default:
			return m.handleBoardKey(msg)
		}
	}
	return m, nil
}

// over reports whether the engine has closed this run.
func (m Model) over() bool { return m.run != nil && m.run.State.Over != game.EndOpen }

// startRun replaces the run in place, which is how New Run and Replay This Seed
// work: same code path, different seed.
func (m Model) startRun(seed uint64) (Model, error) {
	run, err := game.New(dealConfig(seed, m.full))
	if err != nil {
		return m, err
	}
	if m.store != nil && m.seed != seed {
		m.releaseSave()
		m.runID = "seed-" + strconv.FormatUint(seed, 10)
	}
	m.run = run
	m.seed = seed
	m.mode = modeBoard
	m.sel = 0
	m.status = ""
	m.statusBad = false
	m.dayLog = nil
	m.qtyDraft = ""
	m.loanDraft = ""
	m.travelSel = 0
	m.stayMode, m.stayPick, m.stayPowered, m.staySel, m.allocDraft =
		game.StayAuto, nil, nil, 0, ""
	// A new run starts with an empty history and a clean save slate. If the slot
	// already holds a run for this seed, the player asked for this: the write that
	// replaces it is allowed, and the store keeps what was there in a .bak copy.
	m.actions = nil
	m.saveErr = ""
	m.savedTo = ""
	m.saves = nil
	if m.store != nil {
		m.restart = true
	}
	return m, nil
}

// snapshot reads the engine for the frame. It calls only read-only engine
// accessors and pure previews: no randomness, no game time.
func (m Model) snapshot() Board {
	r := m.run
	b := Board{Tag: "trading prototype", DayTotal: content.DayCount, Hosting: "off (prototype)"}
	if r == nil {
		return b
	}
	st := r.State
	b.Full = m.full
	b.Day = st.Day
	b.Town = content.MarketOf(st.Market).Name
	b.Cash = st.Cash
	b.Debt = st.Debt
	b.NetWorth = r.NetWorth()
	b.Heat = st.Heat
	b.SlotsUsed = r.SlotsUsed()
	b.SlotsTotal = content.MarketOf(st.Market).Slots

	// The board carries today's wire plus the warning for the coming close. A day
	// outside the campaign (day 30 has no day 31) simply has no wire.
	if hl := content.HeadlineFor(st.Day); hl != nil {
		b.Headline = hl.Text
		b.Warning = hl.Tomorrow
	}

	for _, row := range r.QuoteBoard() {
		def := content.ProductOf(row.Product)
		class := "CON"
		if def.Enterprise {
			class = "ENT"
		}
		if row.Product == content.Mod40948 {
			class = "MOD"
		}
		b.Rows = append(b.Rows, ProductRow{
			Product: row.Product,
			Name:    row.Name,
			Ask:     row.Ask,
			Bid:     row.Bid,
			Stock:   row.Stock,
			Owned:   row.Owned,
			Slots:   row.Slots,
			KW100:   def.PoweredKW,
			Host:    game.Money(def.GrossHostingPerDay),
			Class:   class,
		})
	}

	stay := r.PreviewStay()
	b.StayCost = "-" + game.FormatMoney(stay.Interest+stay.Lease)
	if b.Full {
		b.Tag = "five markets"
		b.Hosting = "tonight: host +" + game.FormatMoney(stay.Hosted) +
			"  electric -" + game.FormatMoney(stay.Electric) +
			"  lease -" + game.FormatMoney(stay.Lease)
		b.Power = kwStr(stay.PowerKW) + " of " + kwStr(stay.PowerCap) +
			" kW on the meter"
		if curtail := r.PreviewStayMode(game.StayCurtail, nil); curtail.Valid {
			b.Curtail = "grid operator will pay " + game.FormatMoney(curtail.Curtail) +
				" for a silent night — [p]lan the night"
		}
		if alloc := r.PreviewAllocation(); alloc.Product != "" {
			b.Alloc = "ANDY'S ALLOCATION: " + itoa(alloc.Stock) + " " +
				content.ProductOf(alloc.Product).Name + " @" + itoa(alloc.Pct) +
				"% of ask (" + game.FormatMoney(alloc.UnitPrice) + ") — [a]"
			if !alloc.Eligible {
				b.Alloc += " [locked: " + alloc.BlockReason + "]"
			}
		}
	}
	if dest := m.destMarket(); dest != "" {
		pv := r.PreviewTravel(dest)
		b.TravelCost = "-" + game.FormatMoney(pv.Freight) + " to " + content.MarketOf(dest).Name
	} else {
		b.TravelCost = "nowhere else to go"
	}
	b.Side = m.sideLines()
	return b
}

// otherMarkets lists the towns this run's cast can reach, always in content
// order so the list is stable across runs and terminals. The list comes from
// the run's own config: the prototype sees its three towns and the full game
// sees all five, with no branch about which game is being played.
func (m Model) otherMarkets() []content.MarketID {
	if m.run == nil {
		return nil
	}
	out := make([]content.MarketID, 0, len(m.run.Markets()))
	for _, id := range m.run.Markets() {
		if id != m.run.State.Market {
			out = append(out, id)
		}
	}
	return out
}

func (m Model) destMarket() content.MarketID {
	opts := m.otherMarkets()
	if m.travelSel < 0 || m.travelSel >= len(opts) {
		return ""
	}
	return opts[m.travelSel]
}

func itoa(n int) string { return strconv.Itoa(n) }

// parseQuantity reads the typed quantity. Only bare digits count; anything else
// is not a number the player meant.
func parseQuantity(s string) (int, bool) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	n := 0
	for _, c := range []byte(s) {
		n = n*10 + int(c-'0')
	}
	return n, true
}
