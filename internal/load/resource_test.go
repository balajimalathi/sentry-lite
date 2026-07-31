package load

import (
	"path/filepath"
	"testing"
)

func TestCollectDiskDetailedFast(t *testing.T) {
	root := findRepoRoot()
	dataDir := filepath.Join(root, "data")
	data, events, sqlite, free, files := collectDiskDetailed(dataDir)
	if sqlite == 0 {
		t.Fatalf("expected sqlite size > 0, got data=%d sqlite=%d free=%d events=%d files=%d", data, sqlite, free, events, files)
	}
	if free == 0 {
		t.Fatalf("expected disk free > 0, got %d", free)
	}
	if data < sqlite {
		t.Fatalf("data (%d) should be >= sqlite (%d)", data, sqlite)
	}
	t.Logf("data=%d sqlite=%d free=%d", data, sqlite, free)
}
