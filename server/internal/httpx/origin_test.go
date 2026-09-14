package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestOriginPolicyAllowsSameOriginAndNativeClients(t *testing.T) {
	p := OriginPolicy{}

	native := httptest.NewRequest("GET", "http://api.example.com/ws", nil)
	if !p.AllowsRequest(native) {
		t.Fatal("request without Origin should be allowed for native/non-browser clients")
	}

	same := httptest.NewRequest("GET", "http://api.example.com/ws", nil)
	same.Header.Set("Origin", "http://api.example.com")
	if !p.AllowsRequest(same) {
		t.Fatal("same-origin browser request should be allowed")
	}
}

func TestOriginPolicyRejectsUnknownCrossOrigin(t *testing.T) {
	p := OriginPolicy{}
	r := httptest.NewRequest("GET", "http://api.example.com/ws", nil)
	r.Header.Set("Origin", "https://evil.example")
	if p.AllowsRequest(r) {
		t.Fatal("unknown cross-origin request should be rejected")
	}
}

func TestOriginPolicyAllowsConfiguredOrigin(t *testing.T) {
	p := OriginPolicy{AllowedOrigins: []string{"https://game.example.com/"}}
	r := httptest.NewRequest("GET", "https://api.example.com/ws", nil)
	r.Header.Set("Origin", "https://game.example.com")
	if !p.AllowsRequest(r) {
		t.Fatal("configured frontend origin should be allowed")
	}
}

func TestOriginPolicyLoopbackModeAllowsLocalDevPorts(t *testing.T) {
	p := OriginPolicy{AllowLoopback: true}
	for _, origin := range []string{
		"http://localhost:5173",
		"http://127.0.0.1:7456",
		"http://[::1]:8081",
	} {
		r := httptest.NewRequest("GET", "http://localhost:8080/ws", nil)
		r.Header.Set("Origin", origin)
		if !p.AllowsRequest(r) {
			t.Fatalf("loopback origin should be allowed in local-dev mode: %s", origin)
		}
	}
}

func TestOriginPolicyRejectsMalformedOrNonHTTPOrigin(t *testing.T) {
	p := OriginPolicy{AllowLoopback: true}
	for _, origin := range []string{"not-a-url", "file://localhost/tmp", "ftp://localhost:21"} {
		r := httptest.NewRequest("GET", "http://localhost:8080/ws", nil)
		r.Header.Set("Origin", origin)
		if p.AllowsRequest(r) {
			t.Fatalf("malformed/non-http origin should be rejected: %s", origin)
		}
	}
}
