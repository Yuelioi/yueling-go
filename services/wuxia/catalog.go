package wuxia

import (
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/progression"
	"math"
)

type Catalog struct{ content.Catalog }

func DefaultCatalog() Catalog { return Catalog{content.Default()} }
func ParseCatalog(data []byte) (Catalog, error) {
	c, err := content.Parse(data)
	return Catalog{c}, err
}
func Level(xp int64) int         { return progression.Level(xp, content.LevelCap()) }
func XPForLevel(level int) int64 { return progression.XPForLevel(level) }

func (c Catalog) View(p Character, settings Settings) (View, error) {
	prof, ok := c.Profession(p.ProfessionID)
	if !ok {
		return View{}, fmt.Errorf("角色职业不可用")
	}
	weapon, ok := c.Weapon(p.Weapons[p.WeaponKind])
	if !ok {
		return View{}, fmt.Errorf("角色武器不可用")
	}
	l := progression.Level(p.Experience, c.MaxLevel)
	item, _ := c.Item(weapon.ID)
	if !c.Allowed(item, prof.ID, l) {
		return View{}, rulef("当前兵器不符合职业或等级要求")
	}
	stats := progression.Stats(prof, l, p.Mastery)
	stats.HP += weapon.Stats.HP
	stats.Attack += weapon.Attack
	stats.Defense += weapon.Stats.Defense + p.Armor*3
	stats.Speed += weapon.Stats.Speed + p.Charm*2
	var armorItem, charmItem *Item
	for _, slot := range []string{"armor", "charm"} {
		id := p.Equipment[slot]
		if id == "" {
			continue
		}
		item, ok := c.Item(id)
		if !ok || item.Kind != slot || !c.Allowed(item, prof.ID, l) {
			return View{}, fmt.Errorf("角色装备不可用")
		}
		stats.HP += item.Stats.HP
		stats.Attack += item.Stats.Attack
		stats.Defense += item.Stats.Defense
		stats.Speed += item.Stats.Speed
		if slot == "armor" {
			armorItem = &item
		} else {
			charmItem = &item
		}
	}
	var activeSkill *Skill
	if p.ActiveSkill != "" {
		skill, ok := c.Skill(p.ActiveSkill)
		if !ok || !p.LearnedSkills[skill.ID] {
			return View{}, fmt.Errorf("角色武学不可用")
		}
		if skill.WeaponKind == weapon.Kind && l >= skill.MinLevel {
			activeSkill = &skill
		}
	}
	title := "初入江湖"
	if l >= 5 {
		title = "略有小成"
	}
	if l >= 12 {
		title = "声名鹊起"
	}
	if l >= 25 {
		title = "名动一方"
	}
	if l >= 50 {
		title = "一代宗师"
	}
	collection := []Weapon{}
	for _, w := range c.Weapons {
		if p.Weapons[w.Kind] == w.ID {
			collection = append(collection, w)
		}
	}
	return View{Collection: collection, ArmorItem: armorItem, CharmItem: charmItem, Skill: activeSkill, Character: p, Level: l, MaxLevel: c.MaxLevel, NextXP: XPForLevel(min(l+1, c.MaxLevel)), Title: title, Profession: prof, Weapon: weapon, Stats: stats, Power: Power(stats), PracticeCost: int64(settings.PracticeBase + 20*p.Mastery)}, nil
}
func Power(s Stats) int {
	return int(math.Round(float64(s.HP)/5 + 2*float64(s.Attack) + 1.5*float64(s.Defense) + float64(s.Speed)))
}
func (v View) Fighter() Fighter {
	t := v.Profession.Traits
	t.Critical += v.Weapon.Traits.Critical
	t.Dodge += v.Weapon.Traits.Dodge
	t.Guard += v.Weapon.Traits.Guard
	t.Pierce += v.Weapon.Traits.Pierce
	techniques := v.Weapon.Techniques
	bonuses := []Traits{}
	for _, item := range []*Item{v.ArmorItem, v.CharmItem} {
		if item != nil {
			bonuses = append(bonuses, item.Traits)
		}
	}
	if v.Skill != nil {
		bonuses = append(bonuses, v.Skill.Traits)
		techniques[v.Skill.TechniqueSlot] = v.Skill.Name
	}
	for _, bonus := range bonuses {
		t.Critical += bonus.Critical
		t.Dodge += bonus.Dodge
		t.Guard += bonus.Guard
		t.Pierce += bonus.Pierce
	}
	portrait := v.Character.Portrait
	if portrait == "" {
		portrait = v.Profession.Portrait
	}
	return Fighter{Techniques: techniques, Portrait: portrait, Name: v.Character.Name, Profession: v.Profession.Name, Weapon: v.Weapon.Name, Stats: v.Stats, Traits: t}
}

func (s Settings) Validate() error {
	if s.DailyRuns < 1 || s.DailyRuns > 12 || s.SignSilver < 0 || s.SignSilver > 10000 || s.PracticeBase < 1 || s.PracticeBase > 10000 {
		return fmt.Errorf("历练次数须1–12，签到银两0–10000，练武基础花费1–10000")
	}
	return nil
}
