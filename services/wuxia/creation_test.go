package wuxia

import (
	"math/rand/v2"
	"testing"
	"time"
)

func TestNewCharacterRandomProfessionAndMatchingStarter(t *testing.T) {
	c := DefaultCatalog()
	if len(c.Professions) != 6 || c.Version != sixCatalog(t).Version {
		t.Fatal("production catalog must use the complete six-profession rules")
	}
	{
		rng := rand.New(rand.NewPCG(12, 34))
		counts := map[string]int{}
		for i := 0; i < 6000; i++ {
			p := c.newCharacter(100, int64(i+1), "新侠客", time.Unix(1, 0), rng)
			prof, ok := c.Profession(p.ProfessionID)
			if !ok {
				t.Fatal("unknown profession")
			}
			starter := c.StarterFor(prof.ID)
			if p.WeaponKind != prof.WeaponKind || p.Weapons[prof.WeaponKind] != starter.ID || p.Inventory[starter.ID] != 1 || len(p.Inventory) != 1 {
				t.Fatalf("invalid starter: %+v", p)
			}
			if _, err := c.View(p, DefaultSettings()); err != nil {
				t.Fatal(err)
			}
			counts[prof.ID]++
		}
		mean := 6000 / len(c.Professions)
		for _, prof := range c.Professions {
			if counts[prof.ID] < mean*8/10 || counts[prof.ID] > mean*12/10 {
				t.Fatalf("profession distribution: %v", counts)
			}
		}
	}
}
