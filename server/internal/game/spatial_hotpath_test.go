package game

import "testing"

func TestSpatialFoodMatchesBruteForce(t *testing.T) {
	brute := foodCollisionRoom(t)
	spatial := foodCollisionRoom(t)

	brute.eatFoodLocked()
	spatial.eatFoodSpatialLocked()

	bp := brute.players["u1"]
	sp := spatial.players["u1"]
	if len(brute.foods) != len(spatial.foods) {
		t.Fatalf("remaining food mismatch: brute=%d spatial=%d", len(brute.foods), len(spatial.foods))
	}
	if bp.Balls[0].Mass != sp.Balls[0].Mass {
		t.Fatalf("mass mismatch: brute=%v spatial=%v", bp.Balls[0].Mass, sp.Balls[0].Mass)
	}
	if bp.EatFoodCount != sp.EatFoodCount {
		t.Fatalf("food count mismatch: brute=%d spatial=%d", bp.EatFoodCount, sp.EatFoodCount)
	}
}

func foodCollisionRoom(t *testing.T) *Room {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	p.Balls[0].X = 1000
	p.Balls[0].Y = 1000
	r.foods = map[string]*Food{
		"near1": {ID: "near1", X: 1010, Y: 1000, Mass: r.cfg.FoodMass},
		"near2": {ID: "near2", X: 1020, Y: 1000, Mass: r.cfg.FoodMass},
		"far":   {ID: "far", X: 3000, Y: 3000, Mass: r.cfg.FoodMass},
	}
	return r
}

func TestSpatialEjectedMatchesBruteForce(t *testing.T) {
	brute := ejectedCollisionRoom(t)
	spatial := ejectedCollisionRoom(t)

	brute.eatEjectedLocked(1000)
	spatial.eatEjectedSpatialLocked(1000)

	bp := brute.players["u1"]
	sp := spatial.players["u1"]
	if len(brute.ejected) != len(spatial.ejected) {
		t.Fatalf("remaining ejected mismatch: brute=%d spatial=%d", len(brute.ejected), len(spatial.ejected))
	}
	if bp.Balls[0].Mass != sp.Balls[0].Mass {
		t.Fatalf("mass mismatch: brute=%v spatial=%v", bp.Balls[0].Mass, sp.Balls[0].Mass)
	}
}

func ejectedCollisionRoom(t *testing.T) *Room {
	r := testRoom(t)
	p := addPlayingHuman(r, "u1", 100)
	p.Balls[0].X = 1000
	p.Balls[0].Y = 1000
	radius := Radius(r.cfg.EjectMass, r.cfg.RadiusFactor)
	r.ejected = map[string]*EjectedMass{
		"near": {ID: "near", OwnerID: "u2", X: 1010, Y: 1000, Mass: r.cfg.EjectMass, Radius: radius},
		"far":  {ID: "far", OwnerID: "u2", X: 3000, Y: 3000, Mass: r.cfg.EjectMass, Radius: radius},
	}
	return r
}

func TestSpatialPlayerCollisionMatchesBruteForceGrowthChain(t *testing.T) {
	brute := playerCollisionRoom(t)
	spatial := playerCollisionRoom(t)

	brute.eatPlayersLocked()
	spatial.eatPlayersSpatialLocked()

	ba := brute.players["attacker"]
	sa := spatial.players["attacker"]
	if ba.Balls[0].Mass != sa.Balls[0].Mass {
		t.Fatalf("attacker mass mismatch: brute=%v spatial=%v", ba.Balls[0].Mass, sa.Balls[0].Mass)
	}
	for _, id := range []string{"victim1", "victim2"} {
		if brute.players[id].dead != spatial.players[id].dead {
			t.Fatalf("dead state mismatch for %s: brute=%v spatial=%v", id, brute.players[id].dead, spatial.players[id].dead)
		}
	}
	if ba.EatPlayerCount != sa.EatPlayerCount {
		t.Fatalf("eat player count mismatch: brute=%d spatial=%d", ba.EatPlayerCount, sa.EatPlayerCount)
	}
}

func playerCollisionRoom(t *testing.T) *Room {
	r := testRoom(t)
	attacker := addPlayingHuman(r, "attacker", 100)
	v1 := addPlayingHuman(r, "victim1", 40)
	v2 := addPlayingHuman(r, "victim2", 90)
	far := addPlayingHuman(r, "far", 20)

	attacker.Balls[0].X, attacker.Balls[0].Y = 1000, 1000
	v1.Balls[0].X, v1.Balls[0].Y = 1005, 1000
	v2.Balls[0].X, v2.Balls[0].Y = 1010, 1000
	far.Balls[0].X, far.Balls[0].Y = 3500, 3500
	return r
}
