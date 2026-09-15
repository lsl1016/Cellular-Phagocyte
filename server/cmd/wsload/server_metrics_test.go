package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSamplerArgs(t *testing.T) {
	cfg := parseSamplerArgs([]string{
		"-base-url", "http://127.0.0.1:19000/",
		"-timeseries-csv=/tmp/runtime.csv",
		"-metrics-interval", "2s",
	})
	if cfg.URL != "http://127.0.0.1:19000/debug/runtime" {
		t.Fatalf("URL=%q", cfg.URL)
	}
	if cfg.CSVPath != "/tmp/runtime.csv" {
		t.Fatalf("CSVPath=%q", cfg.CSVPath)
	}
	if cfg.Interval != 2*time.Second {
		t.Fatalf("Interval=%s", cfg.Interval)
	}
}

func TestFetchServerRuntimeSample(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"serverTime":1,"uptimeSeconds":2.5,"process":{"cpuSeconds":1.25,"rssBytes":4096,"openFDs":7,"cpuAvailable":true,"rssAvailable":true,"fdAvailable":true},"go":{"gomaxprocs":4,"numCPU":4,"goroutines":9,"heapAllocBytes":10,"heapInuseBytes":20,"heapObjects":3,"stackInuseBytes":4,"sysBytes":30,"totalAllocBytes":40,"mallocs":5,"frees":2,"numGC":1,"gcPauseTotalMs":0.5},"game":{"rooms":2,"runningRooms":1,"players":100,"humanPlayers":100,"connectedHumans":99,"balls":110,"foods":500,"ejectedMass":20}}`))
	}))
	defer ts.Close()

	got, err := fetchServerRuntimeSample(ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("fetch sample: %v", err)
	}
	if got.Process.CPUSeconds != 1.25 || got.Go.Goroutines != 9 || got.Game.ConnectedHumans != 99 {
		t.Fatalf("unexpected sample: %+v", got)
	}
}
