package ingest

import "testing"

func TestTokenBucketAllowsBurstThenLimits(t *testing.T) {
	h := &Handler{IngestRPS: 2}
	if !h.allowIngest(1) || !h.allowIngest(1) {
		t.Fatal("burst should allow rps tokens")
	}
	if h.allowIngest(1) {
		t.Fatal("should rate limit after burst")
	}
	if h.IngestRPS = 0; !(&Handler{IngestRPS: 0}).allowIngest(9) {
		t.Fatal("rps 0 is unlimited")
	}
}
