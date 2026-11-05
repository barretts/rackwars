package game

import (
	"strconv"
)

// Money is whole dollars. Every engine value that represents cash, a price, a
// lease, freight, or interest is an int64 Money, so previews and settlement use
// identical arithmetic.
type Money = int64

// FormatMoney renders a dollar amount with thousands separators and a leading
// minus for negatives: -12345 -> "-$12,345".
func FormatMoney(m Money) string {
	neg := m < 0
	if neg {
		m = -m
	}
	digits := strconv.FormatInt(m, 10)
	var b []byte
	for i, c := range []byte(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, c)
	}
	out := "$" + string(b)
	if neg {
		out = "-" + out
	}
	return out
}
