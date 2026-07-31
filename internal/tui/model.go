package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

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
	sp.Style = styleFg

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

func (m *Model) selectPanel(p PanelID) {
	m.focus = p
	m.syncViewport()
}

func (m *Model) moveFocus(delta int) {
	n := int(panelCount)
	m.focus = PanelID((int(m.focus) + delta + n) % n)
	m.syncViewport()
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
			m.selectPanel(PanelRedpanda)
		case "2":
			m.selectPanel(PanelAPI)
		case "3":
			m.selectPanel(PanelWeb)
		case "4":
			m.selectPanel(PanelStats)
		case "tab":
			m.moveFocus(1)
		case "shift+tab":
			m.moveFocus(-1)
		case "up":
			m.moveFocus(-1)
		case "down":
			m.moveFocus(1)
		case "k":
			m.viewport.LineUp(1)
		case "j":
			m.viewport.LineDown(1)
		case "pgup", "b":
			m.viewport.HalfViewUp()
		case "pgdown", "f", " ":
			m.viewport.HalfViewDown()
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
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
		case "r":
			if !m.statsPending {
				m.statsPending = true
				m.status = "refreshing stats…"
				return m, m.collectStatsCmd()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vw, vh := m.viewportSize()
		m.viewport = viewport.New(vw, vh)
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
			m.viewport.SetContent(styleMuted.Render("collecting stats…"))
		} else {
			m.viewport.SetContent(formatStats(m.stats, m.cfg, m.viewport.Width))
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
	return m.renderFrame()
}
