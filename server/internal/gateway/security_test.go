package gateway

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"cellular-phagocyte/server/internal/httpx"
)

func TestGatewayUsesConfiguredOriginPolicy(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	g := NewWithSecurity(nil, log, httpx.OriginPolicy{
		AllowedOrigins: []string{"https://game.example.com"},
	}, 12345)

	allowed := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	allowed.Header.Set("Origin", "https://game.example.com")
	if !g.upgrader.CheckOrigin(allowed) {
		t.Fatal("configured websocket origin should be allowed")
	}

	rejected := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	rejected.Header.Set("Origin", "https://evil.example")
	if g.upgrader.CheckOrigin(rejected) {
		t.Fatal("unknown websocket origin should be rejected")
	}

	if g.readLimitBytes != 12345 {
		t.Fatalf("unexpected websocket read limit: %d", g.readLimitBytes)
	}
}

func TestGatewayNewUsesSafeLocalDefaults(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	g := New(nil, log)

	local := httptest.NewRequest("GET", "http://localhost:8080/ws", nil)
	local.Header.Set("Origin", "http://localhost:5173")
	if !g.upgrader.CheckOrigin(local) {
		t.Fatal("default gateway should allow loopback development origins")
	}

	external := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	external.Header.Set("Origin", "https://evil.example")
	if g.upgrader.CheckOrigin(external) {
		t.Fatal("default gateway must not allow arbitrary cross-origin websocket requests")
	}

	if g.readLimitBytes != defaultWSReadLimitBytes {
		t.Fatalf("unexpected default websocket read limit: %d", g.readLimitBytes)
	}
}
