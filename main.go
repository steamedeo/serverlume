// Command serverlume is a k9s-style terminal dashboard for monitoring a
// fleet of servers discovered from ~/.ssh/config: live CPU/memory/disk/
// network metrics and history charts, a real-time SSH connection/error
// log, and a per-host command runner, built with Bubble Tea and Lip Gloss.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const appVersion = "v0.2.1"
