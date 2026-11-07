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
		fmt.Sprintf("%s is %s each  %d slot%s  fleet %d",
			row.Name, game.FormatMoney(price), row.Slots, plural(row.Slots), b.SlotsUsed),
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
		case game.ReasonInsufficientStock:
			return []string{c.Bad(fmt.Sprintf("cost %s  %d short of stock", game.FormatMoney(pv.Cost),
				pv.StockShort))}
		default:
			return []string{c.Bad("refused: " + pv.Reason.String())}
		}
	}
	return []string{fmt.Sprintf("cost %s   cash after %s   fleet %d slots",
		game.FormatMoney(pv.Cost), game.FormatMoney(pv.CashAfter), pv.SlotsAfter)}
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
		lines = append(lines, marker+fmt.Sprintf("[%d] %-12s freight %-9s  hosts %d  lease %s/day",
			i+1, dest.Name, "-"+game.FormatMoney(pv.Freight), pv.HostCap,
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
			"   debt after interest "+game.FormatMoney(pv.DebtAfter),
		"fleet "+itoa(pv.FleetSlots)+" slots ride along; "+
			itoa(pv.HostCap)+" host racks there.",
	)
	lines = append(lines, m.knownQuoteLines(dest)...)
	lines = append(lines, m.chrome.Warn(m.chrome.Bullet()+" Travelling costs the day: this closes day "+
		itoa(b.Day)+", and prices elsewhere are only what they were when you saw them."))
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	lines = append(lines, m.chrome.Muted("[1]-"+itoa(len(opts))+" pick a road   [up]/[down] move the marker"))
	return append(lines, m.boxFooter("go, and close the day")...)
}

// stayBox is the five-market night plan. Every figure is one preview from the
// engine's single plan function — the same call settlement makes — so the
// screen and the morning cannot disagree about what a night costs.
func (m Model) stayBox(b Board) []string {
	pvAuto := m.run.PreviewStayMode(game.StayAuto, nil)
	pvOff := m.run.PreviewStayMode(game.StayPowerOff, nil)
	pvCurtail := m.run.PreviewStayMode(game.StayCurtail, nil)
	lines := []string{
		m.chrome.Title("THE NIGHT AHEAD AT " + b.Town),
		m.chrome.Muted(m.chrome.Rule(46)),
		"lease " + game.FormatMoney(pvAuto.Lease) + "/day   meter " +
			kwStr(pvAuto.PowerCap) + " kW   slots " + itoa(b.SlotsTotal) +
			"   heat " + itoa(b.Heat) + "°",
		"",
		m.modeKey("1", "AUTO — power the best set the meter and slots can carry",
			m.stayMode == game.StayAuto) + "   [p] pick the cards by hand",
	}
	shown := pvAuto
	if m.stayMode == game.StayAuto && len(m.stayPowered) > 0 {
		shown = m.run.PreviewStayMode(game.StayAuto, m.stayPowered)
		lines[4] = m.modeKey("1", "HAND-PICKED — "+itoa(lenCards(m.stayPowered))+" cards run",
			true) + "   [p] change them   [1] back to auto"
	}
	lines = append(lines, stayPlanLines(m.chrome, shown)...)
	lines = append(lines,
		"",
		m.modeKey("2", "POWER OFF — lease only, the GPUs sleep", m.stayMode == game.StayPowerOff),
		"     cash after "+game.FormatMoney(pvOff.CashAfter)+"   the rack cools",
		"",
	)
	if pvCurtail.Valid {
		lines = append(lines,
			m.modeKey("3", "CURTAIL — take the grid operator's money", m.stayMode == game.StayCurtail),
			"     they pay "+game.FormatMoney(pvCurtail.Curtail)+" for a silent night"+
				"   cash after "+game.FormatMoney(pvCurtail.CashAfter))
	} else {
		lines = append(lines, m.chrome.Muted("[3] curtail — no offer standing here tonight"))
	}
	if shown.Clawback > 0 {
		lines = append(lines, m.chrome.Warn(m.chrome.Bullet()+" an enterprise deal is under review:"+
			" clawback risk "+game.FormatMoney(shown.Clawback)))
	}
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	footer := "close the day on this plan"
	if b.Day >= content.DayCount {
		footer = "close the run on this plan"
	}
	return append(lines, m.boxFooter(footer)...)
}

func stayPlanLines(c Chrome, pv game.StayPreview) []string {
	if !pv.Valid {
		return []string{c.Bad("     refused: " + pv.Reason.String())}
	}
	out := []string{"     power " + kwStr(pv.PowerKW) + "/" + kwStr(pv.PowerCap) +
		" kW   hosted +" + game.FormatMoney(pv.Hosted) +
		"   electric -" + game.FormatMoney(pv.Electric)}
	if pv.Curtail > 0 {
		out[0] = "     power off by deal   the operator pays +" + game.FormatMoney(pv.Curtail)
	} else if pv.Mode == game.StayPowerOff {
		out[0] = "     power 0 kW   nothing runs"
	}
	tail := "     cash after " + game.FormatMoney(pv.CashAfter)
	if pv.Eligible && pv.Curtail == 0 && pv.Mode != game.StayPowerOff {
		tail += "   hosting " + itoa(pv.HostedSlots) + "/" + itoa(pv.SlotCap) + " slots"
	}
	if pv.Clawback > 0 {
		tail += "   clawback -" + game.FormatMoney(pv.Clawback)
	}
	return append(out, tail)
}

// subsetBox edits the powered set a card at a time. The validity read comes
// from the engine preview built from the exact counts on screen, so the meter
// verdict shown here is the verdict settlement will use.
func (m Model) subsetBox(b Board) []string {
	pv := m.run.PreviewStayMode(game.StayAuto, m.stayPick)
	lines := []string{
		m.chrome.Title("WHICH CARDS RUN TONIGHT?"),
		m.chrome.Muted(m.chrome.Rule(46)),
	}
	rows := m.run.QuoteBoard()
	picked := 0
	for i, row := range rows {
		if row.Owned <= 0 {
			continue
		}
		picked += m.stayPick[i] * row.Slots
		marker := m.chrome.Marker(i == m.staySel)
		lines = append(lines, marker+fmt.Sprintf("%-12s run %d of %d",
			row.Name, m.stayPick[i], row.Owned))
	}
	if len(rows) == 0 {
		lines = append(lines, m.chrome.Muted("Nothing to run."))
	}
	lines = append(lines, m.chrome.Muted(m.chrome.Rule(46)))
	hostLine := "hosting " + itoa(picked) + "/" + itoa(b.SlotsTotal) + " slots"
	if pv.Valid {
		lines = append(lines, "meter "+kwStr(pv.PowerKW)+"/"+kwStr(pv.PowerCap)+" kW"+
			"   "+hostLine+
			"   hosted +"+game.FormatMoney(pv.Hosted)+
			"   electric -"+game.FormatMoney(pv.Electric))
	} else {
		lines = append(lines, m.chrome.Bad("meter "+kwStr(pv.PowerKW)+"/"+kwStr(pv.PowerCap)+
			" kW   "+hostLine+" — "+pv.Reason.String()))
	}
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	lines = append(lines, m.chrome.Muted("[up]/[down] pick a card   [+] and [-] change the count"))
	return append(lines, m.boxFooter("take this set back to the night plan")...)
}

// allocBox is Andy's window: what he has, at what share of ask, and whether
// this player can reach it today. All of it is the engine's preview.
func (m Model) allocBox(b Board) []string {
	alloc := m.run.PreviewAllocation()
	if alloc.Product == "" {
		return append([]string{
			m.chrome.Title("ANDY'S ALLOCATION"),
			m.chrome.Muted(m.chrome.Rule(46)),
			"No allocation window is open here today.",
		}, m.boxFooter("go back")...)
	}
	name := content.ProductOf(alloc.Product).Name
	lines := []string{
		m.chrome.Title("ANDY'S ALLOCATION"),
		m.chrome.Muted(m.chrome.Rule(46)),
		name + ": " + itoa(alloc.Stock) + " left, " + itoa(alloc.Pct) + "% of the local ask",
		"unit price " + game.FormatMoney(alloc.UnitPrice) +
			"   full ask " + game.FormatMoney(alloc.Ask),
	}
	if !alloc.Eligible {
		lines = append(lines, m.chrome.Bad(m.chrome.Bullet()+" "+alloc.BlockReason))
	}
	if !alloc.Eligible {
		return append(lines, m.chrome.Muted("[esc] leave them in the crate"))
	}
	lines = append(lines, "", "claim: "+itoa(m.allocQty())+" "+name+" = "+
		game.FormatMoney(alloc.UnitPrice*game.Money(m.allocQty()))+
		"   cash on hand "+game.FormatMoney(m.run.State.Cash))
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	return append(lines, m.boxFooter("claim them now (no day passes)")...)
}

func (m Model) allocQty() int {
	n, ok := parseQuantity(m.allocDraft)
	if !ok {
		return 0
	}
	return n
}

// encounterBox is the stop the calendar takes for you: the story as written,
// the exact fine, the disclosed odds, and the three doors. The engine drew
// its coin already; nothing on this screen leaks which way it landed.
func (m Model) encounterBox() []string {
	pv, ok := m.run.PreviewEncounter()
	if !ok {
		return nil
	}
	lines := []string{
		m.chrome.Title(pv.Title),
		m.chrome.Muted(m.chrome.Rule(46)),
		pv.Story,
		"",
	}
	if pv.Fine > 0 {
		lines = append(lines, "fine: "+game.FormatMoney(pv.Fine)+
			"   cash you hold: "+game.FormatMoney(m.run.State.Cash))
		if pv.DebtAfterFine > m.run.State.Debt {
			lines = append(lines, m.chrome.Warn(m.chrome.Bullet()+
				"paying eats the cash and the rest goes on the note: debt becomes "+
				game.FormatMoney(pv.DebtAfterFine)))
		}
	}
	if pv.Chance > 0 {
		lines = append(lines, pv.ContestNote)
	}
	if pv.LoseNote != "" {
		lines = append(lines, m.chrome.Muted(pv.LoseNote))
	}
	if pv.Named != "" {
		lines = append(lines, m.chrome.Muted("they point at one box: the "+
			content.ProductOf(pv.Named).Name+" with the broken corner."))
	}
	keys := m.chrome.Muted("[p] pay it")
	if pv.CanSurrender {
		keys += "   [s] hand over the goods"
	}
	if pv.Chance > 0 {
		keys += "   [c] contest (" + itoa(pv.Chance) + "%)"
	}
	lines = append(lines, "", keys)
	if m.status != "" {
		lines = append(lines, m.boxStatus())
	}
	return append(lines, m.chrome.Muted("The calendar is stopped: nothing else answers to a key."))
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
	if m.full {
		// The preview above is the auto plan; the night screen it opens next
		// lets the player pick a different one, so say what Enter really does.
		lines = append(lines, m.chrome.Muted("these are the AUTO numbers; enter opens the night plan"))
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
	if m.full {
		footer = "plan the night (auto)"
		if last {
			footer = "close the run on the AUTO plan"
		}
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
	lines := []string{
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
	}
	if m.full {
		lines = append(lines,
			"a                 Andy's allocation window, when one is open",
			"                  (in the quantity box a/b/c stay quick 1/5/max)",
			"e or enter        opens the NIGHT PLAN: 1 auto-power, 2 power off,",
			"                  3 curtail (when the grid operator is buying),",
			"                  p pick which cards run by hand",
			"p in the plan     the powered-card editor: +/- set each card line",
			"[p]ay [s]urrender at a police stop: pay the fine, hand over the",
			"[c]ontest           named goods, or gamble at the shown odds;",
			"                  the calendar waits for one of these three",
		)
	}
	lines = append(lines,
		"x                 write this run's trace out for a bug report",
		"S                 save and quit",
		"esc               cancel the current box, nothing changes",
		"?                 this help",
		"q or ctrl+c       quit",
		"",
		m.chrome.Muted("Buying, selling, borrowing, and repaying never cost a day."),
		m.chrome.Muted("Only a confirmed Travel or Stay advances the clock."),
		m.chrome.Muted("Resizing the window never advances the clock either."),
		m.chrome.Muted("Every accepted move is written to the save folder as it happens."),
		"",
		m.chrome.Muted("press any key to go back"),
	)
	return lines
}
