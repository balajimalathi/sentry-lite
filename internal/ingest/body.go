package ingest

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxIngestBody = 32 << 20 // 32 MiB

var errBodyTooLarge = errors.New("body too large")

func readIngestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	var reader io.Reader = io.LimitReader(r.Body, maxIngestBody+1)
	enc := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	if i := strings.IndexByte(enc, ','); i >= 0 {
		enc = strings.TrimSpace(enc[:i])
	}
	if enc == "gzip" || enc == "x-gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = io.LimitReader(gz, maxIngestBody+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxIngestBody {
		return nil, errBodyTooLarge
	}
	return body, nil
}
