package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skndan/sentry-lite/internal/auth"
)

func TestRequireAdmin(t *testing.T) {
	const token = "s3cret-token"
	h := auth.RequireAdmin(token)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", rec.Code)
		}
	})

	t.Run("bearer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d", rec.Code)
		}
	})

	t.Run("wrong bearer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", rec.Code)
		}
	})

	t.Run("basic", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("admin", token)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d", rec.Code)
		}
	})
}
