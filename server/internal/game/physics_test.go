package game

import "testing"

const eps = 1e-6

func almostEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestRadius(t *testing.T) {
	// radius = sqrt(mass) * radiusFactor
	got := Radius(100, 2.5)
	want := 25.0 // sqrt(100)=10, 再乘以 2.5
	if !almostEqual(got, want) {
		t.Fatalf("Radius(100,2.5) = %v, want %v", got, want)
	}
}

func TestSpeedDecreasesWithMass(t *testing.T) {
	base := 200.0
	baseMass := 20.0
	// 基础质量时，速度等于 base
	if got := Speed(base, baseMass, baseMass); !almostEqual(got, base) {
		t.Fatalf("Speed at base mass = %v, want %v", got, base)
	}
	// 质量为 4 倍时，速度减半（1/sqrt(4)）
	if got := Speed(base, 4*baseMass, baseMass); !almostEqual(got, base/2) {
		t.Fatalf("Speed at 4x mass = %v, want %v", got, base/2)
	}
	// 质量低于基础质量时，速度被限制为 base（永远不会快于 base）
	if got := Speed(base, baseMass/2, baseMass); !almostEqual(got, base) {
		t.Fatalf("Speed below base mass = %v, want %v", got, base)
	}
}

func TestDistance(t *testing.T) {
	if got := Distance(0, 0, 3, 4); !almostEqual(got, 5) {
		t.Fatalf("Distance = %v, want 5", got)
	}
}

func TestCanEatFood(t *testing.T) {
	// 当距离 <= ballRadius + foodRadius 时食物被吃掉
	if !CanEatFood(10, 2, 11) {
		t.Fatal("expected food eaten when dist within radii sum")
	}
	if CanEatFood(10, 2, 13) {
		t.Fatal("expected food NOT eaten when dist beyond radii sum")
	}
}

func TestCanEatPlayer(t *testing.T) {
	const (
		eatMassRatio   = 1.2
		eatDepthFactor = 0.4
	)
	// 攻击方质量 100，目标质量 50 -> 100 >= 50*1.2=60 满足
	// 攻击方半径 25，目标半径 17.7 -> 需要 dist <= 25 - 17.7*0.4 = 17.92
	aMass, tMass := 100.0, 50.0
	aR := Radius(aMass, 2.5) // 25
	tR := Radius(tMass, 2.5) // 约 17.677
	depth := aR - tR*eatDepthFactor

	if !CanEatPlayer(aMass, tMass, aR, tR, depth-0.1, eatMassRatio, eatDepthFactor) {
		t.Fatal("expected eat when mass and depth conditions met")
	}
	// 质量比例不满足：目标太重
	if CanEatPlayer(aMass, 90, aR, Radius(90, 2.5), 0, eatMassRatio, eatDepthFactor) {
		t.Fatal("expected no eat when mass ratio not met")
	}
	// 距离不满足：相距太远
	if CanEatPlayer(aMass, tMass, aR, tR, depth+5, eatMassRatio, eatDepthFactor) {
		t.Fatal("expected no eat when too far")
	}
}

func TestPlayerEatGain(t *testing.T) {
	// 攻击方获得 target.mass * playerEatMassGainRatio 的质量
	if got := PlayerEatGain(50, 0.8); !almostEqual(got, 40) {
		t.Fatalf("PlayerEatGain = %v, want 40", got)
	}
}
