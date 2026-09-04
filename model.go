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
	name     string
	region   string
	ip       string
	status   string
	cpu      float64
	mem      float64
	disk     float64
	netIn    float64
	netOut   float64
	uptime   time.Duration
	cpuHist  []float64
	memHist  []float64
	diskHist []float64
	netHist  []float64
	loadAvg  [3]float64
	procs    int
	isHost   bool // true for the single "this machine" sidebar entry, never part of the demo fleet
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

// logTemplate is one kind of simulated log line. levels restricts which
// severities the message can plausibly appear under (so a routine "gc cycle
// completed" never shows up tagged ERROR), and argLo/argHi bounds the number
// substituted into a "%d"-style template to a range that makes sense for
// what it represents (a disk percentage tops out at 100, not 999).
type logTemplate struct {
	tmpl         string
	levels       []string
	argLo, argHi int
}

var logTemplates = []logTemplate{
	{"health check passed", []string{"OK"}, 0, 0},
	{"connection accepted from 10.0.4.%d", []string{"INFO"}, 2, 254},
	{"gc cycle completed in %dms", []string{"OK", "INFO"}, 20, 600},
	{"disk usage at %d%%", []string{"WARN"}, 70, 96},
	{"deploy hook triggered", []string{"INFO"}, 0, 0},
	{"cache invalidated for shard-%d", []string{"INFO"}, 0, 15},
	{"systemd unit restarted", []string{"WARN", "INFO"}, 0, 0},
	{"tls certificate renewed", []string{"OK"}, 0, 0},
	{"request latency spike detected", []string{"WARN", "ERROR"}, 0, 0},
	{"memory pressure easing", []string{"OK"}, 0, 0},
	{"cron job finished successfully", []string{"OK"}, 0, 0},
	{"replication lag %dms", []string{"WARN"}, 50, 900},
	{"connection refused, retrying", []string{"ERROR"}, 0, 0},
	{"out of memory: worker process killed", []string{"ERROR"}, 0, 0},
}

type logLine struct {
	t   time.Time
	tag string
	col lipgloss.Color
	msg string
}

func logLevelColor(tag string) lipgloss.Color {
	for _, l := range logLevels {
		if l.tag == tag {
			return l.col
		}
	}
	return colDim
}

// newServers builds the fleet list from the local SSH client config
// (~/.ssh/config), falling back to a small hardcoded demo fleet when no
// config file exists or it defines no concrete hosts (e.g. a fresh machine
// with nothing set up yet). Either way, per-server metrics are currently
// simulated placeholders — see seedPlaceholderMetrics — pending a follow-up
// that polls each host live over SSH.
func newServers() ([]server, string) {
	if srv := discoverSSHFleet(); len(srv) > 0 {
		return srv, "~/.ssh/config"
	}
	return demoFleet(), "demo data"
}

// demoFleet is the fallback fleet shown when no SSH hosts are configured, so
// the dashboard still has something to display out of the box.
func demoFleet() []server {
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
		s := server{name: n.name, region: n.region, ip: n.ip, status: n.status}
		seedPlaceholderMetrics(&s)
		srv = append(srv, s)
	}
	return srv
}

// seedSSHServer turns a discovered SSH config alias into a server entry.
// Its identity (name/IP/region) is real; its metrics are simulated
// placeholders until live polling lands.
func seedSSHServer(h sshHost) server {
	region := "ssh"
	if h.User != "" {
		region = h.User + "@" + h.HostName
	} else {
		region = h.HostName
	}
	if h.Port != "" && h.Port != "22" {
		region += ":" + h.Port
	}
	s := server{name: h.Alias, region: region, ip: h.HostName, status: "up"}
	seedPlaceholderMetrics(&s)
	return s
}

// seedPlaceholderMetrics fills in plausible, slightly-randomized starting
// metrics and a flat history so the gauges and charts have something to draw
// before the first tick. Down hosts are zeroed out. Swap this out (and
// updateMetrics's per-tick walk) for a real inventory/exporter source to
// point serverlume at live infrastructure.
func seedPlaceholderMetrics(s *server) {
	s.cpu = rand.Float64()*60 + 10
	s.mem = rand.Float64()*50 + 20
	s.disk = rand.Float64()*40 + 30
	s.netIn = rand.Float64() * 80
	s.netOut = rand.Float64() * 40
	s.uptime = time.Duration(rand.Intn(90)) * 24 * time.Hour
	s.loadAvg = [3]float64{rand.Float64() * 2, rand.Float64() * 2, rand.Float64() * 2}
	s.procs = rand.Intn(200) + 40
	if s.status == "down" {
		s.cpu, s.mem = 0, 0
	}
	for i := 0; i < 40; i++ {
		s.cpuHist = append(s.cpuHist, s.cpu)
		s.memHist = append(s.memHist, s.mem)
		s.diskHist = append(s.diskHist, s.disk)
		s.netHist = append(s.netHist, s.netIn)
	}
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
	tabLogs
)

var tabNames = []string{"Overview", "Logs"}

type tickMsg time.Time
type bootTickMsg time.Time

// bootFrames controls how long the startup splash animates before the
// dashboard takes over.
const bootFrames = 14

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type model struct {
	servers     []server
	fleetSource string // where servers came from, shown in the sidebar (e.g. "~/.ssh/config", "demo data")
	host        server // this machine — kept separate from servers, never mixed into fleet-wide views
	cursor      int
	tab         tab
	logs        []logLine
	width       int
	height      int
	paused      bool
	quitting    bool
	booting     bool
	bootFrame   int
	pulseOn     bool
}

func initialModel() model {
	servers, source := newServers()
	return model{
		servers:     servers,
		fleetSource: source,
		host:        newHostEntry(),
		tab:         tabOverview,
		booting:     true,
	}
}

// selected returns the server or host entry the cursor currently points at.
// The cursor range is 0..len(servers) inclusive: the extra slot past the
// fleet list is the host entry.
func (m model) selected() server {
	if m.cursor >= len(m.servers) {
		return m.host
	}
	return m.servers[m.cursor]
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), bootTickCmd(), tea.SetWindowTitle("serverlume"))
}

func tickCmd() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func bootTickCmd() tea.Cmd {
	return tea.Tick(70*time.Millisecond, func(t time.Time) tea.Msg {
		return bootTickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.booting {
			m.booting = false
			return m, nil
		}
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.servers) {
				m.cursor++
			}
		case "tab":
			m.tab = (m.tab + 1) % tab(len(tabNames))
		case "shift+tab":
			m.tab = (m.tab - 1 + tab(len(tabNames))) % tab(len(tabNames))
		case "1":
			m.tab = tabOverview
		case "2":
			m.tab = tabLogs
		case "p":
			m.paused = !m.paused
		}
		return m, nil

	case tickMsg:
		m.pulseOn = !m.pulseOn
		if !m.paused {
			m.updateMetrics()
			m.maybeLog()
		}
		return m, tickCmd()

	case bootTickMsg:
		if !m.booting {
			return m, nil
		}
		m.bootFrame++
		if m.bootFrame >= bootFrames {
			m.booting = false
			return m, nil
		}
		return m, bootTickCmd()
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
		s.disk = walk(s.disk, 1, 5, 97) // disk fills slowly compared to cpu/mem
		s.netIn = walk(s.netIn, 8, 0, 400)
		s.netOut = walk(s.netOut, 5, 0, 200)
		s.cpuHist = append(s.cpuHist[1:], s.cpu)
		s.memHist = append(s.memHist[1:], s.mem)
		s.diskHist = append(s.diskHist[1:], s.disk)
		s.netHist = append(s.netHist[1:], s.netIn)
		s.uptime += 600 * time.Millisecond
	}

	m.host.refreshHost()
	m.host.cpuHist = append(m.host.cpuHist[1:], m.host.cpu)
	m.host.memHist = append(m.host.memHist[1:], m.host.mem)
	m.host.diskHist = append(m.host.diskHist[1:], m.host.disk)
	m.host.netHist = append(m.host.netHist[1:], m.host.netIn)
}

func (m *model) maybeLog() {
	if rand.Float64() > 0.55 {
		return
	}
	srv := m.servers[rand.Intn(len(m.servers))]
	tmpl := logTemplates[rand.Intn(len(logTemplates))]
	tag := tmpl.levels[rand.Intn(len(tmpl.levels))]

	msg := tmpl.tmpl
	if strings.Contains(tmpl.tmpl, "%d") {
		arg := tmpl.argLo
		if tmpl.argHi > tmpl.argLo {
			arg += rand.Intn(tmpl.argHi - tmpl.argLo + 1)
		}
		msg = fmt.Sprintf(tmpl.tmpl, arg)
	}

	line := logLine{
		t:   time.Now(),
		tag: tag,
		col: logLevelColor(tag),
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
