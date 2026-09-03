package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// ---------- theme (bright pink + lavender, true color) ----------

const (
	hexPink     = "#ff5fd7" // primary accent
	hexPinkSoft = "#ff9ad1"
	hexLav      = "#af87ff" // secondary accent
	hexLavDim   = "#8b7cc9"
	hexBgPage   = "#0f0c14" // page background
	hexBgCard   = "#171320" // card background
	hexBgCardHi = "#1f1a2c" // hovered/selected card background
	hexBorder   = "#332c47" // idle card border
	hexBorderHi = "#af87ff" // focused card border (lavender)
	hexText     = "#f3f0fa"
	hexDim      = "#8a80a8"
	hexTrack    = "#2a2438" // gauge track (unfilled)
	hexGreen    = "#4ade80"
	hexAmber    = "#fbbf24"
	hexRed      = "#fb7185"
)

var (
	colPink     = lipgloss.Color(hexPink)
	colPinkSoft = lipgloss.Color(hexPinkSoft)
	colLav      = lipgloss.Color(hexLav)
	colLavDim   = lipgloss.Color(hexLavDim)
	colBgCard   = lipgloss.Color(hexBgCard)
	colBgCardHi = lipgloss.Color(hexBgCardHi)
	colBorder   = lipgloss.Color(hexBorder)
	colBorderHi = lipgloss.Color(hexBorderHi)
	colText     = lipgloss.Color(hexText)
	colDim      = lipgloss.Color(hexDim)
	colGreen    = lipgloss.Color(hexGreen)
	colAmber    = lipgloss.Color(hexAmber)
	colRed      = lipgloss.Color(hexRed)

	subtleStyle = lipgloss.NewStyle().Foreground(colDim)

	cardBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	cardStyle = lipgloss.NewStyle().
			Border(cardBorder).
			BorderForeground(colBorder).
			Background(colBgCard).
			Padding(0, 1)

	cardStyleFocus = cardStyle.
			BorderForeground(colBorderHi)
)

// ---------- gradient helpers ----------

func mustColorful(hex string) colorful.Color {
	c, _ := colorful.Hex(hex)
	return c
}

func gradientText(s string, from, to string, bold bool) string {
	c1, c2 := mustColorful(from), mustColorful(to)
	runes := []rune(s)
	n := len(runes)
	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		c := c1.BlendLuv(c2, t)
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		if bold {
			st = st.Bold(true)
		}
		b.WriteString(st.Render(string(r)))
	}
	return b.String()
}

// gradientBar renders a filled/track progress bar with a smooth color ramp across the fill.
func gradientBar(pct float64, width int, from, to string) string {
	if width < 1 {
		width = 1
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	c1, c2 := mustColorful(from), mustColorful(to)
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			t := 0.0
			if width > 1 {
				t = float64(i) / float64(width-1)
			}
			c := c1.BlendLuv(c2, t)
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Render("█"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(hexTrack)).Render("╌"))
		}
	}
	return b.String()
}

// pctGradient picks a bar gradient endpoint based on how hot the value is.
func pctGradient(pct float64) (string, string) {
	switch {
	case pct >= 85:
		return hexPink, hexRed
	case pct >= 60:
		return hexLav, hexAmber
	default:
		return hexLavDim, hexLav
	}
}

// padOpaque force-pads every line of s to width with an explicitly backgrounded
// blank run. Content mixing lipgloss styling with foreign raw-ANSI (asciigraph's
// per-line output, in particular) can leave short lines under-padded, which on
// a translucent terminal shows whatever sits behind the window through the gap
// and, combined with per-tick full redraws, reads as a flickering "ghost" smear.
// Painting every cell ourselves avoids depending on any library's width guess.
func padOpaque(s string, width int, bg lipgloss.Color) string {
	fill := lipgloss.NewStyle().Background(bg)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w < width {
			lines[i] = line + fill.Render(strings.Repeat(" ", width-w))
		}
	}
	return strings.Join(lines, "\n")
}

func pill(text string, bg, fg lipgloss.Color, bold bool) string {
	st := lipgloss.NewStyle().Background(bg).Foreground(fg).Padding(0, 1)
	if bold {
		st = st.Bold(true)
	}
	return st.Render(text)
}

func sectionTitle(s string) string {
	return lipgloss.NewStyle().Foreground(colLav).Bold(true).Render(s)
}

func statusColor(s string) lipgloss.Color {
	switch s {
	case "up":
		return colGreen
	case "warn":
		return colAmber
	case "down":
		return colRed
	}
	return colDim
}

func statusPill(s string) string {
	label := map[string]string{"up": " UP ", "warn": " WARN ", "down": " DOWN "}[s]
	if label == "" {
		label = " ? "
	}
	return pill(label, statusColor(s), lipgloss.Color(hexBgPage), true)
}
