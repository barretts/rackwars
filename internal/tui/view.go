package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

const sideWidth = 30

// View implements tea.Model. The board always owns the alternate screen, so an
// overlay closing cannot leave stale output behind.
func (m Model) View() tea.View {
	rendered := m.render()
	if m.saveErr != "" {
		rendered = "NOT ON DISK ? press S to retry saving\n" + rendered
	}
	v := tea.NewView(rendered)
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
	if m.mode == modeContinue {
		return strings.Join(m.continueLines(), "\n")
	}
	if m.over() {
		m.mode = modeEnd
		return strings.Join(m.endLines(), "\n")
	}
	// A pending stop is drawn over the board, not instead of it: the player
	// can still read the town they are stuck in behind the box.
	if m.run.State.PendingEncounter != nil {
		if box := m.encounterBox(); len(box) > 0 {
			return strings.Join(m.overlayLines(box), "\n")
		}
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
	case modeStay:
		return strings.Join(m.overlayLines(m.stayBox(b)), "\n")
	case modeSubset:
		return strings.Join(m.overlayLines(m.subsetBox(b)), "\n")
	case modeAlloc:
		return strings.Join(m.overlayLines(m.allocBox(b)), "\n")
	case modeHelp:
		return strings.Join(m.overlayLines(m.helpBox()), "\n")
	default:
		return strings.Join(m.fitHeight(m.boardLines(b)), "\n")
	}
}

// fitHeight caps the board frame at the screen height. The row set itself has
// always fit the minimum terminal; only wrapped prose can push it over, and
// then the top of the board and the key line are what a player still needs,
// so they stay and the lost middle says so plainly. Frames that fit are
// returned untouched: short frames are today's normal, not an error.
func (m Model) fitHeight(lines []string) []string {
	if len(lines) <= m.height {
		return lines
	}
	lost := len(lines) - (m.height - 2) - 1
	trimmed := make([]string, 0, m.height)
	trimmed = append(trimmed, lines[:m.height-2]...)
	trimmed = append(trimmed, m.chrome.Muted(m.chrome.Bullet()+" "+itoa(lost)+
		" wrapped line"+plural(lost)+" hidden by the window height; resize to read them"),
		lines[len(lines)-1])
	return trimmed
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

// continueLines is the start-up list of saved runs. Like the ending screen it
// fills the frame instead of overlaying a box, because the choice it is asking
// for must not be missable: the alternative is starting a fresh run over a run the
// player believed they had kept.
func (m Model) continueLines() []string {
	folder := "(no save folder)"
	if m.store != nil {
		folder = m.store.Dir
	}
	lines := []string{
		m.chrome.Title("RACK WARS") + m.chrome.Muted("   saved runs"),
		m.chrome.Muted(folder),
		m.chrome.Muted(m.chrome.Rule(64)),
		"",
	}
	for i, e := range m.saves {
		marker, num := "  ", itoa(i+1)
		if i == m.saveSel {
			marker, num = m.chrome.Plain("> "), m.chrome.Bold(num)
		}
		if e.Err != nil {
			lines = append(lines, m.chrome.fitPad(marker+num+"  "+m.chrome.Bad(e.ID+"  this save cannot be opened"), m.width))
			lines = append(lines, m.chrome.fitPad("    "+m.chrome.Muted(e.Err.Error()), m.width))
			continue
		}
		state := "still open"
		switch e.Over {
		case game.EndClosed:
			state = "closed on day " + itoa(content.DayCount)
		case game.EndRepossessed:
			state = "repossessed"
		}
		lines = append(lines, m.chrome.fitPad(marker+num+"  seed "+itoa(int(e.Seed))+
			m.chrome.Plain("   day "+itoa(e.Day)+"/"+itoa(content.DayCount)+
				"   cash "+game.FormatMoney(e.Cash)+"   debt "+game.FormatMoney(e.Debt)), m.width))
		lines = append(lines, m.chrome.fitPad("    "+m.chrome.Muted(string(e.Market)+"   "+state+
			"   written "+e.Written), m.width))
	}
	lines = append(lines, "")
	if m.status != "" {
		lines = append(lines, m.chrome.fitPad(m.chrome.Bad(m.status), m.width))
	}
	lines = append(lines, m.chrome.Muted("[enter] carry on   [n] new run   [r] recover the backup   [1-9] pick   [q] quit"))
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
				"  fleet "+itoa(b.SlotsUsed)+"  racks "+itoa(b.SlotsTotal)), width),
	}
	lines = append(lines, m.chrome.wrapPad(m.chrome.Bold(b.Headline), width)...)
	if b.Warning != "" {
		lines = append(lines, m.chrome.wrapPad(m.chrome.Warn(m.chrome.Bullet()+" "+b.Warning), width)...)
	} else {
		lines = append(lines, m.chrome.Muted(m.chrome.Rule(width)))
	}
	lines = append(lines, m.chrome.Rule(width))

	lines = append(lines, m.chrome.Head(fmt.Sprintf("%-2s%12s %10s %10s %6s %6s %5s %6s %8s %5s",
		"", "PRODUCT", "BUY ASK", "SELL BID", "STOCK", "OWNED", "SLOTS", "KW", "HOST/NGT", "CLASS")))
	for i, row := range b.Rows {
		lines = append(lines, m.productLine(width, i, row))
	}

	lines = append(lines, m.chrome.Rule(width))
	lines = append(lines, m.chrome.wrapPad(m.chrome.Muted("Hosting: "+b.Hosting)+
		m.chrome.Plain("  overnight "+b.StayCost+" interest  travel "+b.TravelCost), width)...)
	if b.Power != "" {
		lines = append(lines, m.chrome.wrapPad(m.chrome.Muted("Meter:   "+b.Power), width)...)
	}
	if b.Curtail != "" {
		lines = append(lines, m.chrome.wrapPad(m.chrome.Good(m.chrome.Bullet()+" "+b.Curtail), width)...)
	}
	if b.Alloc != "" {
		lines = append(lines, m.chrome.wrapPad(m.chrome.Warn(m.chrome.Bullet()+" "+b.Alloc), width)...)
	}
	lines = append(lines, m.chrome.wrapPad(m.statusLine(b), width)...)
	keys := "[b]uy  [s]ell  [l]ender  [t]ravel  [e]/[enter] face morning  [?]help  [q]uit"
	if b.Full {
		keys = "[b]uy  [s]ell  [l]ender  [a]llocation  [t]ravel  [e]/[enter] plan the night  [?]help  [q]uit"
	}
	return append(lines, m.chrome.wrapPad(m.chrome.Muted(keys), width)...)
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
	case game.ReasonInsufficientStock:
		bound = "shop stock"
	case game.ReasonOK:
		bound = "what you hold"
	}
	return m.chrome.Muted(fmt.Sprintf("%s: buy %s  sell %s  stock %d, you hold %d (%d slot%s each)  most %s: %d (%s)",
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
	line := fmt.Sprintf("%-2s%12s %10s %10s %6d %6d %5d %6s %8s %5s",
		m.chrome.Marker(selected), row.Name,
		game.FormatMoney(row.Ask), game.FormatMoney(row.Bid),
		row.Stock, row.Owned, row.Slots,
		kwStr(row.KW100), game.FormatMoney(row.Host), row.Class)
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
		out = append(out, m.chrome.wrapPad(m.chrome.Muted(l), width)...)
	}
	return out
}

// overlayLines draws box on top of the current board, centered, sized to the
// widest box line. Base rows outside the box stay visible.
func (m Model) overlayLines(box []string) []string {
	base := m.fitHeight(m.mainLines(m.snapshot(), m.width))
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

// wrapFlow breaks one styled line into lines of at most w cells, breaking
// words at spaces, so a headline or a status line that outgrows the terminal
// is read to the end instead of losing its punchline to an ellipsis. Escape
// sequences ride with the text that follows them and are not counted, which
// keeps the word logic correct for anything the chrome styles can produce;
// a word wider than the line is cut at the cell limit, which still lands on
// a printable boundary because the cut is measured in cells, not bytes.
func (c Chrome) wrapFlow(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	if ansi.StringWidth(s) <= w {
		return []string{s}
	}
	// Tokenize into whole words plus escape stubs. A space ends a word; runs
	// of spaces collapse, because prose re-wrapping owns the gaps now.
	var words []struct {
		text  string
		cells int
	}
	var buf strings.Builder
	cells := 0
	inEsc := false
	flush := func() {
		if buf.Len() > 0 {
			words = append(words, struct {
				text  string
				cells int
			}{buf.String(), cells})
			buf.Reset()
			cells = 0
		}
	}
	for _, r := range s {
		if inEsc {
			buf.WriteRune(r)
			if (r >= '0' && r <= '9') || r == ';' || r == '?' || r == '[' {
				continue
			}
			inEsc = false
			continue
		}
		if r == 0x1b {
			buf.WriteRune(r)
			inEsc = true
			continue
		}
		if r == ' ' {
			flush()
			continue
		}
		buf.WriteRune(r)
		cells += runewidth.RuneWidth(r)
	}
	flush()

	var (
		out   []string
		line  strings.Builder
		lineW int
	)
	put := func(text string, cw int) {
		for cw > w {
			// A word wider than the line: fill the line, then keep cutting.
			if lineW > 0 {
				out = append(out, line.String())
				line.Reset()
				lineW = 0
			}
			// Cut the over-long word cell by cell. Rune-wise
			// re-measuring stops on a cell boundary with its escapes; a
			// single rune wider than the whole line is kept whole
			// (sub-floor terminals are the game's own problem).
			rs := []rune(text)
			n, taken := 0, 1
			for i, r := range rs {
				nw := n + runewidth.RuneWidth(r)
				if nw > w {
					if i > 0 {
						taken = i
					}
					break
				}
				n = nw
			}
			if taken == len(rs) {
				taken = max(taken-1, 1) // safety: always consume progress
			}
			head := string(rs[:taken])
			hw := 0
			for _, r := range rs[:taken] {
				hw += runewidth.RuneWidth(r)
			}
			text, cw = string(rs[taken:]), cw-hw
			out = append(out, head)
		}
		if lineW == 0 {
			line.WriteString(text)
			lineW = cw
			return
		}
		if lineW+1+cw <= w {
			line.WriteString(" ")
			line.WriteString(text)
			lineW += 1 + cw
			return
		}
		out = append(out, line.String())
		line.Reset()
		line.WriteString(text)
		lineW = cw
	}
	for _, word := range words {
		put(word.text, word.cells)
	}
	if line.Len() > 0 || len(out) == 0 {
		out = append(out, line.String())
	}
	return out
}

// wrapPad wraps prose like wrapFlow and pads each resulting line to exactly w
// cells, the prose sibling of fitPad.
func (c Chrome) wrapPad(s string, w int) []string {
	lines := c.wrapFlow(s, w)
	for i, l := range lines {
		if pad := w - ansi.StringWidth(l); pad > 0 {
			lines[i] = l + strings.Repeat(" ", pad)
		}
	}
	return lines
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
