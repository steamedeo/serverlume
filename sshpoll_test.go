package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSampleFirstAndSecondPoll(t *testing.T) {
	p := &sshPoller{prev: map[string]rawSample{}}

	first := []byte("---CPU---\n100 0 0 900 0 0 0\n" +
		"---MEM---\n1000000 400000\n" +
		"---LOAD---\n0.10 0.20 0.30 2/150 12345\n" +
		"---DISK---\n42%\n" +
		"---NET---\n1000 2000\n" +
		"---UPTIME---\n86400.5\n")

	s1, err := p.parseSample("host-a", first)
	if err != nil {
		t.Fatalf("parseSample (first): %v", err)
	}
	if s1.cpu != 0 || s1.netIn != 0 || s1.netOut != 0 {
		t.Errorf("first poll should have no rate yet, got cpu=%v netIn=%v netOut=%v", s1.cpu, s1.netIn, s1.netOut)
	}
	if got, want := s1.mem, 60.0; got != want {
		t.Errorf("mem = %v, want %v", got, want)
	}
	if got, want := s1.disk, 42.0; got != want {
		t.Errorf("disk = %v, want %v", got, want)
	}
	if got, want := s1.loadAvg, ([3]float64{0.10, 0.20, 0.30}); got != want {
		t.Errorf("loadAvg = %v, want %v", got, want)
	}
	if got, want := s1.procs, 150; got != want {
		t.Errorf("procs = %v, want %v", got, want)
	}
	if got, want := s1.uptime, time.Duration(86400.5*float64(time.Second)); got != want {
		t.Errorf("uptime = %v, want %v", got, want)
	}

	// Simulate ~1s passing with the CPU going 50% busy (total +200, idle
	// +100) and some network traffic, by seeding the previous raw sample
	// directly rather than depending on parseSample's internal timing.
	p.prev["host-a"] = rawSample{
		t:        time.Now().Add(-time.Second),
		cpuTotal: 1000, // 100+0+0+900+0+0+0
		cpuIdle:  900,
		netRx:    1000, netTx: 2000,
	}
	second := []byte("---CPU---\n200 0 0 1000 0 0 0\n" +
		"---MEM---\n1000000 400000\n" +
		"---LOAD---\n0.10 0.20 0.30 2/150 12345\n" +
		"---DISK---\n42%\n" +
		"---NET---\n2000000 3000000\n" +
		"---UPTIME---\n86401.5\n")

	s2, err := p.parseSample("host-a", second)
	if err != nil {
		t.Fatalf("parseSample (second): %v", err)
	}
	// total delta = (200+1000) - (100+900) = 200; idle delta = 1000-900 = 100
	// busy% = 100 * (1 - 100/200) = 50
	if got, want := s2.cpu, 50.0; got != want {
		t.Errorf("cpu = %v, want %v", got, want)
	}
	if s2.netIn <= 0 || s2.netOut <= 0 {
		t.Errorf("expected positive network rates, got netIn=%v netOut=%v", s2.netIn, s2.netOut)
	}
}

func TestParseSampleMalformedCPU(t *testing.T) {
	p := &sshPoller{prev: map[string]rawSample{}}
	if _, err := p.parseSample("host-a", []byte("---CPU---\nnot enough fields\n")); err == nil {
		t.Fatal("expected an error for malformed /proc/stat output")
	}
}

// TestPinnedHostKeyAlgorithms guards against a real bug that hit two out of
// four of a real fleet: golang.org/x/crypto/ssh's default host-key algorithm
// negotiation order has no idea what's already in known_hosts, so a host
// pinned under only one key type (e.g. just ssh-ed25519, not also
// ssh-rsa/ecdsa) could get flagged as a "key mismatch" simply because the
// negotiation happened to pick a different, but otherwise valid, type the
// server also supports. Restricting HostKeyAlgorithms to what's already
// pinned — same as what `ssh` itself does — avoids that.
func TestPinnedHostKeyAlgorithms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	contents := "one-key-host ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOnekey\n" +
		"multi-key-host ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOmultia\n" +
		"multi-key-host ssh-rsa AAAAB3NzaC1yc2EAAAADmultib\n" +
		"[custom-port-host]:2222 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOcustom\n" +
		"|1|abc123hashed|def456== ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOhashed\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	cases := []struct {
		name string
		want []string
	}{
		{"one-key-host", []string{"ssh-ed25519"}},
		{"multi-key-host", []string{"ssh-ed25519", "ssh-rsa"}},
		{"unknown-host", nil},
	}
	for _, c := range cases {
		got := pinnedHostKeyAlgorithms(path, c.name)
		if !equalStrings(got, c.want) {
			t.Errorf("pinnedHostKeyAlgorithms(%q) = %v, want %v", c.name, got, c.want)
		}
	}

	if got := pinnedHostKeyAlgorithms(path, "custom-port-host:2222"); len(got) != 0 {
		t.Errorf("plain host:port form should not match the bracketed known_hosts form, got %v", got)
	}
	if got := pinnedHostKeyAlgorithms(path, knownHostsAddr("custom-port-host", "2222")); !equalStrings(got, []string{"ssh-ed25519"}) {
		t.Errorf("pinnedHostKeyAlgorithms with knownHostsAddr formatting = %v, want [ssh-ed25519]", got)
	}
	// The hashed entry must never match a plaintext lookup — matching it
	// would require reproducing the HMAC, which this helper deliberately
	// doesn't attempt (see its doc comment).
	if got := pinnedHostKeyAlgorithms(path, "abc123hashed"); len(got) != 0 {
		t.Errorf("hashed known_hosts entries should not be matched, got %v", got)
	}
}

func TestKnownHostsAddr(t *testing.T) {
	if got, want := knownHostsAddr("example.com", ""), "example.com"; got != want {
		t.Errorf("knownHostsAddr with empty port = %q, want %q", got, want)
	}
	if got, want := knownHostsAddr("example.com", "22"), "example.com"; got != want {
		t.Errorf("knownHostsAddr with port 22 = %q, want %q", got, want)
	}
	if got, want := knownHostsAddr("example.com", "2222"), "[example.com]:2222"; got != want {
		t.Errorf("knownHostsAddr with non-default port = %q, want %q", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
