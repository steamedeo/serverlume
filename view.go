package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

// ---------- top-level layout ----------

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "loading..."
	}

	page := lipgloss.NewStyle().Background(lipgloss.Color(hexBgPage))

	header := m.renderHeader()
	footer := m.renderFooter()

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	sidebarWidth := 32
	gap := 1
	mainWidth := m.width - sidebarWidth - gap
	if mainWidth < 20 {
		mainWidth = 20
	}

	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	main := m.renderMain(mainWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, strings.Repeat(" ", gap), main)

	full := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
	return page.Width(m.width).Height(m.height).Render(full)
}

func (m model) renderHeader() string {
	up, warn, down := 0, 0, 0
	for _, s := range m.servers {
		switch s.status {
		case "up":
			up++
		case "warn":
			warn++
		case "down":
			down++
		}
	}
	brand := gradientText("✦ SERVERLUME", hexLav, hexPink, true)
	sub := subtleStyle.Render(" server fleet dashboard")

	statUp := pill(fmt.Sprintf(" %d up ", up), colGreen, lipgloss.Color(hexBgPage), true)
	statWarn := pill(fmt.Sprintf(" %d warn ", warn), colAmber, lipgloss.Color(hexBgPage), true)
	statDown := pill(fmt.Sprintf(" %d down ", down), colRed, lipgloss.Color(hexBgPage), true)
	stats := lipgloss.JoinHorizontal(lipgloss.Center, statUp, " ", statWarn, " ", statDown)

	clock := subtleStyle.Render(time.Now().Format("Mon 15:04:05"))
	pauseTag := ""
	if m.paused {
		pauseTag = "  " + pill("PAUSED", colAmber, lipgloss.Color(hexBgPage), true)
	}

	left := brand + sub
	right := lipgloss.JoinHorizontal(lipgloss.Center, stats, pauseTag, "   ", clock)

	gapW := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gapW < 1 {
		gapW = 1
	}
	line := left + strings.Repeat(" ", gapW) + right
	rule := gradientText(strings.Repeat("─", m.width), hexLav, hexPink, false)
	return lipgloss.JoinVertical(lipgloss.Left, line, rule)
}

func (m model) renderFooter() string {
	keys := []struct{ k, d string }{
		{"↑/↓", "select"},
		{"tab", "switch view"},
		{"1-4", "jump tab"},
		{"p", "pause"},
		{"q", "quit"},
	}
	chip := lipgloss.NewStyle().Background(colBgCard).Padding(0, 1)
	var parts []string
	for _, kd := range keys {
		txt := lipgloss.NewStyle().Foreground(colLav).Bold(true).Render(kd.k) +
			lipgloss.NewStyle().Foreground(colDim).Render(" "+kd.d)
		parts = append(parts, chip.Render(txt))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func (m model) renderSidebar(width, height int) string {
	var rows []string
	header := lipgloss.NewStyle().Bold(true).Foreground(colPink).Render("SERVERS") +
		subtleStyle.Render(fmt.Sprintf("  %d nodes", len(m.servers)))
	rows = append(rows, header)
	rows = append(rows, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", width-2)))

	innerW := width - 4 // minus card border+padding
	for i, s := range m.servers {
		selected := i == m.cursor
		bar := "▏"
		barStyle := lipgloss.NewStyle().Foreground(statusColor(s.status))

		nameW := innerW - 9
		if nameW < 4 {
			nameW = 4
		}
		name := s.name
		if lipgloss.Width(name) > nameW {
			name = name[:nameW]
		}
		nameStyle := lipgloss.NewStyle().Foreground(colText)
		cpuStyle := lipgloss.NewStyle().Foreground(colDim)
		if selected {
			nameStyle = nameStyle.Foreground(colPink).Bold(true)
			cpuStyle = lipgloss.NewStyle().Foreground(colPinkSoft).Bold(true)
			barStyle = barStyle.Bold(true)
		}
		cpuTxt := "  --  "
		if s.status != "down" {
			cpuTxt = fmt.Sprintf("%5.0f%%", s.cpu)
		}
		line := fmt.Sprintf("%s %-*s%s", barStyle.Render(bar), nameW+1, nameStyle.Render(name), cpuStyle.Render(cpuTxt))

		rowStyle := lipgloss.NewStyle().Width(innerW)
		if selected {
			rowStyle = rowStyle.Background(colBgCardHi)
		}
		rows = append(rows, rowStyle.Render(line))
	}

	content := padOpaque(strings.Join(rows, "\n"), innerW, colBgCard)
	return cardStyleFocus.Width(width - 2).Height(height - 2).Render(content)
}

func (m model) renderMain(width, height int) string {
	if len(m.servers) == 0 {
		return cardStyle.Width(width - 2).Height(height - 2).Render("no servers")
	}
	s := m.servers[m.cursor]

	var tabsRendered []string
	for i, name := range tabNames {
		if tab(i) == m.tab {
			tabsRendered = append(tabsRendered, pill(name, colPink, lipgloss.Color(hexBgPage), true))
		} else {
			tabsRendered = append(tabsRendered, lipgloss.NewStyle().Foreground(colDim).Padding(0, 1).Render(name))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabsRendered...)
	divider := lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", width-4))

	innerHeight := height - 5
	if innerHeight < 3 {
		innerHeight = 3
	}
	innerW := width - 4

	var content string
	switch m.tab {
	case tabOverview:
		content = m.renderOverview(s, innerW, innerHeight)
	case tabMetrics:
		content = m.renderMetrics(s, innerW, innerHeight)
	case tabLogs:
		content = m.renderLogs(innerW, innerHeight)
	case tabCharts:
		content = m.renderCharts(innerW, innerHeight)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		padOpaque(tabBar, innerW, colBgCard),
		divider,
		padOpaque(content, innerW, colBgCard),
	)
	return cardStyleFocus.Width(width - 2).Height(height - 2).Render(body)
}

func kv(k, v string, labelW int) string {
	ks := lipgloss.NewStyle().Foreground(colDim).Width(labelW).Render(k)
	return ks + lipgloss.NewStyle().Foreground(colText).Render(v)
}

// ---------- Overview tab ----------

func (m model) renderOverview(s server, width, height int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colText).Render(s.name))
	b.WriteString("  " + statusPill(s.status))
	b.WriteString("\n\n")

	labelW := 13
	b.WriteString(kv("Region", s.region, labelW) + "\n")
	b.WriteString(kv("IP Address", s.ip, labelW) + "\n")
	b.WriteString(kv("Uptime", fmtDuration(s.uptime), labelW) + "\n")
	b.WriteString(kv("Processes", fmt.Sprintf("%d", s.procs), labelW) + "\n")
	b.WriteString(kv("Load Avg", fmt.Sprintf("%.2f  %.2f  %.2f", s.loadAvg[0], s.loadAvg[1], s.loadAvg[2]), labelW) + "\n\n")

	gaugeW := width - labelW - 9
	if gaugeW < 8 {
		gaugeW = 8
	}

	gauge := func(label string, pct float64) string {
		from, to := pctGradient(pct)
		return kv(label, "", labelW) + gradientBar(pct, gaugeW, from, to) + fmt.Sprintf(" %5.1f%%", pct)
	}
	b.WriteString(gauge("CPU", s.cpu) + "\n")
	b.WriteString(gauge("Memory", s.mem) + "\n")
	b.WriteString(gauge("Disk", s.disk) + "\n\n")

	b.WriteString(kv("Net In", fmt.Sprintf("%.1f Mb/s", s.netIn), labelW) + "\n")
	b.WriteString(kv("Net Out", fmt.Sprintf("%.1f Mb/s", s.netOut), labelW) + "\n")

	return b.String()
}

// ---------- Metrics tab ----------

const (
	ansiAxis  = asciigraph.AnsiColor(60)  // dim slate, matches hexBorder
	ansiLabel = asciigraph.AnsiColor(103) // muted lavender-gray, matches hexDim
	ansiCPU   = asciigraph.AnsiColor(183) // pale lavender-pink
	ansiMem   = asciigraph.AnsiColor(205) // hot pink
)

// lineChart renders one solid-color series. A single color per line (rather than
// per-point gradient coloring) keeps the emitted ANSI simple, which matters here:
// this redraws every tick, and heavier per-character color codes are more prone
// to terminal redraw glitches on fast-changing content.
func lineChart(hist []float64, width, height int, lo, hi float64, col asciigraph.AnsiColor) string {
	return asciigraph.Plot(hist,
		asciigraph.Width(width),
		asciigraph.Height(height),
		asciigraph.LowerBound(lo),
		asciigraph.UpperBound(hi),
		asciigraph.SeriesColors(col),
		asciigraph.AxisColor(ansiAxis),
		asciigraph.LabelColor(ansiLabel),
		asciigraph.Precision(0),
	)
}

func (m model) renderMetrics(s server, width, height int) string {
	var b strings.Builder
	chartW := width - 10
	if chartW < 12 {
		chartW = 12
	}
	chartH := 5
	if height < 18 {
		chartH = 3
	}

	section := func(label string, val float64, hist []float64, lo, hi float64, unit string, col asciigraph.AnsiColor) {
		b.WriteString(lipgloss.NewStyle().Foreground(colLav).Bold(true).Render(label))
		b.WriteString(subtleStyle.Render(fmt.Sprintf("   now %.1f%s", val, unit)))
		b.WriteString("\n")
		b.WriteString(lineChart(hist, chartW, chartH, lo, hi, col))
		b.WriteString("\n\n")
	}

	section("CPU history", s.cpu, s.cpuHist, 0, 100, "%", ansiCPU)
	section("Memory history", s.mem, s.memHist, 0, 100, "%", ansiMem)

	return b.String()
}

// ---------- Logs tab ----------

func (m model) renderLogs(width, height int) string {
	var lines []string
	start := 0
	if len(m.logs) > height-1 {
		start = len(m.logs) - (height - 1)
	}
	for _, l := range m.logs[start:] {
		ts := subtleStyle.Render(l.t.Format("15:04:05"))
		tag := lipgloss.NewStyle().Foreground(l.col).Bold(true).Width(6).Render(l.tag)
		lines = append(lines, fmt.Sprintf("%s %s %s", ts, tag, l.msg))
	}
	if len(lines) == 0 {
		return subtleStyle.Render("waiting for log events...")
	}
	return strings.Join(lines, "\n")
}
