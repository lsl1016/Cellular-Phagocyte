package gateway

import (
	"encoding/json"
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestCoalesceFoldsPendingDeltaIntoFull(t *testing.T) {
	full := protocol.RoomSnapshotData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIFull, SnapshotSeq: 1, TickSeq: 2, ServerTime: 1000,
		Players: []protocol.SnapshotPlayer{{UserID: "u1", Mass: 10, Score: 10}},
		Foods: []protocol.SnapshotFood{{FoodID: "f_old", X: 10, Y: 10}},
		Events: []protocol.SnapshotEvent{{Type: "E1"}},
	}
	delta := protocol.AOIDeltaData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIDelta, BaseSeq: 1, SnapshotSeq: 2, TickSeq: 4, ServerTime: 1100,
		Updated: protocol.SnapshotObjects{Players: []protocol.SnapshotPlayer{{UserID: "u1", Mass: 20, Score: 20}}},
		Entered: protocol.SnapshotObjects{Foods: []protocol.SnapshotFood{{FoodID: "f_new", X: 20, Y: 20}}},
		Left: protocol.SnapshotObjectIDs{FoodIDs: []string{"f_old"}},
		Events: []protocol.SnapshotEvent{{Type: "E2"}},
	}

	gotEnv := coalesceSnapshots(
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 1, Data: protocol.MustMarshal(full)},
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 2, Data: protocol.MustMarshal(delta)},
	)
	var got protocol.RoomSnapshotData
	if err := json.Unmarshal(gotEnv.Data, &got); err != nil {
		t.Fatalf("decode folded full: %v", err)
	}
	if got.SnapshotType != protocol.SnapshotAOIFull || got.SnapshotSeq != 2 || got.TickSeq != 4 {
		t.Fatalf("folded snapshot metadata wrong: %+v", got)
	}
	if len(got.Players) != 1 || got.Players[0].Mass != 20 {
		t.Fatalf("updated player was not folded into full: %+v", got.Players)
	}
	if len(got.Foods) != 1 || got.Foods[0].FoodID != "f_new" {
		t.Fatalf("full object set is wrong after fold: %+v", got.Foods)
	}
	if len(got.Events) != 2 || got.Events[0].Type != "E1" || got.Events[1].Type != "E2" {
		t.Fatalf("events were not preserved in order: %+v", got.Events)
	}
}

func TestCoalesceComposesDeltaChainAgainstOriginalBase(t *testing.T) {
	a := protocol.AOIDeltaData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIDelta, BaseSeq: 10, SnapshotSeq: 11, TickSeq: 20,
		Entered: protocol.SnapshotObjects{
			Players: []protocol.SnapshotPlayer{{UserID: "u2", Mass: 10}},
			Foods:   []protocol.SnapshotFood{{FoodID: "temp"}},
		},
		Updated: protocol.SnapshotObjects{Players: []protocol.SnapshotPlayer{{UserID: "u1", Mass: 11}}},
		Left:    protocol.SnapshotObjectIDs{FoodIDs: []string{"f0"}},
		Events:  []protocol.SnapshotEvent{{Type: "E1"}},
	}
	b := protocol.AOIDeltaData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIDelta, BaseSeq: 11, SnapshotSeq: 12, TickSeq: 22,
		Entered: protocol.SnapshotObjects{Foods: []protocol.SnapshotFood{{FoodID: "f0", X: 5}}},
		Updated: protocol.SnapshotObjects{Players: []protocol.SnapshotPlayer{{UserID: "u2", Mass: 12}}},
		Left: protocol.SnapshotObjectIDs{
			PlayerIDs: []string{"u1"},
			FoodIDs:   []string{"temp"},
		},
		Events: []protocol.SnapshotEvent{{Type: "E2"}},
	}

	gotEnv := coalesceSnapshots(
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 11, Data: protocol.MustMarshal(a)},
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 12, Data: protocol.MustMarshal(b)},
	)
	var got protocol.AOIDeltaData
	if err := json.Unmarshal(gotEnv.Data, &got); err != nil {
		t.Fatalf("decode composed delta: %v", err)
	}
	if got.BaseSeq != 10 || got.SnapshotSeq != 12 {
		t.Fatalf("composed chain metadata wrong: base=%d seq=%d", got.BaseSeq, got.SnapshotSeq)
	}
	if len(got.Entered.Players) != 1 || got.Entered.Players[0].UserID != "u2" || got.Entered.Players[0].Mass != 12 {
		t.Fatalf("entered player should survive with latest value: %+v", got.Entered.Players)
	}
	if len(got.Left.PlayerIDs) != 1 || got.Left.PlayerIDs[0] != "u1" {
		t.Fatalf("base player should end as left: %+v", got.Left.PlayerIDs)
	}
	if len(got.Updated.Foods) != 1 || got.Updated.Foods[0].FoodID != "f0" {
		t.Fatalf("left then re-entered base food should compose to updated: %+v", got.Updated.Foods)
	}
	if len(got.Entered.Foods) != 0 || len(got.Left.FoodIDs) != 0 || len(got.Deleted.FoodIDs) != 0 {
		t.Fatalf("entered-then-left temp food should cancel out: entered=%+v left=%+v deleted=%+v", got.Entered.Foods, got.Left.FoodIDs, got.Deleted.FoodIDs)
	}
	if len(got.Events) != 2 || got.Events[0].Type != "E1" || got.Events[1].Type != "E2" {
		t.Fatalf("delta events were not composed in order: %+v", got.Events)
	}
}

func TestCoalesceKeepsPeriodicFullAndCarriesPendingDeltaEvents(t *testing.T) {
	delta := protocol.AOIDeltaData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIDelta, BaseSeq: 1, SnapshotSeq: 2,
		Events: []protocol.SnapshotEvent{{Type: "E1"}},
	}
	full := protocol.RoomSnapshotData{
		RoomID: "r1", SnapshotType: protocol.SnapshotAOIFull, SnapshotSeq: 3,
		Events: []protocol.SnapshotEvent{{Type: "E2"}},
	}
	gotEnv := coalesceSnapshots(
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 2, Data: protocol.MustMarshal(delta)},
		protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 3, Data: protocol.MustMarshal(full)},
	)
	var got protocol.RoomSnapshotData
	if err := json.Unmarshal(gotEnv.Data, &got); err != nil {
		t.Fatalf("decode periodic full: %v", err)
	}
	if got.SnapshotSeq != 3 || len(got.Events) != 2 || got.Events[0].Type != "E1" || got.Events[1].Type != "E2" {
		t.Fatalf("periodic full should supersede state but preserve pending events: %+v", got)
	}
}
