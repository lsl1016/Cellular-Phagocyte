package game

import "testing"

func TestSpatialGridQueryAABB(t *testing.T) {
	g := newSpatialGrid[string](100)
	g.Insert(10, 10, "a")
	g.Insert(150, 10, "b")
	g.Insert(250, 250, "c")

	got := g.QueryAABB(nil, 0, 0, 199, 99)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %v", len(got), got)
	}
	seen := map[string]bool{}
	for _, v := range got {
		seen[v] = true
	}
	if !seen["a"] || !seen["b"] || seen["c"] {
		t.Fatalf("unexpected query result: %v", got)
	}
}

func TestSpatialGridHandlesNegativeCoordinates(t *testing.T) {
	g := newSpatialGrid[int](100)
	g.Insert(-10, -10, 1)
	g.Insert(10, 10, 2)

	got := g.QueryAABB(nil, -50, -50, -1, -1)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected only negative-cell value, got %v", got)
	}
}

func TestSpatialGridNormalizesReversedBounds(t *testing.T) {
	g := newSpatialGrid[string](100)
	g.Insert(25, 25, "inside")

	got := g.QueryAABB(nil, 99, 99, 0, 0)
	if len(got) != 1 || got[0] != "inside" {
		t.Fatalf("reversed bounds should still query correctly, got %v", got)
	}
}
