package load

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultDSN = "http://fa41276d7b8b1b7e58c5aa350c965f04@localhost:8080/3"

type Mode string

const (
	ModeStress Mode = "stress"
	ModePeak   Mode = "peak"
)

type Category int

const (
	CatError Category = iota
	CatMessage
	CatContext
	CatFingerprint
	CatTransaction
	CatCheckoutFail
	CatRelease
	CatCron
	catCount
)

func (c Category) String() string {
	switch c {
	case CatError:
		return "errors"
	case CatMessage:
		return "messages"
	case CatContext:
		return "context"
	case CatFingerprint:
		return "fingerprint"
	case CatTransaction:
		return "transactions"
	case CatCheckoutFail:
		return "checkout_fail"
	case CatRelease:
		return "releases"
	case CatCron:
		return "crons"
	default:
		return "unknown"
	}
}

type Config struct {
	Mode      Mode
	DSN       string
	BaseURL   string
	PublicKey string
	ProjectID int64

	Total   int64
	Workers int
	RPS     int // stress sustained RPS

	PeakRPS   int // peak burst RPS
	BurstSec  int
	IdleSec   int
	IdleRPS   int // peak baseline RPS

	Mix [catCount]int

	Headless bool
	Yes      bool // skip large-run confirm
	DataDir  string
	Root     string // repo root for compose lookup
	RedpandaCompose string
}

func DefaultConfig() Config {
	root := findRepoRoot()
	cfg := Config{
		Mode:      ModeStress,
		DSN:       envDSN(),
		Total:     1_000_000,
		Workers:   200,
		RPS:       5000,
		PeakRPS:   10000,
		BurstSec:  30,
		IdleSec:   10,
		IdleRPS:   500,
		Mix:       defaultMix(),
		DataDir:   "./data",
		Root:      root,
		RedpandaCompose: "docker-compose.redpanda.yml",
	}
	if err := cfg.ApplyDSN(cfg.DSN); err != nil {
		cfg.DSN = DefaultDSN
		_ = cfg.ApplyDSN(cfg.DSN)
	}
	return cfg
}

func findRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.redpanda.yml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == "" || parent == dir {
			break
		}
		dir = parent
	}
	return wd
}

func defaultMix() [catCount]int {
	return [catCount]int{
		CatError:         20,
		CatMessage:       8,
		CatContext:       10,
		CatFingerprint:   5,
		CatTransaction:   30,
		CatCheckoutFail:  12,
		CatRelease:       10,
		CatCron:          5,
	}
}

func envDSN() string {
	for _, k := range []string{"SENTRY_DSN", "NEXT_PUBLIC_SENTRY_DSN", "LOAD_DSN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return DefaultDSN
}

func (c *Config) ApplyDSN(dsn string) error {
	base, key, pid, err := ParseDSN(dsn)
	if err != nil {
		return err
	}
	c.DSN = dsn
	c.BaseURL = base
	c.PublicKey = key
	c.ProjectID = pid
	return nil
}

func ParseDSN(dsn string) (baseURL, publicKey string, projectID int64, err error) {
	u, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil {
		return "", "", 0, err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", 0, fmt.Errorf("invalid dsn: missing scheme or host")
	}
	publicKey = u.User.Username()
	if publicKey == "" {
		return "", "", 0, fmt.Errorf("invalid dsn: missing public key")
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", "", 0, fmt.Errorf("invalid dsn: missing project id")
	}
	projectID, err = strconv.ParseInt(path, 10, 64)
	if err != nil || projectID <= 0 {
		return "", "", 0, fmt.Errorf("invalid dsn project id: %q", path)
	}
	baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	return baseURL, publicKey, projectID, nil
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.DSN, "dsn", c.DSN, "Sentry DSN (http://key@host/project)")
	fs.StringVar((*string)(&c.Mode), "mode", string(c.Mode), "stress or peak")
	fs.Int64Var(&c.Total, "total", c.Total, "max events to send")
	fs.IntVar(&c.Workers, "workers", c.Workers, "worker goroutines")
	fs.IntVar(&c.RPS, "rps", c.RPS, "stress mode sustained RPS")
	fs.IntVar(&c.PeakRPS, "peak-rps", c.PeakRPS, "peak mode burst RPS")
	fs.IntVar(&c.BurstSec, "burst-sec", c.BurstSec, "peak mode burst duration (seconds)")
	fs.IntVar(&c.IdleSec, "idle-sec", c.IdleSec, "peak mode idle duration (seconds)")
	fs.IntVar(&c.IdleRPS, "idle-rps", c.IdleRPS, "peak mode baseline RPS")
	fs.BoolVar(&c.Headless, "headless", false, "run without TUI")
	fs.BoolVar(&c.Yes, "yes", false, "skip confirmation for large runs")
	fs.StringVar(&c.DataDir, "data-dir", c.DataDir, "sentry-lite data dir for disk stats")
	fs.StringVar(&c.RedpandaCompose, "redpanda-compose", c.RedpandaCompose, "docker compose file for redpanda stats")
}

func (c *Config) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("workers must be >= 1")
	}
	if c.Total < 1 {
		return fmt.Errorf("total must be >= 1")
	}
	if c.Mode == ModeStress && c.RPS < 1 {
		return fmt.Errorf("rps must be >= 1 in stress mode")
	}
	if c.Mode == ModePeak {
		if c.PeakRPS < 1 || c.IdleRPS < 0 {
			return fmt.Errorf("peak-rps must be >= 1 and idle-rps >= 0")
		}
	}
	if err := c.ApplyDSN(c.DSN); err != nil {
		return err
	}
	sum := 0
	for _, w := range c.Mix {
		sum += w
	}
	if sum == 0 {
		return fmt.Errorf("feature mix weights must sum to > 0")
	}
	return nil
}

func (c *Config) NeedsConfirm() bool {
	return c.Total > 100_000 && !c.Yes
}

func (c Config) ComposePath() string {
	if c.RedpandaCompose == "" {
		return "docker-compose.redpanda.yml"
	}
	if filepath.IsAbs(c.RedpandaCompose) {
		return c.RedpandaCompose
	}
	if c.Root != "" {
		return filepath.Join(c.Root, c.RedpandaCompose)
	}
	return c.RedpandaCompose
}
