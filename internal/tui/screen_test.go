package tui

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// screen is a deliberately small terminal emulator: it reconstructs what a real
// terminal would be showing from the renderer's byte stream. Bubble Tea repaints
// only the cells that changed, so asserting on raw output bytes proves nothing
// about the picture -- this rebuilds the grid instead.
//
// It understands exactly what the renderer emits: cursor addressing (absolute
// and relative), erase line/screen, the alternate-screen and mode switches
// (ignored), graphic renditions (ignored, colour is tested separately), and the
// C0 controls. Anything else is skipped, because the game must never need an
// escape sequence this program has not been taught to place on screen.
type screen struct {
	mu   sync.Mutex
	w    int
	h    int
	rows [][]rune
	cx   int
	cy   int
}

func newScreen(w, h int) *screen {
	s := &screen{w: w, h: h, rows: make([][]rune, h)}
	for i := range s.rows {
		s.rows[i] = blankRunes(w)
	}
	return s
}

// Write implements io.Writer; the renderer calls it from its own goroutine.
func (s *screen) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feed(string(p))
	return len(p), nil
}

// Flush implements the renderer's flusher interface.
func (s *screen) Flush() error { return nil }

func (s *screen) feed(in string) {
	for i := 0; i < len(in); {
		c := in[i]
		switch {
		case c == 0x1b:
			used, ok := s.escape(in[i:])
			if !ok {
				i++
				continue
			}
			i += used
		case c == '\r':
			s.cx = 0
			i++
		case c == '\n' || c == '\v' || c == '\f':
			s.cy++
			s.scrollIfNeeded()
			i++
		case c == '\b':
			if s.cx > 0 {
				s.cx--
			}
			i++
		case c == '\t':
			s.cx = min(((s.cx/8)+1)*8, s.w-1)
			i++
		case c < 0x20:
			i++
		default:
			r, size := utf8.DecodeRuneInString(in[i:])
			i += size
			s.put(r)
		}
	}
}

// put writes one rune at the cursor and advances it, wrapping like a terminal
// with autowrap on.
func (s *screen) put(r rune) {
	if s.cx >= s.w {
		s.cx = 0
		s.cy++
		s.scrollIfNeeded()
	}
	if s.cy < 0 || s.cy >= s.h || s.cx < 0 || s.cx >= s.w {
		return
	}
	s.rows[s.cy][s.cx] = r
	s.cx += max(1, ansi.StringWidth(string(r)))
}

func blankRunes(n int) []rune {
	row := make([]rune, n)
	blankInto(row)
	return row
}

// blankInto clears a row to spaces: a real terminal shows blanks, and the
// renderer never writes a space over a blank it already matches.
func blankInto(row []rune) {
	for i := range row {
		row[i] = ' '
	}
}

func (s *screen) scrollIfNeeded() {
	if s.cy < s.h {
		return
	}
	s.cy = s.h - 1
	for y := 0; y < s.h-1; y++ {
		copy(s.rows[y], s.rows[y+1])
	}
	blankInto(s.rows[s.h-1])
}

// escape consumes one escape sequence at the front of in and reports how many
// bytes it took. Unknown sequences are consumed and ignored.
func (s *screen) escape(in string) (int, bool) {
	if len(in) < 2 {
		return 0, false
	}
	switch in[1] {
	case '[':
		i := 2
		for i < len(in) && (in[i] == '?' || in[i] == ';' || (in[i] >= '0' && in[i] <= '9') || in[i] == '>') {
			i++
		}
		if i == len(in) {
			return 0, false
		}
		final := in[i]
		params := in[2:i]
		s.csi(final, params)
		return i + 1, true
	case ']': // operating system command: terminated by BEL or ST
		for i := 2; i < len(in); i++ {
			if in[i] == 0x07 {
				return i + 1, true
			}
			if in[i] == 0x1b && i+1 < len(in) && in[i+1] == '\\' {
				return i + 2, true
			}
		}
		return 0, false
	case '(', ')': // character set selection
		if len(in) < 3 {
			return 0, false
		}
		return 3, true
	default: // ESC M, ESC 7, ESC =, and friends we do not need
		return 2, true
	}
}

func (s *screen) csi(final byte, params string) {
	nums := parseParams(params)
	n := func(i int) int {
		if i < len(nums) && nums[i] != 0 {
			return nums[i]
		}
		return 1
	}
	switch final {
	case 'H', 'f':
		s.cy = n(0) - 1
		s.cx = n(1) - 1
	case 'A':
		s.cy -= count(nums, 1)
	case 'B':
		s.cy += count(nums, 1)
	case 'C':
		s.cx += count(nums, 1)
	case 'D':
		s.cx -= count(nums, 1)
	case 'E':
		s.cy += count(nums, 1)
		s.cx = 0
	case 'F':
		s.cy -= count(nums, 1)
		s.cx = 0
	case 'G':
		s.cx = n(0) - 1
	case 'd':
		s.cy = n(0) - 1
	case 'J':
		s.eraseDisplay(count(nums, 0))
	case 'K':
		s.eraseLine(count(nums, 0))
	}
	// m (colour), h/l (modes, including the alternate screen), s/u (save/restore
	// cursor), n (report), and anything private are deliberately ignored here:
	// colour has its own test, and the picture must not depend on either.
}

func count(nums []int, def int) int {
	if len(nums) > 0 {
		return nums[0]
	}
	return def
}

func parseParams(p string) []int {
	if p == "" {
		return nil
	}
	parts := strings.Split(p, ";")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		v := 0
		digits := strings.TrimPrefix(part, "?")
		for _, c := range digits {
			if c < '0' || c > '9' {
				v = 0
				break
			}
			v = v*10 + int(c-'0')
		}
		out = append(out, v)
	}
	return out
}

func (s *screen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		for x := s.cx; x < s.w && s.cy < s.h; x++ {
			s.rows[s.cy][x] = ' '
		}
		for y := s.cy + 1; y < s.h; y++ {
			blankInto(s.rows[y])
		}
	case 1:
		for y := 0; y < s.cy && y < s.h; y++ {
			blankInto(s.rows[y])
		}
		if s.cy < s.h {
			for x := 0; x <= s.cx && x < s.w; x++ {
				s.rows[s.cy][x] = ' '
			}
		}
	default:
		for y := range s.rows {
			blankInto(s.rows[y])
		}
	}
}

func (s *screen) eraseLine(mode int) {
	if s.cy >= s.h || s.cy < 0 {
		return
	}
	switch mode {
	case 0:
		for x := s.cx; x < s.w; x++ {
			s.rows[s.cy][x] = ' '
		}
	case 1:
		for x := 0; x <= s.cx && x < s.w; x++ {
			s.rows[s.cy][x] = ' '
		}
	default:
		blankInto(s.rows[s.cy])
	}
}

// text is the visible screen: trailing blanks trimmed, rows joined.
func (s *screen) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := make([]string, len(s.rows))
	for i, row := range s.rows {
		lines[i] = strings.TrimRight(string(row), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// selectedLine returns the screen row carrying the selection marker, which is
// the one thing on the board that must survive colour being turned off.
func selectedLine(screen string) (string, bool) {
	for _, l := range strings.Split(screen, "\n") {
		if strings.HasPrefix(l, ">") || strings.HasPrefix(l, "»") {
			return l, true
		}
	}
	return "", false
}

// waitUntil polls the visible screen until pred holds or the timeout passes.
func (s *screen) waitUntil(timeout time.Duration, pred func(string) bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if pred(s.text()) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(15 * time.Millisecond)
	}
}
