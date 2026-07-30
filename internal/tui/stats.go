package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ProcUsage struct {
	Running  bool
	PID      int
	PGID     int
	Procs    int
	RSSBytes uint64
	CPUPct   float64
	Elapsed  string
	Comm     string
	Err      string
}

type DockerUsage struct {
	Running    bool
	Name       string
	CPUPct     string
	MemUsage   string
	MemPct     string
	NetIO      string
	BlockIO    string
	PIDs       string
	Err        string
}

type DiskUsage struct {
	DataDir    uint64
	EventsDir  uint64
	SQLite     uint64 // db + shm + wal
	EventFiles int
	DataFree   uint64 // free bytes on filesystem hosting data/
	Err        string
}

type HostUsage struct {
	TotalRAM uint64
	GOOS     string
	GOARCH   string
	NumCPU   int
}

type StatsSnapshot struct {
	At       time.Time
	API      ProcUsage
	Web      ProcUsage
	Redpanda DockerUsage
	Disk     DiskUsage
	Host     HostUsage
}

func collectStats(cfg Config, services map[ServiceID]*Service) StatsSnapshot {
	snap := StatsSnapshot{
		At: time.Now(),
		Host: HostUsage{
			GOOS:   runtime.GOOS,
			GOARCH: runtime.GOARCH,
			NumCPU: runtime.NumCPU(),
		},
	}
	snap.Host.TotalRAM = hostTotalRAM()
	snap.API = procUsage(services[SvcAPI])
	snap.Web = procUsage(services[SvcWeb])
	snap.Redpanda = dockerUsage(cfg)
	snap.Disk = diskUsage(cfg.Root)
	return snap
}

func procUsage(svc *Service) ProcUsage {
	if svc == nil {
		return ProcUsage{}
	}
	st, errMsg := svc.SnapshotState()
	pid, pgid := svc.PIDs()
	u := ProcUsage{
		Running: st == StateRunning || st == StateStarting,
		PID:     pid,
		PGID:    pgid,
		Err:     errMsg,
	}
	if pid <= 0 {
		return u
	}
	rows, err := psRows()
	if err != nil {
		u.Err = err.Error()
		return u
	}
	var rssKB uint64
	var cpu float64
	var elapsed, comm string
	count := 0
	for _, r := range rows {
		if r.pid == pid || (pgid > 0 && r.pgid == pgid) {
			count++
			rssKB += r.rssKB
			cpu += r.cpu
			if r.pid == pid {
				elapsed = r.elapsed
				comm = r.comm
			}
		}
	}
	u.Procs = count
	u.RSSBytes = rssKB * 1024
	u.CPUPct = cpu
	u.Elapsed = elapsed
	u.Comm = comm
	return u
}

type psRow struct {
	pid     int
	pgid    int
	rssKB   uint64
	cpu     float64
	elapsed string
	comm    string
}

func psRows() ([]psRow, error) {
	out, err := exec.Command("ps", "-axo", "pid=,pgid=,rss=,pcpu=,etime=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var rows []psRow
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		pgid, _ := strconv.Atoi(fields[1])
		rss, _ := strconv.ParseUint(fields[2], 10, 64)
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		rows = append(rows, psRow{
			pid:     pid,
			pgid:    pgid,
			rssKB:   rss,
			cpu:     cpu,
			elapsed: fields[4],
			comm:    fields[5],
		})
	}
	return rows, nil
}

func dockerUsage(cfg Config) DockerUsage {
	u := DockerUsage{}
	idCmd := exec.Command("docker", "compose", "-f", cfg.RedpandaCompose, "ps", "-q", "redpanda")
	idCmd.Dir = cfg.Root
	idOut, err := idCmd.Output()
	if err != nil {
		u.Err = "compose ps: " + err.Error()
		return u
	}
	id := strings.TrimSpace(string(idOut))
	if id == "" {
		u.Err = "container not running"
		return u
	}
	format := "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	out, err := exec.Command("docker", "stats", "--no-stream", "--format", format, id).Output()
	if err != nil {
		u.Err = "stats: " + err.Error()
		return u
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, "\t")
	if len(parts) < 7 {
		u.Err = "unexpected docker stats format"
		return u
	}
	u.Running = true
	u.Name = parts[0]
	u.CPUPct = parts[1]
	u.MemUsage = parts[2]
	u.MemPct = parts[3]
	u.NetIO = parts[4]
	u.BlockIO = parts[5]
	u.PIDs = parts[6]
	return u
}

func diskUsage(root string) DiskUsage {
	d := DiskUsage{}
	dataDir := filepath.Join(root, "data")
	eventsDir := filepath.Join(dataDir, "events")

	var err error
	d.DataDir, err = dirSize(dataDir)
	if err != nil && !os.IsNotExist(err) {
		d.Err = err.Error()
	}
	d.EventsDir, _ = dirSize(eventsDir)
	d.EventFiles = countFiles(eventsDir)

	for _, name := range []string{"sentry-lite.db", "sentry-lite.db-shm", "sentry-lite.db-wal"} {
		if fi, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
			d.SQLite += uint64(fi.Size())
		}
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &st); err == nil {
		d.DataFree = uint64(st.Bavail) * uint64(st.Bsize)
	} else if err := syscall.Statfs(root, &st); err == nil {
		d.DataFree = uint64(st.Bavail) * uint64(st.Bsize)
	}
	return d
}

func dirSize(path string) (uint64, error) {
	var total uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info != nil && !info.IsDir() {
			total += uint64(info.Size())
		}
		return nil
	})
	return total, err
}

func countFiles(path string) int {
	n := 0
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func hostTotalRAM() uint64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		return v
	case "linux":
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.ParseUint(fields[1], 10, 64)
					return kb * 1024
				}
			}
		}
	}
	return 0
}

func formatBytes(n uint64) string {
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

func formatStats(s StatsSnapshot, cfg Config) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("updated %s\n\n", s.At.Format("15:04:05")))

	b.WriteString("HOST\n")
	b.WriteString(fmt.Sprintf("  os          %s/%s  cpus %d\n", s.Host.GOOS, s.Host.GOARCH, s.Host.NumCPU))
	if s.Host.TotalRAM > 0 {
		b.WriteString(fmt.Sprintf("  total ram   %s\n", formatBytes(s.Host.TotalRAM)))
	}
	b.WriteString("\n")

	writeProc := func(title string, p ProcUsage) {
		b.WriteString(strings.ToUpper(title) + "\n")
		if !p.Running && p.PID == 0 {
			b.WriteString("  status      stopped\n\n")
			return
		}
		status := "running"
		if !p.Running {
			status = "idle"
		}
		b.WriteString(fmt.Sprintf("  status      %s\n", status))
		b.WriteString(fmt.Sprintf("  pid/pgid    %d / %d\n", p.PID, p.PGID))
		b.WriteString(fmt.Sprintf("  procs       %d  (process group)\n", p.Procs))
		b.WriteString(fmt.Sprintf("  rss         %s\n", formatBytes(p.RSSBytes)))
		b.WriteString(fmt.Sprintf("  cpu         %.1f%%\n", p.CPUPct))
		if p.Elapsed != "" {
			b.WriteString(fmt.Sprintf("  uptime      %s\n", p.Elapsed))
		}
		if p.Comm != "" {
			b.WriteString(fmt.Sprintf("  command     %s\n", p.Comm))
		}
		if p.Err != "" {
			b.WriteString(fmt.Sprintf("  note        %s\n", p.Err))
		}
		b.WriteString("\n")
	}
	writeProc("api", s.API)
	writeProc("web", s.Web)

	b.WriteString("REDPANDA (docker)\n")
	if !s.Redpanda.Running {
		msg := "not running"
		if s.Redpanda.Err != "" {
			msg = s.Redpanda.Err
		}
		b.WriteString(fmt.Sprintf("  status      %s\n\n", msg))
	} else {
		b.WriteString(fmt.Sprintf("  name        %s\n", s.Redpanda.Name))
		b.WriteString(fmt.Sprintf("  cpu         %s\n", s.Redpanda.CPUPct))
		b.WriteString(fmt.Sprintf("  memory      %s  (%s)\n", s.Redpanda.MemUsage, s.Redpanda.MemPct))
		b.WriteString(fmt.Sprintf("  net i/o     %s\n", s.Redpanda.NetIO))
		b.WriteString(fmt.Sprintf("  block i/o   %s\n", s.Redpanda.BlockIO))
		b.WriteString(fmt.Sprintf("  pids        %s\n\n", s.Redpanda.PIDs))
	}

	b.WriteString("STORAGE\n")
	b.WriteString(fmt.Sprintf("  data/       %s\n", formatBytes(s.Disk.DataDir)))
	b.WriteString(fmt.Sprintf("  events/     %s  (%d files)\n", formatBytes(s.Disk.EventsDir), s.Disk.EventFiles))
	b.WriteString(fmt.Sprintf("  sqlite      %s  (db+shm+wal)\n", formatBytes(s.Disk.SQLite)))
	if s.Disk.DataFree > 0 {
		b.WriteString(fmt.Sprintf("  disk free   %s  (volume for data/)\n", formatBytes(s.Disk.DataFree)))
	}
	b.WriteString(fmt.Sprintf("  root        %s\n", cfg.Root))
	if s.Disk.Err != "" {
		b.WriteString(fmt.Sprintf("  note        %s\n", s.Disk.Err))
	}

	totalRSS := s.API.RSSBytes + s.Web.RSSBytes
	b.WriteString("\nSUMMARY\n")
	b.WriteString(fmt.Sprintf("  api+web rss %s\n", formatBytes(totalRSS)))
	b.WriteString(fmt.Sprintf("  data total  %s\n", formatBytes(s.Disk.DataDir)))
	return b.String()
}
