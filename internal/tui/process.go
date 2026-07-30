package tui

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxLogLines = 500

type ProcState string

const (
	StateIdle     ProcState = "idle"
	StateStarting ProcState = "starting"
	StateRunning  ProcState = "running"
	StateStopped  ProcState = "stopped"
	StateFailed   ProcState = "failed"
)

type ServiceID int

const (
	SvcRedpanda ServiceID = iota
	SvcAPI
	SvcWeb
)

func (s ServiceID) String() string {
	switch s {
	case SvcRedpanda:
		return "redpanda"
	case SvcAPI:
		return "api"
	case SvcWeb:
		return "web"
	default:
		return "unknown"
	}
}

type logLine struct {
	t   time.Time
	msg string
}

type Service struct {
	ID      ServiceID
	Title   string
	State   ProcState
	Err     string
	cmd     *exec.Cmd
	logs    []logLine
	mu      sync.Mutex
	onLog   func(ServiceID)
	onExit  func(ServiceID, error)
}

func NewService(id ServiceID, title string) *Service {
	return &Service{ID: id, Title: title, State: StateIdle}
}

func (s *Service) Append(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg = strings.TrimRight(msg, "\r\n")
	if msg == "" {
		return
	}
	s.logs = append(s.logs, logLine{t: time.Now(), msg: msg})
	if len(s.logs) > maxLogLines {
		s.logs = s.logs[len(s.logs)-maxLogLines:]
	}
	if s.onLog != nil {
		s.onLog(s.ID)
	}
}

func (s *Service) LogText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, l := range s.logs {
		b.WriteString(l.t.Format("15:04:05"))
		b.WriteString(" ")
		b.WriteString(l.msg)
		b.WriteByte('\n')
	}
	return b.String()
}

func (s *Service) setState(st ProcState, errMsg string) {
	s.mu.Lock()
	s.State = st
	s.Err = errMsg
	s.mu.Unlock()
}

func (s *Service) SnapshotState() (ProcState, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.Err
}

// PIDs returns the root process pid and process-group id (0 if not running).
func (s *Service) PIDs() (pid, pgid int) {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return 0, 0
	}
	pid = cmd.Process.Pid
	if g, err := syscall.Getpgid(pid); err == nil {
		pgid = g
	}
	return pid, pgid
}

func (s *Service) pipe(r io.Reader) {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		s.Append(sc.Text())
	}
}

// StartDockerUp brings Redpanda up detached, then tails compose logs.
func (s *Service) StartDockerUp(cfg Config) error {
	s.Stop()
	s.setState(StateStarting, "")
	s.Append("$ docker compose -f " + cfg.RedpandaCompose + " up -d")

	up := exec.Command("docker", "compose", "-f", cfg.RedpandaCompose, "up", "-d")
	up.Dir = cfg.Root
	out, err := up.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			s.Append(line)
		}
	}
	if err != nil {
		s.setState(StateFailed, err.Error())
		if s.onExit != nil {
			s.onExit(s.ID, err)
		}
		return err
	}

	s.Append("$ docker compose -f " + cfg.RedpandaCompose + " logs -f --tail=50")
	cmd := exec.Command("docker", "compose", "-f", cfg.RedpandaCompose, "logs", "-f", "--tail", "50")
	cmd.Dir = cfg.Root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	if err := cmd.Start(); err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	s.cmd = cmd
	s.setState(StateRunning, "")
	go s.pipe(stdout)
	go s.pipe(stderr)
	go func() {
		err := cmd.Wait()
		s.cmd = nil
		if err != nil {
			s.setState(StateStopped, err.Error())
		} else {
			s.setState(StateStopped, "")
		}
		if s.onExit != nil {
			s.onExit(s.ID, err)
		}
	}()
	return nil
}

func (s *Service) StartCmd(dir string, argv []string, extraEnv []string) error {
	s.Stop()
	s.setState(StateStarting, "")
	s.Append("$ " + strings.Join(argv, " "))

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	if err := cmd.Start(); err != nil {
		s.setState(StateFailed, err.Error())
		return err
	}
	s.cmd = cmd
	s.setState(StateRunning, "")
	go s.pipe(stdout)
	go s.pipe(stderr)
	go func() {
		err := cmd.Wait()
		s.cmd = nil
		msg := ""
		if err != nil {
			msg = err.Error()
			s.setState(StateFailed, msg)
		} else {
			s.setState(StateStopped, "")
		}
		if s.onExit != nil {
			s.onExit(s.ID, err)
		}
	}()
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	s.cmd = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	s.Append("stopping…")
	// Kill process group so child processes (go run, vite) die too
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
	}
	s.setState(StateStopped, "")
}

func StopRedpanda(cfg Config) {
	cmd := exec.Command("docker", "compose", "-f", cfg.RedpandaCompose, "stop")
	cmd.Dir = cfg.Root
	_ = cmd.Run()
}
