package main

import (
	"net/http"
	"testing"
)

func TestNewHTTPServerUsesDefensiveTimeouts(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	srv := newHTTPServer(":9999", h)

	if srv.Addr != ":9999" {
		t.Fatalf("unexpected addr: %s", srv.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("handler should be configured")
	}
	if srv.ReadHeaderTimeout != httpReadHeaderTimeout {
		t.Fatalf("unexpected read header timeout: %s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != httpReadTimeout {
		t.Fatalf("unexpected read timeout: %s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != httpWriteTimeout {
		t.Fatalf("unexpected write timeout: %s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != httpIdleTimeout {
		t.Fatalf("unexpected idle timeout: %s", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != httpMaxHeaderBytes {
		t.Fatalf("unexpected max header bytes: %d", srv.MaxHeaderBytes)
	}
}
