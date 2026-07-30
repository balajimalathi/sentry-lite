package tui

import "github.com/charmbracelet/lipgloss"

// Shadcn-inspired muted terminal tokens.
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

	styleKbd = lipgloss.NewStyle().
			Foreground(colFg).
			Background(colChipBg).
			Padding(0, 1)

	styleBadgeDefault = lipgloss.NewStyle().
				Foreground(colMuted).
				Background(colChipBg).
				Padding(0, 1)

	styleBadgeOK = lipgloss.NewStyle().
			Foreground(colSuccess).
			Background(colChipBg).
			Padding(0, 1)

	styleBadgeBad = lipgloss.NewStyle().
			Foreground(colDestructive).
			Background(colChipBg).
			Padding(0, 1)

	styleBadgeWarn = lipgloss.NewStyle().
			Foreground(colWarning).
			Background(colChipBg).
			Padding(0, 1)

	styleChip = lipgloss.NewStyle().
			Foreground(colMuted).
			Background(colChipBg).
			Padding(0, 1)

	styleSidebarTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colMuted).
				MarginBottom(1)

	styleSideItem = lipgloss.NewStyle().
			Foreground(colMuted).
			Padding(0, 1)

	styleSideItemSelected = lipgloss.NewStyle().
				Foreground(colSelectedFg).
				Background(colSelectedBg).
				Bold(true).
				Padding(0, 1)

	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colFg).
			Padding(0, 1)

	styleSectionTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colMuted).
				MarginTop(1).
				MarginBottom(0)

	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1).
			MarginRight(1)

	styleCardLabel = lipgloss.NewStyle().Foreground(colMuted)
	styleCardValue = lipgloss.NewStyle().Bold(true).Foreground(colFg)

	styleOuter = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder)

	styleInnerBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colBorder)
)

func badgeForState(state string) string {
	switch state {
	case "running", "live":
		return styleBadgeOK.Render("● " + shortState(state))
	case "failed":
		return styleBadgeBad.Render("● " + shortState(state))
	case "starting":
		return styleBadgeWarn.Render("● " + shortState(state))
	default:
		return styleBadgeDefault.Render("○ " + shortState(state))
	}
}

func shortState(state string) string {
	switch state {
	case "running":
		return "run"
	case "starting":
		return "…"
	case "stopped":
		return "stop"
	case "failed":
		return "fail"
	case "live":
		return "live"
	case "idle":
		return "idle"
	default:
		if len(state) > 5 {
			return state[:5]
		}
		return state
	}
}

func kbd(s string) string {
	return styleKbd.Render(s)
}
