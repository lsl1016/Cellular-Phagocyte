package user

import "testing"

func TestLevelForExp(t *testing.T) {
	cases := []struct {
		exp   int64
		level int
	}{
		{0, 1},
		{99, 1},
		{100, 2},
		{299, 2},
		{300, 3},
		{350, 3},
		{599, 3},
		{600, 4},
		{999, 4},
		{1000, 5},
		{1500, 5}, // 超过表范围则保持最高级
	}
	for _, c := range cases {
		if got := LevelForExp(c.exp); got != c.level {
			t.Fatalf("LevelForExp(%d) = %d, want %d", c.exp, got, c.level)
		}
	}
}

func TestNextLevelExp(t *testing.T) {
	// 经验 350（3 级）时，下一阈值为 600
	if got := NextLevelExp(350); got != 600 {
		t.Fatalf("NextLevelExp(350) = %d, want 600", got)
	}
	// 经验 0（1 级）时，下一阈值为 100
	if got := NextLevelExp(0); got != 100 {
		t.Fatalf("NextLevelExp(0) = %d, want 100", got)
	}
	// 满级时，下一阈值等于当前最高阈值（无更高等级）
	if got := NextLevelExp(2000); got != 1000 {
		t.Fatalf("NextLevelExp(2000) = %d, want 1000", got)
	}
}
