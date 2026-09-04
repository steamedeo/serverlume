// SSH-based live metrics polling, Linux targets only for now (see README).
//
// Rather than pushing and running a remote agent, this reads the same
// files any shell on the box already has access to — /proc/stat,
// /proc/meminfo, /proc/loadavg, /proc/net/dev, /proc/uptime — plus `df`,
// in a single round trip per poll. It's the same "agentless" approach
// tools like Ansible use for fact-gathering: SSH there, run something
// with what's already installed, parse the output, disconnect (or in our
// case, keep the connection warm for the next poll).
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// remoteProbeCmd reads everything we need in one SSH round trip. Each
// section is preceded by a "---NAME---" marker line so the output can be
// split back apart without relying on a fixed number of lines per section.
// Only awk, cat, and df are required, all present on essentially any Linux
// system (including busybox-based ones, e.g. a Raspberry Pi OS Lite image).
const remoteProbeCmd = `echo '---CPU---'; awk '/^cpu /{print $2,$3,$4,$5,$6,$7,$8}' /proc/stat; ` +
	`echo '---MEM---'; awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{print t, a}' /proc/meminfo; ` +
	`echo '---LOAD---'; cat /proc/loadavg; ` +
	`echo '---DISK---'; df -P / | awk 'NR==2{print $5}'; ` +
	`echo '---NET---'; awk 'NR>2 && $1!="lo:"{gsub(":","",$1); rx+=$2; tx+=$10} END{print rx+0, tx+0}' /proc/net/dev; ` +
	`echo '---UPTIME---'; awk '{print $1}' /proc/uptime`

const (
	dialTimeout    = 5 * time.Second
	commandTimeout = 5 * time.Second

	// userCommandTimeout is longer than the metrics probe's: a command
	// typed on the Command tab (a package install, a log grep, whatever)
	// can reasonably take longer than a handful of /proc reads.
	userCommandTimeout = 20 * time.Second
)

// hostSample is one successful poll's worth of numbers for a server.
type hostSample struct {
	cpu, mem, disk float64
	netIn, netOut  float64
	loadAvg        [3]float64
	procs          int
	uptime         time.Duration
}

// rawSample is the previous poll's raw (cumulative) counters, kept per host
// so the next poll can turn them into a rate — the same trick gopsutil uses
// locally in host.go, just over SSH instead of the local /proc.
type rawSample struct {
	t                 time.Time
	cpuTotal, cpuIdle uint64
	netRx, netTx      uint64
}

// sshPoller holds one persistent *ssh.Client per host (reconnecting only on
// failure, rather than paying a fresh handshake every poll — the same
// connection-reuse idea as OpenSSH's ControlPersist) and the previous raw
// sample needed to compute CPU% and network throughput as rates.
type sshPoller struct {
	mu             sync.Mutex
	clients        map[string]*ssh.Client
	prev           map[string]rawSample
	knownHostsPath string
	hostKeyCB      ssh.HostKeyCallback
	hostKeyCBError error
}

func newSSHPoller() *sshPoller {
	p := &sshPoller{clients: map[string]*ssh.Client{}, prev: map[string]rawSample{}}
	if home, err := os.UserHomeDir(); err == nil {
		p.knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}
	p.hostKeyCB, p.hostKeyCBError = sshHostKeyCallback(p.knownHostsPath)
	return p
}

// sshHostKeyCallback verifies remote host keys against the user's own
// ~/.ssh/known_hosts, the same file `ssh` itself trusts. We deliberately do
// not fall back to skipping verification: a dashboard silently accepting any
// host key would defeat the point of using SSH at all. If a host isn't in
// known_hosts yet, connect to it once with a regular `ssh` client first.
func sshHostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, fmt.Errorf("could not determine home directory")
	}
	return knownhosts.New(path)
}

// pinnedHostKeyAlgorithms reads which key type(s) are already recorded in
// known_hosts for hostPort, so the handshake can be restricted to exactly
// those. This matters because golang.org/x/crypto/ssh's default algorithm
// negotiation order has no idea what's already pinned — real `ssh` avoids
// the mismatch by doing this same lookup before connecting. Without it, a
// host with only e.g. an ed25519 entry can get flagged as a key mismatch
// simply because the negotiation happened to pick a different (but
// otherwise valid) key type the server also supports.
//
// Returns nil (meaning: use the library default) if the file can't be read
// or has no plain-text entries for hostPort — including every hashed entry
// (the "|1|..." form), which can't be matched without also computing the
// HMAC, so those are simply left to the default negotiation order.
func pinnedHostKeyAlgorithms(path, hostPort string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var algos []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "|1|") {
			continue
		}
		keyType := fields[1]
		for _, h := range strings.Split(fields[0], ",") {
			if strings.TrimPrefix(h, "!") == hostPort && !seen[keyType] {
				seen[keyType] = true
				algos = append(algos, keyType)
			}
		}
	}
	return algos
}

// knownHostsAddr formats host/port the way OpenSSH writes known_hosts
// entries: bare hostname for the default port 22, "[host]:port" otherwise.
func knownHostsAddr(host, port string) string {
	if port == "" || port == "22" {
		return host
	}
	return "[" + host + "]:" + port
}

// pollEvent flags the connection-lifecycle side effects of one Poll call,
// so the caller can log them as real events (connected/disconnected)
// instead of the UI just seeing a number change.
type pollEvent struct {
	connected    bool // a new SSH connection was established this call
	disconnected bool // the connection was torn down this call, due to a failure
}

// Poll connects to (or reuses a connection to) s and returns one fresh
// sample. Only s.isSSH servers are meaningful inputs here.
func (p *sshPoller) Poll(s server) (hostSample, pollEvent, error) {
	var ev pollEvent

	client, didDial, err := p.client(s)
	ev.connected = didDial
	if err != nil {
		return hostSample{}, ev, err
	}

	out, err := runRemote(client, remoteProbeCmd, commandTimeout)
	if err != nil {
		p.invalidate(s.name)
		ev.disconnected = true
		return hostSample{}, ev, err
	}

	sample, err := p.parseSample(s.name, out)
	return sample, ev, err
}

// RunCommand runs an arbitrary user-supplied command on s over the same
// persistent connection Poll uses, returning its stdout/stderr separately
// (unlike Poll's probe, a user command legitimately producing output before
// failing is common and worth showing, not just discarding). A non-zero
// exit status is a normal command failure, not a broken connection, so the
// connection is only invalidated on an actual transport-level error (a
// timeout, a dial failure, a dropped session) — not on the command itself
// simply failing.
func (p *sshPoller) RunCommand(s server, cmdStr string) (stdout, stderr string, err error) {
	client, _, err := p.client(s)
	if err != nil {
		return "", "", err
	}

	stdout, stderr, err = runUserCommand(client, cmdStr, userCommandTimeout)
	if err != nil {
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			p.invalidate(s.name)
		}
	}
	return stdout, stderr, err
}

// client returns a connection for s, reusing an existing one when possible.
// The bool return says whether a fresh connection was just dialed.
func (p *sshPoller) client(s server) (*ssh.Client, bool, error) {
	p.mu.Lock()
	c, ok := p.clients[s.name]
	p.mu.Unlock()
	if ok {
		return c, false, nil
	}

	c, err := p.dial(s)
	if err != nil {
		return nil, false, err
	}
	p.mu.Lock()
	p.clients[s.name] = c
	p.mu.Unlock()
	return c, true, nil
}

func (p *sshPoller) invalidate(alias string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.clients[alias]; ok {
		c.Close()
		delete(p.clients, alias)
	}
	delete(p.prev, alias)
}

func (p *sshPoller) dial(s server) (*ssh.Client, error) {
	if p.hostKeyCB == nil {
		return nil, fmt.Errorf("no ~/.ssh/known_hosts available: %w", p.hostKeyCBError)
	}

	signers := sshSigners(s)
	if len(signers) == 0 {
		return nil, fmt.Errorf("no usable SSH key for %s (checked ssh-agent and %s)", s.name, identityFileList(s))
	}

	user := s.sshUser
	if user == "" {
		user = currentUsername()
	}
	port := s.sshPort
	if port == "" {
		port = "22"
	}

	cfg := &ssh.ClientConfig{
		User:              user,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		HostKeyCallback:   p.hostKeyCB,
		HostKeyAlgorithms: pinnedHostKeyAlgorithms(p.knownHostsPath, knownHostsAddr(s.ip, port)),
		Timeout:           dialTimeout,
	}
	return ssh.Dial("tcp", net.JoinHostPort(s.ip, port), cfg)
}

func identityFileList(s server) string {
	if len(s.sshIdentityFiles) == 0 {
		return "default key locations"
	}
	return strings.Join(s.sshIdentityFiles, ", ")
}

// sshSigners gathers every private key we can plausibly use to authenticate
// as s: whatever ssh-agent is holding, plus any IdentityFile(s) named in the
// SSH config (or the same default filenames OpenSSH itself tries). Keys that
// fail to load (missing, or passphrase-protected — we have no way to prompt
// for one) are skipped rather than treated as fatal.
func sshSigners(s server) []ssh.Signer {
	var signers []ssh.Signer
	signers = append(signers, agentSigners()...)

	paths := s.sshIdentityFiles
	if len(paths) == 0 {
		paths = defaultIdentityFiles()
	}
	for _, path := range paths {
		if signer, err := loadPrivateKey(path); err == nil {
			signers = append(signers, signer)
		}
	}
	return signers
}

// agentSigners best-effort pulls keys from a running ssh-agent via
// SSH_AUTH_SOCK. This works on Linux and macOS (and WSL); on native Windows
// SSH_AUTH_SOCK is typically unset, so this simply contributes nothing and
// callers fall back to IdentityFile-based keys instead.
func agentSigners() []ssh.Signer {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	signers, err := agent.NewClient(conn).Signers()
	if err != nil {
		return nil
	}
	return signers
}

func defaultIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	names := []string{"id_ed25519", "id_ecdsa", "id_rsa"}
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(home, ".ssh", n)
	}
	return paths
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(data)
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// runRemote executes cmd on client and returns its stdout, bounded by
// timeout so one unresponsive host can't stall the poller (and by
// extension, the UI) indefinitely.
func runRemote(client *ssh.Client, cmd string, timeout time.Duration) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	case <-time.After(timeout):
		session.Close()
		return nil, fmt.Errorf("timed out after %s", timeout)
	}
}

// runUserCommand is runRemote's counterpart for arbitrary user-typed
// commands: it keeps stdout and stderr separate (rather than folding
// stderr into the error text) and returns whatever was captured even on a
// non-zero exit, since a failing command's output is usually exactly what
// the user typed it to see.
func runUserCommand(client *ssh.Client, cmdStr string, timeout time.Duration) (stdout, stderr string, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	done := make(chan error, 1)
	go func() { done <- session.Run(cmdStr) }()

	select {
	case runErr := <-done:
		return outBuf.String(), errBuf.String(), runErr
	case <-time.After(timeout):
		session.Close()
		return outBuf.String(), errBuf.String(), fmt.Errorf("timed out after %s", timeout)
	}
}

// parseSample turns remoteProbeCmd's marker-delimited output into a
// hostSample, using alias to look up (and then update) the previous raw
// sample needed for the CPU% and network-throughput rate calculations.
func (p *sshPoller) parseSample(alias string, out []byte) (hostSample, error) {
	sections := map[string]string{}
	var current string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "---") && strings.HasSuffix(line, "---") {
			current = strings.Trim(line, "-")
			continue
		}
		if current != "" && sections[current] == "" {
			sections[current] = line
		}
	}

	cpuFields := strings.Fields(sections["CPU"])
	if len(cpuFields) < 7 {
		return hostSample{}, fmt.Errorf("unexpected /proc/stat output: %q", sections["CPU"])
	}
	var cpuNums [7]uint64
	for i, f := range cpuFields[:7] {
		cpuNums[i], _ = strconv.ParseUint(f, 10, 64)
	}
	cpuTotal := cpuNums[0] + cpuNums[1] + cpuNums[2] + cpuNums[3] + cpuNums[4] + cpuNums[5] + cpuNums[6]
	cpuIdle := cpuNums[3] + cpuNums[4] // idle + iowait

	memFields := strings.Fields(sections["MEM"])
	var memTotal, memAvail float64
	if len(memFields) >= 2 {
		memTotal, _ = strconv.ParseFloat(memFields[0], 64)
		memAvail, _ = strconv.ParseFloat(memFields[1], 64)
	}
	memPct := 0.0
	if memTotal > 0 {
		memPct = (1 - memAvail/memTotal) * 100
	}

	loadFields := strings.Fields(sections["LOAD"])
	var loadAvg [3]float64
	procs := 0
	if len(loadFields) >= 4 {
		loadAvg[0], _ = strconv.ParseFloat(loadFields[0], 64)
		loadAvg[1], _ = strconv.ParseFloat(loadFields[1], 64)
		loadAvg[2], _ = strconv.ParseFloat(loadFields[2], 64)
		if parts := strings.SplitN(loadFields[3], "/", 2); len(parts) == 2 {
			procs, _ = strconv.Atoi(parts[1])
		}
	}

	diskPct := 0.0
	if d := strings.TrimSuffix(strings.TrimSpace(sections["DISK"]), "%"); d != "" {
		diskPct, _ = strconv.ParseFloat(d, 64)
	}

	netFields := strings.Fields(sections["NET"])
	var rx, tx uint64
	if len(netFields) >= 2 {
		rx, _ = strconv.ParseUint(netFields[0], 10, 64)
		tx, _ = strconv.ParseUint(netFields[1], 10, 64)
	}

	uptimeSec, _ := strconv.ParseFloat(sections["UPTIME"], 64)

	now := time.Now()
	p.mu.Lock()
	prev, hadPrev := p.prev[alias]
	p.prev[alias] = rawSample{t: now, cpuTotal: cpuTotal, cpuIdle: cpuIdle, netRx: rx, netTx: tx}
	p.mu.Unlock()

	sample := hostSample{
		mem: memPct, disk: diskPct,
		loadAvg: loadAvg, procs: procs,
		uptime: time.Duration(uptimeSec * float64(time.Second)),
	}
	// Guard every delta against the counter having gone backwards (a reboot,
	// or a NIC/counter reset) — an underflowing uint64 subtraction would
	// otherwise wrap into a huge bogus rate instead of just "no reading yet".
	if hadPrev {
		dt := now.Sub(prev.t).Seconds()
		if cpuTotal > prev.cpuTotal && cpuIdle >= prev.cpuIdle {
			totalDelta := float64(cpuTotal - prev.cpuTotal)
			idleDelta := float64(cpuIdle - prev.cpuIdle)
			sample.cpu = clampPct(100 * (1 - idleDelta/totalDelta))
		}
		if dt > 0 && rx >= prev.netRx && tx >= prev.netTx {
			sample.netIn = bytesToMbps(rx-prev.netRx, dt)
			sample.netOut = bytesToMbps(tx-prev.netTx, dt)
		}
	}
	return sample, nil
}
