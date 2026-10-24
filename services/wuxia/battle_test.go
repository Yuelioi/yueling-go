package wuxia

import (
	"encoding/json"
	"math"
	"testing"
)

func TestBattleUpsetsAndProfessionBalance(t *testing.T) {
	c := DefaultCatalog()
	settings := DefaultSettings()
	for _, prof := range c.Professions {
		p := Character{Name: prof.Name, ProfessionID: prof.ID, Experience: 360, WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: c.Starter(prof.WeaponKind).ID}}
		v, err := c.View(p, settings)
		if err != nil {
			t.Fatal(err)
		}
		a := v.Fighter()
		a.Name = "甲"
		b := a
		b.Name = "乙"
		wins, lowWins := 0, 0
		for i := range 6000 {
			r := Fight(a, b, uint64(i))
			if r.Winner == 0 {
				wins++
			}
			if r.Rounds > MaxRounds {
				t.Fatal("unbounded fight")
			}
			weaker := a
			weaker.Stats.Attack = int(float64(a.Stats.Attack) * .8)
			weaker.Stats.HP = int(float64(a.Stats.HP) * .8)
			weaker.Stats.Defense = int(float64(a.Stats.Defense) * .8)
			weaker.Stats.Speed = int(float64(a.Stats.Speed) * .8)
			if Fight(weaker, b, uint64(i)).Winner == 0 {
				lowWins++
			}
		}
		equal := float64(wins) / 6000
		low := float64(lowWins) / 6000
		t.Logf("%s equal=%.3f lower_20pct=%.3f", prof.Name, equal, low)
		if math.Abs(equal-.5) > .04 || low < 0 || low >= equal {
			t.Fatalf("unbalanced combat equal=%f lower=%f", equal, low)
		}
		x, _ := json.Marshal(Fight(a, b, 78))
		y, _ := json.Marshal(Fight(a, b, 78))
		if string(x) != string(y) {
			t.Fatal("battle replay changed")
		}
	}
}

func TestRandomAdventuresKeepEconomyBounded(t *testing.T) {
	c := DefaultCatalog()
	seen := map[string]bool{}
	for seed := range 300 {
		p := Character{Name: "游侠", ProfessionID: c.Professions[0].ID, WeaponKind: "剑", Weapons: map[string]string{"剑": c.Starter("剑").ID}}
		r, err := c.Explore(&p, DefaultSettings(), uint64(seed))
		if err != nil {
			t.Fatal(err)
		}
		seen[r.EventID] = true
		if p.Silver != r.Silver || p.Experience != r.Experience || p.Silver < 0 || r.Battle != nil && r.Battle.Rounds > MaxRounds {
			t.Fatalf("bad encounter %+v", r)
		}
	}
	if len(seen) != len(c.Events) {
		t.Fatalf("event pool not reachable: %v", seen)
	}
	for level := 1; level <= c.MaxLevel; level++ {
		if Level(XPForLevel(level)) != level {
			t.Fatalf("wrong level at threshold %d", level)
		}
	}
}

func TestProfessionMatchupsStayPlayable(t *testing.T) {
	c := DefaultCatalog()
	var fighters []Fighter
	for _, p := range c.Professions {
		v, err := c.View(Character{Name: p.Name, ProfessionID: p.ID, Experience: 360, WeaponKind: p.WeaponKind, Weapons: map[string]string{p.WeaponKind: c.Starter(p.WeaponKind).ID}}, DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		fighters = append(fighters, v.Fighter())
	}
	for i, a := range fighters {
		for j, b := range fighters {
			if i >= j {
				continue
			}
			wins := 0
			for seed := range 6000 {
				if Fight(a, b, uint64(seed)).Winner == 0 {
					wins++
				}
			}
			rate := float64(wins) / 6000
			t.Logf("%s vs %s = %.3f", a.Profession, b.Profession, rate)
			if rate < .25 || rate > .75 {
				t.Fatal("profession dominates matchups")
			}
		}
	}
}

func TestCatalogCanAddProfessionAndWeaponWithoutBattleBranches(t *testing.T) {
	c := DefaultCatalog()
	c.Professions = append(c.Professions, Profession{Base: Stats{HP: 196, Attack: 29, Defense: 14, Speed: 20}, PerLevel: Stats{HP: 26, Attack: 5, Defense: 2, Speed: 2}, ID: "lancer", Name: "枪客", WeaponKind: "枪", Portrait: "boxer", Traits: Traits{Pierce: .05}})
	c.Weapons = append(c.Weapons, Weapon{Techniques: [3]string{"拦拿扎", "回马枪", "破阵枪"}, Professions: []string{"lancer"}, ID: "spear-1", Name: "长枪", Kind: "枪", Tier: 1, MinLevel: 1, Attack: 6})
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	c, err = ParseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.View(Character{Name: "新侠客", ProfessionID: "lancer", WeaponKind: "枪", Weapons: map[string]string{"枪": "spear-1"}}, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	r := Fight(v.Fighter(), v.Fighter(), 5)
	if r.Rounds < 1 || r.Rounds > MaxRounds {
		t.Fatal("extended profession did not participate")
	}
}

func TestDuelWholeMatchProbability(t *testing.T) {
	base := Fighter{Techniques: [3]string{"探云式", "流星赶月", "穿云一线"}, Weapon: "松纹剑", Name: "甲", Stats: Stats{HP: 300, Attack: 50, Defense: 30, Speed: 28}, Traits: Traits{Critical: .1}}
	peer := base
	peer.Name = "乙"
	weak := base
	weak.Stats = Stats{HP: 1, Attack: 1, Defense: 1, Speed: 1}
	strong := base
	strong.Stats = Stats{HP: 1000000, Attack: 1000000, Defense: 1000000, Speed: 1000000}
	medium := base
	medium.Stats = Stats{HP: 240, Attack: 40, Defense: 24, Speed: 22}
	for _, tt := range []struct {
		name        string
		left, right Fighter
		want        float64
	}{
		{"equal", base, peer, .5},
		{"weakest", weak, strong, DuelWinProbability(weak, strong)},
		{"strongest", strong, weak, DuelWinProbability(strong, weak)},
		{"lower20pct", medium, peer, DuelWinProbability(medium, peer)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := DuelWinProbability(tt.left, tt.right)
			if math.Abs(p-tt.want) > 1e-12 || math.Abs(p+DuelWinProbability(tt.right, tt.left)-1) > 1e-12 {
				t.Fatalf("probability %f want %f", p, tt.want)
			}
			wins := 0
			for seed := range 20000 {
				b := Duel(tt.left, tt.right, uint64(seed))
				if b.Version != DuelVersion || b.WinProbability[0] != p || b.Winner < 0 || b.Winner > 1 || b.Remaining[1-b.Winner] != 0 || b.Remaining[b.Winner] <= 0 || b.Rounds < 1 || b.Rounds > MaxRounds {
					t.Fatalf("inconsistent duel %+v", b)
				}
				hp := [2]int{tt.left.Stats.HP, tt.right.Stats.HP}
				for i, a := range b.Actions {
					target := 1 - a.Actor
					if hp[0] <= 0 || hp[1] <= 0 || a.Technique == "" || a.Effect == "" || a.Damage < 0 {
						t.Fatalf("invalid action %d: %+v", i, a)
					}
					hp[target] -= a.Damage
					if hp != a.Remaining {
						t.Fatal("damage does not match HP snapshot")
					}
				}
				if hp != b.Remaining {
					t.Fatal("final HP differs from actions")
				}
				if b.Winner == 0 {
					wins++
				}
			}
			rate := float64(wins) / 20000
			t.Logf("whole-match predicted=%.4f observed=%.4f", p, rate)
			if tt.name == "equal" && math.Abs(rate-.5) > .03 {
				t.Fatalf("actual chance %f differs from advertised %f", rate, p)
			}
			x, _ := json.Marshal(Duel(tt.left, tt.right, 89))
			y, _ := json.Marshal(Duel(tt.left, tt.right, 89))
			if string(x) != string(y) {
				t.Fatal("duel replay changed")
			}
		})
	}
	last := 0.0
	for factor := 1; factor <= 200; factor++ {
		a := base
		a.Stats = Stats{HP: factor * 3, Attack: factor, Defense: factor, Speed: factor}
		p := DuelWinProbability(a, peer)
		if p < last || p < MinDuelWinProbability || p > MaxDuelWinProbability {
			t.Fatalf("nonmonotonic or unbounded probability %f", p)
		}
		last = p
	}
}

func TestDuelTechniquesFollowEquippedWeapon(t *testing.T) {
	c := DefaultCatalog()
	seen := map[string]bool{}
	for _, prof := range c.Professions {
		for _, weapon := range c.Weapons {
			if weapon.Kind != prof.WeaponKind {
				continue
			}
			v, err := c.View(Character{Name: "月离", ProfessionID: prof.ID, Experience: XPForLevel(10), WeaponKind: weapon.Kind, Weapons: map[string]string{weapon.Kind: weapon.ID}}, DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			a := v.Fighter()
			b := a
			b.Name = "野良"
			for seed := range 40 {
				battle := Duel(a, b, uint64(seed))
				for _, act := range battle.Actions {
					if act.Weapon != weapon.Name {
						t.Fatal("attack ignored equipped weapon")
					}
					found := false
					for _, name := range weapon.Techniques {
						found = found || act.Technique == name
					}
					if !found {
						t.Fatalf("unlearned technique: %+v", act)
					}
					seen[act.Effect] = true
				}
			}
		}
	}
	for _, effect := range []string{"命中", "暴击", "招架", "闪避"} {
		if !seen[effect] {
			t.Fatalf("effect not reachable: %s", effect)
		}
	}
	c.Weapons[0].Techniques[0] = ""
	raw, _ := json.Marshal(c)
	if _, err := ParseCatalog(raw); err == nil {
		t.Fatal("weapon with missing techniques accepted")
	}
}
