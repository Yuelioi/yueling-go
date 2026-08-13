package db

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxGroupAIStylePromptChars = 4000

var ErrGroupAIStylePromptTooLong = errors.New("group AI style prompt is too long")

// GroupAISetting stores the administrator-defined conversation style for one group.
// A missing row means the built-in default style is in use.
type GroupAISetting struct {
	GroupID     int64  `gorm:"primaryKey;autoIncrement:false" json:"group_id"`
	StylePrompt string `gorm:"type:text;not null" json:"style_prompt"`
	UpdatedAt   int64  `gorm:"not null" json:"updated_at"`
}

func GetGroupAIStylePrompt(groupID int64) (string, bool, error) {
	var row GroupAISetting
	err := DB.Where("group_id = ?", groupID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.StylePrompt, true, nil
}

func SetGroupAIStylePrompt(groupID int64, prompt string) (*GroupAISetting, error) {
	prompt = strings.TrimSpace(prompt)
	if utf8.RuneCountInString(prompt) > MaxGroupAIStylePromptChars {
		return nil, ErrGroupAIStylePromptTooLong
	}
	row := GroupAISetting{
		GroupID:     groupID,
		StylePrompt: prompt,
		UpdatedAt:   time.Now().Unix(),
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"style_prompt", "updated_at",
		}),
	}).Create(&row).Error
	return &row, err
}

func DeleteGroupAIStylePrompt(groupID int64) error {
	return DB.Where("group_id = ?", groupID).Delete(&GroupAISetting{}).Error
}
