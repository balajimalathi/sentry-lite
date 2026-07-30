package load

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ResourceSnapshot is host / process / storage usage for the load TUI.
type ResourceSnapshot struct {
	At time.Time

	// API process (sentry-lite)
	APIRSS   uint64
	APICPU   float64
	APIPID   int
	APIFound bool

	// Host
	HostTotalRAM uint64
	HostUsedRAM  uint64

	// Redpanda (docker)
	RedpandaMem    string // raw "123MiB / 2GiB"
	RedpandaMemPct string
	RedpandaCPU    string
	RedpandaOK     bool

	// Storage
	DataBytes   uint64
	EventsBytes uint64
	SQLiteBytes uint64
	DiskFree    uint64
	EventFiles  int
}

type HealthSnapshot struct {
	At          time.Time
	OK          bool
	Latency     time.Duration
	Err         string
	WasHealthy  bool
	Crashed     bool
	DataBytes   uint64
	SQLiteBytes uint64
	EventFiles  int
	Resource    ResourceSnapshot
	FullDisk    bool
}

// RunBaselines tracks growth from run start for RAM / disk.
type RunBaselines struct {
	Set         bool
	APIRSS      uint64
	DataBytes   uint64
	EventsBytes uint64
	SQLiteBytes uint64
	EventFiles  int
	PeakAPIRSS  uint64
	PeakData    uint64
	PeakEvents  uint64
}

func (b *RunBaselines) Observe(r ResourceSnapshot) {
	if !b.Set {
		b.Set = true
		b.APIRSS = r.APIRSS
		b.DataBytes = r.DataBytes
		b.EventsBytes = r.EventsBytes
		b.SQLiteBytes = r.SQLiteBytes
		b.EventFiles = r.EventFiles
		b.PeakAPIRSS = r.APIRSS
		b.PeakData = r.DataBytes
		b.PeakEvents = r.EventsBytes
		return
	}
	if r.APIRSS > b.PeakAPIRSS {
		b.PeakAPIRSS = r.APIRSS
	}
	if r.DataBytes > b.PeakData {
		b.PeakData = r.DataBytes
	}
	if r.EventsBytes > b.PeakEvents {
		b.PeakEvents = r.EventsBytes
	}
}

func CollectResources(dataDir, composeFile string) ResourceSnapshot {
	s := ResourceSnapshot{At: time.Now()}
	s.HostTotalRAM = hostTotalRAM()
	s.HostUsedRAM = hostUsedRAM()
	s.APIRSS, s.APICPU, s.APIPID, s.APIFound = findAPIProcess()
	s.RedpandaMem, s.RedpandaMemPct, s.RedpandaCPU, s.RedpandaOK = redpandaStats(composeFile)
	s.DataBytes, s.EventsBytes, s.SQLiteBytes, s.DiskFree, s.EventFiles = collectDiskDetailed(dataDir)
	return s
}

// CollectResourcesLight skips full directory walks (use for high-frequency polls).
func CollectResourcesLight(composeFile string) ResourceSnapshot {
	s := ResourceSnapshot{At: time.Now()}
	s.HostTotalRAM = hostTotalRAM()
	s.HostUsedRAM = hostUsedRAM()
	s.APIRSS, s.APICPU, s.APIPID, s.APIFound = findAPIProcess()
	s.RedpandaMem, s.RedpandaMemPct, s.RedpandaCPU, s.RedpandaOK = redpandaStats(composeFile)
	return s
}

func (s *ResourceSnapshot) MergeDisk(full ResourceSnapshot) {
	s.DataBytes = full.DataBytes
	s.EventsBytes = full.EventsBytes
	s.SQLiteBytes = full.SQLiteBytes
	s.DiskFree = full.DiskFree
	s.EventFiles = full.EventFiles
}

func CollectDisk(dataDir string) (dataBytes, sqliteBytes uint64, eventFiles int) {
	data, _, sqlite, _, files := collectDiskDetailed(dataDir)
	return data, sqlite, files
}

func collectDiskDetailed(dataDir string) (dataBytes, eventsBytes, sqliteBytes, diskFree uint64, eventFiles int) {
	dataDir = filepath.Clean(dataDir)
	eventsDir := filepath.Join(dataDir, "events")

	dataBytes, _ = dirSize(dataDir)
	eventsBytes, _ = dirSize(eventsDir)
	eventFiles = countJSONFiles(eventsDir)

	for _, name := range []string{"sentry-lite.db", "sentry-lite.db-shm", "sentry-lite.db-wal"} {
		if fi, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
			sqliteBytes += uint64(fi.Size())
		}
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &st); err == nil {
		diskFree = uint64(st.Bavail) * uint64(st.Bsize)
	} else if err := syscall.Statfs(".", &st); err == nil {
		diskFree = uint64(st.Bavail) * uint64(st.Bsize)
	}
	return
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

func countJSONFiles(path string) int {
	n := 0
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(info.Name()) == ".json" {
			n++
		}
		return nil
	})
	return n
}

func findAPIProcess() (rss uint64, cpu float64, pid int, found bool) {
	out, err := exec.Command("ps", "-axo", "pid=,rss=,pcpu=,command=").Output()
	if err != nil {
		return 0, 0, 0, false
	}
	var bestRSS uint64
	var bestCPU float64
	var bestPID int
	matched := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		cmd := strings.Join(fields[3:], " ")
		if !isAPICommand(cmd) {
			continue
		}
		p, _ := strconv.Atoi(fields[0])
		r, _ := strconv.ParseUint(fields[1], 10, 64)
		c, _ := strconv.ParseFloat(fields[2], 64)
		rssBytes := r * 1024
		if !matched || rssBytes > bestRSS {
			matched = true
			bestRSS = rssBytes
			bestCPU = c
			bestPID = p
		}
	}
	return bestRSS, bestCPU, bestPID, matched
}

func isAPICommand(cmd string) bool {
	lower := strings.ToLower(cmd)
	if strings.Contains(lower, "sentry-lite-load") {
		return false
	}
	if strings.Contains(lower, "sentry-lite-tui") {
		return false
	}
	return strings.Contains(lower, "sentry-lite") ||
		strings.Contains(cmd, "./cmd/sentry-lite") ||
		strings.Contains(cmd, "/cmd/sentry-lite")
}

func redpandaStats(composeFile string) (mem, memPct, cpu string, ok bool) {
	id := resolveRedpandaContainerID(composeFile)
	if id == "" {
		return "—", "—", "—", false
	}
	format := "{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"
	out, err := exec.Command("docker", "stats", "--no-stream", "--format", format, id).Output()
	if err != nil {
		return "—", "—", "—", false
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) < 3 {
		return "—", "—", "—", false
	}
	cpu = parts[0]
	mem = parts[1]
	if i := strings.Index(mem, " /"); i > 0 {
		mem = strings.TrimSpace(mem[:i])
	}
	memPct = parts[2]
	return mem, memPct, cpu, true
}

func resolveRedpandaContainerID(composeFile string) string {
	// Prefer compose-project container (same as process-manager TUI).
	if composeFile == "" {
		composeFile = "docker-compose.redpanda.yml"
	}
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "-q", "redpanda")
	if root := findRepoRoot(); root != "" && root != "." {
		cmd.Dir = root
	}
	if out, err := cmd.Output(); err == nil {
		if id := firstLine(string(out)); id != "" {
			return id
		}
	}
	// Prefer sentry-lite project name over other stacks (e.g. auth-redpanda).
	if id := firstLine(execOutput("docker", "ps", "-q", "--filter", "name=sentry-lite-redpanda")); id != "" {
		return id
	}
	// Last resort: any redpanda image, take the first ID only.
	out := execOutput("docker", "ps", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}")
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		name, image := strings.ToLower(parts[1]), strings.ToLower(parts[2])
		if strings.Contains(image, "redpanda") || strings.Contains(name, "redpanda") {
			return parts[0]
		}
	}
	return ""
}

func execOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
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

func hostUsedRAM() uint64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("vm_stat").Output()
		if err != nil {
			return 0
		}
		pageSize := uint64(4096)
		if psOut, err := exec.Command("sysctl", "-n", "hw.pagesize").Output(); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(psOut)), 10, 64); err == nil {
				pageSize = v
			}
		}
		var pagesFree, pagesInactive, pagesSpeculative uint64
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			parse := func(prefix string) uint64 {
				if !strings.HasPrefix(line, prefix) {
					return 0
				}
				rest := strings.TrimPrefix(line, prefix)
				rest = strings.Trim(rest, " .")
				v, _ := strconv.ParseUint(rest, 10, 64)
				return v
			}
			if v := parse("Pages free:"); v > 0 {
				pagesFree = v
			}
			if v := parse("Pages inactive:"); v > 0 {
				pagesInactive = v
			}
			if v := parse("Pages speculative:"); v > 0 {
				pagesSpeculative = v
			}
		}
		total := hostTotalRAM()
		free := (pagesFree + pagesInactive + pagesSpeculative) * pageSize
		if total > free {
			return total - free
		}
		return 0
	case "linux":
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		var total, avail uint64
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			kb, _ := strconv.ParseUint(fields[1], 10, 64)
			switch fields[0] {
			case "MemTotal:":
				total = kb * 1024
			case "MemAvailable:":
				avail = kb * 1024
			}
		}
		if total > avail {
			return total - avail
		}
	}
	return 0
}

func fmtDelta(now, base uint64) string {
	if now >= base {
		return "+" + fmtBytes(now-base)
	}
	return "-" + fmtBytes(base-now)
}

func fmtRate(bytes uint64, elapsed time.Duration) string {
	if elapsed <= 0 {
		return "—"
	}
	bps := float64(bytes) / elapsed.Seconds()
	return fmtBytes(uint64(bps)) + "/s"
}
