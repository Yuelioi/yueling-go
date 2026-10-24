package wuxia

import (
	"encoding/json"
	"math"
	"testing"
)

func TestDuplicateConversionAndOverflowAtomicity(t *testing.T) {
	c := DefaultCatalog()
	p := upgradePlayer(c, c.Professions[0], 10)
	item, _ := c.Item("剑-1")
	rewards := []Reward{{ItemID: item.ID, Quantity: 2}, {ItemID: item.ID, Quantity: 1}}
	start := p.Silver
	if err := c.Grant(&p, rewards); err != nil {
		t.Fatal(err)
	}
	want := ItemPrice(item) * 40 / 100 * 3
	if p.Inventory[item.ID] != 1 || p.Silver != start+want || rewards[0].ConvertedQuantity != 2 || rewards[1].ConvertedQuantity != 1 {
		t.Fatalf("duplicate settlement: %+v %+v", p, rewards)
	}
	p.LearnedSkills = map[string]bool{"sword-rain": true}
	manual, _ := c.Item("manual-sword-rain")
	start = p.Silver
	if err := c.Grant(&p, []Reward{{ItemID: manual.ID, Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	if p.Inventory[manual.ID] != 0 || p.Silver != start+ItemPrice(manual)*40/100 {
		t.Fatal("learned manual was retained")
	}
	p.Silver = math.MaxInt64
	before, _ := json.Marshal(p)
	if err := c.Grant(&p, []Reward{{ItemID: item.ID, Quantity: 1}}); err == nil {
		t.Fatal("overflow accepted")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("failed grant mutated character")
	}
}

func TestAutoEquipmentPreservesSameTierChoice(t *testing.T) {
	c := sixCatalog(t)
	p := upgradePlayer(c, c.Professions[0], 1)
	for _, w := range c.Weapons {
		if w.Kind == p.WeaponKind {
			p.Inventory[w.ID] = 1
		}
	}
	c.AutoEquip(&p)
	equipped, _ := c.Weapon(p.Weapons[p.WeaponKind])
	if equipped.Tier != 5 {
		t.Fatal("did not equip highest quality")
	}
	for _, w := range c.Weapons {
		if w.Kind == p.WeaponKind && w.Tier == 5 {
			p.Weapons[w.Kind] = w.ID
			c.AutoEquip(&p)
			if p.Weapons[w.Kind] != w.ID {
				t.Fatal("same tier choice replaced")
			}
		}
	}
}

func TestDuplicateEconomyCannotSelfFundDraws(t *testing.T) {
	for _, c := range []Catalog{DefaultCatalog(), sixCatalog(t)} {
		for _, prof := range c.Professions {
			for level := 1; level <= c.MaxLevel; level++ {
				for _, pool := range c.Pools {
					if pool.CostItem != "silver" {
						continue
					}
					total, returned := 0, float64(0)
					for _, e := range c.EligibleEntries(pool, prof.ID, level) {
						item, _ := c.Item(e.Reward.ItemID)
						total += e.Weight
						returned += float64(e.Weight) * float64(ItemPrice(item)*40/100*e.Reward.Quantity)
					}
					if total > 0 && returned/float64(total) >= float64(pool.Cost) {
						t.Fatalf("refund loop for %s: %f >= %d", prof.Name, returned/float64(total), pool.Cost)
					}
				}
			}
		}
	}
}
