package combat

import "github.com/Yuelioi/yueling-go/services/wuxia/internal/content"

type Stats = content.Stats
type Traits = content.Traits
type Fighter struct {
	Techniques [3]string `json:"techniques,omitempty"`
	Portrait   string    `json:"portrait,omitempty"`
	Name       string    `json:"name"`
	Profession string    `json:"profession"`
	Weapon     string    `json:"weapon"`
	Stats      Stats     `json:"stats"`
	Traits     Traits    `json:"traits"`
}

type CombatAction struct {
	Actor     int    `json:"actor"`
	Technique string `json:"technique"`
	Weapon    string `json:"weapon"`
	Effect    string `json:"effect"`
	Damage    int    `json:"damage"`
	Remaining [2]int `json:"remaining"`
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
