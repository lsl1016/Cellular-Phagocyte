package game

import (
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

type recordingConn struct {
	reliable  []protocol.Envelope
	snapshots []protocol.Envelope
	closed    bool
}

func (c *recordingConn) Send(env protocol.Envelope) {
	c.reliable = append(c.reliable, env)
}

func (c *recordingConn) SendSnapshot(env protocol.Envelope) {
	c.snapshots = append(c.snapshots, env)
}

func (c *recordingConn) Close() { c.closed = true }

func TestRoomSnapshotUsesLatestOnlyLane(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	conn := &recordingConn{}
	p.conn = conn

	r.broadcastSnapshotLocked()

	if len(conn.snapshots) != 1 {
		t.Fatalf("ROOM_SNAPSHOT should use snapshot lane, got %d snapshots", len(conn.snapshots))
	}
	if got := conn.snapshots[0].Type; got != protocol.TypeRoomSnapshot {
		t.Fatalf("unexpected snapshot message type: %s", got)
	}
	if len(conn.reliable) != 0 {
		t.Fatalf("ROOM_SNAPSHOT must not consume reliable queue, got %d reliable messages", len(conn.reliable))
	}
}

func TestRankUpdateStaysOnReliableLane(t *testing.T) {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	p.MaxMass = 100
	conn := &recordingConn{}
	p.conn = conn

	r.broadcastRankLocked()

	if len(conn.reliable) != 1 {
		t.Fatalf("RANK_UPDATE should use reliable lane, got %d messages", len(conn.reliable))
	}
	if got := conn.reliable[0].Type; got != protocol.TypeRankUpdate {
		t.Fatalf("unexpected reliable message type: %s", got)
	}
	if len(conn.snapshots) != 0 {
		t.Fatalf("RANK_UPDATE must not use snapshot lane, got %d snapshots", len(conn.snapshots))
	}
}
