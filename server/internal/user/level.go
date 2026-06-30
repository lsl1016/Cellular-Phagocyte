package user

// levelThresholds[i] 是达到第 (i+1) 级所需的累计经验值。
// MVP 经验表：L1=0, L2=100, L3=300, L4=600, L5=1000。
var levelThresholds = []int64{0, 100, 300, 600, 1000}

// LevelForExp 根据总经验返回等级，上限为表中最高等级。
func LevelForExp(exp int64) int {
	level := 1
	for i, threshold := range levelThresholds {
		if exp >= threshold {
			level = i + 1
		}
	}
	return level
}

// NextLevelExp 返回下一等级的经验阈值。已满级时返回最高阈值（无更高等级可升）。
func NextLevelExp(exp int64) int64 {
	level := LevelForExp(exp)
	if level >= len(levelThresholds) {
		return levelThresholds[len(levelThresholds)-1]
	}
	return levelThresholds[level]
}
