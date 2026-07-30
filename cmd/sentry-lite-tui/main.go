package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/skndan/sentry-lite/internal/tui"
)

func main() {
	cfg, err := tui.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
	// Skip WithMouseCellMotion so the terminal can select/copy text.
	p := tea.NewProgram(
		tui.New(cfg),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
