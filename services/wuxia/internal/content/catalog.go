package content

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sync"
)

//go:embed content.json
var content []byte

type Stats struct {
	HP      int `json:"hp"`
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Speed   int `json:"speed"`
}

type Traits struct {
	CriticalDamage float64 `json:"critical_damage,omitempty"` // Additive bonus to the base 1.75 multiplier.
	Combo          float64 `json:"combo,omitempty"`
	Poison         float64 `json:"poison,omitempty"`
	LifeSteal      float64 `json:"life_steal,omitempty"`
	Critical       float64 `json:"critical"`
	Dodge          float64 `json:"dodge"`
	Guard          float64 `json:"guard"`
	Pierce         float64 `json:"pierce"`
}

type Profession struct {
	Base        Stats  `json:"base"`
	PerLevel    Stats  `json:"per_level"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	WeaponKind  string `json:"weapon_kind"`
	Traits      Traits `json:"traits"`
	Portrait    string `json:"portrait"`
}

type Weapon struct {
	Affinity       string    `json:"affinity,omitempty"`
	Image          string    `json:"image,omitempty"`
	Description    string    `json:"description,omitempty"`
	MaxWeaponLevel int       `json:"max_weapon_level,omitempty"`
	WeaponLevel    int       `json:"weapon_level,omitempty"`
	Professions    []string  `json:"professions"`
	Stats          Stats     `json:"stats"`
	Techniques     [3]string `json:"techniques"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Tier           int       `json:"tier"`
	MinLevel       int       `json:"min_level"`
	Attack         int       `json:"attack"`
	Traits         Traits    `json:"traits"`
}

type Event struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Kind       string  `json:"kind"`
	Scene      string  `json:"scene"`
	Weight     int     `json:"weight"`
	MinLevel   int     `json:"min_level"`
	Difficulty float64 `json:"difficulty"`
	Silver     [2]int  `json:"silver"`
	Experience int     `json:"experience"`
	LootChance float64 `json:"loot_chance"`
	Text       string  `json:"text"`
}

type AdventureBonus struct {
	SilverPercent     int `json:"silver_percent"`
	ExperiencePercent int `json:"experience_percent"`
}

type Item struct {
	AdventureBonus AdventureBonus `json:"adventure_bonus,omitempty"`
	ManualKind     string         `json:"manual_kind,omitempty"`
	Image          string         `json:"image,omitempty"`
	Description    string         `json:"description,omitempty"`
	Professions    []string       `json:"professions,omitempty"`
	WeaponKind     string         `json:"weapon_kind,omitempty"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Kind           string         `json:"kind"`
	Tier           int            `json:"tier"`
	MinLevel       int            `json:"min_level"`
	Stats          Stats          `json:"stats"`
	Traits         Traits         `json:"traits"`
	SkillID        string         `json:"skill_id,omitempty"`
}

// Skill stores either a profession technique or a universal inner practice.
type Skill struct {
	ReadyTurn     int     `json:"ready_turn,omitempty"`
	GuaranteeTurn int     `json:"guarantee_turn,omitempty"`
	Kind          string  `json:"kind,omitempty"`
	Tier          int     `json:"tier,omitempty"`
	Affinity      string  `json:"affinity,omitempty"`
	Chance        float64 `json:"chance,omitempty"`
	Every         int     `json:"every,omitempty"`
	Hits          int     `json:"hits,omitempty"`
	Damage        float64 `json:"damage,omitempty"`
	Pierce        float64 `json:"pierce,omitempty"`
	Cooldown      int     `json:"cooldown,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	MindEffect    string  `json:"mind_effect,omitempty"`
	Bonus         Stats   `json:"bonus,omitempty"`

	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	WeaponKind    string `json:"weapon_kind"`
	MinLevel      int    `json:"min_level"`
	TechniqueSlot int    `json:"technique_slot"`
	Traits        Traits `json:"traits"`
}

type Reward struct {
	ConvertedSilver   int64  `json:"converted_silver,omitempty"`
	ConvertedQuantity int64  `json:"converted_quantity,omitempty"`
	ItemID            string `json:"item_id"`
	Quantity          int64  `json:"quantity"`
}

type DrawEntry struct {
	Reward Reward `json:"reward"`
	Weight int    `json:"weight"`
}

type DrawPool struct {
	CategoryWeights map[string]int `json:"category_weights,omitempty"`
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CostItem        string         `json:"cost_item"`
	Cost            int64          `json:"cost"`
	Entries         []DrawEntry    `json:"entries"`
}

type Appearance struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Catalog struct {
	MaxLevel    int          `json:"max_level"`
	Items       []Item       `json:"items"`
	Skills      []Skill      `json:"skills"`
	Pools       []DrawPool   `json:"pools"`
	DuelScene   string       `json:"duel_scene"`
	Appearances []Appearance `json:"appearances"`
	Scenes      []Appearance `json:"scenes"`
	Version     string       `json:"version"`
	Professions []Profession `json:"professions"`
	Weapons     []Weapon     `json:"weapons"`
	Events      []Event      `json:"events"`
}

var defaultCatalog = sync.OnceValue(func() Catalog {
	c, err := Parse(content)
	if err != nil {
		panic(err)
	}
	return c
})

func Default() Catalog { return cloneCatalog(defaultCatalog()) }
func cloneCatalog(source Catalog) Catalog {
	raw, err := json.Marshal(source)
	if err != nil {
		panic(err)
	}
	var c Catalog
	if err = json.Unmarshal(raw, &c); err != nil {
		panic(err)
	}
	return c
}
func LevelCap() int { return defaultCatalog().MaxLevel }

func Parse(data []byte) (Catalog, error) {
	var c Catalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&c)
	if err != nil {
		return c, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("内容定义只能有一个JSON文档")
	}
	if c.MaxLevel < 1 || c.MaxLevel > 1000 || c.Version == "" || len(c.Professions) == 0 || len(c.Weapons) == 0 || len(c.Events) == 0 {
		return c, fmt.Errorf("武侠内容缺少版本或定义")
	}
	appearances := map[string]bool{}
	scenes := map[string]bool{}
	for _, a := range c.Appearances {
		if a.ID == "" || a.Name == "" || appearances[a.ID] {
			return c, fmt.Errorf("形象定义无效")
		}
		appearances[a.ID] = true
	}
	for _, a := range c.Scenes {
		if a.ID == "" || a.Name == "" || scenes[a.ID] {
			return c, fmt.Errorf("场景定义无效")
		}
		scenes[a.ID] = true
	}
	if !scenes[c.DuelScene] {
		return c, fmt.Errorf("缺少切磋场景")
	}
	seen := map[string]bool{}
	for _, p := range c.Professions {
		if p.ID == "" || p.Name == "" || seen[p.ID] || !appearances[p.Portrait] {
			return c, fmt.Errorf("职业定义无效: %s", p.ID)
		}
		seen[p.ID] = true
		if !validTraits(p.Traits) || p.Base.HP < 1 || p.Base.Attack < 1 || p.Base.Defense < 0 || p.Base.Speed < 1 || p.PerLevel.HP < 1 || p.PerLevel.Attack < 1 || p.PerLevel.Defense < 0 || p.PerLevel.Speed < 0 {
			return c, fmt.Errorf("职业特性无效: %s", p.ID)
		}
	}
	seen = map[string]bool{}
	kinds := map[string]bool{}
	for _, w := range c.Weapons {
		if w.ID == "" || w.Name == "" || w.Kind == "" || seen[w.ID] || w.Tier < 1 || w.MaxWeaponLevel < 0 || w.MaxWeaponLevel > 100 || w.MinLevel < 1 || w.MinLevel > c.MaxLevel || len(w.Professions) == 0 || w.Stats.HP < 0 || w.Stats.Attack != 0 || w.Stats.Defense < 0 || w.Stats.Speed < 0 || w.Attack < 0 || !validTraits(w.Traits) {
			return c, fmt.Errorf("武器定义无效: %s", w.ID)
		}
		for _, name := range w.Techniques {
			if name == "" {
				return c, fmt.Errorf("武器缺少招式: %s", w.ID)
			}
		}
		for _, id := range w.Professions {
			p, ok := c.Profession(id)
			if !ok || p.ID != id || p.WeaponKind != w.Kind {
				return c, fmt.Errorf("武器职业限制无效: %s", w.ID)
			}
		}
		seen[w.ID] = true
		if w.MinLevel == 1 {
			kinds[w.Kind] = true
		}
	}
	for _, p := range c.Professions {
		if !kinds[p.WeaponKind] || c.StarterFor(p.ID).ID == "" {
			return c, fmt.Errorf("职业缺少初始武器: %s", p.ID)
		}
	}
	seen = map[string]bool{}
	for _, e := range c.Events {
		if e.ID == "" || seen[e.ID] || e.Weight < 1 || e.Weight > 10000 || e.MinLevel < 1 || e.MinLevel > c.MaxLevel || !scenes[e.Scene] || (e.Kind != "setback" && e.Silver[0] < 0) || e.Silver[1] < e.Silver[0] || e.Experience < 0 || e.LootChance < 0 || e.LootChance > 1 || math.IsNaN(e.LootChance) || math.IsNaN(e.Difficulty) || e.Difficulty < 0.5 || e.Difficulty > 2 {
			return c, fmt.Errorf("事件定义无效: %s", e.ID)
		}
		if e.Kind != "combat" && e.Kind != "treasure" && e.Kind != "mentor" && e.Kind != "setback" {
			return c, fmt.Errorf("未知事件类型: %s", e.Kind)
		}
		if e.Kind == "mentor" && (e.Silver != [2]int{} || e.LootChance != 0) {
			return c, fmt.Errorf("指点事件只能给阅历: %s", e.ID)
		}
		if e.Kind == "setback" && (e.Silver[1] >= 0 || e.Experience != 0 || e.LootChance != 0) {
			return c, fmt.Errorf("破财事件奖励无效: %s", e.ID)
		}
		seen[e.ID] = true
	}
	if err := c.validateInventoryContent(); err != nil {
		return c, err
	}
	return c, nil
}

func validTraits(t Traits) bool {
	for _, v := range []float64{t.Critical, t.Dodge, t.Guard, t.Pierce, t.CriticalDamage, t.Combo, t.Poison, t.LifeSteal} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 0.5 {
			return false
		}
	}
	return true
}

func (c Catalog) Profession(id string) (Profession, bool) {
	for _, p := range c.Professions {
		if p.ID == id || p.Name == id {
			return p, true
		}
	}
	return Profession{}, false
}
func (c Catalog) Weapon(id string) (Weapon, bool) {
	for _, w := range c.Weapons {
		if w.ID == id || w.Name == id {
			return w, true
		}
	}
	return Weapon{}, false
}
func (c Catalog) Starter(kind string) Weapon {
	for _, w := range c.Weapons {
		if w.Kind == kind && w.MinLevel == 1 {
			return w
		}
	}
	panic("validated catalog missing starter")
}

func (c Catalog) Item(id string) (Item, bool) {
	if w, ok := c.Weapon(id); ok {
		return Item{ID: w.ID, Name: w.Name, Kind: "weapon", Tier: w.Tier, MinLevel: w.MinLevel, Traits: w.Traits, Professions: w.Professions, WeaponKind: w.Kind, Stats: Stats{HP: w.Stats.HP, Attack: w.Attack, Defense: w.Stats.Defense, Speed: w.Stats.Speed}}, true
	}
	for _, item := range c.Items {
		if item.ID == id || item.Name == id {
			return item, true
		}
	}
	return Item{}, false
}
func (c Catalog) Skill(id string) (Skill, bool) {
	for _, skill := range c.Skills {
		if skill.ID == id || skill.Name == id {
			return skill, true
		}
	}
	return Skill{}, false
}
func (c Catalog) Pool(id string) (DrawPool, bool) {
	for _, pool := range c.Pools {
		if pool.ID == id || pool.Name == id {
			return pool, true
		}
	}
	return DrawPool{}, false
}

func (c Catalog) validateInventoryContent() error {
	ids, names := map[string]bool{}, map[string]bool{}
	for _, w := range c.Weapons {
		ids[w.ID] = true
		names[w.Name] = true
	}
	skills := map[string]bool{}
	for _, s := range c.Skills {
		if s.ID == "" || s.Name == "" || skills[s.ID] || skills[s.Name] || s.MinLevel < 1 || s.MinLevel > c.MaxLevel || s.TechniqueSlot < 0 || s.TechniqueSlot > 2 || !validTraits(s.Traits) {
			return fmt.Errorf("武学定义无效: %s", s.ID)
		}
		if s.Kind != "mind" && c.starterKind(s.WeaponKind) == "" {
			return fmt.Errorf("武学兵器类型无效: %s", s.ID)
		}
		if err := validateSkill(s); err != nil {
			return err
		}
		skills[s.ID] = true
		skills[s.Name] = true
	}
	for _, item := range c.Items {
		b := item.AdventureBonus
		if b.SilverPercent < 0 || b.SilverPercent > 25 || b.ExperiencePercent < 0 || b.ExperiencePercent > 25 || (item.Kind != "charm" && b != (AdventureBonus{})) {
			return fmt.Errorf("历练收益词条无效: %s", item.ID)
		}
		if item.ID == "" || item.Name == "" || ids[item.ID] || names[item.Name] || ids[item.Name] || names[item.ID] || item.ID == "silver" || item.MinLevel < 1 || item.MinLevel > c.MaxLevel || item.Tier < 1 || !validTraits(item.Traits) || item.Stats.HP < 0 || item.Stats.Attack < 0 || item.Stats.Defense < 0 || item.Stats.Speed < 0 {
			return fmt.Errorf("物品定义无效: %s", item.ID)
		}
		for _, id := range item.Professions {
			p, ok := c.Profession(id)
			if !ok || p.ID != id {
				return fmt.Errorf("物品职业限制无效: %s", item.ID)
			}
		}
		switch item.Kind {
		case "armor", "charm", "ticket":
			if item.SkillID != "" {
				return fmt.Errorf("非秘籍物品不能引用武学: %s", item.ID)
			}
		case "manual":
			if skill, ok := c.Skill(item.SkillID); !ok || skill.ID != item.SkillID {
				return fmt.Errorf("秘籍缺少对应武学: %s", item.ID)
			}
		default:
			return fmt.Errorf("未知物品类型: %s", item.Kind)
		}
		ids[item.ID] = true
		names[item.Name] = true
	}
	pools := map[string]bool{}
	for _, p := range c.Pools {
		if p.ID == "" || p.Name == "" || pools[p.ID] || pools[p.Name] || p.Cost < 1 || p.Cost > 100000 || len(p.Entries) == 0 {
			return fmt.Errorf("奖池定义无效: %s", p.ID)
		}
		pools[p.ID] = true
		pools[p.Name] = true
		if p.CostItem != "silver" {
			item, ok := c.Item(p.CostItem)
			if !ok || item.Kind != "ticket" {
				return fmt.Errorf("奖池消耗必须是银两或抽奖券: %s", p.ID)
			}
		}
		for kind, weight := range p.CategoryWeights {
			if (kind != "weapon" && kind != "armor" && kind != "charm" && kind != "manual") || weight < 1 || weight > 100 {
				return fmt.Errorf("奖池类别权重无效: %s", p.ID)
			}
		}
		total := 0
		for _, entry := range p.Entries {
			item, ok := c.Item(entry.Reward.ItemID)
			if !ok || item.ID != entry.Reward.ItemID || entry.Reward.Quantity < 1 || entry.Reward.Quantity > 1000 || entry.Weight < 1 || entry.Weight > 100000 {
				return fmt.Errorf("奖池奖励无效: %s", p.ID)
			}
			total += entry.Weight
			if total > 1000000 {
				return fmt.Errorf("奖池权重过大: %s", p.ID)
			}
		}
	}
	return nil
}

func (c Catalog) starterKind(kind string) string {
	for _, w := range c.Weapons {
		if w.Kind == kind && w.MinLevel == 1 {
			return w.ID
		}
	}
	return ""
}
