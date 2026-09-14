package game

import (
	"math"
	"testing"
)

func TestNearestFoodMatchesBruteForceAcrossMap(t *testing.T) {
	foods := make([]*Food, 0, 120)
	grid := newSpatialGrid[*Food](spatialCellSize)
	for i := 0; i < 120; i++ {
		// 确定性地铺散到多个格子，避免测试依赖随机数。
		f := &Food{
			ID:   string(rune('a' + i%26)) + string(rune('A'+(i/26)%26)),
			X:    float64((i*347)%3900) + 25,
			Y:    float64((i*613)%3900) + 25,
			Mass: 1,
		}
		foods = append(foods, f)
		grid.Insert(f.X, f.Y, f)
	}

	queries := [][2]float64{
		{10, 10},
		{500, 500},
		{1999, 2001},
		{3990, 3990},
		{1275, 3420},
		{3050, 750},
	}
	for _, q := range queries {
		got := nearestFood(grid, q[0], q[1])
		want := bruteNearestFood(foods, q[0], q[1])
		if got == nil || want == nil {
			t.Fatalf("nearest lookup unexpectedly nil at %v: got=%v want=%v", q, got, want)
		}
		if got.ID != want.ID {
			t.Fatalf("nearest mismatch at %v: spatial=%s brute=%s", q, got.ID, want.ID)
		}
	}
}

func TestNearestFoodFindsTargetBeyondEmptyNeighborRings(t *testing.T) {
	grid := newSpatialGrid[*Food](100)
	far := &Food{ID: "far", X: 950, Y: 950, Mass: 1}
	grid.Insert(far.X, far.Y, far)

	got := nearestFood(grid, 10, 10)
	if got == nil || got.ID != far.ID {
		t.Fatalf("search must expand through empty rings, got %+v", got)
	}
}

func TestBotAISpatialTargetsSameFoodAsBruteForce(t *testing.T) {
	rBrute := testRoom(t)
	rSpatial := testRoom(t)
	configureBotAITestRoom(rBrute)
	configureBotAITestRoom(rSpatial)

	rBrute.botAILocked()
	rSpatial.botAISpatialLocked()

	got := rSpatial.players["bot_test"].Direction
	want := rBrute.players["bot_test"].Direction
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("bot direction changed: spatial=%v brute=%v", got, want)
	}
}

func configureBotAITestRoom(r *Room) {
	mass := r.cfg.BotInitialMass
	radius := Radius(mass, r.cfg.RadiusFactor)
	bot := &Player{
		UserID:   "bot_test",
		Nickname: "bot",
		IsBot:    true,
		Status:   StatusPlaying,
		Balls: []*Ball{{
			BallID: "bot_ball", X: 1000, Y: 1000, Mass: mass, Radius: radius,
		}},
	}
	r.players = map[string]*Player{"bot_test": bot}
	r.order = []string{"bot_test"}
	r.foods = map[string]*Food{
		"nearest": {ID: "nearest", X: 1080, Y: 1030, Mass: 1},
		"sameCellFarther": {ID: "sameCellFarther", X: 1120, Y: 1120, Mass: 1},
		"otherCell": {ID: "otherCell", X: 500, Y: 500, Mass: 1},
		"far": {ID: "far", X: 3500, Y: 3500, Mass: 1},
	}
}

func bruteNearestFood(foods []*Food, x, y float64) *Food {
	var best *Food
	bestDistSq := math.MaxFloat64
	for _, f := range foods {
		dx := f.X - x
		dy := f.Y - y
		d2 := dx*dx + dy*dy
		if d2 < bestDistSq || (d2 == bestDistSq && best != nil && f.ID < best.ID) {
			bestDistSq = d2
			best = f
		}
	}
	return best
}
