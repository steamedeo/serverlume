package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------- data ----------

type server struct {
	name    string
	region  string
	ip      string
	status  string
	cpu     float64
	mem     float64
	disk    float64
	netIn   float64
	netOut  float64
	uptime  time.Duration
	cpuHist []float64
	memHist []float64
	netHist []float64
	loadAvg [3]float64
	procs   int
}

var logLevels = []struct {
	tag string
	col lipgloss.Color
}{
	{"INFO", colLav},
	{"WARN", colAmber},
	{"ERROR", colRed},
	{"OK", colGreen},
}

var logMsgs = []string{
	"health check passed",
	"connection accepted from 10.0.4.%d",
	"gc cycle completed in %dms",
	"disk usage at %d%%",
	"deploy hook triggered",
	"cache invalidated for shard-%d",
	"systemd unit restarted",
	"tls certificate renewed",
	"request latency spike detected",
	"memory pressure easing",
	"cron job finished successfully",
	"replication lag %dms",
}

type logLine struct {
	t   time.Time
	tag string
	col lipgloss.Color
	msg string
}

// newServers seeds a demo fleet with plausible, slightly-randomized starting
// metrics. Swap this out for a real inventory/exporter source to point
// serverlume at actual infrastructure.
func newServers() []server {
	names := []struct{ name, region, ip, status string }{
		{"web-01", "us-east-1", "10.0.1.11", "up"},
		{"web-02", "us-east-1", "10.0.1.12", "up"},
		{"api-01", "us-east-1", "10.0.2.10", "warn"},
		{"api-02", "us-west-2", "10.0.2.11", "up"},
		{"db-primary", "us-east-1", "10.0.3.5", "up"},
		{"db-replica", "us-west-2", "10.0.3.6", "up"},
		{"cache-01", "us-east-1", "10.0.4.2", "up"},
		{"worker-01", "eu-west-1", "10.0.5.20", "down"},
		{"worker-02", "eu-west-1", "10.0.5.21", "up"},
		{"lb-edge", "global", "10.0.0.1", "up"},
	}
	srv := make([]server, 0, len(names))
	for _, n := range names {
		s := server{
			name: n.name, region: n.region, ip: n.ip, status: n.status,
			cpu:     rand.Float64()*60 + 10,
			mem:     rand.Float64()*50 + 20,
			disk:    rand.Float64()*40 + 30,
			netIn:   rand.Float64() * 80,
			netOut:  rand.Float64() * 40,
			uptime:  time.Duration(rand.Intn(90)) * 24 * time.Hour,
			loadAvg: [3]float64{rand.Float64() * 2, rand.Float64() * 2, rand.Float64() * 2},
			procs:   rand.Intn(200) + 40,
		}
		if s.status == "down" {
			s.cpu, s.mem = 0, 0
		}
		for i := 0; i < 40; i++ {
			s.cpuHist = append(s.cpuHist, s.cpu)
			s.memHist = append(s.memHist, s.mem)
			s.netHist = append(s.netHist, s.netIn)
		}
		srv = append(srv, s)
	}
	return srv
}

func walk(v float64, spread float64, lo, hi float64) float64 {
	v += (rand.Float64()*2 - 1) * spread
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// ---------- model ----------

type tab int

const (
	tabOverview tab = iota
	tabMetrics
	tabLogs
	tabCharts
)

var tabNames = []string{"Overview", "Metrics", "Logs", "Charts"}

type tickMsg time.Time

type model struct {
	servers  []server
	cursor   int
	tab      tab
	logs     []logLine
	width    int
	height   int
	paused   bool
	quitting bool
}

func initialModel() model {
	return model{
		servers: newServers(),
		tab:     tabOverview,
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.servers)-1 {
				m.cursor++
			}
		case "tab":
			m.tab = (m.tab + 1) % tab(len(tabNames))
		case "shift+tab":
			m.tab = (m.tab - 1 + tab(len(tabNames))) % tab(len(tabNames))
		case "1":
			m.tab = tabOverview
		case "2":
			m.tab = tabMetrics
		case "3":
			m.tab = tabLogs
		case "4":
			m.tab = tabCharts
		case "p":
			m.paused = !m.paused
		}
		return m, nil

	case tickMsg:
		if !m.paused {
			m.updateMetrics()
			m.maybeLog()
		}
		return m, tickCmd()
	}
	return m, nil
}

func (m *model) updateMetrics() {
	for i := range m.servers {
		s := &m.servers[i]
		if s.status == "down" {
			continue
		}
		s.cpu = walk(s.cpu, 6, 2, 98)
		s.mem = walk(s.mem, 3, 5, 95)
		s.netIn = walk(s.netIn, 8, 0, 400)
		s.netOut = walk(s.netOut, 5, 0, 200)
		s.cpuHist = append(s.cpuHist[1:], s.cpu)
		s.memHist = append(s.memHist[1:], s.mem)
		s.netHist = append(s.netHist[1:], s.netIn)
		s.uptime += 600 * time.Millisecond
	}
}

func (m *model) maybeLog() {
	if rand.Float64() > 0.55 {
		return
	}
	srv := m.servers[rand.Intn(len(m.servers))]
	lvl := logLevels[rand.Intn(len(logLevels))]
	msgTmpl := logMsgs[rand.Intn(len(logMsgs))]
	msg := msgTmpl
	if strings.Contains(msgTmpl, "%d") {
		msg = fmt.Sprintf(msgTmpl, rand.Intn(999))
	}
	line := logLine{
		t:   time.Now(),
		tag: lvl.tag,
		col: lvl.col,
		msg: fmt.Sprintf("[%s] %s", srv.name, msg),
	}
	m.logs = append(m.logs, line)
	if len(m.logs) > 200 {
		m.logs = m.logs[len(m.logs)-200:]
	}
}

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}
