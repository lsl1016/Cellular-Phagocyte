package gateway

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/protocol"
)

const (
	reliableBuffer = 256
	writeWait      = 5 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = 25 * time.Second
)

// wsConn 把 gorilla websocket 适配成 game.Conn。
//
// 消息分为两类：
//   - Send: 可靠控制消息。保持顺序；队列溢出时主动断开慢客户端，由重连机制恢复，绝不静默丢弃。
//   - SendSnapshot: 高频 ROOM_SNAPSHOT。只保留尚未写出的最新逻辑状态。AOI_DELTA 不直接丢弃中间帧，
//     而是在覆盖时组合成“原 base -> 最新状态”的累计 DELTA；若待发送的是 FULL，则把 DELTA 折叠进 FULL。
type wsConn struct {
	ws *websocket.Conn

	reliable chan protocol.Envelope

	snapshotMu     sync.Mutex
	latestSnapshot protocol.Envelope
	hasSnapshot    bool
	snapshotReady  chan struct{}

	done      chan struct{}
	closeOnce sync.Once
}

func newWSConn(ws *websocket.Conn) *wsConn {
	return &wsConn{
		ws:            ws,
		reliable:      make(chan protocol.Envelope, reliableBuffer),
		snapshotReady: make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
}

// Send 非阻塞地排队可靠控制消息。
// 如果客户端持续写不出去导致可靠队列耗尽，直接关闭连接，避免把 GAME_END、
// SETTLEMENT_RESULT、RECONNECT_RESULT 等关键消息静默丢掉。
func (c *wsConn) Send(env protocol.Envelope) {
	select {
	case <-c.done:
		return
	default:
	}

	select {
	case c.reliable <- env:
	case <-c.done:
		return
	default:
		c.Close()
	}
}

// SendSnapshot 非阻塞地更新待发送的最新快照。
// FULL 可以被更新到最新状态；连续 DELTA 会在内存中组合，保证 next.baseSeq 永远指向客户端实际可达的基线。
func (c *wsConn) SendSnapshot(env protocol.Envelope) {
	select {
	case <-c.done:
		return
	default:
	}

	c.snapshotMu.Lock()
	if c.hasSnapshot {
		env = coalesceSnapshots(c.latestSnapshot, env)
	}
	c.latestSnapshot = env
	c.hasSnapshot = true
	c.snapshotMu.Unlock()

	// 通知只需要一个：真正写出时会读取当时最新的逻辑快照。
	select {
	case c.snapshotReady <- struct{}{}:
	default:
	}
}

type snapshotMeta struct {
	SnapshotType string `json:"snapshotType"`
}

// coalesceSnapshots 合并尚未写出的连续 ROOM_SNAPSHOT。
// 失败时优先保留 next 的最新权威状态；客户端若检测到 baseSeq 不连续会主动请求 FULL_SYNC。
func coalesceSnapshots(previous, next protocol.Envelope) protocol.Envelope {
	if previous.Type != protocol.TypeRoomSnapshot || next.Type != protocol.TypeRoomSnapshot {
		return next
	}

	var prevMeta, nextMeta snapshotMeta
	if err := json.Unmarshal(previous.Data, &prevMeta); err != nil {
		return next
	}
	if err := json.Unmarshal(next.Data, &nextMeta); err != nil {
		return next
	}

	prevDelta := prevMeta.SnapshotType == protocol.SnapshotAOIDelta
	nextDelta := nextMeta.SnapshotType == protocol.SnapshotAOIDelta

	switch {
	case !prevDelta && !nextDelta:
		return mergeFullSnapshotEvents(previous, next)
	case !prevDelta && nextDelta:
		return foldDeltaIntoFull(previous, next)
	case prevDelta && nextDelta:
		return composeDeltaSnapshots(previous, next)
	case prevDelta && !nextDelta:
		return mergeDeltaEventsIntoFull(previous, next)
	default:
		return next
	}
}

func mergeFullSnapshotEvents(previous, next protocol.Envelope) protocol.Envelope {
	var prevData, nextData protocol.RoomSnapshotData
	if err := json.Unmarshal(previous.Data, &prevData); err != nil {
		return next
	}
	if err := json.Unmarshal(next.Data, &nextData); err != nil {
		return next
	}
	if len(prevData.Events) > 0 {
		nextData.Events = append(append([]protocol.SnapshotEvent{}, prevData.Events...), nextData.Events...)
	}
	if data, err := json.Marshal(nextData); err == nil {
		next.Data = data
	}
	return next
}

// foldDeltaIntoFull 处理“FULL 还没写出，下一帧 DELTA 已到”的情况。
// 直接在 FULL 上应用 DELTA，最终仍发送一份自包含 FULL，避免客户端依赖从未到达的 base。
func foldDeltaIntoFull(previous, next protocol.Envelope) protocol.Envelope {
	var full protocol.RoomSnapshotData
	var delta protocol.AOIDeltaData
	if err := json.Unmarshal(previous.Data, &full); err != nil {
		return next
	}
	if err := json.Unmarshal(next.Data, &delta); err != nil {
		return next
	}
	if delta.BaseSeq != full.SnapshotSeq {
		return next
	}

	players := make(map[string]protocol.SnapshotPlayer, len(full.Players))
	for _, p := range full.Players {
		players[p.UserID] = p
	}
	foods := make(map[string]protocol.SnapshotFood, len(full.Foods))
	for _, f := range full.Foods {
		foods[f.FoodID] = f
	}
	ejected := make(map[string]protocol.SnapshotEjected, len(full.Ejected))
	for _, e := range full.Ejected {
		ejected[e.EjectID] = e
	}

	for _, p := range delta.Entered.Players {
		players[p.UserID] = p
	}
	for _, p := range delta.Updated.Players {
		players[p.UserID] = p
	}
	for _, f := range delta.Entered.Foods {
		foods[f.FoodID] = f
	}
	for _, f := range delta.Updated.Foods {
		foods[f.FoodID] = f
	}
	for _, e := range delta.Entered.Ejected {
		ejected[e.EjectID] = e
	}
	for _, e := range delta.Updated.Ejected {
		ejected[e.EjectID] = e
	}
	removeSnapshotIDs(players, delta.Left.PlayerIDs, delta.Deleted.PlayerIDs)
	removeSnapshotIDs(foods, delta.Left.FoodIDs, delta.Deleted.FoodIDs)
	removeSnapshotIDs(ejected, delta.Left.EjectedIDs, delta.Deleted.EjectedIDs)

	full.SnapshotType = protocol.SnapshotAOIFull
	full.SnapshotSeq = delta.SnapshotSeq
	full.TickSeq = delta.TickSeq
	full.ServerTime = delta.ServerTime
	full.Players = sortedMapValues(players, func(v protocol.SnapshotPlayer) string { return v.UserID })
	full.Foods = sortedMapValues(foods, func(v protocol.SnapshotFood) string { return v.FoodID })
	full.Ejected = sortedMapValues(ejected, func(v protocol.SnapshotEjected) string { return v.EjectID })
	full.Events = append(append([]protocol.SnapshotEvent{}, full.Events...), delta.Events...)
	if data, err := json.Marshal(full); err == nil {
		next.Data = data
	}
	return next
}

func mergeDeltaEventsIntoFull(previous, next protocol.Envelope) protocol.Envelope {
	var delta protocol.AOIDeltaData
	var full protocol.RoomSnapshotData
	if err := json.Unmarshal(previous.Data, &delta); err != nil {
		return next
	}
	if err := json.Unmarshal(next.Data, &full); err != nil {
		return next
	}
	if len(delta.Events) > 0 {
		full.Events = append(append([]protocol.SnapshotEvent{}, delta.Events...), full.Events...)
	}
	if data, err := json.Marshal(full); err == nil {
		next.Data = data
	}
	return next
}

func composeDeltaSnapshots(previous, next protocol.Envelope) protocol.Envelope {
	var a, b protocol.AOIDeltaData
	if err := json.Unmarshal(previous.Data, &a); err != nil {
		return next
	}
	if err := json.Unmarshal(next.Data, &b); err != nil {
		return next
	}
	if b.BaseSeq != a.SnapshotSeq {
		return next
	}

	out := b
	out.BaseSeq = a.BaseSeq
	out.Events = append(append([]protocol.SnapshotEvent{}, a.Events...), b.Events...)
	out.Entered.Players, out.Updated.Players, out.Left.PlayerIDs, out.Deleted.PlayerIDs = composeObjectDelta(
		a.Entered.Players, a.Updated.Players, a.Left.PlayerIDs, a.Deleted.PlayerIDs,
		b.Entered.Players, b.Updated.Players, b.Left.PlayerIDs, b.Deleted.PlayerIDs,
		func(v protocol.SnapshotPlayer) string { return v.UserID },
	)
	out.Entered.Foods, out.Updated.Foods, out.Left.FoodIDs, out.Deleted.FoodIDs = composeObjectDelta(
		a.Entered.Foods, a.Updated.Foods, a.Left.FoodIDs, a.Deleted.FoodIDs,
		b.Entered.Foods, b.Updated.Foods, b.Left.FoodIDs, b.Deleted.FoodIDs,
		func(v protocol.SnapshotFood) string { return v.FoodID },
	)
	out.Entered.Ejected, out.Updated.Ejected, out.Left.EjectedIDs, out.Deleted.EjectedIDs = composeObjectDelta(
		a.Entered.Ejected, a.Updated.Ejected, a.Left.EjectedIDs, a.Deleted.EjectedIDs,
		b.Entered.Ejected, b.Updated.Ejected, b.Left.EjectedIDs, b.Deleted.EjectedIDs,
		func(v protocol.SnapshotEjected) string { return v.EjectID },
	)

	if data, err := json.Marshal(out); err == nil {
		next.Data = data
	}
	return next
}

type objectOpKind uint8

const (
	opPresent objectOpKind = iota + 1
	opLeft
	opDeleted
)

type objectOp[T any] struct {
	basePresent bool
	kind        objectOpKind
	value       T
}

// composeObjectDelta 把 S0->S1 与 S1->S2 两组对象操作合成为 S0->S2。
// 例如“entered 后又 left”最终是 no-op；“left 后重新 entered”对于 S0 则是 updated。
func composeObjectDelta[T any](
	aEntered, aUpdated []T, aLeft, aDeleted []string,
	bEntered, bUpdated []T, bLeft, bDeleted []string,
	idOf func(T) string,
) (entered, updated []T, left, deleted []string) {
	ops := make(map[string]objectOp[T])

	for _, v := range aEntered {
		ops[idOf(v)] = objectOp[T]{basePresent: false, kind: opPresent, value: v}
	}
	for _, v := range aUpdated {
		ops[idOf(v)] = objectOp[T]{basePresent: true, kind: opPresent, value: v}
	}
	for _, id := range aLeft {
		ops[id] = objectOp[T]{basePresent: true, kind: opLeft}
	}
	for _, id := range aDeleted {
		ops[id] = objectOp[T]{basePresent: true, kind: opDeleted}
	}

	applyPresent := func(v T, enteredAtB bool) {
		id := idOf(v)
		if op, ok := ops[id]; ok {
			op.kind = opPresent
			op.value = v
			ops[id] = op
			return
		}
		ops[id] = objectOp[T]{basePresent: !enteredAtB, kind: opPresent, value: v}
	}
	applyAbsent := func(id string, kind objectOpKind) {
		if op, ok := ops[id]; ok {
			if !op.basePresent {
				delete(ops, id)
				return
			}
			op.kind = kind
			ops[id] = op
			return
		}
		ops[id] = objectOp[T]{basePresent: true, kind: kind}
	}

	for _, v := range bEntered {
		applyPresent(v, true)
	}
	for _, v := range bUpdated {
		applyPresent(v, false)
	}
	for _, id := range bLeft {
		applyAbsent(id, opLeft)
	}
	for _, id := range bDeleted {
		applyAbsent(id, opDeleted)
	}

	keys := make([]string, 0, len(ops))
	for id := range ops {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		op := ops[id]
		switch op.kind {
		case opPresent:
			if op.basePresent {
				updated = append(updated, op.value)
			} else {
				entered = append(entered, op.value)
			}
		case opLeft:
			left = append(left, id)
		case opDeleted:
			deleted = append(deleted, id)
		}
	}
	return
}

func removeSnapshotIDs[T any](items map[string]T, groups ...[]string) {
	for _, ids := range groups {
		for _, id := range ids {
			delete(items, id)
		}
	}
}

func sortedMapValues[T any](items map[string]T, idOf func(T) string) []T {
	out := make([]T, 0, len(items))
	for _, v := range items {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return idOf(out[i]) < idOf(out[j]) })
	return out
}

func (c *wsConn) takeLatestSnapshot() (protocol.Envelope, bool) {
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if !c.hasSnapshot {
		return protocol.Envelope{}, false
	}
	env := c.latestSnapshot
	c.hasSnapshot = false
	return env, true
}

// Close 仅终止连接一次。
func (c *wsConn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.ws != nil {
			_ = c.ws.Close()
		}
	})
}

func (c *wsConn) writeEnvelope(env protocol.Envelope) bool {
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.ws.WriteJSON(env); err != nil {
		c.Close()
		return false
	}
	return true
}

// writePump 串行写 WebSocket。可靠控制消息优先于可覆盖的快照消息。
func (c *wsConn) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		// 先快速排空一个可靠消息，避免快照持续就绪时抢占控制消息。
		select {
		case <-c.done:
			return
		case env := <-c.reliable:
			if !c.writeEnvelope(env) {
				return
			}
			continue
		default:
		}

		select {
		case <-c.done:
			return
		case env := <-c.reliable:
			if !c.writeEnvelope(env) {
				return
			}
		case <-c.snapshotReady:
			if env, ok := c.takeLatestSnapshot(); ok {
				if !c.writeEnvelope(env) {
					return
				}
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		}
	}
}
