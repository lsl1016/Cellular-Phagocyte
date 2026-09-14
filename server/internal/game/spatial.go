package game

import "math"

const spatialCellSize = 256.0

type spatialCell struct {
	x int
	y int
}

type spatialGrid[T any] struct {
	cellSize float64
	cells    map[spatialCell][]T
}

func newSpatialGrid[T any](cellSize float64) *spatialGrid[T] {
	if cellSize <= 0 {
		cellSize = spatialCellSize
	}
	return &spatialGrid[T]{
		cellSize: cellSize,
		cells:    make(map[spatialCell][]T),
	}
}

func (g *spatialGrid[T]) cellFor(x, y float64) spatialCell {
	return spatialCell{
		x: int(math.Floor(x / g.cellSize)),
		y: int(math.Floor(y / g.cellSize)),
	}
}

func (g *spatialGrid[T]) Insert(x, y float64, value T) {
	key := g.cellFor(x, y)
	g.cells[key] = append(g.cells[key], value)
}

// QueryAABB 把与给定包围盒相交的网格单元中的元素追加到 dst。
// 元素以点形式只插入一个格子，因此结果不会因为跨格查询而产生重复项。
func (g *spatialGrid[T]) QueryAABB(dst []T, minX, minY, maxX, maxY float64) []T {
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	minCell := g.cellFor(minX, minY)
	maxCell := g.cellFor(maxX, maxY)
	for y := minCell.y; y <= maxCell.y; y++ {
		for x := minCell.x; x <= maxCell.x; x++ {
			dst = append(dst, g.cells[spatialCell{x: x, y: y}]...)
		}
	}
	return dst
}
