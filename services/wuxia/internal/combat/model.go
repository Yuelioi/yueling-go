package combat

import "github.com/Yuelioi/yueling-go/services/wuxia/internal/content"

type Stats = content.Stats
type Traits = content.Traits
type Fighter struct {
	Martial    *content.Skill `json:"martial,omitempty"`
	Mind       *content.Skill `json:"mind,omitempty"`
	Affinity   string         `json:"affinity,omitempty"`
	Synergy    string         `json:"synergy,omitempty"`
	Techniques [3]string      `json:"techniques,omitempty"`
	Portrait   string         `json:"portrait,omitempty"`
	Name       string         `json:"name"`
	Profession string         `json:"profession"`
	Weapon     string         `json:"weapon"`
	Stats      Stats          `json:"stats"`
	Traits     Traits         `json:"traits"`
}

type CombatAction struct {
	Combo         bool `json:"combo,omitempty"`
	ComboDamage   int  `json:"combo_damage,omitempty"`   // Included in Damage and last Hits element.
	LifeSteal     int  `json:"life_steal,omitempty"`     // Included in Healing.
	PoisonApplied int  `json:"poison_applied,omitempty"` // Pending damage on the defender.
	PoisonDamage  int  `json:"poison_damage,omitempty"`  // Delayed damage suffered by this actor.
	Guarded       bool `json:"guarded,omitempty"`

	Opening        bool   `json:"opening,omitempty"`
	DefenderMindID string `json:"defender_mind_id,omitempty"`
	Hits           []int  `json:"hits,omitempty"`
	Healing        int    `json:"healing,omitempty"`
	SkillID        string `json:"skill_id,omitempty"`
	MindID         string `json:"mind_id,omitempty"`
	Synergy        string `json:"synergy,omitempty"`
	Actor          int    `json:"actor"`
	Technique      string `json:"technique"`
	Weapon         string `json:"weapon"`
	Effect         string `json:"effect"`
	Damage         int    `json:"damage"`
	Remaining      [2]int `json:"remaining"`
}

type Battle struct {
	Actions        []CombatAction `json:"actions,omitempty"`
	WinProbability [2]float64     `json:"win_probability,omitempty"`
	Score          [2]int         `json:"score,omitempty"`
	Exchanges      []int          `json:"exchanges,omitempty"`
	Version        string         `json:"version"`
	Seed           uint64         `json:"seed"`
	Left           Fighter        `json:"left"`
	Right          Fighter        `json:"right"`
	Winner         int            `json:"winner"`
	Rounds         int            `json:"rounds"`
	Remaining      [2]int         `json:"remaining"`
	Form           [2]float64     `json:"form"`
	Highlights     []string       `json:"highlights"`
}
