package load

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectDiskDetailedFast(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "sentry-lite.db")
	wal := filepath.Join(dir, "sentry-lite.db-wal")
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(db, make([]byte, 128), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wal, make([]byte, 32), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, make([]byte, 16), 0o644); err != nil {
		t.Fatal(err)
	}

	data, events, sqlite, free, files := collectDiskDetailed(dir)
	if sqlite != 160 {
		t.Fatalf("expected sqlite size 160, got data=%d sqlite=%d free=%d events=%d files=%d", data, sqlite, free, events, files)
	}
	if data != 176 {
		t.Fatalf("expected data size 176 (sqlite + notes.txt), got %d", data)
	}
	if free == 0 {
		t.Fatalf("expected disk free > 0, got %d", free)
	}
	if files != 0 {
		t.Fatalf("event file count should stay 0 without walking events/, got %d", files)
	}
}
