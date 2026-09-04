package main

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	gopshost "github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	gopsnet "github.com/shirou/gopsutil/v3/net"
)

// netSample remembers the last cumulative network-byte counters so
// refreshHost can turn them into a throughput rate between ticks.
type netSample struct {
	t         time.Time
	bytesSent uint64
	bytesRecv uint64
}

var lastNetSample netSample

// newHostEntry builds the sidebar entry for the machine serverlume is
// actually running on. Unlike the demo fleet, its metrics come from a real
// system read, not a random walk.
func newHostEntry() server {
	s := server{
		name:   "this-host",
		user:   currentUsername(),
		ip:     "127.0.0.1",
		os:     runtime.GOOS,
		status: "up",
		isHost: true,
	}
	if hi, err := gopshost.Info(); err == nil {
		if hi.Hostname != "" {
			s.name = hi.Hostname
		}
		s.os = hi.Platform + " " + hi.PlatformVersion
	}
	s.refreshHost()
	for i := 0; i < 40; i++ {
		s.cpuHist = append(s.cpuHist, s.cpu)
		s.memHist = append(s.memHist, s.mem)
		s.diskHist = append(s.diskHist, s.disk)
		s.netHist = append(s.netHist, s.netIn)
	}
	return s
}

// refreshHost samples live CPU/memory/disk/network/uptime numbers for the
// local machine. A failed sample is left as the previous reading rather than
// blanking the UI, since gopsutil calls can occasionally miss on some
// platforms.
func (s *server) refreshHost() {
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		s.cpu = pct[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		s.mem = vm.UsedPercent
	}
	if du, err := disk.Usage(hostDiskPath()); err == nil {
		s.disk = du.UsedPercent
	}
	if hi, err := gopshost.Info(); err == nil {
		s.uptime = time.Duration(hi.Uptime) * time.Second
		s.procs = int(hi.Procs)
	}
	if counters, err := gopsnet.IOCounters(false); err == nil && len(counters) > 0 {
		now := time.Now()
		c := counters[0]
		if !lastNetSample.t.IsZero() {
			dt := now.Sub(lastNetSample.t).Seconds()
			if dt > 0 {
				s.netIn = bytesToMbps(c.BytesRecv-lastNetSample.bytesRecv, dt)
				s.netOut = bytesToMbps(c.BytesSent-lastNetSample.bytesSent, dt)
			}
		}
		lastNetSample = netSample{t: now, bytesRecv: c.BytesRecv, bytesSent: c.BytesSent}
	}
}

func bytesToMbps(deltaBytes uint64, seconds float64) float64 {
	return float64(deltaBytes) / seconds / 1e6 * 8
}

func hostDiskPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}
