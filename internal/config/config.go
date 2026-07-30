package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr        string
	SQLitePath      string
	RedpandaBrokers []string
	DataDir         string
	CORSOrigins     []string
	IngestTopic     string
	WebDist         string
	PublicURL       string
	AlertSMTP       string
	AlertFrom       string
}

func Load() Config {
	brokers := env("REDPANDA_BROKERS", "localhost:19092")
	cors := env("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000")
	return Config{
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		SQLitePath:      env("SQLITE_PATH", "./data/sentry-lite.db"),
		RedpandaBrokers: splitCSV(brokers),
		DataDir:         env("DATA_DIR", "./data"),
		CORSOrigins:     splitCSV(cors),
		IngestTopic:     env("INGEST_TOPIC", "events.ingest"),
		WebDist:         env("WEB_DIST", "./web/dist"),
		PublicURL:       env("PUBLIC_URL", "http://localhost:8080"),
		AlertSMTP:       env("ALERT_SMTP", ""),
		AlertFrom:       env("ALERT_FROM", "sentry-lite@localhost"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
