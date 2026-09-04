# serverlume

A [k9s](https://k9scli.io/)-style terminal dashboard for keeping an eye on a fleet of
servers — live CPU/memory/network metrics with history charts, and a
scrolling log tail, all rendered with smooth true-color gradients. Built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
 ✦ SERVERLUME  server fleet dashboard        7 up  2 warn  1 down     Mon 14:02:11
───────────────────────────────────────────────────────────────────────────────────
╭ SERVERS  10 nodes ─────────╮ ╭ Overview  Logs ──────────────────────────────────╮
│ ▏ web-01           62%     │ │ web-01                                  UP       │
│ ▏ web-02           12%     │ │                                                  │
│ ▏ api-01           88%     │ │ Region        us-east-1                         │
│ ▏ api-02            9%     │ │ IP Address    10.0.1.11                         │
│ ▏ db-primary       41%     │ │ Uptime        12d 4h 30m                        │
│ ▏ db-replica       28%     │ │ Processes     138                               │
│ ▏ cache-01          8%     │ │ Load Avg      0.82  0.71  0.65                  │
│ ▏ worker-01        --      │ │                                                  │
│ ▏ worker-02        28%     │ │ CPU           ████████████░░░░░░░░  62.0%       │
│ ▏ lb-edge          58%     │ │ Memory        ██████░░░░░░░░░░░░░░  34.1%       │
│                             │ │                                                  │
│ ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌ │ │ CPU history      now 62.0%                     │
│ ⌂ THIS HOST                 │ │  100 ┤          ╭╮                             │
│ ⌂ my-laptop         31%     │ │   80 ┤╭╮   ╭╮  ╭╯╰╮   ╭╮                       │
╰─────────────────────────────╯ ╰──────────────────────────────────────────────────╯
 ↑/↓ select   tab switch view   1-2 jump tab   p pause   q quit
```

> This started as an exploration of how far a terminal UI can go toward
> looking like a modern web dashboard, all in true-color ANSI. Per-host
> metrics are currently simulated — see
> [Bringing your own data](#bringing-your-own-data) to wire up live polling.

## Features

- **Server list populated from `~/.ssh/config`** — serverlume reads the local
  OpenSSH client config and lists every concrete `Host` alias it defines, on
  Linux, macOS, or Windows. Falls back to a small demo fleet if no config
  file is found (or it defines no hosts), so there's always something to
  look at. See [Bringing your own data](#bringing-your-own-data) for what's
  simulated today vs. real.
- **This host** — a divider-separated entry below the fleet list that monitors
  the actual machine serverlume is running on (real CPU/memory/disk/network,
  via [gopsutil](https://github.com/shirou/gopsutil)), kept visually and
  functionally separate from the fleet
- **Overview tab** — region, IP, uptime, load average, gradient gauge bars for
  CPU/memory/disk, network throughput, and real CPU/memory/disk history line
  charts (via [asciigraph](https://github.com/guptarohit/asciigraph)) with
  axes and gridlines
- **Logs tab** — a live-streaming, color-coded log tail per server
- Smooth true-color (24-bit) gradients throughout, computed in LUV color
  space via [go-colorful](https://github.com/lucasb-eyer/go-colorful)

## Install

```sh
go install github.com/steamedeo/serverlume@latest
```

Requires a terminal with 24-bit (true color) support for the intended look;
most modern terminals (Windows Terminal, iTerm2, Kitty, WezTerm, Alacritty,
GNOME Terminal, ...) support this out of the box.

### Build from source

```sh
git clone https://github.com/steamedeo/serverlume.git
cd serverlume
go build -o serverlume .
./serverlume
```

## Usage

| Key       | Action                |
| --------- | ---------------------- |
| `↑` / `k` | Select previous server |
| `↓` / `j` | Select next server      |
| `Tab`     | Next detail tab         |
| `Shift+Tab` | Previous detail tab   |
| `1` – `2` | Jump to a specific tab (Overview / Logs) |
| `p`       | Pause / resume live updates |
| `q` / `Ctrl+C` | Quit               |

## Bringing your own data

The fleet **list** is already real: [`ssh.go`](./ssh.go) parses
`~/.ssh/config` (via [kevinburke/ssh_config](https://github.com/kevinburke/ssh_config))
and turns each concrete `Host` alias into a server entry, resolving
`HostName`/`User`/`Port` through the config's normal matching rules
(including values inherited from a catch-all `Host *` block).

Per-host **metrics** (CPU, memory, disk, network, load, uptime) are still
simulated placeholders, seeded and random-walked by `seedPlaceholderMetrics`
and `updateMetrics` in [`model.go`](./model.go) — real per-host polling
(SSH + `/proc`, an exporter, cloud provider SDK, etc.) is a natural next
step. [`host.go`](./host.go) shows the pattern for wiring in a real metrics
source: it reads live data for the local machine via
[gopsutil](https://github.com/shirou/gopsutil) on every tick instead of
walking a random value. The `server` struct and the rest of the UI don't
need to change either way.

## Tech stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling and layout
- [asciigraph](https://github.com/guptarohit/asciigraph) — line charts
- [go-colorful](https://github.com/lucasb-eyer/go-colorful) — gradient color blending
- [kevinburke/ssh_config](https://github.com/kevinburke/ssh_config) — SSH client config parsing
- [gopsutil](https://github.com/shirou/gopsutil) — local host metrics

## Contributing

Issues and pull requests are welcome. Please run `gofmt` and `go vet ./...`
before submitting.

## License

[MIT](./LICENSE)
