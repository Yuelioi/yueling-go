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
		return skill.Kind == "mind" || p.WeaponKind == skill.WeaponKind
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
	return categoryEntries(c, pool, entries)
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

// Allocate fixed category budgets after eligibility, so adding a manual cannot dilute equipment odds.
func categoryEntries(c Catalog, pool DrawPool, entries []DrawEntry) []DrawEntry {
	if len(pool.CategoryWeights) == 0 {
		return entries
	}
	groups := map[string][]int{}
	for i, e := range entries {
		item, _ := c.Item(e.Reward.ItemID)
		groups[item.Kind] = append(groups[item.Kind], i)
	}
	for kind, indices := range groups {
		budget := pool.CategoryWeights[kind] * 10000
		total := 0
		for _, i := range indices {
			total += entries[i].Weight
		}
		if total == 0 {
			continue
		}
		assigned := 0
		for _, i := range indices {
			entries[i].Weight = budget * entries[i].Weight / total
			assigned += entries[i].Weight
		}
		for n := 0; n < budget-assigned; n++ {
			entries[indices[n%len(indices)]].Weight++
		}
	}
	out := entries[:0]
	for _, e := range entries {
		if e.Weight > 0 {
			out = append(out, e)
		}
	}
	return out
}
