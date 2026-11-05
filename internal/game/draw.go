package game

import (
	"hash/fnv"
	"strconv"
)

// Every random outcome in RACK WARS is derived from the run seed plus a key
// that names the decision, rather than from a mutable RNG stream. The
// consequences matter more than the style:
//
//   - a saved run needs no RNG state to be resumable, and a versioned save
//     cannot silently drift because a stream advanced an extra step;
//   - the same committed action on the same seed produces the same prices,
//     customs roll, and fire, so reloading is not a slot machine;
//   - a headless trace replays identically on Windows and macOS, because the
//     arithmetic below is pure integer arithmetic with no library RNG to
//     change format under us.
func mix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	z := x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func hashKey(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// drawUint64 returns the mix for a key. Keys are built by the helpers below and
// must never be reused for a different purpose, or two unrelated decisions
// would share a roll.
func (r *Run) drawUint64(key string) uint64 {
	return mix64(r.cfg.Seed ^ hashKey(key))
}

// draw returns 0 <= n < span. span must be positive.
func (r *Run) draw(key string, span int) int {
	if span <= 0 {
		return 0
	}
	return int(r.drawUint64(key) % uint64(span))
}

// drawRange returns an inclusive value in [lo, hi].
func (r *Run) drawRange(key string, lo, hi int) int {
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo + r.draw(key, hi-lo+1)
}

// chance returns true when a percent chance (0-100) hits.
func (r *Run) chance(key string, pct int) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return r.draw(key, 100) < pct
}

// priceKey names the daily variation roll for one card in one town on one day.
func priceKey(day int, market, product string) string {
	return "price|" + strconv.Itoa(day) + "|" + market + "|" + product
}

// stockKey names the replenishment roll for one card in one town on one day.
func stockKey(day int, market, product string) string {
	return "stock|" + strconv.Itoa(day) + "|" + market + "|" + product
}
