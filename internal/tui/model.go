package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"rackwars/internal/game"
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
)

type tradeAction int

const (
	actionBuy tradeAction = iota
	actionSell
)

// ProductRow is one market line as the board displays it.
type ProductRow struct {
	Name  string
	Ask   game.Money
	Bid   game.Money
	Stock int
	Owned int
	Slots int
}

// Board is everything the board screen renders. The engine produces it once the
// trading rules are wired; nothing here is computed by the view.
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
}

// Model is the Bubble Tea model. It uses value receivers so the renderer can
// never observe a half-applied update.
type Model struct {
	chrome Chrome
	width  int
	height int
	mode   mode
	board  Board
	sel    int
	status string

	qtyAction tradeAction
	qtyDraft  string
}

// New returns the initial model. board holds the current market snapshot.
func New(chrome Chrome, board Board) Model {
	return Model{chrome: chrome, board: board}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// PreviewBoard is the Phase 1 shell fixture: real design numbers for the three
// prototype products, fixed by hand so the shell renders a market-shaped board
// before any engine rules exist. Phase 3 replaces it with engine state.
func PreviewBoard() Board {
	return Board{
		Tag:        "shell preview",
		Day:        1,
		DayTotal:   30,
		Town:       "The garage",
		Cash:       10_000,
		Debt:       20_000,
		NetWorth:   -10_000,
		Heat:       0,
		SlotsUsed:  0,
		SlotsTotal: 8,
		Headline:   "Crypto looks tired; used-card listings pile up",
		Warning:    "Deal sites say the used-card aisle is full. Cheap days may be coming.",
		Rows: []ProductRow{
			{Name: "3090 SLED", Ask: 1540, Bid: 1260, Stock: 12, Slots: 2},
			{Name: "4090", Ask: 1980, Bid: 1620, Stock: 20, Slots: 1},
			{Name: "H100", Ask: 35_200, Bid: 28_800, Stock: 2, Slots: 1},
		},
		Hosting:    "power off",
		StayCost:   "-$0 lease",
		TravelCost: "-$250 freight",
		Side: []string{
			"HOLDINGS",
			"  nothing yet",
			"",
			"SHORELINE CREDIT",
			"  debt     $20,000",
			"  headroom $10,000",
			"  tomorrow -$2,000 interest",
			"",
			"TRAVEL",
			"  Ashburn    -800 lease, 16 slots",
			"  West Texas -250 lease, 24 slots",
		},
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Resizing never advances time and never mutates run state; it only
		// changes the layout inputs.
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeHelp:
			return m.handleHelpKey(msg)
		case modeQty:
			return m.handleQtyKey(msg)
		default:
			return m.handleBoardKey(msg)
		}
	}
	return m, nil
}

func (m Model) handleBoardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
		m.status = ""
		return m, nil
	case "up", "k":
		return m.moveSelection(-1), nil
	case "down", "j":
		return m.moveSelection(1), nil
	case "b", "s":
		m.qtyAction = actionBuy
		if msg.String() == "s" {
			m.qtyAction = actionSell
		}
		m.mode = modeQty
		m.qtyDraft = ""
		m.status = ""
		return m, nil
	case "l", "t", "e":
		m.status = "Lender, travel, and stay arrive with the trading engine; nothing changed."
		return m, nil
	case "esc":
		m.status = ""
		return m, nil
	case "enter", "ctrl+m":
		m.status = "Committing a day arrives with the trading engine; nothing changed."
		return m, nil
	}
	return m, nil
}

func (m Model) handleQtyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode = modeBoard
		m.qtyDraft = ""
		return m, nil
	case "backspace":
		if len(m.qtyDraft) > 0 {
			m.qtyDraft = m.qtyDraft[:len(m.qtyDraft)-1]
		}
		return m, nil
	case "a", "b", "c":
		row := m.selectedRow()
		switch key {
		case "a":
			m.qtyDraft = "1"
		case "b":
			m.qtyDraft = "5"
		case "c":
			m.qtyDraft = itoa(m.maxQuantity(row))
		}
		m.status = ""
		return m, nil
	case "enter", "ctrl+m":
		n, ok := parseQuantity(m.qtyDraft)
		row := m.selectedRow()
		if !ok || n < 1 {
			m.status = "Enter a whole number of units, or Esc to cancel."
			return m, nil
		}
		if limit := m.maxQuantity(row); n > limit {
			m.status = "That many will not fit: cash, stock, and rack slots allow " + itoa(limit) + "."
			return m, nil
		}
		verb := "Buying"
		if m.qtyAction == actionSell {
			verb = "Selling"
		}
		m.mode = modeBoard
		m.status = verb + " " + itoa(n) + " " + row.Name +
			" needs the trading engine; nothing changed."
		m.qtyDraft = ""
		return m, nil
	}
	if r, ok := digitOf(msg); ok && len(m.qtyDraft) < 7 {
		m.qtyDraft += string(r)
		m.status = ""
	}
	return m, nil
}

// digitOf reads a typed digit. Some input paths report a printable key as text,
// others only as a rune code with no text, so both are accepted -- but never for
// a modified key, because Alt+digit and Ctrl+digit are commands, not numbers.
func digitOf(msg tea.KeyPressMsg) (rune, bool) {
	if msg.Mod != 0 {
		return 0, false
	}
	if len(msg.Text) == 1 && msg.Text[0] >= '0' && msg.Text[0] <= '9' {
		return rune(msg.Text[0]), true
	}
	if msg.Text == "" && msg.Code >= '0' && msg.Code <= '9' {
		return msg.Code, true
	}
	return 0, false
}

func (m Model) handleHelpKey(_ tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.mode = modeBoard
	return m, nil
}

func (m Model) moveSelection(delta int) Model {
	n := len(m.board.Rows)
	if n == 0 {
		return m
	}
	m.sel = (m.sel + delta + n*2) % n
	return m
}

func (m Model) selectedRow() ProductRow {
	if m.sel < 0 || m.sel >= len(m.board.Rows) {
		return ProductRow{}
	}
	return m.board.Rows[m.sel]
}

// maxQuantity is the preview-only clamp used by the shell fixture so quantity
// entry behaves sensibly before the engine exposes its own limits.
func (m Model) maxQuantity(row ProductRow) int {
	if m.qtyAction == actionSell {
		return row.Owned
	}
	max := row.Stock
	if row.Slots > 0 {
		free := m.board.SlotsTotal - m.board.SlotsUsed
		if free < 0 {
			free = 0
		}
		if bySlots := free / row.Slots; bySlots < max {
			max = bySlots
		}
	}
	if row.Ask > 0 {
		room := 0
		if m.board.Cash > 0 {
			room = int(m.board.Cash / row.Ask)
		}
		if room < max {
			max = room
		}
	}
	if max < 0 {
		max = 0
	}
	return max
}

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

func itoa(n int) string { return strconv.Itoa(n) }
