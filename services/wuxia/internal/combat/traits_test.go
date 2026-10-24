package combat

import (
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
	"testing"
)

func TestAttributeEventsConserveHealth(t *testing.T) {
	skill := content.Skill{ID: "multi", Kind: "martial", Chance: 1, Hits: 3, Damage: 1.5, Cooldown: 1}
	for seed := range 1000 {
		a := Fighter{Stats: Stats{HP: 500, Attack: 55, Defense: 20, Speed: 25}, Traits: Traits{Combo: .5, Poison: .3, LifeSteal: .25, CriticalDamage: .3}, Martial: &skill}
		b := a
		b.Stats.Speed = 17
		result := Duel(a, b, uint64(seed))
		hp := [2]int{500, 500}
		pending := [2]int{}
		for _, act := range result.Actions {
			i, j := act.Actor, 1-act.Actor
			if hp[i] <= 0 || hp[j] <= 0 {
				t.Fatal("dead fighter acted")
			}
			maxHits := 1
			if act.SkillID != "" {
				maxHits = 3
			}
			if act.Combo {
				maxHits++
			}
			if len(act.Hits) > maxHits {
				t.Fatal("recursive pursuit")
			}
			sum := 0
			for _, h := range act.Hits {
				sum += h
			}
			if act.Guarded && act.Damage > 0 && act.Effect != "招架" {
				t.Fatal("guard did not cover whole action")
			}
			if sum != act.Damage {
				t.Fatal("direct damage mismatch")
			}
			if act.LifeSteal > int(float64(act.Damage)*.25) || act.Healing != act.LifeSteal {
				t.Fatal("recovery used poison or overkill")
			}
			hp[i] += act.Healing
			hp[j] -= act.Damage
			if hp[j] > 0 {
				if act.PoisonApplied != int(float64(act.Damage)*.3) {
					t.Fatal("poison source mismatch")
				}
				pending[j] += act.PoisonApplied
				if act.PoisonDamage != min(hp[i], pending[i]) {
					t.Fatal("poison did not settle after target's action")
				}
				pending[i] = 0
				hp[i] -= act.PoisonDamage
			} else if act.PoisonDamage != 0 || act.PoisonApplied != 0 {
				t.Fatal("effects continued after lethal hit")
			}
			if hp != act.Remaining || hp[i] > 500 || hp[j] < 0 {
				t.Fatalf("health mismatch: %+v", act)
			}
		}
		if hp != result.Remaining || (result.Winner >= 0 && hp[1-result.Winner] != 0) {
			t.Fatal("invalid settlement")
		}
	}
}

func TestGuardOncePerActionAndCriticalDamage(t *testing.T) {
	a := Fighter{Stats: Stats{HP: 100000, Attack: 60, Speed: 20}, Martial: &content.Skill{ID: "triple", Kind: "martial", Chance: 1, Hits: 3, Damage: 1, Cooldown: 1}}
	b := a
	b.Traits.Guard = .3
	for seed := range 100 {
		first := Duel(a, b, uint64(seed))
		a.Traits.CriticalDamage = .5
		second := Duel(a, b, uint64(seed))
		a.Traits.CriticalDamage = 0
		if first.Actions[0].Guarded != second.Actions[0].Guarded {
			t.Fatal("crit damage changed guard roll")
		}
		if first.Actions[0].Actor == 0 && second.Actions[0].Damage < first.Actions[0].Damage {
			t.Fatal("critical damage reduced direct damage")
		}
	}
}
