package wuxia

import "github.com/Yuelioi/yueling-go/services/wuxia/internal/combat"

const BattleVersion = combat.BattleVersion
const DuelVersion = combat.DuelVersion
const MaxRounds = combat.MaxRounds
const MinDuelWinProbability = combat.MinDuelWinProbability
const MaxDuelWinProbability = combat.MaxDuelWinProbability

func Fight(l, r Fighter, seed uint64) Battle  { return combat.Fight(l, r, seed) }
func Duel(l, r Fighter, seed uint64) Battle   { return combat.Duel(l, r, seed) }
func DuelWinProbability(l, r Fighter) float64 { return combat.DuelWinProbability(l, r) }
func CombatReaction(effect string) string     { return combat.CombatReaction(effect) }
