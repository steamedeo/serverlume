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
	user     string
	ip       string
	os       string
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

	// SSH connection details, set only when isSSH is true. Metrics for these
	// servers come from live polling (see sshpoll.go), not the local random
	// walk that drives the demo fleet.
	isSSH            bool
	sshUser          string
	sshPort          string
	sshIdentityFiles []string
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

// appendLog records one real event line (a connect, disconnect, poll run,
// or error — see applySSHSample) in the Logs tab's feed.
func (m *model) appendLog(tag, msg string) {
	m.logs = append(m.logs, logLine{t: time.Now(), tag: tag, col: logLevelColor(tag), msg: msg})
	if len(m.logs) > 200 {
		m.logs = m.logs[len(m.logs)-200:]
	}
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
	names := []struct{ name, user, ip, os, status string }{
		{"web-01", "deploy", "10.0.1.11", "Ubuntu 22.04", "up"},
		{"web-02", "deploy", "10.0.1.12", "Ubuntu 22.04", "up"},
		{"api-01", "deploy", "10.0.2.10", "Debian 12", "warn"},
		{"api-02", "deploy", "10.0.2.11", "Debian 12", "up"},
		{"db-primary", "postgres", "10.0.3.5", "Ubuntu 22.04", "up"},
		{"db-replica", "postgres", "10.0.3.6", "Ubuntu 22.04", "up"},
		{"cache-01", "redis", "10.0.4.2", "Alpine 3.19", "up"},
		{"worker-01", "worker", "10.0.5.20", "Debian 12", "down"},
		{"worker-02", "worker", "10.0.5.21", "Debian 12", "up"},
		{"lb-edge", "root", "10.0.0.1", "Alpine 3.19", "up"},
	}
	srv := make([]server, 0, len(names))
	for _, n := range names {
		s := server{name: n.name, user: n.user, ip: n.ip, os: n.os, status: n.status}
		seedPlaceholderMetrics(&s)
		srv = append(srv, s)
	}
	return srv
}

// seedSSHServer turns a discovered SSH config alias into a server entry.
// Its identity (name/IP/region) is real; its metrics start as simulated
// placeholders and get overwritten by a live SSH poll (see sshpoll.go)
// within the first few seconds. Status starts "warn" (unverified) rather
// than "up", since we haven't actually reached the host yet.
func seedSSHServer(h sshHost) server {
	user := h.User
	if user == "" {
		user = currentUsername()
	}
	s := server{
		name: h.Alias, user: user, ip: h.HostName, status: "warn",
		isSSH: true, sshUser: h.User, sshPort: h.Port, sshIdentityFiles: h.IdentityFile,
	}
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
	tabCommand
	tabLogs
)

var tabNames = []string{"Overview", "Command", "Logs"}

type tickMsg time.Time
type bootTickMsg time.Time

// sshPollTickMsg drives the periodic SSH re-poll of every discovered host.
// It runs on its own, much slower cadence than tickMsg: SSH round trips
// (tens to hundreds of ms, more over a slow link) are far too slow to fit
// inside the UI's 600ms redraw tick.
type sshPollTickMsg time.Time

const sshPollInterval = 4 * time.Second

// sshSampleMsg carries one host's poll result back into Update. err is set
// (and sample left zero) when the poll failed — a closed connection,
// unreachable host, auth failure, or timeout. connected/disconnected mirror
// pollEvent, so Update can log the connection lifecycle as real events.
type sshSampleMsg struct {
	alias                   string
	sample                  hostSample
	err                     error
	connected, disconnected bool
}

// bootFrames controls how long the startup splash animates before the
// dashboard takes over.
const bootFrames = 14

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type model struct {
	servers     []server
	fleetSource string // where servers came from, shown in the sidebar (e.g. "~/.ssh/config", "demo data")
	host        server // this machine — kept separate from servers, never mixed into fleet-wide views
	poller      *sshPoller
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

	// Command tab state, keyed by server alias so each host keeps its own
	// in-progress input and scrollback independently of which one is
	// currently selected.
	cmdInputs  map[string]string
	cmdHistory map[string][]cmdEntry
	cmdRunning map[string]bool
}

func initialModel() model {
	servers, source := newServers()
	return model{
		servers:     servers,
		fleetSource: source,
		host:        newHostEntry(),
		poller:      newSSHPoller(),
		tab:         tabOverview,
		booting:     true,
		cmdInputs:   map[string]string{},
		cmdHistory:  map[string][]cmdEntry{},
		cmdRunning:  map[string]bool{},
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
	return tea.Batch(
		tickCmd(), bootTickCmd(), tea.SetWindowTitle("serverlume"),
		sshPollTickCmd(), m.pollAllSSHCmd(),
	)
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

func sshPollTickCmd() tea.Cmd {
	return tea.Tick(sshPollInterval, func(t time.Time) tea.Msg {
		return sshPollTickMsg(t)
	})
}

// pollAllSSHCmd fires off one poll per SSH-sourced host, concurrently
// (each tea.Cmd in the batch runs in its own goroutine) rather than one
// after another, so a single slow or unreachable host can't delay the rest.
func (m model) pollAllSSHCmd() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.servers))
	for _, s := range m.servers {
		if s.isSSH {
			cmds = append(cmds, pollHostCmd(m.poller, s))
		}
	}
	return tea.Batch(cmds...)
}

func pollHostCmd(poller *sshPoller, s server) tea.Cmd {
	return func() tea.Msg {
		sample, ev, err := poller.Poll(s)
		return sshSampleMsg{
			alias: s.name, sample: sample, err: err,
			connected: ev.connected, disconnected: ev.disconnected,
		}
	}
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

		// While the Command tab is focused on an SSH host, typing keys go
		// into that host's command line instead of being read as shortcuts
		// — otherwise you could never type a command containing "q", "p",
		// or a digit. Navigation keys (arrows, tab-switching, ctrl+c) are
		// deliberately not intercepted here, so they still work normally.
		if m.tab == tabCommand && m.selected().isSSH {
			if cmd, handled := m.handleCommandInputKey(msg); handled {
				return m, cmd
			}
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
			m.tab = tabCommand
		case "3":
			m.tab = tabLogs
		case "p":
			m.paused = !m.paused
		}
		return m, nil

	case tickMsg:
		m.pulseOn = !m.pulseOn
		if !m.paused {
			m.updateMetrics()
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

	case sshPollTickMsg:
		return m, tea.Batch(m.pollAllSSHCmd(), sshPollTickCmd())

	case sshSampleMsg:
		m.applySSHSample(msg)
		return m, nil

	case cmdResultMsg:
		m.cmdRunning[msg.alias] = false
		hist := append(m.cmdHistory[msg.alias], cmdEntry{
			t: time.Now(), cmd: msg.cmd, stdout: msg.stdout, stderr: msg.stderr, err: msg.err,
		})
		if len(hist) > 50 {
			hist = hist[len(hist)-50:]
		}
		m.cmdHistory[msg.alias] = hist
		return m, nil
	}
	return m, nil
}

// cmdEntry is one command run on the Command tab and its result.
type cmdEntry struct {
	t              time.Time
	cmd            string
	stdout, stderr string
	err            error
}

// cmdResultMsg carries a finished command run back into Update.
type cmdResultMsg struct {
	alias          string
	cmd            string
	stdout, stderr string
	err            error
}

func runCommandCmd(poller *sshPoller, s server, cmdStr string) tea.Cmd {
	return func() tea.Msg {
		stdout, stderr, err := poller.RunCommand(s, cmdStr)
		return cmdResultMsg{alias: s.name, cmd: cmdStr, stdout: stdout, stderr: stderr, err: err}
	}
}

// handleCommandInputKey feeds one keypress into the currently selected
// host's command line. The bool return says whether the key was consumed
// (so the caller shouldn't also treat it as a global shortcut) — only
// text-editing key types are handled here; anything else (arrows,
// tab-switching, etc.) is left alone.
func (m *model) handleCommandInputKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	alias := m.selected().name
	switch msg.Type {
	case tea.KeyEnter:
		cmdStr := strings.TrimSpace(m.cmdInputs[alias])
		if cmdStr == "" || m.cmdRunning[alias] {
			return nil, true
		}
		m.cmdInputs[alias] = ""
		m.cmdRunning[alias] = true
		return runCommandCmd(m.poller, m.selected(), cmdStr), true

	case tea.KeyBackspace:
		if r := []rune(m.cmdInputs[alias]); len(r) > 0 {
			m.cmdInputs[alias] = string(r[:len(r)-1])
		}
		return nil, true

	case tea.KeySpace:
		m.cmdInputs[alias] += " "
		return nil, true

	case tea.KeyRunes:
		m.cmdInputs[alias] += string(msg.Runes)
		return nil, true
	}
	return nil, false
}

// applySSHSample writes a completed poll's result into the matching server,
// and logs exactly what happened this round: a connect, the poll run
// itself, any error, and a disconnect — nothing simulated. A failed poll
// marks the host "down" and otherwise leaves its last-known values in
// place, rather than blanking them out on one flaky round trip.
func (m *model) applySSHSample(msg sshSampleMsg) {
	if msg.connected {
		m.appendLog("OK", fmt.Sprintf("[%s] ssh connection established", msg.alias))
	}

	if msg.err != nil {
		m.appendLog("ERROR", fmt.Sprintf("[%s] %s", msg.alias, msg.err))
		if msg.disconnected {
			m.appendLog("WARN", fmt.Sprintf("[%s] ssh connection closed", msg.alias))
		}
		for i := range m.servers {
			if m.servers[i].name == msg.alias {
				m.servers[i].status = "down"
				break
			}
		}
		return
	}

	smp := msg.sample
	m.appendLog("INFO", fmt.Sprintf("[%s] metrics collected (cpu %.1f%%, mem %.1f%%, disk %.1f%%)",
		msg.alias, smp.cpu, smp.mem, smp.disk))

	for i := range m.servers {
		s := &m.servers[i]
		if s.name != msg.alias {
			continue
		}
		s.status = "up"
		s.cpu, s.mem, s.disk = smp.cpu, smp.mem, smp.disk
		s.netIn, s.netOut = smp.netIn, smp.netOut
		s.loadAvg = smp.loadAvg
		s.procs = smp.procs
		s.uptime = smp.uptime
		if smp.os != "" {
			s.os = smp.os
		}
		s.cpuHist = append(s.cpuHist[1:], s.cpu)
		s.memHist = append(s.memHist[1:], s.mem)
		s.diskHist = append(s.diskHist[1:], s.disk)
		s.netHist = append(s.netHist[1:], s.netIn)
		return
	}
}

func (m *model) updateMetrics() {
	for i := range m.servers {
		s := &m.servers[i]
		if s.isSSH {
			// Real numbers arrive asynchronously from sshSampleMsg on their
			// own cadence (sshPollInterval); a local random walk here would
			// just fight with — and get overwritten by — the real reading.
			s.uptime += 600 * time.Millisecond
			continue
		}
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

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}
