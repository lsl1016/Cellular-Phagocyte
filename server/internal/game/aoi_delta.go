package game

import (
	"math"

	"cellular-phagocyte/server/internal/protocol"
)

// aoiVisibleEntry 保存上一次逻辑快照中对象的同步字段指纹，以及本次构建是否仍可见。
// seenSeq 直接复用 snapshotSeq 作为 generation marker，避免每个 viewer 每帧再分配 currentVisible map。
type aoiVisibleEntry struct {
	fingerprint uint64
	seenSeq     int64
}

// aoiClientState 记录某个客户端上一份“逻辑已发送”AOI 状态。
// latest-only 通道若覆盖尚未写出的 DELTA，会在 gateway 层把增量继续合并，
// 因而这里可以按每次生成的 snapshotSeq 连续推进。
type aoiClientState struct {
	initialized bool
	lastSeq     int64
	lastFullSeq int64
	forceFull   bool
	players     map[string]aoiVisibleEntry
	foods       map[string]int64 // value = 最近一次可见的 snapshotSeq；食物静止，无需状态指纹
	ejected     map[string]aoiVisibleEntry
}

func newAOIClientState() *aoiClientState {
	return &aoiClientState{
		players: make(map[string]aoiVisibleEntry),
		foods:   make(map[string]int64),
		ejected: make(map[string]aoiVisibleEntry),
	}
}

func (r *Room) aoiStateLocked(userID string) *aoiClientState {
	if r.aoiStates == nil {
		r.aoiStates = make(map[string]*aoiClientState)
	}
	state := r.aoiStates[userID]
	if state == nil {
		state = newAOIClientState()
		r.aoiStates[userID] = state
	}
	return state
}

func (r *Room) resetAOIStateLocked(userID string) {
	if r.aoiStates != nil {
		delete(r.aoiStates, userID)
	}
}

// RequestFullSync 由客户端在发现 baseSeq 不连续时调用。
// 不直接在读协程中构建快照，只标记下一次 10Hz snapshot 周期发送 AOI_FULL，
// 避免和权威 Tick 的快照构建形成第二条并发路径。
func (r *Room) RequestFullSync(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.cfg.AOIEnabled {
		return
	}
	if p := r.players[userID]; p == nil || p.conn == nil {
		return
	}
	r.aoiStateLocked(userID).forceFull = true
}

func (r *Room) aoiFullIntervalSnapshotsLocked() int64 {
	rate := r.cfg.SnapshotRate
	if rate <= 0 {
		rate = 1
	}
	seconds := r.cfg.AOIFullSnapshotIntervalSeconds
	if seconds <= 0 {
		seconds = 5
	}
	return int64(rate * seconds)
}

func (r *Room) shouldSendAOIFullLocked(state *aoiClientState) bool {
	if state == nil || !state.initialized || state.forceFull {
		return true
	}
	return r.snapshotSeq-state.lastFullSeq >= r.aoiFullIntervalSnapshotsLocked()
}

// rememberAOIFullLocked 把刚刚发送的当前可见集合保存为下一份 DELTA 的比较基线。
func (r *Room) rememberAOIFullLocked(state *aoiClientState, full protocol.RoomSnapshotData, isFull bool) {
	clear(state.players)
	clear(state.foods)
	clear(state.ejected)
	for _, p := range full.Players {
		state.players[p.UserID] = aoiVisibleEntry{fingerprint: snapshotPlayerFingerprint(p), seenSeq: full.SnapshotSeq}
	}
	for _, f := range full.Foods {
		state.foods[f.FoodID] = full.SnapshotSeq
	}
	for _, e := range full.Ejected {
		state.ejected[e.EjectID] = aoiVisibleEntry{fingerprint: snapshotEjectedFingerprint(e), seenSeq: full.SnapshotSeq}
	}
	state.initialized = true
	state.lastSeq = full.SnapshotSeq
	if isFull {
		state.lastFullSeq = full.SnapshotSeq
		state.forceFull = false
	}
}

// aoiDeltaFromFullLocked 将当前 AOI_FULL 与上一份可见状态做集合 + 值差异比较。
// 只有真正变化过的持续可见玩家/吐出物才进入 updated；食物静止，持续可见时永不重复发送。
// 对象从视野消失但仍存在于房间记为 left；已经被吞噬/销毁记为 deleted。
// 本函数会原地把 state 推进到 full.SnapshotSeq，因此不需要再构造 currentVisible 临时 map。
func (r *Room) aoiDeltaFromFullLocked(state *aoiClientState, full protocol.RoomSnapshotData) protocol.AOIDeltaData {
	delta := protocol.AOIDeltaData{
		RoomID:       full.RoomID,
		SnapshotType: protocol.SnapshotAOIDelta,
		SnapshotSeq:  full.SnapshotSeq,
		BaseSeq:      state.lastSeq,
		TickSeq:      full.TickSeq,
		ServerTime:   full.ServerTime,
		Events:       full.Events,
	}

	seq := full.SnapshotSeq
	for _, p := range full.Players {
		fingerprint := snapshotPlayerFingerprint(p)
		entry, existed := state.players[p.UserID]
		if !existed {
			delta.Entered.Players = append(delta.Entered.Players, p)
		} else if entry.fingerprint != fingerprint {
			delta.Updated.Players = append(delta.Updated.Players, p)
		}
		state.players[p.UserID] = aoiVisibleEntry{fingerprint: fingerprint, seenSeq: seq}
	}
	for id, entry := range state.players {
		if entry.seenSeq == seq {
			continue
		}
		if p := r.players[id]; p != nil && p.alive() {
			delta.Left.PlayerIDs = append(delta.Left.PlayerIDs, id)
		} else {
			delta.Deleted.PlayerIDs = append(delta.Deleted.PlayerIDs, id)
		}
		delete(state.players, id)
	}

	for _, f := range full.Foods {
		if _, existed := state.foods[f.FoodID]; !existed {
			delta.Entered.Foods = append(delta.Entered.Foods, f)
		}
		state.foods[f.FoodID] = seq
	}
	for id, seenSeq := range state.foods {
		if seenSeq == seq {
			continue
		}
		if _, exists := r.foods[id]; exists {
			delta.Left.FoodIDs = append(delta.Left.FoodIDs, id)
		} else {
			delta.Deleted.FoodIDs = append(delta.Deleted.FoodIDs, id)
		}
		delete(state.foods, id)
	}

	for _, e := range full.Ejected {
		fingerprint := snapshotEjectedFingerprint(e)
		entry, existed := state.ejected[e.EjectID]
		if !existed {
			delta.Entered.Ejected = append(delta.Entered.Ejected, e)
		} else if entry.fingerprint != fingerprint {
			delta.Updated.Ejected = append(delta.Updated.Ejected, e)
		}
		state.ejected[e.EjectID] = aoiVisibleEntry{fingerprint: fingerprint, seenSeq: seq}
	}
	for id, entry := range state.ejected {
		if entry.seenSeq == seq {
			continue
		}
		if _, exists := r.ejected[id]; exists {
			delta.Left.EjectedIDs = append(delta.Left.EjectedIDs, id)
		} else {
			delta.Deleted.EjectedIDs = append(delta.Deleted.EjectedIDs, id)
		}
		delete(state.ejected, id)
	}

	state.initialized = true
	state.lastSeq = seq
	return delta
}

// 下列指纹只覆盖实际出现在协议里的同步字段，并基于已经 round1 后的值计算。
// 使用无分配的 FNV-1a 风格混合，避免每个 viewer 为 change detection 创建 hasher/string JSON。
const (
	aoiHashOffset uint64 = 1469598103934665603
	aoiHashPrime  uint64 = 1099511628211
)

func snapshotPlayerFingerprint(p protocol.SnapshotPlayer) uint64 {
	h := aoiHashOffset
	h = aoiHashString(h, p.Nickname)
	h = aoiHashString(h, p.Status)
	h = aoiHashUint64(h, uint64(p.Score))
	h = aoiHashUint64(h, math.Float64bits(p.Mass))
	h = aoiHashUint64(h, uint64(len(p.Balls)))
	for _, b := range p.Balls {
		h = aoiHashString(h, b.BallID)
		h = aoiHashUint64(h, math.Float64bits(b.X))
		h = aoiHashUint64(h, math.Float64bits(b.Y))
		h = aoiHashUint64(h, math.Float64bits(b.Radius))
		h = aoiHashUint64(h, math.Float64bits(b.Mass))
	}
	return h
}

func snapshotEjectedFingerprint(e protocol.SnapshotEjected) uint64 {
	h := aoiHashOffset
	h = aoiHashString(h, e.OwnerID)
	h = aoiHashUint64(h, math.Float64bits(e.X))
	h = aoiHashUint64(h, math.Float64bits(e.Y))
	h = aoiHashUint64(h, math.Float64bits(e.Radius))
	h = aoiHashUint64(h, math.Float64bits(e.Mass))
	return h
}

func aoiHashString(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= aoiHashPrime
	}
	// 字段边界，避免 ["ab","c"] 与 ["a","bc"] 产生同样的字节串。
	h ^= 0xff
	h *= aoiHashPrime
	return h
}

func aoiHashUint64(h, v uint64) uint64 {
	for i := 0; i < 8; i++ {
		h ^= v & 0xff
		h *= aoiHashPrime
		v >>= 8
	}
	return h
}
