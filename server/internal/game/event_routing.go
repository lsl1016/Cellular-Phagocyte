package game

import "cellular-phagocyte/server/internal/protocol"

// snapshotEventRoute 是仅存在于服务端内存中的事件路由元数据。
// 它不会进入 SnapshotEvent JSON；客户端继续消费原来的 {type,data} 结构。
//
// AOI 开启后，事件按以下优先级决定是否发送给某个 viewer：
//  1. viewer 是事件直接参与者：必达；
//  2. 事件关联对象存在于 viewer 上一份逻辑快照的 VisibleSet：发送；
//  3. 事件发生位置落在 viewer 当前 AOI 圆内：发送；
//  4. 其他情况不发送。
//
// Global 仅作为兼容兜底：尚未迁移到 typed route 的事件仍保持旧的全员投递语义，
// 避免新增事件因为忘记声明路由元数据而静默丢失。
type snapshotEventRoute struct {
	Global bool

	ParticipantUserIDs []string
	PlayerIDs          []string
	FoodIDs            []string
	EjectedIDs         []string

	HasPosition bool
	X           float64
	Y           float64
	Radius      float64
}

// addRoutedEvent 同时记录线上事件和服务端路由元数据。
// 两个 slice 按相同下标对应，broadcastSnapshotLocked 会一起取出并清空。
func (r *Room) addRoutedEvent(t string, data map[string]any, route snapshotEventRoute) {
	r.pendingEvents = append(r.pendingEvents, protocol.SnapshotEvent{
		Type: t, Data: protocol.MustMarshal(data),
	})
	r.pendingEventRoutes = append(r.pendingEventRoutes, route)
}

// addEvent 保留给尚未声明 typed route 的调用点和测试代码。
// 兼容事件默认全局发送，优先保证正确性；生产事件应逐步迁移到 addRoutedEvent。
func (r *Room) addEvent(t string, data map[string]any) {
	r.addRoutedEvent(t, data, snapshotEventRoute{Global: true})
}

// eventsForViewerLocked 按参与者、上一帧可见集、当前 AOI 三层规则过滤事件。
// state 必须是应用当前 FULL/DELTA 之前的状态，这样对象即使在本 Tick 已被删除，
// 只要上一帧可见，观察者仍能收到对应的吞噬/死亡事件。
func (r *Room) eventsForViewerLocked(
	viewer *Player,
	state *aoiClientState,
	events []protocol.SnapshotEvent,
	routes []snapshotEventRoute,
) []protocol.SnapshotEvent {
	if len(events) == 0 {
		return nil
	}

	view, hasView := r.aoiViewForPlayerLocked(viewer)
	out := make([]protocol.SnapshotEvent, 0, len(events))
	for i, event := range events {
		route := snapshotEventRoute{Global: true}
		if i < len(routes) {
			route = routes[i]
		}
		if eventRouteMatchesViewer(viewer, state, view, hasView, route) {
			out = append(out, event)
		}
	}
	return out
}

func eventRouteMatchesViewer(
	viewer *Player,
	state *aoiClientState,
	view aoiView,
	hasView bool,
	route snapshotEventRoute,
) bool {
	if route.Global {
		return true
	}
	if viewer == nil {
		return false
	}

	// 第一层：直接参与者必达，不依赖 AOI/VisibleSet。
	if containsString(route.ParticipantUserIDs, viewer.UserID) {
		return true
	}

	// 第二层：上一份逻辑快照里看得到关联对象。
	// 这里故意在当前 DELTA 推进 state 之前判断，覆盖“本帧已经删除”的对象。
	if state != nil {
		for _, id := range route.PlayerIDs {
			if _, ok := state.players[id]; ok {
				return true
			}
		}
		for _, id := range route.FoodIDs {
			if _, ok := state.foods[id]; ok {
				return true
			}
		}
		for _, id := range route.EjectedIDs {
			if _, ok := state.ejected[id]; ok {
				return true
			}
		}
	}

	// 第三层：事件发生点当前落在 viewer 的 AOI 范围。
	if route.HasPosition && hasView {
		return intersectsAOI(view, route.X, route.Y, route.Radius)
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
