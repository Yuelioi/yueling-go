package combat

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const BattleVersion = "combat-5"
const DuelVersion = "duel-6"
const MaxRounds = 20

// Probability is an attribute-only reference, never an input to settlement.
const MinDuelWinProbability = 0.0
const MaxDuelWinProbability = 1.0

func DuelWinProbability(l, r Fighter) float64 {
	strength := func(a, d Fighter) float64 {
		return math.Log(float64(max(1, a.Stats.HP))) + math.Log(float64(max(1, a.Stats.Attack))*80/(80+float64(max(0, d.Stats.Defense)))) + .15*math.Log(float64(max(1, a.Stats.Speed)))
	}
	return 1 / (1 + math.Exp(1.5*(strength(r, l)-strength(l, r))))
}
func Fight(l, r Fighter, seed uint64) Battle { return run(l, r, seed, BattleVersion) }
func Duel(l, r Fighter, seed uint64) Battle  { return run(l, r, seed, DuelVersion) }

type actorState struct {
	turns, used, last, received int
	mindUsed                    bool
	poison                      int
}

func run(left, right Fighter, seed uint64, version string) Battle {
	rng := rand.New(rand.NewPCG(seed, 0x4f1bbcdc6765a901))
	f := [2]Fighter{left, right}
	state := [2]actorState{}
	p := DuelWinProbability(left, right)
	b := Battle{Version: version, Seed: seed, Left: left, Right: right, Winner: -1, Remaining: [2]int{max(1, left.Stats.HP), max(1, right.Stats.HP)}, WinProbability: [2]float64{p, 1 - p}, Form: [2]float64{1, 1}}
	lastActor, streak := -1, 0
	for round := 0; round < MaxRounds; round++ {
		for action := 0; action < 2; action++ {
			i := 0
			leftSpeed, rightSpeed := math.Sqrt(float64(max(1, left.Stats.Speed))), math.Sqrt(float64(max(1, right.Stats.Speed)))
			if rng.Float64() >= leftSpeed/(leftSpeed+rightSpeed) {
				i = 1
			}
			if i == lastActor && streak >= 2 {
				i = 1 - i
			}
			if i == lastActor {
				streak++
			} else {
				lastActor, streak = i, 1
			}
			j := 1 - i
			a, d := f[i], f[j]
			st := &state[i]
			st.turns++
			act := CombatAction{Opening: version == DuelVersion && len(b.Actions) == 0, Actor: i, Weapon: a.Weapon, Technique: a.Techniques[0], Effect: "命中"}
			if act.Technique == "" {
				act.Technique = "平击"
			}
			if a.Mind != nil && a.Mind.MindEffect == "recover" && st.turns == 3 {
				act.Healing = min(max(1, int(float64(a.Stats.HP)*.05)), max(0, a.Stats.HP-b.Remaining[i]))
				b.Remaining[i] += act.Healing
				act.MindID = a.Mind.ID
			}
			hits, mult, pierce := 1, 1.0, 0.0
			sk := a.Martial
			if sk != nil && sk.Kind == "martial" && (sk.Limit == 0 || st.used < sk.Limit) && (st.used == 0 || st.turns-st.last > sk.Cooldown) && st.turns >= sk.ReadyTurn {
				trigger := sk.Every > 0 && st.turns%sk.Every == 0
				if sk.Chance > 0 {
					trigger = rng.Float64() < sk.Chance
					if st.used == 0 && sk.GuaranteeTurn > 0 && st.turns >= sk.GuaranteeTurn {
						trigger = true
					}
				}
				if trigger {
					st.used++
					st.last = st.turns
					hits = sk.Hits
					mult = sk.Damage
					pierce = sk.Pierce
					act.Technique = sk.Name
					act.SkillID = sk.ID
					if a.Affinity != "" && a.Affinity == sk.Affinity {
						mult *= 1.1
						act.Synergy = a.Synergy
					}
					if a.Mind != nil && a.Mind.MindEffect == "opening" && !st.mindUsed {
						mult *= 1.1
						st.mindUsed = true
						act.MindID = a.Mind.ID
					}
					if a.Mind != nil && a.Mind.MindEffect == "focus" && !st.mindUsed {
						pierce += .15
						st.mindUsed = true
						act.MindID = a.Mind.ID
					}
				}
			}
			// A pursuit is one extra normal hit, never a new turn or another skill.
			mainHits := hits
			if a.Traits.Combo > 0 && rng.Float64() < min(.5, a.Traits.Combo) {
				act.Combo = true
				hits++
			}
			act.Guarded = rng.Float64() < min(.4, .08+d.Traits.Guard)
			openingRaw := 0
			for h := 0; h < hits && b.Remaining[j] > 0; h++ {
				if h == mainHits {
					mult, pierce = 1, 0
				}
				dodge := min(.34, .14+d.Traits.Dodge+.05*float64(max(0, d.Stats.Speed))/float64(max(1, a.Stats.Speed+d.Stats.Speed)))
				if rng.Float64() < dodge {
					act.Hits = append(act.Hits, 0)
					if act.Damage == 0 {
						act.Effect = "闪避"
					}
					continue
				}
				defense := float64(max(0, d.Stats.Defense)) * (1 - min(.65, a.Traits.Pierce+pierce))
				damage := float64(max(1, a.Stats.Attack)) * 80 / (80 + defense) * mult * (.85 + .3*rng.Float64())
				effect := "命中"
				if pierce > 0 {
					effect = "破防"
				}
				if rng.Float64() < min(.4, .1+a.Traits.Critical) {
					damage *= 1.75 + min(.5, max(0, a.Traits.CriticalDamage))
					effect = "暴击"
				}
				if act.Guarded {
					damage *= .55
					effect = "招架"
				}
				if d.Mind != nil && d.Mind.MindEffect == "protect" && state[j].received == 0 {
					damage *= .8
					act.DefenderMindID = d.Mind.ID
				}
				state[j].received++
				dealt := max(1, int(math.Round(damage)))
				// Only the opening action of a duel is softened, including every segment.
				if version == DuelVersion && len(b.Actions) == 0 {
					before := openingRaw / 2
					openingRaw += dealt
					dealt = openingRaw/2 - before
				}
				dealt = min(b.Remaining[j], dealt)
				b.Remaining[j] -= dealt
				act.Damage += dealt
				if h == mainHits {
					act.ComboDamage = dealt
				}
				act.Hits = append(act.Hits, dealt)
				act.Effect = effect
			}
			// Only actual direct HP loss generates poison or recovery; neither can recurse.
			if act.Damage > 0 {
				act.LifeSteal = min(max(0, a.Stats.HP-b.Remaining[i]), int(float64(act.Damage)*min(.5, max(0, a.Traits.LifeSteal))))
				b.Remaining[i] += act.LifeSteal
				act.Healing += act.LifeSteal
				if b.Remaining[j] > 0 {
					act.PoisonApplied = int(float64(act.Damage) * min(.5, max(0, a.Traits.Poison)))
					state[j].poison += act.PoisonApplied
				}
			}
			// A direct kill ends combat before pending damage. Otherwise the acting
			// fighter suffers poison acquired before this turn, after its attack/recovery.
			if b.Remaining[j] > 0 {
				act.PoisonDamage = min(b.Remaining[i], st.poison)
				b.Remaining[i] -= act.PoisonDamage
				st.poison = 0
			}
			act.Remaining = b.Remaining
			b.Actions = append(b.Actions, act)
			b.Highlights = append(b.Highlights, fmt.Sprintf("%s使%s施展%s，气血−%d", a.Name, a.Weapon, act.Technique, act.Damage))
			b.Rounds = round + 1
			if b.Remaining[i] == 0 {
				b.Winner = j
				return b
			}
			if b.Remaining[j] == 0 {
				b.Winner = i
				return b
			}
		}
	}
	// Reaching the action cap is a draw; no fabricated finishing blow.
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
