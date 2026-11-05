package game

import "testing"

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		in   Money
		want string
	}{
		{0, "$0"},
		{1, "$1"},
		{999, "$999"},
		{1000, "$1,000"},
		{12345, "$12,345"},
		{1_000_000, "$1,000,000"},
		{123_456_789_012, "$123,456,789,012"},
		{-1, "-$1"},
		{-12345, "-$12,345"},
		{-1_000_000_000, "-$1,000,000,000"},
	}
	for _, c := range cases {
		if got := FormatMoney(c.in); got != c.want {
			t.Errorf("FormatMoney(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
