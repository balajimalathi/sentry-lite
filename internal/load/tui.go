package load

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenSetup screen = iota
	screenRun
	screenConfirm
)

type field int

const (
	fieldMode field = iota
	fieldTotal
	fieldWorkers
	fieldRPS
	fieldPeakRPS
	fieldBurstSec
	fieldIdleSec
	fieldIdleRPS
	fieldDSN
	fieldCount
)

type tickMsg time.Time
type healthMsg HealthSnapshot
type runDoneMsg struct{}

type Model struct {
	cfg        Config
	runner     *Runner
	screen     screen
	focus      field
	editing    bool
	editBuf    string
	status     string
	quitting   bool
	width      int

	health     HealthSnapshot
	wasHealthy bool
	rate       RateTracker
	instantRPS float64
	baseline   RunBaselines
}

func NewModel(cfg Config) Model {
	return Model{
		cfg:    cfg,
		runner: NewRunner(cfg),
		screen: screenSetup,
		width:  100,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), healthCmd(m.cfg, true), resourceTickCmd())
}

func healthCmd(cfg Config, fullDisk bool) tea.Cmd {
	return func() tea.Msg {
		client := NewClient(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ok, lat, err := client.Health(ctx)
		var res ResourceSnapshot
		if fullDisk {
			res = CollectResources(cfg.DataDir, cfg.ComposePath())
		} else {
			res = CollectResourcesLight(cfg.ComposePath())
		}
		h := HealthSnapshot{
			At:          time.Now(),
			OK:          ok,
			Latency:     lat,
			DataBytes:   res.DataBytes,
			SQLiteBytes: res.SQLiteBytes,
			EventFiles:  res.EventFiles,
			Resource:    res,
			FullDisk:    fullDisk,
		}
		if err != nil {
			h.Err = err.Error()
		}
		return healthMsg(h)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func resourceTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return resourceTickMsg(t) })
}

type resourceTickMsg time.Time

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}
		switch m.screen {
		case screenSetup:
			return m.updateSetupKey(msg)
		case screenConfirm:
			return m.updateConfirmKey(msg)
		case screenRun:
			return m.updateRunKey(msg)
		}

	case tickMsg:
		if m.screen == screenRun {
			sent := m.runner.Counters().Sent.Load()
			m.instantRPS = m.rate.Update(sent)
		}
		return m, tea.Batch(tickCmd(), healthCmd(m.cfg, false))

	case resourceTickMsg:
		return m, tea.Batch(resourceTickCmd(), healthCmd(m.cfg, true))

	case healthMsg:
		h := HealthSnapshot(msg)
		h.WasHealthy = m.wasHealthy
		if m.wasHealthy && !h.OK {
			h.Crashed = true
		}
		if h.OK {
			m.wasHealthy = true
		}
		if !h.FullDisk {
			// keep previous disk numbers when light poll
			prev := m.health.Resource
			h.Resource.MergeDisk(prev)
			h.DataBytes = prev.DataBytes
			h.SQLiteBytes = prev.SQLiteBytes
			h.EventFiles = prev.EventFiles
		}
		m.health = h
		if m.screen == screenRun {
			st := m.runner.State()
			if st == StateRunning || st == StatePaused || st == StateCrashProbe || st == StateDone {
				m.baseline.Observe(h.Resource)
			}
		}
		return m, nil

	case runDoneMsg:
		m.status = "run finished"
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	}
	return m, nil
}

func (m Model) updateSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "tab", "down":
		if m.editing {
			m.commitEdit()
		}
		m.focus = (m.focus + 1) % fieldCount
		return m, nil
	case "shift+tab", "up":
		if m.editing {
			m.commitEdit()
		}
		m.focus = (m.focus + fieldCount - 1) % fieldCount
		return m, nil
	case "enter":
		if m.editing {
			m.commitEdit()
			return m, nil
		}
		if err := m.cfg.Validate(); err != nil {
			m.status = err.Error()
			return m, nil
		}
		if m.cfg.NeedsConfirm() {
			m.screen = screenConfirm
			return m, nil
		}
		return m.startRun()
	case " ":
		if m.focus == fieldMode {
			if m.cfg.Mode == ModeStress {
				m.cfg.Mode = ModePeak
			} else {
				m.cfg.Mode = ModeStress
			}
			return m, nil
		}
		m.editing = !m.editing
		if m.editing {
			m.editBuf = m.fieldValue(m.focus)
		} else {
			m.commitEdit()
		}
		return m, nil
	default:
		if m.editing {
			switch msg.String() {
			case "backspace":
				if len(m.editBuf) > 0 {
					m.editBuf = m.editBuf[:len(m.editBuf)-1]
				}
			case "esc":
				m.editing = false
				m.editBuf = ""
			default:
				if len(msg.Runes) > 0 {
					m.editBuf += string(msg.Runes)
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.cfg.Yes = true
		return m.startRun()
	case "n", "N", "q", "esc", "ctrl+c":
		m.screen = screenSetup
		m.status = "cancelled large run"
		return m, nil
	}
	return m, nil
}

func (m Model) updateRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.runner.Stop()
		m.quitting = true
		return m, tea.Quit
	case "s":
		st := m.runner.State()
		switch st {
		case StateRunning, StateCrashProbe:
			m.runner.Stop()
			m.status = "stopped"
		case StatePaused:
			m.runner.Resume()
			m.status = "resumed"
		case StateIdle, StateDone:
			m.runner = NewRunner(m.cfg)
			return m.startRun()
		}
		return m, nil
	case "p":
		if m.runner.State() == StateRunning {
			m.runner.Pause()
			m.status = "paused"
		} else if m.runner.State() == StatePaused {
			m.runner.Resume()
			m.status = "resumed"
		}
		return m, nil
	case "k":
		m.runner.Stop()
		time.Sleep(100 * time.Millisecond)
		_ = m.runner.StartCrashProbe()
		m.status = "crash probe — flooding API until it dies or you press s/q"
		return m, nil
	}
	return m, nil
}

func (m *Model) startRun() (Model, tea.Cmd) {
	m.runner = NewRunner(m.cfg)
	if err := m.runner.Start(); err != nil {
		m.status = err.Error()
		return *m, nil
	}
	m.screen = screenRun
	m.status = "running"
	m.wasHealthy = m.health.OK
	m.baseline = RunBaselines{}
	m.baseline.Observe(m.health.Resource)
	return *m, tickCmd()
}

func (m *Model) fieldValue(f field) string {
	switch f {
	case fieldMode:
		return string(m.cfg.Mode)
	case fieldTotal:
		return strconv.FormatInt(m.cfg.Total, 10)
	case fieldWorkers:
		return strconv.Itoa(m.cfg.Workers)
	case fieldRPS:
		return strconv.Itoa(m.cfg.RPS)
	case fieldPeakRPS:
		return strconv.Itoa(m.cfg.PeakRPS)
	case fieldBurstSec:
		return strconv.Itoa(m.cfg.BurstSec)
	case fieldIdleSec:
		return strconv.Itoa(m.cfg.IdleSec)
	case fieldIdleRPS:
		return strconv.Itoa(m.cfg.IdleRPS)
	case fieldDSN:
		return m.cfg.DSN
	default:
		return ""
	}
}

func (m *Model) commitEdit() {
	v := strings.TrimSpace(m.editBuf)
	switch m.focus {
	case fieldTotal:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			m.cfg.Total = n
		}
	case fieldWorkers:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.Workers = n
		}
	case fieldRPS:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.RPS = n
		}
	case fieldPeakRPS:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.PeakRPS = n
		}
	case fieldBurstSec:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.BurstSec = n
		}
	case fieldIdleSec:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.IdleSec = n
		}
	case fieldIdleRPS:
		if n, err := strconv.Atoi(v); err == nil {
			m.cfg.IdleRPS = n
		}
	case fieldDSN:
		if v != "" {
			_ = m.cfg.ApplyDSN(v)
		}
	}
	m.editing = false
	m.editBuf = ""
}

func (m Model) View() string {
	if m.quitting {
		return "Shutting down load test...\n"
	}
	switch m.screen {
	case screenConfirm:
		return m.viewConfirm()
	case screenRun:
		return m.viewRun()
	default:
		return m.viewSetup()
	}
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Confirm large run"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Send %s events to %s?\n\n", styleFg.Render(formatInt(m.cfg.Total)), m.cfg.BaseURL))
	b.WriteString(styleMuted.Render("This may fill disk under ./data. "))
	b.WriteString(kbd("y") + " confirm  " + kbd("n") + " cancel\n")
	return styleOuter.Padding(1, 2).Render(b.String())
}

func (m Model) viewSetup() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("sentry-lite load test"))
	b.WriteString("\n")
	b.WriteString(styleMuted.Render("Simulates playground traffic (errors, transactions, crons, releases) via ingest API.\n\n"))

	labels := []string{"Mode (space toggles)", "Total events", "Workers", "RPS (stress)", "Peak RPS", "Burst sec", "Idle sec", "Idle RPS", "DSN"}
	for i, label := range labels {
		val := m.fieldValue(field(i))
		if m.editing && m.focus == field(i) {
			val = m.editBuf + "▌"
		}
		line := label + ": "
		if field(i) == m.focus {
			line = lipgloss.NewStyle().Bold(true).Foreground(colSelectedFg).Background(colSelectedBg).Render("› "+line) + val
		} else {
			line += styleFg.Render(val)
		}
		b.WriteString(line + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + styleSubtle.Render(m.status) + "\n")
	}
	b.WriteString("\n" + m.viewHealthBrief())
	res := m.health.Resource
	if res.APIFound || res.DataBytes > 0 {
		b.WriteString("\n")
		b.WriteString(styleSubtle.Render(fmt.Sprintf("  api %s · data/ %s · free %s · files %d",
			fmtBytes(res.APIRSS), fmtBytes(res.DataBytes), fmtBytes(res.DiskFree), res.EventFiles)))
	}
	b.WriteString("\n\n")
	b.WriteString(styleMuted.Render("↑↓ field · space edit · enter start · q quit"))
	return styleOuter.Padding(1, 2).Render(b.String())
}

func (m Model) viewRun() string {
	c := m.runner.Counters()
	sent := c.Sent.Load()
	elapsed := time.Since(m.runner.StartedAt())
	if elapsed < 0 {
		elapsed = 0
	}
	inst := m.instantRPS
	avg := m.rate.Avg(sent, elapsed)
	p50, p95, p99 := c.Percentiles()
	w := m.width - 4
	if w < 72 {
		w = 72
	}

	var b strings.Builder
	b.WriteString(m.viewHeader(sent, elapsed))
	b.WriteString("\n\n")

	if m.health.Crashed {
		b.WriteString(styleBadgeBad.Render("API CRASH / UNREACHABLE") + "\n\n")
	}

	// Progress toward total
	var progressPct float64
	if m.cfg.Total > 0 {
		progressPct = float64(sent) / float64(m.cfg.Total)
	}
	b.WriteString(styleMuted.Render("progress     ") + progressBar(progressPct, maxInt(24, w-28)) + "\n")
	b.WriteString(styleSubtle.Render(fmt.Sprintf("             %s / %s events", formatInt(sent), formatInt(m.cfg.Total))) + "\n")

	cardW := maxInt(14, (w-8)/4)
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		miniCard("sent", formatInt(sent), fmt.Sprintf("ok %s", formatInt(c.OK.Load())), cardW),
		miniCard("rps", fmt.Sprintf("%.0f", inst), fmt.Sprintf("avg %.0f", avg), cardW),
		miniCard("latency", p50.Round(time.Millisecond).String(), fmt.Sprintf("p95 %s  p99 %s", p95.Round(time.Millisecond), p99.Round(time.Millisecond)), cardW),
		miniCard("errors", formatInt(c.Err5xx.Load()+c.Timeout.Load()+c.OtherErr.Load()), fmt.Sprintf("4xx %s  in-flight %s", formatInt(c.Err4xx.Load()), formatInt(c.InFlight.Load())), cardW),
	))
	b.WriteString("\n")

	b.WriteString("\n" + renderResourcePanel(m.health.Resource, m.baseline, elapsed, w))

	b.WriteString("\n" + styleSection.Render("By feature") + "\n")
	var featParts []string
	for i := 0; i < int(catCount); i++ {
		n := c.ByCat[i].Load()
		if n > 0 {
			featParts = append(featParts, styleChip.Render(fmt.Sprintf("%s %s", Category(i).String(), formatInt(n))))
		}
	}
	if len(featParts) > 0 {
		b.WriteString("  " + strings.Join(featParts, " ") + "\n")
	}

	apiLine := styleBadgeOK.Render("healthz ok")
	if !m.health.OK {
		apiLine = styleBadgeBad.Render("healthz fail")
		if m.health.Err != "" {
			apiLine += " " + styleSubtle.Render(m.health.Err)
		}
	} else {
		apiLine += styleMuted.Render(fmt.Sprintf("  %s", m.health.Latency.Round(time.Millisecond)))
	}
	b.WriteString("\n  " + apiLine + "\n")

	if m.status != "" {
		b.WriteString("\n" + styleSubtle.Render(m.status))
	}
	b.WriteString("\n\n")
	b.WriteString(styleMuted.Render(kbd("s") + " start/stop · " + kbd("p") + " pause · " + kbd("k") + " crash probe · " + kbd("q") + " quit"))
	return styleOuter.Padding(1, 2).Render(b.String())
}

func (m Model) viewHeader(sent int64, elapsed time.Duration) string {
	mode := string(m.cfg.Mode)
	if m.runner.State() == StateCrashProbe {
		mode = "crash-probe"
	}
	st := "running"
	switch m.runner.State() {
	case StatePaused:
		st = "paused"
	case StateDone:
		st = "done"
	case StateCrashProbe:
		st = "crash-probe"
	}
	apiBadge := styleBadgeOK.Render("api ok")
	if !m.health.OK {
		apiBadge = styleBadgeBad.Render("api down")
	}
	chips := []string{
		styleChip.Render("mode " + mode),
		styleChip.Render(fmt.Sprintf("%s/%s", formatInt(sent), formatInt(m.cfg.Total))),
		styleChip.Render("state " + st),
		apiBadge,
		styleChip.Render(elapsed.Round(time.Second).String()),
	}
	return styleTitle.Render("sentry-lite load") + "  " + strings.Join(chips, " ")
}

func (m Model) viewHealthBrief() string {
	if m.health.OK {
		return styleBadgeOK.Render("● api healthy") + styleMuted.Render(fmt.Sprintf("  healthz %s", m.health.Latency.Round(time.Millisecond)))
	}
	if m.health.Err != "" {
		return styleBadgeBad.Render("○ api unreachable") + " " + m.health.Err
	}
	return styleBadgeWarn.Render("○ api unknown")
}

func formatInt(n int64) string {
	if n < 0 {
		return "0"
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func RunHeadless(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.NeedsConfirm() {
		return fmt.Errorf("total %d > 100000: pass --yes to confirm", cfg.Total)
	}
	r := NewRunner(cfg)
	if err := r.Start(); err != nil {
		return err
	}

	var base RunBaselines
	base.Observe(CollectResources(cfg.DataDir, cfg.ComposePath()))

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Done():
			printHeadlessSummary(cfg, r, base)
			return nil
		case <-ticker.C:
			c := r.Counters()
			res := CollectResources(cfg.DataDir, cfg.ComposePath())
			base.Observe(res)
			rp := "—"
			if res.RedpandaOK {
				rp = res.RedpandaMem
			}
			fmt.Printf("\r sent=%d ok=%d 5xx=%d | api=%s (peak %s Δ%s) | rp=%s | data=%s (Δ%s) files=%d free=%s     ",
				c.Sent.Load(), c.OK.Load(), c.Err5xx.Load(),
				fmtBytes(res.APIRSS), fmtBytes(base.PeakAPIRSS), fmtDelta(res.APIRSS, base.APIRSS),
				rp,
				fmtBytes(res.DataBytes), fmtDelta(res.DataBytes, base.DataBytes),
				res.EventFiles, fmtBytes(res.DiskFree))
			if r.State() == StateDone {
				printHeadlessSummary(cfg, r, base)
				return nil
			}
		}
	}
}

func printHeadlessSummary(cfg Config, r *Runner, base RunBaselines) {
	c := r.Counters()
	p50, p95, p99 := c.Percentiles()
	res := CollectResources(cfg.DataDir, cfg.ComposePath())
	base.Observe(res)
	elapsed := time.Since(r.StartedAt())
	fmt.Printf("\n\n=== load summary ===\n")
	fmt.Printf("sent=%d ok=%d 4xx=%d 5xx=%d timeout=%d\n",
		c.Sent.Load(), c.OK.Load(), c.Err4xx.Load(), c.Err5xx.Load(), c.Timeout.Load())
	fmt.Printf("latency p50=%s p95=%s p99=%s  elapsed=%s\n",
		p50.Round(time.Millisecond), p95.Round(time.Millisecond), p99.Round(time.Millisecond),
		elapsed.Round(time.Second))
	fmt.Printf("\n--- RAM ---\n")
	fmt.Printf("api rss now=%s  start=%s  peak=%s  Δ=%s\n",
		fmtBytes(res.APIRSS), fmtBytes(base.APIRSS), fmtBytes(base.PeakAPIRSS), fmtDelta(res.APIRSS, base.APIRSS))
	if res.HostTotalRAM > 0 {
		fmt.Printf("host ram=%s / %s\n", fmtBytes(res.HostUsedRAM), fmtBytes(res.HostTotalRAM))
	}
	if res.RedpandaOK {
		fmt.Printf("redpanda mem=%s cpu=%s\n", res.RedpandaMem, res.RedpandaCPU)
	}
	fmt.Printf("\n--- Storage ---\n")
	fmt.Printf("data/ now=%s  start=%s  peak=%s  Δ=%s\n",
		fmtBytes(res.DataBytes), fmtBytes(base.DataBytes), fmtBytes(base.PeakData), fmtDelta(res.DataBytes, base.DataBytes))
	fmt.Printf("events/ now=%s  Δ=%s", fmtBytes(res.EventsBytes), fmtDelta(res.EventsBytes, base.EventsBytes))
	if elapsed > 0 && res.EventsBytes >= base.EventsBytes {
		fmt.Printf("  %s", fmtRate(res.EventsBytes-base.EventsBytes, elapsed))
	}
	fmt.Printf("\n")
	fmt.Printf("sqlite=%s (Δ %s)  event files=%d (Δ %+d)  disk free=%s\n",
		fmtBytes(res.SQLiteBytes), fmtDelta(res.SQLiteBytes, base.SQLiteBytes),
		res.EventFiles, res.EventFiles-base.EventFiles, fmtBytes(res.DiskFree))
}

func RunTUI(cfg Config) error {
	p := tea.NewProgram(NewModel(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
