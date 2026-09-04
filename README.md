# serverlume

A [k9s](https://k9scli.io/)-style terminal dashboard for keeping an eye on a fleet of
servers — live CPU/memory/network metrics with history charts, and a
scrolling log tail, all rendered with smooth true-color gradients. Built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
 ✦ SERVERLUME  server fleet dashboard        7 up  2 warn  1 down     Mon 14:02:11
───────────────────────────────────────────────────────────────────────────────────
╭ SERVERS  10 nodes ─────────╮ ╭ Overview  Command  Logs ─────────────────────────╮
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
 ↑/↓ select   tab switch view   1-3 jump tab   p pause   q quit
```

> This started as an exploration of how far a terminal UI can go toward
> looking like a modern web dashboard, all in true-color ANSI. See
> [Bringing your own data](#bringing-your-own-data) for exactly what's real
> vs. simulated today.

## Features

- **Server list populated from `~/.ssh/config`, with live metrics polled
  over SSH** — serverlume reads the local OpenSSH client config, lists every
  concrete `Host` alias it defines, and (Linux hosts only, for now) connects
  to each one to read real CPU/memory/disk/network/load numbers straight
  from `/proc`, agentlessly — no daemon, no port, nothing installed on the
  target beyond a standard shell. Falls back to a small demo fleet if no
  config file is found (or it defines no hosts). See
  [Bringing your own data](#bringing-your-own-data) for how this works and
  its current limits.
- **This host** — a divider-separated entry below the fleet list that monitors
  the actual machine serverlume is running on (real CPU/memory/disk/network,
  via [gopsutil](https://github.com/shirou/gopsutil)), kept visually and
  functionally separate from the fleet
- **Overview tab** — region, IP, uptime, load average, gradient gauge bars for
  CPU/memory/disk, network throughput, and real CPU/memory/disk history line
  charts (via [asciigraph](https://github.com/guptarohit/asciigraph)) with
  axes and gridlines
- **Command tab** — run one-off commands on a host over the same persistent
  SSH connection the poller uses (Linux hosts only, same as metrics), with
  real stdout/stderr shown per host, scrollback kept per host as you switch
  between them. One-shot exec, not an interactive shell — no `vim`, `top`,
  or state (`cd`, env vars) carried between commands, since each run is its
  own SSH session
- **Logs tab** — a real, color-coded event feed: every SSH connect,
  disconnect, poll run, and error, as they actually happen (nothing
  simulated) — see [Bringing your own data](#bringing-your-own-data)
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
| `1` – `3` | Jump to a specific tab (Overview / Command / Logs) |
| `p`       | Pause / resume live updates |
| `q` / `Ctrl+C` | Quit               |

## Bringing your own data

The fleet **list** is real: [`ssh.go`](./ssh.go) parses `~/.ssh/config` (via
[kevinburke/ssh_config](https://github.com/kevinburke/ssh_config)) and turns
each concrete `Host` alias into a server entry, resolving
`HostName`/`User`/`Port`/`IdentityFile` through the config's normal matching
rules (including values inherited from a catch-all `Host *` block).

Per-host **metrics** are real too, for Linux hosts: [`sshpoll.go`](./sshpoll.go)
connects to each one directly over SSH (via
[golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh), not a
shelled-out `ssh` binary) and runs a single read of `/proc/stat`,
`/proc/meminfo`, `/proc/loadavg`, `/proc/net/dev`, `/proc/uptime`, and `df`,
the same agentless approach tools like Ansible use for fact-gathering —
nothing is installed on the target, and the connection is reused across
polls (every 4s) rather than reconnected each time. Two consecutive samples
turn the raw `/proc/stat` counters into a real CPU percentage and
`/proc/net/dev` counters into a real throughput rate.

Current limits worth knowing about:

- **Linux targets only.** A remote Windows host would need a different
  transport (WinRM) and a different set of counters (PowerShell/WMI) —
  a reasonable follow-up, not implemented yet.
- **Key-based auth only, no passphrase prompting.** Signing keys are pulled
  from a running `ssh-agent` (via `SSH_AUTH_SOCK` — works on Linux/macOS/WSL,
  typically not on native Windows) and/or the config's `IdentityFile`
  entries (or the usual `~/.ssh/id_ed25519` / `id_ecdsa` / `id_rsa`
  defaults); passphrase-protected keys that aren't in the agent are skipped,
  since there's nowhere in a TUI redraw loop to prompt for one.
- **Host keys are verified against `~/.ssh/known_hosts`**, the same file
  `ssh` itself trusts — deliberately not skipped, since silently accepting
  any host key would defeat the point of using SSH. `ssh <host>` once by
  hand first if a host isn't in there yet. The handshake is also restricted
  to whichever key type(s) are already pinned for that host, the same way
  `ssh` itself behaves — without that, a host pinned under only one key type
  can get flagged as a false "key mismatch" if Go's default negotiation
  picks a different (but otherwise valid) type the server also offers.
- A poll that fails (unreachable host, auth failure, timeout) marks the
  server `down` and leaves its last-known numbers in place rather than
  blanking them.

[`host.go`](./host.go) still shows the pattern for "This host": it reads live
data for the local machine via [gopsutil](https://github.com/shirou/gopsutil)
on every tick instead of over SSH, since it's already local. The `server`
struct and the rest of the UI don't need to change for either data source.

## Tech stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling and layout
- [asciigraph](https://github.com/guptarohit/asciigraph) — line charts
- [go-colorful](https://github.com/lucasb-eyer/go-colorful) — gradient color blending
- [kevinburke/ssh_config](https://github.com/kevinburke/ssh_config) — SSH client config parsing
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) — SSH client for live remote polling
- [gopsutil](https://github.com/shirou/gopsutil) — local host metrics

## Contributing

Issues and pull requests are welcome. Please run `gofmt` and `go vet ./...`
before submitting.

## License

[MIT](./LICENSE)
