package wuxia

import (
	"fmt"
	"math/rand/v2"
)

type WeaponQualityChance struct {
	Tier   int    `json:"tier"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

func WeaponPreviewChances() []WeaponQualityChance {
	return []WeaponQualityChance{{1, "凡品", 6000}, {2, "精制", 2800}, {3, "上品", 1000}, {4, "名器", 180}, {5, "神兵", 20}}
}

// First choose quality, then an eligible definition. Adding items cannot dilute quality odds.
func PreviewWeaponDraw(profession string, count int, seed uint64) ([]Weapon, error) {
	c := EquipmentPreviewCatalog()
	if _, ok := c.Profession(profession); !ok || (count != 1 && count != 10) {
		return nil, fmt.Errorf("职业无效或试抽次数不是1/10")
	}
	groups := map[int][]Weapon{}
	for _, w := range c.Weapons {
		item, _ := c.Item(w.ID)
		if c.Allowed(item, profession, 1) {
			groups[w.Tier] = append(groups[w.Tier], w)
		}
	}
	chances := WeaponPreviewChances()
	for _, ch := range chances {
		if len(groups[ch.Tier]) == 0 {
			return nil, fmt.Errorf("奖池品质%d缺少本职业武器", ch.Tier)
		}
	}
	rng := rand.New(rand.NewPCG(seed, 0xd7803))
	result := make([]Weapon, 0, count)
	for range count {
		roll := rng.IntN(10000)
		for _, ch := range chances {
			roll -= ch.Weight
			if roll < 0 {
				group := groups[ch.Tier]
				result = append(result, weaponAtLevel(group[rng.IntN(len(group))], 1))
				break
			}
		}
	}
	return result, nil
}
