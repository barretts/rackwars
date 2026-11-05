package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"rackwars/internal/game"
)

const sideWidth = 30

// View implements tea.Model. The board always owns the alternate screen, so an
// overlay closing cannot leave stale output behind.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.width < MinWidth || m.height < MinHeight {
		return strings.Join(m.tooSmallLines(), "\n")
	}
	// One snapshot per frame: every box, panel, and status line on this screen
	// reads the same engine state, so no two parts of the frame can disagree.
	b := m.snapshot()
	if m.over() {
		m.mode = modeEnd
		return strings.Join(m.endLines(), "\n")
	}
	switch m.mode {
	case modeQty:
		return strings.Join(m.overlayLines(m.qtyBox(b)), "\n")
	case modeLender:
		return strings.Join(m.overlayLines(m.lenderBox(b)), "\n")
	case modeTravel:
		return strings.Join(m.overlayLines(m.travelBox(b)), "\n")
	case modeConfirm:
		return strings.Join(m.overlayLines(m.confirmBox(b)), "\n")
	case modeHelp:
		return strings.Join(m.overlayLines(m.helpBox()), "\n")
	default:
		return strings.Join(m.boardLines(b), "\n")
	}
}

// endLines fills the screen with the ending rather than overlaying a box, so the
// closing numbers cannot be scrolled off by a small terminal.
func (m Model) endLines() []string {
	lines := m.endScreen(m.snapshot())
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, l := range lines {
		lines[i] = m.chrome.fitPad(l, m.width)
	}
	return lines
}

func (m Model) boardLines(b Board) []string {
	if m.width >= WideWidth {
		return m.wideBoardLines(b)
	}
	return m.mainLines(b, m.width)
}

func (m Model) wideBoardLines(b Board) []string {
	mainW := m.width - sideWidth - 1
	main := m.mainLines(b, mainW)
	panel := m.sidePanel(b.Side, sideWidth)
	out := make([]string, max(len(main), len(panel)))
	for i := range out {
		left, right := "", ""
		if i < len(main) {
			left = main[i]
		}
		if i < len(panel) {
			right = panel[i]
		}
		out[i] = m.chrome.fitPad(left, mainW) + " " + right
	}
	return out
}

// mainLines is the market board. Width drives truncation only; the row set and
// order come from the engine snapshot.
func (m Model) mainLines(b Board, width int) []string {
	tag := ""
	if b.Tag != "" {
		tag = "  [" + b.Tag + "]"
	}
	lines := []string{
		m.chrome.fitPad(m.chrome.Title("RACK WARS")+
			m.chrome.Plain("  Day "+itoa(b.Day)+"/"+itoa(b.DayTotal)+"  "+b.Town)+
			m.chrome.Muted(tag), width),
		m.chrome.fitPad(m.chrome.Bold(game.FormatMoney(b.Cash)+" cash")+
			m.chrome.Plain("  ")+m.chrome.Bad(game.FormatMoney(b.Debt)+" debt")+
			m.chrome.Plain("  net ")+m.moneyColor(b.NetWorth)+
			m.chrome.Plain("  heat "+itoa(b.Heat)+
				"  slots "+itoa(b.SlotsUsed)+"/"+itoa(b.SlotsTotal)), width),
		m.chrome.fitPad(m.chrome.Bold(b.Headline), width),
	}
	if b.Warning != "" {
		lines = append(lines, m.chrome.fitPad(m.chrome.Warn(m.chrome.Bullet()+" "+b.Warning), width))
	} else {
		lines = append(lines, m.chrome.Muted(m.chrome.Rule(width)))
	}
	lines = append(lines, m.chrome.Rule(width))

	lines = append(lines, m.chrome.Head(fmt.Sprintf("%-2s%12s %10s %10s %6s %6s %5s",
		"", "PRODUCT", "BUY ASK", "SELL BID", "STOCK", "OWNED", "SLOTS")))
	for i, row := range b.Rows {
		lines = append(lines, m.productLine(width, i, row))
	}

	lines = append(lines, m.chrome.Rule(width))
	lines = append(lines, m.chrome.fitPad(m.chrome.Muted("Hosting: "+b.Hosting)+
		m.chrome.Plain("  overnight "+b.StayCost+" interest  travel "+b.TravelCost), width))
	lines = append(lines, m.chrome.fitPad(m.statusLine(b), width))
	lines = append(lines, m.chrome.fitPad(m.chrome.Muted(
		"[b]uy  [s]ell  [l]ender  [t]ravel  [e]/[enter] face morning  [?]help  [q]uit"), width))
	return lines
}

// statusLine is the board's live read of the selected card: its quotes, what the
// player holds, and the largest quantity the engine would accept right now, with
// the constraint that binds. None of it is computed here.
func (m Model) statusLine(b Board) string {
	if m.status != "" {
		if m.statusBad {
			return m.chrome.Bad(m.status)
		}
		return m.chrome.Warn(m.status)
	}
	row := boardRow(b, m.sel)
	if row.Name == "" {
		return m.chrome.Muted("No products here.")
	}
	limits := m.run.BuyLimits(row.Product)
	if m.qtyAction == actionSell {
		limits = m.run.SellLimits(row.Product)
	}
	bound := "cash"
	switch limits.LimitBy {
	case game.ReasonOvercapacity:
		bound = "slots"
	case game.ReasonInsufficientStock:
		bound = "shop stock"
	case game.ReasonOK:
		bound = "what you hold"
	}
	return m.chrome.Muted(fmt.Sprintf("%s: buy %s  sell %s  stock %d, you hold %d (%d slot%s)  most %s: %d (%s)",
		row.Name, game.FormatMoney(row.Ask), game.FormatMoney(row.Bid), row.Stock, row.Owned,
		row.Slots, plural(row.Slots), verbOf(m.qtyAction), limits.Max, bound))
}

func verbOf(a tradeAction) string {
	if a == actionSell {
		return "sell"
	}
	return "buy"
}

func (m Model) productLine(width, i int, row ProductRow) string {
	selected := i == m.sel
	line := fmt.Sprintf("%-2s%12s %10s %10s %6d %6d %5d",
		m.chrome.Marker(selected), row.Name,
		game.FormatMoney(row.Ask), game.FormatMoney(row.Bid),
		row.Stock, row.Owned, row.Slots)
	if selected {
		line = m.chrome.Sel(m.chrome.fitPad(line, width))
	} else {
		line = m.chrome.fitPad(line, width)
	}
	return line
}

func (m Model) sidePanel(lines []string, width int) []string {
	out := make([]string, 0, len(lines)+2)
	out = append(out, m.chrome.Head(m.chrome.fitPad("DETAIL", width)))
	for _, l := range lines {
		out = append(out, m.chrome.fitPad(m.chrome.Muted(l), width))
	}
	return out
}

// overlayLines draws box on top of the current board, centered, sized to the
// widest box line. Base rows outside the box stay visible.
func (m Model) overlayLines(box []string) []string {
	base := m.mainLines(m.snapshot(), m.width)
	if len(base) < m.height {
		for len(base) < m.height {
			base = append(base, "")
		}
	}
	boxW := 0
	for _, l := range box {
		if w := ansi.StringWidth(l); w > boxW {
			boxW = w
		}
	}
	boxW = min(boxW+4, m.width-2)
	start := (m.height - len(box)) / 2
	if start < 1 {
		start = 1
	}
	out := append([]string(nil), base...)
	for i, l := range box {
		r := start + i
		if r >= m.height {
			break
		}
		out[r] = m.chrome.fitPad("  "+m.chrome.fit(l, boxW), m.width)
	}
	return out
}

func (m Model) tooSmallLines() []string {
	w := m.width
	if w < 1 {
		w = 1
	}
	h := m.height
	if h < 1 {
		h = 1
	}
	lines := []string{
		m.chrome.Title("RACK WARS"),
		fmt.Sprintf("Terminal too small: %dx%d.", m.width, m.height),
		fmt.Sprintf("Needs at least %dx%d.", MinWidth, MinHeight),
		"Resize the window. The run keeps its state;",
		"no day passes and no key commits anything.",
		"",
		m.chrome.Muted("[q] quit"),
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		lines[i] = m.chrome.fitPad(l, w)
	}
	return lines
}

func (m Model) moneyColor(v game.Money) string {
	s := game.FormatMoney(v)
	switch {
	case v < 0:
		return m.chrome.Bad(s)
	case v > 0:
		return m.chrome.Good(s)
	default:
		return m.chrome.Plain(s)
	}
}

// fit truncates to w terminal cells, and fitPad also pads to exactly w cells so
// column starts stay put under ANSI styling.
func (c Chrome) fit(s string, w int) string {
	if ansi.StringWidth(s) <= w {
		return s
	}
	tail := "…"
	if c.ASCII {
		tail = "~"
	}
	return ansi.Truncate(s, max(w-len(tail), 0), tail)
}

func (c Chrome) fitPad(s string, w int) string {
	s = c.fit(s, w)
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
