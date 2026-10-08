package db

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	JoinModeInherit  = "inherit"
	JoinModeOverride = "override"
	JoinModeDisabled = "disabled"
)

type GroupJoinSetting struct {
	GroupID int64 `gorm:"primaryKey;autoIncrement:false"`
	Mode    string
}

type JoinReviewConfig struct {
	Mode  string   `json:"mode"`
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

type JoinReviewState struct {
	GroupID   int64            `json:"group_id"`
	Config    JoinReviewConfig `json:"config"`
	Global    JoinReviewConfig `json:"global"`
	Effective JoinReviewConfig `json:"effective"`
}

func NormalizeJoinKeywords(words []string) ([]string, error) {
	result := []string{}
	seen := map[string]bool{}
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" || seen[word] {
			continue
		}
		if utf8.RuneCountInString(word) > 128 {
			return nil, fmt.Errorf("每个关键词最多 128 字")
		}
		seen[word] = true
		result = append(result, word)
	}
	if len(result) > 200 {
		return nil, fmt.Errorf("每份名单最多 200 个关键词")
	}
	return result, nil
}

func ValidateJoinReview(groupID int64, cfg JoinReviewConfig) (JoinReviewConfig, error) {
	if groupID < 0 || (cfg.Mode != JoinModeInherit && cfg.Mode != JoinModeOverride && cfg.Mode != JoinModeDisabled) || (groupID == 0 && cfg.Mode == JoinModeInherit) {
		return cfg, fmt.Errorf("无效的入群审核作用域或模式")
	}
	var err error
	if cfg.Allow, err = NormalizeJoinKeywords(cfg.Allow); err != nil {
		return cfg, err
	}
	if cfg.Deny, err = NormalizeJoinKeywords(cfg.Deny); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func resolveJoinReview(local, global JoinReviewConfig) JoinReviewConfig {
	if local.Mode == JoinModeInherit {
		local = global
	}
	if local.Mode == JoinModeDisabled {
		return JoinReviewConfig{Mode: JoinModeDisabled, Allow: []string{}, Deny: []string{}}
	}
	return local
}

func GetJoinReview(groupID int64) (JoinReviewState, error) {
	state := JoinReviewState{GroupID: groupID}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var settings []GroupJoinSetting
		if err := tx.Where("group_id IN ?", []int64{0, groupID}).Find(&settings).Error; err != nil {
			return err
		}
		var rows []GroupJoinRule
		if err := tx.Where("group_id IN ?", []int64{0, groupID}).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		read := func(id int64) JoinReviewConfig {
			cfg := JoinReviewConfig{Mode: JoinModeInherit, Allow: []string{}, Deny: []string{}}
			if id == 0 {
				cfg.Mode = JoinModeOverride
			}
			for _, row := range rows {
				if row.GroupID != id {
					continue
				}
				cfg.Mode = JoinModeOverride
				if row.Action == JoinActionAllow {
					cfg.Allow = append(cfg.Allow, row.Keyword)
				}
				if row.Action == JoinActionDeny {
					cfg.Deny = append(cfg.Deny, row.Keyword)
				}
			}
			for _, setting := range settings {
				if setting.GroupID == id {
					cfg.Mode = setting.Mode
				}
			}
			return cfg
		}
		state.Config, state.Global = read(groupID), read(0)
		state.Effective = resolveJoinReview(state.Config, state.Global)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return state, err
}

func SetJoinReview(groupID int64, cfg JoinReviewConfig) error {
	cfg, err := ValidateJoinReview(groupID, cfg)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := setJoinMode(tx, groupID, cfg.Mode); err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&GroupJoinRule{}).Error; err != nil {
			return err
		}
		for action, words := range map[string][]string{JoinActionAllow: cfg.Allow, JoinActionDeny: cfg.Deny} {
			for _, word := range words {
				if err := tx.Create(&GroupJoinRule{GroupID: groupID, Action: action, Keyword: word}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func setJoinMode(tx *gorm.DB, groupID int64, mode string) error {
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "group_id"}}, DoUpdates: clause.AssignmentColumns([]string{"mode"})}).Create(&GroupJoinSetting{GroupID: groupID, Mode: mode}).Error
}
