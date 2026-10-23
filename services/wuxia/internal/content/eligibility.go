package content

import "slices"

func (c Catalog) Allowed(item Item, professionID string, level int) bool {
	if level < item.MinLevel || item.MinLevel > c.MaxLevel {
		return false
	}
	if _, ok := c.Profession(professionID); !ok {
		return false
	}
	if len(item.Professions) > 0 && !slices.Contains(item.Professions, professionID) {
		return false
	}
	if item.Kind == "manual" {
		skill, ok := c.Skill(item.SkillID)
		if !ok || level < skill.MinLevel {
			return false
		}
		p, _ := c.Profession(professionID)
		return p.WeaponKind == skill.WeaponKind
	}
	return true
}

func (c Catalog) EligibleEntries(pool DrawPool, professionID string, level int) []DrawEntry {
	entries := make([]DrawEntry, 0, len(pool.Entries))
	for _, entry := range pool.Entries {
		item, ok := c.Item(entry.Reward.ItemID)
		if ok && c.Allowed(item, professionID, level) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (c Catalog) StarterFor(professionID string) Weapon {
	p, ok := c.Profession(professionID)
	if !ok {
		return Weapon{}
	}
	for _, w := range c.Weapons {
		if w.Kind == p.WeaponKind && w.MinLevel == 1 && slices.Contains(w.Professions, professionID) {
			return w
		}
	}
	return Weapon{}
}
