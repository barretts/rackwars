package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Chrome carries the two presentation switches the design requires: monochrome
// (--no-color / NO_COLOR) and ASCII (--ascii). Every string the interface draws
// goes through Chrome so both modes stay testable.
type Chrome struct {
	Color bool
	ASCII bool

	title lipgloss.Style
	muted lipgloss.Style
	head  lipgloss.Style
	sel   lipgloss.Style
	warn  lipgloss.Style
	bad   lipgloss.Style
	good  lipgloss.Style
	plain lipgloss.Style
	bold  lipgloss.Style
}

// NewChrome builds styles. Colors are plain ANSI-16 constants so the game never
// requires true color; selection is marked with a glyph and reverse video so it
// survives monochrome terminals.
func NewChrome(color, ascii bool) Chrome {
	c := Chrome{Color: color, ASCII: ascii}
	c.title = lipgloss.NewStyle().Bold(true)
	c.muted = lipgloss.NewStyle()
	c.head = lipgloss.NewStyle().Bold(true)
	c.sel = lipgloss.NewStyle().Reverse(true).Bold(true)
	c.warn = lipgloss.NewStyle().Bold(true)
	c.bad = lipgloss.NewStyle().Bold(true)
	c.good = lipgloss.NewStyle().Bold(true)
	c.plain = lipgloss.NewStyle()
	c.bold = lipgloss.NewStyle().Bold(true)
	if color {
		c.title = c.title.Foreground(lipgloss.BrightCyan)
		c.muted = c.muted.Foreground(lipgloss.BrightBlack)
		c.warn = c.warn.Foreground(lipgloss.Yellow)
		c.bad = c.bad.Foreground(lipgloss.Red)
		c.good = c.good.Foreground(lipgloss.Green)
	}
	return c
}

func (c Chrome) Title(s string) string { return c.title.Render(s) }
func (c Chrome) Muted(s string) string { return c.muted.Render(s) }
func (c Chrome) Head(s string) string  { return c.head.Render(s) }
func (c Chrome) Sel(s string) string   { return c.sel.Render(s) }
func (c Chrome) Warn(s string) string  { return c.warn.Render(s) }
func (c Chrome) Bad(s string) string   { return c.bad.Render(s) }
func (c Chrome) Good(s string) string  { return c.good.Render(s) }
func (c Chrome) Plain(s string) string { return c.plain.Render(s) }
func (c Chrome) Bold(s string) string  { return c.bold.Render(s) }

// Rule is a horizontal separator one cell wide per unit of width.
func (c Chrome) Rule(width int) string {
	if width < 0 {
		width = 0
	}
	if c.ASCII {
		return strings.Repeat("-", width)
	}
	return strings.Repeat("─", width)
}

// Marker is the row-selection indicator.
func (c Chrome) Marker(selected bool) string {
	switch {
	case !selected:
		return "  "
	case c.ASCII:
		return "> "
	default:
		return "\u00bb "
	}
}

// Bullet prefixes a warning line with text as well as color.
func (c Chrome) Bullet() string {
	return "[!]"
}
