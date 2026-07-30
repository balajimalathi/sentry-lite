package tui

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Root            string
	APIURL          string
	RedpandaCompose string
	WebDir          string
	GoAPICmd        []string
	WebCmd          []string
}

func LoadConfig() (Config, error) {
	root, err := findRoot()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Root:            root,
		APIURL:          env("TUI_API_URL", "http://localhost:8080"),
		RedpandaCompose: env("TUI_REDPANDA_COMPOSE", "docker-compose.redpanda.yml"),
		WebDir:          filepath.Join(root, "web"),
		GoAPICmd:        []string{"go", "run", "./cmd/sentry-lite"},
		WebCmd:          []string{"bun", "run", "dev"},
	}, nil
}

func findRoot() (string, error) {
	if r := os.Getenv("SENTRY_LITE_ROOT"); r != "" {
		return filepath.Abs(r)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd, nil
		}
		dir = parent
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
