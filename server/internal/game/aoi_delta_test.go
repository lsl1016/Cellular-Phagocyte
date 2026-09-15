package game

import (
	"encoding/json"
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestAOIDeltaTracksEnteredLeftDeletedAndUpdatedObjects(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 300
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 300
	r.cfg.AOIFullSnapshotIntervalSeconds = 60

	self := addPlayingHuman(r, "u1", 100)
	self.Balls[0].X, self.Balls[0].Y = 1000, 1000
	conn := &recordingConn{}
	self.conn = conn

	foodLeft := &Food{ID: "f_left", X: 1100, Y: 1000, Mass: r.cfg.FoodMass, Color: "blue"}
	foodDeleted := &Food{ID: "f_deleted", X: 1150, Y: 1000, Mass: r.cfg.FoodMass, Color: "red"}
	r.foods[foodLeft.ID] = foodLeft
	r.foods[foodDeleted.ID] = foodDeleted

	ejectRadius := Radius(r.cfg.EjectMass, r.cfg.RadiusFactor)
	e := &EjectedMass{ID: "e1", OwnerID: "u1", X: 1200, Y: 1000, Mass: r.cfg.EjectMass, Radius: ejectRadius}
	r.ejected[e.ID] = e

	r.broadcastSnapshotLocked()
	if got := decodeSnapshot(t, conn.snapshots[0]); got.SnapshotType != protocol.SnapshotAOIFull || got.SnapshotSeq != 1 {
		t.Fatalf("first snapshot should establish AOI_FULL seq=1: %+v", got)
	}

	// 第二帧：一个食物离开视野、一个被真正删除、一个新进入；吐出物和自己仍可见，走 updated。
	foodLeft.X = 2000
	delete(r.foods, foodDeleted.ID)
	r.foods["f_enter"] = &Food{ID: "f_enter", X: 1250, Y: 1000, Mass: r.cfg.FoodMass, Color: "green"}
	e.X = 1210
	self.Balls[0].X = 1010

	r.broadcastSnapshotLocked()
	if len(conn.snapshots) != 2 {
		t.Fatalf("expected two snapshots, got %d", len(conn.snapshots))
	}
	delta := decodeDelta(t, conn.snapshots[1])
	if delta.SnapshotType != protocol.SnapshotAOIDelta || delta.BaseSeq != 1 || delta.SnapshotSeq != 2 {
		t.Fatalf("unexpected delta chain: %+v", delta)
	}
	if len(delta.Updated.Players) != 1 || delta.Updated.Players[0].UserID != "u1" {
		t.Fatalf("self should be an updated player: %+v", delta.Updated.Players)
	}
	if len(delta.Entered.Foods) != 1 || delta.Entered.Foods[0].FoodID != "f_enter" {
		t.Fatalf("entered foods = %+v", delta.Entered.Foods)
	}
	if len(delta.Left.FoodIDs) != 1 || delta.Left.FoodIDs[0] != "f_left" {
		t.Fatalf("left foods = %+v", delta.Left.FoodIDs)
	}
	if len(delta.Deleted.FoodIDs) != 1 || delta.Deleted.FoodIDs[0] != "f_deleted" {
		t.Fatalf("deleted foods = %+v", delta.Deleted.FoodIDs)
	}
	if len(delta.Updated.Ejected) != 1 || delta.Updated.Ejected[0].EjectID != "e1" {
		t.Fatalf("visible ejected mass should be updated: %+v", delta.Updated.Ejected)
	}
}

func TestAOIFullSyncRequestForcesNextSnapshotFull(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.AOIFullSnapshotIntervalSeconds = 60
	p := addPlayingHuman(r, "u1", 100)
	conn := &recordingConn{}
	p.conn = conn

	r.broadcastSnapshotLocked()
	r.broadcastSnapshotLocked()
	if got := decodeDelta(t, conn.snapshots[1]); got.SnapshotType != protocol.SnapshotAOIDelta {
		t.Fatalf("second frame should be delta: %+v", got)
	}

	r.RequestFullSync("u1")
	r.broadcastSnapshotLocked()
	full := decodeSnapshot(t, conn.snapshots[2])
	if full.SnapshotType != protocol.SnapshotAOIFull || full.SnapshotSeq != 3 {
		t.Fatalf("FULL_SYNC should force next AOI_FULL: %+v", full)
	}
}

func TestAOIPeriodicFullSnapshotBoundsDeltaChain(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.SnapshotRate = 2
	r.cfg.AOIFullSnapshotIntervalSeconds = 1 // 每 2 个 snapshotSeq 校正一次
	p := addPlayingHuman(r, "u1", 100)
	conn := &recordingConn{}
	p.conn = conn

	r.broadcastSnapshotLocked() // seq=1 FULL
	r.broadcastSnapshotLocked() // seq=2 DELTA
	r.broadcastSnapshotLocked() // seq=3 FULL (3-1 >= 2)

	if got := decodeDelta(t, conn.snapshots[1]); got.SnapshotType != protocol.SnapshotAOIDelta || got.BaseSeq != 1 {
		t.Fatalf("middle frame should be delta based on seq1: %+v", got)
	}
	if got := decodeSnapshot(t, conn.snapshots[2]); got.SnapshotType != protocol.SnapshotAOIFull || got.SnapshotSeq != 3 {
		t.Fatalf("periodic correction should send AOI_FULL seq3: %+v", got)
	}
}

func TestAOIRecoverSnapshotBecomesNextDeltaBase(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.AOIFullSnapshotIntervalSeconds = 60
	p := addPlayingHuman(r, "u1", 100)
	conn := &recordingConn{}
	p.conn = conn
	r.snapshotSeq = 7

	recover := r.recoverSnapshotLocked("u1")
	if recover.SnapshotSeq != 7 || recover.SnapshotType != protocol.SnapshotAOIRecover {
		t.Fatalf("unexpected recovery baseline: %+v", recover)
	}

	r.broadcastSnapshotLocked()
	delta := decodeDelta(t, conn.snapshots[0])
	if delta.BaseSeq != 7 || delta.SnapshotSeq != 8 {
		t.Fatalf("post-reconnect delta should continue from recovery seq: %+v", delta)
	}
}

func decodeDelta(t *testing.T, env protocol.Envelope) protocol.AOIDeltaData {
	t.Helper()
	var delta protocol.AOIDeltaData
	if err := json.Unmarshal(env.Data, &delta); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return delta
}
