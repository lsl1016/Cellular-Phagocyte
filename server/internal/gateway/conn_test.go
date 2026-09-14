package gateway

import (
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
