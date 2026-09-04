package main

import (
	"strings"
	"testing"
)

// TestViewNeverExceedsTerminalHeight guards against a real, three-layered
// bug: the page's line count was structurally 1 taller than the terminal
// (a wrong blank-line count in View's own math), individual tabs' fixed
// content didn't shrink with height, and a too-wide line could get
// word-wrapped by lipgloss's own Width()/Height() rendering into extra
// lines *after* any of our own budgeting ran. On an actual terminal this
// doesn't clip — it makes the terminal itself scroll, which (since the app
// draws top-to-bottom) hides the top of the UI, not the bottom.
func TestViewNeverExceedsTerminalHeight(t *testing.T) {
	sizes := []struct{ w, h int }{
		{40, 5}, {50, 8}, {60, 12}, {80, 15}, {80, 20}, {100, 24},
		{120, 15}, {120, 30}, {120, 60}, {200, 100},
	}
	for _, sz := range sizes {
		m := model{
			servers: demoFleet(),
			host:    server{name: "test-host"},
			width:   sz.w,
			height:  sz.h,
			tab:     tabOverview,
		}
		out := m.View()
		if got := len(strings.Split(out, "\n")); got != sz.h {
			t.Errorf("w=%d h=%d: got %d rendered lines, want exactly %d", sz.w, sz.h, got, sz.h)
		}
	}
}

// TestViewFooterSurvivesAtRealisticSize is the specific regression this was
// first noticed as: the version string silently disappearing from the
// footer because the last line of an over-tall frame got clipped away.
func TestViewFooterSurvivesAtRealisticSize(t *testing.T) {
	m := model{servers: demoFleet(), host: server{name: "test-host"}, width: 120, height: 30, tab: tabOverview}
	if out := m.View(); !strings.Contains(out, "serverlume "+appVersion) {
		t.Error("footer version string missing at a realistic terminal size")
	}
}

func TestScrollWindowFitsEverything(t *testing.T) {
	start, end, above, below := scrollWindow(4, 1, 10)
	if start != 0 || end != 4 || above != 0 || below != 0 {
		t.Errorf("got (%d,%d,%d,%d), want (0,4,0,0)", start, end, above, below)
	}
}

func TestScrollWindowKeepsCursorInView(t *testing.T) {
	// 10 items, only 3 rows available: the cursor must always land inside
	// [start, end), and above+below+len(window) must always equal total.
	const total, avail = 10, 3
	for cursor := 0; cursor < total; cursor++ {
		start, end, above, below := scrollWindow(total, cursor, avail)
		if cursor < start || cursor >= end {
			t.Errorf("cursor=%d not in window [%d,%d)", cursor, start, end)
		}
		if end-start > avail {
			t.Errorf("cursor=%d: window size %d exceeds avail %d", cursor, end-start, avail)
		}
		if above+(end-start)+below != total {
			t.Errorf("cursor=%d: above(%d)+window(%d)+below(%d) != total(%d)", cursor, above, end-start, below, total)
		}
	}
}

func TestScrollWindowIndicatorsMatchEdges(t *testing.T) {
	start, end, above, below := scrollWindow(10, 0, 4)
	if above != 0 {
		t.Errorf("cursor at first item: above = %d, want 0", above)
	}
	if start != 0 {
		t.Errorf("cursor at first item: start = %d, want 0", start)
	}
	_ = end
	_ = below

	start, end, above, below = scrollWindow(10, 9, 4)
	if below != 0 {
		t.Errorf("cursor at last item: below = %d, want 0", below)
	}
	if end != 10 {
		t.Errorf("cursor at last item: end = %d, want 10", end)
	}
	_ = start
	_ = above
}

func TestScrollWindowAvailFloor(t *testing.T) {
	// avail <= 0 should still return a usable single-row window, not panic
	// or return an empty one.
	start, end, _, _ := scrollWindow(5, 2, 0)
	if end-start < 1 {
		t.Errorf("expected at least 1 visible row, got window [%d,%d)", start, end)
	}
}
