package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 26

func (m Model) viewportSize() (w, h int) {
	innerW := maxInt(40, m.width-2)
	innerH := maxInt(8, m.height-2)
	bodyH := maxInt(5, innerH-3)
	contentW := maxInt(18, innerW-sidebarWidth-3)
	contentH := maxInt(3, bodyH-2)
	return contentW, contentH
}

func (m Model) renderFrame() string {
	innerW := maxInt(40, m.width-2)

	header := m.renderHeader(innerW)
	sidebar := m.renderSidebar(bodyHeight(m.height))
	main := m.renderMain()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
	footer := m.renderFooter(innerW)

	inner := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return styleOuter.Width(m.width).Height(m.height).Render(inner)
}

func bodyHeight(termH int) int {
	innerH := maxInt(8, termH-2)
	return maxInt(5, innerH-3)
}

func (m Model) renderHeader(width int) string {
	left := styleTitle.Render("sentry-lite") + " " + styleMuted.Render("dev")

	chips := []string{
		styleChip.Render("api " + shortURL(m.cfg.APIURL)),
		styleChip.Render("web :5173"),
	}
	rss := m.stats.API.RSSBytes + m.stats.Web.RSSBytes
	if rss > 0 {
		chips = append(chips, styleChip.Render("rss "+formatBytes(rss)))
	}
	if m.stats.Disk.DataDir > 0 {
		chips = append(chips, styleChip.Render("data "+formatBytes(m.stats.Disk.DataDir)))
	}
	right := strings.Join(chips, " ")

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return styleSubtle.Width(width).Render(line)
}

func (m Model) renderSidebar(height int) string {
	var b strings.Builder
	b.WriteString(styleSidebarTitle.Render("Services") + "\n")

	panels := []PanelID{PanelRedpanda, PanelAPI, PanelWeb, PanelStats}
	innerW := sidebarWidth - 2

	for _, p := range panels {
		state := m.panelState(p)
		marker := "  "
		if p == m.focus {
			marker = "▸ "
		}
		name := p.String()
		badge := badgeForState(state)

		namePart := marker + name
		pad := innerW - lipgloss.Width(namePart) - lipgloss.Width(badge)
		if pad < 1 {
			pad = 1
		}
		row := namePart + strings.Repeat(" ", pad) + badge

		if p == m.focus {
			b.WriteString(styleSideItemSelected.Width(innerW).Render(row) + "\n")
		} else {
			b.WriteString(styleSideItem.Width(innerW).Render(row) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styleSubtle.Render("Resources") + "\n")
	if m.stats.API.RSSBytes > 0 {
		b.WriteString(styleMuted.Render(fmt.Sprintf("  api  %s", formatBytes(m.stats.API.RSSBytes))) + "\n")
	}
	if m.stats.Web.RSSBytes > 0 {
		b.WriteString(styleMuted.Render(fmt.Sprintf("  web  %s", formatBytes(m.stats.Web.RSSBytes))) + "\n")
	}
	if m.stats.Disk.DataDir > 0 {
		b.WriteString(styleMuted.Render(fmt.Sprintf("  data %s", formatBytes(m.stats.Disk.DataDir))) + "\n")
	}
	if m.stats.Disk.EventFiles > 0 {
		b.WriteString(styleMuted.Render(fmt.Sprintf("  evts %d", m.stats.Disk.EventFiles)) + "\n")
	}

	content := b.String()
	return styleInnerBorder.
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(true).
		Width(sidebarWidth).
		Height(height).
		Padding(0, 1).
		Render(content)
}

func (m Model) renderMain() string {
	title := "Logs · " + m.focus.String()
	if m.focus == PanelStats {
		title = "Stats"
		if !m.stats.At.IsZero() {
			title += " · " + m.stats.At.Format("15:04:05")
		}
	}
	header := stylePanelTitle.Render(title)
	body := m.viewport.View()
	inner := lipgloss.JoinVertical(lipgloss.Left, header, body)

	innerW := maxInt(20, m.width-2-sidebarWidth-1)
	h := bodyHeight(m.height)
	return lipgloss.NewStyle().
		Width(innerW).
		Height(h).
		Padding(0, 1).
		Render(inner)
}

func (m Model) renderFooter(width int) string {
	status := m.status
	if status == "" {
		status = "ready"
	}
	hints := strings.Join([]string{
		kbd("↑↓") + styleMuted.Render(" select"),
		kbd("jk") + styleMuted.Render(" scroll"),
		kbd("a") + styleMuted.Render(" all"),
		kbd("s") + styleMuted.Render(" restart"),
		kbd("x") + styleMuted.Render(" stop"),
		kbd("r") + styleMuted.Render(" refresh"),
		kbd("q") + styleMuted.Render(" quit"),
	}, styleMuted.Render(" · "))

	left := styleMuted.Render(status)
	gap := width - lipgloss.Width(left) - lipgloss.Width(hints) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + hints
	sep := styleSubtle.Render(strings.Repeat("─", maxInt(1, width-2)))
	return sep + "\n" + line
}

func (m Model) panelState(p PanelID) string {
	if sid, ok := p.serviceID(); ok {
		st, _ := m.services[sid].SnapshotState()
		return string(st)
	}
	if m.statsPending {
		return "starting"
	}
	if !m.stats.At.IsZero() {
		return "live"
	}
	return "idle"
}

func shortURL(u string) string {
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	return u
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
