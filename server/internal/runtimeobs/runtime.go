package runtimeobs

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cellular-phagocyte/server/internal/game"
)

// Snapshot is the server-side runtime sample consumed by wsload.
type Snapshot struct {
	ServerTime    int64             `json:"serverTime"`
	UptimeSeconds float64           `json:"uptimeSeconds"`
	Process       ProcessStats      `json:"process"`
	Go            GoRuntimeStats    `json:"go"`
	Game          game.RuntimeStats `json:"game"`
}

type ProcessStats struct {
	CPUSeconds   float64 `json:"cpuSeconds"`
	RSSBytes     uint64  `json:"rssBytes"`
	OpenFDs      int     `json:"openFDs"`
	CPUAvailable bool    `json:"cpuAvailable"`
	RSSAvailable bool    `json:"rssAvailable"`
	FDAvailable  bool    `json:"fdAvailable"`
}

type GoRuntimeStats struct {
	GOMAXPROCS       int     `json:"gomaxprocs"`
	NumCPU           int     `json:"numCPU"`
	Goroutines       int     `json:"goroutines"`
	HeapAllocBytes   uint64  `json:"heapAllocBytes"`
	HeapInuseBytes   uint64  `json:"heapInuseBytes"`
	HeapObjects      uint64  `json:"heapObjects"`
	StackInuseBytes  uint64  `json:"stackInuseBytes"`
	SysBytes         uint64  `json:"sysBytes"`
	TotalAllocBytes  uint64  `json:"totalAllocBytes"`
	Mallocs          uint64  `json:"mallocs"`
	Frees            uint64  `json:"frees"`
	NumGC            uint32  `json:"numGC"`
	GCPauseTotalMs   float64 `json:"gcPauseTotalMs"`
	LastGCUnixMillis int64   `json:"lastGCUnixMillis"`
}

// Wrap adds GET /debug/runtime in front of the application handler.
// Callers should only use this wrapper when runtime metrics are explicitly enabled.
func Wrap(next http.Handler, mgr *game.Manager) http.Handler {
	startedAt := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/runtime", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Collect(startedAt, mgr))
	})
	mux.Handle("/", next)
	return mux
}

func Collect(startedAt time.Time, mgr *game.Manager) Snapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	gameStats := game.RuntimeStats{}
	if mgr != nil {
		gameStats = mgr.RuntimeStats()
	}

	lastGC := int64(0)
	if ms.LastGC > 0 {
		lastGC = int64(ms.LastGC / uint64(time.Millisecond))
	}

	return Snapshot{
		ServerTime:    time.Now().UnixMilli(),
		UptimeSeconds: time.Since(startedAt).Seconds(),
		Process:       collectProcessStats(),
		Go: GoRuntimeStats{
			GOMAXPROCS:       runtime.GOMAXPROCS(0),
			NumCPU:           runtime.NumCPU(),
			Goroutines:       runtime.NumGoroutine(),
			HeapAllocBytes:   ms.HeapAlloc,
			HeapInuseBytes:   ms.HeapInuse,
			HeapObjects:      ms.HeapObjects,
			StackInuseBytes:  ms.StackInuse,
			SysBytes:         ms.Sys,
			TotalAllocBytes:  ms.TotalAlloc,
			Mallocs:          ms.Mallocs,
			Frees:            ms.Frees,
			NumGC:            ms.NumGC,
			GCPauseTotalMs:   float64(ms.PauseTotalNs) / float64(time.Millisecond),
			LastGCUnixMillis: lastGC,
		},
		Game: gameStats,
	}
}

func collectProcessStats() ProcessStats {
	var out ProcessStats
	if b, err := os.ReadFile("/proc/self/schedstat"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if cpuNs, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
				out.CPUSeconds = float64(cpuNs) / float64(time.Second)
				out.CPUAvailable = true
			}
		}
	}
	if b, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 1 {
			if residentPages, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				out.RSSBytes = residentPages * uint64(os.Getpagesize())
				out.RSSAvailable = true
			}
		}
	}
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		out.OpenFDs = len(entries)
		out.FDAvailable = true
	}
	return out
}
