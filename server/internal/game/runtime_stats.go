package game

// RuntimeStats is a point-in-time, read-only view of game service cardinality.
// It is intentionally small so load-test observability can sample it once per second
// without exposing room internals or protocol payloads.
type RuntimeStats struct {
	Rooms           int `json:"rooms"`
	RunningRooms    int `json:"runningRooms"`
	Players         int `json:"players"`
	HumanPlayers    int `json:"humanPlayers"`
	AliveHumans     int `json:"aliveHumans"`
	DeadHumans      int `json:"deadHumans"`
	ExitedHumans    int `json:"exitedHumans"`
	ConnectedHumans int `json:"connectedHumans"`
	Balls           int `json:"balls"`
	Foods           int `json:"foods"`
	EjectedMass     int `json:"ejectedMass"`
}

// RuntimeStats returns a point-in-time aggregate of all rooms owned by this manager.
// The manager lock is only held while copying room pointers; each room is then sampled
// under its own lock so room Tick work is not blocked by a global manager lock.
func (m *Manager) RuntimeStats() RuntimeStats {
	m.mu.RLock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.mu.RUnlock()

	stats := RuntimeStats{Rooms: len(rooms)}
	for _, room := range rooms {
		room.mu.Lock()
		if room.status == RoomRunning {
			stats.RunningRooms++
		}
		stats.Players += len(room.players)
		stats.Foods += len(room.foods)
		stats.EjectedMass += len(room.ejected)
		for _, player := range room.players {
			stats.Balls += len(player.Balls)
			if player.IsBot {
				continue
			}
			stats.HumanPlayers++
			if player.alive() {
				stats.AliveHumans++
			}
			switch player.Status {
			case StatusDead:
				stats.DeadHumans++
			case StatusExited:
				stats.ExitedHumans++
			}
			if player.conn != nil {
				stats.ConnectedHumans++
			}
		}
		room.mu.Unlock()
	}
	return stats
}
