package ingest

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"testing"
)

func TestReadIngestBodyGzip(t *testing.T) {
	plain := []byte(`{"event_id":"abc"}`)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Encoding", "gzip")

	got, err := readIngestBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestReadIngestBodyUncompressed(t *testing.T) {
	plain := []byte(`{"event_id":"abc"}`)
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	got, err := readIngestBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestReadIngestBodyTooLarge(t *testing.T) {
	huge := []byte(strings.Repeat("x", maxIngestBody+2))
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(huge))
	if err != nil {
		t.Fatal(err)
	}
	_, err = readIngestBody(req)
	if err != errBodyTooLarge {
		t.Fatalf("got %v want errBodyTooLarge", err)
	}
}
