package tui

import (
	"rackwars/internal/content"
	"rackwars/internal/game"
)

// stressFallPct is the bid drop the lender panel models. It is the design's
// "what if they all fall twenty percent" question, asked of the engine, which
// answers with its own collateral, ceiling, and breach numbers.
const stressFallPct = 20

// sideLines builds the wide-terminal detail panel from engine read accessors.
// Nothing here is computed by the interface: every figure is one the engine
// already uses internally.
func (m Model) sideLines() []string {
	if m.run == nil {
		return nil
	}
	r := m.run
	st := r.State

	out := []string{"HOLDINGS"}
	held := 0
	for _, row := range r.QuoteBoard() {
		if row.Owned <= 0 {
			continue
		}
		held++
		out = append(out, "  "+row.Name+" x"+itoa(row.Owned)+" avg "+game.FormatMoney(row.AvgCost))
	}
	if held == 0 {
		out = append(out, "  nothing yet")
	}

	out = append(out, "", "SHORELINE CREDIT",
		"  debt       "+game.FormatMoney(st.Debt),
		"  collateral "+game.FormatMoney(r.CollateralValue()),
		"  ceiling    "+game.FormatMoney(r.CreditCeiling()),
		"  headroom   "+game.FormatMoney(r.AvailableCredit()),
		"  tomorrow   "+game.FormatMoney(r.TomorrowInterest())+" interest")
	s := r.LenderStress(stressFallPct)
	out = append(out,
		"  if bids -"+itoa(stressFallPct)+"%: ceiling "+game.FormatMoney(s.Ceiling))
	if s.ProjectedBreach > 0 {
		out = append(out, "  breach then "+game.FormatMoney(s.ProjectedBreach))
	}

	if len(m.dayLog) > 0 {
		out = append(out, "")
		out = append(out, m.dayLog...)
	}

	// The panel must never outgrow the screen: rows that do not fit would push
	// the board off the terminal, so the least important lines go first.
	limit := m.height
	if limit <= 0 {
		limit = 22
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// knownQuoteLines reports the last-seen prices in another town, tagged with the
// day they were seen. The design forbids presenting them as arrival prices, so
// the day is part of every line.
func (m Model) knownQuoteLines(to content.MarketID) []string {
	seen := m.run.SeenDayOf(to)
	if seen <= 0 {
		return []string{"  no quotes yet: you have never been there"}
	}
	out := []string{"  last seen day " + itoa(seen) + " (not an arrival price):"}
	for _, p := range m.run.Products() {
		q, ok := m.run.QuoteOf(to, p)
		if !ok {
			continue
		}
		out = append(out, "    "+padName(content.ProductOf(p).Name)+
			" ask "+game.FormatMoney(q.Ask)+"  bid "+game.FormatMoney(q.Bid))
	}
	if len(out) == 1 {
		return []string{"  no quotes yet: you have never been there"}
	}
	return out
}

func padName(s string) string {
	for len(s) < 11 {
		s += " "
	}
	return s
}

// signedMoney prints a realised gain or loss with its sign, because "profit
// $-3,284" reads as a number the player has to decode.
func signedMoney(v game.Money) string {
	if v > 0 {
		return "+" + game.FormatMoney(v)
	}
	return game.FormatMoney(v)
}
