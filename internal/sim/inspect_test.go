package sim

import (
	"flag"
	"strconv"
	"testing"

	"rackwars/internal/content"
	"rackwars/internal/game"
)

// TestInspectTrace is a development view: it prints policy runs in full so a
// losing strategy can be read as a story rather than guessed at from totals.
func TestInspectTrace(t *testing.T) {
	seeds := []uint64{1}
	names := []string{"trade-repay", "hold", "leveraged"}
	if s := inspectSeed(t); s != 0 {
		seeds = []uint64{s}
	}
	if n := inspectPolicy(t); n != "" {
		names = []string{n}
	}
	for _, name := range names {
		var p Policy
		for _, cand := range All() {
			if cand.Name == name {
				p = cand
			}
		}
		if p.Next == nil {
			t.Fatalf("no policy named %q", name)
		}
		for _, seed := range seeds {
			cfg := Prototype(seed)
			o, run, steps, err := Run(cfg, p)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("\n%s seed %d\n%s\n%s", p.Name, seed, o.String(), game.TextTrace(run, steps))
		}
	}
}

// TestBoardWalk walks the map one town per morning and prints each board against
// the campaign factor the content says should be in force, so a strategy can be
// judged against what a player could actually see. The campaign column is
// developer-side knowledge: policies never read it.
func TestBoardWalk(t *testing.T) {
	seed := uint64(1)
	if s := inspectSeed(t); s != 0 {
		seed = s
	}
	r, err := game.New(game.Config{Seed: seed, Products: content.PrototypeProducts, Markets: content.PrototypeMarkets, Rules: content.PrototypeRules()})
	if err != nil {
		t.Fatal(err)
	}
	markets := content.PrototypeMarkets
	next := 0
	for day := 1; day <= content.DayCount && r.State.Over == game.EndOpen; day++ {
		t.Logf("day %2d %-10s cash %s debt %s", day, r.State.Market, game.FormatMoney(r.State.Cash), game.FormatMoney(r.State.Debt))
		for _, row := range r.QuoteBoard() {
			factor := campaignFactorFor(r, day, r.State.Market, row.Product)
			t.Logf("      %-9s ask %8s (%4d) bid %8s (%4d) stock %2d  campaign %d%%",
				row.Name, game.FormatMoney(row.Ask), rel(row.Ask, r.State.Market, row.Product),
				game.FormatMoney(row.Bid), rel(row.Bid, r.State.Market, row.Product), row.Stock, factor)
		}
		next = (next + 1) % len(markets)
		if markets[next] == r.State.Market {
			next = (next + 1) % len(markets)
		}
		if _, res := r.CommitDay(game.Action{Kind: game.ActTravel, Market: markets[next]}); !res.OK {
			if _, res := r.CommitDay(game.Action{Kind: game.ActStay}); !res.OK {
				t.Fatalf("day %d: %s", day, res.Reason)
			}
		}
	}
}

// campaignFactorFor multiplies the content's price factors in force, for the
// probe's right-hand column only.
func campaignFactorFor(r *game.Run, day int, m content.MarketID, p content.ProductID) int {
	pct := 100
	for _, h := range content.Campaign() {
		if day < h.Day {
			continue
		}
		for _, e := range h.Effects {
			if e.Kind != content.EffectPriceFactor || day >= h.Day+e.Days || !e.Matches(m, p) {
				continue
			}
			pct = pct * e.Pct / 100
		}
	}
	return pct
}

func rel(price game.Money, m content.MarketID, p content.ProductID) int64 {
	base := game.Money(content.ProductOf(p).BaseMidpoint) * game.Money(content.MarketOf(m).TownFactorPct) / 100
	if base <= 0 {
		return 0
	}
	return int64(price) * 1000 / int64(base)
}

func inspectSeed(t *testing.T) uint64 {
	t.Helper()
	f := flag.Lookup("seed")
	if f == nil {
		return 0
	}
	v, _ := strconv.ParseUint(f.Value.String(), 10, 64)
	return v
}

func inspectPolicy(t *testing.T) string {
	t.Helper()
	if f := flag.Lookup("policy"); f != nil {
		return f.Value.String()
	}
	return ""
}
