package game

import (
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestReconnectWhileSettlingRequestsRetry(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	p.conn = nil
	r.finished = true
	r.status = RoomSettling

	conn := &recordingConn{}
	result, snapshot, ok := r.Reconnect("u1", conn)
	if ok {
		t.Fatal("reconnect during settling should ask client to retry")
	}
	if snapshot != nil {
		t.Fatal("settling reconnect should not return a recovery snapshot")
	}
	if result.Reason != "ROOM_SETTLING" || result.Status != RoomSettling {
		t.Fatalf("unexpected settling reconnect result: %+v", result)
	}
	if p.conn != nil {
		t.Fatal("failed settling reconnect must not bind the new connection")
	}
}

func TestReconnectFinishedRoomRestoresSettlementForDeadPlayer(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	p.dead = true
	p.Status = StatusDead
	p.Balls = nil
	p.conn = nil

	r.finished = true
	r.status = RoomFinished
	r.endReason = "TIME_LIMIT"
	r.settlementResults["u1"] = protocol.SettlementResultData{
		RoomID: "r_test", UserID: "u1", Rank: 2, TotalPlayers: 8,
		FinalScore: 123, CoinReward: 10, ExpReward: 5, Status: "SUCCESS",
	}

	conn := &recordingConn{}
	result, snapshot, ok := r.Reconnect("u1", conn)
	if !ok {
		t.Fatalf("finished-room reconnect should succeed during grace period: %+v", result)
	}
	if snapshot != nil {
		t.Fatal("finished-room reconnect should not return a live recovery snapshot")
	}
	if result.Status != RoomFinished {
		t.Fatalf("expected FINISHED status, got %+v", result)
	}
	if p.conn != conn {
		t.Fatal("finished-room reconnect should bind the new connection")
	}

	end, settlement, finished := r.FinishedPayload("u1")
	if !finished {
		t.Fatal("finished payload should be available during grace period")
	}
	if end.Reason != "TIME_LIMIT" || end.RoomID != "r_test" {
		t.Fatalf("unexpected recovered game end: %+v", end)
	}
	if settlement == nil {
		t.Fatal("settlement result should be retained for reconnect")
	}
	if settlement.UserID != "u1" || settlement.FinalScore != 123 || settlement.Status != "SUCCESS" {
		t.Fatalf("unexpected recovered settlement: %+v", settlement)
	}
}

func TestFinishedPayloadDoesNotLeakAnotherPlayersSettlement(t *testing.T) {
	r := testRoom(t)
	addPlayingHuman(r, "u1", 100)
	r.finished = true
	r.status = RoomFinished
	r.endReason = "TIME_LIMIT"
	r.settlementResults["u2"] = protocol.SettlementResultData{RoomID: "r_test", UserID: "u2"}

	end, settlement, finished := r.FinishedPayload("u1")
	if !finished {
		t.Fatal("room should still report finished payload state")
	}
	if end.RoomID != "r_test" {
		t.Fatalf("unexpected game end payload: %+v", end)
	}
	if settlement != nil {
		t.Fatalf("must not return another player's settlement: %+v", settlement)
	}
}
