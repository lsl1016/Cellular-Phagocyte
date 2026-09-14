package game

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"testing"

	"cellular-phagocyte/server/internal/config"
	"cellular-phagocyte/server/internal/protocol"
)

// benchmarkConn 只消费消息，不做网络 IO。
// snapshotDataBytes 统计 ROOM_SNAPSHOT data JSON 的总字节数，用于对比 AOI 与全量广播的 payload 趋势；
// 它不包含 WebSocket frame 和 Envelope 外层字段，因此不是完整 wire bytes。
type benchmarkConn struct {
	sent              int64
	snapshotDataBytes int64
}

func (c *benchmarkConn) Send(protocol.Envelope) { c.sent++ }
func (c *benchmarkConn) SendSnapshot(env protocol.Envelope) {
	c.sent++
	c.snapshotDataBytes += int64(len(env.Data))
}
func (c *benchmarkConn) Close() {}

// BenchmarkRoomTickSimulationOnly 测量纯逻辑 Tick 热路径，不包含 snapshot/rank 构建。
// 这是碰撞、食物、吐出物扫描等算法变化最直接的趋势基线。
func BenchmarkRoomTickSimulationOnly(b *testing.B) {
	for _, players := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("P%d_F500_E200", players), func(b *testing.B) {
			r := newBenchmarkRoom(b, players, 500, 200, false, true)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if ended := r.stepLocked(1<<30, 1<<30); ended {
					b.Fatal("benchmark room ended unexpectedly")
				}
			}
		})
	}
}

// BenchmarkRoomTickProductionCadence 按默认生产节奏执行：20Hz Tick、10Hz snapshot、1Hz rank。
// Full 与 AOI 两条基准同时保留，避免 AOI 默认开关变化导致历史 benchmark 语义漂移。
func BenchmarkRoomTickProductionCadence(b *testing.B) {
	for _, players := range []int{10, 50, 100} {
		for _, mode := range benchmarkSnapshotModes() {
			b.Run(fmt.Sprintf("%s_P%d_F500_E200", mode.name, players), func(b *testing.B) {
				r := newBenchmarkRoom(b, players, 500, 200, true, mode.aoiEnabled)
				snapshotEvery := maxInt(1, r.cfg.TickRate/r.cfg.SnapshotRate)
				rankEvery := maxInt(1, r.cfg.TickRate/r.cfg.RankUpdateRate)

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if ended := r.stepLocked(snapshotEvery, rankEvery); ended {
						b.Fatal("benchmark room ended unexpectedly")
					}
				}
				b.StopTimer()
				reportSnapshotDataBytes(b, r)
			})
		}
	}
}

// BenchmarkRoomSnapshotAssembly 单独观察快照对象构建与 JSON 编码成本。
// 同一场景分别运行 Full 与 AOI，既保留 #10 建立的全量历史基线，也能持续观察 AOI 的 CPU/分配/包体收益。
func BenchmarkRoomSnapshotAssembly(b *testing.B) {
	for _, players := range []int{10, 50, 100} {
		for _, mode := range benchmarkSnapshotModes() {
			b.Run(fmt.Sprintf("%s_P%d_F500_E200", mode.name, players), func(b *testing.B) {
				r := newBenchmarkRoom(b, players, 500, 200, true, mode.aoiEnabled)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					r.broadcastSnapshotLocked()
				}
				b.StopTimer()
				reportSnapshotDataBytes(b, r)
			})
		}
	}
}

type benchmarkSnapshotMode struct {
	name       string
	aoiEnabled bool
}

func benchmarkSnapshotModes() []benchmarkSnapshotMode {
	return []benchmarkSnapshotMode{
		{name: "Full", aoiEnabled: false},
		{name: "AOI", aoiEnabled: true},
	}
}

func reportSnapshotDataBytes(b *testing.B, r *Room) {
	if b.N <= 0 {
		return
	}
	var total int64
	for _, id := range r.order {
		if conn, ok := r.players[id].conn.(*benchmarkConn); ok {
			total += conn.snapshotDataBytes
		}
	}
	b.ReportMetric(float64(total)/float64(b.N), "snapshot-data-bytes/op")
}

func newBenchmarkRoom(b *testing.B, playerCount, foodCount, ejectedCount int, attachConn, aoiEnabled bool) *Room {
	b.Helper()

	cfg := config.Default()
	cfg.Game.BotFillCount = 0
	cfg.Game.InitialFoodCount = foodCount
	cfg.Game.MaxFoodCount = foodCount
	cfg.Game.AOIEnabled = aoiEnabled

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(cfg, nil, log, NewMemoryTokenStore())
	r := newRoom("r_bench", "m_bench", "classic", cfg.Game, mgr, log)
	r.status = RoomRunning
	r.started = true
	r.startTimeMs = 1
	r.endTimeMs = math.MaxInt64

	// 玩家固定在地图右下区域的稀疏网格，并标记为 DISCONNECTED 以冻结移动。
	// alive() 仍为 true，故碰撞/统计路径与在线玩家一致；deadline=0 不会触发超时死亡。
	for i := 0; i < playerCount; i++ {
		id := fmt.Sprintf("u_%03d", i)
		mass := 100.0
		radius := Radius(mass, r.cfg.RadiusFactor)
		p := &Player{
			UserID:   id,
			Nickname: id,
			Status:   StatusDisconnected,
			Entered:  true,
			Balls: []*Ball{{
				BallID: fmt.Sprintf("b_%03d", i),
				X:      2100 + float64(i%10)*150,
				Y:      2100 + float64(i/10)*150,
				Mass:   mass,
				Radius: radius,
			}},
			MaxMass: mass,
		}
		if attachConn {
			p.conn = &benchmarkConn{}
		}
		r.players[id] = p
		r.order = append(r.order, id)
	}

	// 食物和吐出物集中在地图左上，避免 benchmark 自身因吞噬而持续改变数据规模。
	// 这种布局也能稳定体现 AOI 对远端无关对象的过滤收益。
	for i := 0; i < foodCount; i++ {
		id := fmt.Sprintf("f_%04d", i)
		r.foods[id] = &Food{
			ID:    id,
			X:     100 + float64(i%25)*4,
			Y:     100 + float64((i/25)%25)*4,
			Mass:  r.cfg.FoodMass,
			Color: "blue",
		}
	}

	ejectRadius := Radius(r.cfg.EjectMass, r.cfg.RadiusFactor)
	for i := 0; i < ejectedCount; i++ {
		id := fmt.Sprintf("e_%04d", i)
		r.ejected[id] = &EjectedMass{
			ID:      id,
			OwnerID: "bench_owner",
			X:       400 + float64(i%20)*4,
			Y:       400 + float64((i/20)%20)*4,
			Mass:    r.cfg.EjectMass,
			Radius:  ejectRadius,
		}
	}

	return r
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
