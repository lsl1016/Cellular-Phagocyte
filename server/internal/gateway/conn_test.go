package gateway

import (
	"encoding/json"
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

func TestSendReliablePreservesQueueUntilOverflow(t *testing.T) {
	c := newWSConn(nil)

	for i := 0; i < reliableBuffer; i++ {
		c.Send(protocol.Envelope{Type: protocol.TypeGameEnd, Seq: int64(i + 1)})
	}

	select {
	case <-c.done:
		t.Fatal("reliable queue reaching capacity should not close connection")
	default:
	}

	for i := 0; i < reliableBuffer; i++ {
		env := <-c.reliable
		if env.Seq != int64(i+1) {
			t.Fatalf("reliable queue order changed at %d: got seq=%d", i, env.Seq)
		}
	}
}

func TestSendReliableClosesInsteadOfDroppingOnOverflow(t *testing.T) {
	c := newWSConn(nil)

	for i := 0; i < reliableBuffer; i++ {
		c.Send(protocol.Envelope{Type: protocol.TypeSettlementResult, Seq: int64(i + 1)})
	}
	c.Send(protocol.Envelope{Type: protocol.TypeSettlementResult, Seq: int64(reliableBuffer + 1)})

	select {
	case <-c.done:
		// expected: a slow client must reconnect rather than silently lose a control message.
	default:
		t.Fatal("connection should close when reliable queue overflows")
	}
}

func TestSendSnapshotKeepsOnlyLatestPendingFrame(t *testing.T) {
	c := newWSConn(nil)

	for i := int64(1); i <= 100; i++ {
		c.SendSnapshot(protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: i})
	}

	if got := len(c.snapshotReady); got != 1 {
		t.Fatalf("snapshot notification should be coalesced to one pending signal, got %d", got)
	}

	<-c.snapshotReady
	env, ok := c.takeLatestSnapshot()
	if !ok {
		t.Fatal("expected a pending latest snapshot")
	}
	if env.Seq != 100 {
		t.Fatalf("expected latest snapshot seq=100, got %d", env.Seq)
	}
	if _, ok := c.takeLatestSnapshot(); ok {
		t.Fatal("snapshot should be cleared after taking the latest frame")
	}

	select {
	case <-c.done:
		t.Fatal("snapshot flood must not close the connection")
	default:
	}
}

func TestSendSnapshotPreservesEventsFromOverwrittenFrames(t *testing.T) {
	c := newWSConn(nil)
	first := protocol.RoomSnapshotData{
		RoomID: "r1", SnapshotType: "FULL", TickSeq: 1, ServerTime: 1000,
		Events: []protocol.SnapshotEvent{{Type: "FOOD_EATEN", Data: protocol.MustMarshal(map[string]any{"foodId": "f1"})}},
	}
	second := protocol.RoomSnapshotData{
		RoomID: "r1", SnapshotType: "FULL", TickSeq: 2, ServerTime: 1100,
		Events: []protocol.SnapshotEvent{{Type: "PLAYER_EATEN", Data: protocol.MustMarshal(map[string]any{"userId": "u2"})}},
	}
	third := protocol.RoomSnapshotData{
		RoomID: "r1", SnapshotType: "FULL", TickSeq: 3, ServerTime: 1200,
	}

	c.SendSnapshot(protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 1, Data: protocol.MustMarshal(first)})
	c.SendSnapshot(protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 2, Data: protocol.MustMarshal(second)})
	c.SendSnapshot(protocol.Envelope{Type: protocol.TypeRoomSnapshot, Seq: 3, Data: protocol.MustMarshal(third)})

	<-c.snapshotReady
	env, ok := c.takeLatestSnapshot()
	if !ok {
		t.Fatal("expected coalesced snapshot")
	}
	if env.Seq != 3 {
		t.Fatalf("latest state should win, got seq=%d", env.Seq)
	}

	var got protocol.RoomSnapshotData
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode coalesced snapshot: %v", err)
	}
	if got.TickSeq != 3 || got.ServerTime != 1200 {
		t.Fatalf("latest snapshot state was not preserved: tick=%d time=%d", got.TickSeq, got.ServerTime)
	}
	if len(got.Events) != 2 {
		t.Fatalf("expected both overwritten events to survive, got %d", len(got.Events))
	}
	if got.Events[0].Type != "FOOD_EATEN" || got.Events[1].Type != "PLAYER_EATEN" {
		t.Fatalf("event order changed: %+v", got.Events)
	}
}

func TestClosedConnectionIgnoresFurtherMessages(t *testing.T) {
	c := newWSConn(nil)
	c.Close()

	c.Send(protocol.Envelope{Type: protocol.TypeGameEnd})
	c.SendSnapshot(protocol.Envelope{Type: protocol.TypeRoomSnapshot})

	if got := len(c.reliable); got != 0 {
		t.Fatalf("closed connection should not queue reliable messages, got %d", got)
	}
	if got := len(c.snapshotReady); got != 0 {
		t.Fatalf("closed connection should not queue snapshots, got %d", got)
	}
}
