package wuxia

import "testing"

func TestWeaponLevelIndependentFromCharacter(t *testing.T) {
	c := EquipmentPreviewCatalog()
	b := PreviewBuild{Profession: "swordsman", Level: 1, Weapon: "legend-xuanyuan", WeaponLevel: 1}
	low, err := c.PreviewBuild(b)
	if err != nil {
		t.Fatal(err)
	}
	b.WeaponLevel = 10
	high, err := c.PreviewBuild(b)
	if err != nil {
		t.Fatal(err)
	}
	if high.Level != 1 || high.Weapon.WeaponLevel != 10 || high.Stats.Attack <= low.Stats.Attack {
		t.Fatal("weapon level coupled to character or not applied")
	}
	b.WeaponLevel = 11
	if _, err := c.PreviewBuild(b); err == nil {
		t.Fatal("invalid weapon level accepted")
	}
	b.WeaponLevel = 1
	b.Profession = "blade"
	if _, err := c.PreviewBuild(b); err == nil {
		t.Fatal("profession restriction lost")
	}
	w, _ := c.Weapon("legend-xuanyuan")
	if high.Weapon.Attack != w.Attack {
		t.Fatal("max level differs from catalog budget")
	}
	if w.WeaponLevel != 0 {
		t.Fatal("definition mutated")
	}
}
func TestWeaponPreviewDrawQualityAndEligibility(t *testing.T) {
	counts := map[int]int{}
	for seed := range 1000 {
		items, err := PreviewWeaponDraw("swordsman", 10, uint64(seed))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range items {
			if w.Kind != "剑" || w.WeaponLevel != 1 {
				t.Fatal("draw eligibility/level")
			}
			counts[w.Tier]++
		}
	}
	if counts[5] < 5 || counts[5] > 40 {
		t.Fatalf("legendary distribution: %v", counts)
	}
	for _, q := range WeaponPreviewChances() {
		if counts[q.Tier] == 0 {
			t.Fatal("missing quality")
		}
	}
	for _, p := range []string{"blade", "boxer"} {
		if _, err := PreviewWeaponDraw(p, 10, 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PreviewWeaponDraw("swordsman", 100, 1); err == nil {
		t.Fatal("unbounded request")
	}
}
