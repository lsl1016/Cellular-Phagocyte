// Command e2e drives a separately running game server through real HTTP and WebSocket sockets.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/protocol"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type player struct {
	name           string
	userID         string
	accessToken    string
	matchID        string
	roomID         string
	enterToken     string
	reconnectToken string
	sock           *socket
}

type socket struct {
	ws   *websocket.Conn
	recv chan protocol.Envelope
	err  chan error
}

type snapshotFrame struct {
	kind   string
	seq    int64
	base   int64
	full   *protocol.RoomSnapshotData
	delta  *protocol.AOIDeltaData
	events []protocol.SnapshotEvent
}

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:18080", "HTTP base URL")
	wsURL := flag.String("ws-url", "ws://127.0.0.1:18080/ws", "WebSocket URL")
	flag.Parse()

	if err := run(strings.TrimRight(*baseURL, "/"), *wsURL); err != nil {
		fmt.Fprintf(os.Stderr, "REAL_E2E FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("REAL_E2E PASS")
}

func run(baseURL, wsURL string) error {
	if err := waitHealth(baseURL+"/healthz", 10*time.Second); err != nil {
		return err
	}
	fmt.Println("[1/9] server healthz ok")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	a, err := guestLogin(baseURL, "e2e-a-"+suffix, "A")
	if err != nil {
		return err
	}
	b, err := guestLogin(baseURL, "e2e-b-"+suffix, "B")
	if err != nil {
		return err
	}
	fmt.Printf("[2/9] guest login ok: A=%s B=%s\n", a.userID, b.userID)

	if err := startMatch(baseURL, a); err != nil {
		return err
	}
	if err := startMatch(baseURL, b); err != nil {
		return err
	}
	if err := waitMatched(baseURL, a, 8*time.Second); err != nil {
		return err
	}
	if err := waitMatched(baseURL, b, 8*time.Second); err != nil {
		return err
	}
	if a.roomID == "" || a.roomID != b.roomID {
		return fmt.Errorf("players matched into different rooms: A=%s B=%s", a.roomID, b.roomID)
	}
	fmt.Printf("[3/9] two-human matchmaking ok: room=%s\n", a.roomID)

	if err := enter(wsURL, a); err != nil {
		return err
	}
	defer a.close()
	if err := enter(wsURL, b); err != nil {
		return err
	}
	defer b.close()
	if err := a.send(protocol.TypeReady, 2, protocol.ReadyData{RoomID: a.roomID, UserID: a.userID}); err != nil {
		return err
	}
	if err := b.send(protocol.TypeReady, 2, protocol.ReadyData{RoomID: b.roomID, UserID: b.userID}); err != nil {
		return err
	}
	if _, err := a.waitType(protocol.TypeGameStart, 5*time.Second); err != nil {
		return fmt.Errorf("A game start: %w", err)
	}
	if _, err := b.waitType(protocol.TypeGameStart, 5*time.Second); err != nil {
		return fmt.Errorf("B game start: %w", err)
	}
	fmt.Println("[4/9] real WebSocket ENTER_ROOM + READY + GAME_START ok")

	aFull, err := a.waitSnapshotKind(protocol.SnapshotAOIFull, 4*time.Second)
	if err != nil {
		return fmt.Errorf("A initial AOI_FULL: %w", err)
	}
	bFull, err := b.waitSnapshotKind(protocol.SnapshotAOIFull, 4*time.Second)
	if err != nil {
		return fmt.Errorf("B initial AOI_FULL: %w", err)
	}
	aDelta, err := a.waitSnapshotKind(protocol.SnapshotAOIDelta, 4*time.Second)
	if err != nil {
		return fmt.Errorf("A first AOI_DELTA: %w", err)
	}
	if aDelta.base != aFull.seq {
		return fmt.Errorf("A delta base=%d want initial full seq=%d", aDelta.base, aFull.seq)
	}
	if _, err := b.waitSnapshotKind(protocol.SnapshotAOIDelta, 4*time.Second); err != nil {
		return fmt.Errorf("B first AOI_DELTA: %w", err)
	}
	initialX, ok := playerXFromFull(aFull.full, a.userID)
	if !ok {
		return errors.New("A missing from own initial full snapshot")
	}
	peerInitiallySeesA := hasPlayer(bFull.full.Players, a.userID)
	fmt.Printf("[5/9] AOI_FULL -> AOI_DELTA ok: fullSeq=%d deltaSeq=%d baseSeq=%d peerSeesA=%v\n", aFull.seq, aDelta.seq, aDelta.base, peerInitiallySeesA)

	// Use a high MOVE sequence before reconnect. A normal client creates a new WS client after reconnect
	// and its sequence starts from 1 again; the server must accept that fresh sequence space.
	if err := a.send(protocol.TypeMove, 100, protocol.InputData{Direction: 0}); err != nil {
		return err
	}
	eastX, err := a.waitPlayerX(a.userID, initialX, true, 3*time.Second)
	if err != nil {
		return fmt.Errorf("pre-reconnect east move: %w", err)
	}
	fmt.Printf("[6/9] MOVE over real socket changed authoritative position: %.1f -> %.1f\n", initialX, eastX)

	a.close()
	time.Sleep(200 * time.Millisecond)
	if err := reconnect(wsURL, a); err != nil {
		return err
	}
	defer a.close()
	recoverFrame, err := a.waitRecover(4 * time.Second)
	if err != nil {
		return fmt.Errorf("recover snapshot: %w", err)
	}
	if recoverFrame.full.SnapshotType != protocol.SnapshotAOIRecover {
		return fmt.Errorf("recover snapshot type=%s want %s", recoverFrame.full.SnapshotType, protocol.SnapshotAOIRecover)
	}
	recoverX, ok := playerXFromFull(recoverFrame.full, a.userID)
	if !ok {
		return errors.New("A missing from recover snapshot")
	}
	postRecoverDelta, err := a.waitSnapshotKind(protocol.SnapshotAOIDelta, 4*time.Second)
	if err != nil {
		return fmt.Errorf("post-recover delta: %w", err)
	}
	if postRecoverDelta.base != recoverFrame.seq {
		return fmt.Errorf("post-recover delta base=%d want recover seq=%d", postRecoverDelta.base, recoverFrame.seq)
	}

	// Fresh connection deliberately restarts MOVE seq from 1 and reverses direction.
	if err := a.send(protocol.TypeMove, 1, protocol.InputData{Direction: math.Pi}); err != nil {
		return err
	}
	westX, err := a.waitPlayerX(a.userID, recoverX, false, 3*time.Second)
	if err != nil {
		return fmt.Errorf("post-reconnect fresh seq MOVE was not accepted: %w", err)
	}
	fmt.Printf("[7/9] disconnect -> RECONNECT -> AOI_FULL_RECOVER -> DELTA and fresh seq reset ok: %.1f -> %.1f\n", recoverX, westX)

	b.drain()
	if err := a.send(protocol.TypeSplit, 2, protocol.InputData{Direction: math.Pi}); err != nil {
		return err
	}
	if err := a.waitSnapshotEvent("PLAYER_SPLIT", 3*time.Second); err != nil {
		return fmt.Errorf("participant PLAYER_SPLIT event: %w", err)
	}
	if !peerInitiallySeesA {
		if err := b.assertNoSnapshotEvent("PLAYER_SPLIT", 900*time.Millisecond); err != nil {
			return fmt.Errorf("far unrelated peer event routing: %w", err)
		}
		fmt.Println("[8/9] typed event routing ok: participant received PLAYER_SPLIT, far peer filtered")
	} else {
		fmt.Println("[8/9] typed event routing participant path ok; negative far-peer assertion skipped because spawn geometry made A visible to B")
	}

	if err := a.send(protocol.TypeFullSync, 3, struct{}{}); err != nil {
		return err
	}
	fullSync, err := a.waitSnapshotKind(protocol.SnapshotAOIFull, 3*time.Second)
	if err != nil {
		return fmt.Errorf("FULL_SYNC repair: %w", err)
	}
	if fullSync.seq <= recoverFrame.seq {
		return fmt.Errorf("FULL_SYNC seq=%d should advance beyond recover seq=%d", fullSync.seq, recoverFrame.seq)
	}

	settleA, err := a.waitSettlement(15 * time.Second)
	if err != nil {
		return fmt.Errorf("A settlement: %w", err)
	}
	settleB, err := b.waitSettlement(15 * time.Second)
	if err != nil {
		return fmt.Errorf("B settlement: %w", err)
	}
	if settleA.Status != "SUCCESS" || settleB.Status != "SUCCESS" {
		return fmt.Errorf("settlement statuses: A=%s B=%s", settleA.Status, settleB.Status)
	}
	var assets struct {
		Coin int64 `json:"coin"`
		Exp  int64 `json:"exp"`
	}
	if err := getJSON(baseURL+"/api/assets/me", a.accessToken, &assets); err != nil {
		return err
	}
	if assets.Coin < settleA.CoinReward || assets.Exp < settleA.ExpReward {
		return fmt.Errorf("asset grant missing: assets=%+v settlement=%+v", assets, settleA)
	}
	fmt.Printf("[9/9] FULL_SYNC + GAME_END + settlement + asset persistence ok: A coin=%d exp=%d\n", assets.Coin, assets.Exp)
	return nil
}

func guestLogin(baseURL, deviceID, name string) (*player, error) {
	var out struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			UserID string `json:"userId"`
		} `json:"user"`
	}
	if err := postJSON(baseURL+"/api/auth/guest-login", "", map[string]any{"deviceId": deviceID}, &out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" || out.User.UserID == "" {
		return nil, errors.New("guest login returned empty token/userId")
	}
	return &player{name: name, userID: out.User.UserID, accessToken: out.AccessToken}, nil
}

func startMatch(baseURL string, p *player) error {
	var out struct {
		MatchID string `json:"matchId"`
	}
	if err := postJSON(baseURL+"/api/match/start", p.accessToken, map[string]any{"mode": "classic"}, &out); err != nil {
		return err
	}
	if out.MatchID == "" {
		return fmt.Errorf("%s match start returned empty matchId", p.name)
	}
	p.matchID = out.MatchID
	return nil
}

func waitMatched(baseURL string, p *player, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var out struct {
			Status     string `json:"status"`
			RoomID     string `json:"roomId"`
			EnterToken string `json:"enterToken"`
		}
		if err := getJSON(baseURL+"/api/match/status?matchId="+p.matchID, p.accessToken, &out); err != nil {
			return err
		}
		if out.Status == "MATCHED" {
			p.roomID, p.enterToken = out.RoomID, out.EnterToken
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s did not reach MATCHED", p.name)
}

func enter(wsURL string, p *player) error {
	s, err := dial(wsURL)
	if err != nil {
		return fmt.Errorf("%s ws dial: %w", p.name, err)
	}
	p.sock = s
	if err := p.send(protocol.TypeEnterRoom, 1, protocol.EnterRoomData{RoomID: p.roomID, UserID: p.userID, EnterToken: p.enterToken}); err != nil {
		return err
	}
	env, err := p.waitType(protocol.TypeEnterRoomResult, 4*time.Second)
	if err != nil {
		return err
	}
	var out protocol.EnterRoomResultData
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return err
	}
	if !out.Success || out.ReconnectToken == "" {
		return fmt.Errorf("%s ENTER_ROOM failed: %+v", p.name, out)
	}
	p.reconnectToken = out.ReconnectToken
	return nil
}

func reconnect(wsURL string, p *player) error {
	s, err := dial(wsURL)
	if err != nil {
		return fmt.Errorf("%s reconnect dial: %w", p.name, err)
	}
	p.sock = s
	if err := p.send(protocol.TypeReconnect, 1, protocol.ReconnectData{RoomID: p.roomID, UserID: p.userID, ReconnectToken: p.reconnectToken}); err != nil {
		return err
	}
	env, err := p.waitType(protocol.TypeReconnectResult, 4*time.Second)
	if err != nil {
		return err
	}
	var out protocol.ReconnectResultData
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("%s reconnect failed: %+v", p.name, out)
	}
	return nil
}

func dial(wsURL string) (*socket, error) {
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	s := &socket{ws: ws, recv: make(chan protocol.Envelope, 512), err: make(chan error, 1)}
	go func() {
		for {
			var env protocol.Envelope
			if err := ws.ReadJSON(&env); err != nil {
				select {
				case s.err <- err:
				default:
				}
				return
			}
			s.recv <- env
		}
	}()
	return s, nil
}

func (p *player) close() {
	if p.sock != nil && p.sock.ws != nil {
		_ = p.sock.ws.Close()
	}
}

func (p *player) send(typ string, seq int64, data any) error {
	if p.sock == nil {
		return errors.New("socket is nil")
	}
	return p.sock.ws.WriteJSON(protocol.Envelope{Type: typ, Seq: seq, Data: protocol.MustMarshal(data)})
}

func (p *player) next(timeout time.Duration) (protocol.Envelope, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case env := <-p.sock.recv:
		return env, nil
	case err := <-p.sock.err:
		return protocol.Envelope{}, err
	case <-t.C:
		return protocol.Envelope{}, errors.New("timeout waiting for websocket message")
	}
}

func (p *player) waitType(typ string, timeout time.Duration) (protocol.Envelope, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		env, err := p.next(time.Until(deadline))
		if err != nil {
			return protocol.Envelope{}, err
		}
		if env.Type == typ {
			return env, nil
		}
	}
	return protocol.Envelope{}, fmt.Errorf("timeout waiting for %s", typ)
}

func (p *player) waitSnapshotKind(kind string, timeout time.Duration) (*snapshotFrame, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		env, err := p.next(time.Until(deadline))
		if err != nil {
			return nil, err
		}
		if env.Type != protocol.TypeRoomSnapshot {
			continue
		}
		frame, err := decodeSnapshot(env)
		if err != nil {
			return nil, err
		}
		if frame.kind == kind {
			return frame, nil
		}
	}
	return nil, fmt.Errorf("timeout waiting for snapshotType=%s", kind)
}

func (p *player) waitRecover(timeout time.Duration) (*snapshotFrame, error) {
	env, err := p.waitType(protocol.TypeRoomRecover, timeout)
	if err != nil {
		return nil, err
	}
	var full protocol.RoomSnapshotData
	if err := json.Unmarshal(env.Data, &full); err != nil {
		return nil, err
	}
	return &snapshotFrame{kind: full.SnapshotType, seq: full.SnapshotSeq, full: &full, events: full.Events}, nil
}

func decodeSnapshot(env protocol.Envelope) (*snapshotFrame, error) {
	var probe struct {
		SnapshotType string `json:"snapshotType"`
	}
	if err := json.Unmarshal(env.Data, &probe); err != nil {
		return nil, err
	}
	if probe.SnapshotType == protocol.SnapshotAOIDelta {
		var d protocol.AOIDeltaData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			return nil, err
		}
		return &snapshotFrame{kind: d.SnapshotType, seq: d.SnapshotSeq, base: d.BaseSeq, delta: &d, events: d.Events}, nil
	}
	var f protocol.RoomSnapshotData
	if err := json.Unmarshal(env.Data, &f); err != nil {
		return nil, err
	}
	return &snapshotFrame{kind: f.SnapshotType, seq: f.SnapshotSeq, full: &f, events: f.Events}, nil
}

func (p *player) waitPlayerX(userID string, baseline float64, greater bool, timeout time.Duration) (float64, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		env, err := p.next(time.Until(deadline))
		if err != nil {
			return 0, err
		}
		if env.Type != protocol.TypeRoomSnapshot {
			continue
		}
		frame, err := decodeSnapshot(env)
		if err != nil {
			return 0, err
		}
		x, ok := playerXFromFrame(frame, userID)
		if !ok {
			continue
		}
		if greater && x > baseline+2 {
			return x, nil
		}
		if !greater && x < baseline-2 {
			return x, nil
		}
	}
	if greater {
		return 0, fmt.Errorf("x never increased beyond %.1f", baseline)
	}
	return 0, fmt.Errorf("x never decreased below %.1f", baseline)
}

func playerXFromFrame(frame *snapshotFrame, userID string) (float64, bool) {
	if frame.full != nil {
		return playerXFromPlayers(frame.full.Players, userID)
	}
	if frame.delta != nil {
		if x, ok := playerXFromPlayers(frame.delta.Updated.Players, userID); ok {
			return x, true
		}
		return playerXFromPlayers(frame.delta.Entered.Players, userID)
	}
	return 0, false
}

func playerXFromFull(full *protocol.RoomSnapshotData, userID string) (float64, bool) {
	if full == nil {
		return 0, false
	}
	return playerXFromPlayers(full.Players, userID)
}

func playerXFromPlayers(players []protocol.SnapshotPlayer, userID string) (float64, bool) {
	for _, p := range players {
		if p.UserID != userID || len(p.Balls) == 0 {
			continue
		}
		var sum float64
		for _, b := range p.Balls {
			sum += b.X
		}
		return sum / float64(len(p.Balls)), true
	}
	return 0, false
}

func hasPlayer(players []protocol.SnapshotPlayer, userID string) bool {
	for _, p := range players {
		if p.UserID == userID {
			return true
		}
	}
	return false
}

func (p *player) waitSnapshotEvent(eventType string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		env, err := p.next(time.Until(deadline))
		if err != nil {
			return err
		}
		if env.Type != protocol.TypeRoomSnapshot {
			continue
		}
		frame, err := decodeSnapshot(env)
		if err != nil {
			return err
		}
		for _, event := range frame.events {
			if event.Type == eventType {
				return nil
			}
		}
	}
	return fmt.Errorf("timeout waiting for snapshot event %s", eventType)
}

func (p *player) assertNoSnapshotEvent(eventType string, duration time.Duration) error {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		env, err := p.next(time.Until(deadline))
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				return nil
			}
			return err
		}
		if env.Type != protocol.TypeRoomSnapshot {
			continue
		}
		frame, err := decodeSnapshot(env)
		if err != nil {
			return err
		}
		for _, event := range frame.events {
			if event.Type == eventType {
				return fmt.Errorf("unexpected event %s delivered", eventType)
			}
		}
	}
	return nil
}

func (p *player) drain() {
	for {
		select {
		case <-p.sock.recv:
		default:
			return
		}
	}
}

func (p *player) waitSettlement(timeout time.Duration) (*protocol.SettlementResultData, error) {
	env, err := p.waitType(protocol.TypeSettlementResult, timeout)
	if err != nil {
		return nil, err
	}
	var out protocol.SettlementResultData
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func waitHealth(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("health check did not become ready: %s", url)
}

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
	return doJSON(req, token, out)
}

func getJSON(url, token string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return doJSON(req, token, out)
}

func doJSON(req *http.Request, token string, out any) error {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode %s: %w", req.URL, err)
	}
	if env.Code != 0 {
		return fmt.Errorf("%s %s -> code=%d message=%s", req.Method, req.URL, env.Code, env.Message)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode data %s: %w", req.URL, err)
		}
	}
	return nil
}
