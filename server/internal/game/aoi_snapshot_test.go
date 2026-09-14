package game

import (
	"encoding/json"
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestAOISnapshotFiltersFarObjectsAndAlwaysIncludesSelf(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 300
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 300

	self := addPlayingHuman(r, "u1", 100)
	self.Balls[0].X, self.Balls[0].Y = 1000, 1000
	self.conn = &recordingConn{}

	near := addPlayingHuman(r, "u2", 25)
	near.Balls = []*Ball{
		{BallID: "b_u2_near", X: 1250, Y: 1000, Mass: 25, Radius: Radius(25, r.cfg.RadiusFactor)},
		{BallID: "b_u2_far", X: 2000, Y: 1000, Mass: 25, Radius: Radius(25, r.cfg.RadiusFactor)},
	}
	far := addPlayingHuman(r, "u3", 25)
	far.Balls[0].X, far.Balls[0].Y = 3000, 3000

	r.foods["f_near"] = &Food{ID: "f_near", X: 1290, Y: 1000, Mass: r.cfg.FoodMass, Color: "green"}
	r.foods["f_far"] = &Food{ID: "f_far", X: 1500, Y: 1000, Mass: r.cfg.FoodMass, Color: "red"}
	ejectRadius := Radius(r.cfg.EjectMass, r.cfg.RadiusFactor)
	r.ejected["e_near"] = &EjectedMass{ID: "e_near", OwnerID: "u2", X: 1295, Y: 1000, Mass: r.cfg.EjectMass, Radius: ejectRadius}
	r.ejected["e_far"] = &EjectedMass{ID: "e_far", OwnerID: "u3", X: 1500, Y: 1000, Mass: r.cfg.EjectMass, Radius: ejectRadius}

	r.broadcastSnapshotLocked()

	conn := self.conn.(*recordingConn)
	if len(conn.snapshots) != 1 {
		t.Fatalf("expected one personalized snapshot, got %d", len(conn.snapshots))
	}
	snap := decodeSnapshot(t, conn.snapshots[0])
	if snap.SnapshotType != "AOI_FULL" {
		t.Fatalf("snapshotType = %q, want AOI_FULL", snap.SnapshotType)
	}

	selfSnap := snapshotPlayerByID(snap.Players, "u1")
	if selfSnap == nil || len(selfSnap.Balls) != 1 {
		t.Fatalf("self must always be fully synchronized: %+v", selfSnap)
	}
	nearSnap := snapshotPlayerByID(snap.Players, "u2")
	if nearSnap == nil || len(nearSnap.Balls) != 1 || nearSnap.Balls[0].BallID != "b_u2_near" {
		t.Fatalf("near player should contain only the visible split ball: %+v", nearSnap)
	}
	if nearSnap.Mass != 25 {
		t.Fatalf("remote visible mass = %v, want 25 from visible balls only", nearSnap.Mass)
	}
	if snapshotPlayerByID(snap.Players, "u3") != nil {
		t.Fatal("far player must be filtered from AOI snapshot")
	}
	if got := snapshotFoodIDs(snap.Foods); len(got) != 1 || got[0] != "f_near" {
		t.Fatalf("visible foods = %v, want [f_near]", got)
	}
	if got := snapshotEjectedIDs(snap.Ejected); len(got) != 1 || got[0] != "e_near" {
		t.Fatalf("visible ejected = %v, want [e_near]", got)
	}
}

func TestAOISnapshotIsPersonalizedPerConnection(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 250
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 250

	p1 := addPlayingHuman(r, "u1", 100)
	p1.Balls[0].X, p1.Balls[0].Y = 500, 500
	c1 := &recordingConn{}
	p1.conn = c1

	p2 := addPlayingHuman(r, "u2", 100)
	p2.Balls[0].X, p2.Balls[0].Y = 3500, 3500
	c2 := &recordingConn{}
	p2.conn = c2

	r.foods["f_u1"] = &Food{ID: "f_u1", X: 550, Y: 500, Mass: r.cfg.FoodMass, Color: "blue"}
	r.foods["f_u2"] = &Food{ID: "f_u2", X: 3450, Y: 3500, Mass: r.cfg.FoodMass, Color: "yellow"}

	r.broadcastSnapshotLocked()

	s1 := decodeSnapshot(t, c1.snapshots[0])
	s2 := decodeSnapshot(t, c2.snapshots[0])
	if snapshotPlayerByID(s1.Players, "u1") == nil || snapshotPlayerByID(s1.Players, "u2") != nil {
		t.Fatalf("u1 received wrong player set: %+v", s1.Players)
	}
	if snapshotPlayerByID(s2.Players, "u2") == nil || snapshotPlayerByID(s2.Players, "u1") != nil {
		t.Fatalf("u2 received wrong player set: %+v", s2.Players)
	}
	if got := snapshotFoodIDs(s1.Foods); len(got) != 1 || got[0] != "f_u1" {
		t.Fatalf("u1 foods = %v, want [f_u1]", got)
	}
	if got := snapshotFoodIDs(s2.Foods); len(got) != 1 || got[0] != "f_u2" {
		t.Fatalf("u2 foods = %v, want [f_u2]", got)
	}
}

func TestAOIRecoverSnapshotUsesReconnectPlayersView(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = true
	r.cfg.BaseViewRadius = 250
	r.cfg.ViewRadiusFactor = 0
	r.cfg.MaxViewRadius = 250

	p1 := addPlayingHuman(r, "u1", 100)
	p1.Balls[0].X, p1.Balls[0].Y = 500, 500
	p2 := addPlayingHuman(r, "u2", 100)
	p2.Balls[0].X, p2.Balls[0].Y = 3500, 3500
	r.foods["f_near"] = &Food{ID: "f_near", X: 550, Y: 500, Mass: r.cfg.FoodMass, Color: "blue"}
	r.foods["f_far"] = &Food{ID: "f_far", X: 3450, Y: 3500, Mass: r.cfg.FoodMass, Color: "red"}

	snap := r.recoverSnapshotLocked("u1")
	if snap.SnapshotType != "AOI_FULL_RECOVER" {
		t.Fatalf("snapshotType = %q, want AOI_FULL_RECOVER", snap.SnapshotType)
	}
	if snapshotPlayerByID(snap.Players, "u1") == nil || snapshotPlayerByID(snap.Players, "u2") != nil {
		t.Fatalf("recovery leaked far player: %+v", snap.Players)
	}
	if got := snapshotFoodIDs(snap.Foods); len(got) != 1 || got[0] != "f_near" {
		t.Fatalf("recovery foods = %v, want [f_near]", got)
	}
}

func TestDisablingAOIRestoresFullRoomSnapshot(t *testing.T) {
	r := testRoom(t)
	r.cfg.AOIEnabled = false

	p1 := addPlayingHuman(r, "u1", 100)
	p1.Balls[0].X, p1.Balls[0].Y = 500, 500
	c1 := &recordingConn{}
	p1.conn = c1
	p2 := addPlayingHuman(r, "u2", 100)
	p2.Balls[0].X, p2.Balls[0].Y = 3500, 3500
	r.foods["f_far"] = &Food{ID: "f_far", X: 3500, Y: 3500, Mass: r.cfg.FoodMass, Color: "red"}

	r.broadcastSnapshotLocked()

	snap := decodeSnapshot(t, c1.snapshots[0])
	if snap.SnapshotType != "FULL" {
		t.Fatalf("snapshotType = %q, want FULL", snap.SnapshotType)
	}
	if snapshotPlayerByID(snap.Players, "u2") == nil {
		t.Fatal("disabled AOI should preserve full-room player visibility")
	}
	if got := snapshotFoodIDs(snap.Foods); len(got) != 1 || got[0] != "f_far" {
		t.Fatalf("disabled AOI should preserve full-room foods, got %v", got)
	}
}

func decodeSnapshot(t *testing.T, env protocol.Envelope) protocol.RoomSnapshotData {
	t.Helper()
	var snap protocol.RoomSnapshotData
	if err := json.Unmarshal(env.Data, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return snap
}

func snapshotPlayerByID(players []protocol.SnapshotPlayer, userID string) *protocol.SnapshotPlayer {
	for i := range players {
		if players[i].UserID == userID {
			return &players[i]
		}
	}
	return nil
}

func snapshotFoodIDs(foods []protocol.SnapshotFood) []string {
	ids := make([]string, 0, len(foods))
	for _, f := range foods {
		ids = append(ids, f.FoodID)
	}
	return ids
}

func snapshotEjectedIDs(items []protocol.SnapshotEjected) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.EjectID)
	}
	return ids
}
