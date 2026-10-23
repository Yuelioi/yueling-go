package wuxia

import (
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/assets"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/gameplay"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/progression"
	"math"
	"sort"
)

func (c Catalog) Grant(p *Character, rewards []Reward) error {
	additions := map[string]int64{}
	for _, reward := range rewards {
		item, ok := c.Item(reward.ItemID)
		if !ok || item.ID != reward.ItemID || reward.Quantity < 1 || reward.Quantity > 1000000 {
			return rulef("奖励物品或数量无效")
		}
		if additions[item.ID] > math.MaxInt64-reward.Quantity || p.Inventory[item.ID] > math.MaxInt64-additions[item.ID]-reward.Quantity {
			return rulef("物品数量超出上限")
		}
		additions[item.ID] += reward.Quantity
	}
	_, next, err := assets.Settle(p.Silver, p.Inventory, assets.Settlement{Rewards: additions})
	if err != nil {
		return rulef("%s", err)
	}
	p.Inventory = next
	return nil
}

func (c Catalog) Equip(p *Character, id string) error {
	item, ok := c.Item(id)
	if !ok || p.Inventory[item.ID] < 1 {
		return rulef("背包中没有这件装备")
	}
	if progression.Level(p.Experience, c.MaxLevel) < item.MinLevel {
		return rulef("%s需%d级才能装备", item.Name, item.MinLevel)
	}
	if !c.Allowed(item, p.ProfessionID, progression.Level(p.Experience, c.MaxLevel)) {
		return rulef("当前职业不能装备%s", item.Name)
	}
	if item.Kind == "weapon" {
		w, _ := c.Weapon(item.ID)
		if p.Weapons == nil {
			p.Weapons = map[string]string{}
		}
		p.Weapons[w.Kind] = w.ID
		p.WeaponKind = w.Kind
		return nil
	}
	if item.Kind != "armor" && item.Kind != "charm" {
		return rulef("这件物品不能装备")
	}
	if p.Equipment == nil {
		p.Equipment = map[string]string{}
	}
	p.Equipment[item.Kind] = item.ID
	return nil
}

func (c Catalog) Learn(p *Character, id string) error {
	item, ok := c.Item(id)
	if !ok || item.Kind != "manual" || p.Inventory[item.ID] < 1 {
		return rulef("需要背包中的武学秘籍，可发送「江湖背包」查看")
	}
	skill, ok := c.Skill(item.SkillID)
	if !ok {
		return rulef("秘籍对应武学不可用")
	}
	if p.LearnedSkills[skill.ID] {
		return rulef("已学会%s，秘籍保留在背包", skill.Name)
	}
	if progression.Level(p.Experience, c.MaxLevel) < max(item.MinLevel, skill.MinLevel) {
		return rulef("这本秘籍需%d级才能学习", max(item.MinLevel, skill.MinLevel))
	}
	if !c.Allowed(item, p.ProfessionID, progression.Level(p.Experience, c.MaxLevel)) {
		return rulef("这本秘籍不适合当前职业")
	}
	if p.LearnedSkills == nil {
		p.LearnedSkills = map[string]bool{}
	}
	p.Inventory[item.ID]--
	p.LearnedSkills[skill.ID] = true
	if skill.WeaponKind == p.WeaponKind {
		p.ActiveSkill = skill.ID
	}
	return nil
}

func (c Catalog) Draw(p *Character, poolID string, count int, seed uint64) ([]Reward, error) {
	pool, ok := c.Pool(poolID)
	if !ok {
		return nil, rulef("奖池不存在，发送「江湖奖池」查看")
	}
	if count != 1 && count != 5 {
		return nil, rulef("每次可寻宝1次或5次")
	}
	cost := pool.Cost * int64(count)
	if pool.CostItem == "silver" {
		if p.Silver < cost {
			return nil, rulef("寻宝需%d银两，当前%d", cost, p.Silver)
		}
	} else if p.Inventory[pool.CostItem] < cost {
		return nil, rulef("抽奖券不足")
	}
	entries := c.EligibleEntries(pool, p.ProfessionID, progression.Level(p.Experience, c.MaxLevel))
	rewards, err := gameplay.Draw(entries, count, seed)
	if err != nil {
		return nil, rulef("%s", err)
	}
	if p.Draws[pool.ID] > math.MaxInt64-int64(count) {
		return nil, rulef("寻宝次数超出上限")
	}
	change := assets.Settlement{Costs: map[string]int64{}, Rewards: map[string]int64{}}
	for _, r := range rewards {
		change.Rewards[r.ItemID] += r.Quantity
	}
	if pool.CostItem == "silver" {
		change.SilverCost = cost
	} else {
		change.Costs[pool.CostItem] = cost
	}
	silver, inventory, err := assets.Settle(p.Silver, p.Inventory, change)
	if err != nil {
		return nil, rulef("%s", err)
	}
	p.Silver, p.Inventory = silver, inventory
	if p.Draws == nil {
		p.Draws = map[string]int64{}
	}
	p.Draws[pool.ID] += int64(count)
	return rewards, nil
}

func (c Catalog) autoEquipReward(p *Character, id string) {
	item, _ := c.Item(id)
	if !c.Allowed(item, p.ProfessionID, progression.Level(p.Experience, c.MaxLevel)) {
		return
	}
	if item.Kind == "weapon" {
		w, _ := c.Weapon(id)
		old, ok := c.Weapon(p.Weapons[w.Kind])
		oldItem, _ := c.Item(old.ID)
		if !ok || !c.Allowed(oldItem, p.ProfessionID, progression.Level(p.Experience, c.MaxLevel)) || betterItem(item, Item{Tier: old.Tier, MinLevel: old.MinLevel, ID: old.ID}) {
			if p.Weapons == nil {
				p.Weapons = map[string]string{}
			}
			p.Weapons[w.Kind] = w.ID
		}
	} else if item.Kind == "armor" || item.Kind == "charm" {
		old, ok := c.Item(p.Equipment[item.Kind])
		if !ok || betterItem(item, old) {
			if p.Equipment == nil {
				p.Equipment = map[string]string{}
			}
			p.Equipment[item.Kind] = item.ID
		}
	}
}

func betterItem(a, b Item) bool {
	if a.MinLevel != b.MinLevel {
		return a.MinLevel > b.MinLevel
	}
	if a.Tier != b.Tier {
		return a.Tier > b.Tier
	}
	return a.ID < b.ID
}

func (c Catalog) AutoEquip(p *Character) {
	ids := make([]string, 0, len(p.Inventory))
	for id, n := range p.Inventory {
		if n > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		c.autoEquipReward(p, id)
	}
}
