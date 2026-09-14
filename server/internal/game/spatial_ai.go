package game

import "math"

// botAISpatialLocked 为所有存活机器人构建一次食物空间索引，并精确寻找全局最近食物。
// 搜索按网格环向外扩张；当当前最优距离已经小于“已搜索方形到外部区域的最短距离”时，
// 可以证明未搜索格子不可能出现更近目标，因此提前结束，而不是退化为近似寻路。
func (r *Room) botAISpatialLocked() {
	if len(r.foods) == 0 {
		return
	}

	grid := newSpatialGrid[*Food](spatialCellSize)
	for _, f := range r.foods {
		grid.Insert(f.X, f.Y, f)
	}

	for _, id := range r.order {
		p := r.players[id]
		if !p.IsBot || !p.alive() || len(p.Balls) == 0 {
			continue
		}
		b := p.Balls[0]
		if best := nearestFood(grid, b.X, b.Y); best != nil {
			p.Direction = math.Atan2(best.Y-b.Y, best.X-b.X)
		}
	}
}

func nearestFood(grid *spatialGrid[*Food], x, y float64) *Food {
	if grid == nil || !grid.hasBounds {
		return nil
	}

	center := grid.cellFor(x, y)
	var best *Food
	bestDistSq := math.MaxFloat64

	for ring := 0; ; ring++ {
		for cy := center.y - ring; cy <= center.y+ring; cy++ {
			for cx := center.x - ring; cx <= center.x+ring; cx++ {
				// 只扫描当前环的边界，避免重复访问之前的格子。
				if ring > 0 && cx != center.x-ring && cx != center.x+ring && cy != center.y-ring && cy != center.y+ring {
					continue
				}
				for _, f := range grid.cells[spatialCell{x: cx, y: cy}] {
					dx := f.X - x
					dy := f.Y - y
					d2 := dx*dx + dy*dy
					if d2 < bestDistSq || (d2 == bestDistSq && best != nil && f.ID < best.ID) {
						bestDistSq = d2
						best = f
					}
				}
			}
		}

		if grid.coversOccupiedBounds(center, ring) {
			return best
		}
		if best == nil {
			continue
		}

		// 已搜索区域覆盖从 (center-ring) 到 (center+ring+1) 的完整世界坐标矩形。
		// 任意未搜索点至少要跨过四条边中的一条；如果这个最短距离已不小于当前最优距离，
		// 则全局最近目标已经确定。
		left := x - float64(center.x-ring)*grid.cellSize
		right := float64(center.x+ring+1)*grid.cellSize - x
		bottom := y - float64(center.y-ring)*grid.cellSize
		top := float64(center.y+ring+1)*grid.cellSize - y
		outsideMin := math.Min(math.Min(left, right), math.Min(bottom, top))
		if outsideMin >= 0 && bestDistSq <= outsideMin*outsideMin {
			return best
		}
	}
}
