package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr           string
	SQLitePath         string
	RedpandaBrokers    []string
	DataDir            string
	IngestTopic        string
	WebDist            string
	PublicURL          string
	AlertSMTP          string
	AlertSMTPUser      string
	AlertSMTPPass      string
	AlertFrom          string
	AdminToken         string
	RequireAdminToken  bool
	IngestRPS          int
	EventRetentionDays int
}

func Load() Config {
	loadDotEnv()
	brokers := env("REDPANDA_BROKERS", "localhost:19092")
	return Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		SQLitePath:         env("SQLITE_PATH", "./data/sentry-lite.db"),
		RedpandaBrokers:    splitCSV(brokers),
		DataDir:            env("DATA_DIR", "./data"),
		IngestTopic:        env("INGEST_TOPIC", "events.ingest"),
		WebDist:            env("WEB_DIST", "./web/dist"),
		PublicURL:          env("PUBLIC_URL", "http://localhost:8080"),
		AlertSMTP:          env("ALERT_SMTP", ""),
		AlertSMTPUser:      env("ALERT_SMTP_USER", ""),
		AlertSMTPPass:      env("ALERT_SMTP_PASS", ""),
		AlertFrom:          env("ALERT_FROM", "sentry-lite@localhost"),
		AdminToken:         env("ADMIN_TOKEN", ""),
		RequireAdminToken:  envBool("REQUIRE_ADMIN_TOKEN") || dockerEnvPresent(),
		IngestRPS:          envInt("INGEST_RPS", 0),
		EventRetentionDays: envInt("EVENT_RETENTION_DAYS", 14),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func dockerEnvPresent() bool {
	if st, err := os.Stat("/.dockerenv"); err == nil && !st.IsDir() {
		return true
	}
	return false
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv loads KEY=VALUE pairs from a nearby .env into the process
// environment without overriding already-set vars. Missing file is a no-op.
func loadDotEnv() {
	path := findDotEnv()
	if path == "" {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		_ = os.Setenv(key, val)
	}
}

func findDotEnv() string {
	wd, err := os.Getwd()
	if err != nil {
		if st, err := os.Stat(".env"); err == nil && !st.IsDir() {
			return ".env"
		}
		return ""
	}
	dir := wd
	for {
		p := filepath.Join(dir, ".env")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
