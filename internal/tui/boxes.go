package tui

import (
	"fmt"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

// boxFooter is the shared instruction line: every box states its two exits, so
// the player always knows how to get out without committing anything.
func (m Model) boxFooter(commit string) []string {
	return []string{
		"",
		m.chrome.Muted("[enter] " + commit + "   [esc] cancel, nothing changes"),
	}
}

// qtyBox is the quantity entry. The price lines come from the board snapshot and
// the arithmetic comes from the engine preview for the exact number typed, so
// what the player sees is the number the engine will act on.
func (m Model) qtyBox(b Board) []string {
	row := boardRow(b, m.sel)
	verb, price := "BUY", row.Ask
	if m.qtyAction == actionSell {
		verb, price = "SELL", row.Bid
	}
	q, ok := parseQuantity(m.qtyDraft)
	if !ok {
		q = 0
	}
	limits := m.run.BuyLimits(row.Product)
	pvBuy := m.run.PreviewBuy(row.Product, q)
	pvSell := m.run.PreviewSell(row.Product, q)
	if m.qtyAction == actionSell {
		limits = m.run.SellLimits(row.Product)
	}

	lines := []string{
		m.chrome.Title(verb + " " + row.Name),
		m.chrome.Muted(m.chrome.Rule(46)),
		fmt.Sprintf("ask %-10s bid %-10s stock %d  you hold %d",
			game.FormatMoney(row.Ask), game.FormatMoney(row.Bid), row.Stock, row.Owned),
		fmt.Sprintf("free slots %d of %d   %s is %s each",
			b.SlotsTotal-b.SlotsUsed, b.SlotsTotal, row.Name, game.FormatMoney(price)),
		"",
		"quantity: " + m.qtyDraft + "_",
	}
	bound := "the engine will take " + itoa(limits.Max)
	if limits.Max > 0 {
		bound += " here"
	} else {
		bound += ": " + limits.LimitBy.String()
	}
	lines = append(lines, m.chrome.Muted(bound))

	switch m.qtyAction {
	case actionBuy:
		lines = append(lines, buyPreview(m.chrome, pvBuy)...)
	default:
		lines = append(lines, sellPreview(m.chrome, pvSell)...)
	}

	// The reason a submission was refused sits inside the box: the box covers the
	// board's status line, so an explanation out there is invisible while the
	// player is still correcting the number.
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	commit := "buy it now (no day passes)"
	if m.qtyAction == actionSell {
		commit = "sell it now (no day passes)"
	}
	lines = append(lines,
		"",
		m.chrome.Muted("[a] 1  [b] 5  [c] max   type digits for an exact number"),
	)
	return append(lines, m.boxFooter(commit)...)
}

func buyPreview(c Chrome, pv game.PreviewBuy) []string {
	if !pv.Valid {
		switch pv.Reason {
		case game.ReasonInsufficientCash:
			return []string{c.Bad(fmt.Sprintf("cost %s  short %s", game.FormatMoney(pv.Cost),
				game.FormatMoney(pv.ShortBy)))}
		case game.ReasonOvercapacity:
			return []string{c.Bad(fmt.Sprintf("cost %s  %d slots short", game.FormatMoney(pv.Cost),
				pv.SlotsShort))}
		case game.ReasonInsufficientStock:
			return []string{c.Bad(fmt.Sprintf("cost %s  %d short of stock", game.FormatMoney(pv.Cost),
				pv.StockShort))}
		default:
			return []string{c.Bad("refused: " + pv.Reason.String())}
		}
	}
	return []string{fmt.Sprintf("cost %s   cash after %s   slots %d/%d",
		game.FormatMoney(pv.Cost), game.FormatMoney(pv.CashAfter),
		pv.SlotsAfter, pv.SlotsTotal)}
}

func sellPreview(c Chrome, pv game.PreviewSell) []string {
	if !pv.Valid {
		return []string{c.Bad("refused: " + pv.Reason.String())}
	}
	line := fmt.Sprintf("gross %s   lender takes %s   you get %s",
		game.FormatMoney(pv.Gross), game.FormatMoney(pv.ToLender), game.FormatMoney(pv.CashIn))
	out := []string{line}
	if pv.ToLender > 0 {
		out = append(out, c.Warn("the lender is first in line: debt "+game.FormatMoney(pv.DebtBefore)+
			" -> "+game.FormatMoney(pv.DebtAfter)))
	} else {
		out = append(out, "cash after "+game.FormatMoney(pv.CashAfter)+"   debt "+game.FormatMoney(pv.DebtAfter))
	}
	return append(out, "realized "+signedMoney(pv.Realized)+" against your average cost")
}

// lenderBox shows the note on the garage door in the design's own format, plus a
// preview of the exact amount typed.
func (m Model) lenderBox(b Board) []string {
	st := m.run.State
	amount, ok := parseQuantity(m.loanDraft)
	pv := m.run.PreviewBorrow(game.Money(amount))
	if !ok {
		amount = 0
		pv = m.run.PreviewBorrow(0)
	}
	if m.loanAction == loanRepay {
		pv = m.run.PreviewRepay(game.Money(amount))
	}
	s := m.run.LenderStress(stressFallPct)

	lines := []string{
		m.chrome.Title("SHORELINE CREDIT") + "   " +
			m.modeKey("b", "borrow", m.loanAction == loanBorrow) + "  " +
			m.modeKey("r", "repay", m.loanAction == loanRepay),
		m.chrome.Muted(m.chrome.Rule(46)),
		"LENDER: Ceiling " + game.FormatMoney(pv.Ceiling) + ". Tomorrow's interest: " +
			game.FormatMoney(m.run.TomorrowInterest()) + ".",
		fmt.Sprintf("debt %-10s collateral %-10s headroom %s",
			game.FormatMoney(st.Debt), game.FormatMoney(m.run.CollateralValue()),
			game.FormatMoney(pv.Available)),
		"if bids fall " + itoa(stressFallPct) + "%: ceiling " + game.FormatMoney(s.Ceiling) +
			"   debt then " + game.FormatMoney(s.ProjectedDebt),
		"",
		"amount: $" + m.loanDraft + "_   [a] all of it",
	}
	if amount > 0 {
		lines = append(lines, loanPreview(m.chrome, m.loanAction, pv))
	} else {
		lines = append(lines, m.chrome.Muted("type an amount to see what it does"))
	}
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	lines = append(lines, m.boxFooter("apply it now (no day passes)")...)
	return append(lines, m.chrome.Muted("Nothing here advances the clock."))
}

func loanPreview(c Chrome, kind loanAction, pv game.PreviewLoan) string {
	if !pv.Valid {
		return c.Bad("refused: " + pv.Reason.String())
	}
	verb := "Borrow"
	if kind == loanRepay {
		verb = "Repay"
	}
	return fmt.Sprintf("%s %s: cash %s -> %s   debt %s -> %s   tomorrow's interest %s",
		verb, game.FormatMoney(pv.Amount),
		game.FormatMoney(pv.CashBefore), game.FormatMoney(pv.CashAfter),
		game.FormatMoney(pv.DebtBefore), game.FormatMoney(pv.DebtAfter),
		game.FormatMoney(pv.InterestTomorrow))
}

// travelBox lists the roads with their freight and capacity, and the destination
// town's known quotes with the day they were seen.
func (m Model) travelBox(b Board) []string {
	opts := m.otherMarkets()
	lines := []string{
		m.chrome.Title("TRAVEL"),
		m.chrome.Muted(m.chrome.Rule(46)),
	}
	for i, id := range opts {
		pv := m.run.PreviewTravel(id)
		dest := content.MarketOf(id)
		marker := m.chrome.Marker(i == m.travelSel)
		lines = append(lines, marker+fmt.Sprintf("[%d] %-12s freight %-9s  %d slots  lease %s/day",
			i+1, dest.Name, "-"+game.FormatMoney(pv.Freight), pv.SlotsTotal,
			"-"+game.FormatMoney(pv.NextLease)))
	}
	dest := m.destMarket()
	if dest == "" {
		return append(lines, m.chrome.Muted("No town is selected."))
	}
	pv := m.run.PreviewTravel(dest)
	lines = append(lines,
		m.chrome.Muted(m.chrome.Rule(46)),
		"to "+content.MarketOf(dest).Name+": cash "+game.FormatMoney(pv.CashAfter)+
			"   debt after interest "+game.FormatMoney(pv.DebtAfter)+
			"   your "+itoa(pv.OccupiedSlots)+" slots into "+itoa(pv.SlotsTotal),
	)
	if pv.OccupiedSlots > pv.SlotsTotal {
		lines = append(lines, m.chrome.Bad("Your load does not fit there: "+
			itoa(pv.OccupiedSlots-pv.SlotsTotal)+" slots over."))
	}
	lines = append(lines, m.knownQuoteLines(dest)...)
	lines = append(lines, m.chrome.Warn(m.chrome.Bullet()+" Travelling costs the day: this closes day "+
		itoa(b.Day)+", and prices elsewhere are only what they were when you saw them."))
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	lines = append(lines, m.chrome.Muted("[1]-"+itoa(len(opts))+" pick a road   [up]/[down] move the marker"))
	return append(lines, m.boxFooter("go, and close the day")...)
}

// confirmBox is the day-commit guard: time only moves after a second Enter.
func (m Model) confirmBox(b Board) []string {
	pv := m.run.PreviewStay()
	last := b.Day >= content.DayCount
	title, move := "FACE THE MORNING?",
		"stay put at "+b.Town+", day "+itoa(b.Day)+" -> "+itoa(b.Day+1)
	if last {
		// Day 30 has no morning after: this commit is the closing auction.
		title, move = "CLOSE THE BOOKS?",
			"day 30 is the last one: this commit sells what you hold and scores the run"
	}
	lines := []string{
		m.chrome.Title(title),
		m.chrome.Muted(m.chrome.Rule(46)),
		move,
		"interest " + game.FormatMoney(pv.Interest) + "   lease " + game.FormatMoney(pv.Lease) +
			"   hosting " + game.FormatMoney(pv.Hosted),
		"cash after " + game.FormatMoney(pv.CashAfter) + "   debt after " + game.FormatMoney(pv.DebtAfter),
	}
	if hl := content.HeadlineFor(b.Day + 1); hl != nil {
		if hl.Text != "" {
			lines = append(lines, m.chrome.Muted("the wire may say: "+hl.Text))
		}
		if hl.Tomorrow != "" {
			lines = append(lines, m.chrome.Warn(m.chrome.Bullet()+" "+hl.Tomorrow))
		}
	}
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	footer := "close the day"
	if last {
		footer = "close the run for good"
	}
	return append(lines, m.boxFooter(footer)...)
}

// endScreen is the whole-screen ending: the one big dollar line first, then the
// receipts that explain it, then the two ways to play again.
func (m Model) endScreen(b Board) []string {
	st := m.run.State
	end := st.End
	stt := st.Stats
	head := "DAY 30: THE CLOSING AUCTION"
	if end.Kind == game.EndRepossessed {
		head = "REPOSSESSED"
	}
	lines := []string{
		m.chrome.Title("RACK WARS") + m.chrome.Plain("  "+head),
		m.chrome.Muted(m.chrome.Rule(m.width - 2)),
		m.chrome.Plain(end.Epitaph),
		"",
		m.chrome.Bold("RANK: " + end.Rank),
	}
	if end.Kind == game.EndRepossessed {
		lines = append(lines, m.chrome.Bad("DEFICIENCY: the lender is still owed "+
			game.FormatMoney(end.Deficiency)+" after taking everything."),
			m.chrome.Muted("You walked out with nothing, and the debt is still real."))
	} else {
		lines = append(lines, m.chrome.Bold("CASH OUT: "+game.FormatMoney(end.CashOut))+
			m.chrome.Muted("  (after the note is settled)"))
	}
	lines = append(lines,
		"",
		m.chrome.Head("how it went"),
		"  borrowed      "+game.FormatMoney(stt.Borrowed)+
			"        repaid        "+game.FormatMoney(stt.Repaid),
		"  interest paid "+game.FormatMoney(stt.InterestPaid)+
			"        trade profit  "+signedMoney(stt.TradeProfit),
		"  best flip     "+signedMoney(stt.BestFlip)+
			"        peak net      "+game.FormatMoney(stt.PeakNetWorth),
		"  freight paid  "+game.FormatMoney(stt.FreightPaid)+
			"        leases paid   "+game.FormatMoney(stt.LeasePaid),
		"  forced sales  "+itoa(stt.ForcedUnits)+" units  "+game.FormatMoney(stt.ForcedLoss)+
			"        days on road  "+itoa(stt.TravelDays),
		"",
		m.chrome.Muted("seed "+itoa64(int64(m.seed))+"  -  [r] replays this seed exactly, [n] deals a new one"),
		m.chrome.Muted("[q] quit"),
	)
	return lines
}

// modeKey renders a toggle inside a box: the active choice is highlighted.
func (m Model) modeKey(key, label string, on bool) string {
	s := "[" + key + "] " + label
	if on {
		return m.chrome.Sel(m.chrome.Bold(s))
	}
	return m.chrome.Muted(s)
}

// boxStatus colours the last engine refusal inside a box.
func (m Model) boxStatus() string {
	if m.statusBad {
		return m.chrome.Bad(m.status)
	}
	return m.chrome.Warn(m.status)
}

func boardRow(b Board, i int) ProductRow {
	if i < 0 || i >= len(b.Rows) {
		return ProductRow{}
	}
	return b.Rows[i]
}

// helpBox documents the whole key set, including the parts that arrive with the
// final rules, so a player who reads it now is not surprised later.
func (m Model) helpBox() []string {
	return []string{
		m.chrome.Title("RACK WARS KEYS"),
		m.chrome.Muted(m.chrome.Rule(46)),
		"up/down or j/k    pick a product",
		"b / s             buy or sell the selected product",
		"a / b / c         quantity 1, 5, or max in the quantity box",
		"digits            type an exact quantity",
		"l                 lender: borrow, repay, headroom, stress preview",
		"t                 travel: opens the road list",
		"1-9 in travel     pick that road without an arrow key",
		"e or enter        face the morning: a confirmation first, then the day closes",
		"esc               cancel the current box, nothing changes",
		"?                 this help",
		"q or ctrl+c       quit",
		"",
		m.chrome.Muted("Buying, selling, borrowing, and repaying never cost a day."),
		m.chrome.Muted("Only a confirmed Travel or Stay advances the clock."),
		m.chrome.Muted("Resizing the window never advances the clock either."),
		"",
		m.chrome.Muted("press any key to go back"),
	}
}
