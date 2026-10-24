package wuxia

import "testing"

func TestEquippedCharmBoostsOnlyPositiveAdventureRewards(t *testing.T) {
	c := sixCatalog(t)
	var charm Item
	for _, item := range c.Items {
		if item.Kind == "charm" && item.Tier == 5 {
			charm = item
			break
		}
	}
	prof := c.Professions[0]
	for _, kind := range []string{"gift", "mentor", "setback"} {
		t.Run(kind, func(t *testing.T) {
			base := upgradePlayer(c, prof, 1)
			base.Silver = 1000
			equipped := upgradePlayer(c, prof, 1)
			equipped.Silver = 1000
			equipped.Equipment = map[string]string{"charm": charm.ID}
			event := c.Events[0]
			event.Kind = kind
			event.LootChance = 0
			event.Silver = [2]int{100, 100}
			event.Experience = 100
			if kind == "mentor" {
				event.Silver = [2]int{}
			}
			if kind == "setback" {
				event.Silver = [2]int{-100, -100}
				event.Experience = 0
			}
			c.Events = []Event{event}
			a, err := c.Explore(&base, DefaultSettings(), 17)
			if err != nil {
				t.Fatal(err)
			}
			b, err := c.Explore(&equipped, DefaultSettings(), 17)
			if err != nil {
				t.Fatal(err)
			}
			silver, xp := a.Silver, a.Experience
			if silver > 0 {
				silver += silver * 25 / 100
			}
			if xp > 0 {
				xp += xp * 25 / 100
			}
			if b.Silver != silver || b.Experience != xp {
				t.Fatalf("%s: got %d/%d want %d/%d", kind, b.Silver, b.Experience, silver, xp)
			}
		})
	}
}

func TestCandidateHealthAndLegendaryWeaponBudget(t *testing.T) {
	c := sixCatalog(t)
	for _, p := range c.Professions {
		if p.Base.HP < 900 || p.Base.HP > 1150 {
			t.Fatal(p.Name, "unexpected starting health", p.Base.HP)
		}
		var named, legendary Weapon
		for _, w := range c.Weapons {
			if w.Kind == p.WeaponKind {
				if w.Tier == 4 {
					named = w
				}
				if w.Tier == 5 {
					legendary = w
				}
			}
		}
		if legendary.Attack*100 < named.Attack*145 {
			t.Fatal(p.Name, "legendary upgrade too small")
		}
	}
}

func TestUnreinforcedCandidatePreviewUsesFullWeaponStats(t *testing.T) {
	c := sixCatalog(t)
	for _, prof := range c.Professions {
		for _, w := range c.Weapons {
			if w.Kind != prof.WeaponKind || w.MaxWeaponLevel != 0 {
				continue
			}
			preview, err := c.PreviewBuild(PreviewBuild{Profession: prof.ID, Level: 1, Weapon: w.ID})
			if err != nil {
				t.Fatal(err)
			}
			p := upgradePlayer(c, prof, 1)
			p.Weapons[prof.WeaponKind] = w.ID
			actual, err := c.View(p, DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			if preview.Stats != actual.Stats {
				t.Fatal(w.Name, "preview and actual stats differ", preview.Stats, actual.Stats)
			}
		}
	}
}
