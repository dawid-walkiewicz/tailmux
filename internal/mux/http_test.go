package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "test-api-token"

// TestAPIAuth checks who gets past the API's front door: only requests
// with the token, addressed to the server by IP or as localhost. The PAC
// file stays open to browsers but can't be run by a web page.
func TestAPIAuth(t *testing.T) {
	m := New(&Config{Tailnets: []TailnetConfig{{Name: "home"}}}, Options{})
	h := m.HTTPHandler("127.0.0.1:1055")
	do := func(method, path, host, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Host = host
		if token != "" {
			req.Header.Set("X-Tailmux", token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("no token configured refuses everything", func(t *testing.T) {
		if rec := do("GET", "/status", "127.0.0.1:1056", "1"); rec.Code != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", rec.Code)
		}
	})
	m.APIToken = testToken

	for _, c := range []struct {
		name, method, path, host, token string
		want                            int
	}{
		{"token", "GET", "/status", "127.0.0.1:1056", testToken, 200},
		{"localhost", "GET", "/status", "localhost:1056", testToken, 200},
		{"IPv6 loopback", "GET", "/status", "[::1]:1056", testToken, 200},
		{"no token", "GET", "/status", "127.0.0.1:1056", "", 401},
		{"the old constant", "GET", "/logs", "127.0.0.1:1056", "1", 401},
		{"wrong token", "POST", "/restart", "127.0.0.1:1056", "nope", 401},
		{"LocalAPI without token", "GET", "/tailnets/home/localapi/v0/status", "127.0.0.1:1056", "", 401},
		{"DNS rebinding", "GET", "/status", "evil.example:1056", testToken, 421},
		{"rebinding without token", "PUT", "/exit-node", "evil.example", "", 421},
		{"PAC without token", "GET", "/proxy.pac", "127.0.0.1:1056", "", 200},
		{"PAC under a page's domain", "GET", "/proxy.pac", "evil.example:1056", "", 421},
	} {
		t.Run(c.name, func(t *testing.T) {
			if rec := do(c.method, c.path, c.host, c.token); rec.Code != c.want {
				t.Errorf("%s %s Host %s: %d %q, want %d", c.method, c.path, c.host, rec.Code, rec.Body, c.want)
			}
		})
	}

	t.Run("PAC can't be run as a page's script", func(t *testing.T) {
		rec := do("GET", "/proxy.pac", "127.0.0.1:1056", "")
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options %q, want nosniff", got)
		}
	})
}
