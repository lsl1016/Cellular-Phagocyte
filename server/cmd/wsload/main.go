// Command wsload drives many real WebSocket clients against a separately running game server.
// It is intended for repeatable load, reconnect-storm and weak-network validation.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/protocol"
)

type options struct {
	baseURL             string
	wsURL               string
	clients             int
	duration            time.Duration
	moveInterval        time.Duration
	latency             time.Duration
	jitter              time.Duration
	incomingDropRate    float64
	outgoingDropRate    float64
	stormFraction       float64
	stormCount          int
	churnFraction       float64
	reconnectJitter     time.Duration
	setupConcurrency    int
	matchTimeout        time.Duration
	minReconnectSuccess float64
	minSnapshotHealthy  float64
	seed                int64
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type metrics struct {
	logins              atomic.Int64
	matches             atomic.Int64
	connects            atomic.Int64
	gameStarts          atomic.Int64
	snapshotClients     atomic.Int64
	fullSnapshots       atomic.Int64
	deltaSnapshots      atomic.Int64
	recoverSnapshots    atomic.Int64
	incomingDropped     atomic.Int64
	seqMismatches       atomic.Int64
	fullSyncRequests    atomic.Int64
	movesSent           atomic.Int64
	movesDropped        atomic.Int64
	forcedDisconnects   atomic.Int64
	reconnectAttempts   atomic.Int64
	reconnectSuccess    atomic.Int64
	reconnectFailures   atomic.Int64
	unexpectedReadError atomic.Int64
	writeErrors         atomic.Int64
	bytesIn             atomic.Int64

	mu               sync.Mutex
	reconnectLatency []time.Duration
	snapshotLag      []time.Duration
}

func (m *metrics) addReconnectLatency(v time.Duration) {
	m.mu.Lock()
	m.reconnectLatency = append(m.reconnectLatency, v)
	m.mu.Unlock()
}

func (m *metrics) addSnapshotLag(v time.Duration) {
	if v < 0 {
		v = 0
	}
	m.mu.Lock()
	m.snapshotLag = append(m.snapshotLag, v)
	m.mu.Unlock()
}

func (m *metrics) latencies() ([]time.Duration, []time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := append([]time.Duration(nil), m.reconnectLatency...)
	s := append([]time.Duration(nil), m.snapshotLag...)
	return r, s
}

type loadClient struct {
	index int
	opt   *options
	m     *metrics

	userID         string
	accessToken    string
	matchID        string
	roomID         string
	enterToken     string
	reconnectToken string

	mu              sync.RWMutex
	conn            *websocket.Conn
	seq             int64
	lastSnapshotSeq int64
	needsFull       bool

	writeMu sync.Mutex
	rngMu   sync.Mutex
	rng     *rand.Rand

	started      atomic.Bool
	sawSnapshot  atomic.Bool
	reconnecting atomic.Bool
}

type report struct {
	Clients                   int     `json:"clients"`
	Rooms                     int     `json:"rooms"`
	DurationSeconds           float64 `json:"durationSeconds"`
	Logins                    int64   `json:"logins"`
	Matches                   int64   `json:"matches"`
	InitialConnections        int64   `json:"initialConnections"`
	GameStarts                int64   `json:"gameStarts"`
	SnapshotClients           int64   `json:"snapshotClients"`
	FullSnapshots             int64   `json:"fullSnapshots"`
	DeltaSnapshots            int64   `json:"deltaSnapshots"`
	RecoverSnapshots          int64   `json:"recoverSnapshots"`
	IncomingSnapshotsDropped  int64   `json:"incomingSnapshotsDropped"`
	SequenceMismatches        int64   `json:"sequenceMismatches"`
	FullSyncRequests          int64   `json:"fullSyncRequests"`
	MovesSent                 int64   `json:"movesSent"`
	MovesDropped              int64   `json:"movesDropped"`
	ForcedDisconnects         int64   `json:"forcedDisconnects"`
	ReconnectAttempts         int64   `json:"reconnectAttempts"`
	ReconnectSuccess          int64   `json:"reconnectSuccess"`
	ReconnectFailures         int64   `json:"reconnectFailures"`
	ReconnectSuccessRate      float64 `json:"reconnectSuccessRate"`
	HealthySnapshotClients    int     `json:"healthySnapshotClients"`
	SnapshotHealthyRate       float64 `json:"snapshotHealthyRate"`
	UnexpectedReadErrors      int64   `json:"unexpectedReadErrors"`
	WriteErrors               int64   `json:"writeErrors"`
	BytesReceived             int64   `json:"bytesReceived"`
	ReconnectLatencyP50Ms     float64 `json:"reconnectLatencyP50Ms"`
	ReconnectLatencyP95Ms     float64 `json:"reconnectLatencyP95Ms"`
	ReconnectLatencyP99Ms     float64 `json:"reconnectLatencyP99Ms"`
	SnapshotReceiveLagP50Ms   float64 `json:"snapshotReceiveLagP50Ms"`
	SnapshotReceiveLagP95Ms   float64 `json:"snapshotReceiveLagP95Ms"`
	SnapshotReceiveLagP99Ms   float64 `json:"snapshotReceiveLagP99Ms"`
	ConfiguredLatencyMs       float64 `json:"configuredLatencyMs"`
	ConfiguredJitterMs        float64 `json:"configuredJitterMs"`
	ConfiguredIncomingDropPct float64 `json:"configuredIncomingDropPct"`
	ConfiguredOutgoingDropPct float64 `json:"configuredOutgoingDropPct"`
	StormFraction             float64 `json:"stormFraction"`
	StormCount                int     `json:"stormCount"`
	ChurnFractionPerSecond    float64 `json:"churnFractionPerSecond"`
}

func main() {
	opt := parseFlags()
	if err := validateOptions(opt); err != nil {
		fmt.Fprintf(os.Stderr, "WS_LOAD_CHAOS INVALID: %v\n", err)
		os.Exit(2)
	}
	if err := run(opt); err != nil {
		fmt.Fprintf(os.Stderr, "WS_LOAD_CHAOS FAILED: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() *options {
	opt := &options{}
	flag.StringVar(&opt.baseURL, "base-url", "http://127.0.0.1:18080", "HTTP base URL")
	flag.StringVar(&opt.wsURL, "ws-url", "ws://127.0.0.1:18080/ws", "WebSocket URL")
	flag.IntVar(&opt.clients, "clients", 50, "number of real WebSocket clients")
	flag.DurationVar(&opt.duration, "duration", 12*time.Second, "chaos traffic duration")
	flag.DurationVar(&opt.moveInterval, "move-interval", 200*time.Millisecond, "MOVE send interval per client")
	flag.DurationVar(&opt.latency, "latency", 20*time.Millisecond, "application-level one-way processing delay")
	flag.DurationVar(&opt.jitter, "jitter", 20*time.Millisecond, "uniform +/- delay jitter")
	flag.Float64Var(&opt.incomingDropRate, "incoming-drop-rate", 0.02, "fraction of incoming snapshots deliberately ignored")
	flag.Float64Var(&opt.outgoingDropRate, "outgoing-drop-rate", 0.02, "fraction of outgoing MOVE messages deliberately dropped")
	flag.Float64Var(&opt.stormFraction, "storm-fraction", 0.30, "fraction of clients disconnected together in each storm")
	flag.IntVar(&opt.stormCount, "storm-count", 2, "number of reconnect storms during the run")
	flag.Float64Var(&opt.churnFraction, "churn-fraction", 0.01, "fraction of clients randomly disconnected each second")
	flag.DurationVar(&opt.reconnectJitter, "reconnect-jitter", 500*time.Millisecond, "random delay before reconnect attempts")
	flag.IntVar(&opt.setupConcurrency, "setup-concurrency", 50, "maximum concurrent HTTP/dial setup operations")
	flag.DurationVar(&opt.matchTimeout, "match-timeout", 12*time.Second, "maximum matchmaking wait")
	flag.Float64Var(&opt.minReconnectSuccess, "min-reconnect-success", 0.98, "minimum acceptable reconnect success ratio")
	flag.Float64Var(&opt.minSnapshotHealthy, "min-snapshot-healthy", 0.98, "minimum clients ending with a valid snapshot baseline")
	flag.Int64Var(&opt.seed, "seed", 20260915, "deterministic chaos random seed")
	flag.Parse()
	opt.baseURL = strings.TrimRight(opt.baseURL, "/")
	return opt
}

func validateOptions(opt *options) error {
	if opt.clients <= 0 {
		return errors.New("clients must be > 0")
	}
	if opt.duration <= 0 || opt.moveInterval <= 0 {
		return errors.New("duration and move-interval must be > 0")
	}
	if opt.setupConcurrency <= 0 {
		return errors.New("setup-concurrency must be > 0")
	}
	for name, value := range map[string]float64{
		"incoming-drop-rate":    opt.incomingDropRate,
		"outgoing-drop-rate":    opt.outgoingDropRate,
		"storm-fraction":        opt.stormFraction,
		"churn-fraction":        opt.churnFraction,
		"min-reconnect-success": opt.minReconnectSuccess,
		"min-snapshot-healthy":  opt.minSnapshotHealthy,
	} {
		if value < 0 || value > 1 {
			return fmt.Errorf("%s must be in [0,1]", name)
		}
	}
	if opt.stormCount < 0 {
		return errors.New("storm-count must be >= 0")
	}
	return nil
}

func run(opt *options) error {
	if err := waitHealth(opt.baseURL+"/healthz", 10*time.Second); err != nil {
		return err
	}
	fmt.Printf("[setup] server healthy; clients=%d duration=%s latency=%s jitter=%s incomingDrop=%.1f%% outgoingDrop=%.1f%%\n",
		opt.clients, opt.duration, opt.latency, opt.jitter, opt.incomingDropRate*100, opt.outgoingDropRate*100)

	m := &metrics{}
	clients := make([]*loadClient, opt.clients)
	for i := range clients {
		clients[i] = &loadClient{
			index: i,
			opt:   opt,
			m:     m,
			rng:   rand.New(rand.NewSource(opt.seed + int64(i+1)*7919)),
		}
	}

	if err := parallel(opt.clients, opt.setupConcurrency, func(i int) error {
		return clients[i].login()
	}); err != nil {
		closeAll(clients)
		return fmt.Errorf("guest login phase: %w", err)
	}
	fmt.Printf("[setup] guest login complete: %d/%d\n", m.logins.Load(), opt.clients)

	if err := parallel(opt.clients, opt.setupConcurrency, func(i int) error {
		return clients[i].startMatch()
	}); err != nil {
		closeAll(clients)
		return fmt.Errorf("match start phase: %w", err)
	}
	if err := parallel(opt.clients, opt.setupConcurrency, func(i int) error {
		return clients[i].waitMatched(opt.matchTimeout)
	}); err != nil {
		closeAll(clients)
		return fmt.Errorf("match wait phase: %w", err)
	}
	rooms := roomCount(clients)
	fmt.Printf("[setup] matchmaking complete: %d/%d across %d room(s)\n", m.matches.Load(), opt.clients, rooms)

	if err := parallel(opt.clients, opt.setupConcurrency, func(i int) error {
		return clients[i].enterInitial()
	}); err != nil {
		closeAll(clients)
		return fmt.Errorf("initial WebSocket enter phase: %w", err)
	}
	for _, c := range clients {
		c.startReader(c.currentConn())
	}
	fmt.Printf("[setup] WebSocket ENTER_ROOM complete: %d/%d\n", m.connects.Load(), opt.clients)

	if err := parallel(opt.clients, opt.setupConcurrency, func(i int) error {
		return clients[i].sendControl(protocol.TypeReady, protocol.ReadyData{RoomID: clients[i].roomID, UserID: clients[i].userID})
	}); err != nil {
		closeAll(clients)
		return fmt.Errorf("READY phase: %w", err)
	}
	if err := waitForCount(&m.gameStarts, int64(opt.clients), 10*time.Second); err != nil {
		closeAll(clients)
		return fmt.Errorf("GAME_START phase: %w", err)
	}
	if err := waitForCount(&m.snapshotClients, int64(opt.clients), 8*time.Second); err != nil {
		closeAll(clients)
		return fmt.Errorf("initial snapshot phase: %w", err)
	}
	fmt.Printf("[setup] all clients reached GAME_START and received snapshots\n")

	trafficStop := make(chan struct{})
	var trafficWG sync.WaitGroup
	for _, c := range clients {
		trafficWG.Add(1)
		go func(c *loadClient) {
			defer trafficWG.Done()
			c.moveLoop(trafficStop)
		}(c)
	}

	chaosStart := time.Now()
	chaosDeadline := chaosStart.Add(opt.duration)
	stormTimes := make([]time.Time, 0, opt.stormCount)
	for i := 1; i <= opt.stormCount; i++ {
		fraction := float64(i) / float64(opt.stormCount+1)
		stormTimes = append(stormTimes, chaosStart.Add(time.Duration(float64(opt.duration)*fraction)))
	}
	stormIndex := 0
	churnTicker := time.NewTicker(time.Second)
	defer churnTicker.Stop()
	poll := time.NewTicker(50 * time.Millisecond)
	defer poll.Stop()
	chaosRNG := rand.New(rand.NewSource(opt.seed ^ 0x5f3759df))
	var chaosWG sync.WaitGroup

	for time.Now().Before(chaosDeadline) {
		select {
		case <-poll.C:
			for stormIndex < len(stormTimes) && !time.Now().Before(stormTimes[stormIndex]) {
				selected := selectClients(clients, opt.stormFraction, chaosRNG)
				fmt.Printf("[chaos] reconnect storm %d/%d: disconnecting %d clients\n", stormIndex+1, opt.stormCount, len(selected))
				for _, c := range selected {
					chaosWG.Add(1)
					go func(c *loadClient) {
						defer chaosWG.Done()
						c.forceReconnect()
					}(c)
				}
				stormIndex++
			}
		case <-churnTicker.C:
			selected := selectClients(clients, opt.churnFraction, chaosRNG)
			for _, c := range selected {
				chaosWG.Add(1)
				go func(c *loadClient) {
					defer chaosWG.Done()
					c.forceReconnect()
				}(c)
			}
		}
	}

	close(trafficStop)
	trafficWG.Wait()
	chaosWG.Wait()

	// Give FULL_SYNC repair requests generated near the end of the run time to converge.
	for _, c := range clients {
		if c.snapshotNeedsRepair() {
			_ = c.requestFullSync()
		}
	}
	time.Sleep(2 * time.Second)

	r := buildReport(opt, clients, m, rooms)
	printReport(r)
	closeAll(clients)

	if r.GameStarts != int64(opt.clients) {
		return fmt.Errorf("only %d/%d clients received GAME_START", r.GameStarts, opt.clients)
	}
	if r.SnapshotHealthyRate < opt.minSnapshotHealthy {
		return fmt.Errorf("snapshot healthy rate %.3f below required %.3f", r.SnapshotHealthyRate, opt.minSnapshotHealthy)
	}
	if r.ReconnectAttempts > 0 && r.ReconnectSuccessRate < opt.minReconnectSuccess {
		return fmt.Errorf("reconnect success rate %.3f below required %.3f", r.ReconnectSuccessRate, opt.minReconnectSuccess)
	}
	fmt.Println("WS_LOAD_CHAOS PASS")
	return nil
}

func (c *loadClient) login() error {
	var out struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			UserID string `json:"userId"`
		} `json:"user"`
	}
	deviceID := fmt.Sprintf("wsload-%d-%d", c.opt.seed, c.index)
	if err := postJSON(c.opt.baseURL+"/api/auth/guest-login", "", map[string]any{"deviceId": deviceID}, &out); err != nil {
		return fmt.Errorf("client %d login: %w", c.index, err)
	}
	if out.AccessToken == "" || out.User.UserID == "" {
		return fmt.Errorf("client %d login returned empty identity", c.index)
	}
	c.accessToken = out.AccessToken
	c.userID = out.User.UserID
	c.m.logins.Add(1)
	return nil
}

func (c *loadClient) startMatch() error {
	var out struct {
		MatchID string `json:"matchId"`
	}
	if err := postJSON(c.opt.baseURL+"/api/match/start", c.accessToken, map[string]any{"mode": "classic"}, &out); err != nil {
		return fmt.Errorf("client %d match start: %w", c.index, err)
	}
	if out.MatchID == "" {
		return fmt.Errorf("client %d match start returned empty matchId", c.index)
	}
	c.matchID = out.MatchID
	return nil
}

func (c *loadClient) waitMatched(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var out struct {
			Status     string `json:"status"`
			RoomID     string `json:"roomId"`
			EnterToken string `json:"enterToken"`
		}
		if err := getJSON(c.opt.baseURL+"/api/match/status?matchId="+c.matchID, c.accessToken, &out); err != nil {
			return fmt.Errorf("client %d match status: %w", c.index, err)
		}
		if out.Status == "MATCHED" {
			c.roomID = out.RoomID
			c.enterToken = out.EnterToken
			c.m.matches.Add(1)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("client %d did not reach MATCHED", c.index)
}

func (c *loadClient) enterInitial() error {
	ws, err := dial(c.opt.wsURL)
	if err != nil {
		return fmt.Errorf("client %d dial: %w", c.index, err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(6 * time.Second))
	if err := ws.WriteJSON(protocol.Envelope{
		Type: protocol.TypeEnterRoom,
		Seq:  1,
		Data: protocol.MustMarshal(protocol.EnterRoomData{RoomID: c.roomID, UserID: c.userID, EnterToken: c.enterToken}),
	}); err != nil {
		_ = ws.Close()
		return fmt.Errorf("client %d ENTER_ROOM write: %w", c.index, err)
	}
	for {
		var env protocol.Envelope
		if err := ws.ReadJSON(&env); err != nil {
			_ = ws.Close()
			return fmt.Errorf("client %d ENTER_ROOM read: %w", c.index, err)
		}
		if env.Type != protocol.TypeEnterRoomResult {
			continue
		}
		var out protocol.EnterRoomResultData
		if err := json.Unmarshal(env.Data, &out); err != nil {
			_ = ws.Close()
			return err
		}
		if !out.Success || out.ReconnectToken == "" {
			_ = ws.Close()
			return fmt.Errorf("client %d ENTER_ROOM failed: %+v", c.index, out)
		}
		c.reconnectToken = out.ReconnectToken
		break
	}
	_ = ws.SetReadDeadline(time.Time{})
	c.mu.Lock()
	c.conn = ws
	c.seq = 1
	c.lastSnapshotSeq = 0
	c.needsFull = false
	c.mu.Unlock()
	c.m.connects.Add(1)
	return nil
}

func (c *loadClient) startReader(ws *websocket.Conn) {
	if ws == nil {
		return
	}
	go c.readerLoop(ws)
}

func (c *loadClient) readerLoop(ws *websocket.Conn) {
	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			c.mu.RLock()
			stillActive := c.conn == ws
			c.mu.RUnlock()
			if stillActive {
				c.m.unexpectedReadError.Add(1)
			}
			return
		}
		c.m.bytesIn.Add(int64(len(payload)))
		var env protocol.Envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			continue
		}
		switch env.Type {
		case protocol.TypeGameStart:
			if c.started.CompareAndSwap(false, true) {
				c.m.gameStarts.Add(1)
			}
		case protocol.TypeRoomSnapshot:
			c.processSnapshot(env.Data, false)
		case protocol.TypeRoomRecover:
			c.processSnapshot(env.Data, true)
		}
	}
}

func (c *loadClient) processSnapshot(data json.RawMessage, recoverFrame bool) {
	c.sleepImpairment()
	if !recoverFrame && c.chance(c.opt.incomingDropRate) {
		c.m.incomingDropped.Add(1)
		return
	}

	var probe struct {
		SnapshotType string `json:"snapshotType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return
	}

	if probe.SnapshotType == protocol.SnapshotAOIDelta {
		var d protocol.AOIDeltaData
		if err := json.Unmarshal(data, &d); err != nil {
			return
		}
		c.m.deltaSnapshots.Add(1)
		c.recordSnapshotLag(d.ServerTime)
		c.mu.Lock()
		if d.BaseSeq != c.lastSnapshotSeq {
			c.m.seqMismatches.Add(1)
			request := !c.needsFull
			c.needsFull = true
			c.mu.Unlock()
			if request {
				_ = c.requestFullSync()
			}
			return
		}
		c.lastSnapshotSeq = d.SnapshotSeq
		c.needsFull = false
		c.mu.Unlock()
		c.markSawSnapshot()
		return
	}

	var f protocol.RoomSnapshotData
	if err := json.Unmarshal(data, &f); err != nil {
		return
	}
	if recoverFrame || f.SnapshotType == protocol.SnapshotAOIRecover || f.SnapshotType == protocol.SnapshotFullRecover {
		c.m.recoverSnapshots.Add(1)
	} else {
		c.m.fullSnapshots.Add(1)
	}
	c.recordSnapshotLag(f.ServerTime)
	c.mu.Lock()
	c.lastSnapshotSeq = f.SnapshotSeq
	c.needsFull = false
	c.mu.Unlock()
	c.markSawSnapshot()
}

func (c *loadClient) markSawSnapshot() {
	if c.sawSnapshot.CompareAndSwap(false, true) {
		c.m.snapshotClients.Add(1)
	}
}

func (c *loadClient) recordSnapshotLag(serverTime int64) {
	if serverTime <= 0 {
		return
	}
	c.m.addSnapshotLag(time.Duration(time.Now().UnixMilli()-serverTime) * time.Millisecond)
}

func (c *loadClient) moveLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(c.opt.moveInterval)
	defer ticker.Stop()
	step := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			step++
			if c.chance(c.opt.outgoingDropRate) {
				c.m.movesDropped.Add(1)
				continue
			}
			c.sleepImpairment()
			dir := math.Mod(float64(c.index)*0.61803398875+float64(step)*0.17, 2*math.Pi)
			if err := c.send(protocol.TypeMove, protocol.InputData{Direction: dir, ClientTime: time.Now().UnixMilli()}); err != nil {
				// During an intentional reconnect window the client is expected to have no active socket.
				if !errors.Is(err, errDisconnected) {
					c.m.writeErrors.Add(1)
				}
				continue
			}
			c.m.movesSent.Add(1)
		}
	}
}

var errDisconnected = errors.New("client has no active websocket")

func (c *loadClient) send(typ string, data any) error {
	c.mu.Lock()
	ws := c.conn
	if ws == nil {
		c.mu.Unlock()
		return errDisconnected
	}
	c.seq++
	seq := c.seq
	c.mu.Unlock()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return ws.WriteJSON(protocol.Envelope{Type: typ, Seq: seq, Data: protocol.MustMarshal(data)})
}

func (c *loadClient) sendControl(typ string, data any) error {
	c.sleepImpairment()
	return c.send(typ, data)
}

func (c *loadClient) requestFullSync() error {
	c.m.fullSyncRequests.Add(1)
	return c.sendControl(protocol.TypeFullSync, struct{}{})
}

func (c *loadClient) forceReconnect() {
	if !c.reconnecting.CompareAndSwap(false, true) {
		return
	}
	defer c.reconnecting.Store(false)
	c.m.forcedDisconnects.Add(1)
	c.disconnect()
	if c.opt.reconnectJitter > 0 {
		time.Sleep(c.randomDuration(c.opt.reconnectJitter))
	}
	c.m.reconnectAttempts.Add(1)
	start := time.Now()
	if err := c.reconnect(); err != nil {
		c.m.reconnectFailures.Add(1)
		return
	}
	c.m.reconnectSuccess.Add(1)
	c.m.addReconnectLatency(time.Since(start))
}

func (c *loadClient) disconnect() {
	c.mu.Lock()
	ws := c.conn
	c.conn = nil
	c.mu.Unlock()
	if ws != nil {
		_ = ws.Close()
	}
}

func (c *loadClient) reconnect() error {
	ws, err := dial(c.opt.wsURL)
	if err != nil {
		return err
	}
	deadline := 6*time.Second + 2*c.opt.latency + 2*c.opt.jitter
	_ = ws.SetReadDeadline(time.Now().Add(deadline))
	if err := ws.WriteJSON(protocol.Envelope{
		Type: protocol.TypeReconnect,
		Seq:  1,
		Data: protocol.MustMarshal(protocol.ReconnectData{RoomID: c.roomID, UserID: c.userID, ReconnectToken: c.reconnectToken}),
	}); err != nil {
		_ = ws.Close()
		return err
	}

	var gotResult bool
	var recoverSeq int64
	for !(gotResult && recoverSeq > 0) {
		var env protocol.Envelope
		if err := ws.ReadJSON(&env); err != nil {
			_ = ws.Close()
			return err
		}
		switch env.Type {
		case protocol.TypeReconnectResult:
			var out protocol.ReconnectResultData
			if err := json.Unmarshal(env.Data, &out); err != nil {
				_ = ws.Close()
				return err
			}
			if !out.Success {
				_ = ws.Close()
				return fmt.Errorf("reconnect rejected: %s/%s", out.Reason, out.Message)
			}
			gotResult = true
		case protocol.TypeRoomRecover:
			var f protocol.RoomSnapshotData
			if err := json.Unmarshal(env.Data, &f); err != nil {
				_ = ws.Close()
				return err
			}
			recoverSeq = f.SnapshotSeq
			c.m.recoverSnapshots.Add(1)
			c.recordSnapshotLag(f.ServerTime)
		}
	}
	_ = ws.SetReadDeadline(time.Time{})
	c.mu.Lock()
	c.conn = ws
	c.seq = 1
	c.lastSnapshotSeq = recoverSeq
	c.needsFull = false
	c.mu.Unlock()
	c.markSawSnapshot()
	c.startReader(ws)
	return nil
}

func (c *loadClient) currentConn() *websocket.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *loadClient) snapshotNeedsRepair() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && (c.needsFull || c.lastSnapshotSeq <= 0)
}

func (c *loadClient) snapshotHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.needsFull && c.lastSnapshotSeq > 0
}

func (c *loadClient) chance(rate float64) bool {
	if rate <= 0 {
		return false
	}
	c.rngMu.Lock()
	v := c.rng.Float64() < rate
	c.rngMu.Unlock()
	return v
}

func (c *loadClient) randomDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	c.rngMu.Lock()
	n := c.rng.Int63n(int64(max) + 1)
	c.rngMu.Unlock()
	return time.Duration(n)
}

func (c *loadClient) sleepImpairment() {
	d := c.opt.latency
	if c.opt.jitter > 0 {
		c.rngMu.Lock()
		delta := time.Duration(c.rng.Int63n(int64(2*c.opt.jitter)+1)) - c.opt.jitter
		c.rngMu.Unlock()
		d += delta
	}
	if d > 0 {
		time.Sleep(d)
	}
}

func buildReport(opt *options, clients []*loadClient, m *metrics, rooms int) report {
	healthy := 0
	for _, c := range clients {
		if c.snapshotHealthy() {
			healthy++
		}
	}
	reconnectRate := 1.0
	attempts := m.reconnectAttempts.Load()
	if attempts > 0 {
		reconnectRate = float64(m.reconnectSuccess.Load()) / float64(attempts)
	}
	healthyRate := float64(healthy) / float64(opt.clients)
	reconnectLatency, snapshotLag := m.latencies()
	return report{
		Clients:                   opt.clients,
		Rooms:                     rooms,
		DurationSeconds:           opt.duration.Seconds(),
		Logins:                    m.logins.Load(),
		Matches:                   m.matches.Load(),
		InitialConnections:        m.connects.Load(),
		GameStarts:                m.gameStarts.Load(),
		SnapshotClients:           m.snapshotClients.Load(),
		FullSnapshots:             m.fullSnapshots.Load(),
		DeltaSnapshots:            m.deltaSnapshots.Load(),
		RecoverSnapshots:          m.recoverSnapshots.Load(),
		IncomingSnapshotsDropped:  m.incomingDropped.Load(),
		SequenceMismatches:        m.seqMismatches.Load(),
		FullSyncRequests:          m.fullSyncRequests.Load(),
		MovesSent:                 m.movesSent.Load(),
		MovesDropped:              m.movesDropped.Load(),
		ForcedDisconnects:         m.forcedDisconnects.Load(),
		ReconnectAttempts:         attempts,
		ReconnectSuccess:          m.reconnectSuccess.Load(),
		ReconnectFailures:         m.reconnectFailures.Load(),
		ReconnectSuccessRate:      reconnectRate,
		HealthySnapshotClients:    healthy,
		SnapshotHealthyRate:       healthyRate,
		UnexpectedReadErrors:      m.unexpectedReadError.Load(),
		WriteErrors:               m.writeErrors.Load(),
		BytesReceived:             m.bytesIn.Load(),
		ReconnectLatencyP50Ms:     percentileMs(reconnectLatency, 0.50),
		ReconnectLatencyP95Ms:     percentileMs(reconnectLatency, 0.95),
		ReconnectLatencyP99Ms:     percentileMs(reconnectLatency, 0.99),
		SnapshotReceiveLagP50Ms:   percentileMs(snapshotLag, 0.50),
		SnapshotReceiveLagP95Ms:   percentileMs(snapshotLag, 0.95),
		SnapshotReceiveLagP99Ms:   percentileMs(snapshotLag, 0.99),
		ConfiguredLatencyMs:       float64(opt.latency.Microseconds()) / 1000,
		ConfiguredJitterMs:        float64(opt.jitter.Microseconds()) / 1000,
		ConfiguredIncomingDropPct: opt.incomingDropRate * 100,
		ConfiguredOutgoingDropPct: opt.outgoingDropRate * 100,
		StormFraction:             opt.stormFraction,
		StormCount:                opt.stormCount,
		ChurnFractionPerSecond:    opt.churnFraction,
	}
}

func printReport(r report) {
	fmt.Println("\n=== WS LOAD / CHAOS REPORT ===")
	fmt.Printf("clients=%d rooms=%d duration=%.1fs gameStarts=%d snapshotHealthy=%d/%d (%.2f%%)\n",
		r.Clients, r.Rooms, r.DurationSeconds, r.GameStarts, r.HealthySnapshotClients, r.Clients, r.SnapshotHealthyRate*100)
	fmt.Printf("snapshots: full=%d delta=%d recover=%d dropped=%d seqMismatch=%d fullSync=%d bytesIn=%d\n",
		r.FullSnapshots, r.DeltaSnapshots, r.RecoverSnapshots, r.IncomingSnapshotsDropped, r.SequenceMismatches, r.FullSyncRequests, r.BytesReceived)
	fmt.Printf("moves: sent=%d dropped=%d; disconnects=%d reconnect=%d/%d (%.2f%%) failures=%d\n",
		r.MovesSent, r.MovesDropped, r.ForcedDisconnects, r.ReconnectSuccess, r.ReconnectAttempts, r.ReconnectSuccessRate*100, r.ReconnectFailures)
	fmt.Printf("reconnect latency ms p50=%.1f p95=%.1f p99=%.1f\n", r.ReconnectLatencyP50Ms, r.ReconnectLatencyP95Ms, r.ReconnectLatencyP99Ms)
	fmt.Printf("snapshot receive lag ms p50=%.1f p95=%.1f p99=%.1f\n", r.SnapshotReceiveLagP50Ms, r.SnapshotReceiveLagP95Ms, r.SnapshotReceiveLagP99Ms)
	fmt.Printf("socket errors: unexpectedRead=%d write=%d\n", r.UnexpectedReadErrors, r.WriteErrors)
	b, _ := json.Marshal(r)
	fmt.Printf("WS_LOAD_CHAOS_SUMMARY %s\n", b)
}

func percentileMs(values []time.Duration, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	idx := int(math.Ceil(float64(len(values))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return float64(values[idx].Microseconds()) / 1000
}

func selectClients(clients []*loadClient, fraction float64, rng *rand.Rand) []*loadClient {
	if fraction <= 0 || len(clients) == 0 {
		return nil
	}
	n := int(math.Ceil(float64(len(clients)) * fraction))
	if n > len(clients) {
		n = len(clients)
	}
	perm := rng.Perm(len(clients))
	out := make([]*loadClient, 0, n)
	for _, idx := range perm[:n] {
		out = append(out, clients[idx])
	}
	return out
}

func roomCount(clients []*loadClient) int {
	rooms := make(map[string]struct{})
	for _, c := range clients {
		rooms[c.roomID] = struct{}{}
	}
	return len(rooms)
}

func closeAll(clients []*loadClient) {
	for _, c := range clients {
		c.disconnect()
	}
}

func parallel(n, limit int, fn func(int) error) error {
	sem := make(chan struct{}, limit)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := fn(i); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	var first error
	count := 0
	for err := range errCh {
		count++
		if first == nil {
			first = err
		}
	}
	if first != nil {
		return fmt.Errorf("%d operation(s) failed; first: %w", count, first)
	}
	return nil
}

func waitForCount(v *atomic.Int64, want int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := v.Load(); got >= want {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("got %d, want %d", v.Load(), want)
}

func dial(wsURL string) (*websocket.Conn, error) {
	d := websocket.Dialer{HandshakeTimeout: 6 * time.Second}
	ws, _, err := d.Dial(wsURL, nil)
	return ws, err
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

func postJSON(url, token string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doJSON(req, out)
}

func getJSON(url, token string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doJSON(req, out)
}

func doJSON(req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	if env.Code != 0 {
		return fmt.Errorf("%s %s -> code=%d message=%s", req.Method, req.URL, env.Code, env.Message)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return err
		}
	}
	return nil
}

func waitHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("health check did not succeed: %s", url)
}
