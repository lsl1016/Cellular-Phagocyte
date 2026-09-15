package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// These flags are registered here so the existing wsload command can opt into
// server-side runtime sampling without coupling the main load loop to CSV I/O.
// The sampler reads the same command line directly and starts only when
// -timeseries-csv is explicitly supplied.
var (
	serverMetricsURLFlag = flag.String("server-metrics-url", "", "server runtime metrics endpoint; defaults to <base-url>/debug/runtime")
	timeseriesCSVFlag     = flag.String("timeseries-csv", "", "write one-second server runtime samples to this CSV path")
	metricsIntervalFlag  = flag.Duration("metrics-interval", time.Second, "server runtime metrics sampling interval")
)

type serverRuntimeSnapshot struct {
	ServerTime    int64   `json:"serverTime"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
	Process       struct {
		CPUSeconds   float64 `json:"cpuSeconds"`
		RSSBytes     uint64  `json:"rssBytes"`
		OpenFDs      int     `json:"openFDs"`
		CPUAvailable bool    `json:"cpuAvailable"`
		RSSAvailable bool    `json:"rssAvailable"`
		FDAvailable  bool    `json:"fdAvailable"`
	} `json:"process"`
	Go struct {
		GOMAXPROCS      int     `json:"gomaxprocs"`
		NumCPU          int     `json:"numCPU"`
		Goroutines      int     `json:"goroutines"`
		HeapAllocBytes  uint64  `json:"heapAllocBytes"`
		HeapInuseBytes  uint64  `json:"heapInuseBytes"`
		HeapObjects     uint64  `json:"heapObjects"`
		StackInuseBytes uint64  `json:"stackInuseBytes"`
		SysBytes        uint64  `json:"sysBytes"`
		TotalAllocBytes uint64  `json:"totalAllocBytes"`
		Mallocs         uint64  `json:"mallocs"`
		Frees           uint64  `json:"frees"`
		NumGC           uint32  `json:"numGC"`
		GCPauseTotalMs  float64 `json:"gcPauseTotalMs"`
	} `json:"go"`
	Game struct {
		Rooms           int `json:"rooms"`
		RunningRooms    int `json:"runningRooms"`
		Players         int `json:"players"`
		HumanPlayers    int `json:"humanPlayers"`
		AliveHumans     int `json:"aliveHumans"`
		DeadHumans      int `json:"deadHumans"`
		ExitedHumans    int `json:"exitedHumans"`
		ConnectedHumans int `json:"connectedHumans"`
		Balls           int `json:"balls"`
		Foods           int `json:"foods"`
		EjectedMass     int `json:"ejectedMass"`
	} `json:"game"`
}

type samplerConfig struct {
	URL      string
	CSVPath  string
	Interval time.Duration
}

func init() {
	// Keep references alive for flag package documentation and avoid accidental
	// removal as "unused" during future refactors.
	_ = serverMetricsURLFlag
	_ = timeseriesCSVFlag
	_ = metricsIntervalFlag

	cfg := parseSamplerArgs(os.Args[1:])
	if cfg.CSVPath == "" {
		return
	}
	go runServerMetricsSampler(cfg)
}

func parseSamplerArgs(args []string) samplerConfig {
	baseURL := "http://127.0.0.1:18080"
	cfg := samplerConfig{Interval: time.Second}
	for i := 0; i < len(args); i++ {
		name, value, hasValue := splitCLIArg(args[i])
		if !hasValue && i+1 < len(args) && strings.HasPrefix(name, "-") && !strings.HasPrefix(args[i+1], "-") {
			value = args[i+1]
			i++
			hasValue = true
		}
		if !hasValue {
			continue
		}
		switch name {
		case "-base-url", "--base-url":
			baseURL = strings.TrimRight(value, "/")
		case "-server-metrics-url", "--server-metrics-url":
			cfg.URL = value
		case "-timeseries-csv", "--timeseries-csv":
			cfg.CSVPath = value
		case "-metrics-interval", "--metrics-interval":
			if d, err := time.ParseDuration(value); err == nil && d > 0 {
				cfg.Interval = d
			}
		}
	}
	if cfg.URL == "" {
		cfg.URL = baseURL + "/debug/runtime"
	}
	return cfg
}

func splitCLIArg(arg string) (name, value string, hasValue bool) {
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", false
}

func runServerMetricsSampler(cfg samplerConfig) {
	if cfg.Interval <= 0 || cfg.CSVPath == "" || cfg.URL == "" {
		return
	}
	if dir := filepath.Dir(cfg.CSVPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "[metrics] create CSV directory failed: %v\n", err)
			return
		}
	}
	f, err := os.Create(cfg.CSVPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[metrics] create CSV failed: %v\n", err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"timestamp", "elapsed_seconds", "sample_error",
		"server_uptime_seconds", "process_cpu_seconds", "process_cpu_percent",
		"rss_bytes", "open_fds", "gomaxprocs", "num_cpu", "goroutines",
		"heap_alloc_bytes", "heap_inuse_bytes", "heap_objects", "stack_inuse_bytes",
		"sys_bytes", "total_alloc_bytes", "mallocs", "frees", "num_gc",
		"gc_pause_total_ms", "gc_pause_delta_ms",
		"rooms", "running_rooms", "players", "human_players", "alive_humans",
		"dead_humans", "exited_humans", "connected_humans", "balls", "foods", "ejected_mass",
	}
	if err := w.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "[metrics] write CSV header failed: %v\n", err)
		return
	}
	w.Flush()

	client := &http.Client{Timeout: 2 * time.Second}
	started := time.Now()
	var previous *serverRuntimeSnapshot
	var previousAt time.Time
	fmt.Printf("[metrics] server runtime sampling url=%s interval=%s csv=%s\n", cfg.URL, cfg.Interval, cfg.CSVPath)

	sample := func() {
		now := time.Now()
		current, err := fetchServerRuntimeSample(client, cfg.URL)
		if err != nil {
			row := make([]string, len(header))
			row[0] = now.Format(time.RFC3339Nano)
			row[1] = formatFloat(time.Since(started).Seconds())
			row[2] = err.Error()
			_ = w.Write(row)
			w.Flush()
			return
		}

		cpuPercent := 0.0
		gcPauseDelta := 0.0
		if previous != nil {
			wall := now.Sub(previousAt).Seconds()
			if wall > 0 && current.Process.CPUAvailable && previous.Process.CPUAvailable {
				cpuDelta := current.Process.CPUSeconds - previous.Process.CPUSeconds
				if cpuDelta >= 0 {
					cpuPercent = cpuDelta / wall * 100
				}
			}
			gcPauseDelta = current.Go.GCPauseTotalMs - previous.Go.GCPauseTotalMs
			if gcPauseDelta < 0 {
				gcPauseDelta = 0
			}
		}

		row := []string{
			now.Format(time.RFC3339Nano),
			formatFloat(time.Since(started).Seconds()),
			"",
			formatFloat(current.UptimeSeconds),
			formatFloat(current.Process.CPUSeconds),
			formatFloat(cpuPercent),
			strconv.FormatUint(current.Process.RSSBytes, 10),
			strconv.Itoa(current.Process.OpenFDs),
			strconv.Itoa(current.Go.GOMAXPROCS),
			strconv.Itoa(current.Go.NumCPU),
			strconv.Itoa(current.Go.Goroutines),
			strconv.FormatUint(current.Go.HeapAllocBytes, 10),
			strconv.FormatUint(current.Go.HeapInuseBytes, 10),
			strconv.FormatUint(current.Go.HeapObjects, 10),
			strconv.FormatUint(current.Go.StackInuseBytes, 10),
			strconv.FormatUint(current.Go.SysBytes, 10),
			strconv.FormatUint(current.Go.TotalAllocBytes, 10),
			strconv.FormatUint(current.Go.Mallocs, 10),
			strconv.FormatUint(current.Go.Frees, 10),
			strconv.FormatUint(uint64(current.Go.NumGC), 10),
			formatFloat(current.Go.GCPauseTotalMs),
			formatFloat(gcPauseDelta),
			strconv.Itoa(current.Game.Rooms),
			strconv.Itoa(current.Game.RunningRooms),
			strconv.Itoa(current.Game.Players),
			strconv.Itoa(current.Game.HumanPlayers),
			strconv.Itoa(current.Game.AliveHumans),
			strconv.Itoa(current.Game.DeadHumans),
			strconv.Itoa(current.Game.ExitedHumans),
			strconv.Itoa(current.Game.ConnectedHumans),
			strconv.Itoa(current.Game.Balls),
			strconv.Itoa(current.Game.Foods),
			strconv.Itoa(current.Game.EjectedMass),
		}
		if err := w.Write(row); err != nil {
			fmt.Fprintf(os.Stderr, "[metrics] write CSV row failed: %v\n", err)
			return
		}
		w.Flush()
		previous = current
		previousAt = now
	}

	// Sample immediately; early setup failures are retained in the CSV and make
	// observability gaps visible instead of silently disappearing.
	sample()
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for range ticker.C {
		sample()
	}
}

func fetchServerRuntimeSample(client *http.Client, url string) (*serverRuntimeSnapshot, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s -> HTTP %d", url, resp.StatusCode)
	}
	var out serverRuntimeSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
