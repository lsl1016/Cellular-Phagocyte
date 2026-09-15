package game

import (
	"encoding/json"
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestAOIEventRoutingParticipantPreviousVisibleAndCurrentView(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 220
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 220
	// 避免第二帧因为周期校正退回 AOI_FULL，明确验证 AOI_DELTA.events。
	r.cfg.AOIFullSnapshotIntervalSeconds = 999

	previousObserver := addPlayingHuman(r, "u_previous", 100)
	previousObserver.Balls[0].X, previousObserver.Balls[0].Y = 1000, 1000
	previousConn := &recordingConn{}
	previousObserver.conn = previousConn

	participant := addPlayingHuman(r, "u_participant", 100)
	participant.Balls[0].X, participant.Balls[0].Y = 3500, 3500
	participantConn := &recordingConn{}
	participant.conn = participantConn

	unrelated := addPlayingHuman(r, "u_unrelated", 100)
	unrelated.Balls[0].X, unrelated.Balls[0].Y = 3500, 3200
	unrelatedConn := &recordingConn{}
	unrelated.conn = unrelatedConn

	currentObserver := addPlayingHuman(r, "u_current", 100)
	currentObserver.Balls[0].X, currentObserver.Balls[0].Y = 3200, 3500
	currentConn := &recordingConn{}
	currentObserver.conn = currentConn

	foodRadius := Radius(r.cfg.FoodMass, r.cfg.RadiusFactor)
	r.foods["f_event"] = &Food{
		ID: "f_event", X: 1000, Y: 1000,
		Mass: r.cfg.FoodMass, Color: "green",
	}

	// 第一帧建立 VisibleSet：只有 u_previous 看得到 f_event。
	r.broadcastSnapshotLocked()
	if _, ok := r.aoiStateLocked("u_previous").foods["f_event"]; !ok {
		t.Fatal("precondition: u_previous should have f_event in previous VisibleSet")
	}
	if _, ok := r.aoiStateLocked("u_current").foods["f_event"]; ok {
		t.Fatal("precondition: u_current must not see f_event in previous VisibleSet")
	}

	// 事件发生前改变视野：
	// - previousObserver 已离开事件位置，只能靠上一帧 VisibleSet 收到；
	// - currentObserver 刚进入事件位置，只能靠当前 AOI 收到；
	// - participant 始终在远处，但因为是参与者必须收到；
	// - unrelated 始终在远处且从未看见关联对象，必须过滤。
	previousObserver.Balls[0].X, previousObserver.Balls[0].Y = 3000, 3000
	currentObserver.Balls[0].X, currentObserver.Balls[0].Y = 1000, 1000

	delete(r.foods, "f_event")
	r.addRoutedEvent("FOOD_EATEN", map[string]any{
		"userId": "u_participant", "foodId": "f_event",
	}, snapshotEventRoute{
		ParticipantUserIDs: []string{"u_participant"},
		PlayerIDs:          []string{"u_participant"},
		FoodIDs:            []string{"f_event"},
		HasPosition: true, X: 1000, Y: 1000, Radius: foodRadius,
	})

	r.broadcastSnapshotLocked()

	assertDeltaEventTypes(t, previousConn.snapshots[1], []string{"FOOD_EATEN"})
	assertDeltaEventTypes(t, participantConn.snapshots[1], []string{"FOOD_EATEN"})
	assertDeltaEventTypes(t, currentConn.snapshots[1], []string{"FOOD_EATEN"})
	assertDeltaEventTypes(t, unrelatedConn.snapshots[1], nil)
}

func TestUntypedEventKeepsGlobalCompatibilityFallback(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 100
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 100
	r.cfg.AOIFullSnapshotIntervalSeconds = 999

	p1 := addPlayingHuman(r, "u1", 100)
	p1.Balls[0].X, p1.Balls[0].Y = 500, 500
	c1 := &recordingConn{}
	p1.conn = c1

	p2 := addPlayingHuman(r, "u2", 100)
	p2.Balls[0].X, p2.Balls[0].Y = 3500, 3500
	c2 := &recordingConn{}
	p2.conn = c2

	r.broadcastSnapshotLocked()
	r.addEvent("LEGACY_EVENT", map[string]any{"value": 1})
	r.broadcastSnapshotLocked()

	assertDeltaEventTypes(t, c1.snapshots[1], []string{"LEGACY_EVENT"})
	assertDeltaEventTypes(t, c2.snapshots[1], []string{"LEGACY_EVENT"})
}

func assertDeltaEventTypes(t *testing.T, env protocol.Envelope, want []string) {
	t.Helper()
	var delta protocol.AOIDeltaData
	if err := json.Unmarshal(env.Data, &delta); err != nil {
		t.Fatalf("decode AOI_DELTA: %v", err)
	}
	if delta.SnapshotType != protocol.SnapshotAOIDelta {
		t.Fatalf("snapshotType = %q, want %q", delta.SnapshotType, protocol.SnapshotAOIDelta)
	}
	if len(delta.Events) != len(want) {
		t.Fatalf("events = %+v, want types %v", delta.Events, want)
	}
	for i, eventType := range want {
		if delta.Events[i].Type != eventType {
			t.Fatalf("event[%d].type = %q, want %q", i, delta.Events[i].Type, eventType)
		}
	}
}
