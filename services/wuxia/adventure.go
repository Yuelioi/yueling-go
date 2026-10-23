package wuxia

import (
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/progression"
	"math/rand/v2"
)

func (c Catalog) Explore(p *Character, settings Settings, seed uint64) (Encounter, error) {
	rng := rand.New(rand.NewPCG(seed, 0xd1b54a32d192ed03))
	v, err := c.View(*p, settings)
	if err != nil {
		return Encounter{}, err
	}
	var available []Event
	total := 0
	for _, e := range c.Events {
		if e.MinLevel <= v.Level {
			available = append(available, e)
			total += e.Weight
		}
	}
	if total == 0 {
		return Encounter{}, fmt.Errorf("当前等级没有可用历练")
	}
	pick := rng.IntN(total)
	e := available[0]
	for _, candidate := range available {
		pick -= candidate.Weight
		if pick < 0 {
			e = candidate
			break
		}
	}
	r := Encounter{Kind: e.Kind, EventID: e.ID, Title: e.Title, Scene: e.Scene, Text: e.Text, Silver: int64(e.Silver[0] + rng.IntN(e.Silver[1]-e.Silver[0]+1)), Experience: int64(e.Experience)}
	won := true
	if e.Kind == "combat" {
		s := v.Stats
		scale := e.Difficulty * (.87 + .26*rng.Float64())
		s.HP = max(1, int(float64(s.HP)*scale))
		s.Attack = max(1, int(float64(s.Attack)*scale))
		s.Defense = max(1, int(float64(s.Defense)*scale))
		s.Speed = max(1, int(float64(s.Speed)*scale))
		battle := Fight(v.Fighter(), Fighter{Name: e.Title + "的对手", Stats: s}, rng.Uint64())
		r.Battle = &battle
		won = battle.Winner == 0
		if !won {
			r.Silver = max(1, r.Silver/2)
			r.Experience = max(1, r.Experience/2)
			r.Text += " 此番未能取胜，仍带回一些见闻与酬金。"
		}
	}
	if won && rng.Float64() < e.LootChance {
		var pool []Weapon
		for _, w := range c.Weapons {
			item, _ := c.Item(w.ID)
			if c.Allowed(item, p.ProfessionID, v.Level) {
				pool = append(pool, w)
			}
		}
		w := pool[rng.IntN(len(pool))]
		if err := c.Grant(p, []Reward{{ItemID: w.ID, Quantity: 1}}); err != nil {
			return Encounter{}, err
		}
		c.AutoEquip(p)
		r.Loot = w.Name + "（收入背包）"
		if p.WeaponKind == w.Kind && p.Weapons[w.Kind] == w.ID {
			r.Loot = w.Name + "（已装备）"
		}
	}
	if r.Silver < -p.Silver {
		r.Silver = -p.Silver
		r.Text += " 你翻遍钱袋，只付出了身上剩下的银两。"
	}
	p.Silver += r.Silver
	r.Experience = progression.Gain(p.Experience, r.Experience, c.MaxLevel)
	p.Experience += r.Experience
	c.AutoEquip(p)
	return r, nil
}
