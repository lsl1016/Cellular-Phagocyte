package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/app"
	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/protocol"
)

// testConfig 缩短所有时间参数，使完整闭环在几秒内跑完。
func testConfig() config.Config {
	c := config.Default()
	c.Game.CountdownSeconds = 0
	c.Game.BattleDurationSeconds = 2
	c.Game.BotFillCount = 3
	c.Game.InitialFoodCount = 60
	c.Game.MaxFoodCount = 80
	c.Game.MapWidth = 1200
	c.Game.MapHeight = 1200
	c.Match.MinStartPlayers = 1
	c.Match.MaxWaitSeconds = 1
	c.Match.ScanIntervalMs = 200
	return c
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestFullClosedLoop 用内存存储跑完整闭环。
func TestFullClosedLoop(t *testing.T) {
	a, err := app.New(testConfig(), quietLogger())
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	runClosedLoop(t, a)
}

// TestFullClosedLoopRedis 用 Redis 存储（miniredis）跑完整闭环，验证后端可切换。
func TestFullClosedLoopRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := testConfig()
	cfg.Storage = config.StorageRedis
	cfg.RedisAddr = mr.Addr()
	a, err := app.New(cfg, quietLogger())
	if err != nil {
		t.Fatalf("app.New(redis): %v", err)
	}
	runClosedLoop(t, a)
}

func runClosedLoop(t *testing.T, a *app.App) {
	srv := httptest.NewServer(a.Handler)
	defer srv.Close()

	// 1. 游客登录
	var login struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			UserID string `json:"userId"`
		} `json:"user"`
	}
	postJSON(t, srv.URL+"/api/auth/guest-login", "", map[string]any{"deviceId": "dev-1"}, &login)
	if login.AccessToken == "" || login.User.UserID == "" {
		t.Fatal("guest login did not return token/userId")
	}
	token := login.AccessToken

	// 2. 开始匹配
	var startData struct {
		MatchID string `json:"matchId"`
	}
	postJSON(t, srv.URL+"/api/match/start", token, map[string]any{"mode": "classic"}, &startData)
	if startData.MatchID == "" {
		t.Fatal("match start did not return matchId")
	}

	// 3. 轮询直到匹配成功
	var roomID, enterToken string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var st struct {
			Status     string `json:"status"`
			RoomID     string `json:"roomId"`
			EnterToken string `json:"enterToken"`
		}
		getJSON(t, srv.URL+"/api/match/status?matchId="+startData.MatchID, token, &st)
		if st.Status == "MATCHED" {
			roomID, enterToken = st.RoomID, st.EnterToken
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if roomID == "" || enterToken == "" {
		t.Fatal("match never reached MATCHED")
	}

	// 4. 建立 WebSocket 连接
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer ws.Close()

	// 5. ENTER_ROOM 握手
	writeEnv(t, ws, protocol.Envelope{
		Type: protocol.TypeEnterRoom, Seq: 1,
		Data: protocol.MustMarshal(protocol.EnterRoomData{
			RoomID: roomID, UserID: login.User.UserID, EnterToken: enterToken,
		}),
	})

	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	res := readUntil(t, ws, protocol.TypeEnterRoomResult)
	var erd protocol.EnterRoomResultData
	mustData(t, res, &erd)
	if !erd.Success {
		t.Fatalf("enter room failed: %+v", erd)
	}

	// 6. READY
	writeEnv(t, ws, protocol.Envelope{
		Type: protocol.TypeReady, Seq: 2,
		Data: protocol.MustMarshal(protocol.ReadyData{RoomID: roomID, UserID: login.User.UserID}),
	})

	// 7. 对局循环：期望收到 GAME_START、快照，最后是 SETTLEMENT_RESULT
	var gotGameStart, gotSnapshot, sentMove, gotSettlement bool
	var settlement protocol.SettlementResultData

	_ = ws.SetReadDeadline(time.Now().Add(15 * time.Second))
	for !gotSettlement {
		var env protocol.Envelope
		if err := ws.ReadJSON(&env); err != nil {
			t.Fatalf("read loop ended early (gameStart=%v snapshot=%v): %v", gotGameStart, gotSnapshot, err)
		}
		switch env.Type {
		case protocol.TypeGameStart:
			gotGameStart = true
		case protocol.TypeRoomSnapshot:
			gotSnapshot = true
			if !sentMove {
				sentMove = true
				writeEnv(t, ws, protocol.Envelope{
					Type: protocol.TypeMove, Seq: 3,
					Data: protocol.MustMarshal(protocol.InputData{Direction: 1.0}),
				})
			}
		case protocol.TypeSettlementResult:
			mustData(t, env, &settlement)
			gotSettlement = true
		}
	}

	if !gotGameStart {
		t.Error("never received GAME_START")
	}
	if !gotSnapshot {
		t.Error("never received ROOM_SNAPSHOT")
	}
	if settlement.Status != "SUCCESS" {
		t.Errorf("settlement status = %q, want SUCCESS", settlement.Status)
	}
	if settlement.CoinReward <= 0 {
		t.Errorf("coinReward = %d, want > 0", settlement.CoinReward)
	}

	// 8. 资产应反映已发放的奖励
	var assets struct {
		Coin int64 `json:"coin"`
		Exp  int64 `json:"exp"`
	}
	getJSON(t, srv.URL+"/api/assets/me", token, &assets)
	if assets.Coin != settlement.CoinReward {
		t.Errorf("asset coin = %d, want %d", assets.Coin, settlement.CoinReward)
	}
	if assets.Exp != settlement.ExpReward {
		t.Errorf("asset exp = %d, want %d", assets.Exp, settlement.ExpReward)
	}
}

// ---- 辅助函数 ----

func postJSON(t *testing.T, url, token string, body any, out any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	doReq(t, req, out)
}

func getJSON(t *testing.T, url, token string, out any) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	doReq(t, req, out)
}

func doReq(t *testing.T, req *http.Request, out any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode %s: %v", req.URL, err)
	}
	if env.Code != 0 {
		t.Fatalf("%s %s -> code %d: %s", req.Method, req.URL, env.Code, env.Message)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("unmarshal data %s: %v", req.URL, err)
		}
	}
}

func writeEnv(t *testing.T, ws *websocket.Conn, env protocol.Envelope) {
	t.Helper()
	if err := ws.WriteJSON(env); err != nil {
		t.Fatalf("ws write %s: %v", env.Type, err)
	}
}

func readUntil(t *testing.T, ws *websocket.Conn, typ string) protocol.Envelope {
	t.Helper()
	for {
		var env protocol.Envelope
		if err := ws.ReadJSON(&env); err != nil {
			t.Fatalf("waiting for %s: %v", typ, err)
		}
		if env.Type == typ {
			return env
		}
	}
}

func mustData(t *testing.T, env protocol.Envelope, out any) {
	t.Helper()
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("unmarshal %s data: %v", env.Type, err)
	}
}
