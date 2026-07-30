package load

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colBorder      = lipgloss.Color("238")
	colMuted       = lipgloss.Color("245")
	colSubtle      = lipgloss.Color("240")
	colFg          = lipgloss.Color("252")
	colSelectedBg  = lipgloss.Color("236")
	colSelectedFg  = lipgloss.Color("255")
	colSuccess     = lipgloss.Color("42")
	colDestructive = lipgloss.Color("203")
	colWarning     = lipgloss.Color("214")
	colTitle       = lipgloss.Color("255")
	colChipBg      = lipgloss.Color("235")
)

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colTitle)
	styleMuted = lipgloss.NewStyle().Foreground(colMuted)
	styleSubtle = lipgloss.NewStyle().Foreground(colSubtle)
	styleFg    = lipgloss.NewStyle().Foreground(colFg)
	styleKbd = lipgloss.NewStyle().Foreground(colFg).Background(colChipBg).Padding(0, 1)
	styleChip = lipgloss.NewStyle().Foreground(colMuted).Background(colChipBg).Padding(0, 1)
	styleBadgeOK = lipgloss.NewStyle().Foreground(colSuccess).Background(colChipBg).Padding(0, 1)
	styleBadgeBad = lipgloss.NewStyle().Foreground(colDestructive).Background(colChipBg).Padding(0, 1)
	styleBadgeWarn = lipgloss.NewStyle().Foreground(colWarning).Background(colChipBg).Padding(0, 1)
	styleOuter = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colBorder)
	styleSection = lipgloss.NewStyle().Bold(true).Foreground(colMuted).MarginTop(1)
)

func kbd(s string) string { return styleKbd.Render(s) }

func fmtBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
