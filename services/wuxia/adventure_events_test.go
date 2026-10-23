package wuxia

import "testing"

func TestAdventureRewardContracts(t *testing.T) {
	c := DefaultCatalog()
	total, negative := 0, 0
	for _, e := range c.Events {
		total += e.Weight
		if e.Kind == "setback" {
			negative += e.Weight
		}
		for _, balance := range []int64{0, 3, 100} {
			isolated := c
			isolated.Events = []Event{e}
			p := Character{Name: "游侠", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": c.Starter("剑").ID}, Silver: balance}
			r, err := isolated.Explore(&p, DefaultSettings(), 42)
			if err != nil {
				t.Fatal(err)
			}
			if p.Silver < 0 || p.Silver-balance != r.Silver {
				t.Fatalf("%s incorrect debit: %+v", e.ID, r)
			}
			if e.Kind == "mentor" && (r.Silver != 0 || r.Loot != "" || r.Experience <= 0) {
				t.Fatalf("mentor rewards: %+v", r)
			}
			if e.Kind == "setback" && (r.Silver > 0 || r.Experience != 0 || r.Loot != "" || r.Battle != nil) {
				t.Fatalf("setback rewards: %+v", r)
			}
		}
	}
	if negative*4 != total {
		t.Fatalf("negative probability %d/%d, want 25%%", negative, total)
	}
}

func TestAdventureExpectedReturnsStayPositive(t *testing.T) {
	c := DefaultCatalog()
	for _, prof := range c.Professions {
		for _, level := range []int{1, 5, 10} {
			var silver, xp int64
			losses := 0
			for seed := 0; seed < 1000; seed++ {
				p := Character{Name: "游侠", ProfessionID: prof.ID, WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: c.Starter(prof.WeaponKind).ID}, Silver: 100, Experience: XPForLevel(level)}
				r, err := c.Explore(&p, DefaultSettings(), uint64(seed))
				if err != nil {
					t.Fatal(err)
				}
				silver += r.Silver
				xp += r.Experience
				if r.Kind == "setback" {
					losses++
				}
			}
			t.Logf("%s level %d: mean silver %.2f, xp %.2f, setbacks %d/1000", prof.ID, level, float64(silver)/1000, float64(xp)/1000, losses)
			if silver <= 0 || level < c.MaxLevel && xp <= 0 || losses < 200 || losses > 300 {
				t.Fatal("adventure economy outside expected range")
			}
		}
	}
}
