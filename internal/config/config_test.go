package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("ADMIN_TOKEN=dev-token-from-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also need go.mod stop marker optional — put file in dir and chdir
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_TOKEN", "")
	_ = os.Unsetenv("ADMIN_TOKEN")

	cfg := Load()
	if cfg.AdminToken != "dev-token-from-env" {
		t.Fatalf("got %q", cfg.AdminToken)
	}
}

func TestLoadDoesNotOverrideExistingEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ADMIN_TOKEN=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_TOKEN", "from-shell")

	cfg := Load()
	if cfg.AdminToken != "from-shell" {
		t.Fatalf("got %q", cfg.AdminToken)
	}
}
