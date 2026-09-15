package runtimeobs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCollectReturnsGoRuntimeStats(t *testing.T) {
	s := Collect(time.Now().Add(-time.Second), nil)
	if s.ServerTime <= 0 {
		t.Fatal("serverTime should be populated")
	}
	if s.UptimeSeconds <= 0 {
		t.Fatalf("uptimeSeconds=%f, want > 0", s.UptimeSeconds)
	}
	if s.Go.GOMAXPROCS <= 0 || s.Go.NumCPU <= 0 || s.Go.Goroutines <= 0 {
		t.Fatalf("invalid Go runtime stats: %+v", s.Go)
	}
	if s.Go.SysBytes == 0 {
		t.Fatalf("sysBytes=%d, want > 0", s.Go.SysBytes)
	}
}

func TestWrapServesRuntimeAndPreservesNextHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	h := Wrap(next, nil)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/runtime", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("runtime status=%d, want %d", rr.Code, http.StatusOK)
	}
	var got Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode runtime JSON: %v", err)
	}
	if got.Go.Goroutines <= 0 {
		t.Fatalf("goroutines=%d, want > 0", got.Go.Goroutines)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusCreated {
		t.Fatalf("fallback status=%d, want %d", rr.Code, http.StatusCreated)
	}
}
