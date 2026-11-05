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
	switch m.mode {
	case modeQty:
		return strings.Join(m.overlayLines(m.qtyBox()), "\n")
	case modeHelp:
		return strings.Join(m.overlayLines(m.helpBox()), "\n")
	default:
		return strings.Join(m.boardLines(), "\n")
	}
}

func (m Model) boardLines() []string {
	if m.width >= WideWidth {
		return m.wideBoardLines()
	}
	return m.mainLines(m.width)
}

func (m Model) wideBoardLines() []string {
	mainW := m.width - sideWidth - 1
	main := m.mainLines(mainW)
	panel := m.sidePanel(sideWidth)
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
func (m Model) mainLines(width int) []string {
	b := m.board
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
		m.chrome.Plain("  stay "+b.StayCost+"  travel "+b.TravelCost), width))
	lines = append(lines, m.chrome.fitPad(m.statusLine(), width))
	lines = append(lines, m.chrome.fitPad(m.chrome.Muted(
		"[b]uy  [s]ell  [l]ender  [t]ravel  [e]stay  [enter] commit day  [?]help  [q]uit"), width))
	return lines
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

func (m Model) statusLine() string {
	if m.status != "" {
		if strings.HasPrefix(m.status, "That many") || strings.HasPrefix(m.status, "Enter a whole") {
			return m.chrome.Bad(m.status)
		}
		return m.chrome.Warn(m.status)
	}
	row := m.selectedRow()
	if row.Name == "" {
		return m.chrome.Muted("No products here.")
	}
	return m.chrome.Muted(fmt.Sprintf("%s: buy %s  sell %s  %d in stock, you hold %d (%d slot%s each)",
		row.Name, game.FormatMoney(row.Ask), game.FormatMoney(row.Bid), row.Stock, row.Owned,
		row.Slots, plural(row.Slots)))
}

func (m Model) sidePanel(width int) []string {
	out := make([]string, 0, len(m.board.Side)+2)
	out = append(out, m.chrome.Head(m.chrome.fitPad("DETAIL", width)))
	for _, l := range m.board.Side {
		out = append(out, m.chrome.fitPad(m.chrome.Muted(l), width))
	}
	return out
}

// overlayLines draws box on top of the current board, centered, sized to the
// widest box line. Base rows outside the box stay visible.
func (m Model) overlayLines(box []string) []string {
	base := m.mainLines(m.width)
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

func (m Model) qtyBox() []string {
	row := m.selectedRow()
	verb, price := "BUY", row.Ask
	if m.qtyAction == actionSell {
		verb, price = "SELL", row.Bid
	}
	n, ok := parseQuantity(m.qtyDraft)
	if !ok {
		n = 0
	}
	limit := m.maxQuantity(row)
	freeSlots := m.board.SlotsTotal - m.board.SlotsUsed

	lines := []string{
		m.chrome.Title(verb + " " + row.Name),
		m.chrome.Muted(m.chrome.Rule(44)),
		fmt.Sprintf("ask %-10s bid %-10s stock %d  you hold %d",
			game.FormatMoney(row.Ask), game.FormatMoney(row.Bid), row.Stock, row.Owned),
		fmt.Sprintf("free slots %d of %d   %s costs %s each",
			freeSlots, m.board.SlotsTotal, row.Name, game.FormatMoney(price)),
		"",
		fmt.Sprintf("quantity: %s  (max %d here)", m.qtyDraft+"_", limit),
	}
	// The preview shows the consequence even when it is refused, in the colour
	// of a problem, so the number on screen never claims a deal the engine will
	// not allow.
	cost := int64(n) * int64(price)
	cashAfter := int64(m.board.Cash) - cost
	slotsAfter := int64(freeSlots) - int64(n*row.Slots)
	switch {
	case m.qtyAction == actionSell && n > row.Owned:
		lines = append(lines, m.chrome.Bad(fmt.Sprintf("you hold %d %s, not %d",
			row.Owned, row.Name, n)))
	case m.qtyAction == actionSell:
		proceeds := int64(n) * int64(row.Bid)
		lines = append(lines, fmt.Sprintf("proceeds %s   cash after %s",
			game.FormatMoney(game.Money(proceeds)),
			game.FormatMoney(game.Money(int64(m.board.Cash)+proceeds))))
	case cashAfter < 0:
		lines = append(lines, m.chrome.Bad(fmt.Sprintf("cost %s  you need %s more",
			game.FormatMoney(cost), game.FormatMoney(game.Money(-cashAfter)))))
	case slotsAfter < 0:
		lines = append(lines, m.chrome.Bad(fmt.Sprintf("cost %s  you need %d more slots",
			game.FormatMoney(cost), -slotsAfter)))
	case n > row.Stock:
		lines = append(lines, m.chrome.Bad(fmt.Sprintf("cost %s  only %d in stock",
			game.FormatMoney(cost), row.Stock)))
	default:
		lines = append(lines, fmt.Sprintf("cost %s   cash after %s   slots left %d",
			game.FormatMoney(cost), game.FormatMoney(cashAfter), slotsAfter))
	}
	// The reason a submission was refused has to sit inside the box: the box
	// covers the board's status line, so an explanation out there is invisible
	// while the player is still correcting the number.
	if m.status != "" {
		lines = append(lines, m.status)
	}
	return append(lines,
		"",
		m.chrome.Muted("[a] 1  [b] 5  [c] max   [enter] next   [esc] cancel"),
		m.chrome.Muted("type digits to set an exact number"),
	)
}

func (m Model) helpBox() []string {
	return []string{
		m.chrome.Title("RACK WARS KEYS"),
		m.chrome.Muted(m.chrome.Rule(44)),
		"up/down or j/k    pick a product",
		"b / s             buy or sell the selected product",
		"a / b / c         quantity 1, 5, or max in the quantity box",
		"digits            type an exact quantity",
		"l                 lender: borrow, repay, headroom, stress preview",
		"t                 travel: freight, capacity, risk",
		"e                 stay: host or power off",
		"enter             accept; committing a day needs confirmation",
		"esc               cancel the current box, nothing changes",
		"?                 this help",
		"q or ctrl+c       save and quit",
		"",
		m.chrome.Muted("Buying, selling, and borrowing never cost a day."),
		m.chrome.Muted("Only a confirmed Travel or Stay advances the clock."),
		m.chrome.Muted("Resizing the window never advances the clock either."),
		"",
		m.chrome.Muted("press any key to go back"),
	}
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
