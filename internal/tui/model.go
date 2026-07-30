package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 18

type PanelID int

const (
	PanelRedpanda PanelID = iota
	PanelAPI
	PanelWeb
	PanelStats
	panelCount
)

func (p PanelID) String() string {
	switch p {
	case PanelRedpanda:
		return "redpanda"
	case PanelAPI:
		return "api"
	case PanelWeb:
		return "web"
	case PanelStats:
		return "stats"
	default:
		return "unknown"
	}
}

func (p PanelID) serviceID() (ServiceID, bool) {
	switch p {
	case PanelRedpanda:
		return SvcRedpanda, true
	case PanelAPI:
		return SvcAPI, true
	case PanelWeb:
		return SvcWeb, true
	default:
		return 0, false
	}
}

type Model struct {
	cfg          Config
	width        int
	height       int
	focus        PanelID
	prevFocus    PanelID
	services     map[ServiceID]*Service
	viewport     viewport.Model
	spinner      spinner.Model
	ready        bool
	quitting     bool
	status       string
	stats        StatsSnapshot
	statsPending bool
	statsAt      time.Time
}

type startAPIMsg struct{}
type startWebMsg struct{}
type startRedpandaMsg struct{}
type tickMsg time.Time
type shutdownDoneMsg struct{}
type statsMsg StatsSnapshot

func New(cfg Config) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))

	svcs := map[ServiceID]*Service{
		SvcRedpanda: NewService(SvcRedpanda, "redpanda"),
		SvcAPI:      NewService(SvcAPI, "api"),
		SvcWeb:      NewService(SvcWeb, "web"),
	}
	m := Model{
		cfg:      cfg,
		focus:    PanelAPI,
		services: svcs,
		spinner:  sp,
		status:   "starting stack…",
	}
	for _, s := range svcs {
		svc := s
		svc.onLog = func(id ServiceID) {
			_ = id
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startAll(),
		m.collectStatsCmd(),
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) }),
	)
}

func (m Model) startAll() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return startRedpandaMsg{} },
		tea.Tick(800*time.Millisecond, func(time.Time) tea.Msg { return startWebMsg{} }),
		tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return startAPIMsg{} }),
	)
}

func (m Model) collectStatsCmd() tea.Cmd {
	cfg := m.cfg
	svcs := m.services
	return func() tea.Msg {
		return statsMsg(collectStats(cfg, svcs))
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.status = "shutting down…"
			return m, m.shutdown()
		case "1":
			m.focus = PanelRedpanda
			m.syncViewport()
		case "2":
			m.focus = PanelAPI
			m.syncViewport()
		case "3":
			m.focus = PanelWeb
			m.syncViewport()
		case "4":
			m.focus = PanelStats
			m.syncViewport()
		case "tab":
			m.focus = PanelID((int(m.focus) + 1) % int(panelCount))
			m.syncViewport()
		case "shift+tab":
			m.focus = PanelID((int(m.focus) + int(panelCount) - 1) % int(panelCount))
			m.syncViewport()
		case "a":
			m.status = "restarting all…"
			return m, m.restartAll()
		case "s":
			if sid, ok := m.focus.serviceID(); ok {
				m.status = "restarting " + sid.String()
				return m, m.restartFocused()
			}
		case "x":
			if sid, ok := m.focus.serviceID(); ok {
				m.status = "stopped " + sid.String()
				m.services[sid].Stop()
				if sid == SvcRedpanda {
					StopRedpanda(m.cfg)
					m.services[SvcRedpanda].Append("docker compose stop issued")
					m.services[SvcRedpanda].setState(StateStopped, "")
				}
				m.syncViewport()
			}
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "pgup", "b":
			m.viewport.HalfViewUp()
		case "pgdown", "f", " ":
			m.viewport.HalfViewDown()
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "r":
			if m.focus == PanelStats && !m.statsPending {
				m.statsPending = true
				m.status = "refreshing stats…"
				return m, m.collectStatsCmd()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentW := maxInt(20, msg.Width-sidebarWidth-3)
		contentH := maxInt(5, msg.Height-3)
		m.viewport = viewport.New(contentW, contentH)
		m.ready = true
		m.syncViewport()

	case startRedpandaMsg:
		svc := m.services[SvcRedpanda]
		err := svc.StartDockerUp(m.cfg)
		m.status = "redpanda up"
		if err != nil {
			m.status = "redpanda failed: " + err.Error()
		}
		m.syncViewport()
		return m, nil

	case startAPIMsg:
		svc := m.services[SvcAPI]
		err := svc.StartCmd(m.cfg.Root, m.cfg.GoAPICmd, nil)
		if err != nil {
			m.status = "api failed: " + err.Error()
		} else {
			m.status = "api starting"
		}
		m.syncViewport()
		return m, nil

	case startWebMsg:
		svc := m.services[SvcWeb]
		err := svc.StartCmd(m.cfg.WebDir, m.cfg.WebCmd, nil)
		if err != nil {
			m.status = "web failed: " + err.Error()
		} else {
			m.status = "web starting"
		}
		m.syncViewport()
		return m, nil

	case tickMsg:
		m.syncViewport()
		if m.quitting {
			return m, tea.Quit
		}
		cmds := []tea.Cmd{
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) }),
		}
		if !m.statsPending && time.Since(m.statsAt) >= time.Second {
			m.statsPending = true
			cmds = append(cmds, m.collectStatsCmd())
		}
		return m, tea.Batch(cmds...)

	case statsMsg:
		m.stats = StatsSnapshot(msg)
		m.statsPending = false
		m.statsAt = time.Now()
		if m.status == "refreshing stats…" {
			m.status = "stats updated"
		}
		if m.focus == PanelStats {
			m.syncViewport()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case shutdownDoneMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) shutdown() tea.Cmd {
	svcs := m.services
	return func() tea.Msg {
		svcs[SvcWeb].Stop()
		svcs[SvcAPI].Stop()
		svcs[SvcRedpanda].Stop()
		return shutdownDoneMsg{}
	}
}

func (m Model) restartAll() tea.Cmd {
	m.services[SvcWeb].Stop()
	m.services[SvcAPI].Stop()
	m.services[SvcRedpanda].Stop()
	return m.startAll()
}

func (m Model) restartFocused() tea.Cmd {
	sid, ok := m.focus.serviceID()
	if !ok {
		return nil
	}
	cfg := m.cfg
	svc := m.services[sid]
	return func() tea.Msg {
		svc.Stop()
		switch sid {
		case SvcRedpanda:
			StopRedpanda(cfg)
			return startRedpandaMsg{}
		case SvcAPI:
			return startAPIMsg{}
		case SvcWeb:
			return startWebMsg{}
		}
		return nil
	}
}

func (m *Model) syncViewport() {
	if !m.ready {
		return
	}
	focusChanged := m.focus != m.prevFocus
	m.prevFocus = m.focus
	atBottom := m.viewport.AtBottom()

	if m.focus == PanelStats {
		if m.stats.At.IsZero() {
			m.viewport.SetContent("collecting stats…")
		} else {
			m.viewport.SetContent(formatStats(m.stats, m.cfg))
		}
		if focusChanged {
			m.viewport.GotoTop()
		}
		return
	}
	sid, _ := m.focus.serviceID()
	svc := m.services[sid]
	m.viewport.SetContent(svc.LogText())
	st, _ := svc.SnapshotState()
	if st == StateRunning && (focusChanged || atBottom) {
		m.viewport.GotoBottom()
	}
}

func (m Model) View() string {
	if m.quitting {
		return "\n  shutting down…\n"
	}
	if !m.ready {
		return fmt.Sprintf("\n  %s starting sentry-lite dev stack…\n", m.spinner.View())
	}

	sidebar := m.renderSidebar()
	contentH := maxInt(5, m.height-3)
	content := contentStyle.Width(maxInt(20, m.width-sidebarWidth-3)).Height(contentH).Render(m.viewport.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	footer := mutedStyle.Render(m.status) + "\n" +
		mutedStyle.Render("1–4 select · tab · a all · s restart · x stop · r refresh stats · q quit")

	return titleStyle.Render("sentry-lite") + "  " + mutedStyle.Render("dev") + "\n" +
		body + "\n" + footer
}

func (m Model) renderSidebar() string {
	var b strings.Builder
	b.WriteString(sidebarTitleStyle.Render("services") + "\n")

	panels := []PanelID{PanelRedpanda, PanelAPI, PanelWeb, PanelStats}
	for i, p := range panels {
		label := fmt.Sprintf("%d %s", i+1, p.String())
		state := ""
		if sid, ok := p.serviceID(); ok {
			st, _ := m.services[sid].SnapshotState()
			state = string(st)
		} else if m.statsPending {
			state = "…"
		} else if !m.stats.At.IsZero() {
			state = "live"
		} else {
			state = "idle"
		}

		line := label
		style := sideItemStyle
		if p == m.focus {
			style = sideItemActiveStyle
			switch {
			case state == "running" || state == "live":
				style = sideItemActiveOKStyle
			case state == "failed":
				style = sideItemActiveBadStyle
			}
		} else {
			switch {
			case state == "running" || state == "live":
				style = sideItemOKStyle
			case state == "failed":
				style = sideItemBadStyle
			}
		}
		b.WriteString(style.Render(line) + "\n")
		b.WriteString(sideStateStyle.Render("  "+state) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(sideMetaStyle.Render("api") + "\n")
	b.WriteString(sideMetaStyle.Render(truncate(m.cfg.APIURL, sidebarWidth-2)) + "\n")
	b.WriteString(sideMetaStyle.Render("web :5173") + "\n")

	if m.focus != PanelStats {
		rss := uint64(0)
		switch m.focus {
		case PanelAPI:
			rss = m.stats.API.RSSBytes
		case PanelWeb:
			rss = m.stats.Web.RSSBytes
		case PanelRedpanda:
			// mem shown in stats panel; sidebar hint only
		}
		if rss > 0 {
			b.WriteString("\n")
			b.WriteString(sideMetaStyle.Render("rss "+formatBytes(rss)) + "\n")
		}
	} else if m.stats.API.RSSBytes+m.stats.Web.RSSBytes > 0 {
		b.WriteString("\n")
		b.WriteString(sideMetaStyle.Render("rss "+formatBytes(m.stats.API.RSSBytes+m.stats.Web.RSSBytes)) + "\n")
	}
	if m.stats.Disk.DataDir > 0 {
		b.WriteString(sideMetaStyle.Render("data "+formatBytes(m.stats.Disk.DataDir)) + "\n")
	}

	h := maxInt(5, m.height-3)
	return sidebarStyle.Width(sidebarWidth).Height(h).Render(b.String())
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

var (
	titleStyle             = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	mutedStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sidebarStyle           = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("238"))
	sidebarTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245")).Padding(0, 0, 1, 0)
	sideItemStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sideItemActiveStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238"))
	sideItemOKStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	sideItemActiveOKStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("28"))
	sideItemBadStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	sideItemActiveBadStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160"))
	sideStateStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sideMetaStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	contentStyle           = lipgloss.NewStyle().Padding(0, 1)
)

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
