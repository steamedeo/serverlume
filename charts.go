package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------- Charts tab: fleet-wide visualizations ----------

func hbarChart(labels []string, values []float64, hi float64, width int) string {
	type kv struct {
		name string
		val  float64
	}
	items := make([]kv, len(labels))
	for i := range labels {
		items[i] = kv{labels[i], values[i]}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].val > items[j].val })

	nameW := 11
	barW := width - nameW - 8
	if barW < 6 {
		barW = 6
	}
	var lines []string
	for _, it := range items {
		name := it.name
		if lipgloss.Width(name) > nameW {
			name = name[:nameW]
		}
		from, to := pctGradient(it.val)
		bar := gradientBar(it.val, barW, from, to)
		nameCol := lipgloss.NewStyle().Foreground(colText).Render(fmt.Sprintf("%-*s", nameW, name))
		valCol := lipgloss.NewStyle().Foreground(colDim).Render(fmt.Sprintf("%5.1f%%", it.val))
		lines = append(lines, nameCol+" "+bar+" "+valCol)
	}
	return strings.Join(lines, "\n")
}

// dotDonut renders a size x size dot grid proportioned by counts (in order given).
func dotDonut(order []string, counts map[string]int, colors map[string]lipgloss.Color, size int) []string {
	total := 0
	for _, k := range order {
		total += counts[k]
	}
	cells := size * size
	var dotColors []lipgloss.Color
	if total > 0 {
		remaining := cells
		for i, k := range order {
			n := int(float64(counts[k]) / float64(total) * float64(cells))
			if n == 0 && counts[k] > 0 {
				n = 1
			}
			if i == len(order)-1 || n > remaining {
				n = remaining
			}
			for j := 0; j < n; j++ {
				dotColors = append(dotColors, colors[k])
			}
			remaining -= n
		}
	}
	for len(dotColors) < cells {
		dotColors = append(dotColors, lipgloss.Color(hexBorder))
	}

	var lines []string
	idx := 0
	for r := 0; r < size; r++ {
		var sb strings.Builder
		for c := 0; c < size; c++ {
			sb.WriteString(lipgloss.NewStyle().Foreground(dotColors[idx]).Render("●"))
			idx++
			if c < size-1 {
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}
	return lines
}

// heatmapGrid renders a GitHub-contribution-style tile grid: one row per server,
// one column per recent CPU sample, shaded from idle (lavender) to hot (pink).
func heatmapGrid(servers []server, cols int) string {
	c1, c2 := mustColorful(hexLavDim), mustColorful(hexPink)
	nameW := 11
	var lines []string
	for _, s := range servers {
		name := s.name
		if lipgloss.Width(name) > nameW {
			name = name[:nameW]
		}
		nameCol := lipgloss.NewStyle().Foreground(colText).Width(nameW + 1).Render(name)

		var row strings.Builder
		if s.status == "down" {
			row.WriteString(lipgloss.NewStyle().Foreground(colDim).Render(strings.Repeat("░░", cols)))
		} else {
			hist := s.cpuHist
			start := 0
			if len(hist) > cols {
				start = len(hist) - cols
			}
			for _, v := range hist[start:] {
				t := v / 100
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				c := c1.BlendLuv(c2, t)
				row.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(c.Hex())).Render("  "))
			}
		}
		lines = append(lines, nameCol+row.String())
	}
	return strings.Join(lines, "\n")
}

func (m model) renderCharts(width, height int) string {
	names := make([]string, len(m.servers))
	cpus := make([]float64, len(m.servers))
	for i, s := range m.servers {
		names[i] = s.name
		cpus[i] = s.cpu
	}

	leftW := width * 3 / 5
	rightW := width - leftW - 2
	if rightW < 14 {
		rightW = 14
		leftW = width - rightW - 2
	}

	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		sectionTitle("CPU Ranking")+subtleStyle.Render("  (bar chart)"),
		"",
		hbarChart(names, cpus, 100, leftW),
	)

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
	grid := dotDonut(
		[]string{"up", "warn", "down"},
		map[string]int{"up": up, "warn": warn, "down": down},
		map[string]lipgloss.Color{"up": colGreen, "warn": colAmber, "down": colRed},
		6,
	)
	legend := []string{
		fmt.Sprintf("%s up   %d", lipgloss.NewStyle().Foreground(colGreen).Render("●"), up),
		fmt.Sprintf("%s warn %d", lipgloss.NewStyle().Foreground(colAmber).Render("●"), warn),
		fmt.Sprintf("%s down %d", lipgloss.NewStyle().Foreground(colRed).Render("●"), down),
	}
	rightLines := []string{sectionTitle("Fleet Status") + subtleStyle.Render("  (dot chart)"), ""}
	rightLines = append(rightLines, grid...)
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, legend...)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, rightLines...)

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).Render(leftCol),
		"  ",
		lipgloss.NewStyle().Width(rightW).Render(rightCol),
	)

	heat := lipgloss.JoinVertical(lipgloss.Left,
		sectionTitle("Load Heatmap")+subtleStyle.Render("  (recent CPU, all nodes)"),
		"",
		heatmapGrid(m.servers, 24),
	)

	return lipgloss.JoinVertical(lipgloss.Left, top, "", heat)
}
