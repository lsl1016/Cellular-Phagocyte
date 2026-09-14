package game

import "sort"

// eatFoodSpatialLocked 使用空间哈希只检查球体附近的食物。
// 球体吞噬后半径可能增长，因此只要本轮有吞噬就重新查询一次扩大的邻域，
// 保持与原全量扫描“同一 Tick 可连续吞噬”的行为一致。
func (r *Room) eatFoodSpatialLocked() {
	if len(r.foods) == 0 {
		return
	}

	foodRadius := Radius(r.cfg.FoodMass, r.cfg.RadiusFactor)
	grid := newSpatialGrid[*Food](spatialCellSize)
	for _, f := range r.foods {
		grid.Insert(f.X, f.Y, f)
	}

	candidates := make([]*Food, 0, 32)
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			for {
				reach := b.Radius + foodRadius
				candidates = grid.QueryAABB(candidates[:0], b.X-reach, b.Y-reach, b.X+reach, b.Y+reach)
				ateAny := false
				for _, f := range candidates {
					current, ok := r.foods[f.ID]
					if !ok || current != f {
						continue // 已被本 Tick 的其他球吞掉
					}
					if !CanEatFood(b.Radius, foodRadius, Distance(b.X, b.Y, f.X, f.Y)) {
						continue
					}
					b.Mass += f.Mass
					b.Radius = Radius(b.Mass, r.cfg.RadiusFactor)
					delete(r.foods, f.ID)
					p.EatFoodCount++
					ateAny = true
					r.addEvent("FOOD_EATEN", map[string]any{
						"userId": p.UserID, "ballId": b.BallID, "foodId": f.ID,
						"gainMass": f.Mass, "newMass": b.Mass,
					})
				}
				if !ateAny {
					break
				}
			}
		}
	}
}

// eatEjectedSpatialLocked 使用空间哈希只检查球体附近的吐出物。
func (r *Room) eatEjectedSpatialLocked(now int64) {
	if len(r.ejected) == 0 {
		return
	}

	grid := newSpatialGrid[*EjectedMass](spatialCellSize)
	maxRadius := 0.0
	for _, em := range r.ejected {
		grid.Insert(em.X, em.Y, em)
		if em.Radius > maxRadius {
			maxRadius = em.Radius
		}
	}

	candidates := make([]*EjectedMass, 0, 16)
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			for {
				reach := b.Radius + maxRadius
				candidates = grid.QueryAABB(candidates[:0], b.X-reach, b.Y-reach, b.X+reach, b.Y+reach)
				ateAny := false
				for _, em := range candidates {
					current, ok := r.ejected[em.ID]
					if !ok || current != em {
						continue
					}
					if em.OwnerID == p.UserID && now < em.protectUntil {
						continue
					}
					if !CanEatFood(b.Radius, em.Radius, Distance(b.X, b.Y, em.X, em.Y)) {
						continue
					}
					b.Mass += em.Mass * r.cfg.EjectGainRatio
					b.Radius = Radius(b.Mass, r.cfg.RadiusFactor)
					delete(r.ejected, em.ID)
					ateAny = true
					r.addEvent("EJECTED_MASS_EATEN", map[string]any{
						"userId": p.UserID, "ballId": b.BallID, "ejectId": em.ID, "gainMass": em.Mass,
					})
				}
				if !ateAny {
					break
				}
			}
		}
	}
}

type indexedBallRef struct {
	idx int
	ref ballRef
}

// eatPlayersSpatialLocked 用空间哈希生成可能发生碰撞的球体对，再沿原 balls 顺序判定吞噬。
// maxRadius 会在球体吞噬成长时动态更新，避免漏掉同 Tick 内新进入吞噬范围的球体。
func (r *Room) eatPlayersSpatialLocked() {
	balls := make([]ballRef, 0)
	for _, id := range r.order {
		p := r.players[id]
		if !p.alive() {
			continue
		}
		for _, b := range p.Balls {
			balls = append(balls, ballRef{owner: p, ball: b})
		}
	}
	if len(balls) < 2 {
		return
	}

	grid := newSpatialGrid[indexedBallRef](spatialCellSize)
	maxRadius := 0.0
	for i, ref := range balls {
		grid.Insert(ref.ball.X, ref.ball.Y, indexedBallRef{idx: i, ref: ref})
		if ref.ball.Radius > maxRadius {
			maxRadius = ref.ball.Radius
		}
	}

	eaten := make(map[string]bool)
	candidates := make([]indexedBallRef, 0, 32)
	for i := 0; i < len(balls); i++ {
		current := balls[i]
		if eaten[current.ball.BallID] {
			continue
		}

		for {
			reach := current.ball.Radius + maxRadius
			candidates = grid.QueryAABB(candidates[:0],
				current.ball.X-reach, current.ball.Y-reach,
				current.ball.X+reach, current.ball.Y+reach,
			)
			sort.Slice(candidates, func(a, b int) bool { return candidates[a].idx < candidates[b].idx })

			ateAny := false
			for _, candidate := range candidates {
				j := candidate.idx
				if j <= i {
					continue // 每一对只按原嵌套循环顺序处理一次
				}
				a := current
				t := candidate.ref
				if a.owner == t.owner || eaten[a.ball.BallID] || eaten[t.ball.BallID] {
					continue
				}

				big, small := a, t
				if t.ball.Mass > a.ball.Mass {
					big, small = t, a
				}
				dist := Distance(big.ball.X, big.ball.Y, small.ball.X, small.ball.Y)
				if !CanEatPlayer(big.ball.Mass, small.ball.Mass, big.ball.Radius, small.ball.Radius, dist, r.cfg.EatMassRatio, r.cfg.EatDepthFactor) {
					continue
				}

				gain := PlayerEatGain(small.ball.Mass, r.cfg.PlayerEatMassGain)
				big.ball.Mass += gain
				big.ball.Radius = Radius(big.ball.Mass, r.cfg.RadiusFactor)
				if big.ball.Radius > maxRadius {
					maxRadius = big.ball.Radius
				}
				eaten[small.ball.BallID] = true
				if r.eatPlayerBallLocked(small.owner, small.ball, big.owner.UserID, gain, big.ball.Mass) {
					big.owner.EatPlayerCount++
				}
				ateAny = true

				if eaten[current.ball.BallID] {
					break
				}
			}

			if eaten[current.ball.BallID] || !ateAny {
				break
			}
		}
	}
}
