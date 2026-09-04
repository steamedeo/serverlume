// Command serverlume is a k9s-style terminal dashboard for monitoring a fleet
// of servers: live CPU/memory/network metrics, log tailing, and a small
// gallery of terminal-native charts, built with Bubble Tea and Lip Gloss.
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

const appVersion = "v0.1.0"
