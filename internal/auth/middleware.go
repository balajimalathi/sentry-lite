package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// RequireAdmin rejects requests that do not present the configured admin token
// via Authorization: Bearer <token> or HTTP Basic (any username, password = token).
// Pass an empty token to disable (caller should skip installing this middleware).
func RequireAdmin(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authorized(r.Header.Get("Authorization"), token) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="sentry-lite"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authorized(header, token string) bool {
	if token == "" || header == "" {
		return false
	}
	const bearer = "Bearer "
	if strings.HasPrefix(header, bearer) {
		return secureEqual(strings.TrimSpace(header[len(bearer):]), token)
	}
	const basic = "Basic "
	if strings.HasPrefix(header, basic) {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(basic):]))
		if err != nil {
			return false
		}
		userPass := string(raw)
		_, pass, ok := strings.Cut(userPass, ":")
		if !ok {
			return false
		}
		return secureEqual(pass, token)
	}
	return false
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
