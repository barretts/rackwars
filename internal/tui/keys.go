package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

func (m Model) handleBoardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		// Quitting needs no extra write: every state change was saved as it
		// happened. What quitting does owe the next window is giving up the claim
		// on the run, so the save opens again without a stale marker.
		m.releaseSave()
		return m, tea.Quit
	case "S":
		// Save and quit. With a store attached this is also the retry key after a
		// failed autosave.
		return m.saveAndQuit()
	case "x":
		return m.exportTrace(), nil
	case "?":
		m.mode, m.status = modeHelp, ""
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
		m.mode, m.qtyDraft, m.status = modeQty, "", ""
		return m, nil
	case "l":
		m.mode, m.loanDraft, m.loanAction, m.status = modeLender, "", loanBorrow, ""
		return m, nil
	case "t":
		if len(m.otherMarkets()) == 0 {
			m.status, m.statusBad = "There is nowhere else to go in this cast.", true
			return m, nil
		}
		m.mode, m.travelSel, m.status = modeTravel, 0, ""
		return m, nil
	case "e", "enter", "ctrl+m":
		// Committing a day always goes through the confirm box, so one key press
		// can never close a day by accident and a held Enter can never close two.
		m.mode, m.status = modeConfirm, ""
		return m, nil
	case "esc":
		m.status = ""
		return m, nil
	}
	return m, nil
}

// apply hands one morning action to the engine and reports what came back. The
// TUI never inspects state to decide whether the action was legal; it reports
// the engine's answer, in the engine's numbers.
func (m Model) apply(a game.Action) Model {
	rec, res := m.run.Apply(a)
	if !res.OK {
		m.status, m.statusBad = refusal(a, res), true
		return m
	}
	m.status, m.statusBad = receipt(a, rec), false
	// The engine accepted a complete state change, so this is a position worth
	// putting on disk: record it in the run's history and autosave. A failed write
	// reports itself and changes nothing else.
	m = m.recordAction(a)
	return m.writeSave()
}

// commitDay is the only path that moves time. It closes the box it came from
// before it commits, so the next Enter starts a new confirmation instead of a
// second day.
func (m Model) commitDay(a game.Action) Model {
	m.mode = modeBoard
	closing := m.run.State.Day
	rec, res := m.run.CommitDay(a)
	if !res.OK {
		m.status, m.statusBad = refusal(a, res), true
		return m
	}
	m.dayLog = dayLines(closing, rec)
	m.status, m.statusBad = receipt(a, rec), false
	// A closed day is the strongest state boundary there is: the history is
	// recorded and the board is put on disk before the next morning is drawn.
	m = m.recordAction(a)
	m = m.writeSave()
	if m.over() {
		m.mode = modeEnd
		if m.saveErr == "" {
			m.status = ""
		}
	}
	return m
}

// refusal explains a rejected action using the exact gap the engine measured.
func refusal(a game.Action, res game.Result) string {
	switch res.Reason {
	case game.ReasonInsufficientCash:
		return "That costs more than you have: short " + game.FormatMoney(res.ShortCash) + "."
	case game.ReasonOvercapacity:
		return "That will not fit in the rack: " + itoa(res.ShortSlots) + " slot" +
			plural(res.ShortSlots) + " short."
	case game.ReasonInsufficientStock:
		return "The shop cannot sell you that many: " + itoa(res.ShortStock) + " short."
	case game.ReasonInsufficientOwned:
		return "You do not hold that many: the most you can sell is " + itoa(res.MaxAccepted) + "."
	case game.ReasonInsufficientCredit:
		return "The lender will not advance that much: the most left is " + game.FormatMoney(res.MaxAmount) + "."
	case game.ReasonBadQuantity:
		if a.Kind == game.ActRepay || a.Kind == game.ActBorrow {
			return "That amount does not work: enter a whole number of dollars."
		}
		return "Enter a whole number of units, or Esc to cancel."
	default:
		return "Refused: " + res.Reason.String() + "."
	}
}

// receipt reports what the engine did, in one line.
func receipt(a game.Action, rec game.Receipt) string {
	name := ""
	if a.Product != "" {
		name = content.ProductOf(a.Product).Name
	}
	switch a.Kind {
	case game.ActBuy:
		return "Bought " + itoa(rec.Qty) + " " + name + " for " + game.FormatMoney(rec.Cost) +
			"; cash " + game.FormatMoney(rec.CashAfter) + "."
	case game.ActSell:
		out := "Sold " + itoa(rec.Qty) + " " + name + " for " + game.FormatMoney(rec.Gross)
		if rec.ToLender > 0 {
			out += "; the lender took " + game.FormatMoney(rec.ToLender)
		}
		return out + "; you kept " + game.FormatMoney(rec.CashIn) +
			"; realized " + signedMoney(rec.Realized) + "."
	case game.ActBorrow:
		return "Borrowed " + game.FormatMoney(rec.Cost) + "; debt " + game.FormatMoney(rec.DebtAfter) + "."
	case game.ActRepay:
		return "Repaid " + game.FormatMoney(rec.Cost) + "; debt " + game.FormatMoney(rec.DebtAfter) + "."
	case game.ActTravel:
		// CommitDay stamps a close receipt with the morning that follows, so the
		// stamp is already the day the player is now standing in.
		return "Traveled to " + rec.MarketName + "; freight " + game.FormatMoney(rec.Freight) +
			"; interest " + game.FormatMoney(rec.Interest) + "; now day " + itoa(rec.Day) + "."
	case game.ActStay:
		return "Stayed the night; interest " + game.FormatMoney(rec.Interest) + "; now day " +
			itoa(rec.Day) + "."
	}
	return "Done."
}

// dayLines renders the day that just closed. The engine stamps a close receipt
// with the morning that follows (its headline belongs to that morning), so the
// label names the day the player actually committed, passed in by commitDay.
func dayLines(closing int, rec game.Receipt) []string {
	out := []string{"DAY " + itoa(closing) + " CLOSE"}
	if rec.Freight > 0 {
		out = append(out, "  freight     "+game.FormatMoney(rec.Freight))
	}
	if rec.Lease > 0 {
		out = append(out, "  lease       "+game.FormatMoney(rec.Lease))
	}
	if rec.Interest > 0 {
		out = append(out, "  interest    "+game.FormatMoney(rec.Interest))
	}
	if rec.Hosted > 0 {
		out = append(out, "  hosting     +"+game.FormatMoney(rec.Hosted))
	}
	for _, f := range rec.Forced {
		out = append(out, "  FORCED SALE "+itoa(f.Units)+" "+content.ProductOf(f.Product).Name+
			" at "+game.FormatMoney(f.UnitBid)+", "+game.FormatMoney(f.LossVsCost)+" under cost")
	}
	if rec.Repossessed {
		out = append(out, "  REPOSSESSED")
	}
	if rec.Closed {
		out = append(out, "  day 30 close: cash out "+game.FormatMoney(rec.CashOut))
	}
	out = append(out, "  cash "+game.FormatMoney(rec.CashAfter)+"  debt "+game.FormatMoney(rec.DebtAfter))
	if rec.Headline != "" {
		out = append(out, "  morning:    "+rec.Headline)
	}
	return out
}

func (m Model) handleQtyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode, m.qtyDraft = modeBoard, ""
		return m, nil
	case "backspace":
		if len(m.qtyDraft) > 0 {
			m.qtyDraft = m.qtyDraft[:len(m.qtyDraft)-1]
		}
		return m, nil
	case "a", "b", "c":
		switch key {
		case "a":
			m.qtyDraft = "1"
		case "b":
			m.qtyDraft = "5"
		case "c":
			m.qtyDraft = itoa(m.qtyLimit())
		}
		m.status, m.statusBad = "", false
		return m, nil
	case "enter", "ctrl+m":
		n, ok := parseQuantity(m.qtyDraft)
		if !ok || n < 1 {
			m.status, m.statusBad = "Enter a whole number of units, or Esc to cancel.", true
			return m, nil
		}
		a := game.Action{Kind: game.ActBuy, Product: m.selectedProduct(), Qty: n}
		if m.qtyAction == actionSell {
			a.Kind = game.ActSell
		}
		before := len(m.actions)
		m = m.apply(a)
		if len(m.actions) == before {
			// The engine said no: keep the number on screen so the player can fix
			// it, and put the reason inside the box, which covers the status line.
			m.mode = modeQty
			return m, nil
		}
		m.mode, m.qtyDraft = modeBoard, ""
		return m, nil
	}
	if r, ok := digitOf(msg); ok && len(m.qtyDraft) < 7 {
		m.qtyDraft += string(r)
		m.status, m.statusBad = "", false
	}
	return m, nil
}

// qtyLimit asks the engine for the largest quantity it would accept for the
// selected card and trade direction. The interface holds no clamp of its own.
func (m Model) qtyLimit() int {
	p := m.selectedProduct()
	if m.qtyAction == actionSell {
		return m.run.SellLimits(p).Max
	}
	return m.run.BuyLimits(p).Max
}

func (m Model) handleLenderKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode, m.loanDraft = modeBoard, ""
		return m, nil
	case "b", "r":
		m.loanAction = loanBorrow
		if key == "r" {
			m.loanAction = loanRepay
		}
		m.loanDraft, m.status, m.statusBad = "", "", false
		return m, nil
	case "a":
		// Fill the whole amount the box is for: all remaining credit, or the
		// whole note up to the cash on hand. Both numbers are the engine's.
		st := m.run.State
		amount := m.run.AvailableCredit()
		if m.loanAction == loanRepay {
			amount = st.Debt
			if st.Cash < amount {
				amount = st.Cash
			}
		}
		if amount < 0 {
			amount = 0
		}
		m.loanDraft = moneyDigits(amount)
		m.status, m.statusBad = "", false
		return m, nil
	case "backspace":
		if len(m.loanDraft) > 0 {
			m.loanDraft = m.loanDraft[:len(m.loanDraft)-1]
		}
		return m, nil
	case "enter", "ctrl+m":
		amount, ok := parseQuantity(m.loanDraft)
		if !ok || amount < 1 {
			m.status, m.statusBad = "Enter a whole number of dollars, or Esc to cancel.", true
			return m, nil
		}
		a := game.Action{Kind: game.ActBorrow, Amount: game.Money(amount)}
		if m.loanAction == loanRepay {
			a.Kind = game.ActRepay
		}
		before := len(m.actions)
		m = m.apply(a)
		if len(m.actions) == before {
			m.mode = modeLender
			return m, nil
		}
		m.mode, m.loanDraft = modeBoard, ""
		return m, nil
	}
	if r, ok := digitOf(msg); ok && len(m.loanDraft) < 9 {
		m.loanDraft += string(r)
		m.status, m.statusBad = "", false
	}
	return m, nil
}

func (m Model) handleTravelKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	opts := m.otherMarkets()
	switch msg.String() {
	case "esc":
		m.mode = modeBoard
		return m, nil
	case "up", "k":
		m.travelSel = wrap(m.travelSel-1, len(opts))
		return m, nil
	case "down", "j":
		m.travelSel = wrap(m.travelSel+1, len(opts))
		return m, nil
	case "enter", "ctrl+m":
		dest := m.destMarket()
		if dest == "" {
			m.status, m.statusBad = "No town is selected.", true
			return m, nil
		}
		return m.commitDay(game.Action{Kind: game.ActTravel, Market: dest}), nil
	default:
		// Number keys pick a road outright: an arrow key is not the only way to
		// choose, and over ssh/ConPTY links an escape sequence is not reliable.
		if d, ok := digitOf(msg); ok && d >= '1' && d <= '9' {
			n := int(d - '0')
			if n > len(opts) {
				m.status, m.statusBad = "There is no road number "+itoa(n)+
					" from here; the roads are numbered 1 to "+itoa(len(opts))+".", true
				return m, nil
			}
			m.travelSel = n - 1
			m.status, m.statusBad = "", false
			return m, nil
		}
		return m, nil
	}
}

func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBoard
		return m, nil
	case "enter", "ctrl+m":
		return m.commitDay(game.Action{Kind: game.ActStay}), nil
	}
	return m, nil
}

func (m Model) handleEndKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n":
		// New Run deals the next seed; Replay This Seed re-deals this one, which
		// is how a bug report gets re-created exactly as it happened.
		next, err := m.startRun(m.seed + 1)
		if err != nil {
			m.status, m.statusBad = "Cannot start a new run: "+err.Error(), true
			return m, nil
		}
		m = next
		m.status = "New run, seed " + itoa64(int64(m.seed)) + "."
		return m, nil
	case "r":
		next, err := m.startRun(m.seed)
		if err != nil {
			m.status, m.statusBad = "Cannot replay this seed: "+err.Error(), true
			return m, nil
		}
		m = next
		m.status = "Replaying seed " + itoa64(int64(m.seed)) + "."
		return m, nil
	case "x":
		// The trace of a finished run is the bug report; the ending on screen is
		// what the replay reproduces.
		return m.exportTrace(), nil
	case "q":
		m.releaseSave()
		return m, tea.Quit
	}
	return m, nil
}

// handleContinueKey drives the start-up list of saved runs. Enter resumes the
// highlighted run, n starts a fresh one, and q leaves without touching anything
// on disk. A save that refuses to load stays refused: its reason is shown and the
// player keeps the list, the other runs, and the new-run key.
func (m Model) handleContinueKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.saveSel = wrap(m.saveSel-1, len(m.saves))
		m.status, m.statusBad = "", false
		return m, nil
	case "down", "j":
		m.saveSel = wrap(m.saveSel+1, len(m.saves))
		m.status, m.statusBad = "", false
		return m, nil
	case "enter", "ctrl+m":
		next, err := m.continueRun()
		if err != nil {
			m.status, m.statusBad = "That run cannot be opened: "+err.Error(), true
			return m, nil
		}
		return next, nil
	case "n":
		// A new run is dealt on the seed this launch was asked for, which is the
		// seed the continue list was offering by default.
		next, err := m.startRun(m.seed)
		if err != nil {
			m.status, m.statusBad = "Cannot start a new run: "+err.Error(), true
			return m, nil
		}
		next.status = "New run, seed " + itoa64(int64(next.seed)) + ". The saved runs are still on disk."
		return next, nil
	case "q":
		m.releaseSave()
		return m, tea.Quit
	case "r":
		// Recovery from the backup copy the store keeps beside every save. This is
		// the answer to a damaged file, and on a healthy run it is simply the
		// previous position of the same run.
		next, err := m.recoverRun()
		if err != nil {
			m.status, m.statusBad = "Nothing to recover: "+err.Error(), true
			return m, nil
		}
		return next, nil
	}
	if d, ok := digitOf(msg); ok && d >= '1' && d <= '9' {
		n := int(d - '0')
		if n > len(m.saves) {
			m.status, m.statusBad = "There is no run number "+itoa(n)+"; the list holds "+itoa(len(m.saves))+".", true
			return m, nil
		}
		m.saveSel = n - 1
		m.status, m.statusBad = "", false
		return m, nil
	}
	return m, nil
}

func (m Model) handleHelpKey(_ tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.mode = modeBoard
	return m, nil
}

func (m Model) moveSelection(delta int) Model {
	if m.run == nil {
		return m
	}
	n := len(m.run.Products())
	if n == 0 {
		return m
	}
	m.sel = wrap(m.sel+delta, n)
	return m
}

func wrap(i, n int) int {
	if n <= 0 {
		return 0
	}
	return (i%n + n) % n
}

func (m Model) selectedProduct() content.ProductID {
	if m.run == nil || m.sel < 0 || m.sel >= len(m.run.Products()) {
		return ""
	}
	return m.run.Products()[m.sel]
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

func moneyDigits(v game.Money) string {
	if v <= 0 {
		return "0"
	}
	if t := strings.TrimLeft(itoa64(int64(v)), "0"); t != "" {
		return t
	}
	return "0"
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
