package game

import (
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/idgen"
	"cellular-phagocyte/server/internal/protocol"
)

// 房间状态取值。
const (
	RoomLoading   = "LOADING"
	RoomCountdown = "COUNTDOWN"
	RoomRunning   = "RUNNING"
	RoomSettling  = "SETTLING"
	RoomFinished  = "FINISHED"
)

var foodColors = []string{"red", "green", "blue", "yellow", "purple", "orange"}

// Room 是一个内存中的游戏实例，拥有自己的 Tick 循环。
type Room struct {
	mu  sync.Mutex
	mgr *Manager
	log *slog.Logger
	cfg config.GameConfig

	id      string
	matchID string
	mode    string
	status  string

	players map[string]*Player
	order   []string // 稳定的真人+机器人顺序，用于排名/快照
	foods   map[string]*Food

	tickSeq     int64
	snapshotSeq int64
	startTimeMs int64
	endTimeMs   int64

	started  bool
	finished bool
	nextBall int

	pendingEvents []protocol.SnapshotEvent
}

func newRoom(id, matchID, mode string, cfg config.GameConfig, mgr *Manager, log *slog.Logger) *Room {
	return &Room{
		mgr:     mgr,
		log:     log,
		cfg:     cfg,
		id:      id,
		matchID: matchID,
		mode:    mode,
		status:  RoomLoading,
		players: make(map[string]*Player),
		foods:   make(map[string]*Food),
	}
}

func (r *Room) addHuman(p PlayerInfo) {
	r.players[p.UserID] = &Player{
		UserID:   p.UserID,
		Nickname: p.Nickname,
		Status:   StatusMatched,
	}
	r.order = append(r.order, p.UserID)
}

func (r *Room) addBots(n int) {
	for i := 0; i < n; i++ {
		id := "bot_" + idgen.Short()
		p := &Player{
			UserID:   id,
			Nickname: "Bot-" + id[4:8],
			IsBot:    true,
			Status:   StatusPlaying,
		}
		p.Balls = []*Ball{r.newBall(r.cfg.BotInitialMass)}
		p.MaxMass = r.cfg.BotInitialMass
		r.players[id] = p
		r.order = append(r.order, id)
	}
}

func (r *Room) newBall(mass float64) *Ball {
	r.nextBall++
	radius := Radius(mass, r.cfg.RadiusFactor)
	return &Ball{
		BallID: "b_" + idgen.Short(),
		X:      radius + rand.Float64()*(r.cfg.MapWidth-2*radius),
		Y:      radius + rand.Float64()*(r.cfg.MapHeight-2*radius),
		Mass:   mass,
		Radius: radius,
	}
}

func (r *Room) spawnInitialFood() {
	for i := 0; i < r.cfg.InitialFoodCount; i++ {
		r.spawnOneFood()
	}
}

func (r *Room) spawnOneFood() {
	f := &Food{
		ID:    "f_" + idgen.Short(),
		X:     rand.Float64() * r.cfg.MapWidth,
		Y:     rand.Float64() * r.cfg.MapHeight,
		Mass:  r.cfg.FoodMass,
		Color: foodColors[rand.Intn(len(foodColors))],
	}
	r.foods[f.ID] = f
}

// ---- 网关调用的接口 ----

// AttachConn 在真人玩家通过有效的 ENTER_ROOM 后绑定其 WebSocket 连接。
func (r *Room) AttachConn(userID string, conn Conn) (*protocol.EnterRoomResultData, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[userID]
	if !ok {
		return &protocol.EnterRoomResultData{Success: false, ErrorCode: 30012, Message: "用户不属于该房间"}, false
	}
	if r.finished {
		return &protocol.EnterRoomResultData{Success: false, ErrorCode: 30014, Message: "房间已结束"}, false
	}
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn = conn
	p.Entered = true
	if p.Status == StatusMatched {
		p.Status = StatusReady // 已入房，等待 READY
	}
	return &protocol.EnterRoomResultData{
		Success:    true,
		RoomID:     r.id,
		Status:     r.status,
		ServerTime: time.Now().UnixMilli(),
	}, true
}

// MarkReady 标记某个真人玩家已准备；当所有已入房的真人都准备好后开始倒计时。
func (r *Room) MarkReady(userID string) {
	r.mu.Lock()
	p, ok := r.players[userID]
	if !ok || p.IsBot {
		r.mu.Unlock()
		return
	}
	p.Ready = true
	readyCount, playerCount := r.humanReadyCountLocked()
	r.broadcastLocked(protocol.Envelope{
		Type: protocol.TypePlayerReady,
		Data: protocol.MustMarshal(protocol.PlayerReadyData{
			RoomID: r.id, UserID: userID, ReadyCount: readyCount, PlayerCount: playerCount,
		}),
	})
	allReady := r.allEnteredHumansReadyLocked()
	r.mu.Unlock()

	if allReady {
		r.startCountdown()
	}
}

// SubmitInput 记录某个玩家最新的移动方向。
func (r *Room) SubmitInput(userID string, seq int64, dir float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[userID]
	if !ok || !p.alive() {
		return
	}
	if seq != 0 && seq < p.lastInputSeq {
		return // 过期输入
	}
	p.lastInputSeq = seq
	d := dir
	p.pendingDir = &d
}

// Disconnect 标记真人玩家断线，但其球体仍保留在地图上（MVP）。
func (r *Room) Disconnect(userID string, conn Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[userID]
	if !ok || p.conn != conn {
		return
	}
	p.conn = nil
	if p.alive() {
		p.Status = StatusDisconnected
	}
}

func (r *Room) humanReadyCountLocked() (ready, total int) {
	for _, id := range r.order {
		p := r.players[id]
		if p.IsBot || !p.Entered {
			continue
		}
		total++
		if p.Ready {
			ready++
		}
	}
	return
}

func (r *Room) allEnteredHumansReadyLocked() bool {
	entered := 0
	for _, id := range r.order {
		p := r.players[id]
		if p.IsBot || !p.Entered {
			continue
		}
		entered++
		if !p.Ready {
			return false
		}
	}
	return entered > 0
}

// startReadyWatchdog 在真人玩家长时间未准备时强制开始（或销毁）房间。
func (r *Room) startReadyWatchdog() {
	go func() {
		time.Sleep(15 * time.Second)
		r.mu.Lock()
		if r.started || r.finished {
			r.mu.Unlock()
			return
		}
		anyEntered := false
		for _, id := range r.order {
			if p := r.players[id]; !p.IsBot && p.Entered {
				anyEntered = true
				break
			}
		}
		r.mu.Unlock()
		if anyEntered {
			r.startCountdown()
		} else {
			r.endImmediately()
		}
	}()
}

func (r *Room) startCountdown() {
	r.mu.Lock()
	if r.started || r.finished {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.status = RoomCountdown
	countdown := r.cfg.CountdownSeconds
	if countdown < 0 {
		countdown = 0
	}
	startAt := time.Now().Add(time.Duration(countdown) * time.Second).UnixMilli()
	r.broadcastLocked(protocol.Envelope{
		Type: protocol.TypeStartCountdown,
		Data: protocol.MustMarshal(protocol.CountdownData{
			RoomID: r.id, CountdownSeconds: countdown, ServerStartTime: startAt,
		}),
	})
	r.mu.Unlock()

	go func() {
		time.Sleep(time.Duration(countdown) * time.Second)
		r.beginRunning()
	}()
}

func (r *Room) beginRunning() {
	r.mu.Lock()
	if r.finished {
		r.mu.Unlock()
		return
	}
	now := time.Now().UnixMilli()
	r.status = RoomRunning
	r.startTimeMs = now
	r.endTimeMs = now + int64(r.cfg.BattleDurationSeconds)*1000

	for _, id := range r.order {
		p := r.players[id]
		if p.IsBot {
			continue
		}
		if p.Entered {
			p.Status = StatusPlaying
			p.Balls = []*Ball{r.newBall(r.cfg.PlayerInitialMass)}
			p.MaxMass = r.cfg.PlayerInitialMass
		} else {
			p.Status = StatusExited
		}
	}

	r.broadcastLocked(protocol.Envelope{
		Type:       protocol.TypeGameStart,
		ServerTime: now,
		Data: protocol.MustMarshal(protocol.GameStartData{
			RoomID: r.id, ServerTime: now, BattleDurationSeconds: r.cfg.BattleDurationSeconds,
		}),
	})
	r.mu.Unlock()

	r.log.Info("room_start", "roomId", r.id)
	go r.tickLoop()
}

// ---- 广播辅助函数 ----

func (r *Room) broadcastLocked(env protocol.Envelope) {
	for _, id := range r.order {
		p := r.players[id]
		if p.conn != nil {
			p.conn.Send(env)
		}
	}
}
