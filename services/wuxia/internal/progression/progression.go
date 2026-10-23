package progression

import (
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
	"math"
)

func Level(experience int64, cap int) int {
	return min(cap, 1+int((math.Sqrt(1+4*float64(max(0, experience))/30)-1)/2))
}
func XPForLevel(level int) int64 { n := int64(level); return 30 * n * (n - 1) }
func Gain(experience, reward int64, cap int) int64 {
	return min(max(0, reward), max(int64(0), XPForLevel(cap)-experience))
}
func Stats(p content.Profession, level, mastery int) content.Stats {
	practice := 1 + .015*float64(min(level, max(0, mastery)))
	value := func(base, growth int) int { return int(float64(base+(level-1)*growth) * practice) }
	return content.Stats{HP: value(p.Base.HP, p.PerLevel.HP), Attack: value(p.Base.Attack, p.PerLevel.Attack), Defense: value(p.Base.Defense, p.PerLevel.Defense), Speed: value(p.Base.Speed, p.PerLevel.Speed)}
}
