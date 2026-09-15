package game

import (
	"testing"

	"cellular-phagocyte/server/internal/protocol"
)

type runtimeStatsConn struct{}

func (runtimeStatsConn) Send(protocol.Envelope)         {}
func (runtimeStatsConn) SendSnapshot(protocol.Envelope) {}
func (runtimeStatsConn) Close()                         {}

func TestManagerRuntimeStatsAggregatesRooms(t *testing.T) {
	mgr := &Manager{rooms: map[string]*Room{}}
	mgr.rooms["r1"] = &Room{
		status: RoomRunning,
		players: map[string]*Player{
			"u1": {UserID: "u1", conn: runtimeStatsConn{}, Balls: []*Ball{{BallID: "b1"}, {BallID: "b2"}}},
			"u2": {UserID: "u2", Balls: []*Ball{{BallID: "b3"}}},
			"bot": {UserID: "bot", IsBot: true, Balls: []*Ball{{BallID: "b4"}}},
		},
		foods: map[string]*Food{"f1": {}, "f2": {}},
		ejected: map[string]*EjectedMass{"e1": {}},
	}
	mgr.rooms["r2"] = &Room{
		status: RoomLoading,
		players: map[string]*Player{
			"u3": {UserID: "u3", conn: runtimeStatsConn{}},
		},
		foods:   map[string]*Food{},
		ejected: map[string]*EjectedMass{},
	}

	got := mgr.RuntimeStats()
	if got.Rooms != 2 || got.RunningRooms != 1 {
		t.Fatalf("rooms=%d running=%d, want 2/1", got.Rooms, got.RunningRooms)
	}
	if got.Players != 4 || got.HumanPlayers != 3 || got.ConnectedHumans != 2 {
		t.Fatalf("players=%d humans=%d connected=%d, want 4/3/2", got.Players, got.HumanPlayers, got.ConnectedHumans)
	}
	if got.Balls != 4 || got.Foods != 2 || got.EjectedMass != 1 {
		t.Fatalf("balls=%d foods=%d ejected=%d, want 4/2/1", got.Balls, got.Foods, got.EjectedMass)
	}
}
