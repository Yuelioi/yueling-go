package combat

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const BattleVersion = "combat-1"
const MaxRounds = 12

func Fight(left, right Fighter, seed uint64) Battle {
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	b := Battle{Version: BattleVersion, Seed: seed, Left: left, Right: right, Winner: -1, Remaining: [2]int{left.Stats.HP, right.Stats.HP}}
	fighters := [2]Fighter{left, right}
	for i := range b.Form {
		b.Form[i] = .65 + .8*rng.Float64()
	}
	for round := 1; round <= MaxRounds; round++ {
		b.Rounds = round
		first := 0
		if float64(left.Stats.Speed)*(.75+.5*rng.Float64())*b.Form[0] < float64(right.Stats.Speed)*(.75+.5*rng.Float64())*b.Form[1] {
			first = 1
		}
		for _, i := range []int{first, 1 - first} {
			j := 1 - i
			a, d := fighters[i], fighters[j]
			dodge := min(.24, .04+d.Traits.Dodge+.05*float64(d.Stats.Speed)/float64(max(1, a.Stats.Speed+d.Stats.Speed)))
			if rng.Float64() < dodge {
				if len(b.Highlights) < 5 {
					b.Highlights = append(b.Highlights, fmt.Sprintf("%s侧身避开%s的一招", d.Name, a.Name))
				}
				continue
			}
			defense := float64(d.Stats.Defense) * (1 - min(.6, a.Traits.Pierce))
			damage := float64(a.Stats.Attack) * 80 / (80 + defense) * b.Form[i] * (.75 + .5*rng.Float64())
			effect := ""
			if rng.Float64() < min(.4, .1+a.Traits.Critical) {
				damage *= 1.75
				effect = "抓住破绽"
			}
			if rng.Float64() < min(.4, .08+d.Traits.Guard) {
				damage *= .55
				effect = "被格挡"
			}
			hit := max(1, int(math.Round(damage)))
			b.Remaining[j] = max(0, b.Remaining[j]-hit)
			if effect != "" && len(b.Highlights) < 5 {
				b.Highlights = append(b.Highlights, fmt.Sprintf("%s%s，造成%d伤害", a.Name, effect, hit))
			}
			if b.Remaining[j] == 0 {
				b.Winner = i
				return b
			}
		}
	}
	l := float64(b.Remaining[0]) / float64(max(1, left.Stats.HP))
	r := float64(b.Remaining[1]) / float64(max(1, right.Stats.HP))
	if math.Abs(l-r) > .000001 {
		if l > r {
			b.Winner = 0
		} else {
			b.Winner = 1
		}
	}
	return b
}

const DuelVersion = "duel-techniques-2"
const duelVitality = 12
const MinDuelWinProbability = .1
const MaxDuelWinProbability = .9

func DuelWinProbability(left, right Fighter) float64 {
	strength := func(a, d Fighter) float64 {
		defense := float64(max(0, d.Stats.Defense)) * (1 - min(.6, a.Traits.Pierce))
		damage := float64(max(1, a.Stats.Attack)) * 80 / (80 + defense)
		damage *= (1 + .75*min(.4, .1+a.Traits.Critical)) * (1 - .45*min(.4, .08+d.Traits.Guard))
		damage *= 1 - min(.24, .04+d.Traits.Dodge+.05*float64(d.Stats.Speed)/float64(max(1, a.Stats.Speed+d.Stats.Speed)))
		return math.Log(float64(max(1, a.Stats.HP))) + math.Log(max(.001, damage)) + .25*math.Log(float64(max(1, a.Stats.Speed)))
	}
	p := 1 / (1 + math.Exp(1.5*(strength(right, left)-strength(left, right))))
	return max(MinDuelWinProbability, min(MaxDuelWinProbability, p))
}

type duelOutcome struct {
	weight float64
	damage int
	effect string
	move   int
}

func duelOutcomes(a, d Fighter) []duelOutcome {
	outcomes := []duelOutcome{
		{min(.28, .1+a.Traits.Critical), 3, "暴击", 1},
		{min(.22, .04+a.Traits.Pierce), 4, "破防", 2},
		{min(.25, .08+d.Traits.Guard), 1, "招架", 0},
		{min(.2, .05+d.Traits.Dodge), 1, "卸力", 0},
	}
	normal := 1.0
	for _, o := range outcomes {
		normal -= o.weight
	}
	return append(outcomes, duelOutcome{normal, 2, "命中", 0})
}

// Each strike lowers vitality. This finite-state calculation calibrates tempo,
// so the complete HP battle (not a displayed percentage) obeys the win bounds.
func duelChance(q float64, outcomes [2][]duelOutcome) float64 {
	var chance [duelVitality + 1][duelVitality + 1]float64
	for hp := 1; hp <= duelVitality; hp++ {
		chance[hp][0] = 1
	}
	for left := 1; left <= duelVitality; left++ {
		for right := 1; right <= duelVitality; right++ {
			for _, o := range outcomes[0] {
				chance[left][right] += q * o.weight * chance[left][max(0, right-o.damage)]
			}
			for _, o := range outcomes[1] {
				chance[left][right] += (1 - q) * o.weight * chance[max(0, left-o.damage)][right]
			}
		}
	}
	return chance[duelVitality][duelVitality]
}

func Duel(left, right Fighter, seed uint64) Battle {
	p := DuelWinProbability(left, right)
	outcomes := [2][]duelOutcome{duelOutcomes(left, right), duelOutcomes(right, left)}
	lo, hi := 0.0, 1.0
	for range 32 {
		q := (lo + hi) / 2
		if duelChance(q, outcomes) < p {
			lo = q
		} else {
			hi = q
		}
	}
	q := (lo + hi) / 2
	rng := rand.New(rand.NewPCG(seed, 0x4f1bbcdc6765a901))
	b := Battle{Version: DuelVersion, Seed: seed, Left: left, Right: right, Winner: -1, Remaining: [2]int{max(1, left.Stats.HP), max(1, right.Stats.HP)}, WinProbability: [2]float64{p, 1 - p}}
	fighters := [2]Fighter{left, right}
	vitality := [2]int{duelVitality, duelVitality}
	for vitality[0] > 0 && vitality[1] > 0 {
		actor := 1
		if rng.Float64() < q {
			actor = 0
		}
		defender := 1 - actor
		roll := rng.Float64()
		selected := outcomes[actor][len(outcomes[actor])-1]
		for _, o := range outcomes[actor] {
			roll -= o.weight
			if roll < 0 {
				selected = o
				break
			}
		}
		before := b.Remaining[defender]
		vitality[defender] = max(0, vitality[defender]-selected.damage)
		b.Remaining[defender] = (max(1, fighters[defender].Stats.HP)*vitality[defender] + duelVitality - 1) / duelVitality
		technique := fighters[actor].Techniques[selected.move]
		action := CombatAction{Actor: actor, Technique: technique, Weapon: fighters[actor].Weapon, Effect: selected.effect, Damage: before - b.Remaining[defender], Remaining: b.Remaining}
		b.Actions = append(b.Actions, action)
		b.Highlights = append(b.Highlights, fmt.Sprintf("%s使%s「%s」，%s%s，气血−%d", fighters[actor].Name, action.Weapon, action.Technique, fighters[defender].Name, CombatReaction(action.Effect), action.Damage))
		b.Rounds++
		if vitality[defender] == 0 {
			b.Winner = actor
		}
	}
	return b
}

func CombatReaction(effect string) string {
	switch effect {
	case "暴击":
		return "露出破绽"
	case "破防":
		return "架势被破"
	case "招架":
		return "架招挡下大半力道"
	case "卸力":
		return "侧身卸去劲力"
	default:
		return "中招后退"
	}
}
