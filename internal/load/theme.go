package load

import (
	"fmt"
	"strings"
	"time"

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
	colAccent      = lipgloss.Color("117")
	colBarFill     = lipgloss.Color("75")
	colBarEmpty    = lipgloss.Color("238")
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colTitle)
	styleMuted   = lipgloss.NewStyle().Foreground(colMuted)
	styleSubtle  = lipgloss.NewStyle().Foreground(colSubtle)
	styleFg      = lipgloss.NewStyle().Foreground(colFg)
	styleAccent  = lipgloss.NewStyle().Foreground(colAccent)
	styleKbd     = lipgloss.NewStyle().Foreground(colFg).Background(colChipBg).Padding(0, 1)
	styleChip    = lipgloss.NewStyle().Foreground(colMuted).Background(colChipBg).Padding(0, 1)
	styleBadgeOK = lipgloss.NewStyle().Foreground(colSuccess).Background(colChipBg).Padding(0, 1)
	styleBadgeBad = lipgloss.NewStyle().Foreground(colDestructive).Background(colChipBg).Padding(0, 1)
	styleBadgeWarn = lipgloss.NewStyle().Foreground(colWarning).Background(colChipBg).Padding(0, 1)
	styleOuter   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colBorder)
	styleSection = lipgloss.NewStyle().Bold(true).Foreground(colMuted).MarginTop(1)

	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1).
			MarginRight(1)

	styleCardLabel = lipgloss.NewStyle().Foreground(colMuted)
	styleCardValue = lipgloss.NewStyle().Bold(true).Foreground(colFg)
	styleCardDelta = lipgloss.NewStyle().Foreground(colAccent)
	styleCardPeak  = lipgloss.NewStyle().Foreground(colWarning)
)

func kbd(s string) string { return styleKbd.Render(s) }

func fmtBytes(n uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.0f KiB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func miniCard(label, value, sub string, width int) string {
	inner := styleCardLabel.Render(label) + "\n" + styleCardValue.Render(value)
	if sub != "" {
		inner += "\n" + styleCardDelta.Render(sub)
	}
	return styleCard.Width(width).Render(inner)
}

func progressBar(pct float64, width int) string {
	if width < 8 {
		width = 8
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	fillStyle := lipgloss.NewStyle().Foreground(colBarFill)
	emptyStyle := lipgloss.NewStyle().Foreground(colBarEmpty)
	if pct >= 0.85 {
		fillStyle = lipgloss.NewStyle().Foreground(colDestructive)
	} else if pct >= 0.65 {
		fillStyle = lipgloss.NewStyle().Foreground(colWarning)
	}
	bar := fillStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", width-filled))
	return bar + styleMuted.Render(fmt.Sprintf(" %4.1f%%", pct*100))
}

func renderResourcePanel(r ResourceSnapshot, base RunBaselines, _ time.Duration, width int) string {
	if width < 60 {
		width = 60
	}
	cardW := maxInt(16, (width-8)/4)

	apiVal := "—"
	apiSub := ""
	if r.APIFound {
		apiVal = fmtBytes(r.APIRSS)
		parts := []string{fmt.Sprintf("%.0f%% cpu", r.APICPU)}
		if base.Set {
			parts = append(parts, fmt.Sprintf("Δ %s", fmtDelta(r.APIRSS, base.APIRSS)))
			parts = append(parts, fmt.Sprintf("peak %s", fmtBytes(base.PeakAPIRSS)))
		}
		apiSub = strings.Join(parts, "  ")
	}

	hostVal := "—"
	hostSub := ""
	if r.HostTotalRAM > 0 {
		hostVal = fmt.Sprintf("%s / %s", fmtBytes(r.HostUsedRAM), fmtBytes(r.HostTotalRAM))
		if r.HostTotalRAM > 0 {
			hostSub = fmt.Sprintf("%.1f%% used", 100*float64(r.HostUsedRAM)/float64(r.HostTotalRAM))
		}
	}

	rpVal := "—"
	rpSub := ""
	if r.RedpandaOK {
		rpVal = r.RedpandaMem
		rpSub = fmt.Sprintf("cpu %s  mem %s", r.RedpandaCPU, r.RedpandaMemPct)
	}

	dataVal := fmtBytes(r.DataBytes)
	dataSub := ""
	if base.Set {
		dataSub = fmt.Sprintf("Δ %s  peak %s", fmtDelta(r.DataBytes, base.DataBytes), fmtBytes(base.PeakData))
	}

	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		miniCard("api ram", apiVal, apiSub, cardW),
		miniCard("host ram", hostVal, hostSub, cardW),
		miniCard("redpanda", rpVal, rpSub, cardW),
		miniCard("data/", dataVal, dataSub, cardW),
	)

	var b strings.Builder
	b.WriteString(styleSection.Render("RAM & storage") + "\n")
	b.WriteString(cards + "\n")

	// Progress: API vs host
	if r.HostTotalRAM > 0 && r.APIRSS > 0 {
		pct := float64(r.APIRSS) / float64(r.HostTotalRAM)
		b.WriteString("\n")
		b.WriteString(styleMuted.Render("api vs host  ") + progressBar(pct, maxInt(20, width-28)) + "\n")
	}

	// Progress: data growth toward free disk (used of data+free as capacity hint)
	capacity := r.DataBytes + r.DiskFree
	if capacity > 0 && r.DataBytes > 0 {
		pct := float64(r.DataBytes) / float64(capacity)
		b.WriteString(styleMuted.Render("data vs free ") + progressBar(pct, maxInt(20, width-28)) + "\n")
	}

	// Detail row — events/ is not scanned (can be millions of files under load).
	sqliteDelta := ""
	if base.Set {
		sqliteDelta = "  " + styleCardDelta.Render(fmtDelta(r.SQLiteBytes, base.SQLiteBytes))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "events/", styleSubtle.Render("skipped (too large to scan)")))
	b.WriteString(fmt.Sprintf("  %-12s %s%s\n", "sqlite", styleFg.Render(fmtBytes(r.SQLiteBytes)), sqliteDelta))
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "disk free", styleFg.Render(fmtBytes(r.DiskFree))))
	if r.APIFound {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", "api pid", styleSubtle.Render(fmt.Sprintf("%d", r.APIPID))))
	}

	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
