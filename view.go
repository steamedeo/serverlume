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

	if m.booting {
		return page.Width(m.width).Height(m.height).Render(m.renderSplash())
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// full below joins header + "" + body + "" + footer — that's two blank
	// separator lines on top of header/footer's own height, not one.
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 2
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
	// clipLines runs on the final rendered string, after page's own
	// Width/Height — that outer Render can itself wrap a too-wide line
	// (e.g. on a very narrow terminal) into extra output lines, so this has
	// to be the true last step to actually guarantee the line-count cap.
	return clipLines(page.Width(m.width).Height(m.height).Render(full), m.height)
}

// clipLines is a last-resort safety net: lipgloss's Height() only pads
// content that's shorter than requested, it never truncates content that's
// taller. If some section's budgeting is off and the page ends up with more
// lines than the terminal is tall, printing all of it makes the terminal
// itself scroll — which, since the app draws top-to-bottom, shows up as the
// *top* of the UI (the header, the first sidebar rows) scrolling out of
// view instead of a clean bottom cutoff. Hard-capping the line count here
// guarantees that can't happen, regardless of what any individual renderer
// gets wrong.
func clipLines(s string, height int) string {
	if height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}

// renderSplash draws the animated boot screen shown for the first
// bootFrames ticks (or until any key is pressed).
func (m model) renderSplash() string {
	wordmark := gradientText("S E R V E R L U M E", hexLav, hexPink, true)
	tagline := subtleStyle.Render("terminal fleet dashboard")

	pct := float64(m.bootFrame) / float64(bootFrames-1) * 100
	if pct > 100 {
		pct = 100
	}
	bar := gradientBar(pct, 30, hexLav, hexPink)

	spin := spinnerFrames[m.bootFrame%len(spinnerFrames)]
	status := subtleStyle.Render(spin + " booting fleet telemetry...")
	if pct >= 100 {
		status = lipgloss.NewStyle().Foreground(colGreen).Render("✓ ready — press any key")
	}

	version := lipgloss.NewStyle().Foreground(colBorder).Render("serverlume " + appVersion)

	block := lipgloss.JoinVertical(lipgloss.Center,
		wordmark,
		"",
		tagline,
		"",
		bar,
		"",
		status,
		"",
		version,
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block,
		lipgloss.WithWhitespaceBackground(lipgloss.Color(hexBgPage)))
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
	live := ""
	if m.paused {
		pauseTag = "  " + pill("PAUSED", colAmber, lipgloss.Color(hexBgPage), true)
	} else {
		dotStyle := lipgloss.NewStyle().Foreground(colGreen)
		if !m.pulseOn {
			dotStyle = lipgloss.NewStyle().Foreground(colDim)
		}
		live = "  " + dotStyle.Render("●") + subtleStyle.Render(" live")
	}

	left := brand + sub
	right := lipgloss.JoinHorizontal(lipgloss.Center, stats, pauseTag, live, "   ", clock)

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
		{"1-3", "jump tab"},
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
	left := lipgloss.JoinHorizontal(lipgloss.Center, parts...)
	right := lipgloss.NewStyle().Foreground(colBorder).Render("serverlume " + appVersion)

	gapW := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gapW < 1 {
		gapW = 1
	}
	return left + strings.Repeat(" ", gapW) + right
}

func (m model) renderSidebar(width, height int) string {
	innerW := width - 4 // minus card border+padding

	var rows []string
	header := lipgloss.NewStyle().Bold(true).Foreground(colPink).Render("SERVERS") +
		subtleStyle.Render(fmt.Sprintf("  %d nodes", len(m.servers)))
	rows = append(rows, header)
	rows = append(rows, subtleStyle.Render("· "+m.fleetSource))
	rows = append(rows, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", innerW)))

	renderRow := func(s server, selected, isHostRow bool) string {
		bar := "▏"
		barStyle := lipgloss.NewStyle().Foreground(statusColor(s.status))
		if isHostRow {
			bar = "⌂"
			barStyle = lipgloss.NewStyle().Foreground(colLav)
		}

		nameW := innerW - 9
		if nameW < 4 {
			nameW = 4
		}
		name := s.name
		if lipgloss.Width(name) > nameW {
			name = name[:nameW]
		}
		nameStyle := lipgloss.NewStyle().Foreground(colText)
		if isHostRow {
			nameStyle = nameStyle.Foreground(colLav)
		}
		memStyle := lipgloss.NewStyle().Foreground(colDim)
		if selected {
			accent := colPink
			if isHostRow {
				accent = colLav
			}
			nameStyle = nameStyle.Foreground(accent).Bold(true)
			memStyle = lipgloss.NewStyle().Foreground(colPinkSoft).Bold(true)
			if isHostRow {
				memStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
			}
			barStyle = barStyle.Bold(true)
		}
		memTxt := "  --  "
		if s.status != "down" {
			memTxt = fmt.Sprintf("%5.0f%%", s.mem)
		}
		line := fmt.Sprintf("%s %-*s%s", barStyle.Render(bar), nameW+1, nameStyle.Render(name), memStyle.Render(memTxt))

		rowStyle := lipgloss.NewStyle().Width(innerW)
		if selected {
			bg := colBgCardHi
			rowStyle = rowStyle.Background(bg)
		}
		return rowStyle.Render(line)
	}

	// Everything else in the card is fixed overhead (header x3, the "this
	// host" divider/caption/row x4) — whatever's left is what the fleet
	// list actually has room for. On a short terminal that's fewer than
	// len(m.servers), so the list needs to scroll rather than just render
	// past the card's bottom (which lipgloss won't clip on its own, and —
	// worse — can push the whole page taller than the terminal, scrolling
	// real terminal content and hiding rows above instead of below).
	const fixedOverheadRows = 7
	avail := (height - 2) - fixedOverheadRows
	start, end, above, below := scrollWindow(len(m.servers), m.cursor, avail)

	if above > 0 {
		rows = append(rows, subtleStyle.Render(fmt.Sprintf("  ↑ %d more", above)))
	}
	for i := start; i < end; i++ {
		rows = append(rows, renderRow(m.servers[i], i == m.cursor, false))
	}
	if below > 0 {
		rows = append(rows, subtleStyle.Render(fmt.Sprintf("  ↓ %d more", below)))
	}

	// The host entry is visually detached from the fleet: a dashed divider
	// and a caption make clear it's this machine, not another managed node.
	rows = append(rows, "")
	rows = append(rows, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("╌", innerW)))
	rows = append(rows, lipgloss.NewStyle().Foreground(colLav).Bold(true).Render("⌂ THIS HOST"))
	rows = append(rows, renderRow(m.host, m.cursor == len(m.servers), true))

	// Same safety net as renderMain: the fixed-overhead accounting above is
	// meant to keep this exactly at height-2, but never let a rounding case
	// push it taller than the card actually has room for.
	content := clipLines(strings.Join(rows, "\n"), height-2)
	content = padOpaque(content, innerW, colBgCard)
	return cardStyleFocus.Width(width - 2).Height(height - 2).Render(content)
}

// scrollWindow picks a [start, end) slice of a total-item list that both
// fits within avail rows and keeps cursor inside it, growing outward from
// the cursor. above/below report how many items are scrolled past on each
// side, for a "N more" indicator — each indicator line itself eats into the
// budget, hence the two-pass sizing.
func scrollWindow(total, cursor, avail int) (start, end, above, below int) {
	if avail < 1 {
		avail = 1
	}
	if total <= avail {
		return 0, total, 0, 0
	}
	if cursor < 0 || cursor >= total {
		cursor = 0
	}

	fit := func(winSize int) (int, int) {
		if winSize < 1 {
			winSize = 1
		}
		s := cursor - (winSize-1)/2
		if s < 0 {
			s = 0
		}
		e := s + winSize
		if e > total {
			e = total
			s = e - winSize
			if s < 0 {
				s = 0
			}
		}
		return s, e
	}

	start, end = fit(avail)
	reserve := 0
	if start > 0 {
		reserve++
	}
	if end < total {
		reserve++
	}
	if reserve > 0 {
		start, end = fit(avail - reserve)
	}
	if start > 0 {
		above = start
	}
	if end < total {
		below = total - end
	}
	return start, end, above, below
}

func (m model) renderMain(width, height int) string {
	if len(m.servers) == 0 {
		return cardStyle.Width(width - 2).Height(height - 2).Render("no servers")
	}
	s := m.selected()

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
	case tabCommand:
		content = m.renderCommand(s, innerW, innerHeight)
	case tabLogs:
		content = m.renderLogs(innerW, innerHeight)
	}

	// Safety net: a tab's own content budgeting can still be wrong (fixed
	// rows a tab always emits, e.g. Overview's name/region/gauges block,
	// don't shrink with height) — clip rather than let it push the card,
	// and with it the whole page, taller than requested.
	content = clipLines(content, innerHeight)

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
	if s.isHost {
		b.WriteString("  " + pill("⌂ LOCAL HOST", colLav, lipgloss.Color(hexBgPage), true))
	}
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
	b.WriteString(kv("Net Out", fmt.Sprintf("%.1f Mb/s", s.netOut), labelW) + "\n\n")

	chartW := width - 10
	if chartW < 12 {
		chartW = 12
	}

	// overviewFixedLines is everything above the charts: name+badges (2,
	// counting the blank line "\n\n" leaves), 4 kv rows, load avg+blank (2),
	// 3 gauges+blank (4), 2 net rows+blank (3) — see the writes above.
	const overviewFixedLines = 2 + 4 + 2 + 4 + 3
	chartsAvail := height - overviewFixedLines
	if chartsAvail < 0 {
		chartsAvail = 0
	}

	// Each chart section costs (chartH + 2) lines: a label line, the chart
	// itself, and a trailing blank. Rather than just shrinking chartH until
	// it's an unreadable sliver (a fixed 2-line-per-chart cost means that
	// alone was never enough to actually fit on a short terminal), find the
	// largest chart height — and, failing that, the fewest charts — that
	// actually fits the space that's really there. Below a hard floor of 2
	// rows a chart isn't legible anyway, so charts get dropped instead.
	numCharts, chartH := 3, 4
	for numCharts > 0 && numCharts*(chartH+2) > chartsAvail {
		if chartH > 2 {
			chartH--
		} else {
			numCharts--
		}
	}

	section := func(label string, val float64, hist []float64, unit string, col asciigraph.AnsiColor) {
		lo, hi := autoRange(hist)
		b.WriteString(lipgloss.NewStyle().Foreground(colLav).Bold(true).Render(label))
		b.WriteString(subtleStyle.Render(fmt.Sprintf("   now %.1f%s", val, unit)))
		b.WriteString("\n")
		b.WriteString(lineChart(hist, chartW, chartH, lo, hi, col))
		b.WriteString("\n\n")
	}

	charts := []struct {
		label string
		val   float64
		hist  []float64
		col   asciigraph.AnsiColor
	}{
		{"CPU history", s.cpu, s.cpuHist, ansiCPU},
		{"Memory history", s.mem, s.memHist, ansiMem},
		{"Disk history", s.disk, s.diskHist, ansiDisk},
	}
	for _, c := range charts[:numCharts] {
		section(c.label, c.val, c.hist, "%", c.col)
	}

	return b.String()
}

// ---------- history line charts, used by the Overview tab ----------

const (
	ansiAxis  = asciigraph.AnsiColor(60)  // dim slate, matches hexBorder
	ansiLabel = asciigraph.AnsiColor(103) // muted lavender-gray, matches hexDim
	ansiCPU   = asciigraph.AnsiColor(183) // pale lavender-pink
	ansiMem   = asciigraph.AnsiColor(205) // hot pink
	ansiDisk  = asciigraph.AnsiColor(215) // warm amber, matches the disk gauge's hot-usage color
)

// autoRange picks y-axis bounds from a history's actual min/max, padded a
// bit, instead of a fixed 0-100. A percentage that only ever wobbles in a
// narrow band (disk usage drifting a couple points, say) is otherwise
// invisible squashed against a 0-100 scale — real dashboards auto-scale the
// axis to the data for exactly this reason. Bounds are clamped back to
// [0, 100] since these are all percentages, and widened to a minimum span
// so a genuinely flat series still renders as a flat line, not a divide
// with itself.
func autoRange(hist []float64) (lo, hi float64) {
	if len(hist) == 0 {
		return 0, 100
	}
	lo, hi = hist[0], hist[0]
	for _, v := range hist[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	const minSpan = 8.0
	if hi-lo < minSpan {
		mid := (lo + hi) / 2
		lo, hi = mid-minSpan/2, mid+minSpan/2
	} else {
		pad := (hi - lo) * 0.15
		lo -= pad
		hi += pad
	}
	if lo < 0 {
		lo = 0
	}
	if hi > 100 {
		hi = 100
	}
	return lo, hi
}

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

// ---------- Logs tab ----------

// ---------- Command tab ----------

// renderCommand shows a per-host scrollback of commands run over SSH (most
// recent at the bottom) with an input prompt for the currently selected
// host below it. Each host keeps its own input/history (m.cmdInputs /
// m.cmdHistory, keyed by alias), so switching servers doesn't lose either.
func (m model) renderCommand(s server, width, height int) string {
	if !s.isSSH {
		return subtleStyle.Render("Command execution is only available for SSH-connected hosts.")
	}

	const reservedForPrompt = 2 // blank separator + the prompt line itself
	histBudget := height - reservedForPrompt
	if histBudget < 0 {
		histBudget = 0
	}

	var histLines []string
	for _, e := range m.cmdHistory[s.name] {
		histLines = append(histLines, lipgloss.NewStyle().Foreground(colLav).Bold(true).Render("$ "+e.cmd))

		out := strings.TrimRight(e.stdout, "\n")
		if out != "" {
			for _, l := range strings.Split(out, "\n") {
				histLines = append(histLines, lipgloss.NewStyle().Foreground(colText).Render(l))
			}
		}

		errText := strings.TrimRight(e.stderr, "\n")
		if errText == "" && e.err != nil {
			errText = e.err.Error()
		}
		if errText != "" {
			errStyle := lipgloss.NewStyle().Foreground(colRed)
			for _, l := range strings.Split(errText, "\n") {
				histLines = append(histLines, errStyle.Render(l))
			}
		}

		if out == "" && errText == "" {
			histLines = append(histLines, subtleStyle.Render("(no output)"))
		}
		histLines = append(histLines, "")
	}
	if len(histLines) == 0 {
		histLines = append(histLines, subtleStyle.Render("Type a command and press enter to run it on "+s.name+"."))
	}
	if len(histLines) > histBudget {
		histLines = histLines[len(histLines)-histBudget:]
	}

	prompt := lipgloss.NewStyle().Foreground(colPink).Bold(true).Render("$ ") + m.cmdInputs[s.name] + "▏"
	if m.cmdRunning[s.name] {
		prompt = lipgloss.NewStyle().Foreground(colAmber).Render("running…")
	}

	return strings.Join(append(histLines, "", prompt), "\n")
}

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
