# serverlume

A [k9s](https://k9scli.io/)-style terminal dashboard for keeping an eye on a fleet of
servers — live CPU/memory/network metrics, a scrolling log tail, and a small
gallery of terminal-native charts, all rendered with smooth true-color
gradients. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
 ✦ SERVERLUME  server fleet dashboard        7 up  2 warn  1 down     Mon 14:02:11
───────────────────────────────────────────────────────────────────────────────────
╭ SERVERS  10 nodes ─────────╮ ╭ Overview  Metrics  Logs  Charts ─────────────────╮
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
╰─────────────────────────────╯ ╰──────────────────────────────────────────────────╯
 ↑/↓ select   tab switch view   1-4 jump tab   p pause   q quit
```

> This started as an exploration of how far a terminal UI can go toward
> looking like a modern web dashboard, all in true-color ANSI. The data is
> currently simulated — see [Bringing your own data](#bringing-your-own-data)
> to wire it up to real infrastructure.

## Features

- **Server list** with live status (up / warn / down) and CPU at a glance
- **Overview tab** — region, IP, uptime, load average, gradient gauge bars for
  CPU/memory/disk, network throughput
- **Metrics tab** — real line charts (via
  [asciigraph](https://github.com/guptarohit/asciigraph)) of CPU/memory
  history with axes and gridlines
- **Logs tab** — a live-streaming, color-coded log tail per server
- **Charts tab** — a small gallery of other terminal-native visualizations:
  a gradient-filled ranking bar chart, a proportional dot-grid donut, and a
  GitHub-contributions-style load heatmap across the whole fleet
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
| `1` – `4` | Jump to a specific tab (Overview / Metrics / Logs / Charts) |
| `p`       | Pause / resume live updates |
| `q` / `Ctrl+C` | Quit               |

## Bringing your own data

Everything currently comes from `newServers()` and `updateMetrics()` in
[`model.go`](./model.go), which seed and random-walk simulated metrics. To
point serverlume at real infrastructure, replace those with calls to your own
inventory source and metrics backend (SSH + `/proc`, a Prometheus query,
Docker/Kubernetes API, cloud provider SDK, etc.) — the `server` struct and the
rest of the UI don't need to change.

## Tech stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling and layout
- [asciigraph](https://github.com/guptarohit/asciigraph) — line charts
- [go-colorful](https://github.com/lucasb-eyer/go-colorful) — gradient color blending

## Contributing

Issues and pull requests are welcome. Please run `gofmt` and `go vet ./...`
before submitting.

## License

[MIT](./LICENSE)
