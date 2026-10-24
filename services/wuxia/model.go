package wuxia

import (
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/combat"
	"time"
)

type Settings struct {
	DailyRuns    int `json:"daily_runs"`
	SignSilver   int `json:"sign_silver"`
	PracticeBase int `json:"practice_base"`
}

func DefaultSettings() Settings { return Settings{DailyRuns: 3, SignSilver: 0, PracticeBase: 50} }

type Character struct {
	Inventory     map[string]int64  `gorm:"serializer:json;type:jsonb" json:"inventory"`
	Equipment     map[string]string `gorm:"serializer:json;type:jsonb" json:"equipment"`
	LearnedSkills map[string]bool   `gorm:"serializer:json;type:jsonb" json:"learned_skills"`
	ActiveMind    string            `json:"active_mind"`
	ActiveSkill   string            `json:"active_skill"`
	Draws         map[string]int64  `gorm:"serializer:json;type:jsonb" json:"draws"`
	GroupID       int64             `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	UserID        int64             `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	Name          string            `json:"name"`
	ProfessionID  string            `json:"profession_id"`
	Experience    int64             `json:"experience"`
	Silver        int64             `json:"silver"`
	Mastery       int               `json:"mastery"`
	WeaponKind    string            `json:"weapon_kind"`
	Weapons       map[string]string `gorm:"serializer:json;type:jsonb" json:"weapons"`
	Armor         int               `json:"armor"`
	Charm         int               `json:"charm"`
	Portrait      string            `json:"portrait"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `gorm:"autoUpdateTime:false" json:"updated_at"`
}

func (Character) TableName() string { return "wuxia_characters" }

type View struct {
	Collection   []Weapon   `json:"collection,omitempty"`
	MaxLevel     int        `json:"max_level"`
	ArmorItem    *Item      `json:"armor_item,omitempty"`
	CharmItem    *Item      `json:"charm_item,omitempty"`
	Mind         *Skill     `json:"mind,omitempty"`
	Synergy      string     `json:"synergy,omitempty"`
	Skill        *Skill     `json:"skill,omitempty"`
	Character    Character  `json:"character"`
	Level        int        `json:"level"`
	NextXP       int64      `json:"next_xp"`
	Title        string     `json:"title"`
	Profession   Profession `json:"profession"`
	Weapon       Weapon     `json:"weapon"`
	Stats        Stats      `json:"stats"`
	Power        int        `json:"power"`
	PracticeCost int64      `json:"practice_cost"`
}

type Daily struct {
	DuelRewarded bool   `json:"duel_rewarded"`
	GroupID      int64  `gorm:"primaryKey;autoIncrement:false" json:"-"`
	UserID       int64  `gorm:"primaryKey;autoIncrement:false" json:"-"`
	Day          string `gorm:"primaryKey" json:"day"`
	Signed       bool   `json:"signed"`
	Runs         int    `json:"runs"`
}

func (Daily) TableName() string { return "wuxia_daily_claims" }

type Fighter = combat.Fighter
type CombatAction = combat.CombatAction
type Battle = combat.Battle

type Encounter struct {
	Kind       string  `json:"kind,omitempty"`
	EventID    string  `json:"event_id"`
	Title      string  `json:"title"`
	Scene      string  `json:"scene"`
	Text       string  `json:"text"`
	Silver     int64   `json:"silver"`
	Experience int64   `json:"experience"`
	Loot       string  `json:"loot,omitempty"`
	Battle     *Battle `json:"battle,omitempty"`
}

type Result struct {
	LearnedNames    []string         `json:"learned_names,omitempty"`
	Casino          *CasinoResult    `json:"casino,omitempty"`
	RewardItems     []Item           `json:"reward_items,omitempty"`
	DrawPoolName    string           `json:"draw_pool_name,omitempty"`
	RewardSeed      uint64           `json:"reward_seed,omitempty"`
	DrawPool        string           `json:"draw_pool,omitempty"`
	Rewards         []Reward         `json:"rewards,omitempty"`
	InventoryDelta  map[string]int64 `json:"inventory_delta,omitempty"`
	TargetJoined    bool             `json:"target_joined,omitempty"`
	Joined          bool             `json:"joined,omitempty"`
	SettledAt       time.Time        `json:"settled_at"`
	Version         string           `json:"version"`
	Actor           View             `json:"actor"`
	Daily           Daily            `json:"daily"`
	DailyLimit      int              `json:"daily_limit"`
	SilverDelta     int64            `json:"silver_delta"`
	ExperienceDelta int64            `json:"experience_delta"`
	Lines           []string         `json:"lines"`
	Encounters      []Encounter      `json:"encounters,omitempty"`
	Battle          *Battle          `json:"battle,omitempty"`
	Replayed        bool             `json:"replayed"`
}

type Request struct {
	TargetName string
	BotID      int64
	GroupID    int64
	UserID     int64
	MessageID  int32
	Name       string
	Action     string
	Argument   string
	TargetID   int64
}
