package game

import "math"

// Radius 根据质量计算球体半径：radius = sqrt(mass) * radiusFactor。
func Radius(mass, radiusFactor float64) float64 {
	return math.Sqrt(mass) * radiusFactor
}

// Speed 计算给定质量的移动速度。质量越大移动越慢：
// speed = baseSpeed / sqrt(mass/baseMass)，并限制不超过 baseSpeed。
func Speed(baseSpeed, mass, baseMass float64) float64 {
	if mass <= baseMass {
		return baseSpeed
	}
	return baseSpeed / math.Sqrt(mass/baseMass)
}

// Distance 返回两点之间的欧氏距离。
func Distance(ax, ay, bx, by float64) float64 {
	dx := ax - bx
	dy := ay - by
	return math.Sqrt(dx*dx + dy*dy)
}

// CanEatFood 判断球体是否与食物重叠：dist <= ballRadius + foodRadius。
func CanEatFood(ballRadius, foodRadius, dist float64) bool {
	return dist <= ballRadius+foodRadius
}

// CanEatPlayer 判断攻击方能否吞噬目标，需同时满足两个条件：
//   - 质量：attackerMass >= targetMass * eatMassRatio
//   - 重叠深度：dist <= attackerRadius - targetRadius * eatDepthFactor
func CanEatPlayer(attackerMass, targetMass, attackerRadius, targetRadius, dist, eatMassRatio, eatDepthFactor float64) bool {
	if attackerMass < targetMass*eatMassRatio {
		return false
	}
	return dist <= attackerRadius-targetRadius*eatDepthFactor
}

// PlayerEatGain 返回攻击方吞噬目标后获得的质量。
func PlayerEatGain(targetMass, gainRatio float64) float64 {
	return targetMass * gainRatio
}
