package wuxia

import (
	"fmt"

	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
)

type PreviewBuild struct {
	WeaponLevel int    `json:"weapon_level"`
	Profession  string `json:"profession"`
	Level       int    `json:"level"`
	Mastery     int    `json:"mastery"`
	Weapon      string `json:"weapon"`
	Armor       string `json:"armor"`
	Charm       string `json:"charm"`
	Skill       string `json:"skill"`
}

type PreviewResult struct {
	Version        string    `json:"version"`
	Actor          View      `json:"actor"`
	Opponent       View      `json:"opponent"`
	WinProbability float64   `json:"win_probability"`
	Events         [2]Traits `json:"events"`
}

func EquipmentPreviewCatalog() Catalog { return Catalog{content.EquipmentPreview()} }

func (c Catalog) PreviewBuild(b PreviewBuild) (View, error) {
	prof, ok := c.Profession(b.Profession)
	if !ok || b.Level < 1 || b.Level > c.MaxLevel || b.Mastery < 0 || b.Mastery > b.Level {
		return View{}, fmt.Errorf("职业、等级或修为无效")
	}
	if b.Weapon == "" {
		b.Weapon = c.StarterFor(prof.ID).ID
	}
	weapon, exists := c.Weapon(b.Weapon)
	if !exists {
		return View{}, fmt.Errorf("武器不存在")
	}
	if b.WeaponLevel == 0 {
		b.WeaponLevel = 1
	}
	if b.WeaponLevel < 1 || b.WeaponLevel > weapon.MaxWeaponLevel {
		return View{}, fmt.Errorf("武器等级须在1～%d之间", weapon.MaxWeaponLevel)
	}
	// Clone before applying the requested weapon level; the catalog remains immutable.
	c.Weapons = append([]Weapon(nil), c.Weapons...)
	for i, w := range c.Weapons {
		if w.ID == weapon.ID {
			c.Weapons[i] = weaponAtLevel(w, b.WeaponLevel)
		}
	}
	p := Character{Name: prof.Name, ProfessionID: prof.ID, Experience: XPForLevel(b.Level), Mastery: b.Mastery, WeaponKind: prof.WeaponKind, Weapons: map[string]string{}, Inventory: map[string]int64{}, Equipment: map[string]string{}, LearnedSkills: map[string]bool{}}
	for slot, id := range map[string]string{"weapon": b.Weapon, "armor": b.Armor, "charm": b.Charm} {
		if id == "" {
			continue
		}
		item, ok := c.Item(id)
		if !ok || item.Kind != slot || !c.Allowed(item, prof.ID, b.Level) {
			return View{}, fmt.Errorf("%s不符合职业、槽位或等级要求", id)
		}
		p.Inventory[item.ID] = 1
		if err := c.Equip(&p, item.ID); err != nil {
			return View{}, err
		}
	}
	if b.Skill != "" {
		skill, ok := c.Skill(b.Skill)
		if !ok || skill.WeaponKind != prof.WeaponKind || skill.MinLevel > b.Level {
			return View{}, fmt.Errorf("武学不符合当前角色")
		}
		p.LearnedSkills[skill.ID] = true
		p.ActiveSkill = skill.ID
	}
	return c.View(p, DefaultSettings())
}

func PreviewEquipment(left, right PreviewBuild) (PreviewResult, error) {
	c := EquipmentPreviewCatalog()
	a, err := c.PreviewBuild(left)
	if err != nil {
		return PreviewResult{}, err
	}
	b, err := c.PreviewBuild(right)
	if err != nil {
		return PreviewResult{}, err
	}
	result := PreviewResult{Version: c.Version, Actor: a, Opponent: b, WinProbability: DuelWinProbability(a.Fighter(), b.Fighter())}
	for i, v := range []View{a, b} {
		t := v.Fighter().Traits
		result.Events[i] = Traits{Critical: min(.28, .1+t.Critical), Pierce: min(.22, .04+t.Pierce), Guard: min(.25, .08+t.Guard), Dodge: min(.2, .05+t.Dodge)}
	}
	return result, nil
}

// Definitions hold level-10 stats. Training grows base attributes, not proc chances.
func weaponAtLevel(w Weapon, level int) Weapon {
	w.WeaponLevel = level
	scale := func(n int) int { return (n*(50+5*level) + 50) / 100 }
	w.Attack = scale(w.Attack)
	w.Stats.HP = scale(w.Stats.HP)
	w.Stats.Defense = scale(w.Stats.Defense)
	w.Stats.Speed = scale(w.Stats.Speed)
	return w
}
