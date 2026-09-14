package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cellular-phagocyte/server/internal/httpx"
)

func TestCORSAllowsConfiguredOriginAndEchoesIt(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	policy := httpx.OriginPolicy{AllowedOrigins: []string{"https://game.example.com"}}
	h := withCORS(next, policy)

	r := httptest.NewRequest(http.MethodGet, "https://api.example.com/healthz", nil)
	r.Header.Set("Origin", "https://game.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if !called || w.Code != http.StatusOK {
		t.Fatalf("allowed request should reach handler: called=%v status=%d", called, w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://game.example.com" {
		t.Fatalf("CORS should echo exact allowed origin, got %q", got)
	}
	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", got)
	}
}

func TestCORSRejectsUnknownOriginBeforeHandler(t *testing.T) {
	called := false
	h := withCORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}), httpx.OriginPolicy{})

	r := httptest.NewRequest(http.MethodGet, "https://api.example.com/healthz", nil)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if called {
		t.Fatal("rejected origin must not reach application handler")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCORSAllowedPreflightStopsAtMiddleware(t *testing.T) {
	called := false
	h := withCORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}), httpx.OriginPolicy{AllowLoopback: true})

	r := httptest.NewRequest(http.MethodOptions, "http://localhost:8080/api/user/guest", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if called {
		t.Fatal("preflight should be handled by CORS middleware")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}
